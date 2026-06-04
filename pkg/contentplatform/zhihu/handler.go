package zhihu

import (
	"context"
	"fmt"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	zhihupkg "wx_channel/pkg/zhihu"
)

const PlatformID = "zhihu"

type PageFetcher interface {
	FetchAnswerPage(rawURL string) (*zhihupkg.AnswerPage, error)
}

type QuestionPageFetcher interface {
	FetchQuestionPage(rawURL string) (*zhihupkg.QuestionPage, error)
}

type ArticlePageFetcher interface {
	FetchArticlePage(rawURL string) (*zhihupkg.ArticlePage, error)
}

type Handler struct {
	Client PageFetcher
}

type AnswerContent struct {
	Question zhihupkg.Question `json:"question"`
	Answer   zhihupkg.Answer   `json:"answer"`
}

type QuestionContent struct {
	Question zhihupkg.Question `json:"question"`
}

type ArticleContent struct {
	Article zhihupkg.Article `json:"article"`
}

func New(client PageFetcher) *Handler {
	if client == nil {
		client = &zhihupkg.Client{}
	}
	return &Handler{Client: client}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	realURL := zhihupkg.ResolveRealURL(rawURL)
	if _, ok := zhihupkg.ParseAnswerURL(realURL); ok {
		return true
	}
	if _, ok := zhihupkg.ParseQuestionURL(realURL); ok {
		return true
	}
	_, ok := zhihupkg.ParseArticleURL(realURL)
	return ok
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	realURL := zhihupkg.ResolveRealURL(input.URL)
	if answerURL, ok := zhihupkg.ParseAnswerURL(realURL); ok {
		page, err := h.Client.FetchAnswerPage(answerURL.Canonical)
		if err != nil {
			return nil, fmt.Errorf("fetch zhihu answer: %w", err)
		}
		return h.answerProbe(input.URL, answerURL, page), nil
	}
	if questionURL, ok := zhihupkg.ParseQuestionURL(realURL); ok {
		fetcher, ok := h.Client.(QuestionPageFetcher)
		if !ok {
			return nil, contentdownload.ErrResolveUnavailable
		}
		page, err := fetcher.FetchQuestionPage(questionURL.Canonical)
		if err != nil {
			return nil, fmt.Errorf("fetch zhihu question: %w", err)
		}
		return h.questionProbe(input.URL, questionURL, page), nil
	}
	if articleURL, ok := zhihupkg.ParseArticleURL(realURL); ok {
		fetcher, ok := h.Client.(ArticlePageFetcher)
		if !ok {
			return nil, contentdownload.ErrResolveUnavailable
		}
		page, err := fetcher.FetchArticlePage(articleURL.Canonical)
		if err != nil {
			return nil, fmt.Errorf("fetch zhihu article: %w", err)
		}
		return h.articleProbe(input.URL, articleURL, page), nil
	}
	return nil, contentdownload.ErrUnsupportedURL
}

func (h *Handler) answerProbe(sourceURL string, answerURL zhihupkg.AnswerURL, page *zhihupkg.AnswerPage) *contentdownload.Probe {
	title := firstNonEmpty(page.Question.Title, "zhihu_"+answerURL.AnswerID)
	authorNickname := zhihupkg.UserDisplayName(page.Answer.Author)
	authorAvatarURL := zhihupkg.UserAvatarURL(page.Answer.Author)
	coverURL := firstNonEmpty(zhihupkg.FirstImageURL(page.Answer.Content, answerURL.Canonical), authorAvatarURL)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: answerURL.Canonical,
		ContentID:    answerURL.AnswerID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            "answer",
			ID:              answerURL.AnswerID,
			Title:           title,
			Description:     page.Answer.Excerpt,
			Author:          authorNickname,
			URL:             answerURL.Canonical,
			SourceURL:       answerURL.Canonical,
			AuthorNickname:  authorNickname,
			AuthorAvatarURL: authorAvatarURL,
			CoverURL:        coverURL,
		}, AnswerContent{Question: page.Question, Answer: page.Answer}, map[string]any{
			"question_id":      answerURL.QuestionID,
			"answer_id":        answerURL.AnswerID,
			"question_title":   page.Question.Title,
			"author_id":        page.Answer.Author.ID,
			"author_url_token": firstNonEmpty(page.Answer.Author.URLToken, page.Answer.Author.URLTokenSnake),
			"created_time":     page.Answer.CreatedTime,
			"updated_time":     page.Answer.UpdatedTime,
			"comment_count":    page.Answer.CommentCount,
			"source_url":       answerURL.Canonical,
		}, map[string]any{
			"format":        "html",
			"content_type":  "answer",
			"question_id":   answerURL.QuestionID,
			"answer_id":     answerURL.AnswerID,
			"title":         title,
			"source_url":    answerURL.Canonical,
			"canonical_url": answerURL.Canonical,
			"body_html":     page.Answer.Content,
			"question_html": page.Question.Detail,
		}),
		Variants: []contentdownload.Variant{
			htmlVariant("answer"),
		},
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: map[string]any{
			"answer_url": answerURL,
			"page":       page,
		},
	}
}

func (h *Handler) questionProbe(sourceURL string, questionURL zhihupkg.QuestionURL, page *zhihupkg.QuestionPage) *contentdownload.Probe {
	title := firstNonEmpty(page.Question.Title, "zhihu_"+questionURL.QuestionID)
	authorNickname := zhihupkg.UserDisplayName(page.Question.Author)
	authorAvatarURL := zhihupkg.UserAvatarURL(page.Question.Author)
	description := firstNonEmpty(page.Question.Excerpt, strings.TrimSpace(page.Question.Detail))
	coverURL := firstNonEmpty(zhihupkg.FirstImageURL(page.Question.Detail, questionURL.Canonical), authorAvatarURL)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: questionURL.Canonical,
		ContentID:    questionURL.QuestionID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            "question",
			ID:              questionURL.QuestionID,
			Title:           title,
			Description:     description,
			Author:          authorNickname,
			URL:             questionURL.Canonical,
			SourceURL:       questionURL.Canonical,
			AuthorNickname:  authorNickname,
			AuthorAvatarURL: authorAvatarURL,
			CoverURL:        coverURL,
		}, QuestionContent{Question: page.Question}, map[string]any{
			"question_id":      questionURL.QuestionID,
			"author_id":        page.Question.Author.ID,
			"author_url_token": firstNonEmpty(page.Question.Author.URLToken, page.Question.Author.URLTokenSnake),
			"source_url":       questionURL.Canonical,
		}, map[string]any{
			"format":        "html",
			"content_type":  "question",
			"question_id":   questionURL.QuestionID,
			"title":         title,
			"source_url":    questionURL.Canonical,
			"canonical_url": questionURL.Canonical,
			"body_html":     firstNonEmpty(page.Question.Detail, page.Question.Excerpt),
		}),
		Variants: []contentdownload.Variant{
			htmlVariant("question"),
		},
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: map[string]any{
			"question_url": questionURL,
			"page":         page,
		},
	}
}

func (h *Handler) articleProbe(sourceURL string, articleURL zhihupkg.ArticleURL, page *zhihupkg.ArticlePage) *contentdownload.Probe {
	title := firstNonEmpty(page.Article.Title, "zhihu_"+articleURL.ArticleID)
	authorNickname := zhihupkg.UserDisplayName(page.Article.Author)
	authorAvatarURL := zhihupkg.UserAvatarURL(page.Article.Author)
	coverURL := firstNonEmpty(page.Article.ImageURL, page.Article.ImageURLAlt, zhihupkg.FirstImageURL(page.Article.Content, articleURL.Canonical), authorAvatarURL)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: articleURL.Canonical,
		ContentID:    articleURL.ArticleID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            "article",
			ID:              articleURL.ArticleID,
			Title:           title,
			Description:     page.Article.Excerpt,
			Author:          authorNickname,
			URL:             articleURL.Canonical,
			SourceURL:       articleURL.Canonical,
			AuthorNickname:  authorNickname,
			AuthorAvatarURL: authorAvatarURL,
			CoverURL:        coverURL,
		}, ArticleContent{Article: page.Article}, map[string]any{
			"article_id":       articleURL.ArticleID,
			"author_id":        page.Article.Author.ID,
			"author_url_token": firstNonEmpty(page.Article.Author.URLToken, page.Article.Author.URLTokenSnake),
			"created_time":     page.Article.CreatedTime,
			"updated_time":     page.Article.UpdatedTime,
			"source_url":       articleURL.Canonical,
		}, map[string]any{
			"format":        "html",
			"content_type":  "article",
			"article_id":    articleURL.ArticleID,
			"title":         title,
			"source_url":    articleURL.Canonical,
			"canonical_url": articleURL.Canonical,
			"body_html":     page.Article.Content,
		}),
		Variants: []contentdownload.Variant{
			htmlVariant("article"),
		},
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: map[string]any{
			"article_url": articleURL,
			"page":        page,
		},
	}
}

func (h *Handler) Resolve(ctx context.Context, input contentdownload.ResolveInput) (*contentdownload.ResolvedRequest, error) {
	probe := input.Probe
	if probe == nil {
		var err error
		probe, err = h.Probe(ctx, contentdownload.ProbeInput{URL: input.URL, Extra: input.Extra})
		if err != nil {
			return nil, err
		}
	}
	variant, err := contentdownload.SelectVariant(probe, input.Options)
	if err != nil {
		return nil, err
	}
	summary := contentdownload.ContentSummaryOf(probe.Content)
	contentID := firstNonEmpty(probe.ContentID, summary.ID)
	title := firstNonEmpty(summary.Title, contentID)
	sourceURL := firstNonEmpty(probe.SourceURL, input.URL, summary.SourceURL, probe.CanonicalURL)
	canonicalURL := firstNonEmpty(probe.CanonicalURL, summary.URL, sourceURL)
	filename := firstNonEmpty(input.Options.Filename, title, contentID)
	suffix := firstNonEmpty(input.Options.Suffix, variant.Suffix, ".html")
	contentType := firstNonEmpty(summary.Type, "answer")
	questionID, answerID, articleID := zhihuIDs(probe)
	resolved := &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: canonicalURL,
		ContentID:    contentID,
		Title:        title,
		Filename:     filename,
		Suffix:       suffix,
		Download: contentdownload.DownloadSpec{
			URL:         "zhihu://" + canonicalURL,
			Method:      "GET",
			Protocol:    "zhihu",
			Connections: 1,
		},
		Labels: map[string]string{
			"platform":     PlatformID,
			"id":           contentID,
			"question_id":  questionID,
			"answer_id":    answerID,
			"article_id":   articleID,
			"title":        title,
			"key":          "0",
			"spec":         variant.Spec,
			"suffix":       suffix,
			"source_url":   canonicalURL,
			"content_type": contentType,
		},
		Metadata: map[string]any{
			"variant_id":        variant.ID,
			"content_type":      contentType,
			"question_id":       questionID,
			"answer_id":         answerID,
			"article_id":        articleID,
			"author_nickname":   contentdownload.ContentAuthorNickname(probe.Content),
			"author_avatar_url": contentdownload.ContentAuthorAvatarURL(probe.Content),
			"source_url":        sourceURL,
			"canonical_url":     canonicalURL,
			"page":              probe.Internal["page"],
		},
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            contentType,
			ID:              contentID,
			Title:           title,
			Description:     summary.Description,
			URL:             firstNonEmpty(summary.URL, canonicalURL),
			SourceURL:       firstNonEmpty(summary.SourceURL, canonicalURL, sourceURL),
			Author:          firstNonEmpty(summary.Author, summary.AuthorNickname),
			AuthorNickname:  summary.AuthorNickname,
			AuthorAvatarURL: summary.AuthorAvatarURL,
			CoverURL:        summary.CoverURL,
			Duration:        summary.Duration,
		}, contentdownload.ContentDataOf(probe.Content), contentdownload.ContentMetadataOf(probe.Content), contentdownload.ContentOutputOf(probe.Content)),
	}
	plan, err := h.Plan(ctx, resolved)
	if err != nil {
		return nil, err
	}
	resolved.Pipeline = plan
	return resolved, nil
}

func zhihuIDs(probe *contentdownload.Probe) (questionID string, answerID string, articleID string) {
	if probe == nil {
		return "", "", ""
	}
	if answerURL, _ := probe.Internal["answer_url"].(zhihupkg.AnswerURL); answerURL.AnswerID != "" {
		return answerURL.QuestionID, answerURL.AnswerID, ""
	}
	if questionURL, _ := probe.Internal["question_url"].(zhihupkg.QuestionURL); questionURL.QuestionID != "" {
		return questionURL.QuestionID, "", ""
	}
	if articleURL, _ := probe.Internal["article_url"].(zhihupkg.ArticleURL); articleURL.ArticleID != "" {
		return "", "", articleURL.ArticleID
	}
	metadata := contentdownload.ContentMetadataOf(probe.Content)
	questionID = metadataString(metadata, "question_id")
	answerID = metadataString(metadata, "answer_id")
	articleID = metadataString(metadata, "article_id")
	if articleID == "" && contentdownload.ContentType(probe.Content) == "article" {
		articleID = probe.ContentID
	}
	return questionID, answerID, articleID
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func htmlVariant(contentType string) contentdownload.Variant {
	return contentdownload.Variant{
		ID:     "html",
		Type:   "html",
		Label:  "HTML",
		Suffix: ".html",
		Metadata: map[string]any{
			"format":       "html",
			"content_type": contentType,
		},
	}
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	return &contentdownload.PipelinePlan{
		Platform: PlatformID,
		Nodes: []contentdownload.PipelineNode{
			{ID: "download", Type: "download_asset", Stage: "download"},
			{ID: "sanitize_html", Type: "sanitize_html", Stage: "post", DependsOn: []string{"download"}},
			{ID: "render_template", Type: "render_html_template", Stage: "post", DependsOn: []string{"sanitize_html"}, Args: map[string]any{"template": "zhihu/answer"}},
			{ID: "persist", Type: "persist_artifacts", Stage: "persist", DependsOn: []string{"render_template"}},
		},
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
