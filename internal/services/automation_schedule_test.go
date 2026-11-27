package services

import (
	"testing"
	"time"

	"github.com/robfig/cron/v3"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/flowengine"
)

func TestAutomationSchedulePolicies(t *testing.T) {
	service, db := new_automation_checkpoint_service(t)
	if err := db.AutoMigrate(&model.UserFlowDefinition{}, &model.UserFlowInstance{}, &model.FlowSchedule{}); err != nil {
		t.Fatal(err)
	}
	service.flow_engine = flowengine.NewWorkflowEngine()
	service.cron_parser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	flow, err := service.CreateFlowDefinition(CreateUserFlowInput{Name: "Schedule policies"})
	if err != nil {
		t.Fatal(err)
	}
	defaults := []UserFlowTriggerInput{{ID: "timed", Type: "Cron", Name: "Timed", Enabled: true, Schedule: &flowengine.SchedulePolicy{Mode: "interval", Interval: 300}}}
	if _, err := service.UpdateFlowDefinition(flow.ID, UpdateUserFlowInput{Triggers: defaults}); err != nil {
		t.Fatal(err)
	}
	instance, err := service.CreateUserFlowInstance(flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	load := func() *model.FlowSchedule {
		t.Helper()
		rows, _, err := service.ListSchedules(ListSchedulesInput{InstanceID: instance.ID})
		if err != nil || len(rows) != 1 {
			t.Fatalf("schedules: %v, %v", rows, err)
		}
		return &rows[0]
	}
	schedule := load()
	if schedule.NextRunAt == nil || *schedule.NextRunAt < time.Now().Add(299*time.Second).UnixMilli() {
		t.Fatalf("interval not scheduled: %+v", schedule)
	}
	first_time, first_id := *schedule.NextRunAt, schedule.ID
	if _, err := service.UpdateFlowDefinition(flow.ID, UpdateUserFlowInput{Triggers: defaults}); err != nil {
		t.Fatal(err)
	}
	if schedule = load(); schedule.ID != first_id || *schedule.NextRunAt != first_time {
		t.Fatal("unchanged rule lost schedule or next run")
	}
	if _, err := service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{TriggerOverrides: map[string]map[string]interface{}{
		"timed": {"schedule": map[string]interface{}{"mode": "manual"}},
	}}); err != nil {
		t.Fatal(err)
	}
	if schedule = load(); schedule.NextRunAt != nil {
		t.Fatal("manual policy retained next run")
	}
	service.advance_schedule(*schedule, &model.FlowRunRecord{ID: "done", TriggerType: "Cron", Status: "COMPLETED"})
	if load().NextRunAt != nil {
		t.Fatal("completion rearmed manual rule")
	}
	if _, err := service.ToggleSchedule(schedule.ID); err != nil {
		t.Fatal(err)
	}
	if toggled, err := service.ToggleSchedule(schedule.ID); err != nil || toggled.NextRunAt != nil {
		t.Fatalf("manual toggle: %v, %v", toggled, err)
	}
	defaults[0].Schedule.Interval = 600
	if _, err := service.UpdateFlowDefinition(flow.ID, UpdateUserFlowInput{Triggers: defaults}); err != nil {
		t.Fatal(err)
	}
	if load().NextRunAt != nil {
		t.Fatal("default update discarded explicit override")
	}
	if _, err := service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{TriggerOverrides: map[string]map[string]interface{}{}}); err != nil {
		t.Fatal(err)
	}
	if schedule = load(); schedule.NextRunAt == nil || *schedule.NextRunAt < time.Now().Add(599*time.Second).UnixMilli() {
		t.Fatal("restoring inheritance did not rearm interval")
	}
	if _, err := service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{TriggerOverrides: map[string]map[string]interface{}{
		"timed": {"cron_expr": "0 8 * * 1-5"},
	}}); err != nil {
		t.Fatal(err)
	}
	if policy, err := automation_schedule_policy(*load()); err != nil || policy.Mode != "cron" || policy.Cron != "0 8 * * 1-5" {
		t.Fatalf("legacy cron override ignored: %v, %v", policy, err)
	}
	if _, err := service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{TriggerOverrides: map[string]map[string]interface{}{}}); err != nil {
		t.Fatal(err)
	}
	// A policy can be removed by returning the default rule to legacy Cron.
	defaults[0].Schedule, defaults[0].CronExpr = nil, "@daily"
	if _, err := service.UpdateFlowDefinition(flow.ID, UpdateUserFlowInput{Triggers: defaults}); err != nil {
		t.Fatal(err)
	}
	if policy, err := automation_schedule_policy(*load()); err != nil || policy != nil {
		t.Fatalf("legacy cron retained policy: %v, %v", policy, err)
	}
	created, err := service.CreateSchedule(CreateScheduleInput{InstanceID: instance.ID, Name: "Direct", Schedule: &flowengine.SchedulePolicy{Mode: "interval", Interval: 300}})
	if err != nil {
		t.Fatal(err)
	}
	due := int64(1)
	if err := db.Model(created).Update("next_run_at", due).Error; err != nil {
		t.Fatal(err)
	}
	service.advance_claimed_schedule(*created)
	created, err = service.GetSchedule(created.ID)
	if err != nil || created.NextRunAt == nil || *created.NextRunAt <= due {
		t.Fatalf("claim failed to advance: %v, %v", created, err)
	}
	expression := "0 9 * * *"
	if _, err := service.UpdateSchedule(created.ID, UpdateScheduleInput{CronExpr: &expression}); err != nil {
		t.Fatal(err)
	}
	current, _ := service.GetSchedule(created.ID)
	if policy, err := automation_schedule_policy(*current); err != nil || policy.Mode != "cron" || policy.Cron != expression {
		t.Fatalf("direct cron patch ignored: %v, %v", policy, err)
	}
	if _, err := service.CreateSchedule(CreateScheduleInput{InstanceID: instance.ID, Name: "Invalid weekdays", Schedule: &flowengine.SchedulePolicy{Mode: "times", Days: []int{}, Times: []string{"08:00"}}}); err == nil {
		t.Fatal("empty weekdays accepted")
	}
	updated, err := service.UpdateSchedule(created.ID, UpdateScheduleInput{Schedule: &flowengine.SchedulePolicy{Mode: "manual"}})
	if err != nil || updated.NextRunAt != nil {
		t.Fatalf("policy-only patch: %v, %v", updated, err)
	}
	if _, err := service.UpdateSchedule(created.ID, UpdateScheduleInput{Schedule: &flowengine.SchedulePolicy{Mode: "interval", Interval: -1}}); err == nil {
		t.Fatal("invalid policy accepted")
	}
	if after, _ := service.GetSchedule(created.ID); after.NextRunAt != nil {
		t.Fatal("invalid update changed persisted schedule")
	}
}
