package database

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	velodatabase "github.com/ltaoo/velo/database"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
)

func TestInitialMigrationCreatesDownloadTaskPlatformColumns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "migration.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	migrator := velodatabase.NewMigrator(&velodatabase.DBConfig{
		Type: velodatabase.DBTypeSQLite,
		Path: dbPath,
	}, &Migrations)
	if err := migrator.MigrateTo(db, 1); err != nil {
		t.Fatalf("apply initial migration: %v", err)
	}

	contentID := "wxchannels:content-1"
	task := model.DownloadTask{
		ContentId:  &contentID,
		Name:       "测试视频",
		PlatformId: "wxchannels",
		UniqueID:   "content-1",
		Status:     model.TaskStatusWaiting,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create download task: %v", err)
	}

	var persisted model.DownloadTask
	if err := db.First(&persisted, task.Id).Error; err != nil {
		t.Fatalf("load download task: %v", err)
	}
	if persisted.PlatformId != "wxchannels" {
		t.Fatalf("platform id = %q, want %q", persisted.PlatformId, "wxchannels")
	}
	if persisted.UniqueID != "content-1" {
		t.Fatalf("unique id = %q, want %q", persisted.UniqueID, "content-1")
	}

	video := model.ContentVideo{
		Id:          contentID,
		Duration:    9,
		Width:       1080,
		Height:      2128,
		Size:        9613487,
		PlayTimes:   12,
	}
	if err := db.Save(&video).Error; err != nil {
		t.Fatalf("save content video: %v", err)
	}

	var persistedVideo model.ContentVideo
	if err := db.First(&persistedVideo, "id = ?", contentID).Error; err != nil {
		t.Fatalf("load content video: %v", err)
	}
	if persistedVideo.Size != 9613487 {
		t.Fatalf("size = %d, want %d", persistedVideo.Size, int64(9613487))
	}
	if persistedVideo.PlayTimes != 12 {
		t.Fatalf("play times = %d, want %d", persistedVideo.PlayTimes, int64(12))
	}

}
