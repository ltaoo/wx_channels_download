package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"

	"wx_channel/internal/database/model"
)

func TestImportTargetDownloadRecordWritesDirectRecords(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "demo.mp4")
	if err := os.WriteFile(savePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write migrated file: %v", err)
	}

	db, err := openTargetDB(filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("open target db: %v", err)
	}
	defer closeTargetDB(db)

	createdAt := time.Unix(1710000000, 0)
	task := &downloadpkg.Task{
		ID:        "bolt-task-1",
		Protocol:  "http",
		Status:    base.DownloadStatusDone,
		Progress:  &downloadpkg.Progress{Downloaded: 5},
		CreatedAt: createdAt,
		UpdatedAt: createdAt.Add(time.Minute),
	}
	labels := map[string]string{
		"title":  "迁移标题",
		"spec":   "xWT111",
		"suffix": ".mp4",
	}
	object := json.RawMessage(`{
		"id": "12345",
		"objectNonceId": "67890",
		"source_url": "https://channels.weixin.qq.com/web/pages/feed?oid=12345",
		"createtime": 1710000000,
		"contact": {
			"username": "acct_1",
			"nickname": "作者",
			"headUrl": "https://example.com/avatar.jpg",
			"signature": "sig"
		},
		"objectDesc": {
			"description": "profile title",
			"mediaType": 4,
			"media": [{
				"url": "https://video.example/demo.mp4?token=1",
				"thumbUrl": "https://example.com/cover.jpg",
				"width": 1920,
				"height": 1080,
				"fileSize": 5,
				"videoPlayLen": 12,
				"decodeKey": "9"
			}]
		}
	}`)

	targetID, err := importTargetDownloadRecord(db, task, labels, "12345", "67890", savePath, object)
	if err != nil {
		t.Fatalf("import target record: %v", err)
	}
	if targetID <= 0 {
		t.Fatalf("target id = %d", targetID)
	}

	var taskRecord model.DownloadTask
	if err := db.Where("id = ?", targetID).First(&taskRecord).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if taskRecord.PlatformId != platformWXChannels || taskRecord.UniqueID != "12345_xWT111" {
		t.Fatalf("unexpected task identity: platform=%q unique=%q", taskRecord.PlatformId, taskRecord.UniqueID)
	}
	if taskRecord.Status != model.TaskStatusFinished {
		t.Fatalf("task status = %d", taskRecord.Status)
	}
	var config map[string]string
	if err := json.Unmarshal([]byte(taskRecord.ConfigJSON), &config); err != nil {
		t.Fatalf("parse task config: %v", err)
	}
	if len(config) != 3 || config["download_dir"] != dir || config["suffix"] != ".mp4" || config["spec"] != "xWT111" {
		t.Fatalf("unexpected config_json: %s", taskRecord.ConfigJSON)
	}
	var metadata struct {
		GopeedID string `json:"gopeedid"`
	}
	if err := json.Unmarshal([]byte(taskRecord.MetadataJSON), &metadata); err != nil {
		t.Fatalf("parse task metadata: %v", err)
	}
	if metadata.GopeedID != "bolt-task-1" {
		t.Fatalf("unexpected metadata: %+v", metadata)
	}
	if taskRecord.MetadataJSON != `{"gopeedid":"bolt-task-1"}` {
		t.Fatalf("metadata_json = %s", taskRecord.MetadataJSON)
	}
	if existing, err := findMigratedDownloadTask(db, "bolt-task-1"); err != nil || existing.Id != targetID {
		t.Fatalf("find migrated by metadata: id=%v err=%v", func() int {
			if existing == nil {
				return 0
			}
			return existing.Id
		}(), err)
	}

	var content model.Content
	if err := db.Where("id = ?", buildTargetContentID("12345")).First(&content).Error; err != nil {
		t.Fatalf("query content: %v", err)
	}
	if content.Type != "video" || content.ExternalId2 != "67890" {
		t.Fatalf("unexpected content: type=%q external_id2=%q", content.Type, content.ExternalId2)
	}

	var account model.Account
	if err := db.Where("id = ?", buildTargetAccountID("acct_1")).First(&account).Error; err != nil {
		t.Fatalf("query account: %v", err)
	}
	if account.Nickname != "作者" {
		t.Fatalf("account nickname = %q", account.Nickname)
	}

	var association model.ContentAccount
	if err := db.Where("content_id = ? AND account_id = ?", content.Id, account.Id).First(&association).Error; err != nil {
		t.Fatalf("query content account: %v", err)
	}
	if association.Role != "owner" {
		t.Fatalf("association role = %q", association.Role)
	}

	var resource model.DownloadResource
	if err := db.Where("task_id = ?", targetID).First(&resource).Error; err != nil {
		t.Fatalf("query resource: %v", err)
	}
	if resource.DownloadDir != dir || resource.Name != "demo.mp4" || resource.Downloaded != 5 || resource.Status != resourceStatusFinished {
		t.Fatalf("unexpected resource: dir=%q name=%q downloaded=%d status=%d", resource.DownloadDir, resource.Name, resource.Downloaded, resource.Status)
	}

	var endpoint model.DownloadEndpoint
	if err := db.Where("resource_id = ?", resource.Id).First(&endpoint).Error; err != nil {
		t.Fatalf("query endpoint: %v", err)
	}
	if endpoint.Protocol != "https" || endpoint.URL != "https://video.example/demo.mp4?token=1" {
		t.Fatalf("unexpected endpoint: protocol=%q url=%q", endpoint.Protocol, endpoint.URL)
	}

	duplicateID, err := importTargetDownloadRecord(db, task, labels, "12345", "67890", savePath, object)
	var duplicate duplicateImportError
	if !errors.As(err, &duplicate) {
		t.Fatalf("duplicate error = %v", err)
	}
	if duplicateID != targetID || duplicate.TaskID != targetID {
		t.Fatalf("duplicate ids = %d/%d want %d", duplicateID, duplicate.TaskID, targetID)
	}

	var taskCount int64
	if err := db.Model(&model.DownloadTask{}).Where("platform_id = ? AND unique_id = ?", platformWXChannels, "12345_xWT111").Count(&taskCount).Error; err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if taskCount != 1 {
		t.Fatalf("task count = %d", taskCount)
	}
}

func TestFetchProfileWithCacheUsesLabelsID(t *testing.T) {
	dir := t.TempDir()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests += 1
		if got := r.URL.Query().Get("oid"); got != "12345" {
			t.Fatalf("oid = %q", got)
		}
		writeAPIResponse(w, apiResponse{
			Code: 0,
			Msg:  "success",
			Data: channelsFeedProfile{
				ErrCode: 0,
				Data: struct {
					Object json.RawMessage `json:"object"`
				}{
					Object: json.RawMessage(`{"id":"12345","objectDesc":{"mediaType":4,"media":[{"url":"https://example.com/a.mp4"}]}}`),
				},
			},
		})
	}))
	defer server.Close()

	migration := &migrationServer{}
	client := server.Client()
	first, _, cached, err := migration.fetchProfileWithCache(client, dir, server.URL, "12345", "12345", "uid")
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if cached || first == nil || requests != 1 {
		t.Fatalf("first fetch cached=%v profileNil=%v requests=%d", cached, first == nil, requests)
	}

	second, _, cached, err := migration.fetchProfileWithCache(client, dir, server.URL, "12345", "12345", "uid")
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if !cached || second == nil || requests != 1 {
		t.Fatalf("second fetch cached=%v profileNil=%v requests=%d", cached, second == nil, requests)
	}
	if _, err := os.Stat(filepath.Join(dir, profileCacheFile)); err != nil {
		t.Fatalf("profile cache file missing: %v", err)
	}
}
