package zhihuadapter

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/cache"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/scraper/zhihu"
	"wx_channel/pkg/util"
)

const platform_id_zhihu = "zhihu"

func init() {
	adapter.Register(&handler{})
}

type handler struct {
	runtime_mu          sync.RWMutex
	cookie_reader       *cookies.Reader
	logger              *zerolog.Logger
	file_cache          *cache.CacheProvider
	browser_fetcher     zhihu.BrowserFetcher
	status_mu           sync.Mutex
	status_bus          *events.Bus
	cancel_status_check func()
}

var (
	_ adapter.PlatformAdapter          = (*handler)(nil)
	_ adapter.FetchCacheAdapter        = (*handler)(nil)
	_ adapter.FetchDownloadTaskBuilder = (*handler)(nil)
)

func (h *handler) PlatformID() string { return PlatformID }

// PlatformID is the exportable platform identifier for zhihu.
const PlatformID = platform_id_zhihu

func (h *handler) Fetch(raw_url string) (any, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return nil, fmt.Errorf("知乎URL不能为空")
	}

	return h.scraper_client().Fetch(raw_url)
}

func (h *handler) set_runtime(cookie_reader *cookies.Reader, logger *zerolog.Logger, bus *events.Bus) {
	h.runtime_mu.Lock()
	h.cookie_reader = cookie_reader
	h.logger = logger
	h.runtime_mu.Unlock()

	h.status_mu.Lock()
	h.status_bus = bus
	h.status_mu.Unlock()
}

func (h *handler) set_persistent_cache(file_cache *cache.CacheProvider) {
	h.runtime_mu.Lock()
	h.file_cache = file_cache
	h.runtime_mu.Unlock()
}

func (h *handler) set_browser_fetcher(browser_fetcher zhihu.BrowserFetcher) {
	h.runtime_mu.Lock()
	h.browser_fetcher = browser_fetcher
	h.runtime_mu.Unlock()
}

func (h *handler) runtime_browser_fetcher() zhihu.BrowserFetcher {
	h.runtime_mu.RLock()
	browser_fetcher := h.browser_fetcher
	h.runtime_mu.RUnlock()
	return browser_fetcher
}

func (h *handler) scraper_client() *zhihu.Client {
	h.runtime_mu.RLock()
	cookie_reader := h.cookie_reader
	logger := h.logger
	file_cache := h.file_cache
	browser_fetcher := h.browser_fetcher
	h.runtime_mu.RUnlock()
	client := zhihu.NewClient(cookie_reader, logger)
	client.SetPersistentCache(file_cache)
	client.SetBrowserFetcher(browser_fetcher)
	return client
}

// BuildContentID builds a content identifier from an external ID.
func BuildContentID(external_id string) string {
	return PlatformID + ":" + external_id
}

// BuildAccountID builds an account identifier from an external ID.
func BuildAccountID(external_id string) string {
	return PlatformID + ":" + external_id
}

// ContentExternalID builds a unique external identifier for zhihu content.
func ContentExternalID(content_type, token, url string) string {
	if token != "" {
		return "zhihu:" + content_type + ":" + token
	}
	return url
}

// ToContent converts zhihu answer page data into a model.Content.
func ToContent(page *zhihu.AnswerPage) (*model.Content, error) {
	if page == nil || page.Answer.ID == "" {
		return nil, fmt.Errorf("zhihu answer page is empty")
	}

	external_id := page.Answer.ID
	now := util.NowMillis()

	content_url := page.Source
	cover_url := zhihu.FirstImageURL(page.Answer.Content, content_url)

	return &model.Content{
		Id:           BuildContentID(external_id),
		PlatformId:   PlatformID,
		Type:         "answer",
		ExternalId:   external_id,
		ExternalId2:  page.Question.ID,
		Title:        page.Question.Title,
		Description:  page.Answer.Excerpt,
		URL:          content_url,
		SourceURL:    content_url,
		CoverURL:     cover_url,
		PublishTime:  int64_ptr(page.Answer.CreatedTime * 1000),
		UpdateTime:   int64_ptr(page.Answer.UpdatedTime * 1000),
		LikeCount:    int64(page.Answer.VoteupCount),
		CommentCount: int64(page.Answer.CommentCount),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// QuestionToContent converts zhihu question page data into a model.Content.
func QuestionToContent(page *zhihu.QuestionPage) (*model.Content, error) {
	if page == nil || page.Question.ID == "" {
		return nil, fmt.Errorf("zhihu question page is empty")
	}

	external_id := page.Question.ID
	now := util.NowMillis()

	content_url := page.Source
	cover_url := zhihu.FirstImageURL(page.Question.Excerpt, content_url)

	return &model.Content{
		Id:           BuildContentID(external_id),
		PlatformId:   PlatformID,
		Type:         "question",
		ExternalId:   external_id,
		Title:        page.Question.Title,
		Description:  page.Question.Excerpt,
		URL:          content_url,
		SourceURL:    content_url,
		CoverURL:     cover_url,
		PublishTime:  int64_ptr(page.Question.Created * 1000),
		UpdateTime:   int64_ptr(page.Question.UpdatedTime * 1000),
		LikeCount:    int64(page.Question.VoteupCount),
		CommentCount: int64(page.Question.CommentCount),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ArticleToContent converts zhihu article page data into a model.Content.
func ArticleToContent(page *zhihu.ArticlePage) (*model.Content, error) {
	if page == nil || page.Article.ID == "" {
		return nil, fmt.Errorf("zhihu article page is empty")
	}

	external_id := page.Article.ID
	now := util.NowMillis()

	content_url := page.Source
	cover_url := first_non_empty_str(page.Article.ImageURL, page.Article.ImageURLAlt)

	return &model.Content{
		Id:          BuildContentID(external_id),
		PlatformId:  PlatformID,
		Type:        "article",
		ExternalId:  external_id,
		Title:       page.Article.Title,
		Description: page.Article.Excerpt,
		URL:         content_url,
		SourceURL:   content_url,
		CoverURL:    cover_url,
		PublishTime: int64_ptr(page.Article.CreatedTime * 1000),
		UpdateTime:  int64_ptr(page.Article.UpdatedTime * 1000),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ToAccount converts a zhihu user into a model.Account.
func ToAccount(user *zhihu.User) (*model.Account, error) {
	if user == nil {
		return nil, fmt.Errorf("zhihu user is empty")
	}

	external_id := zhihu.UserDisplayName(*user)
	if external_id == "" {
		return nil, fmt.Errorf("zhihu user has no identifiable name")
	}

	now := util.NowMillis()
	profile_url := zhihu.UserURL(*user)

	return &model.Account{
		Id:         BuildAccountID(external_id),
		PlatformId: PlatformID,
		ExternalId: user.ID,
		Nickname:   zhihu.UserDisplayName(*user),
		Signature:  user.Headline,
		AvatarURL:  zhihu.UserAvatarURL(*user),
		ProfileURL: profile_url,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// AnswerToBrowseHistory converts a zhihu answer page into a model.BrowseHistory.
func AnswerToBrowseHistory(page *zhihu.AnswerPage) (*model.BrowseHistory, error) {
	if page == nil || page.Answer.ID == "" {
		return nil, fmt.Errorf("zhihu answer page is empty")
	}

	now := util.NowMillis()

	return &model.BrowseHistory{
		PlatformId:   PlatformID,
		VisitedTimes: 1,
		Type:         "answer",
		ExternalId:   page.Answer.ID,
		Title:        page.Question.Title,
		URL:          page.Source,
		SourceURL:    page.Source,
		CoverURL:     zhihu.FirstImageURL(page.Answer.Content, page.Source),
		PublishTime:  int64_ptr(page.Answer.CreatedTime * 1000),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// QuestionToBrowseHistory converts a zhihu question page into a model.BrowseHistory.
func QuestionToBrowseHistory(page *zhihu.QuestionPage) (*model.BrowseHistory, error) {
	if page == nil || page.Question.ID == "" {
		return nil, fmt.Errorf("zhihu question page is empty")
	}

	now := util.NowMillis()

	return &model.BrowseHistory{
		PlatformId:   PlatformID,
		VisitedTimes: 1,
		Type:         "question",
		ExternalId:   page.Question.ID,
		Title:        page.Question.Title,
		URL:          page.Source,
		SourceURL:    page.Source,
		CoverURL:     zhihu.FirstImageURL(page.Question.Excerpt, page.Source),
		PublishTime:  int64_ptr(page.Question.Created * 1000),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ArticleToBrowseHistory converts a zhihu article page into a model.BrowseHistory.
func ArticleToBrowseHistory(page *zhihu.ArticlePage) (*model.BrowseHistory, error) {
	if page == nil || page.Article.ID == "" {
		return nil, fmt.Errorf("zhihu article page is empty")
	}

	now := util.NowMillis()

	return &model.BrowseHistory{
		PlatformId:   PlatformID,
		VisitedTimes: 1,
		Type:         "article",
		ExternalId:   page.Article.ID,
		Title:        page.Article.Title,
		URL:          page.Source,
		SourceURL:    page.Source,
		CoverURL:     first_non_empty_str(page.Article.ImageURL, page.Article.ImageURLAlt),
		PublishTime:  int64_ptr(page.Article.CreatedTime * 1000),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// BuildBrowseHistory converts an intercepted Zhihu recommendation feed item
// into the standard browse history result.
func (h *handler) BuildBrowseHistory(content_json json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	var feed zhihu.RecommendFeed
	if err := json.Unmarshal(content_json, &feed); err != nil {
		return nil, fmt.Errorf("解析知乎推荐内容失败: %w", err)
	}

	target := &feed.Target
	external_id := strings.TrimSpace(target.ID)
	if external_id == "" {
		return nil, fmt.Errorf("知乎推荐内容ID不能为空")
	}

	content_type := strings.TrimSpace(target.Type)
	if content_type == "" && target.ArticleType != nil {
		content_type = strings.TrimSpace(*target.ArticleType)
	}
	if content_type == "" && target.Question != nil {
		content_type = "answer"
	}
	if content_type == "" {
		content_type = "other"
	}

	title := ""
	if target.Title != nil {
		title = strings.TrimSpace(*target.Title)
	}
	if title == "" && target.Question != nil {
		title = strings.TrimSpace(target.Question.Title)
	}
	title = first_non_empty_str(title, target.PreviewText, target.ExcerptNew, target.Excerpt, "知乎内容")

	content_url := strings.TrimSpace(target.URL)
	cover_url := ""
	if target.Thumbnail != nil {
		cover_url = strings.TrimSpace(*target.Thumbnail)
	}
	if cover_url == "" && target.ImageURL != nil {
		cover_url = strings.TrimSpace(*target.ImageURL)
	}
	if cover_url == "" && len(target.Thumbnails) > 0 {
		cover_url = strings.TrimSpace(target.Thumbnails[0])
	}
	if cover_url == "" && target.Linkbox != nil {
		cover_url = strings.TrimSpace(target.Linkbox.Pic)
	}
	if cover_url == "" {
		cover_url = zhihu.FirstImageURL(target.Content, content_url)
	}

	publish_time_seconds := int64(0)
	if target.CreatedTime != nil {
		publish_time_seconds = *target.CreatedTime
	} else if target.Created != nil {
		publish_time_seconds = *target.Created
	} else if target.Question != nil {
		publish_time_seconds = target.Question.Created
	} else {
		publish_time_seconds = feed.CreatedTime
	}

	extra_data, _ := json.Marshal(&feed)
	now := util.NowMillis()

	return &adapter.BrowseHistoryResult{
		BrowseHistory: &model.BrowseHistory{
			PlatformId:   PlatformID,
			VisitedTimes: 1,
			Type:         content_type,
			ExternalId:   external_id,
			Title:        title,
			URL:          content_url,
			SourceURL:    content_url,
			CoverURL:     cover_url,
			PublishTime:  int64_ptr(publish_time_seconds * 1000),
			ExtraData:    string(extra_data),
			Timestamps: model.Timestamps{
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		Account: RecommendFeedToAccount(&feed),
	}, nil
}

// RecommendFeedToAccount converts the target author attached to a recommendation
// item into the shared account model.
func RecommendFeedToAccount(feed *zhihu.RecommendFeed) *model.Account {
	if feed == nil {
		return nil
	}

	author := &feed.Target.Author
	external_id := first_non_empty_str(author.ID, author.URLToken)
	if external_id == "" {
		return nil
	}

	now := util.NowMillis()
	return &model.Account{
		Id:            BuildAccountID(external_id),
		PlatformId:    PlatformID,
		ExternalId:    external_id,
		Nickname:      author.Name,
		Signature:     author.Headline,
		AvatarURL:     author.AvatarURL,
		ProfileURL:    author.URL,
		FollowerCount: int64(author.FollowersCount),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

// parse_zhihu_page_content recognizes every structured result returned by
// Fetch so task previews can reuse it without another platform request.
func parse_zhihu_page_content(content_json json.RawMessage) (any, bool) {
	var answer_page zhihu.AnswerPage
	if err := json.Unmarshal(content_json, &answer_page); err == nil && answer_page.Source != "" && answer_page.Answer.ID != "" {
		return &answer_page, true
	}

	var question_page zhihu.QuestionPage
	if err := json.Unmarshal(content_json, &question_page); err == nil && question_page.Source != "" && question_page.Question.ID != "" {
		return &question_page, true
	}

	var article_page zhihu.ArticlePage
	if err := json.Unmarshal(content_json, &article_page); err == nil && article_page.Source != "" && article_page.Article.ID != "" {
		return &article_page, true
	}

	return nil, false
}

func (h *handler) BuildDownloadTask(content_json json.RawMessage, config_raw json.RawMessage) (*adapter.DownloadTaskResult, error) {
	var config map[string]any
	if err := json.Unmarshal(config_raw, &config); err != nil {
		return nil, fmt.Errorf("解析下载配置失败: %w", err)
	}

	// Structured Fetch results skip the HTTP fetch.
	var page_data any
	if parsed_page, ok := parse_zhihu_page_content(content_json); ok {
		page_data = parsed_page
	}

	var input struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	_ = json.Unmarshal(content_json, &input)

	if page_data == nil {
		if input.URL == "" {
			return nil, fmt.Errorf("知乎URL不能为空")
		}
		fetched_page, err := h.scraper_client().Fetch(input.URL)
		if err != nil {
			return nil, fmt.Errorf("获取知乎页面失败: %w", err)
		}
		page_data = fetched_page
	}

	content, err := h.ToContent(page_data)
	if err != nil {
		return nil, fmt.Errorf("转换知乎内容失败: %w", err)
	}
	if strings.TrimSpace(input.Title) != "" {
		content.Title = strings.TrimSpace(input.Title)
	}
	title := strings.TrimSpace(content.Title)
	if title == "" {
		title = "知乎内容"
		content.Title = title
	}
	external_id := content.ExternalId
	content_type := content.Type
	cover_url := content.CoverURL
	source_url := first_non_empty_str(content.SourceURL, content.URL, input.URL)

	account, account_err := h.ToAccount(page_data)
	if account_err != nil {
		now := util.NowMillis()
		account = &model.Account{
			Id:         BuildAccountID("unknown"),
			PlatformId: PlatformID,
			ExternalId: "unknown",
			Nickname:   "知乎用户",
			Timestamps: model.Timestamps{
				CreatedAt: now,
				UpdatedAt: now,
			},
		}
	}

	postprocess_data, err := marshal_postprocess_payload(page_data)
	if err != nil {
		return nil, fmt.Errorf("序列化知乎页面失败: %w", err)
	}

	content_details, err := h.ToContentDetails(page_data)
	if err != nil {
		return nil, fmt.Errorf("转换知乎内容详情失败: %w", err)
	}
	var content_detail any
	if len(content_details) > 0 {
		content_detail = content_details[0].Data
	}

	config_json, _ := json.Marshal(build_config_json(config))
	metadata_json, _ := json.Marshal(map[string]any{
		"platform":    PlatformID,
		"external_id": external_id,
		"title":       title,
		"type":        content_type,
	})

	content_id := content.Id
	postprocess_extra, _ := json.Marshal(map[string]string{
		postprocess_marker_key: postprocess_marker_value,
	})

	// HTML resource
	html_resource := model.DownloadResource{
		ContentId: &content_id,
		Name:      title + ".html",
		Kind:      "html",
		UniqueID:  external_id + "_html",
		Extra:     string(postprocess_extra),
	}
	html_endpoint := model.DownloadEndpoint{
		Protocol: "inline",
		URL:      string(postprocess_data),
		Enabled:  1,
	}

	resources := []*adapter.ResourceInfo{
		{
			Resource:  html_resource,
			Endpoints: []model.DownloadEndpoint{html_endpoint},
		},
	}

	// Cover image resource
	if cover_url != "" {
		cover_resource := model.DownloadResource{
			ContentId:  &content_id,
			Name:       title,
			Kind:       "image",
			UniqueID:   external_id + "_cover",
			MergeOrder: 999,
		}
		cover_endpoint := model.DownloadEndpoint{
			Protocol: "https",
			URL:      cover_url,
			Enabled:  1,
		}
		resources = append(resources, &adapter.ResourceInfo{
			Resource:  cover_resource,
			Endpoints: []model.DownloadEndpoint{cover_endpoint},
		})
	}

	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content.Id,
			Name:         title,
			UniqueID:     external_id,
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			SourceURL:    source_url,
			CoverURL:     cover_url,
			ConfigJSON:   string(config_json),
			MetadataJSON: string(metadata_json),
		},
		Resources:      resources,
		ContentDetail:  content_detail,
		ContentDetails: content_details,
		Account:        account,
		Content:        content,
	}, nil
}

// int64_ptr returns a pointer to an int64 value, or nil if the value is zero.
func int64_ptr(v int64) *int64 {
	if v <= 0 {
		return nil
	}
	return &v
}

// first_non_empty_str returns the first non-empty string from the given values.
func first_non_empty_str(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// build_config_json returns a map containing only the non-empty config fields.
func build_config_json(config map[string]any) map[string]any {
	m := make(map[string]any, len(config))
	for key, value := range config {
		m[key] = value
	}
	return m
}

// BuildBrowseRecordFromObject converts a zhihu.AnswerPage into a
// model.BrowseHistory, following the same field mapping as zhihu.main.js
// reportCard. Returns nil if the page is nil or missing the answer ID.
func BuildBrowseRecordFromObject(page *zhihu.AnswerPage) *model.BrowseHistory {
	if page == nil || page.Answer.ID == "" {
		return nil
	}

	now := util.NowMillis()

	extra_data, _ := json.Marshal(map[string]any{
		"question_id":   page.Question.ID,
		"answer_id":     page.Answer.ID,
		"voteup_count":  page.Answer.VoteupCount,
		"comment_count": page.Answer.CommentCount,
		"created_time":  page.Answer.CreatedTime,
		"updated_time":  page.Answer.UpdatedTime,
	})

	return &model.BrowseHistory{
		PlatformId:   platform_id_zhihu,
		VisitedTimes: 1,
		Type:         "answer",
		ExternalId:   page.Answer.ID,
		Title:        page.Question.Title,
		URL:          page.Source,
		SourceURL:    page.Source,
		CoverURL:     zhihu.FirstImageURL(page.Answer.Content, page.Source),
		PublishTime:  int64_ptr(page.Answer.CreatedTime * 1000),
		ExtraData:    string(extra_data),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}
