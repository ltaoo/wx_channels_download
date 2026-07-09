package zhihu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	zhihupkg "wx_channel/pkg/scraper/zhihu"
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
	authorHomepageURL := zhihupkg.UserURL(page.Answer.Author)
	coverURL := zhihupkg.FirstImageURL(page.Answer.Content, answerURL.Canonical)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: answerURL.Canonical,
		ContentID:    answerURL.AnswerID,
		Content: NewAnswerContentEnvelope(
			contentdownload.ContentSummary{
				Platform:        PlatformID,
				Type:            ContentTypeAnswer,
				ID:              answerURL.AnswerID,
				Title:           title,
				Description:     page.Answer.Excerpt,
				Author:          authorNickname,
				URL:             answerURL.Canonical,
				SourceURL:       answerURL.Canonical,
				AuthorNickname:  authorNickname,
				AuthorAvatarURL: authorAvatarURL,
				CoverURL:        coverURL,
			},
			AnswerContent{Question: page.Question, Answer: page.Answer},
			AnswerMetadata{
				QuestionID:        answerURL.QuestionID,
				AnswerID:          answerURL.AnswerID,
				QuestionTitle:     page.Question.Title,
				AuthorID:          page.Answer.Author.ID,
				AuthorURLToken:    firstNonEmpty(page.Answer.Author.URLToken, page.Answer.Author.URLTokenSnake),
				AuthorAvatarURL:   authorAvatarURL,
				AuthorHomepageURL: authorHomepageURL,
				CreatedTime:       page.Answer.CreatedTime,
				UpdatedTime:       page.Answer.UpdatedTime,
				CommentCount:      page.Answer.CommentCount,
				SourceURL:         answerURL.Canonical,
			},
			AnswerOutput{
				Format:            OutputFormatHTML,
				ContentType:       ContentTypeAnswer,
				QuestionID:        answerURL.QuestionID,
				AnswerID:          answerURL.AnswerID,
				Title:             title,
				SourceURL:         answerURL.Canonical,
				CanonicalURL:      answerURL.Canonical,
				AuthorAvatarURL:   authorAvatarURL,
				AuthorHomepageURL: authorHomepageURL,
				BodyHTML:          page.Answer.Content,
				QuestionHTML:      page.Question.Detail,
			},
		),
		Variants: zhihuVariants(ContentTypeAnswer, page),
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: map[string]any{
			"answer_url": answerURL,
			"page":       page,
			"pagejson":   page.InitialDataJSON,
			"pagehtml":   page.PageHTML,
		},
	}
}

func (h *Handler) questionProbe(sourceURL string, questionURL zhihupkg.QuestionURL, page *zhihupkg.QuestionPage) *contentdownload.Probe {
	title := firstNonEmpty(page.Question.Title, "zhihu_"+questionURL.QuestionID)
	authorNickname := zhihupkg.UserDisplayName(page.Question.Author)
	authorAvatarURL := zhihupkg.UserAvatarURL(page.Question.Author)
	authorHomepageURL := zhihupkg.UserURL(page.Question.Author)
	description := firstNonEmpty(page.Question.Excerpt, strings.TrimSpace(page.Question.Detail))
	coverURL := zhihupkg.FirstImageURL(page.Question.Detail, questionURL.Canonical)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: questionURL.Canonical,
		ContentID:    questionURL.QuestionID,
		Content: NewQuestionContentEnvelope(contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            ContentTypeQuestion,
			ID:              questionURL.QuestionID,
			Title:           title,
			Description:     description,
			Author:          authorNickname,
			URL:             questionURL.Canonical,
			SourceURL:       questionURL.Canonical,
			AuthorNickname:  authorNickname,
			AuthorAvatarURL: authorAvatarURL,
			CoverURL:        coverURL,
		}, QuestionContent{Question: page.Question}, QuestionMetadata{
			QuestionID:        questionURL.QuestionID,
			AuthorID:          page.Question.Author.ID,
			AuthorURLToken:    firstNonEmpty(page.Question.Author.URLToken, page.Question.Author.URLTokenSnake),
			AuthorAvatarURL:   authorAvatarURL,
			AuthorHomepageURL: authorHomepageURL,
			SourceURL:         questionURL.Canonical,
		}, QuestionOutput{
			Format:            OutputFormatHTML,
			ContentType:       ContentTypeQuestion,
			QuestionID:        questionURL.QuestionID,
			Title:             title,
			SourceURL:         questionURL.Canonical,
			CanonicalURL:      questionURL.Canonical,
			AuthorAvatarURL:   authorAvatarURL,
			AuthorHomepageURL: authorHomepageURL,
			BodyHTML:          firstNonEmpty(page.Question.Detail, page.Question.Excerpt),
		}),
		Variants: zhihuVariants(ContentTypeQuestion, page),
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: map[string]any{
			"question_url": questionURL,
			"page":         page,
			"pagejson":     page.InitialDataJSON,
			"pagehtml":     page.PageHTML,
		},
	}
}

func (h *Handler) articleProbe(sourceURL string, articleURL zhihupkg.ArticleURL, page *zhihupkg.ArticlePage) *contentdownload.Probe {
	title := firstNonEmpty(page.Article.Title, "zhihu_"+articleURL.ArticleID)
	authorNickname := zhihupkg.UserDisplayName(page.Article.Author)
	authorAvatarURL := zhihupkg.UserAvatarURL(page.Article.Author)
	authorHomepageURL := zhihupkg.UserURL(page.Article.Author)
	coverURL := firstNonEmpty(page.Article.ImageURL, page.Article.ImageURLAlt, zhihupkg.FirstImageURL(page.Article.Content, articleURL.Canonical))
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: articleURL.Canonical,
		ContentID:    articleURL.ArticleID,
		Content: NewArticleContentEnvelope(contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            ContentTypeArticle,
			ID:              articleURL.ArticleID,
			Title:           title,
			Description:     page.Article.Excerpt,
			Author:          authorNickname,
			URL:             articleURL.Canonical,
			SourceURL:       articleURL.Canonical,
			AuthorNickname:  authorNickname,
			AuthorAvatarURL: authorAvatarURL,
			CoverURL:        coverURL,
		}, ArticleContent{Article: page.Article}, ArticleMetadata{
			ArticleID:         articleURL.ArticleID,
			AuthorID:          page.Article.Author.ID,
			AuthorURLToken:    firstNonEmpty(page.Article.Author.URLToken, page.Article.Author.URLTokenSnake),
			AuthorAvatarURL:   authorAvatarURL,
			AuthorHomepageURL: authorHomepageURL,
			CreatedTime:       page.Article.CreatedTime,
			UpdatedTime:       page.Article.UpdatedTime,
			SourceURL:         articleURL.Canonical,
		}, ArticleOutput{
			Format:            OutputFormatHTML,
			ContentType:       ContentTypeArticle,
			ArticleID:         articleURL.ArticleID,
			Title:             title,
			SourceURL:         articleURL.Canonical,
			CanonicalURL:      articleURL.Canonical,
			AuthorAvatarURL:   authorAvatarURL,
			AuthorHomepageURL: authorHomepageURL,
			BodyHTML:          page.Article.Content,
		}),
		Variants: zhihuVariants(ContentTypeArticle, page),
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: map[string]any{
			"article_url": articleURL,
			"page":        page,
			"pagejson":    page.InitialDataJSON,
			"pagehtml":    page.PageHTML,
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
	contentType := firstNonEmpty(summary.Type, ContentTypeAnswer)
	questionID, answerID, articleID := zhihuIDs(probe)
	page := zhihuPageFromProbe(probe)
	probeMetadata := contentdownload.ContentMetadataOf(probe.Content)
	authorHomepageURL, _ := probeMetadata["author_homepage_url"].(string)
	download := contentdownload.DownloadSpec{
		URL:         "zhihu://" + canonicalURL,
		Method:      "GET",
		Protocol:    "zhihu",
		Connections: 1,
	}
	metadata := map[string]any{
		"variant_id":          variant.ID,
		"content_type":        contentType,
		"question_id":         questionID,
		"answer_id":           answerID,
		"article_id":          articleID,
		"author_nickname":     contentdownload.ContentAuthorNickname(probe.Content),
		"author_avatar_url":   contentdownload.ContentAuthorAvatarURL(probe.Content),
		"author_homepage_url": authorHomepageURL,
		"source_url":          sourceURL,
		"canonical_url":       canonicalURL,
		"page":                page,
	}
	if isInitialDataJSONVariant(variant) {
		raw := initialDataJSONFromProbe(probe)
		if len(raw) == 0 {
			return nil, fmt.Errorf("missing zhihu initial data json")
		}
		suffix = firstNonEmpty(input.Options.Suffix, variant.Suffix, ".json")
		download = contentdownload.DownloadSpec{
			URL:         "inline-json://zhihu/" + contentID + "/initial-data",
			Method:      "GET",
			Protocol:    "inline_json",
			Connections: 1,
		}
		metadata["json"] = json.RawMessage(append([]byte(nil), raw...))
	}
	resolvedSummary := contentdownload.ContentSummary{
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
	}
	resolved := &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: canonicalURL,
		ContentID:    contentID,
		Title:        title,
		Filename:     filename,
		Suffix:       suffix,
		Download:     download,
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
		Metadata: metadata,
		Content:  zhihuContentWithSummary(probe.Content, resolvedSummary),
	}
	plan, err := h.Plan(ctx, resolved)
	if err != nil {
		return nil, err
	}
	resolved.Pipeline = plan
	return resolved, nil
}

func zhihuContentWithSummary(content any, summary contentdownload.ContentSummary) any {
	switch c := content.(type) {
	case *AnswerContentEnvelope:
		next := *c
		next.Summary = summary
		return &next
	case *QuestionContentEnvelope:
		next := *c
		next.Summary = summary
		return &next
	case *ArticleContentEnvelope:
		next := *c
		next.Summary = summary
		return &next
	default:
		return contentdownload.NewContent(summary, contentdownload.ContentDataOf(content), contentdownload.ContentMetadataOf(content), contentdownload.ContentOutputOf(content))
	}
}

func zhihuIDs(probe *contentdownload.Probe) (questionID string, answerID string, articleID string) {
	if probe == nil {
		return "", "", ""
	}
	if probe.Internal != nil {
		if answerURL, _ := probe.Internal["answer_url"].(zhihupkg.AnswerURL); answerURL.AnswerID != "" {
			return answerURL.QuestionID, answerURL.AnswerID, ""
		}
		if questionURL, _ := probe.Internal["question_url"].(zhihupkg.QuestionURL); questionURL.QuestionID != "" {
			return questionURL.QuestionID, "", ""
		}
		if articleURL, _ := probe.Internal["article_url"].(zhihupkg.ArticleURL); articleURL.ArticleID != "" {
			return "", "", articleURL.ArticleID
		}
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

func zhihuVariants(contentType string, page any) []contentdownload.Variant {
	variants := []contentdownload.Variant{htmlVariant(contentType)}
	if len(initialDataJSONFromPage(page)) > 0 {
		variants = append(variants, initialDataJSONVariant(contentType))
	}
	return variants
}

func initialDataJSONVariant(contentType string) contentdownload.Variant {
	return contentdownload.Variant{
		ID:     "initial_data_json",
		Type:   OutputFormatJSON,
		Label:  "原始 JSON",
		Suffix: ".json",
		Metadata: map[string]any{
			"format":       OutputFormatJSON,
			"content_type": contentType,
			"source":       "js-initialData",
		},
	}
}

func isInitialDataJSONVariant(variant *contentdownload.Variant) bool {
	if variant == nil {
		return false
	}
	return variant.ID == "initial_data_json"
}

func initialDataJSONFromProbe(probe *contentdownload.Probe) []byte {
	return initialDataJSONFromPage(zhihuPageFromProbe(probe))
}

func initialDataJSONFromPage(page any) []byte {
	switch p := page.(type) {
	case *zhihupkg.AnswerPage:
		return p.InitialDataJSON
	case *zhihupkg.QuestionPage:
		return p.InitialDataJSON
	case *zhihupkg.ArticlePage:
		return p.InitialDataJSON
	default:
		return nil
	}
}

func zhihuPageFromProbe(probe *contentdownload.Probe) any {
	if probe == nil || probe.Internal == nil {
		return nil
	}
	return probe.Internal["page"]
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	if resolved != nil && strings.EqualFold(resolved.Download.Protocol, "inline_json") {
		return &contentdownload.PipelinePlan{
			Platform: PlatformID,
			Nodes: []contentdownload.PipelineNode{
				{ID: "download", Type: "download_asset", Stage: "download"},
				{ID: "persist", Type: "persist_artifacts", Stage: "persist", DependsOn: []string{"download"}},
			},
		}, nil
	}
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
