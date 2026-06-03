package api

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/douyin"
)

func TestUpsertDouyinVideoCreatesDouyinRecords(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Account{}, &model.Video{}, &model.VideoAccount{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	client := &APIClient{db: db}
	info := &douyin.VideoInfo{
		URL:             "https://example.com/video.mp4",
		Title:           "抖音视频",
		VideoID:         "video_1",
		CoverURL:        "https://example.com/cover.jpg",
		AuthorID:        "author_1",
		AuthorSecID:     "sec_author_1",
		AuthorUsername:  "douyin_author",
		AuthorNickname:  "作者",
		AuthorAvatarURL: "https://example.com/avatar.jpg",
	}

	video, err := client.upsertDouyinVideo(info, "https://v.douyin.com/share/")
	if err != nil {
		t.Fatalf("upsertDouyinVideo() error = %v", err)
	}
	if video.PlatformId != "douyin" || video.ExternalId1 != "video_1" {
		t.Fatalf("unexpected video: %#v", video)
	}

	var account model.Account
	if err := db.Where("platform_id = ? AND external_id = ?", "douyin", "author_1").First(&account).Error; err != nil {
		t.Fatalf("load douyin account: %v", err)
	}
	if account.Nickname != "作者" || account.Username != "douyin_author" {
		t.Fatalf("unexpected account: %#v", account)
	}

	var link model.VideoAccount
	if err := db.Where("video_id = ? AND account_id = ?", video.Id, account.Id).First(&link).Error; err != nil {
		t.Fatalf("load video account link: %v", err)
	}
	if link.Role != "owner" {
		t.Fatalf("unexpected link: %#v", link)
	}

	info.Title = "抖音视频更新"
	info.AuthorNickname = "作者更新"
	video, err = client.upsertDouyinVideo(info, "https://v.douyin.com/share/")
	if err != nil {
		t.Fatalf("second upsertDouyinVideo() error = %v", err)
	}
	if video.Title != "抖音视频更新" {
		t.Fatalf("expected updated video title, got %q", video.Title)
	}
	if err := db.Where("platform_id = ? AND external_id = ?", "douyin", "author_1").First(&account).Error; err != nil {
		t.Fatalf("reload douyin account: %v", err)
	}
	if account.Nickname != "作者更新" {
		t.Fatalf("expected updated account nickname, got %q", account.Nickname)
	}
}
