package services

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
)

func TestHandleChannelsFeedUpsert(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Account{}, &model.Content{}, &model.ContentAccount{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	service := NewChannelsUploadService(db, nil)

	// Create content_a + account_a
	contentA := &model.Content{
		PlatformId:  "wx_channels",
		ExternalId:  "object_a",
		ContentType: "video",
		Title:       "video A",
		ContentURL:  "https://example.com/a.mp4",
		URL:         "https://example.com/a.mp4",
		Timestamps:  model.Timestamps{CreatedAt: 1, UpdatedAt: 1},
	}
	accountA := &model.Account{
		PlatformId: "wx_channels",
		ExternalId: "finder_a",
		Username:   "finder_a",
		Nickname:   "account A",
		Timestamps: model.Timestamps{CreatedAt: 1, UpdatedAt: 1},
	}
	if _, err := service.HandleChannelsFeed(contentA, accountA); err != nil {
		t.Fatalf("handle feed a: %v", err)
	}

	// Create content_b + account_b
	contentB := &model.Content{
		PlatformId:  "wx_channels",
		ExternalId:  "object_b",
		ContentType: "video",
		Title:       "video B",
		ContentURL:  "https://example.com/b.mp4",
		URL:         "https://example.com/b.mp4",
		Timestamps:  model.Timestamps{CreatedAt: 2, UpdatedAt: 2},
	}
	accountB := &model.Account{
		PlatformId: "wx_channels",
		ExternalId: "finder_b",
		Username:   "finder_b",
		Nickname:   "account B",
		Timestamps: model.Timestamps{CreatedAt: 2, UpdatedAt: 2},
	}
	if _, err := service.HandleChannelsFeed(contentB, accountB); err != nil {
		t.Fatalf("handle feed b: %v", err)
	}

	// Verify: 2 accounts and 2 contents
	var accounts []model.Account
	if err := db.Order("external_id").Find(&accounts).Error; err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d: %#v", len(accounts), accounts)
	}

	var contents []model.Content
	if err := db.Order("external_id").Find(&contents).Error; err != nil {
		t.Fatalf("list contents: %v", err)
	}
	if len(contents) != 2 {
		t.Fatalf("expected 2 contents, got %d", len(contents))
	}

	// Verify: 2 content-account links (owner role)
	var links []model.ContentAccount
	if err := db.Find(&links).Error; err != nil {
		t.Fatalf("list content_account: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 content account links, got %d: %#v", len(links), links)
	}
}

func TestHandleChannelsFeed_NilParameters(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	service := NewChannelsUploadService(db, nil)

	// nil content
	account := &model.Account{
		PlatformId: "wx_channels",
		ExternalId: "test",
		Username:   "test",
		Nickname:   "test",
		Timestamps: model.Timestamps{CreatedAt: 1, UpdatedAt: 1},
	}
	_, err = service.HandleChannelsFeed(nil, account)
	if err == nil {
		t.Fatal("expected error for nil content")
	}

	// nil account
	content := &model.Content{
		PlatformId:  "wx_channels",
		ExternalId:  "test",
		ContentType: "video",
		Title:       "test",
		Timestamps:  model.Timestamps{CreatedAt: 1, UpdatedAt: 1},
	}
	_, err = service.HandleChannelsFeed(content, nil)
	if err == nil {
		t.Fatal("expected error for nil account")
	}
}

func TestHandleChannelsFeed_AccountUpdate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Account{}, &model.Content{}, &model.ContentAccount{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	service := NewChannelsUploadService(db, nil)

	// First insert
	content1 := &model.Content{
		PlatformId:  "wx_channels",
		ExternalId:  "object_1",
		ContentType: "video",
		Title:       "title v1",
		Timestamps:  model.Timestamps{CreatedAt: 1, UpdatedAt: 1},
	}
	account1 := &model.Account{
		PlatformId: "wx_channels",
		ExternalId: "updater",
		Username:   "updater",
		Nickname:   "old nickname",
		Timestamps: model.Timestamps{CreatedAt: 1, UpdatedAt: 1},
	}
	if _, err := service.HandleChannelsFeed(content1, account1); err != nil {
		t.Fatalf("first handle: %v", err)
	}

	// Update: same content id, updated account nickname
	content2 := &model.Content{
		PlatformId:  "wx_channels",
		ExternalId:  "object_1",
		ContentType: "video",
		Title:       "title v2",
		Timestamps:  model.Timestamps{CreatedAt: 2, UpdatedAt: 2},
	}
	account2 := &model.Account{
		PlatformId: "wx_channels",
		ExternalId: "updater",
		Username:   "updater",
		Nickname:   "new nickname",
		Timestamps: model.Timestamps{CreatedAt: 2, UpdatedAt: 2},
	}
	if _, err := service.HandleChannelsFeed(content2, account2); err != nil {
		t.Fatalf("second handle: %v", err)
	}

	// Verify account nickname was updated
	var acc model.Account
	if err := db.Where("external_id = ?", "updater").First(&acc).Error; err != nil {
		t.Fatalf("find account: %v", err)
	}
	if acc.Nickname != "new nickname" {
		t.Errorf("Nickname = %q, want \"new nickname\"", acc.Nickname)
	}
}

func TestHandleChannelsFeed_StaleOwnerLinkRemoval(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Account{}, &model.Content{}, &model.ContentAccount{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	service := NewChannelsUploadService(db, nil)

	// Create content + legitimate account
	content := &model.Content{
		PlatformId:  "wx_channels",
		ExternalId:  "object_x",
		ContentType: "video",
		Title:       "video X",
		Timestamps:  model.Timestamps{CreatedAt: 1, UpdatedAt: 1},
	}
	legitimate := &model.Account{
		PlatformId: "wx_channels",
		ExternalId: "real_owner",
		Username:   "real_owner",
		Nickname:   "Real Owner",
		Timestamps: model.Timestamps{CreatedAt: 1, UpdatedAt: 1},
	}
	if _, err := service.HandleChannelsFeed(content, legitimate); err != nil {
		t.Fatalf("first handle: %v", err)
	}

	// Manually create a stale owner link (wrong account)
	wrongAccount := model.Account{
		Id:         "wx_channels:wrong_owner",
		PlatformId: "wx_channels",
		ExternalId: "wrong_owner",
		Username:   "wrong_owner",
		Nickname:   "Wrong",
		Timestamps: model.Timestamps{CreatedAt: 2, UpdatedAt: 2},
	}
	if err := db.Create(&wrongAccount).Error; err != nil {
		t.Fatalf("create wrong account: %v", err)
	}
	if err := db.Create(&model.ContentAccount{
		ContentId: content.Id,
		AccountId: wrongAccount.Id,
		Role:      "owner",
		CreatedAt: 2,
	}).Error; err != nil {
		t.Fatalf("create wrong owner link: %v", err)
	}

	// Re-handle with the real account
	content2 := &model.Content{
		PlatformId:  "wx_channels",
		ExternalId:  "object_x",
		ContentType: "video",
		Title:       "video X updated",
		Timestamps:  model.Timestamps{CreatedAt: 3, UpdatedAt: 3},
	}
	if _, err := service.HandleChannelsFeed(content2, legitimate); err != nil {
		t.Fatalf("re-handle: %v", err)
	}

	// Verify stale link was removed, only 1 owner link remains
	var links []model.ContentAccount
	if err := db.Where("content_id = ?", content.Id).Find(&links).Error; err != nil {
		t.Fatalf("list links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d: %#v", len(links), links)
	}
	if links[0].AccountId != legitimate.Id {
		t.Fatalf("remaining link AccountId = %s, want %s", links[0].AccountId, legitimate.Id)
	}
	if links[0].Role != "owner" {
		t.Fatalf("remaining link Role = %q, want \"owner\"", links[0].Role)
	}
}
