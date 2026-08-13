package ttkadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/cache"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/scraper/ttk"
)

// PlatformID is the platform identifier for TTK.
const PlatformID = "ttk"

func init() {
	adapter.Register(NewTTKAdapter())
}

// TTKAdapter converts TTK scraper results into shared models.
type TTKAdapter struct {
	runtime_mu      sync.RWMutex
	bus             *events.Bus
	file_cache      *cache.CacheProvider
	cookie_provider *cookies.Reader
}

var (
	_ adapter.PlatformAdapter             = (*TTKAdapter)(nil)
	_ adapter.ProgressFetchAdapter        = (*TTKAdapter)(nil)
	_ adapter.ContextProgressFetchAdapter = (*TTKAdapter)(nil)
	_ adapter.FetchCacheAdapter           = (*TTKAdapter)(nil)
	_ adapter.FetchDownloadTaskBuilder    = (*TTKAdapter)(nil)
	_ adapter.RuntimeAdapter              = (*TTKAdapter)(nil)
	_ adapter.RuntimeHandle               = (*TTKAdapter)(nil)
	_ adapter.PlatformStatusDescriber     = (*TTKAdapter)(nil)
)

// NewTTKAdapter creates a TTK platform adapter.
func NewTTKAdapter() *TTKAdapter {
	return &TTKAdapter{}
}

func (a *TTKAdapter) PlatformID() string { return PlatformID }

// PlatformStatuses describes the TTK availability entry shown by the host.
func (a *TTKAdapter) PlatformStatuses() []adapter.PlatformStatusDescriptor {
	return []adapter.PlatformStatusDescriptor{{
		Platform: PlatformID,
		Key:      PlatformID,
		Name:     "TT看书",
	}}
}

// Fetch retrieves a TTK novel profile and every chapter body.
func (a *TTKAdapter) Fetch(raw_url string) (any, error) {
	return a.FetchWithProgress(raw_url, "")
}

// FetchWithProgress associates emitted progress with request_id.
func (a *TTKAdapter) FetchWithProgress(raw_url string, request_id string) (any, error) {
	return a.FetchWithProgressContext(context.Background(), raw_url, adapter.FetchOptions{
		RequestID: request_id,
	})
}

// FetchWithProgressContext fetches a TTK novel with cancellation and cache
// control.
func (a *TTKAdapter) FetchWithProgressContext(fetch_context context.Context, raw_url string, options adapter.FetchOptions) (any, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return nil, fmt.Errorf("TT看书 URL 不能为空")
	}
	client := ttk.NewClient(a.runtime_cookie_provider())
	defer client.Close()
	client.SetPersistentCache(a.runtime_file_cache())
	client.SetProgressHandler(func(progress ttk.FetchProgress) {
		a.publish_progress(progress)
		a.emit_fetch_artifacts(progress, options.ArtifactHandler)
	})
	return client.Fetch(ttk.FetchParams{
		URL:          raw_url,
		RequestID:    strings.TrimSpace(options.RequestID),
		ForceRefresh: options.ForceRefresh,
		Context:      fetch_context,
	})
}

// RegisterRuntime attaches host cache, Cookie, and event capabilities.
func (a *TTKAdapter) RegisterRuntime(adapter_options *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if a == nil {
		return nil, fmt.Errorf("ttk adapter is nil")
	}
	if adapter_options == nil {
		return nil, fmt.Errorf("ttk runtime dependencies are nil")
	}
	a.runtime_mu.Lock()
	a.bus = adapter_options.Bus
	a.file_cache = adapter_options.Cache
	a.cookie_provider = adapter_options.Cookies
	a.runtime_mu.Unlock()
	if adapter_options.Logger != nil {
		adapter_options.Logger.Info().
			Str("component", "ttk_adapter").
			Bool("html_cache_enabled", adapter_options.Cache != nil && adapter_options.Cache.Enabled()).
			Bool("cookie_provider_available", adapter_options.Cookies != nil).
			Msg("ttk adapter runtime registered")
	}
	if adapter_options.Bus != nil {
		adapter_options.Bus.Publish(events.PlatformStatusChanged{
			Platform:  PlatformID,
			Key:       PlatformID,
			Name:      "TT看书",
			Status:    "available",
			Available: true,
		})
	}
	return a, nil
}

// Stop releases runtime references owned by the adapter.
func (a *TTKAdapter) Stop() {
	if a == nil {
		return
	}
	a.runtime_mu.Lock()
	a.bus = nil
	a.file_cache = nil
	a.cookie_provider = nil
	a.runtime_mu.Unlock()
}

func (a *TTKAdapter) runtime_file_cache() *cache.CacheProvider {
	if a == nil {
		return nil
	}
	a.runtime_mu.RLock()
	file_cache := a.file_cache
	a.runtime_mu.RUnlock()
	return file_cache
}

func (a *TTKAdapter) runtime_cookie_provider() *cookies.Reader {
	if a == nil {
		return nil
	}
	a.runtime_mu.RLock()
	cookie_provider := a.cookie_provider
	a.runtime_mu.RUnlock()
	return cookie_provider
}

func (a *TTKAdapter) publish_progress(progress ttk.FetchProgress) {
	if a == nil {
		return
	}
	a.runtime_mu.RLock()
	bus := a.bus
	a.runtime_mu.RUnlock()
	if bus == nil {
		return
	}
	bus.Publish(events.ScraperFetchProgress{
		RequestID:    progress.RequestID,
		Platform:     progress.Platform,
		URL:          progress.URL,
		BookID:       progress.BookID,
		BookTitle:    progress.BookTitle,
		Stage:        progress.Stage,
		Status:       progress.Status,
		Current:      progress.Current,
		Total:        progress.Total,
		Percent:      progress.Percent,
		ChapterID:    progress.ChapterID,
		ChapterTitle: progress.ChapterTitle,
		Message:      progress.Message,
		Error:        progress.Error,
		Cached:       progress.Cached,
		CacheHits:    progress.CacheHits,
	})
}

func (a *TTKAdapter) emit_fetch_artifacts(progress ttk.FetchProgress, artifact_handler adapter.FetchArtifactHandler) {
	if artifact_handler == nil {
		return
	}
	a.emit_fetch_cache_artifact(progress, artifact_handler)
	if progress.Stage == ttk.FetchStageDirectory && progress.Status == ttk.FetchStatusCompleted && progress.Profile != nil {
		partial_result := &ttk.TtkFetchResult{Profile: progress.Profile}
		content, content_err := ToContent(partial_result)
		if content_err != nil {
			return
		}
		artifact_handler(adapter.FetchArtifact{Stage: adapter.FetchArtifactStageContent, Content: content})
		if account, account_err := ToAccount(partial_result); account_err == nil && account != nil {
			artifact_handler(adapter.FetchArtifact{Stage: adapter.FetchArtifactStageAccount, Account: account})
		}
		if novel, novel_err := ToContentNovel(partial_result, content.Id); novel_err == nil {
			detail := adapter.ContentDetail{Type: "novel", Key: content.Id, Data: novel}
			artifact_handler(adapter.FetchArtifact{Stage: adapter.FetchArtifactStageContentDetail, ContentDetail: &detail})
		}
		return
	}
	if progress.Stage == ttk.FetchStageChapter && progress.Status == ttk.FetchStatusCompleted && progress.Chapter != nil {
		content_id := BuildContentID(progress.BookID)
		detail := fetched_chapter_content_detail(*progress.Chapter, content_id)
		artifact_handler(adapter.FetchArtifact{
			Stage:         adapter.FetchArtifactStageContentDetail,
			ContentDetail: &detail,
			Current:       progress.Current,
			Total:         progress.Total,
		})
	}
}

func (a *TTKAdapter) emit_fetch_cache_artifact(progress ttk.FetchProgress, artifact_handler adapter.FetchArtifactHandler) {
	if artifact_handler == nil || progress.Status != ttk.FetchStatusCompleted {
		return
	}
	file_cache := a.runtime_file_cache()
	source_url := strings.TrimSpace(progress.URL)
	if source_url == "" {
		return
	}

	if progress.Stage == ttk.FetchStageDirectory && progress.Profile != nil {
		profile_cache, err := ttk.LookupProfileHTMLCacheWithCache(file_cache, source_url)
		if err != nil || profile_cache == nil {
			return
		}
		cache_entry := adapter.FetchCacheEntry{
			Key:  "profile",
			Name: "作品页面",
			URL:  strings.TrimSpace(progress.Profile.URL),
			Path: profile_cache.Path,
			Size: profile_cache.Size,
		}
		artifact_handler(adapter.FetchArtifact{
			Stage:      adapter.FetchArtifactStageCacheEntry,
			CacheEntry: &cache_entry,
			Current:    1,
			Total:      progress.Total + 1,
		})
		return
	}

	if progress.Stage != ttk.FetchStageChapter || progress.Chapter == nil {
		return
	}
	chapter := progress.Chapter
	chapter_cache, err := ttk.LookupChapterHTMLCacheWithCache(file_cache, source_url, chapter.URL)
	if err != nil || chapter_cache == nil {
		return
	}
	chapter_index := chapter.Index
	if chapter_index <= 0 {
		chapter_index = progress.Current
	}
	chapter_name := strings.TrimSpace(chapter.Title)
	if chapter_name == "" {
		chapter_name = fmt.Sprintf("第 %d 章", chapter_index)
	}
	cache_entry := adapter.FetchCacheEntry{
		Key:  fmt.Sprintf("chapter-%d", chapter_index),
		Name: chapter_name,
		URL:  strings.TrimSpace(chapter.URL),
		Path: chapter_cache.Path,
		Size: chapter_cache.Size,
	}
	artifact_handler(adapter.FetchArtifact{
		Stage:      adapter.FetchArtifactStageCacheEntry,
		CacheEntry: &cache_entry,
		Current:    progress.Current + 1,
		Total:      progress.Total + 1,
	})
}

// FetchCacheEntries returns the profile and chapter HTML persisted by Fetch.
func (a *TTKAdapter) FetchCacheEntries(raw_url string, data any) ([]adapter.FetchCacheEntry, error) {
	result, err := ttk_result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	file_cache := a.runtime_file_cache()
	source_url := strings.TrimSpace(raw_url)
	if source_url == "" {
		source_url = strings.TrimSpace(result.Profile.URL)
	}
	entries := make([]adapter.FetchCacheEntry, 0, len(result.Chapters)+1)
	profile_cache, err := ttk.LookupProfileHTMLCacheWithCache(file_cache, source_url)
	if err != nil {
		return nil, err
	}
	if profile_cache != nil {
		entries = append(entries, adapter.FetchCacheEntry{
			Key:  "profile",
			Name: "作品页面",
			URL:  result.Profile.URL,
			Path: profile_cache.Path,
			Size: profile_cache.Size,
		})
	}
	for chapter_index, chapter := range result.Chapters {
		chapter_cache, cache_err := ttk.LookupChapterHTMLCacheWithCache(file_cache, source_url, chapter.URL)
		if cache_err != nil {
			return nil, cache_err
		}
		if chapter_cache == nil {
			continue
		}
		chapter_name := strings.TrimSpace(chapter.Title)
		if chapter_name == "" {
			chapter_name = fmt.Sprintf("第 %d 章", chapter_index+1)
		}
		entries = append(entries, adapter.FetchCacheEntry{
			Key:  fmt.Sprintf("chapter-%d", chapter_index+1),
			Name: chapter_name,
			URL:  chapter.URL,
			Path: chapter_cache.Path,
			Size: chapter_cache.Size,
		})
	}
	return entries, nil
}

// ClearFetchCache removes cached HTML associated with raw_url.
func (a *TTKAdapter) ClearFetchCache(raw_url string) (bool, error) {
	return ttk.ClearHTMLCacheWithCache(a.runtime_file_cache(), strings.TrimSpace(raw_url))
}

func (a *TTKAdapter) BuildBrowseHistory(content_json json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	result, err := ttk_result_from_json(content_json)
	if err != nil {
		return nil, err
	}
	content, err := ToContent(result)
	if err != nil {
		return nil, err
	}
	account, err := ToAccount(result)
	if err != nil {
		return nil, err
	}
	extra_data, _ := json.Marshal(map[string]any{
		"author":        strings.TrimSpace(result.Profile.Author),
		"cover_url":     strings.TrimSpace(result.Profile.CoverURL),
		"chapter_count": len(result.Profile.Chapters),
	})
	return &adapter.BrowseHistoryResult{
		BrowseHistory: &model.BrowseHistory{
			Id:           content.Id,
			PlatformId:   PlatformID,
			VisitedTimes: 1,
			Type:         content.Type,
			ExternalId:   content.ExternalId,
			Title:        content.Title,
			URL:          content.URL,
			SourceURL:    content.SourceURL,
			CoverURL:     content.CoverURL,
			ExtraData:    string(extra_data),
			Timestamps:   content.Timestamps,
		},
		Account: account,
	}, nil
}

func (a *TTKAdapter) BuildDownloadTask(content_json json.RawMessage, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	result, err := ttk_result_from_json(content_json)
	if err != nil || len(result.Chapters) == 0 {
		var params ttk.FetchParams
		_ = json.Unmarshal(content_json, &params)
		if strings.TrimSpace(params.URL) == "" && result != nil && result.Profile != nil {
			params.URL = result.Profile.URL
		}
		if strings.TrimSpace(params.URL) == "" {
			return nil, err
		}
		fetched, fetch_err := a.Fetch(params.URL)
		if fetch_err != nil {
			return nil, fetch_err
		}
		result, err = ttk_result_from_fetch(fetched)
		if err != nil {
			return nil, err
		}
	}
	return build_download_task(result, config_json)
}

// BuildDownloadTaskFromFetch builds a task from an in-memory Fetch result.
func (a *TTKAdapter) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	result, err := ttk_result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return build_download_task(result, config_json)
}
