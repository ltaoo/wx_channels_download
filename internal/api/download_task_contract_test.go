package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"wx_channel/internal/database"
	"wx_channel/internal/database/model"
	"wx_channel/internal/services"
	"wx_channel/pkg/dm"
	"wx_channel/pkg/hermes"
)

func task_contract_database(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	sql_db, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sql_db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sql_db.Close() })
	if err := db.AutoMigrate(&model.DownloadTask{}, &model.DownloadResource{}, &model.DownloadEndpoint{}, &model.DownloadSegment{}, &model.DownloadConnection{}, &model.Content{}, &model.Account{}, &model.ContentAccount{}); err != nil {
		t.Fatal(err)
	}
	return db
}

type task_contract_driver struct{}

func (task_contract_driver) Protocols() []string { return []string{"contract"} }
func (task_contract_driver) Prepare(context.Context, hermes.Endpoint) (hermes.PreparedResource, error) {
	return hermes.PreparedResource{Size: 2, ContentType: "video/mp4"}, nil
}
func (task_contract_driver) Open(_ context.Context, endpoint hermes.Endpoint, _ hermes.ReadRequest) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(endpoint.URL)), nil
}

type task_contract_postprocessor struct{}

func (task_contract_postprocessor) Process(_ context.Context, job *hermes.TaskJob) error {
	if err := os.WriteFile(job.Resources[0].FilePath, []byte("1234"), 0600); err != nil {
		return err
	}
	if err := os.Remove(job.Resources[1].FilePath); err != nil {
		return err
	}
	job.Resources = job.Resources[:1]
	job.Resources[0].Name = "成片"
	job.Resources[0].Size, job.Resources[0].Downloaded = 4, 4
	return nil
}

func TestCompletedTaskRESTGoSDKAndHookShareDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := task_contract_database(t)
	download_dir := t.TempDir()
	log := zerolog.Nop()
	store := database.NewDBTaskStore(db, &log)
	hooks := hermes.NewHookManager()
	engine := hermes.New(hermes.HermesNewConfig{Store: store, Logger: &log, Config: hermes.HermesEngineConfig{BasePath: download_dir}})
	engine.RegisterProtocol(task_contract_driver{})
	engine.SetHooks(hooks)
	engine.SetPostprocessor(task_contract_postprocessor{})
	service := services.NewDownloadTaskService(db, &log, engine, hooks, download_dir, download_dir, nil)
	engine.SetTaskDetailProvider(func(task_id int, snapshot []hermes.TaskFinishedResource) (*dm.DownloadTask, error) {
		return service.BuildTaskDetail(context.Background(), task_id, snapshot)
	})
	content_id := "test:video"
	config_json, _ := json.Marshal(map[string]any{"download_dir": download_dir})
	task := model.DownloadTask{Id: 42, RootTaskID: 42, ContentId: &content_id, Name: "任务", PlatformId: "test", ConfigJSON: string(config_json), MetadataJSON: `{"title":"测试"}`, Timestamps: model.Timestamps{CreatedAt: 1000, UpdatedAt: 1000}}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	content := model.Content{Id: content_id, PlatformId: "test", Type: "video", Title: "测试内容"}
	account := model.Account{Id: "test:author", PlatformId: "test", Nickname: "作者"}
	for _, value := range []any{&content, &account, &model.ContentAccount{ContentId: content_id, AccountId: account.Id}} {
		if err := db.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	for index, body := range []string{"12", "34"} {
		resource := model.DownloadResource{Id: 7 + index, TaskId: &task.Id, DownloadDir: download_dir, Name: "分片", UniqueID: fmt.Sprintf("part-%d", index), Kind: "video/mp4", Type: "FILE"}
		if err := db.Create(&resource).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.DownloadEndpoint{ResourceId: resource.Id, Protocol: "contract", URL: body, Enabled: 1}).Error; err != nil {
			t.Fatal(err)
		}
	}
	received := make(chan map[string]any, 1)
	uploaded := make(chan string, 1)
	webhook := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/upload" {
			if err := request.ParseMultipartForm(1024); err != nil {
				t.Error(err)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			defer request.MultipartForm.RemoveAll()
			file, header, err := request.FormFile("file")
			if err != nil {
				t.Error(err)
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			defer file.Close()
			data, err := io.ReadAll(file)
			if err != nil || header.Filename != "成片.mp4" || header.Header.Get("Content-Type") != "video/mp4" {
				t.Errorf("incorrect upload: filename=%s headers=%v err=%v", header.Filename, header.Header, err)
			}
			uploaded <- string(data)
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		var detail map[string]any
		if err := json.NewDecoder(request.Body).Decode(&detail); err != nil {
			t.Error(err)
		}
		received <- detail
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer webhook.Close()
	if err := hooks.Load(fmt.Sprintf(`import fs from "fs";
export async function onTaskSuccess(task) {
  if (task.id !== 42 || task.status !== 5 || task.file_count !== 1 || task.files.length !== 1) throw new Error("wrong task shape");
  if (!task.files[0].exists || task.files[0].name !== "成片.mp4" || task.content.accounts[0].nickname !== "作者") throw new Error("not finalized");
  const response = await fetch(%q, {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify(task)});
  if (!response.ok) throw new Error("webhook failed");
  const file = task.files[0];
  const form = new FormData();
  form.append("file", await fs.openAsBlob(file.file_path, {type: file.kind}), file.name);
  const upload = await fetch(%q, {method: "POST", body: form});
  if (!upload.ok) throw new Error("file upload failed");
}
export function onTaskFinish(task) { var err = writeTextFile("finish.marker", String(task.id)); if (err) throw err; }`, webhook.URL, webhook.URL+"/upload")); err != nil {
		t.Fatal(err)
	}
	api := &APIClient{db: db, logger: &log, service_download_task: service}
	router := gin.New()
	router.GET("/api/v1/download_task/detail", api.handle_download_task_detail)
	server := httptest.NewServer(router)
	defer server.Close()
	client, err := dm.NewClient(dm.ClientOptions{BaseURL: server.URL, PollInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.StartTask(task.Id); err != nil {
		t.Fatal(err)
	}
	defer engine.PauseAllTask()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	completed, err := client.WaitDownloadTask(ctx, task.Id)
	if err != nil {
		t.Fatal(err)
	}
	var hook_detail map[string]any
	select {
	case hook_detail = <-received:
	case <-ctx.Done():
		t.Fatal("success hook was not called")
	}
	select {
	case data := <-uploaded:
		if data != "1234" {
			t.Fatalf("expected postprocessed file, got %q", data)
		}
	case <-ctx.Done():
		t.Fatal("final file was not uploaded")
	}
	encoded, err := json.Marshal(completed)
	if err != nil {
		t.Fatal(err)
	}
	var sdk_detail map[string]any
	if err := json.Unmarshal(encoded, &sdk_detail); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(hook_detail, sdk_detail) {
		t.Fatalf("hook and SDK differ:\nhook=%v\nsdk=%v", hook_detail, sdk_detail)
	}
	detail, err := client.DownloadTaskDetail(ctx, task.Id)
	if err != nil || !reflect.DeepEqual(detail, completed) {
		t.Fatalf("detail differs from completion: %+v, err=%v", detail, err)
	}
	if completed.Files[0].FilePath != filepath.Join(download_dir, "成片.mp4") || completed.Files[0].LocalPath != completed.Files[0].FilePath || completed.Downloaded != 4 || completed.Speed != 0 {
		t.Fatalf("unexpected output: %+v", completed)
	}
	data_service := services.NewDataQueryService(services.DataQueryServiceConfig{DB: db, DownloadTaskService: service})
	mcp_detail, err := data_service.GetDownloadTaskDetail(ctx, task.Id)
	if err != nil || !reflect.DeepEqual(mcp_detail, completed) {
		t.Fatalf("MCP differs from completion: %+v, err=%v", mcp_detail, err)
	}
	for {
		if _, err := os.Stat(filepath.Join(download_dir, "finish.marker")); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("finish hook did not run after success")
		case <-time.After(time.Millisecond):
		}
	}
}

func TestFinishTaskRollsBackOnResourcePersistenceFailure(t *testing.T) {
	db := task_contract_database(t)
	task := model.DownloadTask{Id: 42, Name: "task", Status: 4, Timestamps: model.Timestamps{CreatedAt: 1000}}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TRIGGER reject_finish BEFORE UPDATE ON download_resource BEGIN SELECT RAISE(FAIL, 'resource write failed'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.DownloadResource{TaskId: &task.Id}).Error; err != nil {
		t.Fatal(err)
	}
	log := zerolog.Nop()
	if err := database.NewDBTaskStore(db, &log).FinishTask(task.Id); err == nil {
		t.Fatal("completion must fail when resource persistence fails")
	}
	var persisted model.DownloadTask
	if err := db.First(&persisted, task.Id).Error; err != nil || persisted.Status != 4 {
		t.Fatalf("task was incorrectly marked completed: %+v, err=%v", persisted, err)
	}
}

func TestSharedTaskDetailPreservesPlaybackAndEmptyValues(t *testing.T) {
	db := task_contract_database(t)
	download_dir := t.TempDir()
	log := zerolog.Nop()
	service := services.NewDownloadTaskService(db, &log, nil, nil, download_dir, download_dir, nil)
	task := model.DownloadTask{Id: 42, Name: "直播", Status: 2, Timestamps: model.Timestamps{CreatedAt: 1000}}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	resource := model.DownloadResource{Id: 7, TaskId: &task.Id, DownloadDir: download_dir, Name: "直播.mkv", UniqueID: "recording", Type: "stream", Kind: "audio/mp4"}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	playback_dir := filepath.Join(hermes.StreamRecordingDir(filepath.Join(download_dir, resource.UniqueID)), "playback")
	if err := os.MkdirAll(playback_dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(playback_dir, "index.m3u8"), []byte("#EXTM3U\n"), 0600); err != nil {
		t.Fatal(err)
	}
	detail, err := service.BuildTaskDetail(context.Background(), task.Id, nil)
	if err != nil {
		t.Fatal(err)
	}
	file := detail.Files[0]
	if file.Type != "STREAM" || file.FileType != "audio" || !file.PlaybackAvailable || file.PlaybackURL != "/api/v1/download_task/live/42/7/index.m3u8" || file.Exists {
		t.Fatalf("incorrect live detail: %+v", file)
	}
	if _, _, err := service.StreamPlaybackLocation(task, model.DownloadResource{DownloadDir: download_dir, UniqueID: "../escape"}); err == nil {
		t.Fatal("playback output must remain inside a download root")
	}
	if err := db.Delete(&resource).Error; err != nil {
		t.Fatal(err)
	}
	detail, err = service.BuildTaskDetail(context.Background(), task.Id, nil)
	if err != nil || detail.Content != nil || detail.Files == nil || len(detail.Files) != 0 || detail.FileCount != 0 {
		t.Fatalf("incorrect empty detail: %+v, err=%v", detail, err)
	}
	data_service := services.NewDataQueryService(services.DataQueryServiceConfig{DB: db, DownloadTaskService: service})
	missing, err := data_service.GetDownloadTaskDetail(context.Background(), 999)
	if err != nil || missing != nil {
		t.Fatalf("missing task must return nil, got %+v, err=%v", missing, err)
	}
}
