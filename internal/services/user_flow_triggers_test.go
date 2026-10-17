package services

import (
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/flowengine"
)

func new_trigger_test_service(t *testing.T) *AutomationService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "automation.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.UserFlow{}, &model.FlowSchedule{}, &model.FlowRunRecord{}); err != nil {
		t.Fatal(err)
	}
	sql_db, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sql_db.Close() })
	return NewAutomationService(db, nil, flowengine.NewWorkflowEngine(), nil)
}

func trigger_test_flow(t *testing.T, service *AutomationService) *model.UserFlow {
	t.Helper()
	flow, err := service.CreateUserFlow(CreateUserFlowInput{Name: "test", TriggerType: "Manual"})
	if err != nil {
		t.Fatal(err)
	}
	return flow
}

func trigger_test_rules(t *testing.T, service *AutomationService, flow_id string) []model.FlowSchedule {
	t.Helper()
	rules, _, err := service.ListSchedules(ListSchedulesInput{FlowID: flow_id})
	if err != nil {
		t.Fatal(err)
	}
	return rules
}

func TestUserFlowMultipleTriggers(t *testing.T) {
	service := new_trigger_test_service(t)
	flow := trigger_test_flow(t, service)
	_, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{Triggers: []UserFlowTriggerInput{
		{Type: "Cron", Name: "morning", CronExpr: "0 8 * * *", Enabled: true},
		{Type: "Cron", Name: "evening", CronExpr: "0 20 * * *", Enabled: false},
		{Type: "Event", Name: "new video", EventKey: "video.created", InitialData: map[string]interface{}{"url": "example"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	rules := trigger_test_rules(t, service, flow.ID)
	if len(rules) != 3 {
		t.Fatalf("got %d triggers", len(rules))
	}
	var morning, event_rule model.FlowSchedule
	for _, rule := range rules {
		metadata := automation_schedule_metadata(rule)
		if metadata.StartNodeID != "start" {
			t.Fatalf("wrong entry node: %+v", metadata)
		}
		switch rule.Name {
		case "morning":
			morning = rule
			if !rule.Enabled || rule.NextRunAt == nil {
				t.Fatal("enabled cron has no next run")
			}
		case "evening", "new video":
			if rule.Enabled || rule.NextRunAt != nil {
				t.Fatal("paused or event rule was scheduled")
			}
		}
		if metadata.Type == "Event" {
			event_rule = rule
		}
	}
	run, err := service.TriggerScheduleAs(event_rule.ID, "Event", "video.created")
	if err != nil || run.Status != model.FlowRunStatusCompleted || run.TriggerType != "Event" || run.TriggerKey != "video.created" {
		t.Fatalf("event execution: run=%+v err=%v", run, err)
	}
	if flow_initial_data(event_rule)["url"] != "example" {
		t.Fatal("event input was lost")
	}
	// The same graph also runs through the existing manual entry.
	if _, err := service.TriggerFlowDirect(flow.ID, nil); err != nil {
		t.Fatal(err)
	}
	service.wg.Wait()
	// Force only the enabled rule due and verify the scheduler records Cron.
	due := time.Now().Add(-time.Minute).UnixMilli()
	if err := service.db.Model(&model.FlowSchedule{}).Where("id = ?", morning.ID).Update("next_run_at", due).Error; err != nil {
		t.Fatal(err)
	}
	service.tick()
	service.wg.Wait()
	runs, _, err := service.ListRuns(ListRunsInput{ScheduleID: morning.ID})
	if err != nil || len(runs) != 1 || runs[0].TriggerType != "Cron" || runs[0].Status != model.FlowRunStatusCompleted {
		t.Fatalf("cron execution: runs=%+v err=%v", runs, err)
	}
}

func TestUserFlowTriggerSavePreservesAndRemovesRules(t *testing.T) {
	service := new_trigger_test_service(t)
	flow := trigger_test_flow(t, service)
	rule, err := service.CreateSchedule(CreateScheduleInput{
		Name: "legacy cron", FlowID: flow.ID, CronExpr: "@daily", TimeoutSec: 120,
		InitialData: map[string]interface{}{"url": "original"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Older clients omit triggers; their graph saves must preserve the rules.
	if _, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{Triggers: []UserFlowTriggerInput{
		{ID: rule.ID, Type: "Cron", Name: "renamed", CronExpr: "@daily", Enabled: true},
	}}); err != nil {
		t.Fatal(err)
	}
	saved, err := service.GetSchedule(rule.ID)
	if err != nil || saved.ID != rule.ID || saved.TimeoutSec != 120 || flow_initial_data(*saved)["url"] != "original" || *saved.NextRunAt != *rule.NextRunAt {
		t.Fatalf("existing rule was not preserved: %+v, err=%v", saved, err)
	}
	if _, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{Triggers: []UserFlowTriggerInput{}}); err != nil {
		t.Fatal(err)
	}
	if rules := trigger_test_rules(t, service, flow.ID); len(rules) != 0 {
		t.Fatalf("empty trigger list left %d rules", len(rules))
	}
}

func TestUserFlowTriggerSaveRollsBack(t *testing.T) {
	service := new_trigger_test_service(t)
	flow := trigger_test_flow(t, service)
	other_flow := trigger_test_flow(t, service)
	foreign_rule, err := service.CreateSchedule(CreateScheduleInput{Name: "other", FlowID: other_flow.ID, CronExpr: "@daily"})
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid_rule := range []UserFlowTriggerInput{
		{Type: "Cron", Name: "invalid cron", CronExpr: "invalid"},
		{Type: "Event", Name: "missing key"},
		{Type: "Unknown", Name: "invalid type"},
		{ID: foreign_rule.ID, Type: "Cron", Name: "foreign", CronExpr: "@daily"},
	} {
		name := "should roll back"
		_, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{Name: &name, Triggers: []UserFlowTriggerInput{
			{Type: "Cron", Name: "valid first", CronExpr: "@daily", Enabled: true}, invalid_rule,
		}})
		if err == nil {
			t.Fatalf("accepted invalid rule %+v", invalid_rule)
		}
		saved, err := service.GetUserFlow(flow.ID)
		if err != nil || saved.Name != flow.Name || len(trigger_test_rules(t, service, flow.ID)) != 0 {
			t.Fatalf("partial save after invalid rule: flow=%+v err=%v", saved, err)
		}
	}
	if _, err := service.GetSchedule(foreign_rule.ID); err != nil {
		t.Fatal("foreign rule was changed", err)
	}
}

func TestUserFlowLegacyEventAndDelete(t *testing.T) {
	service := new_trigger_test_service(t)
	flow, err := service.CreateUserFlow(CreateUserFlowInput{Name: "legacy", TriggerType: "Event", EventKey: "old.event"})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{Triggers: []UserFlowTriggerInput{
		{Type: "Event", Name: "event", EventKey: flow.EventKey},
		{Type: "Cron", Name: "cron", CronExpr: "@daily", Enabled: true},
	}})
	if err != nil || saved.EventKey != "" || saved.TriggerType != "Manual" {
		t.Fatalf("legacy event not migrated: flow=%+v err=%v", saved, err)
	}
	if err := service.DeleteUserFlow(flow.ID); err != nil {
		t.Fatal(err)
	}
	if rules := trigger_test_rules(t, service, flow.ID); len(rules) != 0 {
		t.Fatal("deleted pipeline still has active triggers")
	}
}
