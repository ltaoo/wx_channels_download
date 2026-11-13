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

	wxchannelsadapter "wx_channel/internal/adapter/wxchannels"
	"wx_channel/internal/database/model"
)

func TestImportTargetDownloadRecordRecreatesCurrentGraph(t *testing.T) {
	dir := t.TempDir()
	save_path := filepath.Join(dir, "demo.mp4")
	if err := os.WriteFile(save_path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write migrated file: %v", err)
	}

	db, err := open_target_db(filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("open target db: %v", err)
	}
	defer close_target_db(db)

	created_at := time.Unix(1710000000, 0)
	task := &downloadpkg.Task{
		ID:        "bolt-task-1",
		Protocol:  "http",
		Status:    base.DownloadStatusDone,
		Progress:  &downloadpkg.Progress{Downloaded: 5},
		CreatedAt: created_at,
		UpdatedAt: created_at.Add(time.Minute),
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

	target_id, err := import_target_download_record(db, task, labels, "12345", "67890", save_path, object)
	if err != nil {
		t.Fatalf("import target record: %v", err)
	}
	if target_id <= 0 {
		t.Fatalf("target id = %d", target_id)
	}

	var task_record model.DownloadTask
	if err := db.Where("id = ?", target_id).First(&task_record).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if task_record.PlatformId != platform_wx_channels || task_record.UniqueID != "12345_xWT111_mp4" {
		t.Fatalf("unexpected task identity: platform=%q unique=%q", task_record.PlatformId, task_record.UniqueID)
	}
	if task_record.Status != model.TaskStatusFinished {
		t.Fatalf("task status = %d", task_record.Status)
	}
	if task_record.CreatedAt != created_at.UnixMilli() {
		t.Fatalf("task created_at = %d want %d", task_record.CreatedAt, created_at.UnixMilli())
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(task_record.ConfigJSON), &config); err != nil {
		t.Fatalf("parse task config: %v", err)
	}
	if config["download_dir"] != dir || config["suffix"] != ".mp4" || config["spec"] != "xWT111" {
		t.Fatalf("unexpected config_json: %s", task_record.ConfigJSON)
	}
	var metadata struct {
		GopeedID string `json:"gopeedid"`
	}
	if err := json.Unmarshal([]byte(task_record.MetadataJSON), &metadata); err != nil {
		t.Fatalf("parse task metadata: %v", err)
	}
	if metadata.GopeedID != "bolt-task-1" {
		t.Fatalf("unexpected metadata: %+v", metadata)
	}
	if !strings.Contains(task_record.MetadataJSON, `"migrated":true`) {
		t.Fatalf("metadata_json = %s", task_record.MetadataJSON)
	}
	if existing, err := find_migrated_download_task(db, "bolt-task-1"); err != nil || existing.Id != target_id {
		t.Fatalf("find migrated by metadata: id=%v err=%v", func() int {
			if existing == nil {
				return 0
			}
			return existing.Id
		}(), err)
	}

	var content model.Content
	if err := db.Where("id = ?", wxchannelsadapter.BuildContentID("12345")).First(&content).Error; err != nil {
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
	if err := db.Where("id = ?", wxchannelsadapter.BuildAccountID("acct_1")).First(&account).Error; err != nil {
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
	if err := db.Where("task_id = ?", target_id).First(&resource).Error; err != nil {
		t.Fatalf("query resource: %v", err)
	}
	if resource.DownloadDir != dir || resource.Name != "demo.mp4" || resource.Downloaded != 5 || resource.Status != resource_status_finished {
		t.Fatalf("unexpected resource: dir=%q name=%q downloaded=%d status=%d", resource.DownloadDir, resource.Name, resource.Downloaded, resource.Status)
	}

	var endpoint model.DownloadEndpoint
	if err := db.Where("resource_id = ?", resource.Id).First(&endpoint).Error; err != nil {
		t.Fatalf("query endpoint: %v", err)
	}
	if endpoint.Protocol != "https" || !strings.HasPrefix(endpoint.URL, "https://video.example/demo.mp4?token=1") {
		t.Fatalf("unexpected endpoint: protocol=%q url=%q", endpoint.Protocol, endpoint.URL)
	}

	duplicate_id, err := import_target_download_record(db, task, labels, "12345", "67890", save_path, object)
	var duplicate duplicate_import_error
	if !errors.As(err, &duplicate) {
		t.Fatalf("duplicate error = %v", err)
	}
	if duplicate_id != target_id || duplicate.TaskID != target_id {
		t.Fatalf("duplicate ids = %d/%d want %d", duplicate_id, duplicate.TaskID, target_id)
	}

	var task_count int64
	if err := db.Model(&model.DownloadTask{}).Where("platform_id = ? AND unique_id = ?", platform_wx_channels, "12345_xWT111_mp4").Count(&task_count).Error; err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if task_count != 1 {
		t.Fatalf("task count = %d", task_count)
	}
}

func TestImportTargetWXMPDownloadRecordWritesArticleAndGopeedFile(t *testing.T) {
	dir := t.TempDir()
	save_path := filepath.Join(dir, "article.html")
	html := "<html><body>migrated article</body></html>"
	if err := os.WriteFile(save_path, []byte(html), 0o644); err != nil {
		t.Fatalf("write migrated article: %v", err)
	}

	db, err := open_target_db(filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("open target db: %v", err)
	}
	defer close_target_db(db)

	created_at := time.Unix(1710000000, 0)
	task := &downloadpkg.Task{
		ID:        "mp-task-1",
		Protocol:  "officialaccount",
		Status:    base.DownloadStatusDone,
		Progress:  &downloadpkg.Progress{Downloaded: int64(len(html))},
		CreatedAt: created_at,
		UpdatedAt: created_at.Add(2 * time.Minute),
	}
	labels := map[string]string{
		"article_id": "k_F-1KYn-EPy27W9VoKZng",
	}
	article_url := "https://mp.weixin.qq.com/s/k_F-1KYn-EPy27W9VoKZng"
	content_raw := json.RawMessage(`{
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

	target_id, err := import_target_wxmp_download_record(db, task, labels, "k_F-1KYn-EPy27W9VoKZng", article_url, save_path, content_raw)
	if err != nil {
		t.Fatalf("import wxmp record: %v", err)
	}
	if target_id <= 0 {
		t.Fatalf("target id = %d", target_id)
	}

	var task_record model.DownloadTask
	if err := db.Where("id = ?", target_id).First(&task_record).Error; err != nil {
		t.Fatalf("query task: %v", err)
	}
	if task_record.PlatformId != platform_wxmp {
		t.Fatalf("task platform = %q", task_record.PlatformId)
	}
	if task_record.Status != model.TaskStatusFinished {
		t.Fatalf("task status = %d", task_record.Status)
	}
	if task_record.CreatedAt != created_at.UnixMilli() {
		t.Fatalf("task created_at = %d want %d", task_record.CreatedAt, created_at.UnixMilli())
	}
	var metadata struct {
		GopeedID  string `json:"gopeedid"`
		ArticleID string `json:"article_id"`
	}
	if err := json.Unmarshal([]byte(task_record.MetadataJSON), &metadata); err != nil {
		t.Fatalf("parse task metadata: %v", err)
	}
	if metadata.GopeedID != "mp-task-1" || metadata.ArticleID != "k_F-1KYn-EPy27W9VoKZng" {
		t.Fatalf("unexpected metadata: %+v", metadata)
	}
	if existing, err := find_migrated_download_task_by_platform(db, platform_wxmp, "mp-task-1"); err != nil || existing.Id != target_id {
		t.Fatalf("find migrated wxmp by metadata: task=%+v err=%v", existing, err)
	}

	content_id := "wxmp:239001_2247483666_1"
	var content model.Content
	if err := db.Where("id = ?", content_id).First(&content).Error; err != nil {
		t.Fatalf("query content: %v", err)
	}
	if content.Type != "article" || content.Title != "公众号标题" {
		t.Fatalf("unexpected content: type=%q title=%q", content.Type, content.Title)
	}
	if content.PublishTime == nil || *content.PublishTime != 1700000000000 {
		t.Fatalf("content publish_time = %v want 1700000000000", content.PublishTime)
	}

	var article model.ContentArticle
	if err := db.Where("id = ?", content_id).First(&article).Error; err != nil {
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
	if err := db.Where("content_id = ? AND account_id = ?", content_id, account.Id).First(&association).Error; err != nil {
		t.Fatalf("query content account: %v", err)
	}
	if association.Role != "publisher" {
		t.Fatalf("association role = %q", association.Role)
	}

	var resource model.DownloadResource
	if err := db.Where("task_id = ?", target_id).First(&resource).Error; err != nil {
		t.Fatalf("query resource: %v", err)
	}
	if resource.DownloadDir != dir || resource.Name != "article.html" || resource.Kind != "html" || resource.Downloaded != int64(len(html)) || resource.Status != resource_status_finished {
		t.Fatalf("unexpected resource: dir=%q name=%q kind=%q downloaded=%d status=%d", resource.DownloadDir, resource.Name, resource.Kind, resource.Downloaded, resource.Status)
	}

	var endpoint model.DownloadEndpoint
	if err := db.Where("resource_id = ?", resource.Id).First(&endpoint).Error; err != nil {
		t.Fatalf("query endpoint: %v", err)
	}
	if endpoint.Protocol != "https" || endpoint.URL != article_url {
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
		write_api_response(w, api_response{
			Code: 0,
			Msg:  "success",
			Data: channels_feed_profile{
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

	migration := &migration_server{}
	client := server.Client()
	first, _, cached, source, err := migration.fetch_profile_with_cache(client, dir, server.URL, "12345", "12345", "uid", false)
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if cached || source != "http" || first == nil || requests != 1 {
		t.Fatalf("first fetch cached=%v source=%q profileNil=%v requests=%d", cached, source, first == nil, requests)
	}

	second, _, cached, source, err := migration.fetch_profile_with_cache(client, dir, server.URL, "12345", "12345", "uid", false)
	if err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if !cached || source != "cache" || second == nil || requests != 1 {
		t.Fatalf("second fetch cached=%v source=%q profileNil=%v requests=%d", cached, source, second == nil, requests)
	}
	if _, err := os.Stat(filepath.Join(dir, profile_cache_file)); err != nil {
		t.Fatalf("profile cache file missing: %v", err)
	}
}

func TestFetchProfileWithCacheRejectsEmptyObjectID(t *testing.T) {
	data_dir := t.TempDir()
	request_count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request_count += 1
		write_api_response(w, api_response{
			Code: 0,
			Msg:  "success",
			Data: channels_feed_profile{
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

	migration := &migration_server{}
	for attempt := 0; attempt < 2; attempt += 1 {
		profile, _, cached, _, err := migration.fetch_profile_with_cache(
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

	cache, err := load_profile_cache(profile_cache_path(data_dir))
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
	migration := &migration_server{}
	if err := migration.write_profile_cache(data_dir, cache_key, profile_cache_entry{
		OID:        cache_key,
		ProfileURL: "https://example.com/profile?oid=empty-id",
		Profile:    json.RawMessage(`{"errCode":0,"errMsg":"ok","data":{"object":{"id":""}}}`),
		CachedAt:   time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("write invalid profile cache: %v", err)
	}

	cache, err := load_profile_cache(profile_cache_path(data_dir))
	if err != nil {
		t.Fatalf("load profile cache: %v", err)
	}
	task := must_gopeed_migration_task(t, "empty-id-task", data_dir, "demo.mp4", map[string]string{
		"oid": cache_key,
	})
	if cached, got_key := task_profile_cached(cache, task); cached || got_key != cache_key {
		t.Fatalf("taskProfileCached = %v/%q want false/%q", cached, got_key, cache_key)
	}

	profile, _, hit, err := migration.read_profile_cache(data_dir, cache_key)
	if err != nil {
		t.Fatalf("read invalid profile cache: %v", err)
	}
	if profile != nil || hit {
		t.Fatalf("invalid profile cache profile=%v hit=%v", profile, hit)
	}

	cache, err = load_profile_cache(profile_cache_path(data_dir))
	if err != nil {
		t.Fatalf("reload profile cache: %v", err)
	}
	if _, exists := cache.Items[cache_key]; exists {
		t.Fatalf("invalid profile cache entry was not discarded")
	}
}

func TestProfileCacheCleanupAPIOnlyRemovesInvalidChannelsEntries(t *testing.T) {
	data_dir := t.TempDir()
	migration := &migration_server{default_data_dir: data_dir}
	entries := map[string]json.RawMessage{
		"valid-profile": json.RawMessage(`{"errCode":0,"errMsg":"ok","data":{"object":{"id":"12345"}}}`),
		"empty-id":      json.RawMessage(`{"errCode":0,"errMsg":"ok","data":{"object":{"id":""}}}`),
		"bad-profile":   json.RawMessage(`"not-a-profile"`),
		"wxmp_article:article-id": json.RawMessage(
			`{"title":"公众号文章缓存，不包含 object.id"}`,
		),
	}
	for cache_key, profile := range entries {
		if err := migration.write_profile_cache(data_dir, cache_key, profile_cache_entry{
			OID:        cache_key,
			ProfileURL: "https://example.com/profile/" + cache_key,
			Profile:    profile,
			CachedAt:   time.Now().UnixMilli(),
		}); err != nil {
			t.Fatalf("write profile cache %q: %v", cache_key, err)
		}
	}

	request_body, err := json.Marshal(profile_cache_cleanup_request{DataDir: data_dir})
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
		Code int                          `json:"code"`
		Msg  string                       `json:"msg"`
		Data profile_cache_cleanup_result `json:"data"`
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

	cache, err := load_profile_cache(profile_cache_path(data_dir))
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
	write_gopeed_task(t, dir, must_gopeed_migration_task(t, "cache-list-task-1", dir, "demo.mp4", map[string]string{
		"oid": "12345",
	}))

	migration := &migration_server{}
	db_path := filepath.Join(dir, gopeed_db_file)
	first, hit, err := migration.load_gopeed_task_snapshot(dir, db_path, false)
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if hit {
		t.Fatalf("first snapshot cache hit = true")
	}
	if len(first.Tasks) != 1 || safe_task_id(first.Tasks[0]) != "cache-list-task-1" {
		t.Fatalf("unexpected first snapshot tasks: %+v", first.Tasks)
	}

	second, hit, err := migration.load_gopeed_task_snapshot(dir, db_path, false)
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	if !hit {
		t.Fatalf("second snapshot cache hit = false")
	}
	if second != first {
		t.Fatalf("second snapshot did not reuse cached entry")
	}

	third, hit, err := migration.load_gopeed_task_snapshot(dir, db_path, true)
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
	task := must_gopeed_migration_task(t, "explicit-db-task", data_dir, "demo.mp4", map[string]string{
		"oid": "12345",
	})
	write_gopeed_task(t, data_dir, task)

	original_db_path := filepath.Join(data_dir, gopeed_db_file)
	selected_db_path := filepath.Join(data_dir, "archived.DB")
	if err := os.Rename(original_db_path, selected_db_path); err != nil {
		t.Fatalf("rename gopeed database: %v", err)
	}

	migration := &migration_server{default_data_dir: selected_db_path}
	resolved_data_dir, resolved_db_path, err := migration.resolve_data_dir(selected_db_path)
	if err != nil {
		t.Fatalf("resolve explicit database: %v", err)
	}
	if resolved_data_dir != data_dir || resolved_db_path != selected_db_path {
		t.Fatalf("resolved paths = %q/%q want %q/%q", resolved_data_dir, resolved_db_path, data_dir, selected_db_path)
	}
	if _, _, err := migration.resolve_data_dir(data_dir); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("directory path error = %v", err)
	}

	snapshot, _, err := migration.load_gopeed_task_snapshot(resolved_data_dir, resolved_db_path, false)
	if err != nil {
		t.Fatalf("load explicitly selected database: %v", err)
	}
	if len(snapshot.Tasks) != 1 || safe_task_id(snapshot.Tasks[0]) != "explicit-db-task" {
		t.Fatalf("unexpected explicit database tasks: %+v", snapshot.Tasks)
	}
}

func TestPrefetchMigrationProfileCacheThenMigrateUsesCache(t *testing.T) {
	dir := t.TempDir()
	save_path := filepath.Join(dir, "demo.mp4")
	if err := os.WriteFile(save_path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write migrated file: %v", err)
	}

	task := must_gopeed_migration_task(t, "cache-task-1", dir, "demo.mp4", map[string]string{
		"id":    "12345",
		"oid":   "12345",
		"uid":   "67890",
		"title": "cached title",
	})

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		write_api_response(w, api_response{
			Code: 0,
			Msg:  "success",
			Data: channels_feed_profile{
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

	migration := &migration_server{}
	statuses := migration.prefetch_migration_profile_cache(server.Client(), dir, server.URL, []*downloadpkg.Task{task})
	server.Close()
	if requests != 1 {
		t.Fatalf("profile requests = %d want 1", requests)
	}
	if status := statuses["cache-task-1"]; status.Error != "" || status.CacheKey != "12345" {
		t.Fatalf("unexpected prefetch status: %+v", status)
	}
	if _, _, hit, err := migration.read_profile_cache(dir, "12345"); err != nil || !hit {
		t.Fatalf("read prefetched cache hit=%v err=%v", hit, err)
	}

	db, err := open_target_db(filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("open target db: %v", err)
	}
	defer close_target_db(db)

	item := migration.migrate_one_task(db, server.URL, dir, task, false, false, statuses)
	if item.Action != "migrated" || !item.ProfileCacheHit || item.TargetID <= 0 {
		t.Fatalf("unexpected migration item: %+v", item)
	}
	if requests != 1 {
		t.Fatalf("migration fetched profile again: requests=%d", requests)
	}
}

func TestMigrateOneTaskPreservesContentWhenProfileIsMissing(t *testing.T) {
	directory := t.TempDir()
	filename := filepath.Join(directory, "video.mp4")
	if err := os.WriteFile(filename, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	db, err := open_target_db(filepath.Join(directory, "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer close_target_db(db)
	task := must_gopeed_migration_task(t, "missing-profile", directory, "video.mp4", map[string]string{"id": "missing", "title": "Archived title", "key": "42"})
	migration := &migration_server{}
	item := migration.migrate_one_task(db, "", directory, task, false, false, nil)
	if item.Action != "migrated" {
		t.Fatalf("migration failed: %+v", item)
	}
	var record model.DownloadTask
	if err := db.First(&record, item.TargetID).Error; err != nil {
		t.Fatal(err)
	}
	if record.ContentId == nil {
		t.Fatal("content association missing")
	}
	var content model.Content
	if err := db.First(&content, "id = ?", *record.ContentId).Error; err != nil {
		t.Fatal(err)
	}
	if content.ExternalId != "missing" || content.Title != "Archived title" || !strings.Contains(content.Metadata, `"detail_missing":true`) {
		t.Fatalf("history lost: %+v", content)
	}
	var accounts int64
	if err := db.Model(&model.Account{}).Count(&accounts).Error; err != nil || accounts != 0 {
		t.Fatalf("invented account: %d %v", accounts, err)
	}
}

func TestMigrateRejectsHistoryWithoutContentIdentifier(t *testing.T) {
	directory := t.TempDir()
	db, err := open_target_db(filepath.Join(directory, "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer close_target_db(db)
	task := must_gopeed_migration_task(t, "no-id", directory, "video.mp4", nil)
	migration := &migration_server{}
	item := migration.migrate_one_task(db, "", directory, task, false, true, nil)
	if item.Action != "failed" {
		t.Fatalf("missing ID was accepted: %+v", item)
	}
}

func TestMigrateWXMPRecoversArchivedHTMLAndOriginalURL(t *testing.T) {
	directory := t.TempDir()
	body := `<html><title>Archived article</title><body><div class="rich_media_meta_nickname">Author</div><div class="rich_media_content"><p>Actual archived body</p></div></body></html>`
	if err := os.WriteFile(filepath.Join(directory, "article.html"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	db, err := open_target_db(filepath.Join(directory, "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer close_target_db(db)
	task := must_gopeed_migration_task(t, "article-history", directory, "article.html", map[string]string{"article_id": "123_1"})
	task.Meta.Req.URL = "officialaccount://https://mp.weixin.qq.com/s?__biz=biz&mid=123&idx=1&sn=real"
	if got := gopeed_article_url(task, "123_1"); got != "https://mp.weixin.qq.com/s?__biz=biz&mid=123&idx=1&sn=real" {
		t.Fatalf("lost original URL: %s", got)
	}
	migration := &migration_server{}
	item := migration.migrate_one_task(db, "", directory, task, false, false, nil)
	if item.Action != "migrated" {
		t.Fatalf("HTML recovery failed: %+v", item)
	}
	var record model.DownloadTask
	if err := db.First(&record, item.TargetID).Error; err != nil {
		t.Fatal(err)
	}
	if record.ContentId == nil {
		t.Fatal("article content missing")
	}
	var article model.ContentArticle
	if err := db.First(&article, "id = ?", *record.ContentId).Error; err != nil {
		t.Fatal(err)
	}
	if article.HTML != "<p>Actual archived body</p>" {
		t.Fatalf("body lost: %s", article.HTML)
	}
	var accounts int64
	if err := db.Model(&model.Account{}).Count(&accounts).Error; err != nil || accounts != 0 {
		t.Fatalf("invented account: %d %v", accounts, err)
	}
}

func TestMigrateWXMPArticleTaskUsesArticleCache(t *testing.T) {
	dir := t.TempDir()
	save_path := filepath.Join(dir, "article.html")
	if err := os.WriteFile(save_path, []byte("<html><body>cached</body></html>"), 0o644); err != nil {
		t.Fatalf("write migrated article: %v", err)
	}

	article_id := "k_F-1KYn-EPy27W9VoKZng"
	article_url := build_wxmp_article_url(article_id)
	content_raw := json.RawMessage(`{
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

	task := must_gopeed_migration_task(t, "mp-cache-task-1", dir, "article.html", map[string]string{
		"article_id": article_id,
	})
	migration := &migration_server{}
	cache_key := wxmp_article_profile_cache_key(article_id, article_url)
	if err := migration.write_profile_cache(dir, cache_key, profile_cache_entry{
		OID:        article_id,
		ProfileURL: article_url,
		Profile:    content_raw,
		CachedAt:   time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("write article cache: %v", err)
	}
	cache, err := load_profile_cache(profile_cache_path(dir))
	if err != nil {
		t.Fatalf("load profile cache: %v", err)
	}
	if cached, got_key := task_profile_cached(cache, task); !cached || got_key != cache_key {
		t.Fatalf("taskProfileCached = %v/%q want true/%q", cached, got_key, cache_key)
	}

	db, err := open_target_db(filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatalf("open target db: %v", err)
	}
	defer close_target_db(db)

	item := migration.migrate_one_task(db, "", dir, task, false, false, nil)
	if item.Action != "migrated" || !item.ProfileCacheHit || item.TargetID <= 0 {
		t.Fatalf("unexpected migration item: %+v", item)
	}
}

func must_gopeed_migration_task(t *testing.T, id string, download_dir string, filename string, labels map[string]string) *downloadpkg.Task {
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
				"path": download_dir,
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
	created_at := time.Unix(1710000000, 0)
	task.Status = base.DownloadStatusDone
	task.Progress = &downloadpkg.Progress{Downloaded: 5}
	task.CreatedAt = created_at
	task.UpdatedAt = created_at.Add(time.Minute)
	return &task
}

func write_gopeed_task(t *testing.T, dir string, task *downloadpkg.Task) {
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
	if err := storage.Put("task", safe_task_id(task), task); err != nil {
		t.Fatalf("write gopeed task: %v", err)
	}
}
