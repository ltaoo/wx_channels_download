package services

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/testui/assert"
	"wx_channel/pkg/testui/require"
)

func TestContentServiceListContentsPaginatesAndLoadsAccounts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Content{},
		&model.Account{},
		&model.ContentAccount{},
		&model.DownloadTask{},
	))

	publishOld := int64(1000)
	publishNew := int64(2000)
	require.NoError(t, db.Create(&model.Account{
		Id:            "wxchannels:author-1",
		PlatformId:    "wxchannels",
		ExternalId:    "author-1",
		Alias:         "author_alias",
		Nickname:      "作者一",
		Signature:     "作者签名",
		AvatarURL:     "https://example.com/avatar.jpg",
		ProfileURL:    "https://example.com/author-1",
		IsListen:      1,
		FollowerCount: 1234,
		Timestamps:    model.Timestamps{CreatedAt: 100, UpdatedAt: 100},
	}).Error)
	require.NoError(t, db.Create(&model.Account{
		Id:         "wxchannels:editor-2",
		PlatformId: "wxchannels",
		ExternalId: "editor-2",
		Nickname:   "编辑二",
		Timestamps: model.Timestamps{CreatedAt: 200, UpdatedAt: 200},
	}).Error)
	deletedAccountAt := int64(300)
	require.NoError(t, db.Create(&model.Account{
		Id:         "wxchannels:deleted-3",
		PlatformId: "wxchannels",
		ExternalId: "deleted-3",
		Nickname:   "已删除账号",
		Timestamps: model.Timestamps{CreatedAt: 300, UpdatedAt: 300, DeletedAt: &deletedAccountAt},
	}).Error)
	require.NoError(t, db.Create(&model.Content{
		Id:          "wxchannels:old-video",
		PlatformId:  "wxchannels",
		Type: "video",
		ExternalId:  "old-video",
		Title:       "较早的视频",
		PublishTime: &publishOld,
		Timestamps:  model.Timestamps{CreatedAt: 1000, UpdatedAt: 1000},
	}).Error)
	require.NoError(t, db.Create(&model.Content{
		Id:          "wxchannels:new-video",
		PlatformId:  "wxchannels",
		Type: "video",
		ExternalId:  "new-video",
		Title:       "较新的视频",
		PublishTime: &publishNew,
		Timestamps:  model.Timestamps{CreatedAt: 2000, UpdatedAt: 2000},
	}).Error)
	require.NoError(t, db.Create(&model.ContentAccount{
		ContentId: "wxchannels:old-video",
		AccountId: "wxchannels:author-1",
		Role:      "author",
		CreatedAt: 1000,
	}).Error)
	require.NoError(t, db.Create(&model.ContentAccount{
		ContentId: "wxchannels:old-video",
		AccountId: "wxchannels:editor-2",
		Role:      "editor",
		CreatedAt: 1001,
	}).Error)
	require.NoError(t, db.Create(&model.ContentAccount{
		ContentId: "wxchannels:old-video",
		AccountId: "wxchannels:deleted-3",
		Role:      "author",
		CreatedAt: 1002,
	}).Error)
	oldContentID := "wxchannels:old-video"
	require.NoError(t, db.Create(&model.DownloadTask{
		Id:           101,
		ContentId:    &oldContentID,
		RootTaskID:   101,
		RelationType: model.TaskRelationDiscovered,
		Name:         "较早的视频.mp4",
		PlatformId:   "wxchannels",
		Status:       model.TaskStatusFinished,
		SourceURL:    "https://example.com/old-video",
		CoverURL:     "https://example.com/old-cover.jpg",
		CoverWidth:   "1080",
		CoverHeight:  "1440",
		Timestamps:   model.Timestamps{CreatedAt: 1100, UpdatedAt: 1200},
	}).Error)
	deletedAt := int64(1300)
	require.NoError(t, db.Create(&model.DownloadTask{
		Id:         102,
		ContentId:  &oldContentID,
		RootTaskID: 102,
		Name:       "已删除任务.mp4",
		PlatformId: "wxchannels",
		Status:     model.TaskStatusCancelled,
		Timestamps: model.Timestamps{CreatedAt: 1200, UpdatedAt: 1300, DeletedAt: &deletedAt},
	}).Error)

	pageResult, err := NewContentService(db).ListContents(ContentListOptions{
		Page:     2,
		PageSize: 1,
	})
	require.NoError(t, err)

	assert.Equal(t, int64(2), pageResult.Total)
	assert.Equal(t, 2, pageResult.Page)
	assert.Equal(t, 1, pageResult.PageSize)
	require.Len(t, pageResult.List, 1)
	assert.Equal(t, "wxchannels:old-video", pageResult.List[0].ID)
	assert.Equal(t, "old-video", pageResult.List[0].ExternalID1)
	assert.Equal(t, int64(1000), pageResult.List[0].PublishTime)
	require.Len(t, pageResult.List[0].Accounts, 2)
	assert.Equal(t, "wxchannels:author-1", pageResult.List[0].Accounts[0].ID)
	assert.Equal(t, "author_alias", pageResult.List[0].Accounts[0].Alias)
	assert.Equal(t, "作者签名", pageResult.List[0].Accounts[0].Signature)
	assert.Equal(t, int64(1234), pageResult.List[0].Accounts[0].FollowerCount)
	assert.Equal(t, "author", pageResult.List[0].Accounts[0].Role)
	assert.Equal(t, "wxchannels:editor-2", pageResult.List[0].Accounts[1].ID)
	assert.Equal(t, "editor", pageResult.List[0].Accounts[1].Role)
	require.Len(t, pageResult.List[0].DownloadTasks, 1)
	assert.Equal(t, 101, pageResult.List[0].DownloadTasks[0].ID)
	assert.Equal(t, "wxchannels:old-video", *pageResult.List[0].DownloadTasks[0].ContentID)
	assert.Equal(t, "较早的视频.mp4", pageResult.List[0].DownloadTasks[0].Name)
	assert.Equal(t, model.TaskStatusFinished, pageResult.List[0].DownloadTasks[0].Status)
	assert.Equal(t, "https://example.com/old-cover.jpg", pageResult.List[0].DownloadTasks[0].CoverURL)
}

func TestContentServiceUpsertAccountAndLinkContentUsesPersistedAccountID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Content{},
		&model.Account{},
		&model.ContentAccount{},
	))

	require.NoError(t, db.Create(&model.Content{
		Id:          "test:content-1",
		PlatformId:  "test",
		Type: "video",
		ExternalId:  "content-1",
		Title:       "测试内容",
		Timestamps:  model.Timestamps{CreatedAt: 1000, UpdatedAt: 1000},
	}).Error)
	require.NoError(t, db.Create(&model.Account{
		Id:         "test:canonical-account",
		PlatformId: "test",
		ExternalId: "author-1",
		Nickname:   "旧昵称",
		Timestamps: model.Timestamps{CreatedAt: 500, UpdatedAt: 500},
	}).Error)

	persisted, err := NewContentService(db).UpsertAccountAndLinkContent(
		"test:content-1",
		&model.Account{
			Id:            "test:incoming-account",
			PlatformId:    "test",
			ExternalId:    "author-1",
			Nickname:      "新昵称",
			FollowerCount: 88,
		},
		"author",
		1100,
	)
	require.NoError(t, err)

	assert.Equal(t, "test:canonical-account", persisted.Id)
	assert.Equal(t, "新昵称", persisted.Nickname)
	assert.Equal(t, int64(88), persisted.FollowerCount)

	var accounts []model.Account
	require.NoError(t, db.Find(&accounts).Error)
	require.Len(t, accounts, 1)

	var associations []model.ContentAccount
	require.NoError(t, db.Find(&associations).Error)
	require.Len(t, associations, 1)
	assert.Equal(t, "test:content-1", associations[0].ContentId)
	assert.Equal(t, "test:canonical-account", associations[0].AccountId)
	assert.Equal(t, "author", associations[0].Role)
	assert.Equal(t, int64(1100), associations[0].CreatedAt)
}
