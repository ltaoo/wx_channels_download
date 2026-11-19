package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
)

// UserFlowTriggerInput configures an additional entry rule on a pipeline's
// start node. Manual/debug execution is always available without a rule.
type UserFlowTriggerInput struct {
	default_trigger_id    string
	configured_input_keys []string
	Active                *bool                  `json:"active,omitempty"`
	ID                    string                 `json:"id"`
	Type                  string                 `json:"type"`
	Name                  string                 `json:"name"`
	CronExpr              string                 `json:"cron_expr"`
	EventKey              string                 `json:"event_key"`
	Enabled               bool                   `json:"enabled"`
	InitialData           map[string]interface{} `json:"initial_data"`
	TimeoutSec            *int                   `json:"timeout_sec"`
}

// replace_user_flow_triggers runs in the same transaction as the graph save.
// Reuse the schedule validation and persistence paths while retaining IDs,
// history, input parameters, and unchanged next-fire times for existing rules.
func (s *AutomationService) replace_user_flow_triggers(tx *gorm.DB, flow_id string, start_node_id string, inputs []UserFlowTriggerInput) error {
	var schedules []model.FlowSchedule
	if err := tx.Where("instance_id = ? AND deleted_at IS NULL", flow_id).Find(&schedules).Error; err != nil {
		return err
	}
	existing := make(map[string]model.FlowSchedule, len(schedules))
	for _, schedule := range schedules {
		existing[schedule.ID] = schedule
	}
	seen := make(map[string]bool)
	scoped_service := &AutomationService{db: tx, cron_parser: s.cron_parser, syncing_triggers: true}
	for index, input := range inputs {
		id := strings.TrimSpace(input.ID)
		previous, found := existing[id]
		if id != "" && (!found || seen[id]) {
			return fmt.Errorf("触发器 %s 不属于当前 Pipeline、已删除或重复", id)
		}
		seen[id] = true
		trigger_type, event_key, err := normalize_user_flow_trigger(input.Type, input.EventKey)
		if err != nil {
			return fmt.Errorf("触发器 %d: %w", index+1, err)
		}
		name := strings.TrimSpace(input.Name)
		if name == "" {
			return fmt.Errorf("触发器 %d 名称不能为空", index+1)
		}
		initial_data := decode_initial_data(previous.InitialData)
		if input.InitialData != nil {
			initial_data = make(map[string]interface{}, len(input.InitialData)+1)
			for key, value := range input.InitialData {
				initial_data[key] = value
			}
		}
		previous_metadata := automation_schedule_metadata(previous)
		configured_input_keys := input.configured_input_keys
		if configured_input_keys == nil && input.InitialData != nil {
			configured_input_keys = []string{}
			for key := range input.InitialData {
				if key != automation_metadata_key {
					configured_input_keys = append(configured_input_keys, key)
				}
			}
		}
		default_trigger_id := input.default_trigger_id
		if default_trigger_id == "" {
			default_trigger_id = previous_metadata.DefaultTriggerID
		}
		active := previous_metadata.Active
		if input.Active != nil {
			active = *input.Active
		}
		initial_data[automation_metadata_key] = map[string]interface{}{
			"active": active, "webhook_token": previous_metadata.WebhookToken,
			"type": trigger_type, "event_key": event_key, "start_node": start_node_id,
			"default_trigger_id":    default_trigger_id,
			"configured_input_keys": configured_input_keys,
		}
		cron_expr := strings.TrimSpace(input.CronExpr)
		enabled := trigger_type == model.FlowRunTriggerCron && input.Enabled
		if trigger_type != model.FlowRunTriggerCron {
			cron_expr = "@daily" // Inert storage value for non-timed rules.
		}
		if found {
			update := UpdateScheduleInput{Name: &name, InitialData: initial_data, TimeoutSec: input.TimeoutSec}
			if cron_expr != previous.CronExpr {
				update.CronExpr = &cron_expr
			}
			if enabled != previous.Enabled {
				update.Enabled = &enabled
			}
			if _, err := scoped_service.UpdateSchedule(id, update); err != nil {
				return fmt.Errorf("触发器 %d: %w", index+1, err)
			}
		} else {
			timeout_sec := 0
			if input.TimeoutSec != nil {
				timeout_sec = *input.TimeoutSec
			}
			if _, err := scoped_service.CreateSchedule(CreateScheduleInput{
				Name: name, InstanceID: flow_id, CronExpr: cron_expr, InitialData: initial_data,
				Enabled: &enabled, TimeoutSec: timeout_sec,
			}); err != nil {
				return fmt.Errorf("触发器 %d: %w", index+1, err)
			}
		}
	}
	for id := range existing {
		if !seen[id] {
			if err := scoped_service.DeleteSchedule(id); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *AutomationService) normalize_default_triggers(inputs []UserFlowTriggerInput) ([]UserFlowTriggerInput, error) {
	triggers := append([]UserFlowTriggerInput{}, inputs...)
	seen := map[string]bool{}
	for index := range triggers {
		trigger := &triggers[index]
		trigger.ID = strings.TrimSpace(trigger.ID)
		if trigger.ID == "" {
			trigger.ID = fmt.Sprintf("trigger-%d-%d", time.Now().UnixNano(), index)
		}
		if len(trigger.ID) > 96 || seen[trigger.ID] {
			return nil, fmt.Errorf("触发器 ID 重复或过长: %s", trigger.ID)
		}
		seen[trigger.ID] = true
		trigger.Name = strings.TrimSpace(trigger.Name)
		if trigger.Name == "" {
			return nil, fmt.Errorf("触发器 %d 名称不能为空", index+1)
		}
		var err error
		trigger.Type, trigger.EventKey, err = normalize_user_flow_trigger(trigger.Type, trigger.EventKey)
		if err != nil {
			return nil, err
		}
		if trigger.Type == model.FlowRunTriggerEvent && strings.HasPrefix(trigger.EventKey, "automation.") {
			return nil, fmt.Errorf("不能订阅 automation 内部状态事件")
		}
		if trigger.Type == model.FlowRunTriggerCron {
			trigger.CronExpr = strings.TrimSpace(trigger.CronExpr)
			if _, err := s.next_run_at(trigger.CronExpr, time.Now()); err != nil {
				return nil, err
			}
		} else {
			trigger.Enabled = false
		}
		if trigger.TimeoutSec != nil && (*trigger.TimeoutSec <= 0 || *trigger.TimeoutSec > max_schedule_timeout_sec) {
			return nil, fmt.Errorf("timeout_sec 必须在 1 到 %d 之间", max_schedule_timeout_sec)
		}
		if trigger.TimeoutSec == nil {
			timeout_sec := default_schedule_timeout_sec
			trigger.TimeoutSec = &timeout_sec
		}
		if trigger.Active == nil {
			active := true
			trigger.Active = &active
		}
		if _, err := marshal_initial_data(trigger.InitialData); err != nil {
			return nil, err
		}
	}
	return triggers, nil
}

func (s *AutomationService) sync_instance_default_triggers(tx *gorm.DB, instance_id, start_node_id, raw string) error {
	var triggers []UserFlowTriggerInput
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &triggers); err != nil {
			return fmt.Errorf("流程触发器无法解析: %w", err)
		}
	}
	var instance model.UserFlowInstance
	if err := tx.Select("trigger_overrides").Where("id=?", instance_id).First(&instance).Error; err != nil {
		return err
	}
	overrides := map[string]map[string]interface{}{}
	if instance.TriggerOverrides != "" {
		if err := json.Unmarshal([]byte(instance.TriggerOverrides), &overrides); err != nil {
			return fmt.Errorf("实例触发器覆盖值无法解析: %w", err)
		}
	}
	triggers, overrides, err := s.merge_trigger_overrides(triggers, overrides, false)
	if err != nil {
		return err
	}
	var schedules []model.FlowSchedule
	if err := tx.Where("instance_id=? AND deleted_at IS NULL", instance_id).Find(&schedules).Error; err != nil {
		return err
	}
	existing := map[string]model.FlowSchedule{}
	for _, schedule := range schedules {
		existing[automation_schedule_metadata(schedule).DefaultTriggerID] = schedule
	}
	for index := range triggers {
		trigger := &triggers[index]
		trigger.default_trigger_id = trigger.ID
		previous := existing[trigger.ID]
		trigger.ID = previous.ID
		// Runtime input, history and Webhook tokens stay attached to the same rule.
		initial_data := flow_initial_data(previous)
		metadata, _ := decode_initial_data(previous.InitialData)[automation_metadata_key].(map[string]interface{})
		previous_keys, _ := metadata["configured_input_keys"].([]interface{})
		for _, value := range previous_keys {
			if key, ok := value.(string); ok {
				delete(initial_data, key)
			}
		}
		trigger.configured_input_keys = []string{}
		for key, value := range trigger.InitialData {
			initial_data[key] = value
			trigger.configured_input_keys = append(trigger.configured_input_keys, key)
		}
		trigger.InitialData = initial_data
	}
	if err := s.replace_user_flow_triggers(tx, instance_id, start_node_id, triggers); err != nil {
		return err
	}
	return tx.Model(&model.UserFlowInstance{}).Where("id=?", instance_id).Updates(map[string]interface{}{"inherit_triggers": true, "trigger_overrides": marshal_json(overrides)}).Error
}

func (s *AutomationService) merge_trigger_overrides(defaults []UserFlowTriggerInput, overrides map[string]map[string]interface{}, strict bool) ([]UserFlowTriggerInput, map[string]map[string]interface{}, error) {
	known := map[string]bool{}
	cleaned := map[string]map[string]interface{}{}
	triggers := append([]UserFlowTriggerInput{}, defaults...)
	for index, trigger := range defaults {
		known[trigger.ID] = true
		fields := map[string]bool{"name": true, "timeout_sec": true, "initial_data": true}
		if trigger.Type == model.FlowRunTriggerCron {
			fields["cron_expr"], fields["enabled"] = true, true
		} else {
			fields["active"] = true
			if trigger.Type == model.FlowRunTriggerEvent {
				fields["event_key"] = true
			}
		}
		raw, err := json.Marshal(trigger)
		if err != nil {
			return nil, nil, err
		}
		merged := map[string]interface{}{}
		if err := json.Unmarshal(raw, &merged); err != nil {
			return nil, nil, err
		}
		values := map[string]interface{}{}
		for key, value := range overrides[trigger.ID] {
			if !fields[key] {
				if strict {
					return nil, nil, fmt.Errorf("触发器 %s 不支持覆盖字段 %s", trigger.ID, key)
				}
				continue
			}
			if value != nil {
				merged[key], values[key] = value, value
			}
		}
		if len(values) > 0 {
			cleaned[trigger.ID] = values
		}
		raw, err = json.Marshal(merged)
		if err != nil {
			return nil, nil, err
		}
		var merged_trigger UserFlowTriggerInput
		if err := json.Unmarshal(raw, &merged_trigger); err != nil {
			return nil, nil, fmt.Errorf("触发器 %s 覆盖值类型不正确: %w", trigger.ID, err)
		}
		triggers[index] = merged_trigger
	}
	if strict {
		for id := range overrides {
			if !known[id] {
				return nil, nil, fmt.Errorf("流程中不存在触发器: %s", id)
			}
		}
	}
	normalized, err := s.normalize_default_triggers(triggers)
	return normalized, cleaned, err
}

func (s *AutomationService) persist_schedule_change(instance_id, default_id string, fields map[string]interface{}, save func(*gorm.DB) error) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := save(tx); err != nil {
			return err
		}
		if fields == nil || s.syncing_triggers {
			return nil
		}
		var instance model.UserFlowInstance
		if err := tx.Where("id=?", instance_id).First(&instance).Error; err != nil {
			return err
		}
		if !instance.InheritTriggers || default_id == "" || len(fields) == 0 {
			return tx.Model(&instance).Update("inherit_triggers", false).Error
		}
		overrides := map[string]map[string]interface{}{}
		if instance.TriggerOverrides != "" {
			if err := json.Unmarshal([]byte(instance.TriggerOverrides), &overrides); err != nil {
				return err
			}
		}
		if overrides == nil {
			overrides = map[string]map[string]interface{}{}
		}
		if overrides[default_id] == nil {
			overrides[default_id] = map[string]interface{}{}
		}
		for key, value := range fields {
			overrides[default_id][key] = value
		}
		var flow model.UserFlowDefinition
		if err := tx.Select("default_triggers").Where("id=?", instance.FlowID).First(&flow).Error; err != nil {
			return err
		}
		var defaults []UserFlowTriggerInput
		if err := json.Unmarshal([]byte(flow.DefaultTriggers), &defaults); err != nil {
			return err
		}
		_, cleaned, err := s.merge_trigger_overrides(defaults, overrides, true)
		if err != nil {
			return err
		}
		return tx.Model(&instance).Update("trigger_overrides", marshal_json(cleaned)).Error
	})
}
