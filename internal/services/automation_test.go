package services

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ltaoo/velo"

	"wx_channel/internal/database"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/flowengine/engine"
)

// verify_node is a minimal flow node used to prove a scheduled run executes.
type verify_node struct {
	id string
}

func (n *verify_node) ID() string   { return n.id }
func (n *verify_node) Type() string { return "verify" }

func (n *verify_node) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	ctx.Data["ran"] = true
	return true, nil, nil
}

func new_verify_automation_service(t *testing.T) *AutomationService {
	t.Helper()
	app := velo.NewApp(&velo.VeloAppOpt{Mode: velo.ModeHttp})
	db_path := filepath.Join(t.TempDir(), "automation.db")
	if err := app.Migrate(&velo.VeloDatabaseOpt{
		DBType:                    velo.DBTypeSQLite,
		DBPath:                    database.SQLiteDSN(db_path),
		Migrations:                &database.Migrations,
		DisableTimestampCallbacks: true,
	}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	if err := database.ConfigureSQLiteRuntime(app.DB); err != nil {
		t.Fatalf("configure sqlite failed: %v", err)
	}
	if app.DB == nil {
		t.Fatal("migration produced no database handle")
	}

	flow_engine := engine.FlowEngine{}
	flow_engine.SetFlowDefinitions(map[string]engine.FlowDefinition{
		"flow-verify": {
			ID:          "flow-verify",
			StartNodeID: "n1",
			Nodes: map[string]engine.NodeDefinition{
				"n1": {ID: "n1", Type: "verify"},
				"n2": {ID: "n2", Type: "verify"},
			},
		},
	})
	flow_engine.RegisterNode("verify", func(config map[string]interface{}) engine.Node {
		id, _ := config["id"].(string)
		if id == "" {
			id = "n1"
		}
		return &verify_node{id: id}
	})
	return NewAutomationService(app.DB, nil, &flow_engine, nil)
}

func TestAutomationServiceScheduleLifecycle(t *testing.T) {
	service := new_verify_automation_service(t)
	defer service.Stop()

	schedule, err := service.CreateSchedule(CreateScheduleInput{
		Name:     "every minute",
		CronExpr: "@every 1m",
		FlowID:   "flow-verify",
	})
	if err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}
	if !schedule.Enabled {
		t.Fatal("expected a new schedule to be enabled")
	}
	if schedule.NextRunAt == nil || *schedule.NextRunAt <= time.Now().UnixMilli() {
		t.Fatalf("expected a future next_run_at, got %v", schedule.NextRunAt)
	}

	// Invalid cron expressions must be rejected before anything is persisted.
	if _, err := service.CreateSchedule(CreateScheduleInput{
		Name:     "broken",
		CronExpr: "not a cron",
		FlowID:   "flow-verify",
	}); err == nil {
		t.Fatal("expected an invalid cron expression to be rejected")
	}

	run, err := service.TriggerSchedule(schedule.ID)
	if err != nil {
		t.Fatalf("TriggerSchedule failed: %v", err)
	}
	if run.Status != model.FlowRunStatusCompleted {
		t.Fatalf("expected a completed run, got %s (%s)", run.Status, run.Error)
	}
	if run.TriggerType != model.FlowRunTriggerManual {
		t.Fatalf("expected a manual trigger, got %s", run.TriggerType)
	}

	// The schedule must record the outcome and move past the fire time so the
	// next tick does not immediately re-run it.
	reloaded, err := service.GetSchedule(schedule.ID)
	if err != nil {
		t.Fatalf("GetSchedule failed: %v", err)
	}
	if reloaded.LastRunID != run.ID || reloaded.LastRunStatus != model.FlowRunStatusCompleted {
		t.Fatalf("expected the schedule to record %s/%s, got %s/%s",
			run.ID, model.FlowRunStatusCompleted, reloaded.LastRunID, reloaded.LastRunStatus)
	}

	runs, total, err := service.ListRuns(ListRunsInput{ScheduleID: schedule.ID})
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}
	if total != 1 || len(runs) != 1 {
		t.Fatalf("expected 1 run, got total=%d len=%d", total, len(runs))
	}

	disabled, err := service.ToggleSchedule(schedule.ID)
	if err != nil {
		t.Fatalf("ToggleSchedule failed: %v", err)
	}
	if disabled.Enabled || disabled.NextRunAt != nil {
		t.Fatalf("expected a disabled schedule with no next_run_at, got enabled=%v next=%v",
			disabled.Enabled, disabled.NextRunAt)
	}

	if err := service.DeleteSchedule(schedule.ID); err != nil {
		t.Fatalf("DeleteSchedule failed: %v", err)
	}
	if _, err := service.GetSchedule(schedule.ID); err == nil {
		t.Fatal("expected a deleted schedule to be unreadable")
	}
	schedules, total, err := service.ListSchedules(ListSchedulesInput{})
	if err != nil {
		t.Fatalf("ListSchedules failed: %v", err)
	}
	if total != 0 || len(schedules) != 0 {
		t.Fatalf("expected no schedules after deletion, got total=%d len=%d", total, len(schedules))
	}
}

func TestAutomationServiceEventTriggerUsesConfiguredStartNode(t *testing.T) {
	service := new_verify_automation_service(t)
	defer service.Stop()

	schedule, err := service.CreateSchedule(CreateScheduleInput{
		Name:     "feed event",
		CronExpr: "@daily",
		FlowID:   "flow-verify",
		InitialData: map[string]interface{}{
			"__automation": map[string]interface{}{
				"type":       model.FlowRunTriggerEvent,
				"start_node": "n2",
				"event_key":  "channels.feed.received",
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}
	if schedule.Enabled || schedule.NextRunAt != nil {
		t.Fatalf("expected an event flow to disable cron scheduling, got enabled=%v next=%v", schedule.Enabled, schedule.NextRunAt)
	}

	run, err := service.TriggerScheduleAs(schedule.ID, model.FlowRunTriggerEvent, "")
	if err != nil {
		t.Fatalf("TriggerScheduleAs failed: %v", err)
	}
	if run.TriggerType != model.FlowRunTriggerEvent {
		t.Fatalf("expected an event trigger, got %s", run.TriggerType)
	}
	if run.TriggerKey != "channels.feed.received" {
		t.Fatalf("expected the configured event key, got %s", run.TriggerKey)
	}
	if run.Status != model.FlowRunStatusCompleted {
		t.Fatalf("expected a completed run, got %s (%s)", run.Status, run.Error)
	}
	if run.CurrentNode != "n2" {
		t.Fatalf("expected the run to start at n2, got %s", run.CurrentNode)
	}

	if _, err := service.ToggleSchedule(schedule.ID); err == nil {
		t.Fatal("expected enabling an event flow through cron scheduling to be rejected")
	}
}

func TestAutomationServiceTicksDueSchedules(t *testing.T) {
	service := new_verify_automation_service(t)
	defer service.Stop()

	schedule, err := service.CreateSchedule(CreateScheduleInput{
		Name:     "due now",
		CronExpr: "@every 1m",
		FlowID:   "flow-verify",
	})
	if err != nil {
		t.Fatalf("CreateSchedule failed: %v", err)
	}
	// Force the schedule into the past so the next tick considers it due.
	past := time.Now().Add(-time.Minute).UnixMilli()
	if err := service.db.Model(&model.FlowSchedule{}).
		Where("id = ?", schedule.ID).
		Update("next_run_at", past).Error; err != nil {
		t.Fatalf("failed to backdate schedule: %v", err)
	}

	service.tick()
	// execute_schedule runs in its own goroutine; wait for it to land.
	deadline := time.Now().Add(5 * time.Second)
	for {
		runs, total, err := service.ListRuns(ListRunsInput{ScheduleID: schedule.ID})
		if err != nil {
			t.Fatalf("ListRuns failed: %v", err)
		}
		if total > 0 {
			if runs[0].TriggerType != model.FlowRunTriggerCron {
				t.Fatalf("expected a cron trigger, got %s", runs[0].TriggerType)
			}
			if runs[0].Status != model.FlowRunStatusCompleted {
				t.Fatalf("expected a completed run, got %s (%s)", runs[0].Status, runs[0].Error)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the scheduler tick never produced a run record")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
