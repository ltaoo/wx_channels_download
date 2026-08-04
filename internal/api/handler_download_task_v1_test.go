package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/internal/download/registry"
	"wx_channel/internal/download/types"
	"wx_channel/internal/services"
	"wx_channel/pkg/hermes"
	"wx_channel/pkg/hermes/protocol"
	"wx_channel/pkg/testui/assert"
	"wx_channel/pkg/testui/require"
)

type savePathTestPlatformHandler struct {
	config      json.RawMessage
	endpointURL string
}

var (
	savePathTestHandler     = &savePathTestPlatformHandler{}
	registerSavePathHandler sync.Once
)

type contentAccountTestPlatformHandler struct{}

var registerContentAccountTestHandler sync.Once

func (h *contentAccountTestPlatformHandler) PlatformID() string {
	return "api_test_content_account"
}

func (h *contentAccountTestPlatformHandler) BuildDownloadTask(_ json.RawMessage, _ json.RawMessage) (*types.DownloadTaskResult, error) {
	return &types.DownloadTaskResult{
		Task: &model.DownloadTaskV1{
			Name:       "content-account.bin",
			UniqueID:   "api_test_content_account_file",
			PlatformId: h.PlatformID(),
		},
		Content: &model.Content{
			Id:          "api_test_content_account:content-1",
			PlatformId:  h.PlatformID(),
			Type: "video",
			ExternalId:  "content-1",
			Title:       "关联账号测试",
		},
		Account: &model.Account{
			Id:         "api_test_content_account:incoming-account",
			PlatformId: h.PlatformID(),
			ExternalId: "author-1",
			Nickname:   "更新后的作者",
		},
		Resources: []*types.ResourceInfo{{
			DownloadResource: model.DownloadResource{
				Name:     "content-account.bin",
				Kind:     "video",
				UniqueID: "api_test_content_account_resource",
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "HTTP",
				URL:      "https://example.com/content-account.bin",
				Enabled:  1,
			}},
		}},
	}, nil
}

func (h *savePathTestPlatformHandler) PlatformID() string {
	return "api_test_save_path"
}

func (h *savePathTestPlatformHandler) BuildDownloadTask(_ json.RawMessage, configRaw json.RawMessage) (*types.DownloadTaskResult, error) {
	h.config = configRaw
	var cfg struct {
		DownloadCover bool `json:"download_cover"`
	}
	json.Unmarshal(configRaw, &cfg)
	videoResource := model.DownloadResource{Name: "platform-file.bin", Kind: "video"}
	videoEndpoint := model.DownloadEndpoint{Protocol: "HTTP", URL: h.endpointURL + "/video", Enabled: 1}
	info := &types.DownloadTaskResult{
		Task: &model.DownloadTaskV1{
			Name:       "platform-file.bin",
			UniqueID:   "api_test_platform_file",
			PlatformId: "api_test_save_path",
			Status:     model.TaskStatusWaiting,
		},
		Content: &model.Content{
			Id:          "api_test_save_path:content-1",
			PlatformId:  "api_test_save_path",
			Type: "video",
			ExternalId:  "content-1",
			Title:       "platform-file.bin",
		},
		Resources: []*types.ResourceInfo{{
			DownloadResource: videoResource,
			Endpoints:        []model.DownloadEndpoint{videoEndpoint},
		}},
	}
	if cfg.DownloadCover {
		info.Resources = append(info.Resources, &types.ResourceInfo{
			DownloadResource: model.DownloadResource{Name: "platform-file.jpg", Kind: "cover", MergeOrder: 1},
			Endpoints:        []model.DownloadEndpoint{{Protocol: "HTTP", URL: h.endpointURL + "/cover", Enabled: 1}},
		})
	}
	return info, nil
}

func TestHandleCreateDownloadTaskV1UsesConfiguredSavePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// In-memory SQLite requires a single connection; otherwise Hermes goroutines see an empty DB.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.Content{},
		&model.DownloadTaskV1{},
		&model.DownloadResource{},
		&model.DownloadEndpoint{},
		&model.DownloadSegment{},
		&model.DownloadConnection{},
	))

	registerSavePathHandler.Do(func() {
		registry.Register(savePathTestHandler)
	})
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "4")
		_, _ = w.Write([]byte("test"))
	}))
	defer testServer.Close()
	savePathTestHandler.endpointURL = testServer.URL + "/platform-file.bin"

	nopLogger := zerolog.Nop()
	workDir := t.TempDir()
	expectedSaveDir := filepath.Join(workDir, "downloads")
	client := &APIClient{
		db:     db,
		logger: &nopLogger,
		cfg: &APIConfig{
			WorkDir:     workDir,
			DownloadDir: expectedSaveDir,
		},
	}
	client.downloader = hermes.New(hermes.NewOpt{Store: &dbTaskStore{db: db}, MaxConcurrent: 1, BasePath: expectedSaveDir})
	client.downloader.RegisterProtocol(protocol.NewHTTPDriver())
	client.downloadTaskService = services.NewDownloadTaskService(db, &nopLogger, client.downloader, nil, workDir, expectedSaveDir)
	defer client.downloader.PauseAll()

	body := []byte(`{"objects":[{"platform":"api_test_save_path","content":{},"config":{"download_cover":true}}]}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/download_task/create", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	client.handleCreateDownloadTaskV1(ctx)

	var response struct {
		Code int `json:"code"`
		Data struct {
			Tasks []struct {
				Success bool `json:"success"`
				Data    struct {
					Task      model.DownloadTaskV1     `json:"task"`
					Resources []model.DownloadResource `json:"resources"`
					Endpoints []model.DownloadEndpoint `json:"endpoints"`
				} `json:"data"`
				Error string `json:"error"`
			} `json:"tasks"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Zero(t, response.Code, recorder.Body.String())
	require.Len(t, response.Data.Tasks, 1)
	require.True(t, response.Data.Tasks[0].Success)

	result := response.Data.Tasks[0].Data
	assert.Equal(t, model.TaskStatusPreparing, result.Task.Status)
	require.NotNil(t, result.Task.ContentId)
	assert.Equal(t, "api_test_save_path:content-1", *result.Task.ContentId)
	assert.Nil(t, result.Task.ParentTaskID)
	assert.Equal(t, result.Task.Id, result.Task.RootTaskID)
	var cfg map[string]any
	require.NoError(t, json.Unmarshal(savePathTestHandler.config, &cfg))
	assert.True(t, cfg["download_cover"].(bool))
	assert.Equal(t, expectedSaveDir, cfg["save_path"].(string))
	require.Len(t, result.Resources, 2)
	require.Len(t, result.Endpoints, 2)
	assert.Equal(t, "video", result.Resources[0].Kind)
	assert.Equal(t, "cover", result.Resources[1].Kind)

	var persisted model.DownloadTaskV1
	require.NoError(t, db.First(&persisted, result.Task.Id).Error)
	require.NotNil(t, persisted.ContentId)
	assert.Equal(t, "api_test_save_path:content-1", *persisted.ContentId)
	assert.DirExists(t, expectedSaveDir)

	require.Eventually(t, func() bool {
		if err := db.First(&persisted, persisted.Id).Error; err != nil {
			return false
		}
		return persisted.Status == model.TaskStatusFinished
	}, 2*time.Second, 10*time.Millisecond)
	content, err := os.ReadFile(filepath.Join(expectedSaveDir, "platform-file.bin"))
	require.NoError(t, err)
	assert.Equal(t, []byte("test"), content)
	cover, err := os.ReadFile(filepath.Join(expectedSaveDir, "platform-file.jpg"))
	require.NoError(t, err)
	assert.Equal(t, []byte("test"), cover)

	record, err := client.buildDownloadTaskRecord(persisted.Id)
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.Equal(t, persisted.Id, record.RootTaskID)
	assert.Zero(t, record.ChildCount)
	require.Len(t, record.Files, 2)
	assert.Equal(t, 2, record.FileCount)
	assert.Equal(t, "video", record.Files[0].Kind)
	assert.Equal(t, "platform-file.bin", record.Files[0].OutputPath)
	assert.Equal(t, "finished", record.Files[0].Status)
	assert.Equal(t, "cover", record.Files[1].Kind)
	assert.Equal(t, "platform-file.jpg", record.Files[1].OutputPath)
	assert.Equal(t, "finished", record.Files[1].Status)
}

func TestCreateDownloadTaskV1SingleLinksContentToPersistedAccount(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Content{},
		&model.Account{},
		&model.ContentAccount{},
		&model.DownloadTaskV1{},
		&model.DownloadResource{},
		&model.DownloadEndpoint{},
	))

	handler := &contentAccountTestPlatformHandler{}
	registerContentAccountTestHandler.Do(func() {
		registry.Register(handler)
	})
	require.NoError(t, db.Create(&model.Account{
		Id:         "api_test_content_account:canonical-account",
		PlatformId: handler.PlatformID(),
		ExternalId: "author-1",
		Nickname:   "原作者",
		Timestamps: model.Timestamps{CreatedAt: 100, UpdatedAt: 100},
	}).Error)

	nopLogger := zerolog.Nop()
	client := &APIClient{
		db:     db,
		logger: &nopLogger,
		cfg: &APIConfig{
			WorkDir:     t.TempDir(),
			DownloadDir: t.TempDir(),
		},
	}
	client.downloadTaskService = services.NewDownloadTaskService(db, &nopLogger, nil, nil, client.cfg.WorkDir, client.cfg.DownloadDir)

	_, err = client.createDownloadTaskV1Single(CreateDownloadTaskV1Body{
		Platform: handler.PlatformID(),
		Content:  json.RawMessage(`{}`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Hermes 下载器未初始化")

	var accounts []model.Account
	require.NoError(t, db.Find(&accounts).Error)
	require.Len(t, accounts, 1)
	assert.Equal(t, "api_test_content_account:canonical-account", accounts[0].Id)
	assert.Equal(t, "更新后的作者", accounts[0].Nickname)

	var association model.ContentAccount
	require.NoError(t, db.First(&association).Error)
	assert.Equal(t, "api_test_content_account:content-1", association.ContentId)
	assert.Equal(t, "api_test_content_account:canonical-account", association.AccountId)
	assert.Equal(t, "owner", association.Role)
}

func TestHandleCreateDownloadTaskByURLV1InfersFilenameExtension(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// In-memory SQLite requires a single connection; otherwise Hermes goroutines see an empty DB.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.DownloadTaskV1{},
		&model.DownloadResource{},
		&model.DownloadEndpoint{},
		&model.DownloadSegment{},
		&model.DownloadConnection{},
	))

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", "8")
		_, _ = w.Write([]byte("png-data"))
	}))
	defer testServer.Close()

	workDir := t.TempDir()
	nopLogger := zerolog.Nop()
	client := &APIClient{
		db:     db,
		logger: &nopLogger,
		cfg:    &APIConfig{WorkDir: workDir, DownloadDir: workDir},
	}
	client.downloader = hermes.New(hermes.NewOpt{Store: &dbTaskStore{db: db}, MaxConcurrent: 1, BasePath: workDir})
	client.downloader.RegisterProtocol(protocol.NewHTTPDriver())
	defer client.downloader.PauseAll()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/download_task/create_by_url", bytes.NewBufferString(`{"objects":[{"url":"`+testServer.URL+`/image","filename":"cover"}]}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	client.handleCreateDownloadTaskByURLV1(ctx)

	var response struct {
		Code int `json:"code"`
		Data struct {
			Tasks []struct {
				Success bool `json:"success"`
				Data    struct {
					Task model.DownloadTaskV1 `json:"task"`
				} `json:"data"`
				Error string `json:"error"`
			} `json:"tasks"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Zero(t, response.Code, recorder.Body.String())
	require.Len(t, response.Data.Tasks, 1)
	require.True(t, response.Data.Tasks[0].Success)

	var task model.DownloadTaskV1
	require.Eventually(t, func() bool {
		if err := db.First(&task, response.Data.Tasks[0].Data.Task.Id).Error; err != nil {
			return false
		}
		return task.Status == model.TaskStatusFinished
	}, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, "cover", task.Name)
	var resource model.DownloadResource
	require.NoError(t, db.Where("task_id = ?", task.Id).First(&resource).Error)
	assert.Equal(t, "cover.png", resource.Name)
	content, err := os.ReadFile(filepath.Join(workDir, "cover.png"))
	require.NoError(t, err)
	assert.Equal(t, []byte("png-data"), content)
}

func TestHandleListDownloadTaskV1IncludesLatestFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.DownloadTaskV1{},
		&model.DownloadResource{},
		&model.DownloadEndpoint{},
		&model.DownloadSegment{},
		&model.DownloadConnection{},
	))

	now := time.Now().UnixMilli()
	task := model.DownloadTaskV1{
		Name:         "failed.bin",
		PlatformId:   "wx_channels",
		Status:       model.TaskStatusFailed,
		SourceURL:    "https://channels.weixin.qq.com/web/pages/feed?finderid=test",
		CoverURL:     "https://example.com/cover.jpg",
		CoverWidth:   "1080",
		CoverHeight:  "1440",
		ErrorMessage: "latest error",
		Timestamps:   model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, db.Create(&task).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/download_task/list", nil)
	nopLogger := zerolog.Nop()
	client := &APIClient{db: db, logger: &nopLogger}
	client.handleListDownloadTaskV1(ctx)

	var response struct {
		Code int `json:"code"`
		Data struct {
			List []DownloadTaskRecord `json:"list"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Zero(t, response.Code)
	require.Len(t, response.Data.List, 1)
	assert.Equal(t, "latest error", response.Data.List[0].Error)
	assert.Equal(t, "wx_channels", response.Data.List[0].PlatformID)
	assert.Equal(t, "https://channels.weixin.qq.com/web/pages/feed?finderid=test", response.Data.List[0].SourceURL)
	assert.Equal(t, "https://example.com/cover.jpg", response.Data.List[0].CoverURL)
	assert.Equal(t, "1080", response.Data.List[0].CoverWidth)
	assert.Equal(t, "1440", response.Data.List[0].CoverHeight)

	record, err := client.buildDownloadTaskRecord(task.Id)
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.Equal(t, "latest error", record.Error)

	message := DownloadTaskWSMessage{Type: downloadTaskWSUpsert, Tasks: []DownloadTaskRecord{*record}}
	assert.Equal(t, response.Data.List[0], message.Tasks[0])
}

func TestHandleListDownloadTaskV1FiltersAndReportsLineage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.DownloadTaskV1{},
		&model.DownloadResource{},
		&model.DownloadEndpoint{},
		&model.DownloadSegment{},
		&model.DownloadConnection{},
	))

	now := time.Now().UnixMilli()
	root := model.DownloadTaskV1{
		Name:       "root",
		PlatformId: "wxchannels",
		Status:     model.TaskStatusWaiting,
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, db.Create(&root).Error)
	root.RootTaskID = root.Id
	require.NoError(t, db.Model(&root).Update("root_task_id", root.RootTaskID).Error)

	childOne := model.DownloadTaskV1{
		ParentTaskID: &root.Id,
		RootTaskID:   root.Id,
		RelationType: model.TaskRelationDiscovered,
		Name:         "child-one",
		PlatformId:   "wxchannels",
		Status:       model.TaskStatusWaiting,
		Timestamps:   model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	childTwo := childOne
	childTwo.Name = "child-two"
	require.NoError(t, db.Create(&childOne).Error)
	require.NoError(t, db.Create(&childTwo).Error)

	nopLogger := zerolog.Nop()
	client := &APIClient{db: db, logger: &nopLogger}
	rootRecord, err := client.buildDownloadTaskRecord(root.Id)
	require.NoError(t, err)
	require.NotNil(t, rootRecord)
	assert.Equal(t, 2, rootRecord.ChildCount)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/download_task/list?parent_task_id="+strconv.Itoa(root.Id), nil)
	client.handleListDownloadTaskV1(ctx)

	var response struct {
		Code int `json:"code"`
		Data struct {
			List []DownloadTaskRecord `json:"list"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Zero(t, response.Code, recorder.Body.String())
	require.Len(t, response.Data.List, 2)
	for _, record := range response.Data.List {
		require.NotNil(t, record.ParentTaskID)
		assert.Equal(t, root.Id, *record.ParentTaskID)
		assert.Equal(t, root.Id, record.RootTaskID)
		assert.Equal(t, model.TaskRelationDiscovered, record.RelationType)
	}
}

func TestHandleListDownloadTaskV1ReturnsFractionalProgress(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.DownloadTaskV1{},
		&model.DownloadResource{},
		&model.DownloadEndpoint{},
		&model.DownloadSegment{},
		&model.DownloadConnection{},
	))

	now := time.Now().UnixMilli()
	task := model.DownloadTaskV1{
		Name:       "progress.bin",
		Status:     model.TaskStatusDownloading,
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, db.Create(&task).Error)
	resource := model.DownloadResource{
		TaskId:     task.Id,
		Name:       task.Name,
		Kind:       "file",
		Size:       1000,
		Status:     1,
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, db.Create(&resource).Error)
	segment := model.DownloadSegment{
		ResourceId: resource.Id,
		Index:      0,
		Size:       1000,
		Downloaded: 5,
		Status:     1,
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, db.Create(&segment).Error)
	endpoint := model.DownloadEndpoint{
		ResourceId: resource.Id,
		Protocol:   "HTTP",
		URL:        "http://127.0.0.1/progress.bin",
		Enabled:    1,
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, db.Create(&endpoint).Error)
	connection := model.DownloadConnection{
		EndpointId: endpoint.Id,
		WorkerId:   "worker-progress",
		Speed:      2048,
		Bytes:      5,
		Status:     1,
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, db.Create(&connection).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/download_task/list", nil)
	nopLogger := zerolog.Nop()
	client := &APIClient{db: db, logger: &nopLogger}
	client.handleListDownloadTaskV1(ctx)

	var response struct {
		Code int `json:"code"`
		Data struct {
			List []DownloadTaskRecord `json:"list"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Zero(t, response.Code)
	require.Len(t, response.Data.List, 1)
	assert.Equal(t, int64(5), response.Data.List[0].Downloaded)
	assert.Equal(t, int64(1000), response.Data.List[0].Size)
	assert.Equal(t, int64(2048), response.Data.List[0].Speed)
	assert.InDelta(t, 0.5, response.Data.List[0].Progress, 0.001)
	require.Len(t, response.Data.List[0].Files, 1)
	assert.Equal(t, int64(2048), response.Data.List[0].Files[0].Speed)
	assert.InDelta(t, 0.5, response.Data.List[0].Files[0].Progress, 0.001)
}

func TestHandleCreateDownloadTaskByURLV1AppliesFilenameTemplate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	// In-memory SQLite requires a single connection; otherwise Hermes goroutines see an empty DB.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.DownloadTaskV1{},
		&model.DownloadResource{},
		&model.DownloadEndpoint{},
		&model.DownloadSegment{},
		&model.DownloadConnection{},
	))

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", "8")
		_, _ = w.Write([]byte("png-data"))
	}))
	defer testServer.Close()

	workDir := t.TempDir()
	template := "name + '_' + task_id"
	nopLogger := zerolog.Nop()
	client := &APIClient{
		db:     db,
		logger: &nopLogger,
		cfg:    &APIConfig{WorkDir: workDir, DownloadDir: workDir, FilenameTemplate: template},
	}
	client.downloader = hermes.New(hermes.NewOpt{Store: &dbTaskStore{db: db}, MaxConcurrent: 1, FilenameTemplate: template, BasePath: workDir})
	client.downloader.RegisterProtocol(protocol.NewHTTPDriver())
	defer client.downloader.PauseAll()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/download_task/create_by_url", bytes.NewBufferString(`{"objects":[{"url":"`+testServer.URL+`/image","filename":"cover"}]}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	client.handleCreateDownloadTaskByURLV1(ctx)

	var response struct {
		Code int `json:"code"`
		Data struct {
			Tasks []struct {
				Success bool `json:"success"`
				Data    struct {
					Task model.DownloadTaskV1 `json:"task"`
				} `json:"data"`
				Error string `json:"error"`
			} `json:"tasks"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Zero(t, response.Code, recorder.Body.String())
	require.Len(t, response.Data.Tasks, 1)
	require.True(t, response.Data.Tasks[0].Success)

	taskID := response.Data.Tasks[0].Data.Task.Id
	require.NotZero(t, taskID)

	var task model.DownloadTaskV1
	require.Eventually(t, func() bool {
		if err := db.First(&task, taskID).Error; err != nil {
			return false
		}
		return task.Status == model.TaskStatusFinished
	}, 2*time.Second, 10*time.Millisecond)

	// Template renders "cover_<taskID>", then Content-Type extension appends ".png"
	// Task is a pure container, its name and save_path are not updated.
	var resource model.DownloadResource
	require.NoError(t, db.Where("task_id = ?", taskID).First(&resource).Error)
	expectedFileName := "cover_" + strconv.Itoa(taskID) + ".png"
	assert.Equal(t, expectedFileName, resource.Name)
	content, err := os.ReadFile(filepath.Join(workDir, expectedFileName))
	require.NoError(t, err)
	assert.Equal(t, []byte("png-data"), content)
}

func TestHandleDeleteDownloadTaskV1LogsFileAndCascadeDiagnostics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.DownloadTaskV1{},
		&model.DownloadResource{},
		&model.DownloadEndpoint{},
		&model.DownloadSegment{},
		&model.DownloadConnection{},
	))

	downloadRoot := t.TempDir()
	filePath := filepath.Join(downloadRoot, "delete-diagnostic.mp4")
	require.NoError(t, os.WriteFile(filePath, []byte("video"), 0644))
	partialPath := filePath + ".part"
	require.NoError(t, os.WriteFile(partialPath, []byte("partial"), 0644))
	configJSON, err := json.Marshal(DownloadConfig{SavePath: downloadRoot})
	require.NoError(t, err)

	now := time.Now().UnixMilli()
	task := model.DownloadTaskV1{Name: "delete diagnostic", Status: model.TaskStatusFinished, ConfigJSON: string(configJSON)}
	task.CreatedAt, task.UpdatedAt = now, now
	require.NoError(t, db.Create(&task).Error)
	resource := model.DownloadResource{TaskId: task.Id, Name: filepath.Base(filePath), Kind: "file", ResourceType: model.ResourceTypeFile, Status: 2}
	resource.CreatedAt, resource.UpdatedAt = now, now
	require.NoError(t, db.Create(&resource).Error)
	endpoint := model.DownloadEndpoint{ResourceId: resource.Id, Protocol: "https", URL: "https://example.com/video.mp4", Enabled: 1}
	endpoint.CreatedAt, endpoint.UpdatedAt = now, now
	require.NoError(t, db.Create(&endpoint).Error)
	segment := model.DownloadSegment{ResourceId: resource.Id, Index: 0, Size: 5, Downloaded: 5, Status: 2}
	segment.CreatedAt, segment.UpdatedAt = now, now
	require.NoError(t, db.Create(&segment).Error)
	connection := model.DownloadConnection{EndpointId: endpoint.Id, Status: 2}
	connection.CreatedAt, connection.UpdatedAt = now, now
	require.NoError(t, db.Create(&connection).Error)

	var logOutput bytes.Buffer
	logger := zerolog.New(&logOutput)
	client := &APIClient{
		db:     db,
		logger: &logger,
		cfg:    &APIConfig{DownloadDir: downloadRoot},
	}
	client.downloader = hermes.New(hermes.NewOpt{Store: &dbTaskStore{db: db}, Logger: &logger, BasePath: downloadRoot})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/download_task/delete", bytes.NewBufferString(`{"task_id":`+strconv.Itoa(task.Id)+`,"delete_files":true}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	client.handleDeleteDownloadTaskV1(ctx)

	var response struct {
		Code int `json:"code"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Zero(t, response.Code, recorder.Body.String())
	_, err = os.Stat(filePath)
	if !os.IsNotExist(err) {
		t.Fatalf("expected final file to be removed, stat error: %v", err)
	}
	_, err = os.Stat(partialPath)
	if !os.IsNotExist(err) {
		t.Fatalf("expected partial file to be removed, stat error: %v", err)
	}

	logs := logOutput.String()
	for _, expected := range []string{
		`"delete_files":true`,
		`"phase":"before_delete"`,
		`"candidate_type":"final"`,
		`"path":"` + filePath + `"`,
		`"exists":true`,
		`"local_file_cleanup_attempted":true`,
		`Associated local file removed`,
		`"entity":"connections"`,
	} {
		if !strings.Contains(logs, expected) {
			t.Fatalf("expected deletion diagnostics to contain %q; logs:\n%s", expected, logs)
		}
	}

	for entity, id := range map[string]int{
		"resource":   resource.Id,
		"endpoint":   endpoint.Id,
		"segment":    segment.Id,
		"connection": connection.Id,
	} {
		var count int64
		var query *gorm.DB
		switch entity {
		case "resource":
			query = db.Unscoped().Model(&model.DownloadResource{}).Where("id = ? AND deleted_at IS NOT NULL", id)
		case "endpoint":
			query = db.Unscoped().Model(&model.DownloadEndpoint{}).Where("id = ? AND deleted_at IS NOT NULL", id)
		case "segment":
			query = db.Unscoped().Model(&model.DownloadSegment{}).Where("id = ? AND deleted_at IS NOT NULL", id)
		case "connection":
			query = db.Unscoped().Model(&model.DownloadConnection{}).Where("id = ? AND deleted_at IS NOT NULL", id)
		}
		require.NoError(t, query.Count(&count).Error)
		assert.Equal(t, int64(1), count, entity+" should be soft-deleted")
	}

	// Retrying delete_files=true must recover orphaned files even after the task
	// and resources have already been soft-deleted by an earlier request.
	require.NoError(t, os.WriteFile(filePath, []byte("orphan"), 0644))
	retryRecorder := httptest.NewRecorder()
	retryCtx, _ := gin.CreateTestContext(retryRecorder)
	retryCtx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/download_task/delete", bytes.NewBufferString(`{"task_id":`+strconv.Itoa(task.Id)+`,"delete_files":true}`))
	retryCtx.Request.Header.Set("Content-Type", "application/json")
	client.handleDeleteDownloadTaskV1(retryCtx)
	require.NoError(t, json.Unmarshal(retryRecorder.Body.Bytes(), &response))
	assert.Zero(t, response.Code, retryRecorder.Body.String())
	_, err = os.Stat(filePath)
	if !os.IsNotExist(err) {
		t.Fatalf("expected orphaned file to be removed on retry, stat error: %v", err)
	}
	if !strings.Contains(logOutput.String(), "Recovered local file cleanup for previously soft-deleted download task") {
		t.Fatalf("expected soft-deleted task recovery log; logs:\n%s", logOutput.String())
	}
}

func TestDeleteDownloadTaskLocalFilesRejectsPathOutsideDownloadRoot(t *testing.T) {
	downloadRoot := t.TempDir()
	outsideRoot := t.TempDir()
	outsidePath := filepath.Join(outsideRoot, "keep.mp4")
	require.NoError(t, os.WriteFile(outsidePath, []byte("keep"), 0644))

	var logOutput bytes.Buffer
	logger := zerolog.New(&logOutput)
	client := &APIClient{logger: &logger, cfg: &APIConfig{DownloadDir: downloadRoot}}
	task := model.DownloadTaskV1{Id: 42}
	resource := model.DownloadResource{Id: 7, TaskId: task.Id, Name: outsidePath}

	err := client.deleteDownloadTaskLocalFiles(task, []model.DownloadResource{resource})
	if err == nil {
		t.Fatal("expected path outside the download root to be rejected")
	}
	content, readErr := os.ReadFile(outsidePath)
	require.NoError(t, readErr)
	assert.Equal(t, []byte("keep"), content)
	if !strings.Contains(logOutput.String(), "Rejected local file candidate outside download root") {
		t.Fatalf("expected unsafe path rejection log; logs:\n%s", logOutput.String())
	}
}
