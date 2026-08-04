package api

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/hermes"
)

func TestDBTaskStoreUpdateResourceOutput(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&model.DownloadResource{}); err != nil {
		t.Fatalf("migrate resource: %v", err)
	}
	resource := model.DownloadResource{TaskId: 9, Name: "before", Kind: "video", Type: "FILE", Size: 12}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatalf("create resource: %v", err)
	}
	store := &dbTaskStore{db: db}
	if err := store.UpdateResourceOutput(hermes.ResourceOutputUpdate{
		TaskID:       9,
		ResourceID:   resource.Id,
		ResourceName: "after.zip",
		ResourceKind: "application/zip",
		ResourceSize: 2048,
	}); err != nil {
		t.Fatalf("update resource output: %v", err)
	}

	var got model.DownloadResource
	if err := db.First(&got, resource.Id).Error; err != nil {
		t.Fatalf("load resource: %v", err)
	}
	if got.Name != "after.zip" || got.Kind != "application/zip" || got.Size != 2048 {
		t.Fatalf("persisted resource = %#v", got)
	}
}
