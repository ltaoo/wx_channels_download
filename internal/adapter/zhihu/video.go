package zhihuadapter

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/zhihu"
)

var zhihu_video_url_re = regexp.MustCompile(`(?i)(?:www\.)?zhihu\.com/video/([0-9]+)|/video/([0-9]+)`)
var zhihu_video_id_re = regexp.MustCompile(`^[0-9]+$`)

type zhihu_embedded_video_info struct {
	video_id string
	info     *zhihu.VideoPlayInfo
}

func embedded_zhihu_video_ids(content_html string) []string {
	if strings.TrimSpace(content_html) == "" {
		return nil
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(content_html))
	if err != nil {
		return nil
	}

	video_ids := make([]string, 0)
	seen_video_ids := make(map[string]bool)
	append_video_id := func(video_id string) {
		video_id = strings.TrimSpace(video_id)
		if !zhihu_video_id_re.MatchString(video_id) || seen_video_ids[video_id] {
			return
		}
		seen_video_ids[video_id] = true
		video_ids = append(video_ids, video_id)
	}
	document.Find("a.video-box, a[href]").Each(func(_ int, selection *goquery.Selection) {
		if selection.HasClass("video-box") {
			append_video_id(first_non_empty_str(
				selection.AttrOr("data-lens-id", ""),
				selection.AttrOr("data-video-id", ""),
			))
		}
		append_video_id(zhihu_video_id_from_url(selection.AttrOr("href", "")))
		append_video_id(zhihu_video_id_from_text(selection.Text()))
	})
	for _, matches := range zhihu_video_url_re.FindAllStringSubmatch(html.UnescapeString(content_html), -1) {
		append_video_id(first_non_empty_str(matches[1], matches[2]))
	}
	return video_ids
}

func zhihu_video_id_from_url(raw_url string) string {
	raw_url = html.UnescapeString(strings.TrimSpace(raw_url))
	if raw_url == "" {
		return ""
	}
	parsed_url, err := url.Parse(raw_url)
	if err != nil {
		return zhihu_video_id_from_text(raw_url)
	}
	if strings.EqualFold(parsed_url.Hostname(), "link.zhihu.com") {
		return zhihu_video_id_from_url(parsed_url.Query().Get("target"))
	}
	if parsed_url.Hostname() != "" && !strings.EqualFold(parsed_url.Hostname(), "zhihu.com") && !strings.EqualFold(parsed_url.Hostname(), "www.zhihu.com") {
		return ""
	}
	return zhihu_video_id_from_text(parsed_url.Path)
}

func zhihu_video_id_from_text(value string) string {
	matches := zhihu_video_url_re.FindStringSubmatch(html.UnescapeString(strings.TrimSpace(value)))
	if len(matches) != 3 {
		return ""
	}
	return first_non_empty_str(matches[1], matches[2])
}

func fetch_zhihu_embedded_video_infos(client *zhihu.Client, page_data any, root_content *model.Content) ([]zhihu_embedded_video_info, error) {
	if client == nil || root_content == nil {
		return nil, nil
	}
	video_ids := embedded_zhihu_video_ids(zhihu_primary_content_html(page_data))
	if len(video_ids) == 0 {
		return nil, nil
	}
	content_id := strings.TrimSpace(root_content.ExternalId)
	content_type := strings.ToLower(strings.TrimSpace(root_content.Type))
	if content_id == "" || content_type == "" {
		return nil, fmt.Errorf("知乎视频缺少所属内容信息")
	}
	referer := first_non_empty_str(root_content.SourceURL, root_content.URL)
	scene_code := zhihu_video_scene_code(content_type)
	video_infos := make([]zhihu_embedded_video_info, 0, len(video_ids))
	for _, video_id := range video_ids {
		info, err := client.FetchVideoPlayInfo(content_id, content_type, video_id, scene_code, referer)
		if err != nil {
			return nil, fmt.Errorf("获取知乎视频 %s 播放信息失败: %w", video_id, err)
		}
		if info.VideoPlay.ID != video_id {
			return nil, fmt.Errorf("知乎视频播放信息 ID 不匹配: want %s, got %s", video_id, info.VideoPlay.ID)
		}
		if !has_playable_zhihu_mp4(info) {
			return nil, fmt.Errorf("知乎视频 %s 没有可下载的 MP4 规格", video_id)
		}
		video_infos = append(video_infos, zhihu_embedded_video_info{video_id: video_id, info: info})
	}
	return video_infos, nil
}

func has_playable_zhihu_mp4(info *zhihu.VideoPlayInfo) bool {
	if info == nil {
		return false
	}
	for variant_index := range info.VideoPlay.Playlist.MP4 {
		if first_zhihu_video_url(info.VideoPlay.Playlist.MP4[variant_index].URL) != "" {
			return true
		}
	}
	return false
}

func zhihu_video_scene_code(content_type string) string {
	switch strings.ToLower(strings.TrimSpace(content_type)) {
	case "answer":
		return "answer_detail_web"
	case "article":
		return "article_detail_web"
	case "question":
		return "question_detail_web"
	default:
		return strings.ToLower(strings.TrimSpace(content_type)) + "_detail_web"
	}
}

func build_zhihu_embedded_videos(
	root_content *model.Content,
	video_infos []zhihu_embedded_video_info,
	resource_key string,
	selected_variant_key string,
	selected_variant_spec string,
) ([]*adapter.ResourceInfo, []adapter.ContentDetail) {
	if root_content == nil || len(video_infos) == 0 {
		return nil, nil
	}
	resources := make([]*adapter.ResourceInfo, 0, len(video_infos))
	details := make([]adapter.ContentDetail, 0, len(video_infos))
	for video_index := range video_infos {
		video_info := video_infos[video_index]
		if video_info.info == nil {
			continue
		}
		variants := zhihu_content_video_variants(
			video_info.video_id,
			video_info.info,
			selected_variant_key,
			selected_variant_spec,
		)
		selected_variant := selected_zhihu_content_video_variant(variants)
		if selected_variant == nil {
			continue
		}
		video_number := len(resources) + 1
		video_content := zhihu_embedded_video_content(root_content, video_info, video_number)
		content_video := zhihu_embedded_content_video(video_content.Id, variants, selected_variant)
		resources = append(resources, zhihu_embedded_video_resource(
			video_content.Id,
			resource_key,
			video_info,
			video_number,
			selected_variant,
			first_non_empty_str(root_content.SourceURL, root_content.URL),
		))
		details = append(details, adapter.ContentDetail{
			Type:    model.ContentTypeVideo,
			Key:     video_content.Id,
			Data:    content_video,
			Content: video_content,
			Relation: &model.ContentRelation{
				SourceContentId: root_content.Id,
				TargetContentId: video_content.Id,
				Type:            model.ContentRelationContains,
				SortOrder:       video_index,
				CreatedAt:       root_content.CreatedAt,
			},
		})
	}
	return resources, details
}

func zhihu_embedded_video_content(root_content *model.Content, video_info zhihu_embedded_video_info, video_number int) *model.Content {
	video_title := fmt.Sprintf("视频 %d", video_number)
	if strings.TrimSpace(root_content.Title) != "" {
		video_title = fmt.Sprintf("%s - 视频 %d", strings.TrimSpace(root_content.Title), video_number)
	}
	cover_url := strings.TrimSpace(video_info.info.VideoPlay.DefaultCover)
	if cover_url == "" {
		quality := strings.TrimSpace(video_info.info.VideoPlay.Meta.Resolution.Quality)
		cover_url = strings.TrimSpace(video_info.info.VideoPlay.BeginFrame[quality])
	}
	if cover_url == "" {
		for _, quality := range []string{"FHD", "HD", "SD"} {
			if cover_url = strings.TrimSpace(video_info.info.VideoPlay.BeginFrame[quality]); cover_url != "" {
				break
			}
		}
	}
	video_url := "https://www.zhihu.com/video/" + url.PathEscape(video_info.video_id)
	return &model.Content{
		Id:          BuildTypedContentID(model.ContentTypeVideo, video_info.video_id),
		PlatformId:  PlatformID,
		Type:        model.ContentTypeVideo,
		ExternalId:  video_info.video_id,
		Title:       video_title,
		URL:         video_url,
		SourceURL:   first_non_empty_str(root_content.SourceURL, root_content.URL),
		CoverURL:    cover_url,
		PublishTime: root_content.PublishTime,
		UpdateTime:  root_content.UpdateTime,
		Timestamps:  root_content.Timestamps,
	}
}

func zhihu_embedded_content_video(video_content_id string, variants []model.ContentVideoVariant, selected_variant *model.ContentVideoVariant) *model.ContentVideo {
	content_video := &model.ContentVideo{
		Id:       video_content_id,
		Duration: zhihu_variant_duration_seconds(selected_variant),
		Size:     selected_variant.Size,
		Codec:    selected_variant.Codec,
		Format:   selected_variant.Format,
		URL:      selected_variant.URL,
		Variants: variants,
	}
	if selected_variant.Width != nil {
		content_video.Width = *selected_variant.Width
	}
	if selected_variant.Height != nil {
		content_video.Height = *selected_variant.Height
	}
	if selected_variant.FPS != nil {
		content_video.FPS = *selected_variant.FPS
	}
	if selected_variant.Bitrate != nil {
		content_video.Bitrate = *selected_variant.Bitrate
	}
	return content_video
}

func zhihu_content_video_variants(video_id string, info *zhihu.VideoPlayInfo, selected_variant_key string, selected_variant_spec string) []model.ContentVideoVariant {
	if info == nil {
		return nil
	}
	video_content_id := BuildTypedContentID(model.ContentTypeVideo, video_id)
	variants := make([]model.ContentVideoVariant, 0, len(info.VideoPlay.Playlist.MP4))
	seen_variant_keys := make(map[string]bool)
	for candidate_index := range info.VideoPlay.Playlist.MP4 {
		candidate := &info.VideoPlay.Playlist.MP4[candidate_index]
		video_url := first_zhihu_video_url(candidate.URL)
		if video_url == "" {
			continue
		}
		variant_key := zhihu_video_variant_key(video_id, candidate, video_url)
		if seen_variant_keys[variant_key] {
			continue
		}
		seen_variant_keys[variant_key] = true
		metadata, _ := json.Marshal(map[string]any{
			"name":        candidate.Name,
			"label":       candidate.Label,
			"quality":     candidate.Quality,
			"hdr_type":    candidate.HDRType,
			"max_bitrate": candidate.MaxBitrate,
			"duration":    candidate.Duration,
			"channels":    candidate.Channels,
			"sample_rate": candidate.SampleRate,
		})
		variant := model.ContentVideoVariant{
			VideoId:      video_content_id,
			VariantKey:   variant_key,
			Quality:      first_non_empty_str(candidate.Label, candidate.Name, candidate.Quality),
			Size:         candidate.Size,
			Codec:        strings.TrimSpace(candidate.Codec),
			Format:       first_non_empty_str(candidate.Format, "mp4"),
			StreamType:   model.ContentVideoVariantStreamTypeProgressive,
			HasVideo:     1,
			HasAudio:     1,
			URL:          video_url,
			URLExpiresAt: zhihu_video_url_expires_at(video_url),
			Metadata:     string(metadata),
		}
		if candidate.Key > 0 {
			variant.Spec = strconv.Itoa(candidate.Key)
		}
		variant.Width = positive_zhihu_int_pointer(candidate.Width)
		variant.Height = positive_zhihu_int_pointer(candidate.Height)
		variant.FPS = positive_zhihu_int_pointer(candidate.FPS)
		variant.Bitrate = positive_zhihu_int_pointer(candidate.Bitrate)
		variants = append(variants, variant)
	}

	selected_index := configured_zhihu_variant_index(variants, selected_variant_key, selected_variant_spec)
	if selected_index < 0 {
		selected_index = default_zhihu_variant_index(info, variants)
	}
	if selected_index >= 0 {
		variants[selected_index].IsDefault = 1
	}
	return variants
}

func configured_zhihu_variant_index(variants []model.ContentVideoVariant, selected_variant_key string, selected_variant_spec string) int {
	selected_variant_key = strings.TrimSpace(selected_variant_key)
	selected_variant_spec = strings.TrimSpace(selected_variant_spec)
	for variant_index := range variants {
		variant := &variants[variant_index]
		if selected_variant_key != "" && variant.VariantKey == selected_variant_key {
			return variant_index
		}
		if selected_variant_key == "" && selected_variant_spec != "" && variant.Spec == selected_variant_spec {
			return variant_index
		}
	}
	return -1
}

func default_zhihu_variant_index(info *zhihu.VideoPlayInfo, variants []model.ContentVideoVariant) int {
	best_index := -1
	best_score := int64(-1)
	for variant_index := range variants {
		variant := &variants[variant_index]
		score := zhihu_variant_default_score(info, variant)
		if score > best_score {
			best_index = variant_index
			best_score = score
		}
	}
	return best_index
}

func zhihu_variant_default_score(info *zhihu.VideoPlayInfo, variant *model.ContentVideoVariant) int64 {
	if info == nil || variant == nil {
		return -1
	}
	resolution := info.VideoPlay.Meta.Resolution
	metadata := zhihu_variant_metadata(variant)
	score := int64(0)
	if strings.EqualFold(metadata.Quality, resolution.Quality) {
		score += 3_000_000_000_000_000_000
	}
	if variant.Width != nil && variant.Height != nil && *variant.Width == resolution.Width && *variant.Height == resolution.Height {
		score += 2_000_000_000_000_000_000
	}
	if strings.EqualFold(strings.TrimSpace(variant.Codec), "H264") || strings.EqualFold(strings.TrimSpace(variant.Codec), "AVC") {
		score += 1_000_000_000_000_000_000
	}
	if variant.Width != nil && variant.Height != nil {
		score += int64(*variant.Width) * int64(*variant.Height) * 1_000_000
	}
	if variant.Bitrate != nil {
		score += int64(*variant.Bitrate)
	}
	return score
}

type zhihu_video_variant_metadata struct {
	Quality  string  `json:"quality"`
	Duration float64 `json:"duration"`
}

func zhihu_variant_metadata(variant *model.ContentVideoVariant) zhihu_video_variant_metadata {
	metadata := zhihu_video_variant_metadata{}
	if variant != nil && strings.TrimSpace(variant.Metadata) != "" {
		_ = json.Unmarshal([]byte(variant.Metadata), &metadata)
	}
	return metadata
}

func selected_zhihu_content_video_variant(variants []model.ContentVideoVariant) *model.ContentVideoVariant {
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

func zhihu_embedded_video_resource(
	content_id string,
	resource_key string,
	video_info zhihu_embedded_video_info,
	video_number int,
	selected_variant *model.ContentVideoVariant,
	referer string,
) *adapter.ResourceInfo {
	extra_data, _ := json.Marshal(map[string]string{
		"video_id":   video_info.video_id,
		"source_url": "https://www.zhihu.com/video/" + video_info.video_id,
	})
	resource := model.DownloadResource{
		ContentId:  &content_id,
		Name:       fmt.Sprintf("video_%02d", video_number),
		Kind:       "video/mp4",
		UniqueID:   fmt.Sprintf("%s_video_%s_%s", resource_key, video_info.video_id, zhihu_video_resource_variant_id(selected_variant)),
		Size:       selected_variant.Size,
		MergeOrder: 200 + video_number,
		Duration:   zhihu_variant_duration_seconds(selected_variant),
		Extra:      string(extra_data),
	}
	return &adapter.ResourceInfo{
		Resource:  resource,
		Endpoints: zhihu_video_endpoints(video_info, selected_variant, referer),
		ContentAssets: []adapter.ContentAssetReference{{
			Kind:     model.ContentAssetKindVideo,
			Role:     model.ContentAssetRoleVideoVariant,
			AssetKey: selected_variant.VariantKey,
			Relation: model.DownloadResourceAssetRelationSource,
		}},
	}
}

func zhihu_video_endpoints(video_info zhihu_embedded_video_info, selected_variant *model.ContentVideoVariant, referer string) []model.DownloadEndpoint {
	if video_info.info == nil || selected_variant == nil {
		return nil
	}
	headers_data, _ := json.Marshal(zhihu.VideoDownloadHeaders(referer))
	urls := make([]string, 0)
	for candidate_index := range video_info.info.VideoPlay.Playlist.MP4 {
		candidate := &video_info.info.VideoPlay.Playlist.MP4[candidate_index]
		if selected_variant.Spec != "" && strconv.Itoa(candidate.Key) != selected_variant.Spec {
			continue
		}
		candidate_url := first_zhihu_video_url(candidate.URL)
		if selected_variant.Spec == "" && candidate_url != selected_variant.URL {
			continue
		}
		urls = append(urls, candidate.URL...)
		break
	}
	seen_urls := make(map[string]bool)
	endpoints := make([]model.DownloadEndpoint, 0, len(urls))
	for endpoint_index, endpoint_url := range urls {
		endpoint_url = normalize_zhihu_video_url(endpoint_url)
		if endpoint_url == "" || seen_urls[endpoint_url] {
			continue
		}
		seen_urls[endpoint_url] = true
		endpoints = append(endpoints, model.DownloadEndpoint{
			Protocol: zhihu_video_endpoint_protocol(endpoint_url),
			URL:      endpoint_url,
			Priority: endpoint_index,
			Enabled:  1,
			Headers:  string(headers_data),
		})
	}
	return endpoints
}

func first_zhihu_video_url(urls []string) string {
	for _, raw_url := range urls {
		if video_url := normalize_zhihu_video_url(raw_url); video_url != "" {
			return video_url
		}
	}
	return ""
}

func normalize_zhihu_video_url(raw_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || parsed_url.Host == "" {
		return ""
	}
	parsed_url.Scheme = strings.ToLower(parsed_url.Scheme)
	if parsed_url.Scheme != "http" && parsed_url.Scheme != "https" {
		return ""
	}
	parsed_url.Fragment = ""
	return parsed_url.String()
}

func zhihu_video_variant_key(video_id string, candidate *zhihu.VideoPlayVariant, video_url string) string {
	if candidate != nil && candidate.Key > 0 {
		return fmt.Sprintf("%s:format:%d", video_id, candidate.Key)
	}
	hash := md5.Sum([]byte(video_url))
	return fmt.Sprintf("%s:url:%s", video_id, hex.EncodeToString(hash[:8]))
}

func zhihu_video_resource_variant_id(variant *model.ContentVideoVariant) string {
	if variant != nil && strings.TrimSpace(variant.Spec) != "" {
		return "format_" + strings.TrimSpace(variant.Spec)
	}
	video_url := ""
	if variant != nil {
		video_url = variant.URL
	}
	hash := md5.Sum([]byte(video_url))
	return "url_" + hex.EncodeToString(hash[:8])
}

func zhihu_video_url_expires_at(raw_url string) *int64 {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return nil
	}
	expires_at, err := strconv.ParseInt(parsed_url.Query().Get("expiration"), 10, 64)
	if err != nil || expires_at <= 0 {
		return nil
	}
	if expires_at < 1_000_000_000_000 {
		expires_at *= 1000
	}
	return &expires_at
}

func zhihu_variant_duration_seconds(variant *model.ContentVideoVariant) int64 {
	metadata := zhihu_variant_metadata(variant)
	if metadata.Duration <= 0 {
		return 0
	}
	return int64(metadata.Duration + 0.5)
}

func positive_zhihu_int_pointer(value int) *int {
	if value <= 0 {
		return nil
	}
	result := value
	return &result
}

func zhihu_video_endpoint_protocol(raw_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err == nil && parsed_url.Scheme != "" {
		return strings.ToLower(parsed_url.Scheme)
	}
	return "https"
}
