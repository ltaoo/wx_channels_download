package api

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	"wx_channel/internal/database"
	"wx_channel/internal/database/model"
	"wx_channel/internal/mcpserver"
	"wx_channel/internal/services"
)

type sync_creation_adapter struct {
	adapter.AdapterHandler
	builds int32
}

func (h *sync_creation_adapter) PlatformID() string { return "sync-creation-test" }

func (h *sync_creation_adapter) BuildDownloadTask(raw_content json.RawMessage, raw_config json.RawMessage) (*adapter.DownloadTaskResult, error) {
	atomic.AddInt32(&h.builds, 1)
	content_id := "sync-creation-test:video"
	return &adapter.DownloadTaskResult{
		Task:          &model.DownloadTask{Name: "video", PlatformId: h.PlatformID(), UniqueID: "normal-task", ConfigJSON: string(raw_config)},
		Content:       &model.Content{Id: content_id, PlatformId: h.PlatformID(), ExternalId: "video", Type: "video"},
		Account:       &model.Account{PlatformId: h.PlatformID(), ExternalId: "creator", Nickname: "creator"},
		ContentDetail: &model.ContentVideo{Id: content_id, Duration: 123},
		Resources:     []*adapter.ResourceInfo{{Resource: model.DownloadResource{Name: "video.mp4", Kind: "video/mp4", UniqueID: "normal-resource", ContentId: &content_id}, Endpoints: []model.DownloadEndpoint{{Protocol: "HTTP", URL: "http://example.invalid/expired", Enabled: 1}}}},
	}, nil
}

func seed_sync_test_task(t *testing.T, client *APIClient) {
	t.Helper()
	content_id, task_id := "local-content", 1
	sync_test_insert(t, client,
		&model.Content{Id: content_id, PlatformId: "test", ExternalId: "video", Type: "video", Title: "source title"},
		&model.ContentVideo{Id: content_id, Duration: 123},
		&model.Account{Id: "local-account", PlatformId: "test", ExternalId: "creator", Nickname: "creator"},
		&model.ContentAccount{ContentId: content_id, AccountId: "local-account", Role: "owner"},
		&model.DownloadTask{Id: task_id, PlatformId: "test", UniqueID: "task", Name: "video", ContentId: &content_id, Status: model.TaskStatusFinished, Timestamps: model.Timestamps{CreatedAt: 100}},
		&model.DownloadResource{Id: 1, TaskId: &task_id, ContentId: &content_id, Name: "video.mp4", Kind: "video/mp4", DownloadDir: client.cfg.DownloadDir, UniqueID: "file", Status: model.ResourceStatusFinished, Timestamps: model.Timestamps{CreatedAt: 100}},
	)
	if err := os.WriteFile(filepath.Join(client.cfg.DownloadDir, "video.mp4"), []byte("source bytes"), 0600); err != nil {
		t.Fatal(err)
	}
}

func new_sync_test_client(t *testing.T) *APIClient {
	t.Helper()
	root := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(root, "data.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sql_db, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sql_db.Close() })
	entries, err := database.Migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		raw, err := database.Migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sql_db.Exec(string(raw)); err != nil {
			t.Fatalf("%s: %v", entry.Name(), err)
		}
	}
	return &APIClient{
		db:                    db,
		cfg:                   &APIConfig{WorkDir: root, DBPath: filepath.Join(root, "data.db"), DownloadDir: root},
		service_download_task: services.NewDownloadTaskService(db, nil, nil, nil, root, root, nil),
	}
}

func sync_test_insert(t *testing.T, client *APIClient, records ...any) {
	t.Helper()
	for _, record := range records {
		if err := client.db.Omit(clause.Associations).Create(record).Error; err != nil {
			t.Fatalf("insert %T: %v", record, err)
		}
	}
}

func sync_test_archive(t *testing.T, source *APIClient) []byte {
	t.Helper()
	bundle, files, err := source.build_sync_bundle()
	if err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	if err := write_sync_archive(writer, bundle, files); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestSyncPreservesDownloadCreationTimes(t *testing.T) {
	for _, version := range []int{1, 2} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			source, target := new_sync_test_client(t), new_sync_test_client(t)
			seed_sync_test_task(t, source)
			created_at := int64(1700000000123)
			if err := source.db.Model(&model.DownloadTask{}).Where("id = 1").UpdateColumn("created_at", created_at).Error; err != nil {
				t.Fatal(err)
			}
			if err := source.db.Model(&model.DownloadResource{}).Where("id = 1").UpdateColumn("created_at", created_at+1).Error; err != nil {
				t.Fatal(err)
			}
			// A later download of the same content remains a separate history entry.
			sync_test_insert(t, source, &model.DownloadTask{
				Id: 2, PlatformId: "test", UniqueID: "task", Name: "video",
				Status: model.TaskStatusFinished, Timestamps: model.Timestamps{CreatedAt: created_at + 456},
			})
			// Target IDs differ from source IDs.
			sync_test_insert(t, target, &model.DownloadTask{
				Id: 10, PlatformId: "test", UniqueID: "unrelated", Name: "unrelated",
				Timestamps: model.Timestamps{CreatedAt: created_at - 1},
			})
			bundle, files, err := source.build_sync_bundle()
			if err != nil {
				t.Fatal(err)
			}
			bundle.Version = version
			var buffer bytes.Buffer
			writer := tar.NewWriter(&buffer)
			if err := write_sync_archive(writer, bundle, files); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			for replay := 0; replay < 2; replay++ {
				if _, err := target.import_sync_archive(bytes.NewReader(buffer.Bytes())); err != nil {
					t.Fatal(err)
				}
				var tasks []model.DownloadTask
				if err := target.db.Where("unique_id = ?", "task").Order("created_at").Find(&tasks).Error; err != nil {
					t.Fatal(err)
				}
				if len(tasks) != 2 || tasks[0].CreatedAt != created_at || tasks[1].CreatedAt != created_at+456 {
					t.Fatalf("import %d changed creation times or duplicated history: %+v", replay, tasks)
				}
				var resource model.DownloadResource
				if err := target.db.Where("task_id = ?", tasks[0].Id).First(&resource).Error; err != nil {
					t.Fatal(err)
				}
				if resource.CreatedAt != created_at+1 {
					t.Fatalf("resource creation time = %d, want %d", resource.CreatedAt, created_at+1)
				}
			}
		})
	}
}

func TestSyncRejectsInvalidDownloadCreationTime(t *testing.T) {
	for _, created_at := range []int64{0, -1} {
		t.Run(fmt.Sprint(created_at), func(t *testing.T) {
			source, target := new_sync_test_client(t), new_sync_test_client(t)
			seed_sync_test_task(t, source)
			if err := source.db.Model(&model.DownloadTask{}).Where("id = 1").UpdateColumn("created_at", created_at).Error; err != nil {
				t.Fatal(err)
			}
			_, err := target.import_sync_archive(bytes.NewReader(sync_test_archive(t, source)))
			if err == nil || !strings.Contains(err.Error(), "创建时间无效") {
				t.Fatalf("expected invalid creation time error, got %v", err)
			}
			var count int64
			if err := target.db.Model(&model.DownloadTask{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("failed import left %d tasks: %v", count, err)
			}
		})
	}
}

func TestSyncPreservesContentAndAccountCreationTimes(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing=%t", existing), func(t *testing.T) {
			source, target := new_sync_test_client(t), new_sync_test_client(t)
			seed_sync_test_task(t, source)
			created_at := int64(1700000000123)
			for table, offset := range map[string]int64{"content": 1, "account": 2} {
				if err := source.db.Table(table).Where("1 = 1").UpdateColumn("created_at", created_at+offset).Error; err != nil {
					t.Fatal(err)
				}
			}
			sync_test_insert(t, source,
				&model.Content{Id: "related-content", PlatformId: "test", ExternalId: "related", Type: "series", Timestamps: model.Timestamps{CreatedAt: created_at + 3}},
				&model.ContentRelation{SourceContentId: "local-content", TargetContentId: "related-content", Type: "part_of"},
			)
			content_time, account_time, related_time := created_at+1, created_at+2, created_at+3
			if existing {
				content_time, account_time, related_time = created_at-101, created_at-102, created_at-103
				sync_test_insert(t, target,
					&model.Content{Id: "nas-content", PlatformId: "test", ExternalId: "video", Timestamps: model.Timestamps{CreatedAt: content_time}},
					&model.Content{Id: "nas-related", PlatformId: "test", ExternalId: "related", Timestamps: model.Timestamps{CreatedAt: related_time}},
					&model.Account{Id: "nas-account", PlatformId: "test", ExternalId: "creator", Timestamps: model.Timestamps{CreatedAt: account_time}},
				)
			}
			archive := sync_test_archive(t, source)
			for replay := 0; replay < 2; replay++ {
				if _, err := target.import_sync_archive(bytes.NewReader(archive)); err != nil {
					t.Fatal(err)
				}
				for external_id, expected := range map[string]int64{"video": content_time, "related": related_time} {
					var content model.Content
					if err := target.db.Where("platform_id = ? AND external_id = ?", "test", external_id).First(&content).Error; err != nil {
						t.Fatal(err)
					}
					if content.CreatedAt != expected {
						t.Fatalf("content %s creation time = %d, want %d", external_id, content.CreatedAt, expected)
					}
				}
				var account model.Account
				if err := target.db.Where("platform_id = ? AND external_id = ?", "test", "creator").First(&account).Error; err != nil {
					t.Fatal(err)
				}
				if account.CreatedAt != account_time {
					t.Fatalf("account creation time = %d, want %d", account.CreatedAt, account_time)
				}
			}
		})
	}
}

func TestSyncMCPBackgroundRoundTripWithoutToken(t *testing.T) {
	source, target := new_sync_test_client(t), new_sync_test_client(t)
	previous_token, previous_servers := viper.Get("sync.token"), viper.Get("sync.servers")
	t.Cleanup(func() { viper.Set("sync.token", previous_token); viper.Set("sync.servers", previous_servers) })
	viper.Set("sync.token", "")
	source.cfg.Original = &config.Config{FullPath: filepath.Join(t.TempDir(), "config.yaml")}
	target.cfg.Original = &config.Config{}
	seed_sync_test_task(t, source)
	target_router := gin.New()
	target_router.GET("/api/v1/sync/info", target.handle_sync_info)
	target_router.POST("/api/v1/sync/import", func(ctx *gin.Context) {
		if ctx.GetHeader("Authorization") != "" {
			t.Error("tokenless sync sent an Authorization header")
		}
		target.handle_sync_import(ctx)
	})
	target_server := httptest.NewServer(target_router)
	defer target_server.Close()
	source_router := gin.New()
	source_router.GET("/api/v1/sync/servers", source.handle_sync_servers)
	source_router.POST("/api/v1/sync/servers", source.handle_sync_configure)
	source_router.GET("/api/v1/sync/preview", source.handle_sync_preview)
	source_router.POST("/api/v1/sync/servers/:server_id/jobs", source.handle_sync_start)
	source_router.GET("/api/v1/sync/jobs/:job_id", source.handle_sync_job)
	source_server := httptest.NewServer(source_router)
	defer source_server.Close()
	tools, err := mcpserver.NewToolSet(mcpserver.Config{APIBaseURL: source_server.URL})
	if err != nil {
		t.Fatal(err)
	}
	call := func(name string, arguments map[string]any) json.RawMessage {
		t.Helper()
		response, err := tools.ExecuteTool(context.Background(), name, arguments)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	call("configure_sync_server", map[string]any{"name": "NAS", "url": target_server.URL})
	call("get_sync_servers", map[string]any{})
	preview := call("preview_sync", map[string]any{})
	if !bytes.Contains(preview, []byte(`"accounts":1`)) {
		t.Fatalf("preview: %s", preview)
	}
	var job sync_job
	if err := json.Unmarshal(call("sync_to_server", map[string]any{"server_id": 0, "task_ids": []int{1}}), &job); err != nil {
		t.Fatal(err)
	}
	if job.ID == "" {
		t.Fatal("missing job ID")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := json.Unmarshal(call("get_sync_status", map[string]any{"job_id": job.ID}), &job); err != nil {
			t.Fatal(err)
		}
		if job.Status == "completed" {
			break
		}
		if job.Status == "failed" {
			t.Fatalf("sync failed: %s", job.Error)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job.Status != "completed" || job.Result == nil || job.Result.Accounts != 1 {
		t.Fatalf("job: %+v", job)
	}
	var count int64
	if err := target.db.Model(&model.Account{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("target: %d %v", count, err)
	}
}

func TestSyncImportNeedsNoTokenEvenWithLegacyConfiguration(t *testing.T) {
	source, target := new_sync_test_client(t), new_sync_test_client(t)
	seed_sync_test_task(t, source)
	archive := sync_test_archive(t, source)
	previous_token := viper.Get("sync.token")
	t.Cleanup(func() { viper.Set("sync.token", previous_token) })
	for _, legacy_token := range []string{"", "legacy-token-is-ignored"} {
		viper.Set("sync.token", legacy_token)
		target.cfg.Original = &config.Config{}
		router := gin.New()
		router.GET("/api/v1/sync/info", target.handle_sync_info)
		router.POST("/api/v1/sync/import", target.handle_sync_import)
		info := httptest.NewRecorder()
		router.ServeHTTP(info, httptest.NewRequest(http.MethodGet, "/api/v1/sync/info", nil))
		if !bytes.Contains(info.Body.Bytes(), []byte(`"receive_enabled":true`)) {
			t.Fatalf("sync reception disabled: %s", info.Body.String())
		}
		request := httptest.NewRequest(http.MethodPost, "/api/v1/sync/import", bytes.NewReader(archive))
		request.Header.Set("Content-Type", "application/x-tar")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		var envelope struct {
			Code int                `json:"code"`
			Data sync_import_result `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Code != 0 || envelope.Data.Files != 1 {
			t.Fatalf("import without Authorization: %s", response.Body.String())
		}
	}
}

func TestSyncLegacyFileFailureRollsBackRecordsAndPreservesDirectory(t *testing.T) {
	target := new_sync_test_client(t)
	destination := filepath.Join(target.cfg.DownloadDir, "occupied")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	bundle := sync_bundle{Version: 1, Contents: []model.Content{{Id: "legacy-content", PlatformId: "test", ExternalId: "legacy"}}, Resources: []model.DownloadResource{{Id: 1, Name: "occupied", Timestamps: model.Timestamps{CreatedAt: 1}}}}
	raw, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	if err := writer.WriteHeader(&tar.Header{Name: "metadata.json", Mode: 0600, Size: int64(len(raw))}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteHeader(&tar.Header{Name: "files/1/final/file", Mode: 0600, Size: 4}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("data")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := target.import_sync_archive(bytes.NewReader(buffer.Bytes())); err == nil {
		t.Fatal("expected file installation failure")
	}
	info, err := os.Stat(destination)
	if err != nil || !info.IsDir() {
		t.Fatalf("existing directory replaced: %v", err)
	}
	var count int64
	if err := target.db.Model(&model.Content{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("metadata was not rolled back: %d %v", count, err)
	}
	if err := os.Remove(destination); err != nil {
		t.Fatal(err)
	}
	if _, err := target.import_sync_archive(bytes.NewReader(buffer.Bytes())); err != nil {
		t.Fatalf("legacy import: %v", err)
	}
	raw, err = os.ReadFile(destination)
	if err != nil || string(raw) != "data" {
		t.Fatalf("legacy file: %q %v", raw, err)
	}
}

func TestSyncRecreatesTaskWithoutDownloadingOrUnrelatedData(t *testing.T) {
	source, target := new_sync_test_client(t), new_sync_test_client(t)
	seed_sync_test_task(t, source)
	var downloads int32
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		atomic.AddInt32(&downloads, 1)
		writer.WriteHeader(500)
	}))
	defer remote.Close()
	parent_id, existing_id, unrelated_id := 4, 9, 1
	sync_test_insert(t, source,
		&model.DownloadEndpoint{Id: 1, ResourceId: 1, Protocol: "http", URL: remote.URL + "/expired-video"},
		&model.Account{Id: "unrelated-account", PlatformId: "test", ExternalId: "unrelated"},
		&model.Content{Id: "unrelated-content", PlatformId: "test", ExternalId: "unrelated"},
		&model.BrowseHistory{Id: "history", PlatformId: "test", ExternalId: "video", VisitedTimes: 7},
		&model.DownloadTask{Id: parent_id, PlatformId: "test", UniqueID: "parent", Name: "parent", Status: model.TaskStatusFinished, Timestamps: model.Timestamps{CreatedAt: 50}},
	)
	if err := source.db.Model(&model.DownloadTask{}).Where("id = 1").Update("parent_task_id", parent_id).Error; err != nil {
		t.Fatal(err)
	}
	sync_test_insert(t, target,
		&model.Content{Id: "local-content", PlatformId: "test", ExternalId: "unrelated", Title: "preserve"},
		&model.Content{Id: "nas-content", PlatformId: "test", ExternalId: "video"},
		&model.Account{Id: "nas-account", PlatformId: "test", ExternalId: "creator"},
		&model.DownloadTask{Id: unrelated_id, PlatformId: "test", UniqueID: "unrelated", Name: "preserve", Timestamps: model.Timestamps{CreatedAt: 2}},
		// The same archived task can have a different target ID.
		&model.DownloadTask{Id: existing_id, PlatformId: "test", UniqueID: "task", Name: "video", Timestamps: model.Timestamps{CreatedAt: 100}},
		&model.DownloadResource{Id: 1, TaskId: &unrelated_id, Name: "video.mp4", DownloadDir: target.cfg.DownloadDir, UniqueID: "unrelated", Timestamps: model.Timestamps{CreatedAt: 2}},
		&model.DownloadResource{Id: 9, TaskId: &existing_id, Name: "video.mp4", DownloadDir: target.cfg.DownloadDir, UniqueID: "file", Timestamps: model.Timestamps{CreatedAt: 100}},
	)
	if err := os.WriteFile(filepath.Join(target.cfg.DownloadDir, "video.mp4"), []byte("preserve bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	archive := sync_test_archive(t, source)
	for replay := 0; replay < 2; replay++ {
		imported, err := target.import_sync_archive(bytes.NewReader(archive))
		if err != nil {
			t.Fatal(err)
		}
		if imported.Tasks != 2 || imported.Accounts != 1 || imported.Contents != 1 || imported.Files != 1 {
			t.Fatalf("counts: %+v", imported)
		}
	}
	if atomic.LoadInt32(&downloads) != 0 {
		t.Fatal("sync started a remote download")
	}
	for table, expected := range map[string]int64{"download_task": 3, "download_resource": 2, "content": 2, "account": 1, "browse_history": 0, "download_endpoint": 1} {
		var count int64
		if err := target.db.Table(table).Count(&count).Error; err != nil || count != expected {
			t.Fatalf("%s: count=%d want=%d err=%v", table, count, expected, err)
		}
	}
	var task model.DownloadTask
	if err := target.db.First(&task, existing_id).Error; err != nil {
		t.Fatal(err)
	}
	if task.Status != model.TaskStatusFinished || task.ParentTaskID == nil || *task.ParentTaskID == parent_id || task.RootTaskID != *task.ParentTaskID || task.ContentId == nil || *task.ContentId != "nas-content" {
		t.Fatalf("task links: %+v", task)
	}
	var resource model.DownloadResource
	if err := target.db.First(&resource, 9).Error; err != nil {
		t.Fatal(err)
	}
	if resource.Status != model.ResourceStatusFinished || resource.TaskId == nil || *resource.TaskId != existing_id || resource.ContentId == nil || *resource.ContentId != "nas-content" {
		t.Fatalf("resource links: %+v", resource)
	}
	raw, err := os.ReadFile(filepath.Join(resource.DownloadDir, resource.Name))
	if err != nil || string(raw) != "source bytes" {
		t.Fatalf("copied file: %q %v", raw, err)
	}
	raw, err = os.ReadFile(filepath.Join(target.cfg.DownloadDir, "video.mp4"))
	if err != nil || string(raw) != "preserve bytes" {
		t.Fatalf("unrelated file replaced: %q %v", raw, err)
	}
	var count int64
	if err := target.db.Model(&model.ContentAccount{}).Where("content_id = ? AND account_id = ?", "nas-content", "nas-account").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("account link: %d %v", count, err)
	}
	if err := target.db.Model(&model.DownloadResourceAsset{}).Where("resource_id = ?", resource.Id).Count(&count).Error; err != nil || count == 0 {
		t.Fatalf("shared creation did not create content assets: %d %v", count, err)
	}
}

func TestSyncMissingFileIsPausedAndDoesNotDownload(t *testing.T) {
	source, target := new_sync_test_client(t), new_sync_test_client(t)
	seed_sync_test_task(t, source)
	if err := os.Remove(filepath.Join(source.cfg.DownloadDir, "video.mp4")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source.cfg.DownloadDir, "video.mp4.part"), []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	imported, err := target.import_sync_archive(bytes.NewReader(sync_test_archive(t, source)))
	if err != nil {
		t.Fatal(err)
	}
	if imported.Files != 0 {
		t.Fatalf("copied partial file: %+v", imported)
	}
	var resource model.DownloadResource
	if err := target.db.First(&resource).Error; err != nil {
		t.Fatal(err)
	}
	if resource.Status != model.ResourceStatusWaiting || resource.Downloaded != 0 {
		t.Fatalf("missing file marked complete: %+v", resource)
	}
	var task model.DownloadTask
	if err := target.db.First(&task).Error; err != nil || task.Status != model.TaskStatusPaused {
		t.Fatalf("task should remain paused: %+v %v", task, err)
	}
}

func TestSyncCreationFailureRollsBackFilesAndRecords(t *testing.T) {
	source, target := new_sync_test_client(t), new_sync_test_client(t)
	seed_sync_test_task(t, source)
	// Ordinary task creation validates account identity. Sync must use the same
	// validation and roll back the staged file if record creation fails.
	if err := source.db.Model(&model.Account{}).Where("id = ?", "local-account").Update("platform_id", "").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := target.import_sync_archive(bytes.NewReader(sync_test_archive(t, source))); err == nil {
		t.Fatal("expected account validation failure")
	}
	for _, table := range []string{"download_task", "download_resource", "content", "account"} {
		var count int64
		if err := target.db.Table(table).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%s was not rolled back: %d %v", table, count, err)
		}
	}
	_ = filepath.Walk(target.cfg.DownloadDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && (info.Name() == "video.mp4" || strings.HasPrefix(info.Name(), ".sync-")) {
			t.Errorf("failed import left file: %s", path)
		}
		return err
	})
}

func TestSyncExistingDatabaseTaskRecords(t *testing.T) {
	source_path := os.Getenv("WX_SYNC_FIXTURE_DB")
	if source_path == "" {
		t.Skip("set WX_SYNC_FIXTURE_DB for read-only existing-data regression")
	}
	source_db, err := gorm.Open(sqlite.Open("file:"+source_path+"?mode=ro"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sql_db, err := source_db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sql_db.Close()
	bundle := sync_bundle{Version: 2}
	if err := source_db.Transaction(func(tx *gorm.DB) error { return build_sync_task_records(tx, &bundle) }); err != nil {
		t.Fatal(err)
	}
	target := new_sync_test_client(t)
	for replay := 0; replay < 2; replay++ {
		if err := target.db.Transaction(func(tx *gorm.DB) error { return target.import_sync_records(tx, bundle, &sync_import_result{}) }); err != nil {
			t.Fatal(err)
		}
	}
	for table, expected := range sync_record_counts(bundle) {
		var count int64
		if err := target.db.Table(table).Count(&count).Error; err != nil || int(count) != expected {
			t.Fatalf("%s: %d want=%d err=%v", table, count, expected, err)
		}
	}
	var history_count int64
	if err := target.db.Model(&model.BrowseHistory{}).Count(&history_count).Error; err != nil || history_count != 0 {
		t.Fatalf("copied browsing history: %d %v", history_count, err)
	}
}

func TestSyncRecreatesOrdinarilyCreatedTaskWithoutAdapterBuild(t *testing.T) {
	source, target := new_sync_test_client(t), new_sync_test_client(t)
	handler := &sync_creation_adapter{}
	adapter.Register(handler)
	service := services.NewDownloadTaskService(source.db, nil, nil, nil, source.cfg.WorkDir, source.cfg.DownloadDir, nil)
	auto_start := false
	created, err := service.CreateTask(services.CreateDownloadTaskBody{Platform: handler.PlatformID(), Content: json.RawMessage(`{}`), AutoStart: &auto_start})
	if err != nil {
		t.Fatal(err)
	}
	if created.Task.Id <= 0 || len(created.Resources) != 1 || created.Content == nil || created.Account == nil {
		t.Fatalf("ordinary creation: %+v", created)
	}
	if err := source.db.Model(&model.DownloadTask{}).Where("id = ?", created.Task.Id).Update("status", model.TaskStatusFinished).Error; err != nil {
		t.Fatal(err)
	}
	if err := source.db.Model(&model.DownloadResource{}).Where("task_id = ?", created.Task.Id).Update("status", model.ResourceStatusFinished).Error; err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source.cfg.DownloadDir, "video.mp4"), []byte("already downloaded"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := target.import_sync_archive(bytes.NewReader(sync_test_archive(t, source))); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&handler.builds) != 1 {
		t.Fatal("sync rebuilt the platform task instead of reusing saved inputs")
	}
	for _, table := range []string{"download_task", "download_resource", "content", "account", "content_account", "content_video", "download_endpoint"} {
		var count int64
		if err := target.db.Table(table).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s: %d %v", table, count, err)
		}
	}
}

func TestSyncSelectedTaskDoesNotExpandToParentOrOtherTasks(t *testing.T) {
	source, target := new_sync_test_client(t), new_sync_test_client(t)
	seed_sync_test_task(t, source)
	parent_id, child_id := 1, 2
	sync_test_insert(t, source,
		&model.DownloadTask{Id: child_id, ParentTaskID: &parent_id, RootTaskID: parent_id, RelationType: model.TaskRelationDiscovered, PlatformId: "test", UniqueID: "selected", Name: "selected", Status: model.TaskStatusFinished, Timestamps: model.Timestamps{CreatedAt: 200}},
		&model.DownloadResource{Id: 2, TaskId: &child_id, Name: "selected.txt", DownloadDir: source.cfg.DownloadDir, UniqueID: "selected-file", Status: model.ResourceStatusFinished, Timestamps: model.Timestamps{CreatedAt: 200}},
	)
	if err := os.WriteFile(filepath.Join(source.cfg.DownloadDir, "selected.txt"), []byte("selected only"), 0600); err != nil {
		t.Fatal(err)
	}
	bundle, files, err := source.build_sync_bundle(child_id)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Tasks) != 1 || bundle.Tasks[0].Id != child_id || bundle.Tasks[0].ParentTaskID != nil || len(bundle.Resources) != 1 || len(bundle.Contents) != 0 || len(files) != 1 {
		t.Fatalf("selection expanded: tasks=%d resources=%d contents=%d files=%d", len(bundle.Tasks), len(bundle.Resources), len(bundle.Contents), len(files))
	}
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	if err := write_sync_archive(writer, bundle, files); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	result, err := target.import_sync_archive(bytes.NewReader(buffer.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if result.Tasks != 1 || result.Files != 1 || len(result.TargetTaskIDs) != 1 {
		t.Fatalf("unexpected import: %+v", result)
	}
	var task model.DownloadTask
	if err := target.db.First(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.UniqueID != "selected" || task.ParentTaskID != nil {
		t.Fatalf("wrong task: %+v", task)
	}
	for _, table := range []string{"content", "account"} {
		var count int64
		if err := target.db.Table(table).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("unrelated %s copied: %d %v", table, count, err)
		}
	}
	if _, _, err := source.build_sync_bundle(999999); err == nil {
		t.Fatal("missing selection fell back to all tasks")
	}
}
