package api

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/services"
)

func new_sync_test_client(t *testing.T, root string) *APIClient {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	connection.SetMaxOpenConns(1)
	t.Cleanup(func() { connection.Close() })
	if err := db.AutoMigrate(&model.DownloadTask{}, &model.DownloadResource{}, &model.DownloadEndpoint{}, &model.DownloadResourceAsset{}); err != nil {
		t.Fatal(err)
	}
	return &APIClient{cfg: &APIConfig{DownloadDir: root}, db: db,
		service_download_task: services.NewDownloadTaskService(db, nil, nil, nil, root, root, nil)}
}

func sync_test_archive(t *testing.T, bundle sync_bundle, files map[string]string) *bytes.Buffer {
	t.Helper()
	metadata, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	write := func(name string, data []byte) {
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	write("metadata.json", metadata)
	for name, content := range files {
		write(name, []byte(content))
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &buffer
}

func TestSyncImportRejectsSymlinkPaths(t *testing.T) {
	for _, link_type := range []string{"parent", "file"} {
		t.Run(link_type, func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			victim := filepath.Join(outside, "victim.txt")
			if err := os.WriteFile(victim, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			name, target := "linked/victim.txt", outside
			if link_type == "file" {
				name, target = "linked", victim
			}
			if err := os.Symlink(target, filepath.Join(root, "linked")); err != nil {
				t.Fatal(err)
			}
			client := new_sync_test_client(t, root)
			bundle := sync_bundle{Version: 1, Resources: []model.DownloadResource{{Id: 1, Name: name, Timestamps: model.Timestamps{CreatedAt: 1}}}}
			if _, err := client.import_sync_archive(sync_test_archive(t, bundle, map[string]string{"files/1/final/file": "replacement"})); err == nil {
				t.Fatal("symlink import was accepted")
			}
			content, err := os.ReadFile(victim)
			if err != nil || string(content) != "original" {
				t.Fatalf("outside file changed: content=%q error=%v", content, err)
			}
		})
	}
}

func TestSyncInstallRechecksPaths(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	final_path := filepath.Join(root, "linked", "victim.txt")
	if _, err := path_within_root(root, final_path); err != nil {
		t.Fatal(err)
	}
	temp_path := filepath.Join(root, "staged")
	if err := os.WriteFile(temp_path, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := install_sync_files(root, []sync_file_stage{{TempPath: temp_path, FinalPath: final_path}}); err == nil {
		t.Fatal("installation followed a symlink inserted after staging")
	}
	if _, err := os.Stat(filepath.Join(outside, "victim.txt")); !os.IsNotExist(err) {
		t.Fatalf("installation wrote outside the download root: %v", err)
	}
}

func TestSyncStreamPlaybackUsesImportedFiles(t *testing.T) {
	for _, test_case := range []struct {
		version   int
		duplicate bool
	}{{1, false}, {2, false}, {3, false}, {1, true}, {2, true}, {3, true}} {
		t.Run(fmt.Sprintf("version-%d/duplicate-%t", test_case.version, test_case.duplicate), func(t *testing.T) {
			version := test_case.version
			root := t.TempDir()
			alias := filepath.Join(t.TempDir(), "download-root")
			if err := os.Symlink(root, alias); err != nil {
				t.Fatal(err)
			}
			client := new_sync_test_client(t, alias)
			task_id := 10
			task := model.DownloadTask{Id: task_id, Name: "live", PlatformId: "wxchannels", Status: model.TaskStatusFinished, Timestamps: model.Timestamps{CreatedAt: 1}}
			if test_case.duplicate {
				task.ConfigJSON = `{"duplicate":true}`
			}
			resource := model.DownloadResource{Id: 1, TaskId: &task_id, Name: "title.mp4", UniqueID: "live-123", Type: model.ResourceTypeStream, Status: model.ResourceStatusFinished, Timestamps: model.Timestamps{CreatedAt: 1}}
			bundle := sync_bundle{Version: version, SourceID: "test", Tasks: []model.DownloadTask{task}, Resources: []model.DownloadResource{resource}}
			if version >= 2 {
				bundle.Records = []*services.DownloadTaskArchive{{Task: task, Resources: []*adapter.ResourceInfo{{Resource: resource}}}}
			}
			for attempt := 0; attempt < 2; attempt++ {
				if _, err := client.import_sync_archive(sync_test_archive(t, bundle, map[string]string{
					"files/1/final/file": "video", "files/1/playback/index.m3u8": "#EXTM3U", "files/1/playback/segment_000000.ts": "segment",
				})); err != nil {
					t.Fatal(err)
				}
			}
			var imported model.DownloadResource
			if err := client.db.Where("unique_id = ?", resource.UniqueID).First(&imported).Error; err != nil {
				t.Fatal(err)
			}
			asset, err := client.resolve_stream_playback_asset(*imported.TaskId, imported.Id, "index.m3u8")
			if err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(asset)
			if err != nil || string(content) != "#EXTM3U" {
				t.Fatalf("imported playback unavailable: content=%q error=%v", content, err)
			}
		})
	}
}

func TestSyncCommitsEachTaskBeforeSendingTheNext(t *testing.T) {
	for _, fail_second := range []bool{false, true} {
		t.Run(fmt.Sprintf("fail-second-%t", fail_second), func(t *testing.T) {
			source := new_sync_test_client(t, t.TempDir())
			target := new_sync_test_client(t, t.TempDir())
			parent_id := 20
			// The child sorts first, so sending in ID order would lose its parent.
			for _, task := range []model.DownloadTask{
				{Id: parent_id, Name: "parent", PlatformId: "douyin", UniqueID: "parent", Status: model.TaskStatusFinished, Timestamps: model.Timestamps{CreatedAt: 1}},
				{Id: 10, Name: "child", PlatformId: "douyin", UniqueID: "child", ParentTaskID: &parent_id, RootTaskID: parent_id, RelationType: model.TaskRelationDiscovered, Status: model.TaskStatusFinished, Timestamps: model.Timestamps{CreatedAt: 2}},
			} {
				if err := source.db.Create(&task).Error; err != nil {
					t.Fatal(err)
				}
				name := task.Name + ".mp4"
				if err := os.WriteFile(filepath.Join(source.cfg.DownloadDir, name), []byte(task.Name), 0600); err != nil {
					t.Fatal(err)
				}
				resource := model.DownloadResource{Id: task.Id, TaskId: &task.Id, UniqueID: task.UniqueID, Name: name, DownloadDir: source.cfg.DownloadDir, Status: model.ResourceStatusFinished, Timestamps: model.Timestamps{CreatedAt: task.CreatedAt}}
				if err := source.db.Create(&resource).Error; err != nil {
					t.Fatal(err)
				}
			}
			blocked, release := make(chan struct{}), make(chan struct{})
			var release_once sync.Once
			var imports atomic.Int32
			router := gin.New()
			router.GET("/api/v1/sync/info", target.handle_sync_info)
			router.POST("/api/v1/sync/import", func(ctx *gin.Context) {
				if imports.Add(1) == 2 {
					close(blocked)
					<-release
					if fail_second {
						_, _ = io.Copy(io.Discard, ctx.Request.Body)
						ctx.JSON(200, gin.H{"code": 500, "msg": "second task failed"})
						return
					}
				}
				target.handle_sync_import(ctx)
			})
			upstream := httptest.NewServer(router)
			defer upstream.Close()
			defer release_once.Do(func() { close(release) })
			server := sync_server_config{Name: "test", URL: upstream.URL}
			type outcome struct {
				result *sync_import_result
				err    error
			}
			done := make(chan outcome, 1)
			progress := make(chan *sync_import_result, 8)
			go func() {
				imported, err := source.send_sync_bundle(httptest.NewRequest(http.MethodPost, "/", nil), server, func(total, current_task_id int, imported *sync_import_result) {
					if total != 2 {
						t.Errorf("total=%d, want 2", total)
					}
					progress <- imported
				})
				done <- outcome{imported, err}
			}()
			select {
			case <-blocked:
			case result := <-done:
				t.Fatalf("sync stopped before second task: %+v", result)
			case <-time.After(10 * time.Second):
				t.Fatal("second task was never sent")
			}
			var parent model.DownloadTask
			if err := target.db.Where("unique_id = ?", "parent").First(&parent).Error; err != nil {
				t.Fatalf("first task not visible while second is uploading: %v", err)
			}
			var resource model.DownloadResource
			if err := target.db.Where("task_id = ?", parent.Id).First(&resource).Error; err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(filepath.Join(resource.DownloadDir, resource.Name))
			if err != nil || string(content) != "parent" || parent.Status != model.TaskStatusFinished {
				t.Fatalf("first task's file/state not committed: content=%q status=%d error=%v", content, parent.Status, err)
			}
			// Earlier snapshots must not change as subsequent tasks finish.
			initial, first := <-progress, <-progress
			if initial.Tasks != 0 || first.Tasks != 1 || len(first.TargetTaskIDs) != 1 {
				t.Fatalf("unexpected progress: initial=%+v first=%+v", initial, first)
			}
			release_once.Do(func() { close(release) })
			var result outcome
			select {
			case result = <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("sync did not finish")
			}
			if fail_second {
				if result.err == nil || !strings.Contains(result.err.Error(), "已完成 1/2") || result.result.Tasks != 1 {
					t.Fatalf("partial failure lost confirmed result: %+v", result)
				}
				var count int64
				target.db.Model(&model.DownloadTask{}).Count(&count)
				if count != 1 {
					t.Fatalf("failed task was committed: count=%d", count)
				}
				result.result, result.err = source.send_sync_bundle(httptest.NewRequest(http.MethodPost, "/", nil), server, nil)
			}
			if result.err != nil || result.result.Tasks != 2 || result.result.Files != 2 || result.result.Bytes != int64(len("parentchild")) || len(result.result.TargetTaskIDs) != 2 {
				t.Fatalf("wrong final result: result=%+v error=%v", result.result, result.err)
			}
			var child model.DownloadTask
			if err := target.db.Where("unique_id = ?", "child").First(&child).Error; err != nil {
				t.Fatal(err)
			}
			if child.ParentTaskID == nil || *child.ParentTaskID != parent.Id || child.RootTaskID != parent.Id {
				t.Fatalf("lineage lost across requests: parent=%+v child=%+v", parent, child)
			}
			var count int64
			if err := target.db.Model(&model.DownloadTask{}).Count(&count).Error; err != nil || count != 2 {
				t.Fatalf("retry created duplicates: count=%d error=%v", count, err)
			}
		})
	}
}

func TestSyncRejectsOldReceiverBeforeSending(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/sync/info" {
			t.Errorf("sent data to an old receiver: %s", request.URL.Path)
		}
		_, _ = io.WriteString(w, `{"code":0,"data":{"version":2,"receive_enabled":true}}`)
	}))
	defer upstream.Close()
	if err := check_sync_receiver(context.Background(), sync_server_config{URL: upstream.URL}); err == nil {
		t.Fatal("old receiver accepted for incremental sync")
	}
}
