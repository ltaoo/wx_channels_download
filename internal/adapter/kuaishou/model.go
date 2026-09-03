package kuaishouadapter

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/kuaishou"
	"wx_channel/pkg/util"
)

type video_variant struct {
	model    model.ContentVideoVariant
	backups  []string
	duration int64
}

func fetch_result_from_data(data any) (*kuaishou.FetchResult, error) {
	var result *kuaishou.FetchResult
	switch value := data.(type) {
	case *kuaishou.FetchResult:
		result = value
	case kuaishou.FetchResult:
		result = &value
	case json.RawMessage:
		result = &kuaishou.FetchResult{}
		if err := json.Unmarshal(value, result); err != nil {
			return nil, fmt.Errorf("解析快手视频数据失败: %w", err)
		}
	case []byte:
		return fetch_result_from_data(json.RawMessage(value))
	default:
		return nil, fmt.Errorf("不支持的快手数据类型 %T", data)
	}
	return validate_result(result)
}

func validate_result(result *kuaishou.FetchResult) (*kuaishou.FetchResult, error) {
	if result == nil {
		return nil, errors.New("快手视频数据为空")
	}
	photo_id := strings.TrimSpace(result.PhotoID)
	if photo_id == "" {
		photo_id = strings.TrimSpace(result.Feed.Photo.ID)
	}
	if photo_id == "" {
		return nil, errors.New("快手作品 ID 为空")
	}
	result.PhotoID = photo_id
	result.Feed.Photo.ID = photo_id
	if len(build_video_variants(result)) == 0 {
		return nil, fmt.Errorf("快手视频 %s 没有可下载的视频流", photo_id)
	}
	return result, nil
}

func to_content(result *kuaishou.FetchResult) (*model.Content, error) {
	result, err := validate_result(result)
	if err != nil {
		return nil, err
	}
	photo := result.Feed.Photo
	tags := make([]string, 0, len(result.Feed.Tags))
	for _, tag := range result.Feed.Tags {
		if tag_name := strings.TrimSpace(tag.Name); tag_name != "" {
			tags = append(tags, tag_name)
		}
	}
	tags_json, _ := json.Marshal(tags)
	metadata_json, _ := json.Marshal(map[string]any{
		"feed_type":          result.Feed.Type,
		"origin_caption":     strings.TrimSpace(photo.OriginCaption),
		"exp_tag":            strings.TrimSpace(photo.ExpTag),
		"video_ratio":        photo.VideoRatio,
		"stereo_type":        photo.StereoType,
		"music_blocked":      photo.MusicBlocked,
		"animated_cover_url": strings.TrimSpace(photo.AnimatedCoverURL),
		"author_statement":   result.Feed.AuthorStatement,
	})
	publish_time := normalize_timestamp(photo.TimestampValue())
	var publish_time_pointer *int64
	if publish_time > 0 {
		publish_time_pointer = &publish_time
	}
	now := util.NowMillis()
	return &model.Content{
		Id:           build_content_id(result.PhotoID),
		PlatformId:   PlatformID,
		Type:         model.ContentTypeVideo,
		ExternalId:   result.PhotoID,
		Title:        video_title(result),
		Description:  first_non_empty(photo.OriginCaption, photo.Caption),
		URL:          canonical_page_url(result),
		SourceURL:    first_non_empty(result.SourceURL, result.PageURL),
		CoverURL:     strings.TrimSpace(photo.CoverURL),
		PublishTime:  publish_time_pointer,
		ViewCount:    photo.ViewCountValue(),
		LikeCount:    photo.LikeCountValue(),
		CommentCount: photo.CommentCountValue(),
		Tags:         string(tags_json),
		Metadata:     string(metadata_json),
		Timestamps:   model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}, nil
}

func to_account(result *kuaishou.FetchResult) (*model.Account, error) {
	result, err := validate_result(result)
	if err != nil {
		return nil, err
	}
	author_id := strings.TrimSpace(result.Feed.Author.ID)
	if author_id == "" {
		author_id = result.PhotoID
	}
	nickname := strings.TrimSpace(result.Feed.Author.Name)
	if nickname == "" {
		nickname = "快手用户"
	}
	now := util.NowMillis()
	return &model.Account{
		Id:         build_account_id(author_id),
		PlatformId: PlatformID,
		ExternalId: author_id,
		Nickname:   nickname,
		AvatarURL:  strings.TrimSpace(result.Feed.Author.HeaderURL),
		ProfileURL: "https://www.kuaishou.com/profile/" + url.PathEscape(author_id),
		Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}, nil
}

func to_content_video(result *kuaishou.FetchResult) (*model.ContentVideo, *video_variant, error) {
	result, err := validate_result(result)
	if err != nil {
		return nil, nil, err
	}
	variants := build_video_variants(result)
	selected_variant := select_video_variant(variants, nil)
	if selected_variant == nil {
		return nil, nil, fmt.Errorf("快手视频 %s 没有可下载的视频流", result.PhotoID)
	}
	content_variants := make([]model.ContentVideoVariant, 0, len(variants))
	for variant_index := range variants {
		content_variants = append(content_variants, variants[variant_index].model)
	}
	duration := milliseconds_to_seconds(result.Feed.Photo.DurationMillis())
	if duration == 0 {
		duration = selected_variant.duration
	}
	width := pointer_int_value(selected_variant.model.Width)
	height := pointer_int_value(selected_variant.model.Height)
	fps := pointer_int_value(selected_variant.model.FPS)
	bitrate := pointer_int_value(selected_variant.model.Bitrate)
	return &model.ContentVideo{
		Id:              build_content_id(result.PhotoID),
		Duration:        duration,
		Width:           width,
		Height:          height,
		FPS:             fps,
		Bitrate:         bitrate,
		Codec:           selected_variant.model.Codec,
		Format:          "mp4",
		AudioTrackCount: 1,
		URL:             selected_variant.model.URL,
		PlayTimes:       result.Feed.Photo.ViewCountValue(),
		Variants:        content_variants,
	}, selected_variant, nil
}

func to_content_details(result *kuaishou.FetchResult) ([]adapter.ContentDetail, error) {
	content, err := to_content(result)
	if err != nil {
		return nil, err
	}
	account, err := to_account(result)
	if err != nil {
		return nil, err
	}
	video, _, err := to_content_video(result)
	if err != nil {
		return nil, err
	}
	return []adapter.ContentDetail{{
		Type:    content.Type,
		Key:     content.Id,
		Content: content,
		Data:    video,
		Accounts: []adapter.ContentAccountReference{{
			Account: account,
			Role:    "owner",
		}},
	}}, nil
}

func build_download_task(result *kuaishou.FetchResult, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	result, err := validate_result(result)
	if err != nil {
		return nil, err
	}
	download_config, err := parse_download_config(config_json)
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
	video, _, err := to_content_video(result)
	if err != nil {
		return nil, err
	}
	variants := build_video_variants(result)
	selected_variant := select_video_variant(variants, download_config)
	if selected_variant == nil {
		return nil, fmt.Errorf("快手视频 %s 没有匹配的清晰度", result.PhotoID)
	}
	video.URL = selected_variant.model.URL
	video.Width = pointer_int_value(selected_variant.model.Width)
	video.Height = pointer_int_value(selected_variant.model.Height)
	video.FPS = pointer_int_value(selected_variant.model.FPS)
	video.Bitrate = pointer_int_value(selected_variant.model.Bitrate)
	video.Codec = selected_variant.model.Codec

	task_name := config_string(download_config, "filename")
	if task_name == "" {
		task_name = content.Title
	}
	normalized_config_json, _ := json.Marshal(download_config)
	metadata_json, _ := json.Marshal(map[string]any{
		"platform":          PlatformID,
		"external_id":       content.ExternalId,
		"title":             content.Title,
		"publisher_id":      account.ExternalId,
		"video_variant_key": selected_variant.model.VariantKey,
		"video_quality":     selected_variant.model.Quality,
	})
	resource_extra, _ := json.Marshal(map[string]any{
		"photo_id":    content.ExternalId,
		"source_url":  content.SourceURL,
		"page_url":    canonical_page_url(result),
		"variant_key": selected_variant.model.VariantKey,
	})
	headers_json := media_headers_json(canonical_page_url(result))
	endpoints := build_media_endpoints(selected_variant, headers_json)
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("快手视频 %s 下载地址为空", content.ExternalId)
	}
	content_id := content.Id
	resources := make([]*adapter.ResourceInfo, 0, 2)
	if config_bool(download_config, "download_cover") && content.CoverURL != "" {
		resources = append(resources, &adapter.ResourceInfo{
			Resource: model.DownloadResource{
				ContentId: &content_id,
				Name:      task_name + "_cover",
				Kind:      "image",
				UniqueID:  content.ExternalId + "_cover",
				Extra:     string(resource_extra),
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: endpoint_protocol(content.CoverURL),
				URL:      content.CoverURL,
				Enabled:  1,
				Headers:  headers_json,
			}},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind:     model.ContentAssetKindImage,
				Role:     model.ContentAssetRoleCover,
				AssetKey: "cover",
				Relation: model.DownloadResourceAssetRelationSource,
			}},
		})
	}
	resources = append(resources, &adapter.ResourceInfo{
		Resource: model.DownloadResource{
			ContentId: &content_id,
			Name:      task_name,
			Kind:      "video/mp4",
			UniqueID:  content.ExternalId + "_" + sanitize_variant_key(selected_variant.model.VariantKey),
			Duration:  video.Duration,
			Extra:     string(resource_extra),
		},
		Endpoints: endpoints,
		ContentAssets: []adapter.ContentAssetReference{{
			Kind:     model.ContentAssetKindVideo,
			Role:     model.ContentAssetRoleVideoVariant,
			AssetKey: selected_variant.model.VariantKey,
			Relation: model.DownloadResourceAssetRelationSource,
		}},
	})
	now := util.NowMillis()
	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content_id,
			Name:         task_name,
			UniqueID:     content.ExternalId,
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			SourceURL:    content.SourceURL,
			CoverURL:     content.CoverURL,
			ConfigJSON:   string(normalized_config_json),
			MetadataJSON: string(metadata_json),
			Timestamps:   model.Timestamps{CreatedAt: now, UpdatedAt: now},
		},
		Resources:      resources,
		ContentDetail:  video,
		ContentDetails: content_details(content, account, video),
		Account:        account,
		Content:        content,
	}, nil
}

func content_details(content *model.Content, account *model.Account, video *model.ContentVideo) []adapter.ContentDetail {
	return []adapter.ContentDetail{{
		Type:    content.Type,
		Key:     content.Id,
		Content: content,
		Data:    video,
		Accounts: []adapter.ContentAccountReference{{
			Account: account,
			Role:    "owner",
		}},
	}}
}

func build_video_variants(result *kuaishou.FetchResult) []video_variant {
	if result == nil {
		return nil
	}
	content_id := build_content_id(first_non_empty(result.PhotoID, result.Feed.Photo.ID))
	now := util.NowMillis()
	variants := make([]video_variant, 0)
	seen_urls := make(map[string]int)
	append_manifest_variants := func(manifest kuaishou.Manifest, codec_fallback string) {
		for adaptation_index, adaptation_set := range manifest.AdaptationSet {
			for representation_index, representation := range adaptation_set.Representation {
				video_url := strings.TrimSpace(representation.URL)
				if video_url == "" {
					for _, backup_url := range representation.BackupURLs() {
						if video_url = strings.TrimSpace(backup_url); video_url != "" {
							break
						}
					}
				}
				if video_url == "" {
					continue
				}
				codec := first_non_empty(representation.Codecs, codec_fallback)
				quality := first_non_empty(representation.QualityLabel, representation.QualityType)
				representation_id := interface_string(representation.ID)
				variant_key := manifest_variant_key(codec, quality, representation_id, adaptation_index, representation_index)
				metadata_json, _ := json.Marshal(map[string]any{
					"adaptation_id":     adaptation_set.ID,
					"representation_id": representation.ID,
					"maximum_bitrate":   representation.MaximumBitrate,
					"backup_urls":       representation.BackupURLs(),
				})
				variant_model := model.ContentVideoVariant{
					VideoId:    content_id,
					VariantKey: variant_key,
					Spec:       first_non_empty(representation_id, representation.QualityType),
					Quality:    quality,
					Codec:      codec,
					Format:     "mp4",
					StreamType: model.ContentVideoVariantStreamTypeProgressive,
					HasVideo:   1,
					HasAudio:   1,
					IsDefault:  bool_to_int(representation.IsDefault()),
					URL:        video_url,
					Metadata:   string(metadata_json),
					Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
				}
				variant_model.Width = positive_int_pointer(representation.WidthValue())
				variant_model.Height = positive_int_pointer(representation.HeightValue())
				variant_model.FPS = positive_int_pointer(representation.FrameRateValue())
				variant_model.Bitrate = positive_int_pointer(representation.AverageBitrateValue())
				backups := unique_urls(representation.BackupURLs(), video_url)
				seen_urls[video_url] = len(variants)
				variants = append(variants, video_variant{
					model:    variant_model,
					backups:  backups,
					duration: milliseconds_to_seconds(adaptation_set.DurationMillis()),
				})
			}
		}
	}
	append_manifest_variants(result.Feed.Photo.Manifest, "h264")
	append_manifest_variants(result.Feed.Photo.ManifestH265, "h265")
	append_direct_variant := func(video_url string, variant_key string, codec string, mark_default bool) {
		video_url = strings.TrimSpace(video_url)
		if video_url == "" {
			return
		}
		if mark_default {
			for variant_index := range variants {
				variants[variant_index].model.IsDefault = 0
			}
		}
		if variant_index, exists := seen_urls[video_url]; exists {
			if mark_default {
				variants[variant_index].model.IsDefault = 1
			}
			if variants[variant_index].model.Codec == "" {
				variants[variant_index].model.Codec = codec
			}
			return
		}
		seen_urls[video_url] = len(variants)
		variants = append(variants, video_variant{model: model.ContentVideoVariant{
			VideoId:    content_id,
			VariantKey: variant_key,
			Spec:       "default",
			Quality:    "default",
			Codec:      codec,
			Format:     "mp4",
			StreamType: model.ContentVideoVariantStreamTypeProgressive,
			HasVideo:   1,
			HasAudio:   1,
			IsDefault:  bool_to_int(mark_default),
			URL:        video_url,
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}})
	}
	photo_url := strings.TrimSpace(result.Feed.Photo.PhotoURL)
	append_direct_variant(photo_url, "h264:default", "h264", true)
	append_direct_variant(result.Feed.Photo.PhotoH265URL, "h265:default", "h265", photo_url == "")
	return unique_variant_keys(variants)
}

func unique_variant_keys(variants []video_variant) []video_variant {
	key_counts := make(map[string]int, len(variants))
	for variant_index := range variants {
		variant_key := variants[variant_index].model.VariantKey
		key_counts[variant_key]++
		if key_counts[variant_key] > 1 {
			variants[variant_index].model.VariantKey = variant_key + ":" + strconv.Itoa(key_counts[variant_key])
		}
	}
	return variants
}

func select_video_variant(variants []video_variant, download_config map[string]any) *video_variant {
	if len(variants) == 0 {
		return nil
	}
	configured_variant_key := config_string(download_config, "video_variant_key")
	if configured_variant_key != "" {
		for variant_index := range variants {
			if strings.EqualFold(variants[variant_index].model.VariantKey, configured_variant_key) {
				return &variants[variant_index]
			}
		}
	}
	configured_variant_spec := first_non_empty(
		config_string(download_config, "video_variant_spec"),
		config_string(download_config, "spec"),
	)
	if configured_variant_spec != "" {
		for variant_index := range variants {
			if strings.EqualFold(variants[variant_index].model.Spec, configured_variant_spec) {
				return &variants[variant_index]
			}
		}
	}
	quality := strings.ToLower(config_string(download_config, "quality"))
	if quality != "" && quality != "default" && quality != "highest" && quality != "best" {
		for variant_index := range variants {
			variant := &variants[variant_index]
			if strings.EqualFold(variant.model.VariantKey, quality) || strings.EqualFold(variant.model.Quality, quality) || strings.EqualFold(variant.model.Spec, quality) {
				return variant
			}
		}
	}
	if quality == "highest" || quality == "best" {
		sort.SliceStable(variants, func(left_index, right_index int) bool {
			left_area := pointer_int_value(variants[left_index].model.Width) * pointer_int_value(variants[left_index].model.Height)
			right_area := pointer_int_value(variants[right_index].model.Width) * pointer_int_value(variants[right_index].model.Height)
			if left_area != right_area {
				return left_area > right_area
			}
			return pointer_int_value(variants[left_index].model.Bitrate) > pointer_int_value(variants[right_index].model.Bitrate)
		})
		return &variants[0]
	}
	for variant_index := range variants {
		if variants[variant_index].model.IsDefault == 1 {
			return &variants[variant_index]
		}
	}
	return &variants[0]
}

func build_media_endpoints(variant *video_variant, headers_json string) []model.DownloadEndpoint {
	if variant == nil {
		return nil
	}
	endpoint_urls := unique_urls(append([]string{variant.model.URL}, variant.backups...), "")
	endpoints := make([]model.DownloadEndpoint, 0, len(endpoint_urls))
	for endpoint_index, endpoint_url := range endpoint_urls {
		endpoints = append(endpoints, model.DownloadEndpoint{
			Protocol: endpoint_protocol(endpoint_url),
			URL:      endpoint_url,
			Priority: endpoint_index,
			Enabled:  1,
			Headers:  headers_json,
		})
	}
	return endpoints
}

func media_headers_json(page_url string) string {
	headers_json, _ := json.Marshal(map[string]string{
		"Accept":          "*/*",
		"Accept-Language": "zh-CN,zh;q=0.9",
		"Cache-Control":   "no-cache",
		"Origin":          "https://www.kuaishou.com",
		"Pragma":          "no-cache",
		"Referer":         page_url,
		"User-Agent":      kuaishou.DefaultUserAgent,
	})
	return string(headers_json)
}

func parse_download_config(config_json json.RawMessage) (map[string]any, error) {
	download_config := make(map[string]any)
	config_text := strings.TrimSpace(string(config_json))
	if config_text == "" || config_text == "null" {
		return download_config, nil
	}
	if err := json.Unmarshal(config_json, &download_config); err != nil {
		return nil, fmt.Errorf("解析快手下载配置失败: %w", err)
	}
	return download_config, nil
}

func config_string(download_config map[string]any, key string) string {
	if download_config == nil {
		return ""
	}
	value, _ := download_config[key].(string)
	return strings.TrimSpace(value)
}

func config_bool(download_config map[string]any, key string) bool {
	if download_config == nil {
		return false
	}
	value, _ := download_config[key].(bool)
	return value
}

func build_content_id(photo_id string) string {
	return PlatformID + ":" + strings.TrimSpace(photo_id)
}

func build_account_id(author_id string) string {
	return PlatformID + ":" + strings.TrimSpace(author_id)
}

func video_title(result *kuaishou.FetchResult) string {
	title := strings.TrimSpace(result.Feed.Photo.Caption)
	if title == "" {
		title = strings.TrimSpace(result.Feed.Photo.OriginCaption)
	}
	if title == "" {
		title = "快手视频_" + result.PhotoID
	}
	return title
}

func canonical_page_url(result *kuaishou.FetchResult) string {
	if result != nil && strings.TrimSpace(result.PageURL) != "" {
		return strings.TrimSpace(result.PageURL)
	}
	if result == nil {
		return ""
	}
	return "https://www.kuaishou.com/short-video/" + url.PathEscape(result.PhotoID)
}

func normalize_timestamp(timestamp int64) int64 {
	if timestamp <= 0 {
		return 0
	}
	if timestamp < 100000000000 {
		return timestamp * 1000
	}
	return timestamp
}

func milliseconds_to_seconds(milliseconds int64) int64 {
	if milliseconds <= 0 {
		return 0
	}
	seconds := milliseconds / 1000
	if seconds == 0 {
		return 1
	}
	return seconds
}

func positive_int_pointer(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func pointer_int_value(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func bool_to_int(value bool) int {
	if value {
		return 1
	}
	return 0
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func interface_string(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func unique_urls(values []string, excluded_url string) []string {
	excluded_url = strings.TrimSpace(excluded_url)
	seen_urls := make(map[string]struct{}, len(values))
	urls := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == excluded_url {
			continue
		}
		if _, exists := seen_urls[value]; exists {
			continue
		}
		seen_urls[value] = struct{}{}
		urls = append(urls, value)
	}
	return urls
}

func manifest_variant_key(codec string, quality string, representation_id string, adaptation_index int, representation_index int) string {
	parts := []string{strings.ToLower(strings.TrimSpace(codec)), strings.ToLower(strings.TrimSpace(quality)), strings.TrimSpace(representation_id)}
	filtered_parts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			filtered_parts = append(filtered_parts, strings.ReplaceAll(part, " ", "_"))
		}
	}
	if len(filtered_parts) == 0 {
		return fmt.Sprintf("video:%d:%d", adaptation_index+1, representation_index+1)
	}
	return strings.Join(filtered_parts, ":")
}

func sanitize_variant_key(variant_key string) string {
	variant_key = strings.TrimSpace(variant_key)
	if variant_key == "" {
		return "default"
	}
	replacer := strings.NewReplacer(":", "_", "/", "_", "\\", "_", " ", "_")
	return replacer.Replace(variant_key)
}

func endpoint_protocol(endpoint_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(endpoint_url))
	if err == nil && parsed_url.Scheme != "" {
		return strings.ToLower(parsed_url.Scheme)
	}
	return "https"
}
