package api

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/douyin"
)

func TestUpsertDouyinContentCreatesDouyinRecords(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Account{}, &model.Content{}, &model.ContentAccount{}); err != nil {
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

	content, err := client.upsertDouyinContent(info, "https://v.douyin.com/share/")
	if err != nil {
		t.Fatalf("upsertDouyinContent() error = %v", err)
	}
	if content.PlatformId != "douyin" || content.ExternalId != "video_1" || content.ContentType != "video" {
		t.Fatalf("unexpected content: %#v", content)
	}

	var account model.Account
	if err := db.Where("platform_id = ? AND external_id = ?", "douyin", "author_1").First(&account).Error; err != nil {
		t.Fatalf("load douyin account: %v", err)
	}
	if account.Nickname != "作者" || account.Username != "douyin_author" {
		t.Fatalf("unexpected account: %#v", account)
	}

	var link model.ContentAccount
	if err := db.Where("content_id = ? AND account_id = ?", content.Id, account.Id).First(&link).Error; err != nil {
		t.Fatalf("load content account link: %v", err)
	}
	if link.Role != "owner" {
		t.Fatalf("unexpected link: %#v", link)
	}

	info.Title = "抖音视频更新"
	info.AuthorNickname = "作者更新"
	content, err = client.upsertDouyinContent(info, "https://v.douyin.com/share/")
	if err != nil {
		t.Fatalf("second upsertDouyinContent() error = %v", err)
	}
	if content.Title != "抖音视频更新" {
		t.Fatalf("expected updated content title, got %q", content.Title)
	}
	if err := db.Where("platform_id = ? AND external_id = ?", "douyin", "author_1").First(&account).Error; err != nil {
		t.Fatalf("reload douyin account: %v", err)
	}
	if account.Nickname != "作者更新" {
		t.Fatalf("expected updated account nickname, got %q", account.Nickname)
	}
}
