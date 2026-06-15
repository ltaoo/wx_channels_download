package officialaccount

import (
	"context"
	"fmt"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	officialaccountpkg "wx_channel/pkg/officialaccount"
)

const PlatformID = "officialaccount"

type ArticleFetcher interface {
	FetchArticle(url string) (*officialaccountpkg.WechatOfficialArticle, error)
}

type Handler struct {
	Fetcher ArticleFetcher
}

func New(fetcher ArticleFetcher) *Handler {
	if fetcher == nil {
		fetcher = &officialaccountpkg.OfficialAccountDownload{}
	}
	return &Handler{Fetcher: fetcher}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	return officialaccountpkg.ExtractArticleID(rawURL) != ""
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	articleID := officialaccountpkg.ExtractArticleID(input.URL)
	if articleID == "" {
		return nil, contentdownload.ErrUnsupportedURL
	}
	article, err := h.Fetcher.FetchArticle(input.URL)
	if err != nil {
		return nil, fmt.Errorf("fetch official account article: %w", err)
	}
	title := firstNonEmpty(article.Title, "wechat_official_"+articleID)
	coverURL := ""
	if len(article.Images) > 0 {
		coverURL = article.Images[0]
	}
	return &contentdownload.Probe{
		Platform:  PlatformID,
		SourceURL: input.URL,
		ContentID: articleID,
		Content: NewArticleContentEnvelope(
			contentdownload.ContentSummary{
				Platform:        PlatformID,
				Type:            ContentTypeArticle,
				ID:              articleID,
				Title:           title,
				Description:     article.Creator,
				Author:          firstNonEmpty(article.AuthorNickname, article.Creator, article.AuthorID),
				URL:             input.URL,
				SourceURL:       input.URL,
				AuthorNickname:  firstNonEmpty(article.AuthorNickname, article.Creator, article.AuthorID),
				AuthorAvatarURL: article.AuthorAvatar,
				CoverURL:        coverURL,
			},
			article,
			ArticleMetadata{
				ArticleID: articleID,
				AuthorID:  article.AuthorID,
			},
			ArticleOutput{
				Format:      OutputFormatHTML,
				ContentType: ContentTypeArticle,
				ArticleID:   articleID,
				Title:       title,
				SourceURL:   input.URL,
				BodyHTML:    article.Content,
			},
		),
		Variants: []contentdownload.Variant{
			{ID: "html", Type: "html", Label: "HTML", Suffix: ".html"},
		},
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: map[string]any{
			"pagejson": article.PageJSON,
			"pagehtml": article.PageHTML,
		},
	}, nil
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
	filename := firstNonEmpty(input.Options.Filename, contentdownload.ContentTitle(probe.Content), "wechat_official_"+probe.ContentID)
	suffix := firstNonEmpty(input.Options.Suffix, variant.Suffix, ".html")
	resolved := &contentdownload.ResolvedRequest{
		Platform:  PlatformID,
		SourceURL: probe.SourceURL,
		ContentID: probe.ContentID,
		Title:     contentdownload.ContentTitle(probe.Content),
		Filename:  filename,
		Suffix:    suffix,
		Download: contentdownload.DownloadSpec{
			URL:         "officialaccount://" + probe.SourceURL,
			Method:      "GET",
			Protocol:    "officialaccount",
			Connections: 1,
		},
		Labels: map[string]string{
			"platform":   PlatformID,
			"id":         probe.ContentID,
			"article_id": probe.ContentID,
			"title":      contentdownload.ContentTitle(probe.Content),
			"key":        "0",
			"spec":       variant.Spec,
			"suffix":     suffix,
			"source_url": probe.SourceURL,
		},
		Metadata: map[string]any{
			"variant_id": variant.ID,
			"article":    contentdownload.ContentDataOf(probe.Content),
		},
		Content: officialAccountContentWithSummary(probe.Content, contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            ContentTypeArticle,
			ID:              probe.ContentID,
			Title:           contentdownload.ContentTitle(probe.Content),
			URL:             probe.SourceURL,
			SourceURL:       probe.SourceURL,
			Author:          contentdownload.ContentAuthor(probe.Content),
			AuthorNickname:  contentdownload.ContentAuthorNickname(probe.Content),
			AuthorAvatarURL: contentdownload.ContentAuthorAvatarURL(probe.Content),
			CoverURL:        contentdownload.ContentCoverURL(probe.Content),
		}),
	}
	plan, err := h.Plan(ctx, resolved)
	if err != nil {
		return nil, err
	}
	resolved.Pipeline = plan
	return resolved, nil
}

func officialAccountContentWithSummary(content any, summary contentdownload.ContentSummary) any {
	switch c := content.(type) {
	case *ArticleContentEnvelope:
		next := *c
		next.Summary = summary
		return &next
	default:
		return contentdownload.NewContent(summary, contentdownload.ContentDataOf(content), contentdownload.ContentMetadataOf(content), contentdownload.ContentOutputOf(content))
	}
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	return &contentdownload.PipelinePlan{
		Platform: PlatformID,
		Nodes: []contentdownload.PipelineNode{
			{ID: "download", Type: "download_asset", Stage: "download"},
			{ID: "sanitize_html", Type: "sanitize_html", Stage: "post", DependsOn: []string{"download"}},
			{ID: "render_template", Type: "render_html_template", Stage: "post", DependsOn: []string{"sanitize_html"}, Args: map[string]any{"template": "officialaccount/article"}},
			{ID: "rewrite_assets", Type: "rewrite_html_assets", Stage: "post", DependsOn: []string{"render_template"}},
			{ID: "persist", Type: "persist_artifacts", Stage: "persist", DependsOn: []string{"rewrite_assets"}},
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
