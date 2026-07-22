package services

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
)

func TestBrowseHistoryListOrdersRepeatedVisitsByLatestUpdate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.BrowseHistory{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	service := NewBrowseService(db)
	if err := (&model.BrowseHistory{
		PlatformId:        "wx_channels",
		VisitedTimes:      1,
		ContentExternalId: "old_then_latest",
		ContentTitle:      "old then latest",
		Timestamps:        model.Timestamps{CreatedAt: 1000, UpdatedAt: 1000},
	}).Upsert(db); err != nil {
		t.Fatalf("create old record: %v", err)
	}
	if err := (&model.BrowseHistory{
		PlatformId:        "wx_channels",
		VisitedTimes:      1,
		ContentExternalId: "middle",
		ContentTitle:      "middle",
		Timestamps:        model.Timestamps{CreatedAt: 2000, UpdatedAt: 2000},
	}).Upsert(db); err != nil {
		t.Fatalf("create middle record: %v", err)
	}
	if err := (&model.BrowseHistory{
		PlatformId:        "wx_channels",
		VisitedTimes:      1,
		ContentExternalId: "old_then_latest",
		ContentTitle:      "old then latest again",
		Timestamps:        model.Timestamps{CreatedAt: 3000, UpdatedAt: 3000},
	}).Upsert(db); err != nil {
		t.Fatalf("revisit old record: %v", err)
	}

	histories, err := service.List("wx_channels", nil)
	if err != nil {
		t.Fatalf("list browse histories: %v", err)
	}
	if len(histories) != 2 {
		t.Fatalf("expected 2 browse histories, got %d: %#v", len(histories), histories)
	}
	if histories[0].ContentExternalId != "old_then_latest" {
		t.Fatalf("expected revisited record first, got %#v", histories[0])
	}
	if histories[0].VisitedTimes != 2 {
		t.Fatalf("expected visited times to increment, got %d", histories[0].VisitedTimes)
	}
	if histories[0].CreatedAt != 1000 {
		t.Fatalf("expected original created_at to remain 1000, got %d", histories[0].CreatedAt)
	}
	if histories[0].UpdatedAt != 3000 {
		t.Fatalf("expected updated_at to be latest visit time 3000, got %d", histories[0].UpdatedAt)
	}
}

func TestBrowseHistoryListPlatforms(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.BrowseHistory{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	for _, history := range []model.BrowseHistory{
		{PlatformId: "wx_channels", VisitedTimes: 1, ContentExternalId: "video_1", Timestamps: model.Timestamps{CreatedAt: 1000, UpdatedAt: 1000}},
		{PlatformId: "wxmp", VisitedTimes: 1, ContentExternalId: "article_1", Timestamps: model.Timestamps{CreatedAt: 2000, UpdatedAt: 2000}},
		{PlatformId: "douyin", VisitedTimes: 1, ContentExternalId: "douyin_1", Timestamps: model.Timestamps{CreatedAt: 3000, UpdatedAt: 3000}},
	} {
		if err := history.Upsert(db); err != nil {
			t.Fatalf("create history: %v", err)
		}
	}

	histories, err := NewBrowseService(db).ListPlatforms([]string{"wx_channels", "wxmp"}, nil)
	if err != nil {
		t.Fatalf("list platforms: %v", err)
	}
	if len(histories) != 2 {
		t.Fatalf("expected 2 histories, got %d: %#v", len(histories), histories)
	}
	if histories[0].ContentExternalId != "article_1" || histories[1].ContentExternalId != "video_1" {
		t.Fatalf("unexpected histories: %#v", histories)
	}
}
