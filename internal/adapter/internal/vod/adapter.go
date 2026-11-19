package vod

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
)

type Handler struct {
	spec            Specification
	platform        string
	runtime_mu      sync.RWMutex
	config          *config.Config
	cookie_provider *cookies.Reader
}

var (
	_ adapter.Postprocessor               = (*Handler)(nil)
	_ adapter.PlatformAdapter             = (*Handler)(nil)
	_ adapter.RuntimeAdapter              = (*Handler)(nil)
	_ adapter.RuntimeHandle               = (*Handler)(nil)
	_ adapter.ContextProgressFetchAdapter = (*Handler)(nil)
	_ adapter.FetchDownloadTaskBuilder    = (*Handler)(nil)
)

// NewHandler provides common runtime wiring after the concrete adapter has
// supplied its native fetcher and decoder. It does not choose source platforms.
func NewHandler(spec Specification) *Handler { return &Handler{platform: spec.PlatformID, spec: spec} }

func (h *Handler) PlatformID() string { return h.platform }
func (h *Handler) RegisterRuntime(options *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if options == nil {
		return nil, fmt.Errorf("%s runtime dependencies are nil", h.platform)
	}
	h.runtime_mu.Lock()
	h.config = options.Config
	h.cookie_provider = options.Cookies
	h.runtime_mu.Unlock()
	if options.Bus != nil {
		options.Bus.Publish(events.PlatformStatusChanged{Platform: h.platform, Status: "available", Available: true})
	}
	return h, nil
}
func (h *Handler) Stop() {
	h.runtime_mu.Lock()
	h.config = nil
	h.cookie_provider = nil
	h.runtime_mu.Unlock()
}
func (h *Handler) cookie() string {
	h.runtime_mu.RLock()
	runtime_config, provider := h.config, h.cookie_provider
	h.runtime_mu.RUnlock()
	if runtime_config != nil {
		if cookie := strings.TrimSpace(runtime_config.GetString(h.platform + ".cookie")); cookie != "" {
			return cookie
		}
	}
	if provider != nil {
		cookie, _ := provider.HeaderForURL(h.spec.Homepage)
		return cookie
	}
	return ""
}
func (h *Handler) Fetch(raw_url string) (any, error) {
	return h.FetchWithProgressContext(context.Background(), raw_url, adapter.FetchOptions{})
}
func (h *Handler) FetchWithProgressContext(ctx context.Context, raw_url string, _ adapter.FetchOptions) (any, error) {
	return h.spec.Fetch(ctx, raw_url, h.cookie())
}
func (h *Handler) result(data any) (*Result, error) {
	result, err := h.spec.Parse(data)
	if err != nil {
		return nil, err
	}
	if result.Platform != h.platform || len(result.Episodes) == 0 || (result.Profile.Kind == "series" && strings.TrimSpace(result.Profile.ID) == "") {
		return nil, fmt.Errorf("%s 抓取结果无效", h.platform)
	}
	if _, _, _, err := h.spec.ParseURL(result.SourceURL); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, episode := range result.Episodes {
		_, _, video_id, err := h.spec.ParseURL(episode.SourceURL)
		if err != nil || video_id != episode.ID || video_id == "" || seen[video_id] {
			return nil, fmt.Errorf("%s 分集编号或来源无效", h.platform)
		}
		seen[video_id] = true
	}
	return result, nil
}
func (h *Handler) ToContent(data any) (*model.Content, error) {
	result, err := h.result(data)
	if err != nil {
		return nil, err
	}
	root, _, _, _ := h.build_models(result)
	return root, nil
}
func (h *Handler) ToAccount(data any) (*model.Account, error) {
	if _, err := h.result(data); err != nil {
		return nil, err
	}
	return h.platform_account(), nil
}
func (h *Handler) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	result, err := h.result(data)
	if err != nil {
		return nil, err
	}
	_, _, details, _ := h.build_models(result)
	return details, nil
}
func (h *Handler) BuildDownloadTask(content_json, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	result, err := h.result(content_json)
	if err != nil {
		var input struct {
			URL       string `json:"url"`
			SourceURL string `json:"source_url"`
		}
		if err := json.Unmarshal(content_json, &input); err != nil {
			return nil, err
		}
		raw_url := input.URL
		if raw_url == "" {
			raw_url = input.SourceURL
		}
		data, fetch_err := h.spec.Fetch(context.Background(), raw_url, h.cookie())
		if fetch_err != nil {
			return nil, fetch_err
		}
		result, err = h.result(data)
		if err != nil {
			return nil, err
		}
	}
	return h.build_task(result, config_json)
}
func (h *Handler) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	result, err := h.result(data)
	if err != nil {
		return nil, err
	}
	return h.build_task(result, config_json)
}
func (h *Handler) BuildBrowseHistory(json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	return nil, adapter.ErrBrowseHistoryNotSupported
}

type plugin_config struct {
	platform string
	name     string
}

func NewConfig(spec Specification) config.Configurable {
	return &plugin_config{platform: spec.PlatformID, name: spec.Name}
}

func (c *plugin_config) ConfigNamespace() string { return c.platform }
func (c *plugin_config) ConfigSchema() []config.ConfigField {
	return []config.ConfigField{{Key: "cookie", Type: config.ConfigTypeText, Default: "", Title: c.name + " Cookie", Description: "使用具有对应视频播放权限的账号 Cookie", Group: c.name, HotReload: true}}
}
func (c *plugin_config) ApplyConfig(*config.SubViper) error { return nil }
