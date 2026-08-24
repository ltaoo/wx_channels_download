package xiaohongshuadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/scraper/xiaohongshu"
	"wx_channel/pkg/util"
)

func init() {
	adapter.Register(NewXiaohongshuAdapter())
}

// XiaohongshuAdapter connects the Xiaohongshu HTML scraper to the shared
// content, account, and download-task models.
type XiaohongshuAdapter struct{}

var (
	_ adapter.PlatformAdapter             = (*XiaohongshuAdapter)(nil)
	_ adapter.ContextProgressFetchAdapter = (*XiaohongshuAdapter)(nil)
	_ adapter.FetchDownloadTaskBuilder    = (*XiaohongshuAdapter)(nil)
	_ adapter.RuntimeAdapter              = (*XiaohongshuAdapter)(nil)
	_ adapter.RuntimeHandle               = (*XiaohongshuAdapter)(nil)
	_ adapter.PlatformStatusDescriber     = (*XiaohongshuAdapter)(nil)
)

// NewXiaohongshuAdapter creates a stateless Xiaohongshu adapter.
func NewXiaohongshuAdapter() *XiaohongshuAdapter {
	return &XiaohongshuAdapter{}
}

func (a *XiaohongshuAdapter) PlatformID() string { return PlatformID }

// PlatformStatuses describes the always-available HTML scraper.
func (a *XiaohongshuAdapter) PlatformStatuses() []adapter.PlatformStatusDescriptor {
	return []adapter.PlatformStatusDescriptor{{
		Platform: PlatformID,
		Key:      PlatformID,
		Name:     "小红书",
	}}
}

// RegisterRuntime publishes the stateless scraper's availability.
func (a *XiaohongshuAdapter) RegisterRuntime(adapter_options *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if a == nil {
		return nil, fmt.Errorf("xiaohongshu adapter is nil")
	}
	if adapter_options == nil {
		return nil, fmt.Errorf("xiaohongshu runtime dependencies are nil")
	}
	if adapter_options.Bus != nil {
		adapter_options.Bus.Publish(events.PlatformStatusChanged{
			Platform:  PlatformID,
			Key:       PlatformID,
			Name:      "小红书",
			Status:    "available",
			Available: true,
		})
	}
	return a, nil
}

// Stop releases the stateless adapter runtime.
func (a *XiaohongshuAdapter) Stop() {}

// Fetch retrieves and parses a Xiaohongshu video note page.
func (a *XiaohongshuAdapter) Fetch(raw_url string) (any, error) {
	return a.FetchWithProgressContext(context.Background(), raw_url, adapter.FetchOptions{})
}

// FetchWithProgressContext retrieves a note with cancellation support.
func (a *XiaohongshuAdapter) FetchWithProgressContext(fetch_context context.Context, raw_url string, _ adapter.FetchOptions) (any, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return nil, fmt.Errorf("小红书 URL 不能为空")
	}
	client := xiaohongshu.NewClient()
	defer client.Close()
	html_text, err := client.FetchContext(fetch_context, raw_url)
	if err != nil {
		return nil, err
	}
	return parse_fetch_result(raw_url, html_text)
}

// ToContent converts a parsed Xiaohongshu video note to shared content.
func (a *XiaohongshuAdapter) ToContent(data any) (*model.Content, error) {
	result, err := fetch_result_from_data(data)
	if err != nil {
		return nil, err
	}
	return to_content(result)
}

// ToAccount converts the note publisher to a shared account.
func (a *XiaohongshuAdapter) ToAccount(data any) (*model.Account, error) {
	result, err := fetch_result_from_data(data)
	if err != nil {
		return nil, err
	}
	return to_account(result)
}

// ToContentDetails converts every available stream to video variants.
func (a *XiaohongshuAdapter) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	result, err := fetch_result_from_data(data)
	if err != nil {
		return nil, err
	}
	return to_content_details(result)
}

// BuildDownloadTask accepts either a parsed fetch result or a URL-only object.
func (a *XiaohongshuAdapter) BuildDownloadTask(content_json json.RawMessage, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	result, result_err := fetch_result_from_json(content_json)
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

// BuildDownloadTaskFromFetch builds a task without fetching the page again.
func (a *XiaohongshuAdapter) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	result, err := fetch_result_from_data(data)
	if err != nil {
		return nil, err
	}
	return build_download_task(result, config_json)
}

// BuildBrowseHistory converts a parsed note into a browse-history record.
func (a *XiaohongshuAdapter) BuildBrowseHistory(content_json json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	result, err := fetch_result_from_json(content_json)
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
			CoverWidth:   content.CoverWidth,
			CoverHeight:  content.CoverHeight,
			PublishTime:  content.PublishTime,
			ExtraData:    content.Metadata,
			Timestamps:   content.Timestamps,
		},
		Account: account,
	}, nil
}

func build_download_task(result *fetch_result, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	result, err := validate_fetch_result(result)
	if err != nil {
		return nil, err
	}
	config, err := parse_download_config(config_json)
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
	video, err := to_content_video(result)
	if err != nil {
		return nil, err
	}
	selected_variant := select_video_variant(video.Variants, config)
	if selected_variant == nil || strings.TrimSpace(selected_variant.URL) == "" {
		return nil, fmt.Errorf("小红书视频 %s 没有可下载的视频流", content.ExternalId)
	}
	selected_stream := stream_for_url(result.Note, selected_variant.URL)
	task_name := strings.TrimSpace(config_string(config, "filename"))
	if task_name == "" {
		task_name = content.Title
	}
	normalized_config_json, _ := json.Marshal(config)
	metadata_json, _ := json.Marshal(map[string]any{
		"platform":           PlatformID,
		"external_id":        content.ExternalId,
		"title":              content.Title,
		"video_variant_key":  selected_variant.VariantKey,
		"video_variant_spec": selected_variant.Spec,
		"publisher_id":       account.ExternalId,
	})
	resource_extra, _ := json.Marshal(map[string]any{
		"note_id":     content.ExternalId,
		"source_url":  content.SourceURL,
		"variant_key": selected_variant.VariantKey,
	})
	headers_json, _ := json.Marshal(map[string]string{
		"Accept":          "*/*",
		"Accept-Language": "zh-CN,zh;q=0.9",
		"Cache-Control":   "no-cache",
		"Origin":          "https://www.xiaohongshu.com",
		"Pragma":          "no-cache",
		"Referer":         "https://www.xiaohongshu.com/",
		"User-Agent":      xiaohongshu_user_agent,
	})
	endpoints := video_endpoints(selected_variant.URL, selected_stream, string(headers_json))
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("小红书视频 %s 下载地址为空", content.ExternalId)
	}
	content_id := content.Id
	now := util.NowMillis()
	resource := &adapter.ResourceInfo{
		Resource: model.DownloadResource{
			ContentId: &content_id,
			Name:      task_name,
			Kind:      "video/mp4",
			UniqueID:  content.ExternalId + "_" + strings.ReplaceAll(selected_variant.VariantKey, ":", "_"),
			Size:      selected_variant.Size,
			Duration:  video.Duration,
			Extra:     string(resource_extra),
		},
		Endpoints: endpoints,
		ContentAssets: []adapter.ContentAssetReference{{
			Kind:     model.ContentAssetKindVideo,
			Role:     model.ContentAssetRoleVideoVariant,
			AssetKey: selected_variant.VariantKey,
			Relation: model.DownloadResourceAssetRelationSource,
		}},
	}
	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content_id,
			Name:         task_name,
			UniqueID:     content.ExternalId,
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			SourceURL:    content.SourceURL,
			CoverURL:     content.CoverURL,
			CoverWidth:   content.CoverWidth,
			CoverHeight:  content.CoverHeight,
			ConfigJSON:   string(normalized_config_json),
			MetadataJSON: string(metadata_json),
			Timestamps:   model.Timestamps{CreatedAt: now, UpdatedAt: now},
		},
		Resources:     []*adapter.ResourceInfo{resource},
		ContentDetail: video,
		ContentDetails: []adapter.ContentDetail{{
			Type:    content.Type,
			Key:     content.Id,
			Content: content,
			Data:    video,
			Accounts: []adapter.ContentAccountReference{{
				Account: account,
				Role:    "owner",
			}},
		}},
		Account: account,
		Content: content,
	}, nil
}

func parse_download_config(config_json json.RawMessage) (map[string]any, error) {
	config := make(map[string]any)
	config_text := strings.TrimSpace(string(config_json))
	if config_text == "" || config_text == "null" {
		return config, nil
	}
	if err := json.Unmarshal(config_json, &config); err != nil {
		return nil, fmt.Errorf("解析小红书下载配置失败: %w", err)
	}
	if config == nil {
		config = make(map[string]any)
	}
	return config, nil
}

func config_string(config map[string]any, key string) string {
	value, _ := config[key].(string)
	return strings.TrimSpace(value)
}

func select_video_variant(variants []model.ContentVideoVariant, config map[string]any) *model.ContentVideoVariant {
	configured_key := config_string(config, "video_variant_key")
	configured_spec := config_string(config, "video_variant_spec")
	for variant_index := range variants {
		variant := &variants[variant_index]
		if configured_key != "" && variant.VariantKey == configured_key {
			return variant
		}
		if configured_key == "" && configured_spec != "" && variant.Spec == configured_spec {
			return variant
		}
	}
	for variant_index := range variants {
		if variants[variant_index].IsDefault != 0 {
			return &variants[variant_index]
		}
	}
	if len(variants) == 0 {
		return nil
	}
	return &variants[0]
}

func stream_for_url(note *note_data, stream_url string) *video_stream {
	stream_url = normalize_media_url(stream_url)
	for _, stream := range note_streams(note) {
		if stream.MasterURL == stream_url {
			stream_copy := stream
			return &stream_copy
		}
	}
	return nil
}

func video_endpoints(primary_url string, stream *video_stream, headers_json string) []model.DownloadEndpoint {
	urls := []string{normalize_media_url(primary_url)}
	if stream != nil {
		urls = append(urls, stream.BackupURLs...)
	}
	seen_urls := make(map[string]bool)
	endpoints := make([]model.DownloadEndpoint, 0, len(urls))
	for endpoint_index, endpoint_url := range urls {
		endpoint_url = normalize_media_url(endpoint_url)
		if endpoint_url == "" || seen_urls[endpoint_url] {
			continue
		}
		seen_urls[endpoint_url] = true
		parsed_url, err := url.Parse(endpoint_url)
		if err != nil || parsed_url.Hostname() == "" || (parsed_url.Scheme != "http" && parsed_url.Scheme != "https") {
			continue
		}
		endpoints = append(endpoints, model.DownloadEndpoint{
			Protocol: parsed_url.Scheme,
			URL:      endpoint_url,
			Priority: endpoint_index,
			Enabled:  1,
			Headers:  headers_json,
		})
	}
	return endpoints
}
