package webpageadapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"unicode"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/scraper/webpage"
	"wx_channel/pkg/util"
)

// PlatformID is the generic web page platform identifier.
const PlatformID = webpage.PlatformID

func init() {
	adapter.Register(NewWebpageAdapter())
}

// WebpageAdapter connects the generic webpage scraper to the shared registry.
type WebpageAdapter struct {
	runtime_mu      sync.RWMutex
	cookie_provider *cookies.Reader
}

var (
	_ adapter.PlatformAdapter             = (*WebpageAdapter)(nil)
	_ adapter.ContextProgressFetchAdapter = (*WebpageAdapter)(nil)
	_ adapter.FetchDownloadTaskBuilder    = (*WebpageAdapter)(nil)
	_ adapter.RuntimeAdapter              = (*WebpageAdapter)(nil)
	_ adapter.RuntimeHandle               = (*WebpageAdapter)(nil)
	_ adapter.PlatformStatusDescriber     = (*WebpageAdapter)(nil)
	_ adapter.Postprocessor               = (*WebpageAdapter)(nil)
)

// NewWebpageAdapter creates the generic fallback adapter.
func NewWebpageAdapter() *WebpageAdapter {
	return &WebpageAdapter{}
}

func (a *WebpageAdapter) PlatformID() string { return PlatformID }

// PlatformStatuses describes the always-available generic page scraper.
func (a *WebpageAdapter) PlatformStatuses() []adapter.PlatformStatusDescriptor {
	return []adapter.PlatformStatusDescriptor{{
		Platform: PlatformID,
		Key:      PlatformID,
		Name:     "网页",
	}}
}

// RegisterRuntime attaches the persistent cookie provider.
func (a *WebpageAdapter) RegisterRuntime(adapter_options *adapter.AdapterOptions) (adapter.RuntimeHandle, error) {
	if a == nil {
		return nil, fmt.Errorf("webpage adapter is nil")
	}
	if adapter_options == nil {
		return nil, fmt.Errorf("webpage runtime dependencies are nil")
	}
	a.runtime_mu.Lock()
	a.cookie_provider = adapter_options.Cookies
	a.runtime_mu.Unlock()
	if adapter_options.Logger != nil {
		adapter_options.Logger.Info().
			Str("component", "webpage_adapter").
			Bool("cookie_provider_available", adapter_options.Cookies != nil).
			Msg("webpage adapter runtime registered")
	}
	if adapter_options.Bus != nil {
		adapter_options.Bus.Publish(events.PlatformStatusChanged{
			Platform:  PlatformID,
			Key:       PlatformID,
			Name:      "网页",
			Status:    "available",
			Available: true,
		})
	}
	return a, nil
}

// Stop releases runtime references owned by the adapter.
func (a *WebpageAdapter) Stop() {
	if a == nil {
		return
	}
	a.runtime_mu.Lock()
	a.cookie_provider = nil
	a.runtime_mu.Unlock()
}

func (a *WebpageAdapter) runtime_cookie_provider() *cookies.Reader {
	if a == nil {
		return nil
	}
	a.runtime_mu.RLock()
	cookie_provider := a.cookie_provider
	a.runtime_mu.RUnlock()
	return cookie_provider
}

// Fetch retrieves and extracts a generic web page.
func (a *WebpageAdapter) Fetch(raw_url string) (any, error) {
	return a.FetchWithProgressContext(context.Background(), raw_url, adapter.FetchOptions{})
}

// FetchWithProgressContext retrieves a page with cancellation support.
func (a *WebpageAdapter) FetchWithProgressContext(fetch_context context.Context, raw_url string, _ adapter.FetchOptions) (any, error) {
	client := webpage.NewClient(a.runtime_cookie_provider())
	defer client.Close()
	return client.FetchContext(fetch_context, raw_url)
}

// ToContent converts the extracted page into the common content model.
func (a *WebpageAdapter) ToContent(data any) (*model.Content, error) {
	page, err := webpage_result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return webpage_to_content(page)
}

// ToAccount represents the website itself as the publisher account.
func (a *WebpageAdapter) ToAccount(data any) (*model.Account, error) {
	page, err := webpage_result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return webpage_to_account(page)
}

// ToContentDetails returns the extracted HTML, text, and Markdown article body.
func (a *WebpageAdapter) ToContentDetails(data any) ([]adapter.ContentDetail, error) {
	page, err := webpage_result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	content, err := webpage_to_content(page)
	if err != nil {
		return nil, err
	}
	article := webpage_to_article(page, content.Id)
	return []adapter.ContentDetail{{
		Type:    model.ContentTypeWebpage,
		Key:     content.Id,
		Content: content,
		Data:    article,
	}}, nil
}

// BuildDownloadTask builds a Markdown archive task from an extracted page. A
// URL-only payload is fetched before the task is built.
func (a *WebpageAdapter) BuildDownloadTask(content_json json.RawMessage, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	page, err := webpage_result_from_json(content_json)
	if err != nil || (strings.TrimSpace(page.HTML) == "" && strings.TrimSpace(page.Markdown) == "" && strings.TrimSpace(page.Text) == "") {
		var input struct {
			URL          string `json:"url"`
			RequestedURL string `json:"requested_url"`
		}
		if decode_err := json.Unmarshal(content_json, &input); decode_err != nil {
			return nil, err
		}
		raw_url := first_non_empty(input.RequestedURL, input.URL)
		if raw_url == "" {
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("网页下载数据缺少 URL 或已提取正文")
		}
		fetched, fetch_err := a.Fetch(raw_url)
		if fetch_err != nil {
			return nil, fetch_err
		}
		page, err = webpage_result_from_fetch(fetched)
		if err != nil {
			return nil, err
		}
	}
	return build_download_task(page, config_json, a.runtime_cookie_provider())
}

// BuildDownloadTaskFromFetch builds a task without issuing another request.
func (a *WebpageAdapter) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	page, err := webpage_result_from_fetch(data)
	if err != nil {
		return nil, err
	}
	return build_download_task(page, config_json, a.runtime_cookie_provider())
}

// BuildBrowseHistory builds a generic page browse-history record.
func (a *WebpageAdapter) BuildBrowseHistory(content_json json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	page, err := webpage_result_from_json(content_json)
	if err != nil {
		return nil, err
	}
	content, err := webpage_to_content(page)
	if err != nil {
		return nil, err
	}
	account, err := webpage_to_account(page)
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

func webpage_to_content(page *webpage.Page) (*model.Content, error) {
	page, err := validate_webpage_result(page)
	if err != nil {
		return nil, err
	}
	archive_url := strings.TrimSpace(page.ArchiveURL())
	external_id := webpage_external_id(archive_url)
	metadata_data, _ := json.Marshal(map[string]any{
		"requested_url": page.RequestedURL,
		"final_url":     page.FinalURL,
		"canonical_url": page.CanonicalURL,
		"status_code":   page.StatusCode,
		"content_type":  page.ContentType,
		"author":        page.Author,
		"site_name":     page.SiteName,
		"language":      page.Language,
		"favicon_url":   page.FaviconURL,
		"publish_time":  page.PublishTime,
	})
	now := util.NowMillis()
	return &model.Content{
		Id:          PlatformID + ":" + external_id,
		PlatformId:  PlatformID,
		Type:        model.ContentTypeWebpage,
		ExternalId:  external_id,
		Title:       first_non_empty(page.Title, hostname_from_url(archive_url), "网页"),
		Description: strings.TrimSpace(page.Description),
		URL:         archive_url,
		SourceURL:   first_non_empty(page.RequestedURL, archive_url),
		CoverURL:    strings.TrimSpace(page.ImageURL),
		PublishTime: page.PublishTime,
		Metadata:    string(metadata_data),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

func webpage_to_account(page *webpage.Page) (*model.Account, error) {
	page, err := validate_webpage_result(page)
	if err != nil {
		return nil, err
	}
	site_url, err := webpage_site_url(page)
	if err != nil {
		return nil, err
	}
	domain := strings.ToLower(strings.TrimSuffix(site_url.Hostname(), "."))
	if domain == "" {
		return nil, fmt.Errorf("网页账号缺少网站域名")
	}
	origin_url := (&url.URL{Scheme: site_url.Scheme, Host: site_url.Host, Path: "/"}).String()
	favicon_url := strings.TrimSpace(page.FaviconURL)
	if favicon_url == "" {
		favicon_url = (&url.URL{Scheme: site_url.Scheme, Host: site_url.Host, Path: "/favicon.ico"}).String()
	}
	now := util.NowMillis()
	return &model.Account{
		Id:         PlatformID + ":" + domain,
		PlatformId: PlatformID,
		ExternalId: domain,
		Nickname:   domain,
		AvatarURL:  favicon_url,
		ProfileURL: origin_url,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

func webpage_site_url(page *webpage.Page) (*url.URL, error) {
	if page == nil {
		return nil, fmt.Errorf("网页抓取结果为空")
	}
	raw_url := first_non_empty(page.FinalURL, page.URL, page.CanonicalURL, page.RequestedURL)
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || parsed_url.Hostname() == "" {
		return nil, fmt.Errorf("无法解析网页站点 URL: %s", raw_url)
	}
	parsed_url.Scheme = strings.ToLower(parsed_url.Scheme)
	if parsed_url.Scheme != "http" && parsed_url.Scheme != "https" {
		return nil, fmt.Errorf("网页站点 URL 仅支持 HTTP/HTTPS: %s", raw_url)
	}
	return parsed_url, nil
}

func webpage_to_article(page *webpage.Page, content_id string) *model.ContentArticle {
	word_count := webpage_word_count(page.Text)
	reading_time := 0
	if word_count > 0 {
		reading_time = (word_count + 299) / 300
	}
	return &model.ContentArticle{
		Id:              strings.TrimSpace(content_id),
		Type:            model.ContentArticleTypeHTML,
		WordCount:       word_count,
		ReadingTime:     reading_time,
		Text:            strings.TrimSpace(page.Text),
		HTML:            strings.TrimSpace(page.HTML),
		Markdown:        strings.TrimSpace(page.Markdown),
		PublishPlatform: first_non_empty(page.SiteName, hostname_from_url(page.ArchiveURL())),
	}
}

func build_download_task(page *webpage.Page, config_json json.RawMessage, cookie_provider *cookies.Reader) (*adapter.DownloadTaskResult, error) {
	page, err := validate_webpage_result(page)
	if err != nil {
		return nil, err
	}
	content, err := webpage_to_content(page)
	if err != nil {
		return nil, err
	}
	account, err := webpage_to_account(page)
	if err != nil {
		return nil, err
	}
	article := webpage_to_article(page, content.Id)
	config := make(map[string]any)
	config_text := strings.TrimSpace(string(config_json))
	if config_text != "" && config_text != "null" {
		if err := json.Unmarshal(config_json, &config); err != nil {
			return nil, fmt.Errorf("解析网页下载配置失败: %w", err)
		}
	}
	if config == nil {
		config = make(map[string]any)
	}
	task_name := strings.TrimSpace(config_string(config, "filename"))
	if task_name == "" {
		task_name = content.Title
	}
	body, archive_images, err := build_archive_markdown(page)
	if err != nil {
		return nil, err
	}
	body = strings.TrimSpace(body)
	kind := "text/markdown"
	if body == "" {
		body = strings.TrimSpace(page.Text)
		kind = "text/plain"
	}
	if body == "" {
		body = strings.TrimSpace(page.HTML)
		kind = "text/html"
	}
	if body == "" {
		body = fmt.Sprintf("# %s\n\n%s", content.Title, content.URL)
		kind = "text/markdown"
	}
	body += "\n"
	config_data, _ := json.Marshal(config)
	metadata_data, _ := json.Marshal(map[string]any{
		"platform":     PlatformID,
		"external_id":  content.ExternalId,
		"source_url":   content.SourceURL,
		"archive_url":  content.URL,
		"title":        content.Title,
		"site_name":    page.SiteName,
		"author":       page.Author,
		"publish_time": page.PublishTime,
	})
	markdown_extra, _ := json.Marshal(map[string]string{
		webpage_postprocess_marker_key: webpage_postprocess_marker_value,
	})
	resources := []*adapter.ResourceInfo{{
		Resource: model.DownloadResource{
			ContentId: &content.Id,
			Name:      task_name,
			Kind:      kind,
			UniqueID:  content.ExternalId + "_content",
			Size:      int64(len(body)),
			Extra:     string(markdown_extra),
		},
		Endpoints: []model.DownloadEndpoint{{
			Protocol: "inline",
			URL:      body,
			Enabled:  1,
		}},
		ContentAssets: []adapter.ContentAssetReference{{
			Kind:     model.ContentAssetKindText,
			Role:     model.ContentAssetRoleArticleBody,
			AssetKey: "body",
			Relation: model.DownloadResourceAssetRelationSource,
		}},
	}}
	image_resources, err := build_archive_image_resources(page, content, archive_images, cookie_provider)
	if err != nil {
		return nil, err
	}
	resources = append(resources, image_resources...)

	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content.Id,
			Name:         task_name,
			UniqueID:     content.ExternalId + "_webpage",
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			SourceURL:    content.SourceURL,
			CoverURL:     content.CoverURL,
			ConfigJSON:   string(config_data),
			MetadataJSON: string(metadata_data),
		},
		Resources:     resources,
		Content:       content,
		ContentDetail: article,
		Account:       account,
	}, nil
}

func webpage_result_from_fetch(data any) (*webpage.Page, error) {
	switch value := data.(type) {
	case *webpage.Page:
		return validate_webpage_result(value)
	case webpage.Page:
		return validate_webpage_result(&value)
	case json.RawMessage:
		return webpage_result_from_json(value)
	case []byte:
		return webpage_result_from_json(value)
	case string:
		return webpage_result_from_json([]byte(value))
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("编码网页抓取数据失败: %w", err)
	}
	return webpage_result_from_json(encoded)
}

func webpage_result_from_json(content_json []byte) (*webpage.Page, error) {
	if len(strings.TrimSpace(string(content_json))) == 0 {
		return nil, fmt.Errorf("网页抓取数据为空")
	}
	var page webpage.Page
	if err := json.Unmarshal(content_json, &page); err != nil {
		return nil, fmt.Errorf("解析网页抓取数据失败: %w", err)
	}
	return validate_webpage_result(&page)
}

func validate_webpage_result(page *webpage.Page) (*webpage.Page, error) {
	if page == nil {
		return nil, fmt.Errorf("网页抓取数据为空")
	}
	if strings.TrimSpace(page.ArchiveURL()) == "" {
		return nil, fmt.Errorf("网页抓取数据缺少 URL")
	}
	return page, nil
}

func webpage_external_id(raw_url string) string {
	normalized_url := strings.TrimSpace(raw_url)
	if parsed_url, err := url.Parse(normalized_url); err == nil {
		parsed_url.Fragment = ""
		parsed_url.Scheme = strings.ToLower(parsed_url.Scheme)
		parsed_url.Host = strings.ToLower(parsed_url.Host)
		normalized_url = parsed_url.String()
	}
	hash_value := sha256.Sum256([]byte(normalized_url))
	return hex.EncodeToString(hash_value[:])
}

func webpage_word_count(value string) int {
	count := 0
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsNumber(character) {
			count++
		}
	}
	return count
}

func hostname_from_url(raw_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed_url.Hostname())
}

func config_string(config map[string]any, key string) string {
	value, ok := config[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
