package services

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
)

func marshal_automation_initial_data(initial_data map[string]interface{}) (string, error) {
	if initial_data == nil {
		return "", nil
	}
	// Copy the metadata so callers' editor drafts are never mutated.
	copied := make(map[string]interface{}, len(initial_data))
	for key, value := range initial_data {
		copied[key] = value
	}
	if metadata, ok := initial_data[automation_metadata_key].(map[string]interface{}); ok && metadata["type"] == model.FlowRunTriggerWebhook {
		copied_metadata := make(map[string]interface{}, len(metadata)+1)
		for key, value := range metadata {
			copied_metadata[key] = value
		}
		if token, _ := copied_metadata["webhook_token"].(string); token == "" {
			random := make([]byte, 32)
			if _, err := rand.Read(random); err != nil {
				return "", err
			}
			copied_metadata["webhook_token"] = hex.EncodeToString(random)
		}
		copied[automation_metadata_key] = copied_metadata
	}
	return marshal_initial_data(copied)
}

// HandleEvent bridges application events; automation events are excluded to
// avoid recursive status/log-trigger loops. Work executes off the publisher.
func (s *AutomationService) HandleEvent(event events.Event) {
	if event == nil || strings.HasPrefix(event.Type(), "automation.") || event.Type() == events.TypeServiceCommand || event.Type() == events.TypeProxyCommand {
		return
	}
	raw, err := json.Marshal(event)
	if err != nil {
		return
	}
	payload := map[string]interface{}{}
	if json.Unmarshal(raw, &payload) != nil {
		return
	}
	if _, err := s.DispatchEvent(event.Type(), payload); err != nil {
		s.log_error(err, "dispatch automation event")
	}
}

func (s *AutomationService) DispatchEvent(event_key string, payload map[string]interface{}) ([]model.FlowRunRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	event_key = strings.TrimSpace(event_key)
	if event_key == "" {
		return nil, fmt.Errorf("event_key 不能为空")
	}
	if strings.HasPrefix(event_key, "automation.") {
		return nil, fmt.Errorf("不能订阅 automation 内部状态事件")
	}
	var schedules []model.FlowSchedule
	if err := s.db.Where("deleted_at IS NULL").Find(&schedules).Error; err != nil {
		return nil, err
	}
	runs := []model.FlowRunRecord{}
	for _, schedule := range schedules {
		metadata := automation_schedule_metadata(schedule)
		if metadata.Type != model.FlowRunTriggerEvent || metadata.EventKey != event_key || !metadata.Active {
			continue
		}
		initial_data := decode_initial_data(schedule.InitialData)
		initial_data["event"] = map[string]interface{}{"type": event_key, "data": payload}
		schedule.InitialData = marshal_json(initial_data)
		run, err := s.prepare_run(schedule, model.FlowRunTriggerEvent, event_key, "", "")
		if err != nil {
			s.log_error(err, "event trigger could not start")
			continue
		}
		runs = append(runs, *run)
		s.wg.Add(1)
		go func(schedule model.FlowSchedule, run *model.FlowRunRecord) {
			defer s.wg.Done()
			s.execute_prepared_run(schedule, run)
		}(schedule, run)
	}
	return runs, nil
}

func (s *AutomationService) TriggerWebhook(id, token string, payload map[string]interface{}) (*model.FlowRunRecord, error) {
	schedule, err := s.GetSchedule(id)
	if err != nil {
		return nil, err
	}
	metadata := automation_schedule_metadata(*schedule)
	if metadata.Type != model.FlowRunTriggerWebhook || !metadata.Active || token == "" || subtle.ConstantTimeCompare([]byte(metadata.WebhookToken), []byte(token)) != 1 {
		return nil, fmt.Errorf("Webhook 令牌无效或触发器已停用")
	}
	initial_data := decode_initial_data(schedule.InitialData)
	initial_data["event"] = map[string]interface{}{"type": "webhook", "data": payload}
	schedule.InitialData = marshal_json(initial_data)
	run, err := s.prepare_run(*schedule, model.FlowRunTriggerWebhook, id, "", "")
	if err != nil {
		return nil, err
	}
	s.wg.Add(1)
	go func() { defer s.wg.Done(); s.execute_prepared_run(*schedule, run) }()
	return run, nil
}
