package bilibiliadapter

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	"wx_channel/internal/events"
	"wx_channel/pkg/scraper/bilibili"
)

const platform_id_bilibili = "bilibili"

// PlatformID is the platform identifier for bilibili.
const PlatformID = platform_id_bilibili

func init() {
	adapter.Register(&handler{})
}

type handler struct {
	runtime_mu sync.RWMutex
	logger     *zerolog.Logger
	config     *config.Config
}

var (
	_ adapter.PlatformAdapter          = (*handler)(nil)
	_ adapter.ProgressFetchAdapter     = (*handler)(nil)
	_ adapter.FetchDownloadTaskBuilder = (*handler)(nil)
	_ adapter.RuntimeAdapter           = (*handler)(nil)
	_ adapter.RuntimeHandle            = (*handler)(nil)
)

func (h *handler) PlatformID() string { return PlatformID }

// RegisterRuntime attaches the application logger to the Bilibili adapter.
func (h *handler) RegisterRuntime(adapter_options *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if h == nil {
		return nil, fmt.Errorf("bilibili adapter is nil")
	}
	if adapter_options == nil {
		return nil, fmt.Errorf("bilibili runtime dependencies are nil")
	}
	h.runtime_mu.Lock()
	h.logger = adapter_options.Logger
	h.config = adapter_options.Config
	h.runtime_mu.Unlock()
	if adapter_options.Logger != nil {
		adapter_options.Logger.Info().
			Str("component", "bilibili_adapter").
			Msg("bilibili adapter runtime registered")
	}
	if adapter_options.Bus != nil {
		adapter_options.Bus.Publish(events.PlatformStatusChanged{
			Platform:  PlatformID,
			Status:    "available",
			Available: true,
		})
	}
	return h, nil
}

// Stop releases the Bilibili adapter runtime.
func (h *handler) Stop() {
	if h == nil {
		return
	}
	h.runtime_mu.Lock()
	h.logger = nil
	h.config = nil
	h.runtime_mu.Unlock()
}

func (h *handler) get_logger() *zerolog.Logger {
	if h == nil {
		return nil
	}
	h.runtime_mu.RLock()
	defer h.runtime_mu.RUnlock()
	return h.logger
}

func (h *handler) config_string(key string) string {
	if h == nil {
		return ""
	}
	h.runtime_mu.RLock()
	runtime_config := h.config
	h.runtime_mu.RUnlock()
	if runtime_config == nil {
		return ""
	}
	return runtime_config.GetString(key)
}

func (h *handler) Fetch(raw_url string) (any, error) {
	return h.fetch(raw_url, "")
}

// FetchWithProgress adds the scraper job ID to Bilibili API diagnostics.
func (h *handler) FetchWithProgress(raw_url string, request_id string) (any, error) {
	return h.fetch(raw_url, strings.TrimSpace(request_id))
}

func (h *handler) fetch(raw_url string, request_id string) (any, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return nil, fmt.Errorf("B站URL不能为空")
	}

	cookie := h.config_string("bilibili.cookie")
	logger := h.get_logger()
	if logger != nil && request_id != "" {
		request_logger := logger.With().Str("job_id", request_id).Logger()
		logger = &request_logger
	}
	client := bilibili.NewClientWithLogger(cookie, logger)
	return client.Fetch(raw_url)
}

func (h *handler) BuildBrowseHistory(_ json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	return nil, adapter.ErrBrowseHistoryNotSupported
}

type bilibili_content_json struct {
	URL       string `json:"url"`
	SourceURL string `json:"source_url"`
	PageNum   int    `json:"page_num"`
}

func (h *handler) BuildDownloadTask(content_json json.RawMessage, config_raw json.RawMessage) (*adapter.DownloadTaskResult, error) {
	var config map[string]any
	if err := json.Unmarshal(config_raw, &config); err != nil {
		return nil, fmt.Errorf("解析下载配置失败: %w", err)
	}

	// The create API may receive the structured value previously returned by
	// Fetch. In particular, bangumi results expose source_url rather than the
	// small {url, page_num} request shape. Reuse the embedded streams instead of
	// treating that result as an empty URL or fetching the page again.
	if bangumi_info, is_bangumi, err := bilibili_bangumi_info_from_fetch(content_json); err != nil {
		return nil, fmt.Errorf("解析B站番剧信息失败: %w", err)
	} else if is_bangumi {
		return build_task_from_bangumi_info(bangumi_info, config)
	}

	var input bilibili_content_json
	if err := json.Unmarshal(content_json, &input); err != nil {
		return nil, fmt.Errorf("解析B站URL失败: %w", err)
	}
	if strings.TrimSpace(input.URL) == "" {
		input.URL = input.SourceURL
	}
	input.URL = strings.TrimSpace(input.URL)
	if input.URL == "" {
		return nil, fmt.Errorf("B站URL不能为空")
	}

	cookie := h.config_string("bilibili.cookie")

	client := bilibili.NewClientWithLogger(cookie, h.get_logger())
	if bilibili.IsBangumiURL(input.URL) {
		data, err := client.Fetch(input.URL)
		if err != nil {
			return nil, fmt.Errorf("获取B站番剧信息失败: %w", err)
		}
		bangumi_info, is_bangumi, err := bilibili_bangumi_info_from_fetch(data)
		if err != nil {
			return nil, fmt.Errorf("解析B站番剧信息失败: %w", err)
		}
		if !is_bangumi {
			return nil, fmt.Errorf("B站番剧返回了不支持的数据类型 %T", data)
		}
		bangumi_info.SourceURL = input.URL
		return build_task_from_bangumi_info(bangumi_info, config)
	}

	video_infos, err := client.GetVideoInfo(input.URL, input.PageNum)
	if err != nil {
		return nil, fmt.Errorf("获取B站视频信息失败: %w", err)
	}
	if len(video_infos) == 0 {
		return nil, fmt.Errorf("未获取到B站视频")
	}

	return build_task_from_video_info(video_infos[0], input.URL, config)
}
