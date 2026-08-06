package zhihu

import (
	"encoding/json"
	"fmt"
	"strings"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	scraper "wx_channel/pkg/scraper/zhihu"
	"wx_channel/pkg/util"
)

const platformIDZhihu = "zhihu"

func init() {
	adapter.Register(&handler{})
}

type handler struct{}

func (h *handler) PlatformID() string { return PlatformID }

// PlatformID is the exportable platform identifier for zhihu.
const PlatformID = platformIDZhihu

// BuildContentID builds a content identifier from an external ID.
func BuildContentID(externalID string) string {
	return PlatformID + ":" + externalID
}

// BuildAccountID builds an account identifier from an external ID.
func BuildAccountID(externalID string) string {
	return PlatformID + ":" + externalID
}

// ContentExternalID builds a unique external identifier for zhihu content.
func ContentExternalID(contentType, token, url string) string {
	if token != "" {
		return "zhihu:" + contentType + ":" + token
	}
	return url
}

// ToContent converts zhihu answer page data into a model.Content.
func ToContent(page *scraper.AnswerPage) (*model.Content, error) {
	if page == nil || page.Answer.ID == "" {
		return nil, fmt.Errorf("zhihu answer page is empty")
	}

	externalID := page.Answer.ID
	now := util.NowMillis()

	contentURL := page.Source
	coverURL := scraper.FirstImageURL(page.Answer.Content, contentURL)

	return &model.Content{
		Id:           BuildContentID(externalID),
		PlatformId:   PlatformID,
		Type:         "answer",
		ExternalId:   externalID,
		ExternalId2:  page.Question.ID,
		Title:        page.Question.Title,
		Description:  page.Answer.Excerpt,
		URL:          contentURL,
		SourceURL:    contentURL,
		CoverURL:     coverURL,
		PublishTime:  int64Ptr(page.Answer.CreatedTime * 1000),
		UpdateTime:   int64Ptr(page.Answer.UpdatedTime * 1000),
		LikeCount:    int64(page.Answer.VoteupCount),
		CommentCount: int64(page.Answer.CommentCount),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// QuestionToContent converts zhihu question page data into a model.Content.
func QuestionToContent(page *scraper.QuestionPage) (*model.Content, error) {
	if page == nil || page.Question.ID == "" {
		return nil, fmt.Errorf("zhihu question page is empty")
	}

	externalID := page.Question.ID
	now := util.NowMillis()

	contentURL := page.Source
	coverURL := scraper.FirstImageURL(page.Question.Excerpt, contentURL)

	return &model.Content{
		Id:           BuildContentID(externalID),
		PlatformId:   PlatformID,
		Type:         "question",
		ExternalId:   externalID,
		Title:        page.Question.Title,
		Description:  page.Question.Excerpt,
		URL:          contentURL,
		SourceURL:    contentURL,
		CoverURL:     coverURL,
		PublishTime:  int64Ptr(page.Question.Created * 1000),
		UpdateTime:   int64Ptr(page.Question.UpdatedTime * 1000),
		LikeCount:    int64(page.Question.VoteupCount),
		CommentCount: int64(page.Question.CommentCount),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ArticleToContent converts zhihu article page data into a model.Content.
func ArticleToContent(page *scraper.ArticlePage) (*model.Content, error) {
	if page == nil || page.Article.ID == "" {
		return nil, fmt.Errorf("zhihu article page is empty")
	}

	externalID := page.Article.ID
	now := util.NowMillis()

	contentURL := page.Source
	coverURL := firstNonEmptyStr(page.Article.ImageURL, page.Article.ImageURLAlt)

	return &model.Content{
		Id:          BuildContentID(externalID),
		PlatformId:  PlatformID,
		Type:        "article",
		ExternalId:  externalID,
		Title:       page.Article.Title,
		Description: page.Article.Excerpt,
		URL:         contentURL,
		SourceURL:   contentURL,
		CoverURL:    coverURL,
		PublishTime: int64Ptr(page.Article.CreatedTime * 1000),
		UpdateTime:  int64Ptr(page.Article.UpdatedTime * 1000),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ToAccount converts a zhihu user into a model.Account.
func ToAccount(user *scraper.User) (*model.Account, error) {
	if user == nil {
		return nil, fmt.Errorf("zhihu user is empty")
	}

	externalID := scraper.UserDisplayName(*user)
	if externalID == "" {
		return nil, fmt.Errorf("zhihu user has no identifiable name")
	}

	now := util.NowMillis()
	profileURL := scraper.UserURL(*user)

	return &model.Account{
		Id:         BuildAccountID(externalID),
		PlatformId: PlatformID,
		ExternalId: user.ID,
		Nickname:   scraper.UserDisplayName(*user),
		Signature:  user.Headline,
		AvatarURL:  scraper.UserAvatarURL(*user),
		ProfileURL: profileURL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// AnswerToBrowseHistory converts a zhihu answer page into a model.BrowseHistory.
func AnswerToBrowseHistory(page *scraper.AnswerPage) (*model.BrowseHistory, error) {
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
		CoverURL:     scraper.FirstImageURL(page.Answer.Content, page.Source),
		PublishTime:  int64Ptr(page.Answer.CreatedTime * 1000),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// QuestionToBrowseHistory converts a zhihu question page into a model.BrowseHistory.
func QuestionToBrowseHistory(page *scraper.QuestionPage) (*model.BrowseHistory, error) {
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
		CoverURL:     scraper.FirstImageURL(page.Question.Excerpt, page.Source),
		PublishTime:  int64Ptr(page.Question.Created * 1000),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// ArticleToBrowseHistory converts a zhihu article page into a model.BrowseHistory.
func ArticleToBrowseHistory(page *scraper.ArticlePage) (*model.BrowseHistory, error) {
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
		CoverURL:     firstNonEmptyStr(page.Article.ImageURL, page.Article.ImageURLAlt),
		PublishTime:  int64Ptr(page.Article.CreatedTime * 1000),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}, nil
}

// BuildBrowseHistory converts an intercepted Zhihu recommendation feed item
// into the standard browse history result.
func (h *handler) BuildBrowseHistory(content_json json.RawMessage) (*adapter.BrowseHistoryResult, error) {
	var feed scraper.RecommendFeed
	if err := json.Unmarshal(content_json, &feed); err != nil {
		return nil, fmt.Errorf("解析知乎推荐内容失败: %w", err)
	}

	target := &feed.Target
	externalID := strings.TrimSpace(target.ID)
	if externalID == "" {
		return nil, fmt.Errorf("知乎推荐内容ID不能为空")
	}

	contentType := strings.TrimSpace(target.Type)
	if contentType == "" && target.ArticleType != nil {
		contentType = strings.TrimSpace(*target.ArticleType)
	}
	if contentType == "" && target.Question != nil {
		contentType = "answer"
	}
	if contentType == "" {
		contentType = "other"
	}

	title := ""
	if target.Title != nil {
		title = strings.TrimSpace(*target.Title)
	}
	if title == "" && target.Question != nil {
		title = strings.TrimSpace(target.Question.Title)
	}
	title = firstNonEmptyStr(title, target.PreviewText, target.ExcerptNew, target.Excerpt, "知乎内容")

	contentURL := strings.TrimSpace(target.URL)
	coverURL := ""
	if target.Thumbnail != nil {
		coverURL = strings.TrimSpace(*target.Thumbnail)
	}
	if coverURL == "" && target.ImageURL != nil {
		coverURL = strings.TrimSpace(*target.ImageURL)
	}
	if coverURL == "" && len(target.Thumbnails) > 0 {
		coverURL = strings.TrimSpace(target.Thumbnails[0])
	}
	if coverURL == "" && target.Linkbox != nil {
		coverURL = strings.TrimSpace(target.Linkbox.Pic)
	}
	if coverURL == "" {
		coverURL = scraper.FirstImageURL(target.Content, contentURL)
	}

	publishTimeSeconds := int64(0)
	if target.CreatedTime != nil {
		publishTimeSeconds = *target.CreatedTime
	} else if target.Created != nil {
		publishTimeSeconds = *target.Created
	} else if target.Question != nil {
		publishTimeSeconds = target.Question.Created
	} else {
		publishTimeSeconds = feed.CreatedTime
	}

	extraData, _ := json.Marshal(&feed)
	now := util.NowMillis()

	return &adapter.BrowseHistoryResult{
		BrowseHistory: &model.BrowseHistory{
			PlatformId:   PlatformID,
			VisitedTimes: 1,
			Type:         contentType,
			ExternalId:   externalID,
			Title:        title,
			URL:          contentURL,
			SourceURL:    contentURL,
			CoverURL:     coverURL,
			PublishTime:  int64Ptr(publishTimeSeconds * 1000),
			ExtraData:    string(extraData),
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
func RecommendFeedToAccount(feed *scraper.RecommendFeed) *model.Account {
	if feed == nil {
		return nil
	}

	author := &feed.Target.Author
	external_id := firstNonEmptyStr(author.ID, author.URLToken)
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

// parseAnswerPageContent attempts to unmarshal the content JSON as a scraper.AnswerPage.
// Returns the parsed page and true if the content appears to be a valid AnswerPage.
func parseAnswerPageContent(contentJSON json.RawMessage) (*scraper.AnswerPage, bool) {
	var page scraper.AnswerPage
	if err := json.Unmarshal(contentJSON, &page); err != nil {
		return nil, false
	}
	if page.Source == "" || page.Answer.ID == "" {
		return nil, false
	}
	return &page, true
}

func (h *handler) BuildDownloadTask(contentJSON json.RawMessage, configRaw json.RawMessage) (*adapter.DownloadTaskResult, error) {
	var config map[string]any
	if err := json.Unmarshal(configRaw, &config); err != nil {
		return nil, fmt.Errorf("解析下载配置失败: %w", err)
	}

	// Try AnswerPage format first (from browser injection).
	// When the content is a pre-built AnswerPage, skip the HTTP fetch.
	var page *scraper.AnswerPage
	if p, ok := parseAnswerPageContent(contentJSON); ok {
		page = p
	}

	var input struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	_ = json.Unmarshal(contentJSON, &input)

	client := scraper.NewClient("")

	var htmlContent string
	if page != nil {
		// Use the AnswerPage data directly; build HTML from Source.
		htmlContent, _ = client.BuildHTMLFromURL(page.Source)
		if htmlContent == "" {
			htmlContent = "<html><body></body></html>"
		}
	} else {
		if input.URL == "" {
			return nil, fmt.Errorf("知乎URL不能为空")
		}
		html, err := client.BuildHTMLFromURL(input.URL)
		if err != nil {
			return nil, fmt.Errorf("获取知乎页面失败: %w", err)
		}
		htmlContent = html
		page, _ = client.FetchAnswerPage(input.URL)
	}

	now := util.NowMillis()
	title := input.Title
	contentType := "answer"
	var externalID string
	var coverURL string
	var content *model.Content
	var account *model.Account
	var sourceURL string

	if page != nil && page.Answer.ID != "" {
		externalID = page.Answer.ID
		if title == "" {
			title = page.Question.Title
		}
		contentType = "answer"
		coverURL = scraper.FirstImageURL(page.Answer.Content, page.Source)
		sourceURL = page.Source

		c, err := ToContent(page)
		if err == nil {
			content = c
		}
		a, err := ToAccount(&page.Answer.Author)
		if err == nil {
			account = a
		}
	} else {
		// Fall back to parsing the URL
		if articleURL, ok := scraper.ParseArticleURL(input.URL); ok {
			externalID = articleURL.ArticleID
			contentType = "article"
			sourceURL = articleURL.Canonical
		} else if questionURL, ok := scraper.ParseQuestionURL(input.URL); ok {
			externalID = questionURL.QuestionID
			contentType = "question"
			sourceURL = questionURL.Canonical
		} else {
			externalID = input.URL
			sourceURL = input.URL
		}
	}

	if title == "" {
		title = "知乎内容"
	}

	if content == nil {
		content = &model.Content{
			Id:         BuildContentID(externalID),
			PlatformId: PlatformID,
			Type:       contentType,
			ExternalId: externalID,
			Title:      title,
			URL:        sourceURL,
			SourceURL:  sourceURL,
			CoverURL:   coverURL,
			Timestamps: model.Timestamps{
				CreatedAt: now,
				UpdatedAt: now,
			},
		}
	}

	if account == nil {
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

	configJSON, _ := json.Marshal(buildConfigJSON(config))
	metadataJSON, _ := json.Marshal(map[string]any{
		"platform":    PlatformID,
		"external_id": externalID,
		"title":       title,
		"type":        contentType,
	})

	contentID := content.Id

	// HTML resource
	htmlResource := model.DownloadResource{
		ContentId: &contentID,
		Name:      title + ".html",
		Kind:      "html",
		UniqueID:  externalID + "_html",
	}
	htmlEndpoint := model.DownloadEndpoint{
		Protocol: "inline",
		URL:      htmlContent,
		Enabled:  1,
	}

	resources := []*adapter.ResourceInfo{
		{
			DownloadResource: htmlResource,
			Endpoints:        []model.DownloadEndpoint{htmlEndpoint},
		},
	}

	// Cover image resource
	if coverURL != "" {
		coverResource := model.DownloadResource{
			ContentId:  &contentID,
			Name:       title,
			Kind:       "image",
			UniqueID:   externalID + "_cover",
			MergeOrder: 999,
		}
		coverEndpoint := model.DownloadEndpoint{
			Protocol: "https",
			URL:      coverURL,
			Enabled:  1,
		}
		resources = append(resources, &adapter.ResourceInfo{
			DownloadResource: coverResource,
			Endpoints:        []model.DownloadEndpoint{coverEndpoint},
		})
	}

	return &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{
			ContentId:    &content.Id,
			Name:         title,
			UniqueID:     externalID,
			PlatformId:   PlatformID,
			Status:       model.TaskStatusWaiting,
			SourceURL:    sourceURL,
			CoverURL:     coverURL,
			ConfigJSON:   string(configJSON),
			MetadataJSON: string(metadataJSON),
		},
		Resources: resources,
		ContentDetail: &model.ContentArticle{
			Id:   content.Id,
			Type: model.ContentArticleTypeHTML,
			HTML: htmlContent,
		},
		Account: account,
		Content: content,
	}, nil
}

// int64Ptr returns a pointer to an int64 value, or nil if the value is zero.
func int64Ptr(v int64) *int64 {
	if v <= 0 {
		return nil
	}
	return &v
}

// firstNonEmptyStr returns the first non-empty string from the given values.
func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// buildConfigJSON returns a map containing only the non-empty config fields.
func buildConfigJSON(config map[string]any) map[string]any {
	m := make(map[string]any, len(config))
	for key, value := range config {
		m[key] = value
	}
	return m
}

// BuildBrowseRecordFromObject converts a scraper.AnswerPage into a
// model.BrowseHistory, following the same field mapping as zhihu.main.js
// reportCard. Returns nil if the page is nil or missing the answer ID.
func BuildBrowseRecordFromObject(page *scraper.AnswerPage) *model.BrowseHistory {
	if page == nil || page.Answer.ID == "" {
		return nil
	}

	now := util.NowMillis()

	extraData, _ := json.Marshal(map[string]any{
		"question_id":   page.Question.ID,
		"answer_id":     page.Answer.ID,
		"voteup_count":  page.Answer.VoteupCount,
		"comment_count": page.Answer.CommentCount,
		"created_time":  page.Answer.CreatedTime,
		"updated_time":  page.Answer.UpdatedTime,
	})

	return &model.BrowseHistory{
		PlatformId:   platformIDZhihu,
		VisitedTimes: 1,
		Type:         "answer",
		ExternalId:   page.Answer.ID,
		Title:        page.Question.Title,
		URL:          page.Source,
		SourceURL:    page.Source,
		CoverURL:     scraper.FirstImageURL(page.Answer.Content, page.Source),
		PublishTime:  int64Ptr(page.Answer.CreatedTime * 1000),
		ExtraData:    string(extraData),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}
