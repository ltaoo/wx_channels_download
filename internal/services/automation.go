package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/flowengine"
)

const (
	default_automation_page_size = 20
	max_automation_page_size     = 200
	default_schedule_timeout_sec = 3600
	max_schedule_timeout_sec     = 24 * 60 * 60

	// automation_metadata_key stores UI trigger metadata alongside user context.
	// Keeping it inside initial_data avoids a migration while still allowing a
	// schedule to select a custom start node and event key.
	automation_metadata_key = "__automation"

	// automation_tick_interval is how often due schedules are looked up. The
	// tick drives scheduling; the cron parser only computes the next fire time,
	// so the effective resolution never exceeds this interval.
	automation_tick_interval = 30 * time.Second

	// automation_tick_batch_limit caps how many schedules a single tick may
	// start. Anything left over is picked up by the following tick.
	automation_tick_batch_limit = 64
)

// AutomationService owns persisted flow schedules, drives them from a database
// backed ticker, and records every execution. It is transport agnostic so the
// HTTP API, the MCP server, and the CLI can share the same behaviour.
type AutomationService struct {
	db          *gorm.DB
	logger      *zerolog.Logger
	flow_engine *flowengine.FlowEngine
	event_bus   events.Publisher
	cron_parser cron.Parser

	start_once sync.Once
	stop_once  sync.Once
	stop       chan struct{}
	wg         sync.WaitGroup
	// running holds the schedules with an in-flight execution. It is the
	// concurrency guard that keeps one slow run from being started again by the
	// next tick before it finishes.
	running sync.Map
}

// NewAutomationService builds the automation service. A nil flow engine still
// allows schedule management; only execution is unavailable.
func NewAutomationService(
	db *gorm.DB,
	logger *zerolog.Logger,
	flow_engine *flowengine.FlowEngine,
	event_bus events.Publisher,
) *AutomationService {
	service := &AutomationService{
		db:          db,
		logger:      logger,
		flow_engine: flow_engine,
		event_bus:   event_bus,
		// Descriptor is enabled alongside the standard five fields so
		// convenience expressions such as "@every 1m" and "@daily" are accepted.
		cron_parser: cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor),
		stop:        make(chan struct{}),
	}
	service.load_user_flows()
	return service
}

// FlowEngine exposes the shared engine so callers can register flow
// definitions that schedules refer to.
func (s *AutomationService) FlowEngine() *flowengine.FlowEngine {
	if s == nil {
		return nil
	}
	return s.flow_engine
}

// Start launches the scheduler loop. Calling it more than once is a no-op.
func (s *AutomationService) Start() {
	if s == nil || s.db == nil {
		return
	}
	s.start_once.Do(func() {
		s.wg.Add(1)
		go s.scheduler_loop()
		s.log_info("automation scheduler started")
	})
}

// Stop signals the scheduler and waits for in-flight runs to finish.
func (s *AutomationService) Stop() {
	if s == nil {
		return
	}
	s.stop_once.Do(func() {
		close(s.stop)
	})
	s.wg.Wait()
}

func (s *AutomationService) scheduler_loop() {
	defer s.wg.Done()
	ticker := time.NewTicker(automation_tick_interval)
	defer ticker.Stop()
	s.tick()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *AutomationService) tick() {
	if s == nil || s.db == nil {
		return
	}
	now := time.Now().UnixMilli()
	var due []model.FlowSchedule
	err := s.db.
		Where("enabled = ? AND next_run_at IS NOT NULL AND next_run_at <= ? AND deleted_at IS NULL", true, now).
		Order("next_run_at ASC").
		Limit(automation_tick_batch_limit).
		Find(&due).Error
	if err != nil {
		s.log_error(err, "failed to query due flow schedules")
		return
	}
	for index := range due {
		schedule := due[index]
		if _, claimed := s.running.LoadOrStore(schedule.ID, struct{}{}); claimed {
			// The previous run of this schedule is still executing; skip this
			// tick and let the next one pick it up.
			continue
		}
		s.wg.Add(1)
		go func(schedule model.FlowSchedule) {
			defer s.wg.Done()
			defer s.running.Delete(schedule.ID)
			s.execute_schedule(schedule, model.FlowRunTriggerCron, schedule.CronExpr)
		}(schedule)
	}
}

// CreateScheduleInput describes a new schedule. InitialData is stored as JSON
// and handed to the flow as its starting context.
type CreateScheduleInput struct {
	Name        string
	Description string
	CronExpr    string
	FlowID      string
	InitialData map[string]interface{}
	Enabled     *bool
	TimeoutSec  int
}

// UpdateScheduleInput patches an existing schedule. Nil fields are left
// untouched; a non-nil InitialData replaces the stored JSON.
type UpdateScheduleInput struct {
	Name        *string
	Description *string
	CronExpr    *string
	FlowID      *string
	InitialData map[string]interface{}
	Enabled     *bool
	TimeoutSec  *int
}

// ListSchedulesInput filters and paginates schedule listings.
type ListSchedulesInput struct {
	Page     int
	PageSize int
	FlowID   string
	Enabled  *bool
	Keyword  string
}

// ListRunsInput filters and paginates run history listings.
type ListRunsInput struct {
	Page       int
	PageSize   int
	ScheduleID string
	FlowID     string
	Status     string
}

// CreateSchedule validates the cron expression, computes the first fire time and persists the schedule.
func (s *AutomationService) CreateSchedule(input CreateScheduleInput) (*model.FlowSchedule, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("name 不能为空")
	}
	cron_expr := strings.TrimSpace(input.CronExpr)
	if cron_expr == "" {
		return nil, fmt.Errorf("cron_expr 不能为空")
	}
	flow_id := strings.TrimSpace(input.FlowID)
	if flow_id == "" {
		return nil, fmt.Errorf("flow_id 不能为空")
	}
	timeout_sec := input.TimeoutSec
	if timeout_sec <= 0 {
		timeout_sec = default_schedule_timeout_sec
	}
	if timeout_sec > max_schedule_timeout_sec {
		return nil, fmt.Errorf("timeout_sec 不能超过 %d", max_schedule_timeout_sec)
	}
	if _, err := s.next_run_at(cron_expr, time.Now()); err != nil {
		return nil, err
	}

	initial_data, err := marshal_initial_data(input.InitialData)
	if err != nil {
		return nil, err
	}
	metadata := automation_schedule_metadata(model.FlowSchedule{
		InitialData: initial_data,
	})

	now := time.Now().UnixMilli()
	enabled := metadata.Type == model.FlowRunTriggerCron
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	if enabled && metadata.Type != model.FlowRunTriggerCron {
		return nil, fmt.Errorf("%s 触发的流程不能启用 Cron 调度", metadata.Type)
	}
	schedule := &model.FlowSchedule{
		ID:          fmt.Sprintf("sched-%d", time.Now().UnixNano()),
		Name:        name,
		Description: strings.TrimSpace(input.Description),
		CronExpr:    cron_expr,
		FlowID:      flow_id,
		InitialData: initial_data,
		Enabled:     enabled,
		TimeoutSec:  timeout_sec,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	next_run, err := s.next_run_at(cron_expr, time.Now())
	if err != nil {
		return nil, err
	}
	if enabled {
		schedule.NextRunAt = next_run
	}
	if err := s.db.Create(schedule).Error; err != nil {
		return nil, fmt.Errorf("创建定时任务失败: %w", err)
	}
	return schedule, nil
}

// UpdateSchedule patches a schedule and recomputes the next fire time when the
// cron expression or the enabled flag changed.
func (s *AutomationService) UpdateSchedule(id string, input UpdateScheduleInput) (*model.FlowSchedule, error) {
	schedule, err := s.GetSchedule(id)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, fmt.Errorf("name 不能为空")
		}
		updates["name"] = name
	}
	if input.Description != nil {
		updates["description"] = strings.TrimSpace(*input.Description)
	}
	if input.FlowID != nil {
		flow_id := strings.TrimSpace(*input.FlowID)
		if flow_id == "" {
			return nil, fmt.Errorf("flow_id 不能为空")
		}
		updates["flow_id"] = flow_id
	}
	if input.TimeoutSec != nil {
		if *input.TimeoutSec <= 0 || *input.TimeoutSec > max_schedule_timeout_sec {
			return nil, fmt.Errorf("timeout_sec 必须在 1 到 %d 之间", max_schedule_timeout_sec)
		}
		updates["timeout_sec"] = *input.TimeoutSec
	}
	if input.InitialData != nil {
		initial_data, err := marshal_initial_data(input.InitialData)
		if err != nil {
			return nil, err
		}
		updates["initial_data"] = initial_data
	}

	// Resolve the cron expression and enabled flag after patching so the next
	// fire time matches the values being persisted.
	cron_expr := schedule.CronExpr
	if input.CronExpr != nil {
		cron_expr = strings.TrimSpace(*input.CronExpr)
		if cron_expr == "" {
			return nil, fmt.Errorf("cron_expr 不能为空")
		}
		updates["cron_expr"] = cron_expr
	}
	enabled := schedule.Enabled
	enabled_changed := false
	if input.Enabled != nil {
		enabled = *input.Enabled
		enabled_changed = true
		updates["enabled"] = enabled
	}
	initial_data := schedule.InitialData
	if value, exists := updates["initial_data"]; exists {
		initial_data, _ = value.(string)
	}
	metadata := automation_schedule_metadata(model.FlowSchedule{
		InitialData: initial_data,
	})
	if enabled && metadata.Type != model.FlowRunTriggerCron {
		return nil, fmt.Errorf("%s 触发的流程不能启用 Cron 调度", metadata.Type)
	}
	if input.CronExpr != nil || enabled_changed {
		next_run, err := s.next_run_at(cron_expr, time.Now())
		if err != nil {
			return nil, err
		}
		if enabled {
			updates["next_run_at"] = *next_run
		} else {
			updates["next_run_at"] = nil
		}
	}
	if len(updates) == 0 {
		return schedule, nil
	}
	updates["updated_at"] = time.Now().UnixMilli()
	if err := s.db.Model(&model.FlowSchedule{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("更新定时任务失败: %w", err)
	}
	return s.GetSchedule(id)
}

// DeleteSchedule soft-deletes a schedule and clears its pending fire time so a
// tick that already loaded it cannot start another run.
func (s *AutomationService) DeleteSchedule(id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("自动化服务未初始化")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id 不能为空")
	}
	now := time.Now().UnixMilli()
	result := s.db.Model(&model.FlowSchedule{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"deleted_at":  now,
			"next_run_at": nil,
			"updated_at":  now,
		})
	if result.Error != nil {
		return fmt.Errorf("删除定时任务失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("定时任务不存在: %s", id)
	}
	return nil
}

// GetSchedule loads a single schedule by id.
func (s *AutomationService) GetSchedule(id string) (*model.FlowSchedule, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("id 不能为空")
	}
	var schedule model.FlowSchedule
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&schedule).Error; err != nil {
		return nil, fmt.Errorf("定时任务不存在: %s", id)
	}
	return &schedule, nil
}

// ListSchedules returns one page of schedules and the total matching count.
func (s *AutomationService) ListSchedules(input ListSchedulesInput) ([]model.FlowSchedule, int64, error) {
	if s == nil || s.db == nil {
		return nil, 0, fmt.Errorf("自动化服务未初始化")
	}
	page, page_size, err := normalize_automation_page(input.Page, input.PageSize)
	if err != nil {
		return nil, 0, err
	}
	query := s.db.Model(&model.FlowSchedule{}).Where("deleted_at IS NULL")
	if flow_id := strings.TrimSpace(input.FlowID); flow_id != "" {
		query = query.Where("flow_id = ?", flow_id)
	}
	if input.Enabled != nil {
		query = query.Where("enabled = ?", *input.Enabled)
	}
	if keyword := strings.TrimSpace(input.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("查询定时任务数量失败: %w", err)
	}
	schedules := make([]model.FlowSchedule, 0, page_size)
	if total > 0 {
		if err := query.Order("created_at DESC").
			Offset((page - 1) * page_size).
			Limit(page_size).
			Find(&schedules).Error; err != nil {
			return nil, 0, fmt.Errorf("查询定时任务失败: %w", err)
		}
	}
	return schedules, total, nil
}

// ToggleSchedule flips the enabled flag and recomputes the next fire time.
func (s *AutomationService) ToggleSchedule(id string) (*model.FlowSchedule, error) {
	schedule, err := s.GetSchedule(id)
	if err != nil {
		return nil, err
	}
	enabled := !schedule.Enabled
	metadata := automation_schedule_metadata(*schedule)
	if enabled && metadata.Type != model.FlowRunTriggerCron {
		return nil, fmt.Errorf("%s 触发的流程不能启用 Cron 调度", metadata.Type)
	}
	updates := map[string]interface{}{
		"enabled":    enabled,
		"updated_at": time.Now().UnixMilli(),
	}
	if enabled {
		next_run, err := s.next_run_at(schedule.CronExpr, time.Now())
		if err != nil {
			return nil, err
		}
		updates["next_run_at"] = *next_run
	} else {
		updates["next_run_at"] = nil
	}
	if err := s.db.Model(&model.FlowSchedule{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("切换定时任务状态失败: %w", err)
	}
	return s.GetSchedule(id)
}

// TriggerSchedule runs a schedule immediately as a manual trigger.
func (s *AutomationService) TriggerSchedule(id string) (*model.FlowRunRecord, error) {
	return s.TriggerScheduleAs(id, model.FlowRunTriggerManual, "")
}

// TriggerScheduleAs runs a schedule immediately with an explicit trigger type.
// It lets the HTTP API distinguish an event trigger from a button-driven
// manual trigger while sharing the same concurrency and persistence path.
func (s *AutomationService) TriggerScheduleAs(id string, trigger_type string, trigger_key string) (*model.FlowRunRecord, error) {
	schedule, err := s.GetSchedule(id)
	if err != nil {
		return nil, err
	}
	if s.flow_engine == nil {
		return nil, fmt.Errorf("流程引擎未初始化")
	}
	metadata := automation_schedule_metadata(*schedule)
	switch trigger_type {
	case model.FlowRunTriggerManual, model.FlowRunTriggerEvent:
	case model.FlowRunTriggerCron:
		return nil, fmt.Errorf("Cron 触发由调度器执行")
	default:
		return nil, fmt.Errorf("不支持的触发类型: %s", trigger_type)
	}
	if trigger_key == "" {
		trigger_key = metadata.EventKey
	}
	if _, claimed := s.running.LoadOrStore(schedule.ID, struct{}{}); claimed {
		return nil, fmt.Errorf("定时任务 %s 正在执行中", schedule.ID)
	}
	defer s.running.Delete(schedule.ID)
	return s.execute_schedule(*schedule, trigger_type, trigger_key), nil
}

// ListRuns returns one page of run history and the total matching count.
func (s *AutomationService) ListRuns(input ListRunsInput) ([]model.FlowRunRecord, int64, error) {
	if s == nil || s.db == nil {
		return nil, 0, fmt.Errorf("自动化服务未初始化")
	}
	page, page_size, err := normalize_automation_page(input.Page, input.PageSize)
	if err != nil {
		return nil, 0, err
	}
	query := s.db.Model(&model.FlowRunRecord{}).Where("deleted_at IS NULL")
	if schedule_id := strings.TrimSpace(input.ScheduleID); schedule_id != "" {
		query = query.Where("schedule_id = ?", schedule_id)
	}
	if flow_id := strings.TrimSpace(input.FlowID); flow_id != "" {
		query = query.Where("flow_id = ?", flow_id)
	}
	if status := strings.TrimSpace(input.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("查询运行记录数量失败: %w", err)
	}
	runs := make([]model.FlowRunRecord, 0, page_size)
	if total > 0 {
		if err := query.Order("created_at DESC").
			Offset((page - 1) * page_size).
			Limit(page_size).
			Find(&runs).Error; err != nil {
			return nil, 0, fmt.Errorf("查询运行记录失败: %w", err)
		}
	}
	return runs, total, nil
}

// GetRun loads a single run record by id.
func (s *AutomationService) GetRun(id string) (*model.FlowRunRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("id 不能为空")
	}
	var run model.FlowRunRecord
	if err := s.db.Where("id = ? AND deleted_at IS NULL", id).First(&run).Error; err != nil {
		return nil, fmt.Errorf("运行记录不存在: %s", id)
	}
	return &run, nil
}

// CancelRun marks a non-terminal run as cancelled. The flow engine executes
// synchronously, so an already in-flight execution cannot be interrupted; this
// records the cancellation and stops it from being treated as successful.
func (s *AutomationService) CancelRun(id string) error {
	run, err := s.GetRun(id)
	if err != nil {
		return err
	}
	switch run.Status {
	case model.FlowRunStatusCompleted, model.FlowRunStatusFailed, model.FlowRunStatusCancelled:
		return fmt.Errorf("运行记录已结束，无法取消: %s", id)
	}
	now := time.Now().UnixMilli()
	if err := s.db.Model(&model.FlowRunRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"status":       model.FlowRunStatusCancelled,
			"completed_at": now,
			"updated_at":   now,
		}).Error; err != nil {
		return fmt.Errorf("取消运行记录失败: %w", err)
	}
	run.Status = model.FlowRunStatusCancelled
	run.CompletedAt = &now
	s.publish(events.AutomationRunFailed{
		ScheduleID: run.ScheduleID,
		RunID:      run.ID,
		FlowID:     run.FlowID,
		Trigger:    run.TriggerType,
		Error:      "cancelled by user",
	})
	return nil
}

// execute_schedule runs one flow for a schedule and persists the outcome. It
// never returns nil for a schedule that could be started; callers own the
// concurrency guard around it.
func (s *AutomationService) execute_schedule(schedule model.FlowSchedule, trigger_type string, trigger_key string) *model.FlowRunRecord {
	now := time.Now()
	now_millis := now.UnixMilli()
	started_at := now_millis
	run := &model.FlowRunRecord{
		ID:          fmt.Sprintf("run-%d", now.UnixNano()),
		ScheduleID:  schedule.ID,
		FlowID:      schedule.FlowID,
		TriggerType: trigger_type,
		TriggerKey:  trigger_key,
		Status:      model.FlowRunStatusRunning,
		StartedAt:   &started_at,
		Timestamps: model.Timestamps{
			CreatedAt: now_millis,
			UpdatedAt: now_millis,
		},
	}
	if s.db != nil {
		if err := s.db.Create(run).Error; err != nil {
			s.log_error(err, "failed to persist flow run record")
		}
	}
	s.publish(events.AutomationRunStarted{
		ScheduleID: schedule.ID,
		RunID:      run.ID,
		FlowID:     schedule.FlowID,
		Trigger:    trigger_type,
	})

	instance_id, status, error_text := s.run_flow(schedule, trigger_type, trigger_key)
	run.Status = status
	run.Error = error_text

	current_node, node_attempts, node_outputs := "", "", ""
	if s.flow_engine != nil && instance_id != "" {
		// The engine keys its history by instance id, not by our run id; the
		// per-node detail lives there.
		if snapshot, ok := s.flow_engine.GetRunSnapshot(instance_id); ok && snapshot != nil {
			current_node = snapshot.CurrentNode
			node_attempts = marshal_json(snapshot.NodeAttempts)
			node_outputs = marshal_json(snapshot.NodeOutputs)
		}
	}
	run.CurrentNode = current_node
	run.NodeAttempts = node_attempts
	run.NodeOutputs = node_outputs

	updates := map[string]interface{}{
		"status":        status,
		"current_node":  current_node,
		"node_attempts": node_attempts,
		"node_outputs":  node_outputs,
		"error":         error_text,
		"updated_at":    time.Now().UnixMilli(),
	}
	if is_terminal_run_status(status) {
		completed_at := time.Now().UnixMilli()
		updates["completed_at"] = completed_at
		finished_at := completed_at
		run.CompletedAt = &finished_at
	}
	if s.db != nil {
		if err := s.db.Model(&model.FlowRunRecord{}).Where("id = ?", run.ID).Updates(updates).Error; err != nil {
			s.log_error(err, "failed to update flow run record")
		}
		s.advance_schedule(schedule, run)
	}

	if status == model.FlowRunStatusCompleted {
		s.publish(events.AutomationRunCompleted{
			ScheduleID: schedule.ID,
			RunID:      run.ID,
			FlowID:     schedule.FlowID,
			Trigger:    trigger_type,
		})
	} else {
		s.publish(events.AutomationRunFailed{
			ScheduleID: schedule.ID,
			RunID:      run.ID,
			FlowID:     schedule.FlowID,
			Trigger:    trigger_type,
			Error:      error_text,
		})
	}
	return run
}

// run_flow starts the flow and translates the engine outcome into the engine
// instance id, a run status, and error text.
func (s *AutomationService) run_flow(schedule model.FlowSchedule, trigger_type string, trigger_key string) (string, string, string) {
	if s.flow_engine == nil {
		return "", model.FlowRunStatusFailed, "流程引擎未初始化"
	}
	metadata := automation_schedule_metadata(schedule)
	initial_data := flow_initial_data(schedule)
	instance_id, run_err := s.flow_engine.StartFlowWithOptions(
		schedule.FlowID,
		initial_data,
		flowengine.StartFlowOptions{
			Trigger: flowengine.TriggerInfo{
				Type:   flowengine.TriggerType(trigger_type),
				Key:    trigger_key,
				Source: schedule.ID,
			},
			StartNodeID: metadata.StartNodeID,
		},
	)
	if run_err != nil {
		return instance_id, model.FlowRunStatusFailed, run_err.Error()
	}
	snapshot, ok := s.flow_engine.GetRunSnapshot(instance_id)
	if !ok || snapshot == nil {
		return instance_id, model.FlowRunStatusCompleted, ""
	}
	switch snapshot.Status {
	case flowengine.RunStatusFailed:
		return instance_id, model.FlowRunStatusFailed, snapshot.Error
	case flowengine.RunStatusCancelled:
		return instance_id, model.FlowRunStatusCancelled, snapshot.Error
	case flowengine.RunStatusWaiting:
		// A manual node is parked waiting for input; it is neither done nor broken.
		return instance_id, model.FlowRunStatusWaiting, snapshot.Error
	default:
		return instance_id, model.FlowRunStatusCompleted, snapshot.Error
	}
}

// advance_schedule records the run outcome on the schedule and moves the cron
// plan forward so the next tick does not immediately re-fire it.
func (s *AutomationService) advance_schedule(schedule model.FlowSchedule, run *model.FlowRunRecord) {
	if s.db == nil {
		return
	}
	updates := map[string]interface{}{
		"last_run_id":     run.ID,
		"last_run_status": run.Status,
		"updated_at":      time.Now().UnixMilli(),
	}
	next_run, err := s.next_run_at(schedule.CronExpr, time.Now())
	if err != nil {
		// A schedule whose expression became invalid would otherwise never be
		// selected again; keep it visible but stop it from firing.
		s.log_error(err, "failed to compute next run time")
		updates["next_run_at"] = nil
	} else {
		updates["next_run_at"] = *next_run
	}
	if err := s.db.Model(&model.FlowSchedule{}).
		Where("id = ? AND deleted_at IS NULL", schedule.ID).
		Updates(updates).Error; err != nil {
		s.log_error(err, "failed to advance flow schedule")
	}
}

// next_run_at validates the cron expression and returns the next fire time in
// epoch milliseconds.
func (s *AutomationService) next_run_at(expression string, from time.Time) (*int64, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, fmt.Errorf("cron_expr 不能为空")
	}
	schedule, err := s.cron_parser.Parse(expression)
	if err != nil {
		return nil, fmt.Errorf("cron 表达式无效: %w", err)
	}
	next := schedule.Next(from).UnixMilli()
	return &next, nil
}

func (s *AutomationService) publish(event events.Event) {
	if s == nil || s.event_bus == nil {
		return
	}
	s.event_bus.Publish(event)
}

func (s *AutomationService) log_info(message string) {
	if s == nil || s.logger == nil {
		return
	}
	s.logger.Info().Str("file", "internal/services/automation.go").Msg(message)
}

func (s *AutomationService) log_error(err error, message string) {
	if s == nil || s.logger == nil {
		return
	}
	s.logger.Error().Err(err).Str("file", "internal/services/automation.go").Msg(message)
}

func is_terminal_run_status(status string) bool {
	switch status {
	case model.FlowRunStatusCompleted, model.FlowRunStatusFailed, model.FlowRunStatusCancelled:
		return true
	default:
		return false
	}
}

func normalize_automation_page(page int, page_size int) (int, int, error) {
	if page == 0 {
		page = 1
	}
	if page_size == 0 {
		page_size = default_automation_page_size
	}
	if page < 1 {
		return 0, 0, fmt.Errorf("page 必须是正整数")
	}
	if page_size < 1 || page_size > max_automation_page_size {
		return 0, 0, fmt.Errorf("page_size 必须在 1 到 %d 之间", max_automation_page_size)
	}
	return page, page_size, nil
}

type automation_schedule_config struct {
	Type        string
	StartNodeID string
	EventKey    string
}

func automation_schedule_metadata(schedule model.FlowSchedule) automation_schedule_config {
	raw, _ := decode_initial_data(schedule.InitialData)[automation_metadata_key].(map[string]interface{})
	config := automation_schedule_config{
		Type: model.FlowRunTriggerCron,
	}
	if raw == nil {
		return config
	}
	if value, _ := raw["type"].(string); value != "" {
		switch value {
		case model.FlowRunTriggerCron, model.FlowRunTriggerEvent, model.FlowRunTriggerManual:
			config.Type = value
		}
	}
	config.StartNodeID, _ = raw["start_node"].(string)
	config.EventKey, _ = raw["event_key"].(string)
	return config
}

func flow_initial_data(schedule model.FlowSchedule) map[string]interface{} {
	data := decode_initial_data(schedule.InitialData)
	delete(data, automation_metadata_key)
	return data
}

func marshal_initial_data(initial_data map[string]interface{}) (string, error) {
	if initial_data == nil {
		return "", nil
	}
	encoded, err := json.Marshal(initial_data)
	if err != nil {
		return "", fmt.Errorf("initial_data 无法序列化: %w", err)
	}
	return string(encoded), nil
}

// decode_initial_data tolerates empty and malformed values so a bad schedule
// still runs with an empty context instead of failing to start.
func decode_initial_data(raw string) map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]interface{}{}
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil || data == nil {
		return map[string]interface{}{}
	}
	return data
}

func marshal_json(value interface{}) string {
	if value == nil {
		return ""
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}
