package kuaishouadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/scraper/kuaishou"
)

// PlatformID is the platform identifier for Kuaishou.
const PlatformID = kuaishou.PlatformID

func init() {
	adapter.Register(NewKuaishouAdapter())
}

// KuaishouAdapter connects the Kuaishou scraper to the shared archive and
// download models.
type KuaishouAdapter struct {
	runtime_mu     sync.RWMutex
	runtime_config *config.Config
	cookie_reader  *cookies.Reader
}

var (
	_ adapter.PlatformAdapter             = (*KuaishouAdapter)(nil)
	_ adapter.ContextProgressFetchAdapter = (*KuaishouAdapter)(nil)
	_ adapter.FetchDownloadTaskBuilder    = (*KuaishouAdapter)(nil)
	_ adapter.RuntimeAdapter              = (*KuaishouAdapter)(nil)
	_ adapter.RuntimeHandle               = (*KuaishouAdapter)(nil)
	_ adapter.PlatformStatusDescriber     = (*KuaishouAdapter)(nil)
)

// NewKuaishouAdapter creates a Kuaishou adapter.
func NewKuaishouAdapter() *KuaishouAdapter {
	return &KuaishouAdapter{}
}

func (a *KuaishouAdapter) PlatformID() string { return PlatformID }

// PlatformStatuses describes the public Kuaishou video scraper.
func (a *KuaishouAdapter) PlatformStatuses() []adapter.PlatformStatusDescriptor {
	return []adapter.PlatformStatusDescriptor{{
		Platform: PlatformID,
		Key:      PlatformID,
		Name:     "快手",
	}}
}

// RegisterRuntime stores optional configuration and cookie access and
// publishes scraper availability.
func (a *KuaishouAdapter) RegisterRuntime(adapter_options *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if a == nil {
		return nil, fmt.Errorf("kuaishou adapter is nil")
	}
	if adapter_options == nil {
		return nil, fmt.Errorf("kuaishou runtime dependencies are nil")
	}
	a.runtime_mu.Lock()
	a.runtime_config = adapter_options.Config
	a.cookie_reader = adapter_options.Cookies
	a.runtime_mu.Unlock()
	if adapter_options.Bus != nil {
		adapter_options.Bus.Publish(events.PlatformStatusChanged{
			Platform:  PlatformID,
			Key:       PlatformID,
			Name:      "快手",
			Status:    "available",
			Available: true,
		})
	}
	return a, nil
}

// Stop releases runtime references.
func (a *KuaishouAdapter) Stop() {
	if a == nil {
		return
	}
	a.runtime_mu.Lock()
	a.runtime_config = nil
	a.cookie_reader = nil
	a.runtime_mu.Unlock()
}

// Fetch resolves a Kuaishou share link and retrieves its video metadata.
func (a *KuaishouAdapter) Fetch(raw_url string) (any, error) {
	return a.FetchWithProgressContext(context.Background(), raw_url, adapter.FetchOptions{})
}

// FetchWithProgressContext retrieves a Kuaishou video with cancellation.
func (a *KuaishouAdapter) FetchWithProgressContext(fetch_context context.Context, raw_url string, _ adapter.FetchOptions) (any, error) {
	source_url, err := kuaishou.ExtractURL(raw_url)
	if err != nil {
		return nil, fmt.Errorf("解析快手 URL 失败: %w", err)
	}
	client := kuaishou.NewClient()
	defer client.Close()
	client.SetCookie(a.kuaishou_cookie())
	return client.FetchContext(fetch_context, source_url)
}

// ToContent converts a Kuaishou fetch result to shared content.
func (a *KuaishouAdapter) ToContent(data any) (*model.Content, error) {
	result, err := fetch_result_from_data(data)
	if err != nil {
		return nil, err
	}
	return to_content(result)
}

// ToAccount converts a Kuaishou publisher to a shared account.
func (a *KuaishouAdapter) ToAccount(data any) (*model.Account, error) {
	result, err := fetch_result_from_data(data)
	if err != nil {
		return nil, err
	}
	return to_account(result)
}

// ToContentDetails converts Kuaishou video metadata to a shared detail.
func (a *KuaishouAdapter) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	result, err := fetch_result_from_data(data)
	if err != nil {
		return nil, err
	}
	return to_content_details(result)
}

// BuildDownloadTask accepts either a complete fetch result or a URL-only
// object.
func (a *KuaishouAdapter) BuildDownloadTask(content_json json.RawMessage, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	result, result_err := fetch_result_from_data(content_json)
	if result_err == nil {
		return build_download_task(result, config_json)
	}
	var input struct {
		URL       string `json:"url"`
		SourceURL string `json:"source_url"`
	}
	if err := json.Unmarshal(content_json, &input); err != nil {
		return nil, result_err
	}
	raw_url := first_non_empty(input.URL, input.SourceURL)
	if raw_url == "" {
		return nil, result_err
	}
	fetched, err := a.Fetch(raw_url)
	if err != nil {
		return nil, err
	}
	result, err = fetch_result_from_data(fetched)
	if err != nil {
		return nil, err
	}
	return build_download_task(result, config_json)
}

// BuildDownloadTaskFromFetch builds a task without requesting Kuaishou again.
func (a *KuaishouAdapter) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	result, err := fetch_result_from_data(data)
	if err != nil {
		return nil, err
	}
	return build_download_task(result, config_json)
}

// BuildBrowseHistory converts a fetched Kuaishou video to browse history.
func (a *KuaishouAdapter) BuildBrowseHistory(content_json json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	result, err := fetch_result_from_data(content_json)
	if err != nil {
		return nil, err
	}
	content, err := to_content(result)
	if err != nil {
		return nil, err
	}
	account, err := to_account(result)
	if err != nil {
		return nil, err
	}
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
			PublishTime:  content.PublishTime,
			ExtraData:    content.Metadata,
			Timestamps:   content.Timestamps,
		},
		Account: account,
	}, nil
}

func (a *KuaishouAdapter) config_string(key string) string {
	if a == nil {
		return ""
	}
	a.runtime_mu.RLock()
	runtime_config := a.runtime_config
	a.runtime_mu.RUnlock()
	if runtime_config == nil {
		return ""
	}
	return strings.TrimSpace(runtime_config.GetString(key))
}

func (a *KuaishouAdapter) kuaishou_cookie() string {
	if configured_cookie := a.config_string("kuaishou.cookie"); configured_cookie != "" {
		return configured_cookie
	}
	if a == nil {
		return ""
	}
	a.runtime_mu.RLock()
	cookie_reader := a.cookie_reader
	a.runtime_mu.RUnlock()
	if cookie_reader == nil {
		return ""
	}
	cookie, err := cookie_reader.HeaderForDomain("kuaishou.com")
	if err != nil {
		return ""
	}
	return cookie
}
