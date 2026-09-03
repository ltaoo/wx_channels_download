package xadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	x_scraper "wx_channel/pkg/scraper/x"
	"wx_channel/pkg/util"
)

// PlatformID is the platform identifier for X/Twitter posts.
const PlatformID = x_scraper.PlatformID

type handler struct{}

var (
	_ adapter.PlatformAdapter             = (*handler)(nil)
	_ adapter.ContextProgressFetchAdapter = (*handler)(nil)
	_ adapter.FetchDownloadTaskBuilder    = (*handler)(nil)
	_ adapter.RuntimeAdapter              = (*handler)(nil)
	_ adapter.RuntimeHandle               = (*handler)(nil)
	_ adapter.PlatformStatusDescriber     = (*handler)(nil)
)

func init() {
	adapter.Register(&handler{})
}

func (h *handler) PlatformID() string { return PlatformID }

func (h *handler) PlatformStatuses() []adapter.PlatformStatusDescriptor {
	return []adapter.PlatformStatusDescriptor{{Platform: PlatformID, Key: PlatformID, Name: "X (Twitter)"}}
}

func (h *handler) RegisterRuntime(adapter_options *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if adapter_options == nil {
		return nil, fmt.Errorf("x runtime dependencies are nil")
	}
	if adapter_options.Bus != nil {
		adapter_options.Bus.Publish(events.PlatformStatusChanged{
			Platform:  PlatformID,
			Key:       PlatformID,
			Name:      "X (Twitter)",
			Status:    "available",
			Available: true,
		})
	}
	return h, nil
}

func (h *handler) Stop() {}

func (h *handler) Fetch(raw_url string) (any, error) {
	return h.FetchWithProgressContext(context.Background(), raw_url, adapter.FetchOptions{})
}

func (h *handler) FetchWithProgressContext(fetch_context context.Context, raw_url string, _ adapter.FetchOptions) (any, error) {
	client, err := x_scraper.NewClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()
	return client.FetchContext(fetch_context, raw_url)
}

func (h *handler) ToContent(data any) (*model.Content, error) {
	result, err := result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	metadata, _ := json.Marshal(map[string]any{
		"author_id":       result.AuthorID,
		"author_username": result.AuthorUsername,
		"videos":          result.Videos,
	})
	now := util.NowMillis()
	return &model.Content{
		Id:           PlatformID + ":" + result.ExternalID,
		PlatformId:   PlatformID,
		Type:         model.ContentTypePost,
		Subtype:      model.ContentSubtypeMicroblog,
		ExternalId:   result.ExternalID,
		ExternalId2:  result.AuthorID,
		Title:        post_title(result.BodyText, result.AuthorName),
		Description:  result.BodyText,
		URL:          result.SourceURL,
		SourceURL:    result.SourceURL,
		CoverURL:     first_video_cover(result),
		PublishTime:  positive_int64_pointer(result.PublishTime),
		ViewCount:    result.ViewCount,
		LikeCount:    result.LikeCount,
		CommentCount: result.CommentCount,
		ShareCount:   result.ShareCount,
		Metadata:     string(metadata),
		Timestamps:   model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}, nil
}

func (h *handler) ToAccount(data any) (*model.Account, error) {
	result, err := result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	external_id := first_non_empty(result.AuthorID, result.AuthorUsername)
	now := util.NowMillis()
	return &model.Account{
		Id:         PlatformID + ":" + external_id,
		PlatformId: PlatformID,
		ExternalId: external_id,
		Alias:      result.AuthorUsername,
		Nickname:   result.AuthorName,
		AvatarURL:  result.AuthorAvatar,
		ProfileURL: "https://x.com/" + result.AuthorUsername,
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}, nil
}

func (h *handler) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	result, err := result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	video := content_video(result)
	if video == nil {
		return nil, nil
	}
	return []adapter.ContentDetail{{Type: model.ContentTypeVideo, Key: video.Id, Data: video}}, nil
}

func (h *handler) BuildDownloadTask(content_json json.RawMessage, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	if result, err := result_from_json(content_json); err == nil {
		return h.build_download_task(result, config_json)
	}
	var input struct {
		URL       string `json:"url"`
		SourceURL string `json:"source_url"`
	}
	if err := json.Unmarshal(content_json, &input); err != nil {
		return nil, fmt.Errorf("decode x download data: %w", err)
	}
	raw_url := first_non_empty(input.URL, input.SourceURL)
	if raw_url == "" {
		return nil, fmt.Errorf("x download data is missing URL")
	}
	fetched, err := h.Fetch(raw_url)
	if err != nil {
		return nil, err
	}
	return h.BuildDownloadTaskFromFetch(fetched, config_json)
}

func (h *handler) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	result, err := result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return h.build_download_task(result, config_json)
}

func (h *handler) build_download_task(result *x_scraper.FetchResult, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	content, err := h.ToContent(result)
	if err != nil {
		return nil, err
	}
	account, err := h.ToAccount(result)
	if err != nil {
		return nil, err
	}
	config_text := strings.TrimSpace(string(config_json))
	if config_text == "" || config_text == "null" {
		config_text = "{}"
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(config_text), &config); err != nil {
		return nil, fmt.Errorf("decode x download config: %w", err)
	}
	task_name, _ := config["filename"].(string)
	task_name = first_non_empty(task_name, content.Title)
	content_id := content.Id
	resources := make([]*adapter.ResourceInfo, 0, 2)
	if result.BodyText != "" {
		resources = append(resources, &adapter.ResourceInfo{
			Resource:  model.DownloadResource{ContentId: &content_id, Name: task_name, Kind: "text/plain", UniqueID: result.ExternalID + "_body", Size: int64(len(result.BodyText))},
			Endpoints: []model.DownloadEndpoint{{Protocol: "inline", URL: result.BodyText, Enabled: 1}},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind: model.ContentAssetKindText, Role: model.ContentAssetRoleArticleBody, AssetKey: "body:text", Relation: model.DownloadResourceAssetRelationSource,
			}},
		})
	}
	video := content_video(result)
	if video != nil {
		resource, err := video_resource(content_id, task_name, result.SourceURL, &result.Videos[0])
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	if len(resources) == 0 {
		return nil, fmt.Errorf("x post %s has no downloadable content", result.ExternalID)
	}
	now := util.NowMillis()
	details, _ := h.ToContentDetails(result)
	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId: &content_id, Name: task_name, UniqueID: result.ExternalID, PlatformId: PlatformID,
			Status: model.TaskStatusWaiting, SourceURL: content.SourceURL, CoverURL: content.CoverURL,
			ConfigJSON: config_text, MetadataJSON: content.Metadata,
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		},
		Resources: resources, ContentDetail: video, ContentDetails: details, Account: account, Content: content,
	}, nil
}

func (h *handler) BuildBrowseHistory(_ json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	return nil, adapter.ErrBrowseHistoryNotSupported
}

func content_video(result *x_scraper.FetchResult) *model.ContentVideo {
	if result == nil || len(result.Videos) == 0 {
		return nil
	}
	video := result.Videos[0]
	content_id := PlatformID + ":" + result.ExternalID
	now := util.NowMillis()
	variants := make([]model.ContentVideoVariant, 0, len(video.Variants))
	selected_bitrate := 0
	for variant_index, source_variant := range video.Variants {
		is_hls := strings.Contains(strings.ToLower(source_variant.ContentType), "mpegurl") || strings.Contains(source_variant.URL, ".m3u8")
		stream_type := model.ContentVideoVariantStreamTypeProgressive
		format := "mp4"
		variant_key := fmt.Sprintf("mp4:%d:%d", source_variant.Bitrate, variant_index)
		if is_hls {
			stream_type = model.ContentVideoVariantStreamTypeManifest
			format = "m3u8"
			variant_key = "hls"
		}
		is_default := 0
		if source_variant.URL == video.URL {
			is_default = 1
			selected_bitrate = source_variant.Bitrate
		}
		variants = append(variants, model.ContentVideoVariant{
			VideoId: content_id, VariantKey: variant_key, Width: positive_int_pointer(video.Width), Height: positive_int_pointer(video.Height),
			Bitrate: positive_int_pointer(source_variant.Bitrate), Format: format, StreamType: stream_type,
			HasVideo: 1, HasAudio: 1, IsDefault: is_default, URL: source_variant.URL,
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		})
	}
	return &model.ContentVideo{
		Id: content_id, Duration: (video.DurationMillis + 999) / 1000, Width: video.Width, Height: video.Height,
		Bitrate: selected_bitrate, Format: "mp4", URL: video.URL, Variants: variants,
	}
}

func video_resource(content_id string, task_name string, source_url string, video *x_scraper.Video) (*adapter.ResourceInfo, error) {
	if video == nil || strings.TrimSpace(video.URL) == "" {
		return nil, fmt.Errorf("x post video has no download URL")
	}
	headers, _ := json.Marshal(map[string]string{"Accept": "*/*", "Referer": source_url, "User-Agent": x_scraper.DefaultUserAgent})
	is_hls := strings.Contains(strings.ToLower(video.URL), ".m3u8")
	resource := model.DownloadResource{
		ContentId: &content_id, Name: task_name, Kind: "video/mp4", UniqueID: video.ID, Duration: (video.DurationMillis + 999) / 1000,
	}
	protocol := "https"
	asset_key := selected_variant_key(video)
	if is_hls {
		resource.Kind = "video/x-matroska"
		resource.Type = model.ResourceTypeStream
		resource.StreamURL = video.URL
		protocol = "livestream"
		asset_key = "hls"
	}
	return &adapter.ResourceInfo{
		Resource:  resource,
		Endpoints: []model.DownloadEndpoint{{Protocol: protocol, URL: video.URL, Enabled: 1, Headers: string(headers)}},
		ContentAssets: []adapter.ContentAssetReference{{
			Kind: model.ContentAssetKindVideo, Role: model.ContentAssetRoleVideoVariant, AssetKey: asset_key, Relation: model.DownloadResourceAssetRelationSource,
		}},
	}, nil
}

func selected_variant_key(video *x_scraper.Video) string {
	for variant_index, variant := range video.Variants {
		if variant.URL == video.URL {
			return fmt.Sprintf("mp4:%d:%d", variant.Bitrate, variant_index)
		}
	}
	return "default"
}

func result_from_fetch(data any) (*x_scraper.FetchResult, error) {
	switch result := data.(type) {
	case *x_scraper.FetchResult:
		return validate_result(result)
	case x_scraper.FetchResult:
		return validate_result(&result)
	case json.RawMessage:
		return result_from_json(result)
	}
	return nil, fmt.Errorf("unsupported x fetch data type %T", data)
}

func result_from_json(data json.RawMessage) (*x_scraper.FetchResult, error) {
	var result x_scraper.FetchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode x fetch data: %w", err)
	}
	return validate_result(&result)
}

func validate_result(result *x_scraper.FetchResult) (*x_scraper.FetchResult, error) {
	if result == nil || strings.TrimSpace(result.ExternalID) == "" {
		return nil, fmt.Errorf("x fetch result has no post ID")
	}
	if strings.TrimSpace(result.BodyText) == "" && len(result.Videos) == 0 {
		return nil, fmt.Errorf("x post %s has no text or video", result.ExternalID)
	}
	return result, nil
}

func post_title(body_text string, author_name string) string {
	title_runes := []rune(strings.TrimSpace(body_text))
	if len(title_runes) > 80 {
		title_runes = append(title_runes[:80], '…')
	}
	if len(title_runes) > 0 {
		return string(title_runes)
	}
	return first_non_empty(author_name, "X post")
}

func first_video_cover(result *x_scraper.FetchResult) string {
	if result == nil || len(result.Videos) == 0 {
		return ""
	}
	return result.Videos[0].CoverURL
}

func positive_int_pointer(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func positive_int64_pointer(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
