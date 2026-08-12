package bilibiliadapter

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/scraper/bilibili"
	"wx_channel/pkg/util"
)

const platformIDBilibili = "bilibili"

// PlatformID is the platform identifier for bilibili.
const PlatformID = platformIDBilibili

func init() {
	adapter.Register(&handler{})
}

type handler struct {
	runtime_mu sync.RWMutex
	logger     *zerolog.Logger
	config     *config.Config
}

var (
	_ adapter.PlatformAdapter      = (*handler)(nil)
	_ adapter.ProgressFetchAdapter = (*handler)(nil)
	_ adapter.RuntimeAdapter       = (*handler)(nil)
	_ adapter.RuntimeHandle        = (*handler)(nil)
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
	return bilibili.NewClientWithLogger(cookie, logger).GetVideoInfo(raw_url, 0)
}

func (h *handler) BuildBrowseHistory(contentJSON json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	return nil, adapter.ErrBrowseHistoryNotSupported
}

type bilibiliContentJSON struct {
	URL     string `json:"url"`
	PageNum int    `json:"page_num"`
}

func (h *handler) BuildDownloadTask(contentJSON json.RawMessage, configRaw json.RawMessage) (*adapter.DownloadTaskResult, error) {
	var config map[string]any
	if err := json.Unmarshal(configRaw, &config); err != nil {
		return nil, fmt.Errorf("解析下载配置失败: %w", err)
	}

	var input bilibiliContentJSON
	if err := json.Unmarshal(contentJSON, &input); err != nil {
		return nil, fmt.Errorf("解析B站URL失败: %w", err)
	}
	if input.URL == "" {
		return nil, fmt.Errorf("B站URL不能为空")
	}

	cookie := h.config_string("bilibili.cookie")

	// Call scraper to get video info
	client := bilibili.NewClientWithLogger(cookie, h.get_logger())
	videoInfos, err := client.GetVideoInfo(input.URL, input.PageNum)
	if err != nil {
		return nil, fmt.Errorf("获取B站视频信息失败: %w", err)
	}
	if len(videoInfos) == 0 {
		return nil, fmt.Errorf("未获取到B站视频")
	}

	return buildTaskFromVideoInfo(videoInfos[0], input.URL, config)
}

func buildTaskFromVideoInfo(info *bilibili.VideoInfo, sourceURL string, config map[string]any) (*adapter.DownloadTaskResult, error) {
	now := util.NowMillis()

	content := &model.Content{
		Id:         BuildContentID(info.VideoID),
		PlatformId: platformIDBilibili,
		ExternalId: info.VideoID,
		Type:       "video",
		Title:      info.Title,
		URL:        info.URL,
		CoverURL:   info.CoverURL,
		SourceURL:  sourceURL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	account := &model.Account{
		Id:         BuildAccountID(info.VideoID),
		PlatformId: platformIDBilibili,
		ExternalId: info.VideoID,
		Nickname:   "B站用户",
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	title, _ := config["filename"].(string)
	if title == "" {
		title = info.Title
	}

	download_dir, _ := config["download_dir"].(string)
	if download_dir == "" {
		download_dir = "/downloads/bilibili"
	}

	configJSON, _ := json.Marshal(buildConfigJSON(config))
	metadataJSON, _ := json.Marshal(map[string]any{
		"platform":    PlatformID,
		"external_id": info.VideoID,
		"title":       info.Title,
		"download_at": time.Now().Unix(),
	})

	extraJSON := buildExtraJSON(info.VideoID, info.Title)
	contentID := content.Id

	var resources []*adapter.ResourceInfo

	// Cover resource (optional)
	downloadCover, _ := config["download_cover"].(bool)
	if downloadCover && info.CoverURL != "" {
		coverResource := model.DownloadResource{
			ContentId: &contentID,
			Name:      title,
			Kind:      "image",
			UniqueID:  info.VideoID + "_cover",
			Extra:     extraJSON,
		}
		resources = append(resources, &adapter.ResourceInfo{
			Resource: coverResource,
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "https",
				URL:      info.CoverURL,
				Enabled:  1,
			}},
		})
	}

	// Video resource
	videoResource := model.DownloadResource{
		ContentId: &contentID,
		Name:      title + ".mp4",
		Kind:      "video",
		UniqueID:  info.VideoID,
		Extra:     extraJSON,
	}
	videoEndpoint := model.DownloadEndpoint{
		Protocol: "https",
		URL:      info.URL,
		Enabled:  1,
	}
	resources = append(resources, &adapter.ResourceInfo{
		Resource:  videoResource,
		Endpoints: []model.DownloadEndpoint{videoEndpoint},
		ContentAssets: []adapter.ContentAssetReference{{
			Kind:     model.ContentAssetKindVideo,
			Role:     model.ContentAssetRoleVideoVariant,
			AssetKey: "default",
			Relation: model.DownloadResourceAssetRelationSource,
		}},
	})

	// DASH audio resource (anime/courses etc.)
	if info.AudioURL != "" {
		audioResource := model.DownloadResource{
			ContentId:  &contentID,
			Name:       title + ".audio.m4a",
			Kind:       "audio",
			UniqueID:   info.VideoID + "_audio",
			MergeOrder: 1,
			Extra:      extraJSON,
		}
		resources = append(resources, &adapter.ResourceInfo{
			Resource: audioResource,
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "https",
				URL:      info.AudioURL,
				Enabled:  1,
			}},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind:     model.ContentAssetKindAudio,
				Role:     model.ContentAssetRoleAudioVariant,
				AssetKey: info.VideoID + "_audio",
				Relation: model.DownloadResourceAssetRelationSource,
			}},
		})
	}

	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content.Id,
			Name:         title,
			UniqueID:     info.VideoID,
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			ConfigJSON:   string(configJSON),
			MetadataJSON: string(metadataJSON),
		},
		Resources: resources,
		ContentDetail: &model.ContentVideo{
			Id:     content.Id,
			URL:    info.URL,
			Format: "mp4",
		},
		Account: account,
		Content: content,
	}, nil
}

func BuildContentID(externalID string) string {
	return platformIDBilibili + ":" + externalID
}

func BuildAccountID(externalID string) string {
	return platformIDBilibili + ":" + externalID
}

func buildExtraJSON(id, title string) string {
	data, _ := json.Marshal(map[string]string{
		"id":    id,
		"title": title,
	})
	return string(data)
}

func buildConfigJSON(config map[string]any) map[string]any {
	m := make(map[string]any, len(config))
	for key, value := range config {
		m[key] = value
	}
	return m
}
