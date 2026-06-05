package novelutil

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"path"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
)

type Chapter struct {
	Index int
	Title string
	URL   string
}

type Book struct {
	Title       string
	URL         string
	Author      string
	Category    string
	Status      string
	BookID      string
	Description string
	CoverURL    string
	Tags        []string
	Chapters    []Chapter
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func NormalizeURL(href, pageURL, baseURL string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	if strings.HasPrefix(href, "/") {
		return strings.TrimRight(baseURL, "/") + href
	}
	base, err := url.Parse(pageURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return strings.TrimRight(pageURL, "/") + "/" + href
	}
	base.Path = path.Join(path.Dir(base.Path), href)
	base.RawQuery = ""
	base.Fragment = ""
	return base.String()
}

func IsHTTPHost(rawURL string, hosts ...string) (*url.URL, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, false
	}
	host := strings.ToLower(parsed.Hostname())
	for _, candidate := range hosts {
		if host == strings.ToLower(candidate) {
			return parsed, true
		}
	}
	return nil, false
}

func SplitPath(parsed *url.URL) []string {
	if parsed == nil {
		return nil
	}
	pathValue := strings.Trim(parsed.EscapedPath(), "/")
	if pathValue == "" {
		return nil
	}
	return strings.Split(pathValue, "/")
}

func HTMLVariant(label, contentType string) contentdownload.Variant {
	return contentdownload.Variant{
		ID:     "html",
		Type:   "html",
		Label:  FirstNonEmpty(label, "HTML"),
		Suffix: ".html",
		Metadata: map[string]any{
			"format":       "html",
			"content_type": contentType,
		},
	}
}

func HTMLPlan(platformID string) *contentdownload.PipelinePlan {
	return &contentdownload.PipelinePlan{
		Platform: platformID,
		Nodes: []contentdownload.PipelineNode{
			{ID: "download", Type: "download_asset", Stage: "download"},
			{ID: "persist", Type: "persist_artifacts", Stage: "persist", DependsOn: []string{"download"}},
		},
	}
}

func ResolveInlineHTML(ctx context.Context, platformID string, input contentdownload.ResolveInput, probeFn func(context.Context, contentdownload.ProbeInput) (*contentdownload.Probe, error)) (*contentdownload.ResolvedRequest, error) {
	probe := input.Probe
	if probe == nil {
		var err error
		probe, err = probeFn(ctx, contentdownload.ProbeInput{URL: input.URL, Extra: input.Extra})
		if err != nil {
			return nil, err
		}
	}
	variant, err := contentdownload.SelectVariant(probe, input.Options)
	if err != nil {
		return nil, err
	}
	summary := contentdownload.ContentSummaryOf(probe.Content)
	contentID := FirstNonEmpty(probe.ContentID, summary.ID)
	title := FirstNonEmpty(summary.Title, contentID)
	sourceURL := FirstNonEmpty(probe.SourceURL, input.URL, summary.SourceURL, probe.CanonicalURL)
	canonicalURL := FirstNonEmpty(probe.CanonicalURL, summary.URL, sourceURL)
	filename := FirstNonEmpty(input.Options.Filename, title, contentID)
	suffix := FirstNonEmpty(input.Options.Suffix, variant.Suffix, ".html")
	contentType := FirstNonEmpty(summary.Type, "html")
	contentMetadata := cloneAnyMap(contentdownload.ContentMetadataOf(probe.Content))
	contentOutput := contentdownload.ContentOutputOf(probe.Content)
	metadata := cloneAnyMap(contentMetadata)
	metadata["variant_id"] = variant.ID
	metadata["content_type"] = contentType
	metadata["source_url"] = sourceURL
	metadata["canonical_url"] = canonicalURL
	if bodyHTML, _ := contentOutput["body_html"].(string); strings.TrimSpace(bodyHTML) != "" {
		metadata["body_html"] = bodyHTML
	}

	resolved := &contentdownload.ResolvedRequest{
		Platform:     platformID,
		SourceURL:    sourceURL,
		CanonicalURL: canonicalURL,
		ContentID:    contentID,
		Title:        title,
		Filename:     filename,
		Suffix:       suffix,
		Download: contentdownload.DownloadSpec{
			URL:         inlineHTMLURL(platformID, contentID),
			Method:      "GET",
			Protocol:    "inline_html",
			Connections: 1,
		},
		Labels: map[string]string{
			"platform":     platformID,
			"id":           contentID,
			"title":        title,
			"key":          "0",
			"spec":         variant.Spec,
			"suffix":       suffix,
			"source_url":   canonicalURL,
			"content_type": contentType,
		},
		Metadata: metadata,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:        platformID,
			Type:            contentType,
			ID:              contentID,
			Title:           title,
			Description:     summary.Description,
			Author:          FirstNonEmpty(summary.Author, summary.AuthorNickname),
			URL:             FirstNonEmpty(summary.URL, canonicalURL),
			SourceURL:       FirstNonEmpty(summary.SourceURL, canonicalURL, sourceURL),
			AuthorNickname:  summary.AuthorNickname,
			AuthorAvatarURL: summary.AuthorAvatarURL,
			CoverURL:        summary.CoverURL,
			Duration:        summary.Duration,
		}, contentdownload.ContentDataOf(probe.Content), contentMetadata, contentOutput),
	}
	resolved.Pipeline = HTMLPlan(platformID)
	return resolved, nil
}

func RenderBookHTML(platform string, book Book) string {
	var b strings.Builder
	writeHTMLHead(&b, FirstNonEmpty(book.Title, book.BookID, "novel"))
	b.WriteString("<main>\n")
	b.WriteString("<h1>" + html.EscapeString(FirstNonEmpty(book.Title, book.BookID, "novel")) + "</h1>\n")
	b.WriteString("<dl>\n")
	writeTerm(&b, "平台", platform)
	writeTerm(&b, "作者", book.Author)
	writeTerm(&b, "分类", book.Category)
	writeTerm(&b, "状态", book.Status)
	writeTerm(&b, "来源", book.URL)
	writeTerm(&b, "章节数", fmt.Sprint(len(book.Chapters)))
	b.WriteString("</dl>\n")
	if strings.TrimSpace(book.Description) != "" {
		b.WriteString("<section><h2>简介</h2>\n")
		b.WriteString(TextToHTML(book.Description))
		b.WriteString("</section>\n")
	}
	if len(book.Tags) > 0 {
		b.WriteString("<section><h2>标签</h2><p>")
		for i, tag := range book.Tags {
			if i > 0 {
				b.WriteString(" / ")
			}
			b.WriteString(html.EscapeString(tag))
		}
		b.WriteString("</p></section>\n")
	}
	if len(book.Chapters) > 0 {
		b.WriteString("<section><h2>目录</h2><ol>\n")
		for _, chapter := range book.Chapters {
			title := FirstNonEmpty(chapter.Title, fmt.Sprintf("第 %d 章", chapter.Index))
			if chapter.URL != "" {
				b.WriteString(`<li><a href="` + html.EscapeString(chapter.URL) + `">` + html.EscapeString(title) + "</a></li>\n")
			} else {
				b.WriteString("<li>" + html.EscapeString(title) + "</li>\n")
			}
		}
		b.WriteString("</ol></section>\n")
	}
	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String()
}

func RenderChapterHTML(platform, title, sourceURL, content string) string {
	var b strings.Builder
	writeHTMLHead(&b, FirstNonEmpty(title, "chapter"))
	b.WriteString("<main>\n")
	b.WriteString("<h1>" + html.EscapeString(FirstNonEmpty(title, "chapter")) + "</h1>\n")
	b.WriteString("<dl>\n")
	writeTerm(&b, "平台", platform)
	writeTerm(&b, "来源", sourceURL)
	b.WriteString("</dl>\n")
	b.WriteString("<article>\n")
	b.WriteString(TextToHTML(content))
	b.WriteString("</article>\n")
	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String()
}

func TextToHTML(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var b strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		b.WriteString("<p>" + html.EscapeString(line) + "</p>\n")
	}
	return b.String()
}

func Description(values ...string) string {
	var parts []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, " / ")
}

func writeHTMLHead(b *strings.Builder, title string) {
	b.WriteString("<!doctype html>\n<html lang=\"zh-CN\">\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("<title>" + html.EscapeString(title) + "</title>\n")
	b.WriteString("<style>body{font-family:-apple-system,BlinkMacSystemFont,\"Segoe UI\",sans-serif;line-height:1.75;margin:0;background:#f7f7f5;color:#1f2328}main{max-width:860px;margin:0 auto;padding:32px 20px 56px;background:#fff;min-height:100vh}h1{font-size:28px;line-height:1.3;margin:0 0 20px}h2{font-size:20px;margin:28px 0 12px}dl{display:grid;grid-template-columns:max-content 1fr;gap:6px 14px;color:#4f5762}dt{font-weight:600}dd{margin:0;word-break:break-all}p{margin:0 0 14px}ol{padding-left:1.6em}li{margin:6px 0}a{color:#0b65c2;text-decoration:none}a:hover{text-decoration:underline}</style>\n")
	b.WriteString("</head>\n<body>\n")
}

func writeTerm(b *strings.Builder, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	b.WriteString("<dt>" + html.EscapeString(key) + "</dt><dd>" + html.EscapeString(value) + "</dd>\n")
}

func inlineHTMLURL(platformID, contentID string) string {
	if strings.TrimSpace(contentID) == "" {
		return "inline-html://" + platformID
	}
	return "inline-html://" + platformID + "/" + url.PathEscape(contentID)
}

func cloneAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+4)
	for k, v := range in {
		out[k] = v
	}
	return out
}
