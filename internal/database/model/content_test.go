package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestContentImageLivePhotoPersistence(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:content-image-live-photo?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&ContentImage{}); err != nil {
		t.Fatalf("migrate ContentImage: %v", err)
	}

	want := ContentImage{
		AlbumId:   "album-1",
		ImageType: ContentImageTypeLivePhoto,
		URL:       "https://example.test/poster.jpg",
		LivePhoto: &ContentImageLivePhoto{
			Vid:        "live-1",
			Type:       1,
			URL:        "https://example.test/live.mp4",
			FormatId:   0,
			Width:      2884,
			Height:     2160,
			Size:       7200103,
			DurationMs: 3090,
			Formats: []ContentImageLivePhotoFormat{{
				FormatId:   0,
				URL:        "https://example.test/live.mp4",
				Size:       7200103,
				DurationMs: 3090,
				Width:      2884,
				Height:     2160,
			}},
		},
	}
	if err := db.Create(&want).Error; err != nil {
		t.Fatalf("create ContentImage: %v", err)
	}

	var got ContentImage
	if err := db.First(&got, want.Id).Error; err != nil {
		t.Fatalf("load ContentImage: %v", err)
	}
	if got.LivePhoto == nil {
		t.Fatal("loaded LivePhoto is nil")
	}
	if got.LivePhoto.Vid != want.LivePhoto.Vid || got.LivePhoto.URL != want.LivePhoto.URL || got.LivePhoto.Size != want.LivePhoto.Size || len(got.LivePhoto.Formats) != 1 {
		t.Fatalf("loaded LivePhoto = %+v, want %+v", got.LivePhoto, want.LivePhoto)
	}
}
