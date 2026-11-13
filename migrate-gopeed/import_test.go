package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"wx_channel/internal/adapter"
	wxchannelsadapter "wx_channel/internal/adapter/wxchannels"
	"wx_channel/internal/database/model"
)

const gopeed_cached_profile = `{
		"id":"12345", "objectNonceId":"nonce",
		"contact":{"username":"author", "nickname":"Author"},
		"objectDesc":{"description":"Video", "mediaType":4, "media":[{
			"url":"https://example.com/video.mp4", "videoPlayLen":12,
			"width":1920, "height":1080, "fileSize":5
		}]}
	}`

func current_gopeed_download_info(t *testing.T) *adapter.DownloadTaskResult {
	t.Helper()
	info, err := wxchannelsadapter.NewChannelsAdapter().BuildDownloadTask(json.RawMessage(gopeed_cached_profile), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func TestGopeedImportsQuotedCachedJSONWithoutDownloading(t *testing.T) {
	directory := t.TempDir()
	db, err := open_target_db(filepath.Join(directory, "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer close_target_db(db)
	save_path := filepath.Join(directory, "video.mp4")
	if err := os.WriteFile(save_path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	var download_requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		download_requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	task := must_gopeed_migration_task(t, "quoted-cache", directory, "video.mp4", nil)
	task.Meta.Req.URL = server.URL + "/video.mp4"
	encoded, err := json.Marshal(gopeed_cached_profile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := import_target_download_record(db, task, nil, "12345", "nonce", save_path, encoded); err != nil {
		t.Fatal(err)
	}
	if count := download_requests.Load(); count != 0 {
		t.Fatalf("migration started %d download requests", count)
	}
}

func TestGopeedRecreationUsesCurrentDatabaseAndAssets(t *testing.T) {
	directory := t.TempDir()
	db_path := filepath.Join(directory, "data.db")
	db, err := open_target_db(db_path)
	if err != nil {
		t.Fatal(err)
	}
	close_target_db(db)
	db, err = open_target_db(db_path)
	if err != nil {
		t.Fatal(err)
	}
	defer close_target_db(db)
	var version int
	if err := db.Raw("SELECT version FROM schema_migrations").Scan(&version).Error; err != nil || version != 3 {
		t.Fatalf("current schema version = %d: %v", version, err)
	}
	save_path := filepath.Join(directory, "video.mp4")
	if err := os.WriteFile(save_path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	task := must_gopeed_migration_task(t, "asset-task", directory, "video.mp4", nil)
	target_id, err := recreate_gopeed_task(db, task, nil, platform_wx_channels, "12345", "", save_path, current_gopeed_download_info(t))
	if err != nil {
		t.Fatal(err)
	}
	var resource model.DownloadResource
	if err := db.Where("task_id = ?", target_id).First(&resource).Error; err != nil {
		t.Fatal(err)
	}
	var linked int64
	if err := db.Raw(`SELECT COUNT(*) FROM download_resource_asset AS link
		JOIN content_asset AS asset ON asset.id = link.asset_id
		JOIN content_video_variant AS variant ON variant.asset_id = asset.id
		WHERE link.resource_id = ? AND asset.content_id = ?`, resource.Id, "wxchannels:12345").Scan(&linked).Error; err != nil || linked != 1 {
		t.Fatalf("download file has %d video asset links: %v", linked, err)
	}
	if resource.CreatedAt != task.CreatedAt.UnixMilli() || resource.Name != "video.mp4" || resource.Status != model.ResourceStatusFinished {
		t.Fatalf("file history was not restored: %+v", resource)
	}
}

func TestGopeedRecreationPausesMissingFile(t *testing.T) {
	directory := t.TempDir()
	db, err := open_target_db(filepath.Join(directory, "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer close_target_db(db)
	task := must_gopeed_migration_task(t, "missing-file", directory, "video.mp4", nil)
	target_id, err := recreate_gopeed_task(db, task, nil, platform_wx_channels, "12345", "", filepath.Join(directory, "video.mp4"), current_gopeed_download_info(t))
	if err != nil {
		t.Fatal(err)
	}
	var target model.DownloadTask
	if err := db.First(&target, target_id).Error; err != nil {
		t.Fatal(err)
	}
	var resource model.DownloadResource
	if err := db.Where("task_id = ?", target_id).First(&resource).Error; err != nil {
		t.Fatal(err)
	}
	if target.Status != model.TaskStatusPaused || resource.Status != model.ResourceStatusWaiting || resource.FinishTime != nil || resource.Downloaded != 0 {
		t.Fatalf("missing file was marked as finished: task=%+v resource=%+v", target, resource)
	}
}

func TestGopeedRecreationRollsBackWholeGraph(t *testing.T) {
	directory := t.TempDir()
	db, err := open_target_db(filepath.Join(directory, "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer close_target_db(db)
	if err := db.Exec(`CREATE TRIGGER reject_endpoint BEFORE INSERT ON download_endpoint
		BEGIN SELECT RAISE(ABORT, 'endpoint rejected'); END`).Error; err != nil {
		t.Fatal(err)
	}
	save_path := filepath.Join(directory, "video.mp4")
	if err := os.WriteFile(save_path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	task := must_gopeed_migration_task(t, "rollback-task", directory, "video.mp4", nil)
	if _, err := recreate_gopeed_task(db, task, nil, platform_wx_channels, "12345", "", save_path, current_gopeed_download_info(t)); err == nil {
		t.Fatal("endpoint failure was ignored")
	}
	for _, table := range []string{"download_task", "download_resource", "download_endpoint", "content", "content_video", "account", "content_account", "content_asset", "download_resource_asset"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%s retained %d records after rollback: %v", table, count, err)
		}
	}
	file, err := os.ReadFile(save_path)
	if err != nil || string(file) != "hello" {
		t.Fatalf("source file changed: %q %v", file, err)
	}
}
