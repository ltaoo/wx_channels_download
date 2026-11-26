package services

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/flowengine"
)

func new_automation_checkpoint_service(t *testing.T) (*AutomationService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sql_db, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sql_db.SetMaxOpenConns(1)
	t.Cleanup(func() { sql_db.Close() })
	if err := db.AutoMigrate(&model.FlowRunRecord{}); err != nil {
		t.Fatal(err)
	}
	var seed = model.FlowRunRecord{ID: "run-1", FlowID: "flow-u-1", InstanceID: "instance-u-1", Status: model.FlowRunStatusRunning}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatal(err)
	}
	return &AutomationService{db: db}, db
}

// big_checkpoint stands in for a real context whose serialized form runs to
// several megabytes: persisting it on every in-flight boundary is what starved
// the SQLite writer into "database is locked".
func big_checkpoint(waiting bool) *flowengine.RunCheckpoint {
	rows := make([]string, 0, 4096)
	for i := 0; i < 4096; i++ {
		rows = append(rows, strings.Repeat("x", 256))
	}
	return &flowengine.RunCheckpoint{
		RunID:       "run-1",
		CurrentNode: "n1",
		Waiting:     waiting,
		Data:        map[string]interface{}{"big": rows},
	}
}

func TestRunningCheckpointSkipsSerializedContext(t *testing.T) {
	service, db := new_automation_checkpoint_service(t)
	if err := service.save_run_checkpoint(big_checkpoint(false)); err != nil {
		t.Fatal(err)
	}
	var run model.FlowRunRecord
	if err := db.First(&run, "id = ?", "run-1").Error; err != nil {
		t.Fatal(err)
	}
	if run.Checkpoint != "" {
		t.Fatalf("running checkpoint should not persist serialized context, got %d bytes", len(run.Checkpoint))
	}
	if run.Status != model.FlowRunStatusRunning || run.CurrentNode != "n1" {
		t.Fatalf("expected progress fields persisted, got status=%q node=%q", run.Status, run.CurrentNode)
	}
}

func TestWaitingCheckpointPersistsSerializedContext(t *testing.T) {
	service, db := new_automation_checkpoint_service(t)
	if err := service.save_run_checkpoint(big_checkpoint(true)); err != nil {
		t.Fatal(err)
	}
	var run model.FlowRunRecord
	if err := db.First(&run, "id = ?", "run-1").Error; err != nil {
		t.Fatal(err)
	}
	if run.Status != model.FlowRunStatusWaiting {
		t.Fatalf("expected WAITING, got %q", run.Status)
	}
	if run.Checkpoint == "" {
		t.Fatal("waiting checkpoint must persist the serialized context")
	}
	var restored flowengine.RunCheckpoint
	if err := json.Unmarshal([]byte(run.Checkpoint), &restored); err != nil {
		t.Fatal(err)
	}
	if restored.RunID != "run-1" || !restored.Waiting {
		t.Fatalf("unexpected restored checkpoint: run_id=%q waiting=%v", restored.RunID, restored.Waiting)
	}
}
