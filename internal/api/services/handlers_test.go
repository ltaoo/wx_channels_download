package services

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	apitypes "wx_channel/internal/api/types"
	"wx_channel/internal/database/model"
)

func TestHandleChannelsFeedUsesBrowseHistoryAccountIdentity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Account{}, &model.Video{}, &model.VideoAccount{}, &model.BrowseHistory{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	histories := []model.BrowseHistory{
		{
			PlatformId:        "wx_channels",
			VisitedTimes:      1,
			AccountExternalId: "finder_a",
			AccountUsername:   "finder_a",
			AccountNickname:   "account A",
			ContentExternalId: "object_a",
			Timestamps:        model.Timestamps{CreatedAt: 1, UpdatedAt: 1},
		},
		{
			PlatformId:        "wx_channels",
			VisitedTimes:      1,
			AccountExternalId: "finder_b",
			AccountUsername:   "finder_b",
			AccountNickname:   "account B",
			ContentExternalId: "object_b",
			Timestamps:        model.Timestamps{CreatedAt: 2, UpdatedAt: 2},
		},
	}
	if err := db.Create(&histories).Error; err != nil {
		t.Fatalf("create browse history: %v", err)
	}

	service := NewChannelsUploadService(db, nil)
	for _, feed := range []apitypes.ChannelsFeedProfile{
		{
			ObjectId: "object_a",
			URL:      "https://example.com/a.mp4",
			Title:    "video A",
			Contact:  apitypes.ChannelsFeedAccount{Username: "same_wrong_username", Nickname: "wrong"},
		},
		{
			ObjectId: "object_b",
			URL:      "https://example.com/b.mp4",
			Title:    "video B",
			Contact:  apitypes.ChannelsFeedAccount{Username: "same_wrong_username", Nickname: "wrong"},
		},
	} {
		if _, err := service.HandleChannelsFeed(&feed); err != nil {
			t.Fatalf("handle feed %s: %v", feed.ObjectId, err)
		}
	}

	var accounts []model.Account
	if err := db.Order("external_id").Find(&accounts).Error; err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d: %#v", len(accounts), accounts)
	}
	if accounts[0].ExternalId != "finder_a" || accounts[0].Nickname != "account A" {
		t.Fatalf("unexpected first account: %#v", accounts[0])
	}
	if accounts[1].ExternalId != "finder_b" || accounts[1].Nickname != "account B" {
		t.Fatalf("unexpected second account: %#v", accounts[1])
	}

	var links []model.VideoAccount
	if err := db.Find(&links).Error; err != nil {
		t.Fatalf("list video_account: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 video account links, got %d: %#v", len(links), links)
	}

	wrong := model.Account{
		PlatformId: "wx_channels",
		ExternalId: "same_wrong_username",
		Username:   "same_wrong_username",
		Nickname:   "wrong",
		Timestamps: model.Timestamps{CreatedAt: 3, UpdatedAt: 3},
	}
	if err := db.Create(&wrong).Error; err != nil {
		t.Fatalf("create wrong account: %v", err)
	}
	var videoB model.Video
	if err := db.Where("external_id1 = ?", "object_b").First(&videoB).Error; err != nil {
		t.Fatalf("load video b: %v", err)
	}
	if err := db.Create(&model.VideoAccount{VideoId: videoB.Id, AccountId: wrong.Id, Role: "owner"}).Error; err != nil {
		t.Fatalf("create wrong owner link: %v", err)
	}
	feed := apitypes.ChannelsFeedProfile{
		ObjectId: "object_b",
		URL:      "https://example.com/b.mp4",
		Title:    "video B",
		Contact:  apitypes.ChannelsFeedAccount{Username: "same_wrong_username", Nickname: "wrong"},
	}
	if _, err := service.HandleChannelsFeed(&feed); err != nil {
		t.Fatalf("re-handle feed b: %v", err)
	}
	links = nil
	if err := db.Find(&links).Error; err != nil {
		t.Fatalf("list repaired video_account: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected stale owner link to be removed, got %d links: %#v", len(links), links)
	}
}
