package xadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"
	"sync"
	"unicode/utf8"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/cache"
	"wx_channel/pkg/cookies"
	x_scraper "wx_channel/pkg/scraper/x"
	"wx_channel/pkg/util"
)

// PlatformID is the platform identifier for X/Twitter posts.
const PlatformID = x_scraper.PlatformID

type handler struct {
	runtime_mu    sync.RWMutex
	cookie_reader *cookies.Reader
	file_cache    *cache.CacheProvider
}

var (
	_ adapter.PlatformAdapter             = (*handler)(nil)
	_ adapter.ContextProgressFetchAdapter = (*handler)(nil)
	_ adapter.FetchDownloadTaskBuilder    = (*handler)(nil)
	_ adapter.RuntimeAdapter              = (*handler)(nil)
	_ adapter.RuntimeHandle               = (*handler)(nil)
	_ adapter.PlatformStatusDescriber     = (*handler)(nil)
	_ adapter.HomeContentsBuilder         = (*handler)(nil)
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
	h.runtime_mu.Lock()
	h.cookie_reader = adapter_options.Cookies
	h.file_cache = adapter_options.Cache
	h.runtime_mu.Unlock()
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

func (h *handler) Stop() {
	h.runtime_mu.Lock()
	h.cookie_reader = nil
	h.file_cache = nil
	h.runtime_mu.Unlock()
}

func (h *handler) Fetch(raw_url string) (any, error) {
	return h.FetchWithProgressContext(context.Background(), raw_url, adapter.FetchOptions{})
}

func (h *handler) FetchWithProgressContext(fetch_context context.Context, raw_url string, _ adapter.FetchOptions) (any, error) {
	client, err := x_scraper.NewClient(h.runtime_cookie_reader())
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
		"images":          result.Images,
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
		CoverURL:     post_cover_url(result),
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
	return content_details(result), nil
}

// content_details is the post's detail list in viewer order: every attached
// video, then the photo grid, then the post body. A post can carry several
// videos and photos at once, so every detail needs a key of its own -- the
// scraper job and the viewer both collapse details that share one.
func content_details(result *x_scraper.FetchResult) []adapter.ContentDetail {
	if result == nil {
		return nil
	}
	videos := content_videos(result)
	details := make([]adapter.ContentDetail, 0, len(videos)+2)
	for video_index, video := range videos {
		details = append(details, adapter.ContentDetail{
			Type:    model.ContentTypeVideo,
			Key:     video.Id,
			Content: video_content(result, video_index, video),
			Data:    video,
			// A post's own record cannot carry a video's downloads: the viewer
			// lists resources filed under the post or under a `contains` child,
			// so every video gets a child content record of its own.
			Relation: &model.ContentRelation{
				SourceContentId: PlatformID + ":" + result.ExternalID,
				TargetContentId: video.Id,
				Type:            model.ContentRelationContains,
				SortOrder:       video_index,
			},
		})
	}
	// A photo post is a gallery: its images come before the caption so the
	// viewer shows the grid first.
	if album := content_album(result); album != nil {
		details = append(details, adapter.ContentDetail{Type: model.ContentTypeAlbum, Key: album.Id + ":album", Data: album})
	}
	// A post whose whole content is its text still needs one detail record, so
	// the long-form article body (or the tweet text) stays viewable.
	if article := content_article(result); article != nil {
		details = append(details, adapter.ContentDetail{Type: model.ContentTypePost, Key: article.Id + ":post", Data: article})
	}
	return details
}

// x_video_content_id addresses one attached video. The post id alone cannot:
// X allows more than one video per post.
func x_video_content_id(external_id string, video_index int) string {
	return fmt.Sprintf("%s:%s:video:%d", PlatformID, external_id, video_index+1)
}

func content_album(result *x_scraper.FetchResult) *model.ContentAlbum {
	if result == nil || len(result.Images) == 0 {
		return nil
	}
	album := &model.ContentAlbum{
		Id:          PlatformID + ":" + result.ExternalID,
		ImageCount:  len(result.Images),
		CoverWidth:  result.Images[0].Width,
		CoverHeight: result.Images[0].Height,
		Description: strings.TrimSpace(result.BodyText),
	}
	for image_index, source_image := range result.Images {
		album.Images = append(album.Images, model.ContentImage{
			AlbumId:   album.Id,
			ImageKey:  model.BuildContentAlbumImageKey(source_image.ID, source_image.URL, image_index),
			SortOrder: image_index,
			URL:       source_image.URL,
			Width:     source_image.Width,
			Height:    source_image.Height,
			Ext:       image_extension(source_image.URL),
			ImageType: model.ContentImageTypeStill,
		})
	}
	return album
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
	resources := make([]*adapter.ResourceInfo, 0, 2+len(result.Images))
	body_text := post_text(result)
	if body_text != "" {
		resources = append(resources, &adapter.ResourceInfo{
			Resource:  model.DownloadResource{ContentId: &content_id, Name: task_name, Kind: "text/plain", UniqueID: result.ExternalID + "_body", Size: int64(len(body_text))},
			Endpoints: []model.DownloadEndpoint{{Protocol: "inline", URL: body_text, Enabled: 1}},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind: model.ContentAssetKindText, Role: model.ContentAssetRoleArticleBody, AssetKey: "body:text", Relation: model.DownloadResourceAssetRelationSource,
			}},
		})
	}
	// One resource per attached video; each carries the id of its own video
	// detail so the saved variant links back to the right record.
	for video_index := range result.Videos {
		video_name := task_name
		if len(result.Videos) > 1 {
			video_name += fmt.Sprintf("_%02d", video_index+1)
		}
		resource, err := video_resource(
			x_video_content_id(result.ExternalID, video_index),
			video_name,
			result.SourceURL,
			&result.Videos[video_index],
		)
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	image_headers, _ := json.Marshal(map[string]string{"Accept": "image/avif,image/webp,image/apng,image/*,*/*;q=0.8", "Referer": result.SourceURL, "User-Agent": x_scraper.DefaultUserAgent})
	for image_index, source_image := range result.Images {
		image_name := task_name
		if len(result.Images) > 1 {
			image_name += fmt.Sprintf("_%02d", image_index+1)
		}
		image_key := model.BuildContentAlbumImageKey(source_image.ID, source_image.URL, image_index)
		resources = append(resources, &adapter.ResourceInfo{
			Resource: model.DownloadResource{
				ContentId: &content_id, Name: image_name, Kind: image_mime_type(image_extension(source_image.URL)),
				UniqueID: fmt.Sprintf("%s_image_%d", result.ExternalID, image_index+1), MergeOrder: image_index,
			},
			Endpoints: []model.DownloadEndpoint{{Protocol: "https", URL: source_image.URL, Enabled: 1, Headers: string(image_headers)}},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind: model.ContentAssetKindImage, Role: model.ContentAssetRolePrimary,
				AssetKey: model.BuildContentAlbumImageAssetKey(image_key, "original"), Relation: model.DownloadResourceAssetRelationSource,
				SubjectType: model.ContentAssetSubjectAlbumImage, SubjectKey: image_key, SubjectRelation: model.ContentAssetSubjectRelationRepresentation,
			}},
		})
	}
	if len(resources) == 0 {
		return nil, fmt.Errorf("x post %s has no downloadable content", result.ExternalID)
	}
	now := util.NowMillis()
	details, _ := h.ToContentDetails(result)
	// The preview is only valid when the primary detail is backed by a published
	// detail record, so derive it from the same list instead of a bare pointer.
	var content_detail any
	if len(details) > 0 {
		content_detail = details[0].Data
	}
	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId: &content_id, Name: task_name, UniqueID: result.ExternalID, PlatformId: PlatformID,
			Status: model.TaskStatusWaiting, SourceURL: content.SourceURL, CoverURL: content.CoverURL,
			ConfigJSON: config_text, MetadataJSON: content.Metadata,
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		},
		Resources: resources, ContentDetail: content_detail, ContentDetails: details, Account: account, Content: content,
	}, nil
}

func (h *handler) BuildBrowseHistory(_ json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	return nil, adapter.ErrBrowseHistoryNotSupported
}

// post_text is the post's own text: the long-form article body when the status
// carries one, otherwise the tweet text.
func post_text(result *x_scraper.FetchResult) string {
	if result == nil {
		return ""
	}
	if result.Article != nil {
		if text := strings.TrimSpace(result.Article.Text); text != "" {
			return text
		}
	}
	return strings.TrimSpace(result.BodyText)
}

func content_article(result *x_scraper.FetchResult) *model.ContentArticle {
	text := post_text(result)
	if text == "" {
		return nil
	}
	article := &model.ContentArticle{
		Id:        PlatformID + ":" + result.ExternalID,
		Type:      model.ContentArticleTypeText,
		WordCount: utf8.RuneCountInString(text),
		Text:      text,
	}
	// A long-form post keeps its headings, links, images and code blocks in
	// markdown, which the flattened text above cannot represent.
	if result.Article != nil {
		if markdown := strings.TrimSpace(result.Article.Markdown); markdown != "" {
			article.Type = model.ContentArticleTypeMarkdown
			article.Markdown = markdown
		}
	}
	return article
}

// content_videos turns every video attached to the post into its own record.
func content_videos(result *x_scraper.FetchResult) []*model.ContentVideo {
	if result == nil || len(result.Videos) == 0 {
		return nil
	}
	videos := make([]*model.ContentVideo, 0, len(result.Videos))
	for video_index := range result.Videos {
		videos = append(videos, content_video(result, video_index))
	}
	return videos
}

func content_video(result *x_scraper.FetchResult, video_index int) *model.ContentVideo {
	if result == nil || video_index < 0 || video_index >= len(result.Videos) {
		return nil
	}
	video := result.Videos[video_index]
	content_id := x_video_content_id(result.ExternalID, video_index)
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

// video_content is the child content record one attached video files under.
// Resources are listed per content id, so without a record of its own a video's
// download would never show up on the post's detail page.
func video_content(result *x_scraper.FetchResult, video_index int, content_video *model.ContentVideo) *model.Content {
	if result == nil || content_video == nil || video_index < 0 || video_index >= len(result.Videos) {
		return nil
	}
	video := result.Videos[video_index]
	title := post_title(result.BodyText, result.AuthorName)
	if len(result.Videos) > 1 {
		title = fmt.Sprintf("%s_%02d", title, video_index+1)
	}
	now := util.NowMillis()
	return &model.Content{
		Id:          content_video.Id,
		PlatformId:  PlatformID,
		Type:        model.ContentTypeVideo,
		Subtype:     model.ContentSubtypeShortVideo,
		ExternalId:  video.ID,
		ExternalId2: result.AuthorID,
		Title:       title,
		Description: strings.TrimSpace(result.BodyText),
		URL:         video.URL,
		SourceURL:   result.SourceURL,
		CoverURL:    first_non_empty(video.CoverURL, result.CoverURL),
		PublishTime: positive_int64_pointer(result.PublishTime),
		Timestamps:  model.Timestamps{CreatedAt: now, UpdatedAt: now},
	}
}

func video_resource(content_id string, task_name string, source_url string, video *x_scraper.Video) (*adapter.ResourceInfo, error) {
	if video == nil || strings.TrimSpace(video.URL) == "" {
		return nil, fmt.Errorf("x post video has no download URL")
	}
	headers, _ := json.Marshal(map[string]string{"Accept": "*/*", "Referer": source_url, "User-Agent": x_scraper.DefaultUserAgent})
	is_hls := strings.Contains(strings.ToLower(video.URL), ".m3u8")
	resource := model.DownloadResource{
		ContentId: &content_id, Name: task_name, Kind: "video/mp4", UniqueID: first_non_empty(video.ID, content_id), Duration: (video.DurationMillis + 999) / 1000,
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
	if strings.TrimSpace(result.BodyText) == "" && result.Article == nil && len(result.Videos) == 0 && len(result.Images) == 0 {
		return nil, fmt.Errorf("x post %s has no text, image, or video", result.ExternalID)
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

// post_cover_url prefers the video poster, then the long-form article's own
// cover, then the first attached photo, then the page's og:image.
func post_cover_url(result *x_scraper.FetchResult) string {
	if cover := first_video_cover(result); cover != "" {
		return cover
	}
	if result.Article != nil {
		if cover := strings.TrimSpace(result.Article.CoverURL); cover != "" {
			return cover
		}
	}
	if len(result.Images) > 0 {
		if cover := strings.TrimSpace(result.Images[0].URL); cover != "" {
			return cover
		}
	}
	return strings.TrimSpace(result.CoverURL)
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

// image_extension reads the photo's format, which X carries either in the path
// or in a format query parameter (pbs.twimg.com/media/<id>?format=jpg).
func image_extension(raw_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return ""
	}
	if extension := strings.TrimPrefix(strings.ToLower(path.Ext(parsed_url.Path)), "."); extension != "" {
		return extension
	}
	return strings.ToLower(strings.TrimSpace(parsed_url.Query().Get("format")))
}

func image_mime_type(extension string) string {
	switch strings.ToLower(strings.TrimPrefix(strings.TrimSpace(extension), ".")) {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	case "avif":
		return "image/avif"
	default:
		return "image"
	}
}
