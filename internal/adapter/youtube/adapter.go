package youtubeadapter

import (
	"context"
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
	"wx_channel/pkg/cache"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/scraper/youtube"
	"wx_channel/pkg/util"
)

const PlatformID = youtube.PlatformID

func init() {
	adapter.Register(&handler{})
}

type handler struct {
	runtime_mu sync.RWMutex
	logger     *zerolog.Logger
	config     *config.Config
	file_cache *cache.CacheProvider
	cookies    *cookies.Reader
}

var (
	_ adapter.PlatformAdapter             = (*handler)(nil)
	_ adapter.ProgressFetchAdapter        = (*handler)(nil)
	_ adapter.ContextProgressFetchAdapter = (*handler)(nil)
	_ adapter.FetchCacheAdapter           = (*handler)(nil)
	_ adapter.FetchDownloadTaskBuilder    = (*handler)(nil)
	_ adapter.RuntimeAdapter              = (*handler)(nil)
	_ adapter.RuntimeHandle               = (*handler)(nil)
)

func (h *handler) PlatformID() string { return PlatformID }

func (h *handler) Fetch(rawURL string) (any, error) {
	return h.FetchWithProgress(rawURL, "")
}

func (h *handler) FetchWithProgress(rawURL string, requestID string) (any, error) {
	return h.FetchWithProgressContext(context.Background(), rawURL, adapter.FetchOptions{
		RequestID: strings.TrimSpace(requestID),
	})
}

func (h *handler) FetchWithProgressContext(fetchContext context.Context, rawURL string, options adapter.FetchOptions) (any, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("YouTube URL 不能为空")
	}
	if fetchContext == nil {
		fetchContext = context.Background()
	}
	return h.scraper_client(options.ForceRefresh).FetchContext(fetchContext, rawURL)
}

func (h *handler) ToContent(data any) (*model.Content, error) {
	info, err := youtube_info_from_fetch(data)
	if err != nil {
		return nil, err
	}
	now := util.NowMillis()
	content_type := model.ContentTypeVideo
	subtype := model.ContentSubtypeLongVideo
	switch info.MediaType {
	case "short":
		subtype = model.ContentSubtypeShortVideo
	case "livestream":
		content_type = model.ContentTypeLive
		subtype = model.ContentSubtypeLivestream
	}
	metadata, _ := json.Marshal(map[string]any{
		"channel_id":          info.ChannelID,
		"channel_url":         info.ChannelURL,
		"channel_avatar_url":  info.ChannelAvatarURL,
		"uploader":            info.Uploader,
		"uploader_url":        info.UploaderURL,
		"uploader_avatar_url": info.UploaderAvatarURL,
		"live_status":         info.LiveStatus,
		"age_limit":           info.AgeLimit,
		"format_count":        len(info.Formats),
		"warnings":            info.Warnings,
	})
	return &model.Content{
		Id:          BuildContentID(info.ID),
		PlatformId:  PlatformID,
		Type:        content_type,
		Subtype:     subtype,
		ExternalId:  info.ID,
		Title:       first_non_empty(info.Title, "youtube_"+info.ID),
		Description: info.Description,
		URL:         best_content_url(info),
		SourceURL:   first_non_empty(info.WebpageURL, canonical_watch_url(info.ID)),
		CoverURL:    info.Thumbnail,
		ViewCount:   info.ViewCount,
		Tags:        json_string_array(info.Tags),
		Category:    strings.Join(info.Categories, ","),
		Metadata:    string(metadata),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

func (h *handler) ToAccount(data any) (*model.Account, error) {
	info, err := youtube_info_from_fetch(data)
	if err != nil {
		return nil, err
	}
	external_id := strings.TrimSpace(first_non_empty(info.ChannelID, info.UploaderID, info.ID))
	nickname := strings.TrimSpace(first_non_empty(info.Channel, info.Uploader, "YouTube"))
	now := util.NowMillis()
	return &model.Account{
		Id:         BuildAccountID(external_id),
		PlatformId: PlatformID,
		ExternalId: external_id,
		Nickname:   nickname,
		AvatarURL:  first_non_empty(info.ChannelAvatarURL, info.UploaderAvatarURL),
		ProfileURL: first_non_empty(info.ChannelURL, info.UploaderURL),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

func (h *handler) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	info, err := youtube_info_from_fetch(data)
	if err != nil {
		return nil, err
	}
	content_id := BuildContentID(info.ID)
	return []adapter.ContentDetail{{
		Type: "video",
		Key:  content_id,
		Data: youtube_content_video(info),
	}}, nil
}

func (h *handler) BuildBrowseHistory(_ json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	return nil, adapter.ErrBrowseHistoryNotSupported
}

type youtube_content_json struct {
	URL string `json:"url"`
}

func (h *handler) BuildDownloadTask(contentJSON json.RawMessage, configJSON json.RawMessage) (*adapter.DownloadTaskResult, error) {
	config, err := parse_config_json(configJSON)
	if err != nil {
		return nil, err
	}
	info, err := h.video_info_from_content_json(contentJSON)
	if err != nil {
		return nil, err
	}
	return h.build_download_task(info, config)
}

func (h *handler) BuildDownloadTaskFromFetch(data any, configJSON json.RawMessage) (*adapter.DownloadTaskResult, error) {
	config, err := parse_config_json(configJSON)
	if err != nil {
		return nil, err
	}
	info, err := youtube_info_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return h.build_download_task(info, config)
}

func (h *handler) FetchCacheEntries(rawURL string, data any) ([]adapter.FetchCacheEntry, error) {
	info, err := youtube_info_from_fetch(data)
	if err != nil {
		return nil, err
	}
	source_url := strings.TrimSpace(rawURL)
	if source_url == "" {
		source_url = first_non_empty(info.WebpageURL, canonical_watch_url(info.ID))
	}
	html_cache, err := youtube.LookupHTMLCache(h.runtime_file_cache(), source_url)
	if err != nil {
		return nil, err
	}
	if html_cache == nil {
		return nil, nil
	}
	return []adapter.FetchCacheEntry{{
		Key:  "watch",
		Name: "视频页面",
		URL:  first_non_empty(info.WebpageURL, canonical_watch_url(info.ID)),
		Path: html_cache.Path,
		Size: html_cache.Size,
	}}, nil
}

func (h *handler) ClearFetchCache(rawURL string) (bool, error) {
	return youtube.ClearHTMLCache(h.runtime_file_cache(), strings.TrimSpace(rawURL))
}

func (h *handler) RegisterRuntime(adapterOptions *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if h == nil {
		return nil, fmt.Errorf("youtube adapter is nil")
	}
	if adapterOptions == nil {
		return nil, fmt.Errorf("youtube runtime dependencies are nil")
	}
	h.runtime_mu.Lock()
	h.logger = adapterOptions.Logger
	h.config = adapterOptions.Config
	h.file_cache = adapterOptions.Cache
	h.cookies = adapterOptions.Cookies
	h.runtime_mu.Unlock()
	if adapterOptions.Logger != nil {
		adapterOptions.Logger.Info().
			Str("component", "youtube_adapter").
			Bool("html_cache_enabled", adapterOptions.Cache != nil && adapterOptions.Cache.Enabled()).
			Bool("cookie_reader_available", adapterOptions.Cookies != nil).
			Msg("youtube adapter runtime registered")
	}
	if adapterOptions.Bus != nil {
		adapterOptions.Bus.Publish(events.PlatformStatusChanged{
			Platform:  PlatformID,
			Key:       PlatformID,
			Name:      "YouTube",
			Status:    "available",
			Available: true,
		})
	}
	return h, nil
}

func (h *handler) Stop() {
	if h == nil {
		return
	}
	h.runtime_mu.Lock()
	h.logger = nil
	h.config = nil
	h.file_cache = nil
	h.cookies = nil
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

func (h *handler) runtime_file_cache() *cache.CacheProvider {
	if h == nil {
		return nil
	}
	h.runtime_mu.RLock()
	file_cache := h.file_cache
	h.runtime_mu.RUnlock()
	return file_cache
}

func (h *handler) runtime_cookie_reader() *cookies.Reader {
	if h == nil {
		return nil
	}
	h.runtime_mu.RLock()
	cookie_reader := h.cookies
	h.runtime_mu.RUnlock()
	return cookie_reader
}

func (h *handler) scraper_client(forceRefresh bool) *youtube.Client {
	return youtube.NewClientWithOptions(youtube.ClientOptions{
		Cookie:       h.config_string("youtube.cookie"),
		PoToken:      h.config_string("youtube.poToken"),
		CookieReader: h.runtime_cookie_reader(),
		Cache:        h.runtime_file_cache(),
		ForceRefresh: forceRefresh,
	})
}

func (h *handler) video_info_from_content_json(contentJSON json.RawMessage) (*youtube.VideoInfo, error) {
	var info youtube.VideoInfo
	if len(contentJSON) > 0 && json.Unmarshal(contentJSON, &info) == nil && strings.TrimSpace(info.ID) != "" {
		return &info, nil
	}

	var input youtube_content_json
	if err := json.Unmarshal(contentJSON, &input); err != nil {
		return nil, fmt.Errorf("解析 YouTube 数据失败: %w", err)
	}
	input.URL = strings.TrimSpace(input.URL)
	if input.URL == "" {
		return nil, fmt.Errorf("YouTube URL 不能为空")
	}
	return h.scraper_client(false).FetchContext(context.Background(), input.URL)
}

func (h *handler) build_download_task(info *youtube.VideoInfo, config map[string]any) (*adapter.DownloadTaskResult, error) {
	if info == nil || strings.TrimSpace(info.ID) == "" {
		return nil, fmt.Errorf("YouTube 视频信息为空")
	}
	download_formats, err := select_download_formats(info)
	if err != nil {
		return nil, err
	}

	content, err := h.ToContent(info)
	if err != nil {
		return nil, err
	}
	account, err := h.ToAccount(info)
	if err != nil {
		return nil, err
	}
	content_detail := youtube_content_video(info)
	task_name := strings.TrimSpace(config_string(config, "filename"))
	if task_name == "" {
		task_name = content.Title
	}

	config_data, _ := json.Marshal(config)
	metadata_data, _ := json.Marshal(map[string]any{
		"platform":       PlatformID,
		"external_id":    info.ID,
		"title":          info.Title,
		"channel_id":     info.ChannelID,
		"channel":        info.Channel,
		"source_url":     content.SourceURL,
		"download_at":    time.Now().Unix(),
		"format_ids":     format_ids(download_formats),
		"format_count":   len(info.Formats),
		"requires_merge": len(download_formats) > 1,
	})

	extra_json := build_extra_json(info, download_formats)
	content_id := content.Id
	resources := make([]*adapter.ResourceInfo, 0, len(download_formats)+1)
	if config_bool(config, "download_cover") && info.Thumbnail != "" {
		resources = append(resources, &adapter.ResourceInfo{
			Resource: model.DownloadResource{
				ContentId:  &content_id,
				Name:       "cover",
				Kind:       image_kind_from_url(info.Thumbnail),
				UniqueID:   info.ID + "_cover",
				MergeOrder: 999,
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: endpoint_protocol(info.Thumbnail),
				URL:      info.Thumbnail,
				Enabled:  1,
			}},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind:     model.ContentAssetKindImage,
				Role:     model.ContentAssetRoleCover,
				AssetKey: "cover",
				Relation: model.DownloadResourceAssetRelationSource,
			}},
		})
	}

	headers := h.scraper_client(false).DownloadHeaders(content.SourceURL)
	headers_json := headers_json_string(headers)
	for index, format := range download_formats {
		resource_kind := youtube_resource_kind(format)
		resource_name := task_name
		asset_kind := model.ContentAssetKindVideo
		asset_role := model.ContentAssetRoleVideoVariant
		asset_key := youtube_video_variant_key(format)
		if format.HasAudio && !format.HasVideo {
			resource_name = task_name + "_audio"
			asset_kind = model.ContentAssetKindAudio
			asset_role = model.ContentAssetRoleAudioVariant
			asset_key = youtube_audio_asset_key(format)
		}
		resources = append(resources, &adapter.ResourceInfo{
			Resource: model.DownloadResource{
				ContentId:  &content_id,
				Name:       resource_name,
				Kind:       resource_kind,
				UniqueID:   info.ID + "_" + format.ID,
				Size:       format.ContentLength,
				MergeOrder: index,
				Extra:      extra_json,
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: endpoint_protocol(format.URL),
				URL:      format.URL,
				Headers:  headers_json,
				Enabled:  1,
			}},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind:     asset_kind,
				Role:     asset_role,
				AssetKey: asset_key,
				Relation: model.DownloadResourceAssetRelationSource,
			}},
		})
	}

	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content.Id,
			Name:         task_name,
			UniqueID:     info.ID,
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			SourceURL:    content.SourceURL,
			CoverURL:     content.CoverURL,
			ConfigJSON:   string(config_data),
			MetadataJSON: string(metadata_data),
		},
		Resources:     resources,
		ContentDetail: content_detail,
		Account:       account,
		Content:       content,
	}, nil
}

func youtube_info_from_fetch(data any) (*youtube.VideoInfo, error) {
	switch value := data.(type) {
	case *youtube.VideoInfo:
		if value == nil {
			return nil, fmt.Errorf("youtube video info is nil")
		}
		return value, nil
	case youtube.VideoInfo:
		return &value, nil
	case json.RawMessage:
		var info youtube.VideoInfo
		if err := json.Unmarshal(value, &info); err != nil {
			return nil, fmt.Errorf("decode youtube video info: %w", err)
		}
		if strings.TrimSpace(info.ID) == "" {
			return nil, fmt.Errorf("youtube video id is empty")
		}
		return &info, nil
	case []byte:
		return youtube_info_from_fetch(json.RawMessage(value))
	default:
		return nil, fmt.Errorf("unsupported youtube fetch data type %T", data)
	}
}

func youtube_content_video(info *youtube.VideoInfo) *model.ContentVideo {
	if info == nil {
		return nil
	}
	selected := best_video_detail_format(info)
	video := &model.ContentVideo{
		Id:        BuildContentID(info.ID),
		Duration:  info.Duration,
		PlayTimes: info.ViewCount,
	}
	if selected != nil {
		video.Width = selected.Width
		video.Height = selected.Height
		video.FPS = selected.FPS
		video.Bitrate = first_non_zero_int(selected.AverageBitrate, selected.Bitrate)
		video.Size = selected.ContentLength
		video.Codec = first_non_empty(selected.VideoCodec, selected.AudioCodec)
		video.Format = selected.Ext
		video.URL = selected.URL
	}
	video.Variants = youtube_video_variants(info, selected)
	return video
}

func youtube_video_variants(info *youtube.VideoInfo, selected *youtube.VideoFormat) []model.ContentVideoVariant {
	if info == nil {
		return nil
	}
	variants := make([]model.ContentVideoVariant, 0, len(info.Formats))
	default_key := ""
	if selected != nil {
		default_key = youtube_video_variant_key(*selected)
	}
	for _, format := range info.Formats {
		if !format.HasVideo || strings.TrimSpace(format.URL) == "" || format.NeedsSignature {
			continue
		}
		variant_key := youtube_video_variant_key(format)
		metadata, _ := json.Marshal(map[string]any{
			"itag":            format.Itag,
			"mime_type":       format.MimeType,
			"quality":         format.Quality,
			"audio_quality":   format.AudioQuality,
			"audio_codec":     format.AudioCodec,
			"video_codec":     format.VideoCodec,
			"adaptive":        format.Adaptive,
			"content_length":  format.ContentLength,
			"average_bitrate": format.AverageBitrate,
		})
		stream_type := model.ContentVideoVariantStreamTypeProgressive
		if !format.HasAudio {
			stream_type = model.ContentVideoVariantStreamTypeVideoOnly
		}
		variant := model.ContentVideoVariant{
			VideoId:    BuildContentID(info.ID),
			VariantKey: variant_key,
			Spec:       format.ID,
			Quality:    first_non_empty(format.QualityLabel, format.Quality),
			Size:       format.ContentLength,
			Codec:      first_non_empty(format.VideoCodec, format.AudioCodec),
			Format:     format.Ext,
			StreamType: stream_type,
			HasVideo:   bool_int(format.HasVideo),
			HasAudio:   bool_int(format.HasAudio),
			IsDefault:  bool_int(default_key == variant_key),
			URL:        format.URL,
			Metadata:   string(metadata),
		}
		variant.Width = positive_int_pointer(format.Width)
		variant.Height = positive_int_pointer(format.Height)
		variant.FPS = positive_int_pointer(format.FPS)
		bitrate := first_non_zero_int(format.AverageBitrate, format.Bitrate)
		variant.Bitrate = positive_int_pointer(bitrate)
		variants = append(variants, variant)
	}
	return variants
}

func select_download_formats(info *youtube.VideoInfo) ([]youtube.VideoFormat, error) {
	if info == nil {
		return nil, fmt.Errorf("YouTube 视频信息为空")
	}
	var progressive []youtube.VideoFormat
	var video_only []youtube.VideoFormat
	var audio_only []youtube.VideoFormat
	for _, format := range info.Formats {
		if strings.TrimSpace(format.URL) == "" || format.NeedsSignature {
			continue
		}
		switch {
		case format.HasVideo && format.HasAudio:
			progressive = append(progressive, format)
		case format.HasVideo:
			video_only = append(video_only, format)
		case format.HasAudio:
			audio_only = append(audio_only, format)
		}
	}
	if len(progressive) > 0 {
		return []youtube.VideoFormat{best_progressive_format(progressive)}, nil
	}
	if len(video_only) > 0 && len(audio_only) > 0 {
		return []youtube.VideoFormat{best_video_format(video_only), best_audio_format(audio_only)}, nil
	}
	return nil, fmt.Errorf("YouTube 视频 %s 未返回可直接下载的格式", info.ID)
}

func best_video_detail_format(info *youtube.VideoInfo) *youtube.VideoFormat {
	formats, err := select_download_formats(info)
	if err != nil || len(formats) == 0 {
		return nil
	}
	return &formats[0]
}

func best_content_url(info *youtube.VideoInfo) string {
	if format := best_video_detail_format(info); format != nil {
		return format.URL
	}
	return first_non_empty(info.WebpageURL, canonical_watch_url(info.ID))
}

func best_progressive_format(formats []youtube.VideoFormat) youtube.VideoFormat {
	best := formats[0]
	for _, format := range formats[1:] {
		if video_score(format) > video_score(best) {
			best = format
		}
	}
	return best
}

func best_video_format(formats []youtube.VideoFormat) youtube.VideoFormat {
	best := formats[0]
	for _, format := range formats[1:] {
		if video_score(format) > video_score(best) {
			best = format
		}
	}
	return best
}

func best_audio_format(formats []youtube.VideoFormat) youtube.VideoFormat {
	best := formats[0]
	for _, format := range formats[1:] {
		if audio_score(format) > audio_score(best) {
			best = format
		}
	}
	return best
}

func video_score(format youtube.VideoFormat) int {
	return format.Height*1_000_000 + format.FPS*10_000 + first_non_zero_int(format.AverageBitrate, format.Bitrate)
}

func audio_score(format youtube.VideoFormat) int {
	return first_non_zero_int(format.AverageBitrate, format.Bitrate) + int(format.ContentLength/1024)
}

func youtube_video_variant_key(format youtube.VideoFormat) string {
	if strings.TrimSpace(format.ID) != "" {
		return "format_" + strings.TrimSpace(format.ID)
	}
	if format.Itag > 0 {
		return fmt.Sprintf("format_%d", format.Itag)
	}
	return "default"
}

func youtube_audio_asset_key(format youtube.VideoFormat) string {
	if strings.TrimSpace(format.ID) != "" {
		return "audio_" + strings.TrimSpace(format.ID)
	}
	if format.Itag > 0 {
		return fmt.Sprintf("audio_%d", format.Itag)
	}
	return "audio"
}

func youtube_resource_kind(format youtube.VideoFormat) string {
	if strings.TrimSpace(format.MimeType) != "" {
		if before, _, ok := strings.Cut(format.MimeType, ";"); ok {
			return strings.TrimSpace(before)
		}
		return strings.TrimSpace(format.MimeType)
	}
	if format.HasAudio && !format.HasVideo {
		return "audio/" + first_non_empty(format.Ext, "mp4")
	}
	return "video/" + first_non_empty(format.Ext, "mp4")
}

func image_kind_from_url(raw_url string) string {
	parsed, err := url.Parse(raw_url)
	if err == nil {
		path := strings.ToLower(parsed.Path)
		switch {
		case strings.HasSuffix(path, ".png"):
			return "image/png"
		case strings.HasSuffix(path, ".webp"):
			return "image/webp"
		case strings.HasSuffix(path, ".jpeg"), strings.HasSuffix(path, ".jpg"):
			return "image/jpeg"
		}
	}
	return "image"
}

func endpoint_protocol(endpoint_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(endpoint_url))
	if err == nil && parsed_url.Scheme != "" {
		return strings.ToLower(parsed_url.Scheme)
	}
	return "https"
}

func headers_json_string(headers map[string]string) string {
	if len(headers) == 0 {
		return ""
	}
	data, _ := json.Marshal(headers)
	return string(data)
}

func build_extra_json(info *youtube.VideoInfo, formats []youtube.VideoFormat) string {
	data, _ := json.Marshal(map[string]any{
		"id":         info.ID,
		"title":      info.Title,
		"source_url": first_non_empty(info.WebpageURL, canonical_watch_url(info.ID)),
		"format_ids": format_ids(formats),
	})
	return string(data)
}

func format_ids(formats []youtube.VideoFormat) []string {
	ids := make([]string, 0, len(formats))
	for _, format := range formats {
		ids = append(ids, format.ID)
	}
	return ids
}

func parse_config_json(configJSON json.RawMessage) (map[string]any, error) {
	config := make(map[string]any)
	config_text := strings.TrimSpace(string(configJSON))
	if config_text == "" || config_text == "null" {
		return config, nil
	}
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil, fmt.Errorf("解析下载配置失败: %w", err)
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

func config_bool(config map[string]any, key string) bool {
	value, _ := config[key].(bool)
	return value
}

func json_string_array(values []string) string {
	if len(values) == 0 {
		return ""
	}
	data, _ := json.Marshal(values)
	return string(data)
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func first_non_zero_int(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func bool_int(value bool) int {
	if value {
		return 1
	}
	return 0
}

func positive_int_pointer(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func BuildContentID(externalID string) string {
	return PlatformID + ":" + strings.TrimSpace(externalID)
}

func BuildAccountID(externalID string) string {
	return PlatformID + ":" + strings.TrimSpace(externalID)
}

func canonical_watch_url(videoID string) string {
	if strings.TrimSpace(videoID) == "" {
		return "https://www.youtube.com"
	}
	return "https://www.youtube.com/watch?v=" + url.QueryEscape(strings.TrimSpace(videoID))
}
