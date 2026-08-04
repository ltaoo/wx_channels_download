package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestDownloadTaskV1AllowsMultipleTasksForContent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&Content{}, &DownloadTaskV1{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	content := Content{
		Id:          "wxchannels:content-1",
		PlatformId:  "wxchannels",
		Type: "video",
		ExternalId:  "content-1",
	}
	if err := db.Create(&content).Error; err != nil {
		t.Fatalf("create content: %v", err)
	}

	contentID := "wxchannels:content-1"
	tasks := []DownloadTaskV1{
		{ContentId: &contentID, Name: "video", PlatformId: "wxchannels"},
		{ContentId: &contentID, Name: "audio", PlatformId: "wxchannels"},
	}
	if err := db.Create(&tasks).Error; err != nil {
		t.Fatalf("create download tasks: %v", err)
	}

	var count int64
	if err := db.Model(&DownloadTaskV1{}).Where("content_id = ?", "wxchannels:content-1").Count(&count).Error; err != nil {
		t.Fatalf("count download tasks: %v", err)
	}
	if count != 2 {
		t.Fatalf("download task count = %d, want 2", count)
	}
}
