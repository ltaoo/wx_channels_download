package webpage

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
)

type Handler struct {
	Fetcher PageFetcher
}

func New(fetcher PageFetcher) *Handler {
	if fetcher == nil {
		fetcher = &HTTPFetcher{}
	}
	return &Handler{Fetcher: fetcher}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	parsed, err := parseHTTPURL(rawURL)
	if err != nil {
		return false
	}
	return !looksLikeDirectAssetURL(parsed)
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	if !h.Match(input.URL) {
		return nil, contentdownload.ErrUnsupportedURL
	}
	page, err := h.Fetcher.FetchPage(ctx, input.URL)
	if err != nil {
		return nil, fmt.Errorf("fetch webpage article: %w", err)
	}
	if page == nil || strings.TrimSpace(page.HTML) == "" {
		return nil, fmt.Errorf("fetch webpage article: empty html")
	}
	sourceURL := firstNonEmpty(input.URL, page.URL)
	article, err := ExtractReadableArticle(page.HTML, firstNonEmpty(page.URL, input.URL))
	if err != nil {
		return nil, err
	}
	article.SourceURL = sourceURL
	article.CanonicalURL = firstNonEmpty(article.CanonicalURL, page.URL, sourceURL)
	article.ID = firstNonEmpty(article.ID, articleID(article.CanonicalURL, sourceURL))
	author := firstNonEmpty(article.Author, hostDisplayName(article.CanonicalURL), hostDisplayName(sourceURL), PlatformID)
	article.Author = author

	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: article.CanonicalURL,
		ContentID:    article.ID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:       PlatformID,
			Type:           ContentTypeArticle,
			ID:             article.ID,
			Title:          firstNonEmpty(article.Title, article.ID, "webpage"),
			Description:    article.Description,
			Author:         author,
			URL:            article.CanonicalURL,
			SourceURL:      sourceURL,
			AuthorNickname: author,
			CoverURL:       article.CoverURL,
		}, article, map[string]any{
			"article_id":          article.ID,
			"account_external_id": hostDisplayName(article.CanonicalURL),
			"account_username":    hostDisplayName(article.CanonicalURL),
			"author_name":         author,
			"site_name":           article.SiteName,
			"language":            article.Language,
			"source_url":          sourceURL,
			"canonical_url":       article.CanonicalURL,
			"published_time":      article.PublishedTime,
			"modified_time":       article.ModifiedTime,
			"content_type":        ContentTypeArticle,
			"mime_type":           "text/html",
			"status_code":         page.StatusCode,
			"content_type_header": page.ContentType,
			"extractor":           article.Extractor,
			"extractor_reason":    article.ExtractorReason,
			"quality":             article.Quality,
			"content_length":      article.ContentLength,
		}, ProbeOutput{
			Format:          OutputFormatHTML,
			ContentType:     ContentTypeArticle,
			ArticleID:       article.ID,
			Title:           article.Title,
			SourceURL:       sourceURL,
			CanonicalURL:    article.CanonicalURL,
			Author:          author,
			PublishedTime:   article.PublishedTime,
			ModifiedTime:    article.ModifiedTime,
			BodyHTML:        article.BodyHTML,
			Extractor:       article.Extractor,
			ExtractorReason: article.ExtractorReason,
			Quality:         article.Quality,
			ContentLength:   article.ContentLength,
		}.Map()),
		Variants: []contentdownload.Variant{
			{
				ID:     "html",
				Type:   "html",
				Label:  "文章 HTML",
				Suffix: ".html",
				Metadata: map[string]any{
					"format":       OutputFormatHTML,
					"content_type": ContentTypeArticle,
				},
			},
		},
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
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
	summary := contentdownload.ContentSummaryOf(probe.Content)
	contentID := firstNonEmpty(probe.ContentID, summary.ID)
	title := firstNonEmpty(summary.Title, contentID, "webpage")
	sourceURL := firstNonEmpty(probe.SourceURL, input.URL, summary.SourceURL, probe.CanonicalURL)
	canonicalURL := firstNonEmpty(probe.CanonicalURL, summary.URL, sourceURL)
	filename := sanitizeFilename(firstNonEmpty(input.Options.Filename, title, contentID, "webpage"))
	suffix := firstNonEmpty(input.Options.Suffix, variant.Suffix, ".html")
	contentMetadata := cloneAnyMap(contentdownload.ContentMetadataOf(probe.Content))
	contentOutput := contentdownload.ContentOutputOf(probe.Content)
	metadata := cloneAnyMap(contentMetadata)
	metadata["variant_id"] = variant.ID
	metadata["content_type"] = ContentTypeArticle
	metadata["source_url"] = sourceURL
	metadata["canonical_url"] = canonicalURL
	metadata["mime_type"] = "text/html"
	if bodyHTML, _ := contentOutput["body_html"].(string); strings.TrimSpace(bodyHTML) != "" {
		metadata["body_html"] = bodyHTML
	}

	resolved := &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: canonicalURL,
		ContentID:    contentID,
		Title:        title,
		Filename:     filename,
		Suffix:       suffix,
		Download: contentdownload.DownloadSpec{
			URL:         inlineHTMLURL(contentID),
			Method:      "GET",
			Protocol:    "inline_html",
			Connections: 1,
		},
		Labels: map[string]string{
			"platform":     PlatformID,
			"id":           contentID,
			"article_id":   contentID,
			"title":        title,
			"key":          "0",
			"spec":         variant.Spec,
			"suffix":       suffix,
			"source_url":   canonicalURL,
			"content_type": ContentTypeArticle,
			"mime_type":    "text/html",
		},
		Metadata: metadata,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            ContentTypeArticle,
			ID:              contentID,
			Title:           title,
			Description:     summary.Description,
			Author:          firstNonEmpty(summary.Author, summary.AuthorNickname),
			URL:             firstNonEmpty(summary.URL, canonicalURL),
			SourceURL:       firstNonEmpty(summary.SourceURL, canonicalURL, sourceURL),
			AuthorNickname:  summary.AuthorNickname,
			AuthorAvatarURL: summary.AuthorAvatarURL,
			CoverURL:        summary.CoverURL,
			Duration:        summary.Duration,
		}, contentdownload.ContentDataOf(probe.Content), contentMetadata, contentOutput),
	}
	plan, err := h.Plan(ctx, resolved)
	if err != nil {
		return nil, err
	}
	resolved.Pipeline = plan
	return resolved, nil
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	return &contentdownload.PipelinePlan{
		Platform: PlatformID,
		Nodes: []contentdownload.PipelineNode{
			{ID: "download", Type: "download_asset", Stage: "download"},
			{ID: "persist", Type: "persist_artifacts", Stage: "persist", DependsOn: []string{"download"}},
		},
	}, nil
}

func articleID(values ...string) string {
	h := sha1.New()
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			_, _ = h.Write([]byte(strings.TrimSpace(value)))
			break
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func hostDisplayName(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
}

func looksLikeDirectAssetURL(parsed *url.URL) bool {
	if parsed == nil {
		return true
	}
	if strings.EqualFold(parsed.Hostname(), "finder.video.qq.com") &&
		strings.Contains(strings.ToLower(parsed.EscapedPath()), "/stodownload") {
		return true
	}
	switch strings.ToLower(path.Ext(parsed.EscapedPath())) {
	case ".aac", ".avi", ".bmp", ".csv", ".doc", ".docx", ".epub", ".flac", ".flv", ".gif", ".jpeg", ".jpg", ".json", ".m3u8", ".m4a", ".m4s", ".mkv", ".mov", ".mp3", ".mp4", ".pdf", ".png", ".ppt", ".pptx", ".rar", ".ts", ".wav", ".webm", ".webp", ".xls", ".xlsx", ".xml", ".zip":
		return true
	default:
		return false
	}
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(strings.TrimSuffix(name, filepath.Ext(name)))
	if name == "" {
		return "webpage"
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range name {
		invalid := r < 0x20 || strings.ContainsRune(`/\:*?"<>|`, r)
		if invalid {
			if !lastUnderscore {
				b.WriteRune('_')
				lastUnderscore = true
			}
			continue
		}
		b.WriteRune(r)
		lastUnderscore = false
	}
	out := strings.Trim(strings.TrimSpace(b.String()), ".")
	if out == "" {
		return "webpage"
	}
	runes := []rune(out)
	if len(runes) > 120 {
		out = string(runes[:120])
	}
	return out
}

func inlineHTMLURL(contentID string) string {
	if strings.TrimSpace(contentID) == "" {
		return "inline-html://" + PlatformID
	}
	return "inline-html://" + PlatformID + "/" + url.PathEscape(contentID)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func cloneAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+4)
	for k, v := range in {
		out[k] = v
	}
	return out
}
