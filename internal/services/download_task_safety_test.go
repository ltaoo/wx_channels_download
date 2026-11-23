package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/hermes"
)

func new_task_safety_db(t *testing.T) *gorm.DB {
	t.Helper()
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
	if err := db.AutoMigrate(&model.DownloadTask{}, &model.DownloadResource{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestDeleteTaskUsesOnlyItsResourceDirectory(t *testing.T) {
	for _, location := range []string{"resource", "task", "relative resource"} {
		t.Run(location, func(t *testing.T) {
			root := t.TempDir()
			custom := filepath.Join(root, "custom")
			if err := os.Mkdir(custom, 0700); err != nil {
				t.Fatal(err)
			}
			unrelated := filepath.Join(root, "video.mp4")
			own := filepath.Join(custom, "video.mp4")
			for _, path := range []string{unrelated, own, own + ".part"} {
				if err := os.WriteFile(path, []byte("bytes"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			task := model.DownloadTask{Id: 1, Status: model.TaskStatusFinished}
			resource := model.DownloadResource{Id: 1, Name: "video.mp4", UniqueID: "raw", DownloadDir: custom}
			if location == "task" {
				resource.DownloadDir = ""
				task.ConfigJSON = `{"download_dir":"custom"}`
			}
			if location == "relative resource" {
				resource.DownloadDir = "custom"
			}
			service := NewDownloadTaskService(nil, nil, nil, nil, root, root, nil)
			if err := service.delete_download_task_local_files(task, []model.DownloadResource{resource}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(unrelated); err != nil {
				t.Fatalf("unrelated file removed: %v", err)
			}
			for _, path := range []string{own, own + ".part"} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("owned file remains: %s %v", path, err)
				}
			}
		})
	}
}

func TestDeleteTaskPreservesFileReferencedByAnotherTask(t *testing.T) {
	db := new_task_safety_db(t)
	root := t.TempDir()
	first := model.DownloadTask{Name: "first", PlatformId: "audit", Status: model.TaskStatusFinished, Timestamps: model.Timestamps{CreatedAt: 1}}
	second := model.DownloadTask{Name: "second", PlatformId: "audit", Status: model.TaskStatusFinished, Timestamps: model.Timestamps{CreatedAt: 1}}
	for _, task := range []*model.DownloadTask{&first, &second} {
		if err := db.Create(task).Error; err != nil {
			t.Fatal(err)
		}
	}
	resources := []model.DownloadResource{
		{TaskId: &first.Id, Name: "shared.mp4", UniqueID: "raw", DownloadDir: root, Timestamps: model.Timestamps{CreatedAt: 1}},
		{TaskId: &second.Id, Name: "shared.mp4", UniqueID: "raw", DownloadDir: root, Timestamps: model.Timestamps{CreatedAt: 1}},
	}
	if err := db.Create(&resources).Error; err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "shared.mp4")
	if err := os.WriteFile(path, []byte("shared"), 0600); err != nil {
		t.Fatal(err)
	}
	service := NewDownloadTaskService(db, nil, nil, nil, root, root, nil)
	if err := service.delete_download_task_local_files(first, resources[:1]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("shared file was removed: %v", err)
	}
}

func TestDeleteDuplicateTaskDoesNotDeleteOriginalTransfer(t *testing.T) {
	root := t.TempDir()
	task := model.DownloadTask{Id: 1, ConfigJSON: `{"duplicate":true}`}
	resource := model.DownloadResource{Id: 2, Name: "title.mp4", UniqueID: "raw", DownloadDir: root}
	original := filepath.Join(root, "raw.part")
	duplicate := filepath.Join(root, hermes.ResourceDownloadName(task.Id, resource.Id, resource.UniqueID, true)+".part")
	for _, path := range []string{original, duplicate} {
		if err := os.WriteFile(path, []byte("partial"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	service := NewDownloadTaskService(nil, nil, nil, nil, root, root, nil)
	if err := service.delete_download_task_local_files(task, []model.DownloadResource{resource}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(original); err != nil {
		t.Fatal("original transfer was removed")
	}
	if _, err := os.Stat(duplicate); !os.IsNotExist(err) {
		t.Fatal("duplicate transfer was not removed")
	}
}

func TestPauseOrphanedTaskPersistsItsState(t *testing.T) {
	db := new_task_safety_db(t)
	task := model.DownloadTask{Name: "orphaned", PlatformId: "audit", Status: model.TaskStatusDownloading, Timestamps: model.Timestamps{CreatedAt: 1}}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	service := NewDownloadTaskService(db, nil, hermes.New(hermes.HermesNewConfig{}), nil, t.TempDir(), t.TempDir(), nil)
	paused, _, err := service.PauseTask(task.Id)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.First(&task, task.Id).Error; err != nil {
		t.Fatal(err)
	}
	if paused.Status != model.TaskStatusPaused || task.Status != model.TaskStatusPaused {
		t.Fatalf("response status=%d persisted status=%d", paused.Status, task.Status)
	}
}
