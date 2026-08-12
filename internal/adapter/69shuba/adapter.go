package shuba69adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/cache"
	shuba69 "wx_channel/pkg/scraper/69shuba"
)

// PlatformID is the platform identifier for 69shuba.
const PlatformID = "69shuba"

func init() {
	adapter.Register(NewShuba69Adapter())
}

// Shuba69Adapter converts 69shuba scraper results into shared models.
type Shuba69Adapter struct {
	runtime_mu sync.RWMutex
	logger     *zerolog.Logger
	config     *config.Config
	file_cache *cache.CacheProvider
}

var (
	_ adapter.PlatformAdapter             = (*Shuba69Adapter)(nil)
	_ adapter.ProgressFetchAdapter        = (*Shuba69Adapter)(nil)
	_ adapter.ContextProgressFetchAdapter = (*Shuba69Adapter)(nil)
	_ adapter.RuntimeAdapter              = (*Shuba69Adapter)(nil)
	_ adapter.RuntimeHandle               = (*Shuba69Adapter)(nil)
)

// NewShuba69Adapter creates a 69shuba platform adapter.
func NewShuba69Adapter() *Shuba69Adapter {
	return &Shuba69Adapter{}
}

func (a *Shuba69Adapter) PlatformID() string { return PlatformID }

// Fetch retrieves a novel profile and its chapter directory from 69shuba.
func (a *Shuba69Adapter) Fetch(raw_url string) (any, error) {
	return a.FetchWithProgress(raw_url, "")
}

// FetchWithProgress associates diagnostic logs with a scraper job ID.
func (a *Shuba69Adapter) FetchWithProgress(raw_url string, request_id string) (any, error) {
	return a.FetchWithProgressContext(context.Background(), raw_url, adapter.FetchOptions{
		RequestID: request_id,
	})
}

// FetchWithProgressContext fetches a novel directory with cancellation and
// job-correlated structured diagnostics.
func (a *Shuba69Adapter) FetchWithProgressContext(fetch_context context.Context, raw_url string, options adapter.FetchOptions) (any, error) {
	raw_url = strings.TrimSpace(raw_url)
	logger := a.request_logger(options.RequestID)
	if raw_url == "" {
		err := fmt.Errorf("69书吧 URL 不能为空")
		if logger != nil {
			logger.Error().Err(err).Msg("69shuba adapter: invalid fetch request")
		}
		return nil, err
	}
	cookie := a.config_string("69shuba.cookie")
	configured_fetcher := strings.TrimSpace(a.config_string("69shuba.fetcher"))
	if logger != nil && configured_fetcher != "" && configured_fetcher != "clawreq" {
		logger.Warn().
			Str("configured_fetcher", configured_fetcher).
			Str("active_fetcher", "clawreq").
			Msg("69shuba adapter: configured fetcher is not implemented; using clawreq")
	}
	fetch_started_at := time.Now()
	file_cache := a.runtime_file_cache()
	if logger != nil {
		logger.Info().
			Str("url", raw_url).
			Str("http_client", "clawreq").
			Bool("has_cookie", strings.TrimSpace(cookie) != "").
			Bool("html_cache_enabled", file_cache != nil && file_cache.Enabled()).
			Msg("69shuba adapter: fetch started")
	}
	client := shuba69.NewClientWithLogger(cookie, "", logger)
	client.SetPersistentCache(file_cache)
	novel, err := client.FetchNovelChaptersContext(fetch_context, raw_url)
	if err != nil {
		if logger != nil {
			logger.Error().
				Err(err).
				Str("url", raw_url).
				Str("http_client", "clawreq").
				Dur("elapsed", time.Since(fetch_started_at)).
				Msg("69shuba adapter: fetch failed")
		}
		return nil, err
	}
	if logger != nil {
		logger.Info().
			Str("url", raw_url).
			Str("book_id", novel.BookID).
			Str("book_title", novel.Title).
			Str("cover_url", novel.CoverURL).
			Int("chapter_count", len(novel.Chapters)).
			Dur("elapsed", time.Since(fetch_started_at)).
			Msg("69shuba adapter: fetch completed")
	}
	return novel, nil
}

// RegisterRuntime attaches the application logger to the adapter.
func (a *Shuba69Adapter) RegisterRuntime(adapter_options *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if a == nil {
		return nil, fmt.Errorf("69shuba adapter is nil")
	}
	if adapter_options == nil {
		return nil, fmt.Errorf("69shuba runtime dependencies are nil")
	}
	file_cache := adapter_options.Cache
	a.runtime_mu.Lock()
	a.logger = adapter_options.Logger
	a.config = adapter_options.Config
	a.file_cache = file_cache
	a.runtime_mu.Unlock()
	if adapter_options.Logger != nil {
		adapter_options.Logger.Info().
			Str("component", "69shuba_adapter").
			Str("http_client", "clawreq").
			Str("work_dir", a.runtime_work_dir()).
			Bool("html_cache_enabled", file_cache != nil && file_cache.Enabled()).
			Msg("69shuba adapter runtime registered")
	}
	return a, nil
}

// Stop releases runtime references owned by the adapter.
func (a *Shuba69Adapter) Stop() {
	if a == nil {
		return
	}
	a.runtime_mu.Lock()
	a.logger = nil
	a.config = nil
	a.file_cache = nil
	a.runtime_mu.Unlock()
}

func (a *Shuba69Adapter) get_logger() *zerolog.Logger {
	if a == nil {
		return nil
	}
	a.runtime_mu.RLock()
	logger := a.logger
	a.runtime_mu.RUnlock()
	return logger
}

func (a *Shuba69Adapter) request_logger(request_id string) *zerolog.Logger {
	logger := a.get_logger()
	if logger == nil {
		return nil
	}
	log_context := logger.With().Str("component", "69shuba_adapter")
	if request_id = strings.TrimSpace(request_id); request_id != "" {
		log_context = log_context.Str("job_id", request_id)
	}
	request_logger := log_context.Logger()
	return &request_logger
}

func (a *Shuba69Adapter) runtime_work_dir() string {
	if a == nil {
		return ""
	}
	a.runtime_mu.RLock()
	runtime_config := a.config
	a.runtime_mu.RUnlock()
	if runtime_config == nil {
		return ""
	}
	return strings.TrimSpace(runtime_config.WorkDir)
}

func (a *Shuba69Adapter) runtime_file_cache() *cache.CacheProvider {
	if a == nil {
		return nil
	}
	a.runtime_mu.RLock()
	file_cache := a.file_cache
	a.runtime_mu.RUnlock()
	return file_cache
}

func (a *Shuba69Adapter) config_string(key string) string {
	if a == nil {
		return ""
	}
	a.runtime_mu.RLock()
	runtime_config := a.config
	a.runtime_mu.RUnlock()
	if runtime_config == nil {
		return ""
	}
	return runtime_config.GetString(key)
}

func (a *Shuba69Adapter) BuildBrowseHistory(content_json json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	novel, err := novel_from_json(content_json)
	if err != nil {
		return nil, err
	}
	content, err := ToContent(novel)
	if err != nil {
		return nil, err
	}
	account, err := ToAccount(novel)
	if err != nil {
		return nil, err
	}
	extra_data, _ := json.Marshal(map[string]any{
		"author":        strings.TrimSpace(novel.Author),
		"category":      strings.TrimSpace(novel.Category),
		"status":        strings.TrimSpace(novel.Status),
		"chapter_count": len(novel.Chapters),
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

func (a *Shuba69Adapter) BuildDownloadTask(content_json json.RawMessage, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	novel, err := novel_from_json(content_json)
	if err != nil {
		var params struct {
			URL string `json:"url"`
		}
		if decode_err := json.Unmarshal(content_json, &params); decode_err != nil || strings.TrimSpace(params.URL) == "" {
			return nil, err
		}
		fetched, fetch_err := a.Fetch(params.URL)
		if fetch_err != nil {
			return nil, fetch_err
		}
		novel, err = novel_from_fetch(fetched)
		if err != nil {
			return nil, err
		}
	}

	cookie := a.config_string("69shuba.cookie")
	return build_download_task(novel, config_json, cookie)
}
