package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
		"createtime": 1700000000,
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
	if taskRecord.CreatedAt != createdAt.UnixMilli() {
		t.Fatalf("task created_at = %d want %d", taskRecord.CreatedAt, createdAt.UnixMilli())
	}
	if taskRecord.UpdatedAt != createdAt.Add(time.Minute).UnixMilli() {
		t.Fatalf("task updated_at = %d want %d", taskRecord.UpdatedAt, createdAt.Add(time.Minute).UnixMilli())
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
	if content.PublishTime == nil || *content.PublishTime != 1700000000 {
		t.Fatalf("content publish_time = %v want 1700000000", content.PublishTime)
	}
	var content_metadata map[string]any
	if err := json.Unmarshal([]byte(content.Metadata), &content_metadata); err != nil {
		t.Fatalf("parse content metadata: %v", err)
	}
	if _, exists := content_metadata["profile_object"]; exists {
		t.Fatalf("content metadata contains profile_object: %s", content.Metadata)
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

func TestImportTargetWXMPDownloadRecordWritesArticleAndGopeedFile(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "article.html")
	html := "<html><body>migrated article</body></html>"
	if err := os.WriteFile(savePath, []byte(html), 0o644); err != nil {
		t.Fatalf("write migrated article: %v", err)
	}

	db, err := openTargetDB(filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("open target db: %v", err)
	}
	defer closeTargetDB(db)

	createdAt := time.Unix(1710000000, 0)
	task := &downloadpkg.Task{
		ID:        "mp-task-1",
		Protocol:  "officialaccount",
		Status:    base.DownloadStatusDone,
		Progress:  &downloadpkg.Progress{Downloaded: int64(len(html))},
		CreatedAt: createdAt,
		UpdatedAt: createdAt.Add(2 * time.Minute),
	}
	labels := map[string]string{
		"article_id": "k_F-1KYn-EPy27W9VoKZng",
	}
	articleURL := "https://mp.weixin.qq.com/s/k_F-1KYn-EPy27W9VoKZng"
	contentRaw := json.RawMessage(`{
		"user_name": "biz_user",
		"nick_name": "公众号作者",
		"title": "公众号标题",
		"desc": "公众号摘要",
		"content_noencode": "<p>正文内容</p>",
		"cdn_url": "https://mmbiz.qpic.cn/cover.jpg",
		"link": "https://mp.weixin.qq.com/s/k_F-1KYn-EPy27W9VoKZng",
		"source_url": "",
		"ori_create_time": 1700000000,
		"bizuin": "239001",
		"mid": 2247483666,
		"idx": 1,
		"ori_head_img_url": "https://mmbiz.qpic.cn/avatar.jpg"
	}`)

	targetID, err := importTargetWXMPDownloadRecord(db, task, labels, "k_F-1KYn-EPy27W9VoKZng", articleURL, savePath, contentRaw)
	if err != nil {
		t.Fatalf("import wxmp record: %v", err)
	}
	if targetID <= 0 {
		t.Fatalf("target id = %d", targetID)
	}

	var taskRecord model.DownloadTask
	if err := db.Where("id = ?", targetID).First(&taskRecord).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if taskRecord.PlatformId != platformWXMP {
		t.Fatalf("task platform = %q", taskRecord.PlatformId)
	}
	if taskRecord.Status != model.TaskStatusFinished {
		t.Fatalf("task status = %d", taskRecord.Status)
	}
	if taskRecord.CreatedAt != createdAt.UnixMilli() {
		t.Fatalf("task created_at = %d want %d", taskRecord.CreatedAt, createdAt.UnixMilli())
	}
	var metadata struct {
		GopeedID  string `json:"gopeedid"`
		ArticleID string `json:"article_id"`
	}
	if err := json.Unmarshal([]byte(taskRecord.MetadataJSON), &metadata); err != nil {
		t.Fatalf("parse task metadata: %v", err)
	}
	if metadata.GopeedID != "mp-task-1" || metadata.ArticleID != "k_F-1KYn-EPy27W9VoKZng" {
		t.Fatalf("unexpected metadata: %+v", metadata)
	}
	if existing, err := findMigratedDownloadTaskByPlatform(db, platformWXMP, "mp-task-1"); err != nil || existing.Id != targetID {
		t.Fatalf("find migrated wxmp by metadata: task=%+v err=%v", existing, err)
	}

	contentID := "wxmp:239001_2247483666_1"
	var content model.Content
	if err := db.Where("id = ?", contentID).First(&content).Error; err != nil {
		t.Fatalf("query content: %v", err)
	}
	if content.Type != "article" || content.Title != "公众号标题" {
		t.Fatalf("unexpected content: type=%q title=%q", content.Type, content.Title)
	}
	if content.PublishTime == nil || *content.PublishTime != 1700000000000 {
		t.Fatalf("content publish_time = %v want 1700000000000", content.PublishTime)
	}

	var article model.ContentArticle
	if err := db.Where("id = ?", contentID).First(&article).Error; err != nil {
		t.Fatalf("query content article: %v", err)
	}
	if article.Type != model.ContentArticleTypeHTML || article.HTML != "<p>正文内容</p>" {
		t.Fatalf("unexpected article detail: type=%q html=%q", article.Type, article.HTML)
	}

	var account model.Account
	if err := db.Where("id = ?", "wxmp:biz_user").First(&account).Error; err != nil {
		t.Fatalf("query account: %v", err)
	}
	if account.Nickname != "公众号作者" {
		t.Fatalf("account nickname = %q", account.Nickname)
	}

	var association model.ContentAccount
	if err := db.Where("content_id = ? AND account_id = ?", contentID, account.Id).First(&association).Error; err != nil {
		t.Fatalf("query content account: %v", err)
	}
	if association.Role != "publisher" {
		t.Fatalf("association role = %q", association.Role)
	}

	var resource model.DownloadResource
	if err := db.Where("task_id = ?", targetID).First(&resource).Error; err != nil {
		t.Fatalf("query resource: %v", err)
	}
	if resource.DownloadDir != dir || resource.Name != "article.html" || resource.Kind != "html" || resource.Downloaded != int64(len(html)) || resource.Status != resourceStatusFinished {
		t.Fatalf("unexpected resource: dir=%q name=%q kind=%q downloaded=%d status=%d", resource.DownloadDir, resource.Name, resource.Kind, resource.Downloaded, resource.Status)
	}

	var endpoint model.DownloadEndpoint
	if err := db.Where("resource_id = ?", resource.Id).First(&endpoint).Error; err != nil {
		t.Fatalf("query endpoint: %v", err)
	}
	if endpoint.Protocol != "https" || endpoint.URL != articleURL {
		t.Fatalf("unexpected endpoint: protocol=%q url=%q", endpoint.Protocol, endpoint.URL)
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
	first, _, cached, source, err := migration.fetchProfileWithCache(client, dir, server.URL, "12345", "12345", "uid", false)
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if cached || source != "http" || first == nil || requests != 1 {
		t.Fatalf("first fetch cached=%v source=%q profileNil=%v requests=%d", cached, source, first == nil, requests)
	}

	second, _, cached, source, err := migration.fetchProfileWithCache(client, dir, server.URL, "12345", "12345", "uid", false)
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if !cached || source != "cache" || second == nil || requests != 1 {
		t.Fatalf("second fetch cached=%v source=%q profileNil=%v requests=%d", cached, source, second == nil, requests)
	}
	if _, err := os.Stat(filepath.Join(dir, profileCacheFile)); err != nil {
		t.Fatalf("profile cache file missing: %v", err)
	}
}

func TestFetchProfileWithCacheRejectsEmptyObjectID(t *testing.T) {
	data_dir := t.TempDir()
	request_count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request_count += 1
		writeAPIResponse(w, apiResponse{
			Code: 0,
			Msg:  "success",
			Data: channelsFeedProfile{
				ErrCode: 0,
				ErrMsg:  "ok",
				Data: struct {
					Object json.RawMessage `json:"object"`
				}{
					Object: json.RawMessage(`{"id":"","objectDesc":{"mediaType":4}}`),
				},
			},
		})
	}))
	defer server.Close()

	migration := &migrationServer{}
	for attempt := 0; attempt < 2; attempt += 1 {
		profile, _, cached, _, err := migration.fetchProfileWithCache(
			server.Client(),
			data_dir,
			server.URL,
			"empty-id",
			"empty-id",
			"uid",
			false,
		)
		if err == nil || !strings.Contains(err.Error(), "profile object id is empty") {
			t.Fatalf("attempt %d error = %v", attempt+1, err)
		}
		if profile != nil || cached {
			t.Fatalf("attempt %d profile=%v cached=%v", attempt+1, profile, cached)
		}
	}
	if request_count != 2 {
		t.Fatalf("profile requests = %d want 2", request_count)
	}

	cache, err := loadProfileCache(profileCachePath(data_dir))
	if err != nil {
		t.Fatalf("load profile cache: %v", err)
	}
	if _, exists := cache.Items["empty-id"]; exists {
		t.Fatalf("empty-id profile was cached")
	}
}

func TestReadProfileCacheDiscardsEmptyObjectID(t *testing.T) {
	data_dir := t.TempDir()
	cache_key := "empty-id"
	migration := &migrationServer{}
	if err := migration.writeProfileCache(data_dir, cache_key, profileCacheEntry{
		OID:        cache_key,
		ProfileURL: "https://example.com/profile?oid=empty-id",
		Profile:    json.RawMessage(`{"errCode":0,"errMsg":"ok","data":{"object":{"id":""}}}`),
		CachedAt:   time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("write invalid profile cache: %v", err)
	}

	cache, err := loadProfileCache(profileCachePath(data_dir))
	if err != nil {
		t.Fatalf("load profile cache: %v", err)
	}
	task := mustGopeedMigrationTask(t, "empty-id-task", data_dir, "demo.mp4", map[string]string{
		"oid": cache_key,
	})
	if cached, got_key := taskProfileCached(cache, task); cached || got_key != cache_key {
		t.Fatalf("taskProfileCached = %v/%q want false/%q", cached, got_key, cache_key)
	}

	profile, _, hit, err := migration.readProfileCache(data_dir, cache_key)
	if err != nil {
		t.Fatalf("read invalid profile cache: %v", err)
	}
	if profile != nil || hit {
		t.Fatalf("invalid profile cache profile=%v hit=%v", profile, hit)
	}

	cache, err = loadProfileCache(profileCachePath(data_dir))
	if err != nil {
		t.Fatalf("reload profile cache: %v", err)
	}
	if _, exists := cache.Items[cache_key]; exists {
		t.Fatalf("invalid profile cache entry was not discarded")
	}
}

func TestProfileCacheCleanupAPIOnlyRemovesInvalidChannelsEntries(t *testing.T) {
	data_dir := t.TempDir()
	migration := &migrationServer{defaultDataDir: data_dir}
	entries := map[string]json.RawMessage{
		"valid-profile": json.RawMessage(`{"errCode":0,"errMsg":"ok","data":{"object":{"id":"12345"}}}`),
		"empty-id":      json.RawMessage(`{"errCode":0,"errMsg":"ok","data":{"object":{"id":""}}}`),
		"bad-profile":   json.RawMessage(`"not-a-profile"`),
		"wxmp_article:article-id": json.RawMessage(
			`{"title":"公众号文章缓存，不包含 object.id"}`,
		),
	}
	for cache_key, profile := range entries {
		if err := migration.writeProfileCache(data_dir, cache_key, profileCacheEntry{
			OID:        cache_key,
			ProfileURL: "https://example.com/profile/" + cache_key,
			Profile:    profile,
			CachedAt:   time.Now().UnixMilli(),
		}); err != nil {
			t.Fatalf("write profile cache %q: %v", cache_key, err)
		}
	}

	request_body, err := json.Marshal(profileCacheCleanupRequest{DataDir: data_dir})
	if err != nil {
		t.Fatalf("marshal cleanup request: %v", err)
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/migration/profile-cache/cleanup",
		bytes.NewReader(request_body),
	)
	recorder := httptest.NewRecorder()
	migration.handle_migration_profile_cache_cleanup(recorder, request)

	var response struct {
		Code int                       `json:"code"`
		Msg  string                    `json:"msg"`
		Data profileCacheCleanupResult `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode cleanup response: %v", err)
	}
	if response.Code != 0 {
		t.Fatalf("cleanup response code=%d msg=%q", response.Code, response.Msg)
	}
	result := response.Data
	if result.Total != 4 || result.Checked != 3 || result.SkippedArticle != 1 || result.Removed != 2 || result.Remaining != 2 {
		t.Fatalf("unexpected cleanup result: %+v", result)
	}
	want_removed_keys := []string{"bad-profile", "empty-id"}
	if len(result.RemovedKeys) != len(want_removed_keys) ||
		result.RemovedKeys[0] != want_removed_keys[0] ||
		result.RemovedKeys[1] != want_removed_keys[1] {
		t.Fatalf("removed keys = %v want %v", result.RemovedKeys, want_removed_keys)
	}

	cache, err := loadProfileCache(profileCachePath(data_dir))
	if err != nil {
		t.Fatalf("load cleaned profile cache: %v", err)
	}
	if _, exists := cache.Items["valid-profile"]; !exists {
		t.Fatalf("valid channels profile was removed")
	}
	if _, exists := cache.Items["wxmp_article:article-id"]; !exists {
		t.Fatalf("wxmp article profile was removed")
	}
	for _, cache_key := range want_removed_keys {
		if _, exists := cache.Items[cache_key]; exists {
			t.Fatalf("invalid profile cache %q was not removed", cache_key)
		}
	}
}

func TestLoadGopeedTaskSnapshotCachesUntilForceReload(t *testing.T) {
	dir := t.TempDir()
	writeGopeedTask(t, dir, mustGopeedMigrationTask(t, "cache-list-task-1", dir, "demo.mp4", map[string]string{
		"oid": "12345",
	}))

	migration := &migrationServer{}
	dbPath := filepath.Join(dir, gopeedDBFile)
	first, hit, err := migration.loadGopeedTaskSnapshot(dir, dbPath, false)
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if hit {
		t.Fatalf("first snapshot cache hit = true")
	}
	if len(first.Tasks) != 1 || safeTaskID(first.Tasks[0]) != "cache-list-task-1" {
		t.Fatalf("unexpected first snapshot tasks: %+v", first.Tasks)
	}

	second, hit, err := migration.loadGopeedTaskSnapshot(dir, dbPath, false)
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	if !hit {
		t.Fatalf("second snapshot cache hit = false")
	}
	if second != first {
		t.Fatalf("second snapshot did not reuse cached entry")
	}

	third, hit, err := migration.loadGopeedTaskSnapshot(dir, dbPath, true)
	if err != nil {
		t.Fatalf("forced snapshot: %v", err)
	}
	if hit {
		t.Fatalf("forced snapshot cache hit = true")
	}
	if third == first {
		t.Fatalf("forced snapshot reused cached entry")
	}
}

func TestExplicitDBPathIsUsedWithoutGopeedFilenameFallback(t *testing.T) {
	data_dir := t.TempDir()
	task := mustGopeedMigrationTask(t, "explicit-db-task", data_dir, "demo.mp4", map[string]string{
		"oid": "12345",
	})
	writeGopeedTask(t, data_dir, task)

	original_db_path := filepath.Join(data_dir, gopeedDBFile)
	selected_db_path := filepath.Join(data_dir, "archived.DB")
	if err := os.Rename(original_db_path, selected_db_path); err != nil {
		t.Fatalf("rename gopeed database: %v", err)
	}

	migration := &migrationServer{defaultDataDir: selected_db_path}
	resolved_data_dir, resolved_db_path, err := migration.resolveDataDir(selected_db_path)
	if err != nil {
		t.Fatalf("resolve explicit database: %v", err)
	}
	if resolved_data_dir != data_dir || resolved_db_path != selected_db_path {
		t.Fatalf("resolved paths = %q/%q want %q/%q", resolved_data_dir, resolved_db_path, data_dir, selected_db_path)
	}
	if _, _, err := migration.resolveDataDir(data_dir); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("directory path error = %v", err)
	}

	snapshot, _, err := migration.loadGopeedTaskSnapshot(resolved_data_dir, resolved_db_path, false)
	if err != nil {
		t.Fatalf("load explicitly selected database: %v", err)
	}
	if len(snapshot.Tasks) != 1 || safeTaskID(snapshot.Tasks[0]) != "explicit-db-task" {
		t.Fatalf("unexpected explicit database tasks: %+v", snapshot.Tasks)
	}
}

func TestPrefetchMigrationProfileCacheThenMigrateUsesCache(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "demo.mp4")
	if err := os.WriteFile(savePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write migrated file: %v", err)
	}

	task := mustGopeedMigrationTask(t, "cache-task-1", dir, "demo.mp4", map[string]string{
		"id":    "12345",
		"oid":   "12345",
		"uid":   "67890",
		"title": "cached title",
	})

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		writeAPIResponse(w, apiResponse{
			Code: 0,
			Msg:  "success",
			Data: channelsFeedProfile{
				ErrCode: 0,
				Data: struct {
					Object json.RawMessage `json:"object"`
				}{
					Object: json.RawMessage(`{
						"id":"12345",
						"objectNonceId":"67890",
						"contact":{"username":"acct_1","nickname":"作者"},
						"objectDesc":{"description":"cached title","mediaType":4,"media":[{"url":"https://example.com/a.mp4","fileSize":5}]}
					}`),
				},
			},
		})
	}))

	migration := &migrationServer{}
	statuses := migration.prefetchMigrationProfileCache(server.Client(), dir, server.URL, []*downloadpkg.Task{task})
	server.Close()
	if requests != 1 {
		t.Fatalf("profile requests = %d want 1", requests)
	}
	if status := statuses["cache-task-1"]; status.Error != "" || status.CacheKey != "12345" {
		t.Fatalf("unexpected prefetch status: %+v", status)
	}
	if _, _, hit, err := migration.readProfileCache(dir, "12345"); err != nil || !hit {
		t.Fatalf("read prefetched cache hit=%v err=%v", hit, err)
	}

	db, err := openTargetDB(filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("open target db: %v", err)
	}
	defer closeTargetDB(db)

	item := migration.migrateOneTask(db, server.URL, dir, task, false, false, statuses)
	if item.Action != "migrated" || !item.ProfileCacheHit || item.TargetID <= 0 {
		t.Fatalf("unexpected migration item: %+v", item)
	}
	if requests != 1 {
		t.Fatalf("migration fetched profile again: requests=%d", requests)
	}
}

func TestMigrateOneTaskCreatesStandaloneTaskWhenProfileIsMissing(t *testing.T) {
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

	task := mustGopeedMigrationTask(t, "cache-task-missing", dir, "demo.mp4", map[string]string{
		"id":  "missing",
		"oid": "missing",
		"uid": "67890",
	})

	migration := &migrationServer{}
	item := migration.migrateOneTask(db, "http://127.0.0.1:1", dir, task, false, false, nil)
	if item.Action != "migrated" || item.TargetID <= 0 || item.ProfileCacheHit {
		t.Fatalf("unexpected migration item: %+v", item)
	}

	var task_record model.DownloadTask
	if err := db.Where("id = ?", item.TargetID).First(&task_record).Error; err != nil {
		t.Fatalf("query standalone task: %v", err)
	}
	if task_record.ContentId != nil {
		t.Fatalf("standalone task content_id = %v want nil", task_record.ContentId)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(task_record.MetadataJSON), &metadata); err != nil {
		t.Fatalf("parse standalone task metadata: %v", err)
	}
	if metadata["gopeedid"] != "cache-task-missing" || metadata["detail_missing"] != true {
		t.Fatalf("unexpected standalone task metadata: %v", metadata)
	}

	var resource_record model.DownloadResource
	if err := db.Where("task_id = ?", item.TargetID).First(&resource_record).Error; err != nil {
		t.Fatalf("query resource without content: %v", err)
	}
	if resource_record.ContentId != nil {
		t.Fatalf("resource content_id = %v want nil", resource_record.ContentId)
	}
	if resource_record.Size != 5 || resource_record.Downloaded != 5 || resource_record.Status != resourceStatusFinished {
		t.Fatalf("unexpected resource progress: size=%d downloaded=%d status=%d", resource_record.Size, resource_record.Downloaded, resource_record.Status)
	}
	if resource_path := filepath.Join(resource_record.DownloadDir, resource_record.Name); resource_path != savePath {
		t.Fatalf("resource path = %q want %q", resource_path, savePath)
	}
	var endpoint_record model.DownloadEndpoint
	if err := db.Where("resource_id = ?", resource_record.Id).First(&endpoint_record).Error; err != nil {
		t.Fatalf("query endpoint without content: %v", err)
	}
	if endpoint_record.Protocol != "https" || endpoint_record.URL != "https://example.com/demo.mp4" {
		t.Fatalf("unexpected endpoint: %+v", endpoint_record)
	}

	for name, value := range map[string]any{
		"content": &model.Content{},
		"account": &model.Account{},
	} {
		var count int64
		if err := db.Model(value).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		if count != 0 {
			t.Fatalf("%s count = %d want 0", name, count)
		}
	}
}

func TestMigrateOneTaskCreatesStandaloneTaskWithoutDetailIdentifier(t *testing.T) {
	data_dir := t.TempDir()
	db, err := openTargetDB(filepath.Join(data_dir, "data.db"))
	if err != nil {
		t.Fatalf("open target db: %v", err)
	}
	defer closeTargetDB(db)

	task := mustGopeedMigrationTask(t, "no-detail-identifier", data_dir, "orphan.mp4", map[string]string{})
	migration := &migrationServer{}
	item := migration.migrateOneTask(db, "", data_dir, task, false, false, nil)
	if item.Action != "migrated" || item.TargetID <= 0 {
		t.Fatalf("unexpected migration item: %+v", item)
	}

	var task_record model.DownloadTask
	if err := db.Where("id = ?", item.TargetID).First(&task_record).Error; err != nil {
		t.Fatalf("query standalone task: %v", err)
	}
	if task_record.ContentId != nil || strings.TrimSpace(task_record.UniqueID) == "" {
		t.Fatalf("unexpected standalone task: %+v", task_record)
	}
}

func TestMigrateWXMPTaskCreatesStandaloneTaskWhenArticleDetailIsMissing(t *testing.T) {
	data_dir := t.TempDir()
	db, err := openTargetDB(filepath.Join(data_dir, "data.db"))
	if err != nil {
		t.Fatalf("open target db: %v", err)
	}
	defer closeTargetDB(db)

	article_id := "missing-article-detail"
	task := mustGopeedMigrationTask(t, "wxmp-detail-missing", data_dir, "article.html", map[string]string{
		"article_id": article_id,
	})
	migration := &migrationServer{}
	item := migration.migrateOneTask(db, "", data_dir, task, false, false, nil)
	if item.Action != "migrated" || item.TargetID <= 0 || item.ProfileCacheHit {
		t.Fatalf("unexpected migration item: %+v", item)
	}

	var task_record model.DownloadTask
	if err := db.Where("id = ?", item.TargetID).First(&task_record).Error; err != nil {
		t.Fatalf("query standalone wxmp task: %v", err)
	}
	if task_record.ContentId != nil || task_record.PlatformId != platformWXMP {
		t.Fatalf("unexpected standalone wxmp task: %+v", task_record)
	}
	var resource_record model.DownloadResource
	if err := db.Where("task_id = ?", item.TargetID).First(&resource_record).Error; err != nil {
		t.Fatalf("query standalone wxmp resource: %v", err)
	}
	if resource_record.ContentId != nil {
		t.Fatalf("standalone wxmp resource content_id = %v want nil", resource_record.ContentId)
	}
	if resource_record.Size != 5 || resource_record.Downloaded != 5 || resource_record.Status != resourceStatusFinished {
		t.Fatalf("unexpected wxmp resource progress: size=%d downloaded=%d status=%d", resource_record.Size, resource_record.Downloaded, resource_record.Status)
	}
	want_resource_path := filepath.Join(data_dir, "article.html")
	if resource_path := filepath.Join(resource_record.DownloadDir, resource_record.Name); resource_path != want_resource_path {
		t.Fatalf("wxmp resource path = %q want %q", resource_path, want_resource_path)
	}
	var endpoint_record model.DownloadEndpoint
	if err := db.Where("resource_id = ?", resource_record.Id).First(&endpoint_record).Error; err != nil {
		t.Fatalf("query standalone wxmp endpoint: %v", err)
	}
	if endpoint_record.URL != buildWXMPArticleURL(article_id) {
		t.Fatalf("wxmp endpoint url = %q", endpoint_record.URL)
	}
	var content_count int64
	if err := db.Model(&model.Content{}).Count(&content_count).Error; err != nil {
		t.Fatalf("count wxmp content: %v", err)
	}
	if content_count != 0 {
		t.Fatalf("wxmp content count = %d want 0", content_count)
	}
	var account_count int64
	if err := db.Model(&model.Account{}).Count(&account_count).Error; err != nil {
		t.Fatalf("count wxmp account: %v", err)
	}
	if account_count != 0 {
		t.Fatalf("wxmp account count = %d want 0", account_count)
	}
}

func TestMigrateWXMPArticleTaskUsesArticleCache(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "article.html")
	if err := os.WriteFile(savePath, []byte("<html><body>cached</body></html>"), 0o644); err != nil {
		t.Fatalf("write migrated article: %v", err)
	}

	articleID := "k_F-1KYn-EPy27W9VoKZng"
	articleURL := buildWXMPArticleURL(articleID)
	contentRaw := json.RawMessage(`{
		"user_name": "biz_user",
		"nick_name": "公众号作者",
		"title": "缓存公众号标题",
		"desc": "公众号摘要",
		"content_noencode": "<p>缓存正文</p>",
		"cdn_url": "https://mmbiz.qpic.cn/cover.jpg",
		"link": "https://mp.weixin.qq.com/s/k_F-1KYn-EPy27W9VoKZng",
		"ori_create_time": 1700000000,
		"bizuin": "239001",
		"mid": 2247483666,
		"idx": 1,
		"ori_head_img_url": "https://mmbiz.qpic.cn/avatar.jpg"
	}`)

	task := mustGopeedMigrationTask(t, "mp-cache-task-1", dir, "article.html", map[string]string{
		"article_id": articleID,
	})
	migration := &migrationServer{}
	cacheKey := wxmpArticleProfileCacheKey(articleID, articleURL)
	if err := migration.writeProfileCache(dir, cacheKey, profileCacheEntry{
		OID:        articleID,
		ProfileURL: articleURL,
		Profile:    contentRaw,
		CachedAt:   time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("write article cache: %v", err)
	}
	cache, err := loadProfileCache(profileCachePath(dir))
	if err != nil {
		t.Fatalf("load profile cache: %v", err)
	}
	if cached, gotKey := taskProfileCached(cache, task); !cached || gotKey != cacheKey {
		t.Fatalf("taskProfileCached = %v/%q want true/%q", cached, gotKey, cacheKey)
	}

	db, err := openTargetDB(filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("open target db: %v", err)
	}
	defer closeTargetDB(db)

	item := migration.migrateOneTask(db, "", dir, task, false, false, nil)
	if item.Action != "migrated" || !item.ProfileCacheHit || item.TargetID <= 0 {
		t.Fatalf("unexpected migration item: %+v", item)
	}
}

func mustGopeedMigrationTask(t *testing.T, id string, downloadDir string, filename string, labels map[string]string) *downloadpkg.Task {
	t.Helper()
	payload := map[string]any{
		"id":       id,
		"protocol": "http",
		"meta": map[string]any{
			"req": map[string]any{
				"url":    "https://example.com/" + filename,
				"labels": labels,
			},
			"res": map[string]any{
				"size": 5,
				"files": []map[string]any{{
					"name": filename,
					"size": 5,
				}},
			},
			"opts": map[string]any{
				"path": downloadDir,
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal task payload: %v", err)
	}
	var task downloadpkg.Task
	if err := json.Unmarshal(data, &task); err != nil {
		t.Fatalf("unmarshal task payload: %v", err)
	}
	createdAt := time.Unix(1710000000, 0)
	task.Status = base.DownloadStatusDone
	task.Progress = &downloadpkg.Progress{Downloaded: 5}
	task.CreatedAt = createdAt
	task.UpdatedAt = createdAt.Add(time.Minute)
	return &task
}

func writeGopeedTask(t *testing.T, dir string, task *downloadpkg.Task) {
	t.Helper()
	storage := downloadpkg.NewBoltStorage(dir)
	defer func() {
		if err := storage.Close(); err != nil {
			t.Fatalf("close gopeed storage: %v", err)
		}
	}()
	if err := storage.Setup([]string{"task", "save", "config"}); err != nil {
		t.Fatalf("setup gopeed storage: %v", err)
	}
	if err := storage.Put("task", safeTaskID(task), task); err != nil {
		t.Fatalf("write gopeed task: %v", err)
	}
}
