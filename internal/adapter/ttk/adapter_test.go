package ttkadapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/cache"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/scraper/ttk"
)

func TestTTKAdapterConvertsCompleteFetchResult(t *testing.T) {
	result := test_fetch_result()
	content, err := ToContent(result)
	if err != nil {
		t.Fatalf("ToContent() error = %v", err)
	}
	if content.Id != "ttk:12345" || content.ExternalId != "12345" || content.Type != "novel" {
		t.Fatalf("ToContent() = %#v", content)
	}
	if content.CoverURL != "https://ttks.tw/files/article/image/115/115547/115547s.jpg" {
		t.Fatalf("Content CoverURL = %q", content.CoverURL)
	}
	account, err := ToAccount(result)
	if err != nil {
		t.Fatalf("ToAccount() error = %v", err)
	}
	if account == nil || account.Id != "ttk:测试作者" {
		t.Fatalf("ToAccount() = %#v", account)
	}
	details, err := ToContentDetails(result)
	if err != nil {
		t.Fatalf("ToContentDetails() error = %v", err)
	}
	if len(details) != 3 || details[0].Type != "novel" || details[1].Type != "novel_chapter" {
		t.Fatalf("ToContentDetails() = %#v", details)
	}
	first_chapter, ok := details[1].Data.(*model.ContentNovelChapter)
	if !ok || first_chapter.WordCount != 5 {
		t.Fatalf("first chapter = %#v", details[1].Data)
	}
}

func TestToAccountFallsBackToOfficialTTKAccount(t *testing.T) {
	result := test_fetch_result()
	result.Profile.Author = ""

	account, err := ToAccount(result)
	if err != nil {
		t.Fatalf("ToAccount() error = %v", err)
	}
	if account == nil {
		t.Fatal("ToAccount() = nil, want official account")
	}
	if account.Id != "ttk:ttk" || account.ExternalId != "ttk" || account.Nickname != "ttk" {
		t.Fatalf("official account = %#v", account)
	}
	if account.ProfileURL != "https://ttks.tw/" {
		t.Fatalf("official account ProfileURL = %q", account.ProfileURL)
	}
}

func TestBuildDownloadTaskFromFetchUsesInlineChapterText(t *testing.T) {
	adapter_instance := NewTTKAdapter()
	info, err := adapter_instance.BuildDownloadTaskFromFetch(
		test_fetch_result(),
		json.RawMessage(`{"filename":"自定义书名"}`),
	)
	if err != nil {
		t.Fatalf("BuildDownloadTaskFromFetch() error = %v", err)
	}
	if info.Task == nil || info.Task.PlatformId != PlatformID || info.Task.Name != "自定义书名" {
		t.Fatalf("download task = %#v", info.Task)
	}
	if info.Task.CoverURL != "https://ttks.tw/files/article/image/115/115547/115547s.jpg" {
		t.Fatalf("download task CoverURL = %q", info.Task.CoverURL)
	}
	if len(info.Resources) != 2 {
		t.Fatalf("resource count = %d, want 2", len(info.Resources))
	}
	first_resource := info.Resources[0]
	if first_resource.Resource.Kind != "text/plain" || len(first_resource.Endpoints) != 1 {
		t.Fatalf("first resource = %#v", first_resource)
	}
	if first_resource.Endpoints[0].Protocol != "inline" || !strings.Contains(first_resource.Endpoints[0].URL, "第一章\n\n第一章正文") {
		t.Fatalf("first endpoint = %#v", first_resource.Endpoints[0])
	}
	if len(first_resource.ContentAssets) != 1 || first_resource.ContentAssets[0].SubjectType != "novel_chapter" {
		t.Fatalf("first content assets = %#v", first_resource.ContentAssets)
	}
}

func TestRegisterRuntimeRetainsCookieProviderAndPublishesAvailable(t *testing.T) {
	bus := events.NewBus()
	cookie_provider := cookies.NewPersistentReader(t.TempDir())
	var received_status events.PlatformStatusChanged
	bus.Subscribe(events.TypePlatformStatusChanged, func(event events.Event) {
		received_status, _ = event.(events.PlatformStatusChanged)
	})

	adapter_instance := NewTTKAdapter()
	handle, err := adapter_instance.RegisterRuntime(&adapter.AdapterOptions{
		Bus:     bus,
		Cookies: cookie_provider,
	})
	if err != nil {
		t.Fatalf("RegisterRuntime() error = %v", err)
	}
	if handle != adapter_instance || adapter_instance.runtime_cookie_provider() != cookie_provider {
		t.Fatalf("runtime registration did not retain cookie provider")
	}
	if received_status.Platform != PlatformID || !received_status.Available || received_status.Name != "TT看书" {
		t.Fatalf("platform status = %#v", received_status)
	}
	adapter_instance.Stop()
	if adapter_instance.runtime_cookie_provider() != nil {
		t.Fatal("Stop() did not release cookie provider")
	}
}

func TestEmitFetchArtifactsIncludesNewCacheEntry(t *testing.T) {
	cache_registry, err := cache.NewProviderRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewProviderRegistry() error = %v", err)
	}
	file_cache, err := cache_registry.Namespace(PlatformID)
	if err != nil {
		t.Fatalf("Namespace() error = %v", err)
	}
	source_url := "https://ttks.tw/novel/12345"
	chapter := ttk.TtkFetchedChapter{
		Index: 1,
		URL:   "https://ttks.tw/chapter/1001",
		Title: "第一章",
	}
	cache_path, err := ttk.HTMLCacheFilePathWithCache(file_cache, source_url, chapter.URL)
	if err != nil {
		t.Fatalf("HTMLCacheFilePathWithCache() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cache_path), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	cache_data := []byte("<html>第一章</html>")
	if err := os.WriteFile(cache_path, cache_data, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	adapter_instance := NewTTKAdapter()
	if _, err := adapter_instance.RegisterRuntime(&adapter.AdapterOptions{Cache: file_cache}); err != nil {
		t.Fatalf("RegisterRuntime() error = %v", err)
	}
	defer adapter_instance.Stop()
	var cache_entry *adapter.FetchCacheEntry
	adapter_instance.emit_fetch_artifacts(ttk.FetchProgress{
		Platform: "ttk",
		URL:      source_url,
		Stage:    ttk.FetchStageChapter,
		Status:   ttk.FetchStatusCompleted,
		Current:  1,
		Total:    2,
		Chapter:  &chapter,
	}, func(artifact adapter.FetchArtifact) {
		if artifact.Stage == adapter.FetchArtifactStageCacheEntry {
			cache_entry = artifact.CacheEntry
		}
	})

	if cache_entry == nil {
		t.Fatal("cache entry artifact was not emitted")
	}
	if cache_entry.Key != "chapter-1" || cache_entry.Path != cache_path || cache_entry.Size != int64(len(cache_data)) {
		t.Fatalf("cache entry = %#v", cache_entry)
	}
}

func test_fetch_result() *ttk.TtkFetchResult {
	profile_url := "https://ttks.tw/novel/12345"
	return &ttk.TtkFetchResult{
		Profile: &ttk.TtkNovel{
			Title:    "测试小说",
			URL:      profile_url,
			Author:   "测试作者",
			CoverURL: "https://ttks.tw/files/article/image/115/115547/115547s.jpg",
			Chapters: []ttk.TtkChapter{
				{Index: 1, Title: "第一章", URL: "https://ttks.tw/chapter/1001"},
				{Index: 2, Title: "第二章", URL: "https://ttks.tw/chapter/1002"},
			},
		},
		Chapters: []ttk.TtkFetchedChapter{
			{Index: 1, Title: "第一章", URL: "https://ttks.tw/chapter/1001", Content: "第一章正文"},
			{Index: 2, Title: "第二章", URL: "https://ttks.tw/chapter/1002", Content: "第二章正文"},
		},
	}
}
