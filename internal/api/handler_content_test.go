package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/services"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/testui/assert"
	"wx_channel/pkg/testui/require"
)

func TestHandleCompatContentListUsesPageParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Content{},
		&model.Account{},
		&model.ContentAccount{},
		&model.DownloadTaskV1{},
	))

	publishFirst := int64(1000)
	publishSecond := int64(2000)
	require.NoError(t, db.Create(&model.Content{
		Id:          "test:first",
		PlatformId:  "test",
		Type: "video",
		ExternalId:  "first",
		Title:       "第一页之外",
		PublishTime: &publishFirst,
		Timestamps:  model.Timestamps{CreatedAt: 1000, UpdatedAt: 1000},
	}).Error)
	require.NoError(t, db.Create(&model.Content{
		Id:          "test:second",
		PlatformId:  "test",
		Type: "video",
		ExternalId:  "second",
		Title:       "第一页",
		PublishTime: &publishSecond,
		Timestamps:  model.Timestamps{CreatedAt: 2000, UpdatedAt: 2000},
	}).Error)
	require.NoError(t, db.Create(&model.Account{
		Id:            "test:author",
		PlatformId:    "test",
		ExternalId:    "author",
		Alias:         "test-author",
		Nickname:      "测试作者",
		FollowerCount: 88,
		Timestamps:    model.Timestamps{CreatedAt: 900, UpdatedAt: 950},
	}).Error)
	require.NoError(t, db.Create(&model.ContentAccount{
		ContentId: "test:first",
		AccountId: "test:author",
		Role:      "author",
		CreatedAt: 1000,
	}).Error)
	firstContentID := "test:first"
	require.NoError(t, db.Create(&model.DownloadTaskV1{
		Id:         51,
		ContentId:  &firstContentID,
		RootTaskID: 51,
		Name:       "first.mp4",
		PlatformId: "test",
		Status:     model.TaskStatusDownloading,
		SourceURL:  "https://example.com/first",
		Timestamps: model.Timestamps{CreatedAt: 1100, UpdatedAt: 1200},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/content/list?page=2&page_size=1", nil)
	client := &APIClient{contentService: services.NewContentService(db)}
	client.handleCompatContentList(ctx)

	var response struct {
		Code int                        `json:"code"`
		Data services.ContentListResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Zero(t, response.Code)
	assert.Equal(t, int64(2), response.Data.Total)
	assert.Equal(t, 2, response.Data.Page)
	assert.Equal(t, 1, response.Data.PageSize)
	require.Len(t, response.Data.List, 1)
	assert.Equal(t, "test:first", response.Data.List[0].ID)
	require.Len(t, response.Data.List[0].Accounts, 1)
	assert.Equal(t, "test:author", response.Data.List[0].Accounts[0].ID)
	assert.Equal(t, "test-author", response.Data.List[0].Accounts[0].Alias)
	assert.Equal(t, int64(88), response.Data.List[0].Accounts[0].FollowerCount)
	assert.Equal(t, "author", response.Data.List[0].Accounts[0].Role)
	require.Len(t, response.Data.List[0].DownloadTasks, 1)
	assert.Equal(t, 51, response.Data.List[0].DownloadTasks[0].ID)
	assert.Equal(t, "first.mp4", response.Data.List[0].DownloadTasks[0].Name)
}
