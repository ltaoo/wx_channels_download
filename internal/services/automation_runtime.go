package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/flowengine"
)

func (s *AutomationService) prepare_run(schedule model.FlowSchedule, trigger_type, trigger_key, retry_of, original_definition string) (*model.FlowRunRecord, error) {
	if s == nil || s.db == nil || s.flow_engine == nil {
		return nil, fmt.Errorf("自动化执行服务未初始化")
	}
	select {
	case <-s.stop:
		return nil, fmt.Errorf("自动化服务已停止")
	default:
	}
	s.claim_mu.Lock()
	defer s.claim_mu.Unlock()
	query := s.db.Model(&model.FlowRunRecord{}).Where("status IN ? AND deleted_at IS NULL", []string{"QUEUED", "RUNNING", "WAITING"})
	if schedule.ID != "" {
		query = query.Where("schedule_id = ?", schedule.ID)
	} else {
		query = query.Where("flow_id = ? AND (schedule_id = '' OR schedule_id IS NULL)", schedule.FlowID)
	}
	var active int64
	if err := query.Count(&active).Error; err != nil {
		return nil, err
	}
	if active != 0 {
		return nil, fmt.Errorf("流程触发器正在执行或等待人工确认")
	}
	var total_active int64
	if err := s.db.Model(&model.FlowRunRecord{}).Where("status IN ?", []string{"QUEUED", "RUNNING"}).Count(&total_active).Error; err != nil {
		return nil, err
	}
	if total_active >= 128 {
		return nil, fmt.Errorf("自动化运行队列已满（128），请稍后重试")
	}
	definition, ok := s.flow_engine.GetFlowDefinition(schedule.FlowID)
	if original_definition != "" {
		if err := json.Unmarshal([]byte(original_definition), &definition); err != nil {
			return nil, err
		}
		ok = true
	}
	if !ok {
		return nil, fmt.Errorf("流程不存在: %s", schedule.FlowID)
	}
	if strings.HasPrefix(schedule.FlowID, user_flow_id_prefix) {
		if err := s.snapshot_flow_dependencies(&definition, 0); err != nil {
			return nil, err
		}
	}
	if start := automation_schedule_metadata(schedule).StartNodeID; start != "" {
		definition.StartNodeID = start
	}
	raw_definition, err := json.Marshal(definition)
	if err != nil {
		// Built-in Go callbacks are not serializable. User-defined graphs must be.
		if strings.HasPrefix(schedule.FlowID, user_flow_id_prefix) {
			return nil, err
		}
		raw_definition = nil
	}
	timeout_sec := schedule.TimeoutSec
	if timeout_sec <= 0 {
		timeout_sec = default_schedule_timeout_sec
	}
	if timeout_sec > max_schedule_timeout_sec {
		return nil, fmt.Errorf("timeout_sec 不能超过 %d", max_schedule_timeout_sec)
	}
	now := time.Now().UnixMilli()
	run := &model.FlowRunRecord{ID: fmt.Sprintf("run-%d", time.Now().UnixNano()), FlowID: schedule.FlowID,
		ScheduleID: schedule.ID, TriggerType: trigger_type, TriggerKey: trigger_key, Status: model.FlowRunStatusQueued,
		Definition: string(raw_definition), InitialData: marshal_json(flow_initial_data(schedule)),
		TimeoutSec: timeout_sec, DeadlineAt: now + int64(timeout_sec)*1000, RetryOf: retry_of, StartedAt: &now,
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now}}
	if err := s.db.Create(run).Error; err != nil {
		return nil, fmt.Errorf("创建运行记录失败，流程未执行: %w", err)
	}
	return run, nil
}

func (s *AutomationService) save_run_checkpoint(checkpoint *flowengine.RunCheckpoint) error {
	if s.db == nil || !strings.HasPrefix(checkpoint.RunID, "run-") {
		return nil
	}
	// Callback-based built-in definitions are intentionally process-local.
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		if !strings.HasPrefix(checkpoint.Definition.ID, user_flow_id_prefix) {
			return nil
		}
		return err
	}
	status := model.FlowRunStatusRunning
	if checkpoint.Waiting {
		status = model.FlowRunStatusWaiting
	}
	result := s.db.Model(&model.FlowRunRecord{}).Where("id = ? AND status IN ?", checkpoint.RunID, []string{"QUEUED", "RUNNING", "WAITING"}).Updates(map[string]interface{}{
		"checkpoint": string(raw), "status": status, "current_node": checkpoint.CurrentNode,
		"node_attempts": marshal_json(checkpoint.NodeAttempts), "node_outputs": marshal_json(checkpoint.Data),
		"updated_at": time.Now().UnixMilli(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return context.Canceled
	}
	return nil
}

func (s *AutomationService) persist_node_log(entry flowengine.NodeExecutionLog) {
	if s.db == nil || !strings.HasPrefix(entry.RunID, "run-") {
		return
	}
	s.log_mu.Lock()
	defer s.log_mu.Unlock()
	var run model.FlowRunRecord
	if err := s.db.Select("id", "node_logs").Where("id = ?", entry.RunID).First(&run).Error; err != nil {
		s.log_error(err, "load automation node logs")
		return
	}
	var logs []flowengine.NodeExecutionLog
	_ = json.Unmarshal([]byte(run.NodeLogs), &logs)
	logs = append(logs, entry)
	if len(logs) > 1000 {
		logs = logs[len(logs)-1000:]
	}
	if err := s.db.Model(&model.FlowRunRecord{}).Where("id = ?", run.ID).Update("node_logs", marshal_json(logs)).Error; err != nil {
		s.log_error(err, "persist automation node logs")
	}
}

// ResumeRun claims the existing waiting record before launching its continuation.
func (s *AutomationService) ResumeRun(id, node_id string, input map[string]interface{}) (*model.FlowRunRecord, error) {
	run, err := s.GetRun(id)
	if err != nil {
		return nil, err
	}
	if run.Status != model.FlowRunStatusWaiting {
		return nil, fmt.Errorf("运行未处于人工等待状态")
	}
	if run.DeadlineAt <= time.Now().UnixMilli() {
		s.expire_waiting_runs()
		return nil, fmt.Errorf("运行已超过整体超时时间")
	}
	checkpoint := &flowengine.RunCheckpoint{}
	if err := json.Unmarshal([]byte(run.Checkpoint), checkpoint); err != nil {
		return nil, fmt.Errorf("运行缺少可恢复的上下文")
	}
	if node_id == "" {
		node_id = checkpoint.CurrentNode
	}
	if err := s.flow_engine.ValidateManualInput(checkpoint, node_id, input); err != nil {
		return nil, err
	}
	s.claim_mu.Lock()
	result := s.db.Model(&model.FlowRunRecord{}).Where("id = ? AND status = ?", id, model.FlowRunStatusWaiting).Updates(map[string]interface{}{"status": model.FlowRunStatusRunning, "updated_at": time.Now().UnixMilli()})
	s.claim_mu.Unlock()
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, fmt.Errorf("该运行已恢复或结束，请刷新")
	}
	run.Status = model.FlowRunStatusRunning
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		execution_context, cancel := context.WithDeadline(s.lifecycle_context, time.UnixMilli(run.DeadlineAt))
		s.active_runs.Store(run.ID, context.CancelFunc(cancel))
		defer s.active_runs.Delete(run.ID)
		defer cancel()
		current, err := s.GetRun(run.ID)
		if err != nil || current.Status == model.FlowRunStatusCancelled {
			return
		}
		s.publish_run_started(run)
		resume_err := s.flow_engine.ResumeCheckpoint(execution_context, checkpoint, node_id, input)
		s.finish_run(model.FlowSchedule{ID: run.ScheduleID, FlowID: run.FlowID, CronExpr: "@daily"}, run, resume_err)
	}()
	return run, nil
}

// RetryRun creates a new execution from the recorded input and original graph,
// or the currently saved graph if the caller explicitly chooses it.
func (s *AutomationService) RetryRun(id string, use_current_definition bool) (*model.FlowRunRecord, error) {
	previous, err := s.GetRun(id)
	if err != nil {
		return nil, err
	}
	if previous.Status != model.FlowRunStatusFailed && previous.Status != model.FlowRunStatusCancelled {
		return nil, fmt.Errorf("只能重试失败或已取消的运行")
	}
	definition := previous.Definition
	if use_current_definition {
		definition = ""
	} else if definition == "" {
		return nil, fmt.Errorf("原流程没有可重放的定义，请选择使用当前流程")
	}
	schedule := model.FlowSchedule{ID: previous.ScheduleID, FlowID: previous.FlowID, CronExpr: "@daily", InitialData: previous.InitialData, TimeoutSec: previous.TimeoutSec}
	run, err := s.prepare_run(schedule, model.FlowRunTriggerManual, "", previous.ID, definition)
	if err != nil {
		return nil, err
	}
	s.wg.Add(1)
	go func() { defer s.wg.Done(); s.execute_prepared_run(schedule, run) }()
	return run, nil
}

func (s *AutomationService) recover_interrupted_runs() {
	now := time.Now().UnixMilli()
	result := s.db.Model(&model.FlowRunRecord{}).Where("status IN ? AND deleted_at IS NULL", []string{"QUEUED", "RUNNING"}).Updates(map[string]interface{}{
		"status": model.FlowRunStatusFailed, "error": "进程中断，最后一个节点的外部副作用可能已发生；请核对后手动重试", "completed_at": now, "updated_at": now,
	})
	if result.Error != nil {
		s.log_error(result.Error, "recover interrupted automation runs")
	}
	s.expire_waiting_runs()
}

func (s *AutomationService) expire_waiting_runs() {
	if s.db == nil {
		return
	}
	now := time.Now().UnixMilli()
	var runs []model.FlowRunRecord
	if err := s.db.Where("status = ? AND deadline_at > 0 AND deadline_at <= ? AND deleted_at IS NULL", model.FlowRunStatusWaiting, now).Find(&runs).Error; err != nil {
		s.log_error(err, "query expired workflow waits")
		return
	}
	for _, run := range runs {
		result := s.db.Model(&model.FlowRunRecord{}).Where("id = ? AND status = ?", run.ID, model.FlowRunStatusWaiting).Updates(map[string]interface{}{"status": model.FlowRunStatusFailed, "error": "context deadline exceeded while waiting for human input", "completed_at": now, "updated_at": now})
		if result.Error == nil && result.RowsAffected == 1 {
			s.flow_engine.CancelRun(run.ID)
			run.Status = model.FlowRunStatusFailed
			run.Error = "context deadline exceeded while waiting for human input"
			s.publish_run_result(&run)
		}
	}
}

func (s *AutomationService) publish_run_started(run *model.FlowRunRecord) {
	s.publish(events.AutomationRunStarted{ScheduleID: run.ScheduleID, RunID: run.ID, FlowID: run.FlowID, Trigger: run.TriggerType})
}
