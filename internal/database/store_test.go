package database

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/hermes"
)

func TestRecoverInterruptedTasksAndPersistOutputState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	connection.SetMaxOpenConns(1)
	t.Cleanup(func() { connection.Close() })
	if err := db.AutoMigrate(&model.DownloadTask{}, &model.DownloadResource{}, &model.DownloadConnection{}); err != nil {
		t.Fatal(err)
	}
	for _, status := range []int{model.TaskStatusWaiting, model.TaskStatusPreparing, model.TaskStatusDownloading, model.TaskStatusMerging, model.TaskStatusFinished, model.TaskStatusFailed} {
		task := model.DownloadTask{Name: "task", PlatformId: "audit", Status: status, Timestamps: model.Timestamps{CreatedAt: 1}}
		if err := db.Create(&task).Error; err != nil {
			t.Fatal(err)
		}
	}
	resource := model.DownloadResource{Name: "before", Speed: 10, Timestamps: model.Timestamps{CreatedAt: 1}}
	active := model.DownloadConnection{Status: 1, Speed: 10, Timestamps: model.Timestamps{CreatedAt: 1}}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&active).Error; err != nil {
		t.Fatal(err)
	}
	store := NewDBTaskStore(db, nil)
	for i := 0; i < 2; i++ {
		if err := store.RecoverInterruptedTasks(); err != nil {
			t.Fatal(err)
		}
	}
	var tasks []model.DownloadTask
	if err := db.Order("id").Find(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	expected := []int{model.TaskStatusWaiting, model.TaskStatusPaused, model.TaskStatusPaused, model.TaskStatusPaused, model.TaskStatusFinished, model.TaskStatusFailed}
	for index, task := range tasks {
		if task.Status != expected[index] {
			t.Fatalf("task %d status=%d want=%d", task.Id, task.Status, expected[index])
		}
	}
	if err := db.First(&active, active.Id).Error; err != nil {
		t.Fatal(err)
	}
	if active.Status != 0 || active.Speed != 0 {
		t.Fatalf("connection remains active: %+v", active)
	}
	if err := store.UpdateResourceOutput(hermes.ResourceOutputUpdate{ResourceID: resource.Id, ResourceName: "after.mp4", ResourceExtra: map[string]string{"_hermes_finalized": "1"}}); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&resource, resource.Id).Error; err != nil {
		t.Fatal(err)
	}
	if resource.Name != "after.mp4" || parseExtra(resource.Extra)["_hermes_finalized"] != "1" || resource.Speed != 0 {
		t.Fatalf("output metadata was not saved: %+v", resource)
	}
}
