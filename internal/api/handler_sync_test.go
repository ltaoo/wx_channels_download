package api

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

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
	if err := db.AutoMigrate(&model.DownloadTask{}, &model.DownloadResource{}, &model.DownloadEndpoint{}); err != nil {
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
	}{{1, false}, {2, false}, {1, true}, {2, true}} {
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
			if version == 2 {
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
