package services

import (
	"reflect"
	"strings"
	"testing"

	"github.com/robfig/cron/v3"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/flowengine"
)

func TestFlowInstanceDisabled(t *testing.T) {
	service, db := new_automation_checkpoint_service(t)
	if err := db.AutoMigrate(&model.UserFlowDefinition{}, &model.UserFlowInstance{}, &model.FlowSchedule{}); err != nil {
		t.Fatal(err)
	}
	service.flow_engine = flowengine.NewWorkflowEngine()
	service.cron_parser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	flow, err := service.CreateFlowDefinition(CreateUserFlowInput{Name: "Disable test"})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := service.CreateUserFlowInstance(flow.ID)
	if err != nil || instance.Disabled {
		t.Fatalf("new instance should be enabled: %v, %v", instance, err)
	}
	schedules := map[string]*model.FlowSchedule{}
	for _, trigger_type := range []string{model.FlowRunTriggerCron, model.FlowRunTriggerEvent, model.FlowRunTriggerWebhook} {
		enabled := trigger_type == model.FlowRunTriggerCron
		schedule, err := service.CreateSchedule(CreateScheduleInput{
			InstanceID: instance.ID, Name: trigger_type, CronExpr: "@daily", Enabled: &enabled,
			InitialData: map[string]interface{}{automation_metadata_key: map[string]interface{}{
				"type": trigger_type, "active": true, "event_key": "test.event",
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		schedules[trigger_type] = schedule
	}
	// Prime the cache before disabling to verify that changes take effect immediately.
	if entries, err := service.event_triggered_schedules("test.event"); err != nil || len(entries) != 1 {
		t.Fatalf("enabled event rule missing: %v, %v", entries, err)
	}
	running, err := service.prepare_run(*schedules[model.FlowRunTriggerCron], model.FlowRunTriggerManual, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	disabled := true
	instance, err = service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{Disabled: &disabled})
	if err != nil || !instance.Disabled {
		t.Fatalf("disable failed: %v, %v", instance, err)
	}
	name := "Still disabled"
	instance, err = service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{Name: &name})
	if err != nil || !instance.Disabled {
		t.Fatalf("omitting disabled should preserve it: %v, %v", instance, err)
	}
	copied, err := service.CopyUserFlowInstance(instance.ID)
	if err != nil || !copied.Disabled {
		t.Fatalf("copy should preserve disabled: %v, %v", copied, err)
	}
	previous := model.FlowRunRecord{ID: "run-failed", InstanceID: instance.ID, FlowID: flow.ID, Status: model.FlowRunStatusFailed, Definition: flow.Definition}
	if err := db.Create(&previous).Error; err != nil {
		t.Fatal(err)
	}
	webhook := schedules[model.FlowRunTriggerWebhook]
	for name, start := range map[string]func() (*model.FlowRunRecord, error){
		"manual": func() (*model.FlowRunRecord, error) { return service.TriggerFlowInstance(instance.ID, nil) },
		"schedule": func() (*model.FlowRunRecord, error) {
			return service.TriggerSchedule(schedules[model.FlowRunTriggerCron].ID)
		},
		"event": func() (*model.FlowRunRecord, error) {
			return service.TriggerScheduleAs(schedules[model.FlowRunTriggerEvent].ID, model.FlowRunTriggerEvent, "test.event")
		},
		"webhook": func() (*model.FlowRunRecord, error) {
			return service.TriggerWebhook(webhook.ID, automation_schedule_metadata(*webhook).WebhookToken, nil)
		},
		"retry original": func() (*model.FlowRunRecord, error) { return service.RetryRun(previous.ID, false) },
		"retry current":  func() (*model.FlowRunRecord, error) { return service.RetryRun(previous.ID, true) },
		"cron": func() (*model.FlowRunRecord, error) {
			return service.prepare_run(*schedules[model.FlowRunTriggerCron], model.FlowRunTriggerCron, "", "", "")
		},
	} {
		t.Run(name, func(t *testing.T) {
			run, err := start()
			if run != nil || err == nil || !strings.Contains(err.Error(), "已禁用") {
				t.Fatalf("disabled instance accepted a new run: %v, %v", run, err)
			}
		})
	}
	if runs, err := service.DispatchEvent("test.event", nil); err != nil || len(runs) != 0 {
		t.Fatalf("disabled event should be skipped: %v, %v", runs, err)
	}
	// An overdue disabled schedule must not consume a tick or alter its plan.
	due := int64(1)
	if err := db.Model(schedules[model.FlowRunTriggerCron]).Update("next_run_at", due).Error; err != nil {
		t.Fatal(err)
	}
	schedules[model.FlowRunTriggerCron].NextRunAt = &due
	service.tick()
	service.wg.Wait()
	for _, schedule := range schedules {
		actual, err := service.GetSchedule(schedule.ID)
		if err != nil || !reflect.DeepEqual(actual, schedule) {
			t.Fatalf("disabling changed trigger settings: %v, %v, want %v", actual, err, schedule)
		}
	}
	actual_run, err := service.GetRunMetadata(running.ID)
	if err != nil || actual_run.Status != running.Status {
		t.Fatalf("disabling changed an existing run: %v, %v", actual_run, err)
	}
	var count int64
	if err := db.Model(&model.FlowRunRecord{}).Where("instance_id=?", instance.ID).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("unexpected run count: %d, %v", count, err)
	}
	if err := db.Model(running).Update("status", model.FlowRunStatusCompleted).Error; err != nil {
		t.Fatal(err)
	}
	disabled = false
	instance, err = service.UpdateUserFlowInstance(instance.ID, UpdateUserFlowInput{Disabled: &disabled})
	if err != nil || instance.Disabled {
		t.Fatalf("enable failed: %v, %v", instance, err)
	}
	if entries, err := service.event_triggered_schedules("test.event"); err != nil || len(entries) != 1 {
		t.Fatalf("reenabling should restore the event rule immediately: %v, %v", entries, err)
	}
	for _, trigger_type := range []string{model.FlowRunTriggerManual, model.FlowRunTriggerCron, model.FlowRunTriggerEvent, model.FlowRunTriggerWebhook} {
		run, err := service.prepare_run(*schedules[model.FlowRunTriggerCron], trigger_type, "", "", "")
		if err != nil {
			t.Fatalf("reenabled %s start rejected: %v", trigger_type, err)
		}
		if err := db.Model(run).Update("status", model.FlowRunStatusCompleted).Error; err != nil {
			t.Fatal(err)
		}
	}
}
