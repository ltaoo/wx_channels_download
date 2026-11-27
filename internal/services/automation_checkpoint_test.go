package services

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"wx_channel/internal/database"
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

func TestAutomationAndDownloadsShareSQLiteWriteQueue(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(database.SQLiteDSN(filepath.Join(t.TempDir(), "automation.db"))), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sql_db, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sql_db.Close()
	if err := database.ConfigureSQLiteRuntime(db); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db = db.WithContext(ctx)
	if err := db.AutoMigrate(&model.FlowRunRecord{}, &model.FlowRunNodeLog{}, &model.DownloadTask{}, &model.DownloadResource{}, &model.DownloadEndpoint{}, &model.DownloadConnection{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.FlowRunRecord{ID: "run-1", FlowID: "flow-u-1", Status: model.FlowRunStatusRunning}).Error; err != nil {
		t.Fatal(err)
	}
	service := &AutomationService{db: db}
	download_dir := t.TempDir()
	download_service := NewDownloadTaskService(db, nil, nil, nil, download_dir, download_dir, nil)
	store := database.NewDBTaskStore(db, nil)
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	if err := tx.Model(&model.FlowRunRecord{}).Where("id = ?", "run-1").Update("current_node", "held").Error; err != nil {
		t.Fatal(err)
	}
	writes := []func() error{
		func() error { return service.save_run_checkpoint(big_checkpoint(true)) },
		func() error {
			auto_start := false
			created, err := download_service.CreateTaskByURL(CreateDownloadTaskByURLBody{URL: "https://example.com/video.mp4", AutoStart: &auto_start})
			if err != nil {
				return err
			}
			return store.UpdateResourceProgress(created.Resource.Id, 123, 45)
		},
		func() error {
			service.write_node_log_batch([]model.FlowRunNodeLog{{RunID: "run-1", FlowID: "flow-u-1", NodeID: "n1", Entry: `{}`}})
			return nil
		},
	}
	var workers sync.WaitGroup
	errors := make(chan error, len(writes))
	initial_wait_count := sql_db.Stats().WaitCount
	for _, write := range writes {
		workers.Add(1)
		go func(write func() error) {
			defer workers.Done()
			errors <- write()
		}(write)
	}
	// A held write transaction must queue every service at the shared pool,
	// rather than opening competing SQLite connections and hitting busy_timeout.
	wait_deadline := time.Now().Add(2 * time.Second)
	for sql_db.Stats().WaitCount-initial_wait_count < int64(len(writes)) && time.Now().Before(wait_deadline) {
		time.Sleep(time.Millisecond)
	}
	wait_count := sql_db.Stats().WaitCount - initial_wait_count
	commit_err := tx.Commit().Error
	workers.Wait()
	close(errors)
	if commit_err != nil {
		t.Fatal(commit_err)
	}
	if wait_count < int64(len(writes)) {
		t.Errorf("only %d of %d writers queued at the shared connection pool", wait_count, len(writes))
	}
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var run model.FlowRunRecord
	if err := db.First(&run, "id = ?", "run-1").Error; err != nil {
		t.Fatal(err)
	}
	if run.Status != model.FlowRunStatusWaiting || run.Checkpoint == "" || run.CurrentNode != "n1" {
		t.Fatalf("waiting checkpoint was not persisted: status=%s node=%s", run.Status, run.CurrentNode)
	}
	var log_count int64
	if err := db.Model(&model.FlowRunNodeLog{}).Where("run_id = ?", run.ID).Count(&log_count).Error; err != nil || log_count != 1 {
		t.Fatalf("node logs=%d, err=%v", log_count, err)
	}
	var resource model.DownloadResource
	if err := db.First(&resource).Error; err != nil {
		t.Fatal(err)
	}
	if resource.Downloaded != 123 || resource.Speed != 45 {
		t.Fatalf("download progress was not persisted: %+v", resource)
	}
}
