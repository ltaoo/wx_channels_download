package weiboadapter

import (
	"context"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/scraper/weibo"
	"wx_channel/pkg/util"
)

// PlatformID is the platform identifier for Weibo.
const PlatformID = "weibo"

// Image is one image attached to a Weibo post.
type Image struct {
	URL string `json:"url"`
	Ext string `json:"ext"`
}

// Video is the selected progressive video rendered in a Weibo post.
type Video struct {
	URL       string `json:"url"`
	CoverURL  string `json:"cover_url"`
	MediaID   string `json:"media_id"`
	Quality   string `json:"quality"`
	Template  string `json:"template"`
	Duration  int64  `json:"duration"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	FPS       int    `json:"fps"`
	ExpiresAt *int64 `json:"expires_at,omitempty"`
}

// FetchResult is the rendered Weibo post and the fields extracted from it.
type FetchResult struct {
	SourceURL    string  `json:"source_url"`
	HTML         string  `json:"html"`
	ExternalID   string  `json:"external_id"`
	AuthorID     string  `json:"author_id"`
	AuthorName   string  `json:"author_name"`
	AuthorAvatar string  `json:"author_avatar"`
	BodyText     string  `json:"body_text"`
	BodyHTML     string  `json:"body_html"`
	Region       string  `json:"region"`
	Client       string  `json:"client"`
	PublishTime  *int64  `json:"publish_time"`
	ShareCount   int64   `json:"share_count"`
	CommentCount int64   `json:"comment_count"`
	LikeCount    int64   `json:"like_count"`
	Images       []Image `json:"images"`
	Video        *Video  `json:"video,omitempty"`
}

type handler struct {
	runtime_mu      sync.RWMutex
	cookie_provider *cookies.Reader
}

var (
	_ adapter.PlatformAdapter             = (*handler)(nil)
	_ adapter.ContextProgressFetchAdapter = (*handler)(nil)
	_ adapter.FetchDownloadTaskBuilder    = (*handler)(nil)
	_ adapter.Postprocessor               = (*handler)(nil)
	_ adapter.RuntimeAdapter              = (*handler)(nil)
	_ adapter.RuntimeHandle               = (*handler)(nil)
	_ adapter.PlatformStatusDescriber     = (*handler)(nil)
)

func init() {
	adapter.Register(&handler{})
}

func (h *handler) PlatformID() string { return PlatformID }

func (h *handler) PlatformStatuses() []adapter.PlatformStatusDescriptor {
	return []adapter.PlatformStatusDescriptor{{Platform: PlatformID, Key: PlatformID, Name: "微博"}}
}

func (h *handler) RegisterRuntime(adapter_options *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if h == nil || adapter_options == nil {
		return nil, fmt.Errorf("weibo runtime dependencies are nil")
	}
	h.runtime_mu.Lock()
	h.cookie_provider = adapter_options.Cookies
	h.runtime_mu.Unlock()
	new_routes(weibo.NewClient(adapter_options.Cookies)).register_routes(adapter_options.Routes)
	if adapter_options.Bus != nil {
		adapter_options.Bus.Publish(events.PlatformStatusChanged{
			Platform:  PlatformID,
			Key:       PlatformID,
			Name:      "微博",
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
	h.cookie_provider = nil
	h.runtime_mu.Unlock()
}

func (h *handler) cookie_reader() *cookies.Reader {
	if h == nil {
		return nil
	}
	h.runtime_mu.RLock()
	defer h.runtime_mu.RUnlock()
	return h.cookie_provider
}

func (h *handler) Fetch(raw_url string) (any, error) {
	return h.FetchWithProgressContext(context.Background(), raw_url, adapter.FetchOptions{})
}

func (h *handler) FetchWithProgressContext(fetch_context context.Context, raw_url string, _ adapter.FetchOptions) (any, error) {
	html_text, err := weibo.NewClient(h.cookie_reader()).FetchContext(fetch_context, raw_url)
	if err != nil {
		return nil, err
	}
	return parse_fetch_result(raw_url, html_text)
}

func (h *handler) ToContent(data any) (*model.Content, error) {
	result, err := fetch_result_from_data(data)
	if err != nil {
		return nil, err
	}
	metadata, err := json.Marshal(map[string]any{
		"author_id": result.AuthorID,
		"body_html": result.BodyHTML,
		"region":    result.Region,
		"client":    result.Client,
		"images":    result.Images,
		"video":     result.Video,
	})
	if err != nil {
		return nil, fmt.Errorf("encode weibo metadata: %w", err)
	}
	now := util.NowMillis()
	subtype := "status"
	if result.Video != nil {
		subtype = model.ContentTypeVideo
	}
	return &model.Content{
		Id:           PlatformID + ":" + result.ExternalID,
		PlatformId:   PlatformID,
		Type:         model.ContentTypePost,
		Subtype:      subtype,
		ExternalId:   result.ExternalID,
		ExternalId2:  result.AuthorID,
		Title:        post_title(result.BodyText, result.AuthorName),
		Description:  result.BodyText,
		URL:          result.SourceURL,
		SourceURL:    result.SourceURL,
		CoverURL:     fetch_result_cover_url(result),
		PublishTime:  result.PublishTime,
		LikeCount:    result.LikeCount,
		CommentCount: result.CommentCount,
		ShareCount:   result.ShareCount,
		Metadata:     string(metadata),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

func (h *handler) ToAccount(data any) (*model.Account, error) {
	result, err := fetch_result_from_data(data)
	if err != nil {
		return nil, err
	}
	now := util.NowMillis()
	return &model.Account{
		Id:         PlatformID + ":" + result.AuthorID,
		PlatformId: PlatformID,
		ExternalId: result.AuthorID,
		Nickname:   result.AuthorName,
		AvatarURL:  result.AuthorAvatar,
		ProfileURL: "https://weibo.com/u/" + result.AuthorID,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

func (h *handler) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	result, err := fetch_result_from_data(data)
	if err != nil {
		return nil, err
	}
	content_video := content_video_from_result(result)
	if content_video == nil {
		return nil, nil
	}
	return []adapter.ContentDetail{{Type: model.ContentTypeVideo, Key: content_video.Id, Data: content_video}}, nil
}

func (h *handler) BuildDownloadTask(content_json json.RawMessage, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	var result FetchResult
	if err := json.Unmarshal(content_json, &result); err == nil && result.ExternalID != "" {
		return h.build_download_task(&result, config_json)
	}
	var input struct {
		URL       string `json:"url"`
		SourceURL string `json:"source_url"`
	}
	if err := json.Unmarshal(content_json, &input); err != nil {
		return nil, fmt.Errorf("decode weibo download data: %w", err)
	}
	raw_url := strings.TrimSpace(input.URL)
	if raw_url == "" {
		raw_url = strings.TrimSpace(input.SourceURL)
	}
	if raw_url == "" {
		return nil, fmt.Errorf("weibo download data is missing URL")
	}
	fetched, err := h.Fetch(raw_url)
	if err != nil {
		return nil, err
	}
	return h.BuildDownloadTaskFromFetch(fetched, config_json)
}

func (h *handler) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	result, err := fetch_result_from_data(data)
	if err != nil {
		return nil, err
	}
	return h.build_download_task(result, config_json)
}

func (h *handler) build_download_task(result *FetchResult, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
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
		return nil, fmt.Errorf("decode weibo download config: %w", err)
	}
	task_name, _ := config["filename"].(string)
	task_name = strings.TrimSpace(task_name)
	if task_name == "" {
		task_name = content.Title
	}
	html_text := render_detail_html(result)
	content_video := content_video_from_result(result)
	content_details := content_video_details(content_video)
	resources := make([]*adapter.ResourceInfo, 0, len(result.Images)+3)
	resources = append(resources, &adapter.ResourceInfo{
		Resource: model.DownloadResource{
			ContentId: &content.Id,
			Name:      task_name,
			Kind:      "text/html",
			UniqueID:  result.ExternalID + "_html",
			Size:      int64(len(html_text)),
		},
		Endpoints: []model.DownloadEndpoint{{Protocol: "inline", URL: html_text, Enabled: 1}},
		ContentAssets: []adapter.ContentAssetReference{{
			Kind:     model.ContentAssetKindText,
			Role:     model.ContentAssetRoleArticleBody,
			AssetKey: "body:html",
			Relation: model.DownloadResourceAssetRelationSource,
		}},
	})
	image_headers, _ := json.Marshal(map[string]string{
		"Accept":  "image/avif,image/webp,image/apng,image/*,*/*;q=0.8",
		"Referer": "https://weibo.com/",
	})
	if result.Video != nil && result.Video.CoverURL != "" && !image_list_contains(result.Images, result.Video.CoverURL) {
		resources = append(resources, &adapter.ResourceInfo{
			Resource: model.DownloadResource{
				ContentId: &content.Id,
				Name:      task_name + "_cover",
				Kind:      image_mime_type(image_extension(result.Video.CoverURL)),
				UniqueID:  result.ExternalID + "_video_cover",
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: endpoint_protocol(result.Video.CoverURL),
				URL:      result.Video.CoverURL,
				Enabled:  1,
				Headers:  string(image_headers),
			}},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind:     model.ContentAssetKindImage,
				Role:     model.ContentAssetRoleCover,
				AssetKey: "video:cover",
				Relation: model.DownloadResourceAssetRelationSource,
			}},
		})
	}
	for image_index, image := range result.Images {
		image_name := task_name
		if len(result.Images) > 1 {
			image_name += fmt.Sprintf("_%02d", image_index+1)
		}
		resources = append(resources, &adapter.ResourceInfo{
			Resource: model.DownloadResource{
				ContentId: &content.Id,
				Name:      image_name,
				Kind:      image_mime_type(image.Ext),
				UniqueID:  fmt.Sprintf("%s_image_%d", result.ExternalID, image_index+1),
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: endpoint_protocol(image.URL),
				URL:      image.URL,
				Enabled:  1,
				Headers:  string(image_headers),
			}},
			ContentAssets: []adapter.ContentAssetReference{{
				Kind:     model.ContentAssetKindImage,
				Role:     model.ContentAssetRoleAttachment,
				AssetKey: fmt.Sprintf("image:%d:%s", image_index+1, image.Ext),
				Relation: model.DownloadResourceAssetRelationSource,
			}},
		})
	}
	if result.Video != nil {
		video_headers, _ := json.Marshal(map[string]string{
			"Accept":  "*/*",
			"Referer": "https://weibo.com/",
		})
		resources = append(resources, &adapter.ResourceInfo{
			Resource: model.DownloadResource{
				ContentId: &content.Id,
				Name:      task_name,
				Kind:      "video/mp4",
				UniqueID:  result.ExternalID + "_video",
				Duration:  result.Video.Duration,
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: endpoint_protocol(result.Video.URL),
				URL:      result.Video.URL,
				Enabled:  1,
				Headers:  string(video_headers),
			}},
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
			Name:         task_name,
			UniqueID:     result.ExternalID,
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			SourceURL:    content.SourceURL,
			CoverURL:     content.CoverURL,
			ConfigJSON:   config_text,
			MetadataJSON: content.Metadata,
		},
		Resources:      resources,
		ContentDetail:  content_video,
		ContentDetails: content_details,
		Account:        account,
		Content:        content,
	}, nil
}

func (h *handler) BuildBrowseHistory(content_json json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	var result FetchResult
	if err := json.Unmarshal(content_json, &result); err != nil {
		return nil, fmt.Errorf("decode weibo browse history: %w", err)
	}
	content, err := h.ToContent(&result)
	if err != nil {
		return nil, err
	}
	account, err := h.ToAccount(&result)
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
			PublishTime:  content.PublishTime,
			ExtraData:    content.Metadata,
			Timestamps:   content.Timestamps,
		},
		Account: account,
	}, nil
}

func parse_fetch_result(raw_url, html_text string) (*FetchResult, error) {
	source_url, author_id, external_id, err := parse_detail_url(raw_url)
	if err != nil {
		return nil, err
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(html_text))
	if err != nil {
		return nil, fmt.Errorf("parse weibo HTML: %w", err)
	}
	article := document.Find("article").FilterFunction(func(_ int, selection *goquery.Selection) bool {
		return selection.Find(".wbpro-feed-ogText").Length() > 0
	}).First()
	if article.Length() == 0 {
		return nil, fmt.Errorf("weibo detail %s is missing post content", external_id)
	}
	body := article.Find(".wbpro-feed-ogText").First()
	body_html, _ := body.Html()
	body_text := selection_text(body)
	author_name := strings.TrimSpace(article.Find("header a[usercard] span[title]").First().AttrOr("title", ""))
	if author_name == "" {
		author_name = "微博用户"
	}
	author_avatar := strings.TrimSpace(article.Find("header img[usercard]").First().AttrOr("src", ""))
	result := &FetchResult{
		SourceURL:    source_url,
		HTML:         html_text,
		ExternalID:   external_id,
		AuthorID:     author_id,
		AuthorName:   author_name,
		AuthorAvatar: author_avatar,
		BodyText:     body_text,
		BodyHTML:     strings.TrimSpace(body_html),
		Region:       strings.TrimSpace(strings.TrimPrefix(article.Find(`[title^="发布于 "]`).First().AttrOr("title", ""), "发布于 ")),
		Client:       strings.TrimSpace(strings.TrimPrefix(article.Find(`[title^="来自 "]`).First().AttrOr("title", ""), "来自 ")),
	}
	article.Find("header a[href]").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		if strings.TrimSpace(selection.AttrOr("href", "")) != source_url {
			return true
		}
		result.PublishTime = parse_publish_time(selection.Text())
		return false
	})
	if counts := strings.Split(article.Find("footer[aria-label]").First().AttrOr("aria-label", ""), ","); len(counts) == 3 {
		result.ShareCount = parse_count(counts[0])
		result.CommentCount = parse_count(counts[1])
		result.LikeCount = parse_count(counts[2])
	}
	result.Video = parse_video(article)
	seen_images := make(map[string]struct{})
	article.Find(".picture img[src]").Each(func(_ int, selection *goquery.Selection) {
		image_url := strings.TrimSpace(selection.AttrOr("src", ""))
		if image_url == "" {
			return
		}
		if _, exists := seen_images[image_url]; exists {
			return
		}
		seen_images[image_url] = struct{}{}
		result.Images = append(result.Images, Image{URL: image_url, Ext: image_extension(image_url)})
	})
	if result.BodyText == "" && len(result.Images) == 0 && result.Video == nil {
		return nil, fmt.Errorf("weibo detail %s has no body or images", external_id)
	}
	return result, nil
}

func parse_detail_url(raw_url string) (string, string, string, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || parsed_url == nil || !strings.EqualFold(parsed_url.Scheme, "https") || !strings.EqualFold(parsed_url.Hostname(), "weibo.com") {
		return "", "", "", fmt.Errorf("unsupported weibo detail URL %q", raw_url)
	}
	path_parts := strings.Split(strings.Trim(parsed_url.EscapedPath(), "/"), "/")
	if len(path_parts) != 2 || !decimal_string(path_parts[0]) || !base62_string(path_parts[1]) {
		return "", "", "", fmt.Errorf("unsupported weibo detail URL %q", raw_url)
	}
	parsed_url.RawQuery = ""
	parsed_url.Fragment = ""
	return parsed_url.String(), path_parts[0], path_parts[1], nil
}

func fetch_result_from_data(data any) (*FetchResult, error) {
	switch result := data.(type) {
	case *FetchResult:
		if result != nil && result.ExternalID != "" && result.AuthorID != "" {
			return result, nil
		}
	case FetchResult:
		if result.ExternalID != "" && result.AuthorID != "" {
			return &result, nil
		}
	case json.RawMessage:
		var decoded_result FetchResult
		if err := json.Unmarshal(result, &decoded_result); err != nil {
			return nil, fmt.Errorf("decode weibo fetch data: %w", err)
		}
		return fetch_result_from_data(&decoded_result)
	}
	return nil, fmt.Errorf("unsupported weibo fetch data type %T", data)
}

func selection_text(selection *goquery.Selection) string {
	clone := selection.Clone()
	clone.Find("img[alt]").Each(func(_ int, image *goquery.Selection) {
		image.ReplaceWithHtml(stdhtml.EscapeString(image.AttrOr("alt", "")))
	})
	return strings.Join(strings.Fields(clone.Text()), " ")
}

func parse_publish_time(value string) *int64 {
	china_time := time.FixedZone("Asia/Shanghai", 8*60*60)
	for _, layout := range []string{"06-1-2 15:04", "2006-1-2 15:04"} {
		parsed_time, err := time.ParseInLocation(layout, strings.TrimSpace(value), china_time)
		if err == nil {
			milliseconds := parsed_time.UnixMilli()
			return &milliseconds
		}
	}
	return nil
}

func parse_count(value string) int64 {
	count, _ := strconv.ParseInt(strings.ReplaceAll(strings.TrimSpace(value), ",", ""), 10, 64)
	return count
}

func post_title(body_text, author_name string) string {
	title_runes := []rune(strings.TrimSpace(body_text))
	if len(title_runes) > 80 {
		title_runes = append(title_runes[:80], '…')
	}
	if len(title_runes) > 0 {
		return string(title_runes)
	}
	return strings.TrimSpace(author_name) + "的微博"
}

func first_image_url(images []Image) string {
	if len(images) == 0 {
		return ""
	}
	return images[0].URL
}

func image_extension(raw_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(path.Ext(parsed_url.Path)), ".")
}

func endpoint_protocol(raw_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err == nil && parsed_url.Scheme != "" {
		return strings.ToLower(parsed_url.Scheme)
	}
	return "https"
}

func decimal_string(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func base62_string(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z')) {
			return false
		}
	}
	return true
}
