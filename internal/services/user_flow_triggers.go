package services

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
)

// UserFlowTriggerInput configures an additional entry rule on a pipeline's
// start node. Manual/debug execution is always available without a rule.
type UserFlowTriggerInput struct {
	Active      *bool                  `json:"active,omitempty"`
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	CronExpr    string                 `json:"cron_expr"`
	EventKey    string                 `json:"event_key"`
	Enabled     bool                   `json:"enabled"`
	InitialData map[string]interface{} `json:"initial_data"`
	TimeoutSec  *int                   `json:"timeout_sec"`
}

// replace_user_flow_triggers runs in the same transaction as the graph save.
// Reuse the schedule validation and persistence paths while retaining IDs,
// history, input parameters, and unchanged next-fire times for existing rules.
func (s *AutomationService) replace_user_flow_triggers(tx *gorm.DB, flow_id string, start_node_id string, inputs []UserFlowTriggerInput) error {
	var schedules []model.FlowSchedule
	if err := tx.Where("flow_id = ? AND deleted_at IS NULL", flow_id).Find(&schedules).Error; err != nil {
		return err
	}
	existing := make(map[string]model.FlowSchedule, len(schedules))
	for _, schedule := range schedules {
		existing[schedule.ID] = schedule
	}
	seen := make(map[string]bool)
	scoped_service := &AutomationService{db: tx, cron_parser: s.cron_parser}
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
		active := previous_metadata.Active
		if input.Active != nil {
			active = *input.Active
		}
		initial_data[automation_metadata_key] = map[string]interface{}{
			"active": active, "webhook_token": previous_metadata.WebhookToken,
			"type": trigger_type, "event_key": event_key, "start_node": start_node_id,
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
				Name: name, FlowID: flow_id, CronExpr: cron_expr, InitialData: initial_data,
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
