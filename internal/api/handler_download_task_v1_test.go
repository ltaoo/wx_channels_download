package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/internal/download/registry"
	"wx_channel/pkg/hermes"
)

type savePathTestPlatformHandler struct {
	config      registry.DownloadConfig
	endpointURL string
}

var (
	savePathTestHandler     = &savePathTestPlatformHandler{}
	registerSavePathHandler sync.Once
)

func (h *savePathTestPlatformHandler) PlatformID() string {
	return "api_test_save_path"
}

func (h *savePathTestPlatformHandler) BuildDownloadTask(_ json.RawMessage, config registry.DownloadConfig) (*registry.DownloadInfo, *model.Content, *model.Account, error) {
	h.config = config
	videoResource := model.DownloadResource{Name: "platform-file.bin", Kind: "video"}
	videoEndpoint := model.DownloadEndpoint{Protocol: "HTTP", URL: h.endpointURL + "/video", Enabled: 1}
	info := &registry.DownloadInfo{
		Task: model.DownloadTaskV1{
			Name:         "platform-file.bin",
			ResourceType: model.ResourceTypeFile,
			Status:       model.TaskStatusWaiting,
			SavePath:     "/platform/hard-coded/path",
		},
		Resource: videoResource,
		Endpoint: videoEndpoint,
		Resources: []registry.DownloadResourceInfo{{
			Resource:  videoResource,
			Endpoints: []model.DownloadEndpoint{videoEndpoint},
		}},
	}
	if config.DownloadCover {
		info.Task.ResourceType = model.ResourceTypeCollection
		info.Resources = append(info.Resources, registry.DownloadResourceInfo{
			Resource:  model.DownloadResource{Name: "platform-file.jpg", Kind: "cover", MergeOrder: 1},
			Endpoints: []model.DownloadEndpoint{{Protocol: "HTTP", URL: h.endpointURL + "/cover", Enabled: 1}},
		})
	}
	return info, nil, nil, nil
}

func TestHandleCreateDownloadTaskV1UsesConfiguredSavePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.DownloadTaskV1{},
		&model.DownloadResource{},
		&model.DownloadEndpoint{},
		&model.DownloadSegment{},
		&model.DownloadConnection{},
		&model.DownloadLog{},
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

	workDir := t.TempDir()
	expectedSaveDir := filepath.Join(workDir, "downloads")
	client := &APIClient{
		db: db,
		cfg: &APIConfig{
			WorkDir:     workDir,
			DownloadDir: expectedSaveDir,
		},
	}
	client.downloader = hermes.New(&dbTaskStore{db: db}, nil, 1)
	defer client.downloader.PauseAll()

	body := []byte(`[{"platform":"api_test_save_path","content":{},"config":{"download_cover":true}}]`)
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
	assert.Equal(t, model.ResourceTypeCollection, result.Task.ResourceType)
	assert.True(t, savePathTestHandler.config.DownloadCover)
	assert.Equal(t, expectedSaveDir, savePathTestHandler.config.SavePath)
	assert.Equal(t, expectedSaveDir, result.Task.SavePath)
	require.Len(t, result.Resources, 2)
	require.Len(t, result.Endpoints, 2)
	assert.Equal(t, "video", result.Resources[0].Kind)
	assert.Equal(t, "cover", result.Resources[1].Kind)

	var persisted model.DownloadTaskV1
	require.NoError(t, db.First(&persisted, result.Task.Id).Error)
	assert.Equal(t, expectedSaveDir, persisted.SavePath)
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
	require.Len(t, record.Files, 2)
	assert.Equal(t, 2, record.FileCount)
	assert.Equal(t, "video", record.Files[0].Kind)
	assert.Equal(t, filepath.Join(expectedSaveDir, "platform-file.bin"), record.Files[0].OutputPath)
	assert.Equal(t, "finished", record.Files[0].Status)
	assert.Equal(t, "cover", record.Files[1].Kind)
	assert.Equal(t, filepath.Join(expectedSaveDir, "platform-file.jpg"), record.Files[1].OutputPath)
	assert.Equal(t, "finished", record.Files[1].Status)
}

func TestHandleCreateDownloadTaskByURLV1InfersFilenameExtension(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.DownloadTaskV1{},
		&model.DownloadResource{},
		&model.DownloadEndpoint{},
		&model.DownloadSegment{},
		&model.DownloadConnection{},
		&model.DownloadLog{},
	))

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", "8")
		_, _ = w.Write([]byte("png-data"))
	}))
	defer testServer.Close()

	workDir := t.TempDir()
	client := &APIClient{
		db:  db,
		cfg: &APIConfig{WorkDir: workDir, DownloadDir: workDir},
	}
	client.downloader = hermes.New(&dbTaskStore{db: db}, nil, 1)
	defer client.downloader.PauseAll()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/download_task/create_by_url", bytes.NewBufferString(`[{"url":"`+testServer.URL+`/image","filename":"cover"}]`))
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
	assert.Equal(t, "cover.png", task.Name)
	assert.Equal(t, filepath.Join(workDir, "cover.png"), task.SavePath)

	var resource model.DownloadResource
	require.NoError(t, db.Where("task_id = ?", task.Id).First(&resource).Error)
	assert.Equal(t, "cover.png", resource.Name)
	content, err := os.ReadFile(task.SavePath)
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
		&model.DownloadLog{},
	))

	now := time.Now().UnixMilli()
	task := model.DownloadTaskV1{
		Name:         "failed.bin",
		ResourceType: model.ResourceTypeFile,
		Status:       model.TaskStatusFailed,
		SavePath:     t.TempDir(),
		Timestamps:   model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, db.Create(&task).Error)
	require.NoError(t, db.Create(&model.DownloadLog{TaskId: task.Id, Level: "error", Message: "first error", CreatedAt: now}).Error)
	require.NoError(t, db.Create(&model.DownloadLog{TaskId: task.Id, Level: "error", Message: "latest error", CreatedAt: now + 1}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/download_task/list", nil)
	client := &APIClient{db: db}
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

	record, err := client.buildDownloadTaskRecord(task.Id)
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.Equal(t, "latest error", record.Error)

	message := DownloadTaskWSMessage{Type: downloadTaskWSUpsert, Tasks: []DownloadTaskRecord{*record}}
	assert.Equal(t, response.Data.List[0], message.Tasks[0])
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
		&model.DownloadLog{},
	))

	now := time.Now().UnixMilli()
	task := model.DownloadTaskV1{
		Name:         "progress.bin",
		ResourceType: model.ResourceTypeFile,
		Status:       model.TaskStatusDownloading,
		SavePath:     t.TempDir(),
		Timestamps:   model.Timestamps{CreatedAt: now, UpdatedAt: now},
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
	client := &APIClient{db: db}
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
