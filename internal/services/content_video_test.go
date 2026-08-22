package services

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
)

func TestSaveContentVideoPersistsAllVariants(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.ContentVideo{},
		&model.ContentAsset{},
		&model.ContentVideoVariant{},
	); err != nil {
		t.Fatal(err)
	}
	content_video := &model.ContentVideo{
		Id:     "wxmp:wxv_video",
		Format: "mp4",
		URL:    "https://mpvideo.qpic.cn/high.mp4",
		Variants: []model.ContentVideoVariant{
			{
				VideoId:    "wxmp:wxv_video",
				VariantKey: "wxv_video:format:10002",
				Spec:       "10002",
				Format:     "mp4",
				HasVideo:   1,
				HasAudio:   1,
				IsDefault:  1,
				URL:        "https://mpvideo.qpic.cn/high.mp4",
			},
			{
				VideoId:    "wxmp:wxv_video",
				VariantKey: "wxv_video:format:10004",
				Spec:       "10004",
				Format:     "mp4",
				HasVideo:   1,
				HasAudio:   1,
				URL:        "https://mpvideo.qpic.cn/low.mp4",
			},
		},
	}

	if err := save_content_video(db, content_video); err != nil {
		t.Fatal(err)
	}
	var variants []model.ContentVideoVariant
	if err := db.Where("video_id = ?", content_video.Id).Order("variant_key").Find(&variants).Error; err != nil {
		t.Fatal(err)
	}
	if len(variants) != 2 {
		t.Fatalf("persisted variant count = %d, want 2", len(variants))
	}
	default_count := 0
	for variant_index := range variants {
		if variants[variant_index].IsDefault != 0 {
			default_count++
		}
	}
	if default_count != 1 {
		t.Errorf("default variant count = %d, want 1", default_count)
	}
}
