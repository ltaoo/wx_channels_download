package douyinadapter

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/scraper/douyin"
)

const platform_id_douyin = "douyin"

// PlatformID is the platform identifier for douyin.
const PlatformID = platform_id_douyin

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

// RegisterRuntime attaches the application logger to the Douyin adapter.
func (h *handler) RegisterRuntime(deps *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if h == nil {
		return nil, fmt.Errorf("douyin adapter is nil")
	}
	if deps == nil {
		return nil, fmt.Errorf("douyin runtime dependencies are nil")
	}
	h.runtime_mu.Lock()
	h.logger = deps.Logger
	h.config = deps.Config
	h.runtime_mu.Unlock()
	if deps.Logger != nil {
		deps.Logger.Info().
			Str("component", "douyin_adapter").
			Msg("douyin adapter runtime registered")
	}
	if deps.Bus != nil {
		deps.Bus.Publish(events.PlatformStatusChanged{
			Platform:  PlatformID,
			Status:    "available",
			Available: true,
		})
	}
	return h, nil
}

// Stop releases the Douyin adapter runtime.
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

// FetchWithProgress adds the scraper job ID to all Douyin diagnostic logs.
func (h *handler) FetchWithProgress(raw_url string, request_id string) (any, error) {
	return h.fetch(raw_url, strings.TrimSpace(request_id))
}

func (h *handler) fetch(raw_url string, request_id string) (any, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return nil, fmt.Errorf("抖音URL不能为空")
	}

	cookie := h.config_string("douyin.cookie")
	logger := h.get_logger()
	if logger != nil && request_id != "" {
		request_logger := logger.With().Str("job_id", request_id).Logger()
		logger = &request_logger
	}
	return douyin.NewClientWithLogger(cookie, logger).Fetch(raw_url)
}

func (h *handler) BuildBrowseHistory(content_json json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	return nil, adapter.ErrBrowseHistoryNotSupported
}

type douyin_content_json struct {
	URL string `json:"url"`
}

func (h *handler) BuildDownloadTask(content_json json.RawMessage, config_raw json.RawMessage) (*adapter.DownloadTaskResult, error) {
	config := make(map[string]any)
	if len(strings.TrimSpace(string(config_raw))) > 0 && string(config_raw) != "null" {
		if err := json.Unmarshal(config_raw, &config); err != nil {
			return nil, fmt.Errorf("解析下载配置失败: %w", err)
		}
	}

	model_data, err := h.download_model_data(content_json)
	if err != nil {
		return nil, err
	}
	if model_data.content_type == douyin_content_type_album && len(model_data.images) == 0 {
		return nil, fmt.Errorf("抖音图集 %s 图片列表为空", model_data.video_id)
	}
	if model_data.content_type != douyin_content_type_album && strings.TrimSpace(model_data.video_url) == "" {
		return nil, fmt.Errorf("抖音视频 %s 下载地址为空", model_data.video_id)
	}

	content := douyin_content_from_data(model_data)
	account := douyin_account_from_data(model_data)
	content_detail := douyin_content_detail_from_data(model_data)

	title, _ := config["filename"].(string)
	title = strings.TrimSpace(title)
	if title == "" {
		title = model_data.title
	}

	config_json, err := json.Marshal(build_config_json(config))
	if err != nil {
		return nil, fmt.Errorf("解析下载配置失败: %w", err)
	}
	metadata_json, err := json.Marshal(map[string]any{
		"platform":     PlatformID,
		"external_id":  model_data.video_id,
		"title":        model_data.title,
		"content_type": model_data.content_type,
		"image_count":  len(model_data.images),
		"source":       model_data.source,
		"download_at":  time.Now().Unix(),
	})
	if err != nil {
		return nil, fmt.Errorf("构建抖音下载元数据失败: %w", err)
	}

	extra_json := build_extra_json(model_data.video_id, model_data.title)
	content_id := content.Id

	var resources []*adapter.ResourceInfo
	download_cover, _ := config["download_cover"].(bool)
	if download_cover && model_data.cover_url != "" {
		cover_resource := model.DownloadResource{
			ContentId: &content_id,
			Name:      title,
			Kind:      "image",
			UniqueID:  model_data.video_id + "_cover",
			Extra:     extra_json,
		}
		cover_endpoint := model.DownloadEndpoint{
			Protocol: douyin_endpoint_protocol(model_data.cover_url),
			URL:      model_data.cover_url,
			Enabled:  1,
		}
		resources = append(resources, &adapter.ResourceInfo{
			Resource:  cover_resource,
			Endpoints: []model.DownloadEndpoint{cover_endpoint},
		})
	}

	if model_data.content_type == douyin_content_type_album {
		resources = append(resources, douyin_album_resource_infos(model_data, content_id, title, extra_json)...)
	} else {
		video_resource := model.DownloadResource{
			ContentId: &content_id,
			Name:      title + ".mp4",
			Kind:      "video",
			UniqueID:  model_data.video_id,
			Extra:     extra_json,
		}
		video_endpoint := model.DownloadEndpoint{
			Protocol: douyin_endpoint_protocol(model_data.video_url),
			URL:      model_data.video_url,
			Enabled:  1,
		}
		resources = append(resources, &adapter.ResourceInfo{
			Resource:  video_resource,
			Endpoints: []model.DownloadEndpoint{video_endpoint},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind:     model.ContentAssetKindVideo,
				Role:     model.ContentAssetRoleVideoVariant,
				AssetKey: "default",
				Relation: model.DownloadResourceAssetRelationSource,
			}},
		})
	}

	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content.Id,
			Name:         title,
			UniqueID:     model_data.video_id,
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			SourceURL:    content.SourceURL,
			CoverURL:     content.CoverURL,
			CoverWidth:   content.CoverWidth,
			CoverHeight:  content.CoverHeight,
			ConfigJSON:   string(config_json),
			MetadataJSON: string(metadata_json),
		},
		Resources:     resources,
		ContentDetail: content_detail,
		Account:       account,
		Content:       content,
	}, nil
}

func (h *handler) download_model_data(content_json json.RawMessage) (*douyin_model_data, error) {
	model_data, mobile_err := douyin_model_data_from_json(content_json)
	if mobile_err == nil {
		return model_data, nil
	}

	var input douyin_content_json
	if err := json.Unmarshal(content_json, &input); err != nil {
		return nil, fmt.Errorf("解析抖音数据失败: %w", mobile_err)
	}
	input.URL = strings.TrimSpace(input.URL)
	if input.URL == "" {
		return nil, fmt.Errorf("解析抖音数据失败: %w", mobile_err)
	}

	cookie := h.config_string("douyin.cookie")
	client := douyin.NewClientWithLogger(cookie, h.get_logger())
	video_info, err := client.GetVideoInfo(input.URL)
	if err != nil {
		return nil, fmt.Errorf("获取抖音视频信息失败: %w", err)
	}
	model_data, err = douyin_model_data_from_video_info(video_info)
	if err != nil {
		return nil, err
	}
	model_data.source_url = input.URL
	return model_data, nil
}

// BuildContentID builds a content identifier from an external ID.
func BuildContentID(external_id string) string {
	return platform_id_douyin + ":" + external_id
}

// BuildAccountID builds an account identifier from an external ID.
func BuildAccountID(external_id string) string {
	return platform_id_douyin + ":" + external_id
}

// build_extra_json builds the resource.Extra JSON string.
func build_extra_json(id string, title string) string {
	data, _ := json.Marshal(map[string]string{
		"id":    id,
		"title": title,
	})
	return string(data)
}

// build_config_json returns a copy of the download configuration.
func build_config_json(config map[string]any) map[string]any {
	m := make(map[string]any, len(config))
	for key, value := range config {
		m[key] = value
	}
	return m
}

func douyin_endpoint_protocol(endpoint_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(endpoint_url))
	if err == nil && parsed_url.Scheme != "" {
		return strings.ToLower(parsed_url.Scheme)
	}
	return "https"
}

func douyin_album_resource_infos(model_data *douyin_model_data, content_id string, title string, extra_json string) []*adapter.ResourceInfo {
	resource_count := len(model_data.images)
	if model_data.bgm_url != "" {
		resource_count++
	}
	resources := make([]*adapter.ResourceInfo, 0, resource_count)
	for image_index, image := range model_data.images {
		extension := image.ext
		if extension == "" {
			extension = "webp"
		}
		image_name := title
		if len(model_data.images) > 1 {
			image_name += "_" + fmt.Sprintf("%02d", image_index+1)
		}
		image_name += "." + extension
		image_resource := model.DownloadResource{
			ContentId: &content_id,
			Name:      image_name,
			Kind:      "image",
			UniqueID:  fmt.Sprintf("%s_image_%d", model_data.video_id, image_index+1),
			Extra:     extra_json,
		}
		image_endpoint := model.DownloadEndpoint{
			Protocol: douyin_endpoint_protocol(image.url),
			URL:      image.url,
			Enabled:  1,
		}
		image_key := model.BuildContentAlbumImageKey(image.uri, image.url, image_index)
		resources = append(resources, &adapter.ResourceInfo{
			Resource:  image_resource,
			Endpoints: []model.DownloadEndpoint{image_endpoint},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind:            model.ContentAssetKindImage,
				Role:            model.ContentAssetRolePrimary,
				AssetKey:        model.BuildContentAlbumImageAssetKey(image_key, extension),
				Relation:        model.DownloadResourceAssetRelationSource,
				SubjectType:     model.ContentAssetSubjectAlbumImage,
				SubjectKey:      image_key,
				SubjectRelation: model.ContentAssetSubjectRelationRepresentation,
			}},
		})
	}
	if model_data.bgm_url != "" {
		bgm_resource := model.DownloadResource{
			ContentId: &content_id,
			Name:      title + "_bgm.mp3",
			Kind:      "audio",
			UniqueID:  model_data.video_id + "_bgm",
			Duration:  model_data.bgm_duration,
			Extra:     extra_json,
		}
		bgm_endpoint := model.DownloadEndpoint{
			Protocol: douyin_endpoint_protocol(model_data.bgm_url),
			URL:      model_data.bgm_url,
			Enabled:  1,
		}
		resources = append(resources, &adapter.ResourceInfo{
			Resource:  bgm_resource,
			Endpoints: []model.DownloadEndpoint{bgm_endpoint},
		})
	}
	return resources
}
