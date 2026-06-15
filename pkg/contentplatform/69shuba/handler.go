package shuba69

import (
	"context"
	"fmt"
	"strings"

	shubapkg "wx_channel/pkg/69shuba"
	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/novelutil"
)

const PlatformID = shubapkg.PlatformID

type Fetcher interface {
	FetchNovelChapters(url string) (*Novel, error)
	FetchChapterContent(url string) (*ChapterContent, error)
}

type novelFetcher interface {
	FetchNovel(url string) (*NovelFetchResult, error)
}

type Handler struct {
	Fetcher Fetcher
}

type parsedURL struct {
	Kind      string
	BookID    string
	ChapterID string
	Canonical string
}

func New(fetcher Fetcher) *Handler {
	if fetcher == nil {
		fetcher = NewClient()
	}
	return &Handler{Fetcher: fetcher}
}

func NewClient() *Client {
	return shubapkg.NewClient(nil)
}

func NewClientWithHTTP(client HTTPClient) *Client {
	return shubapkg.NewClient(client)
}

func NewClientWithOptions(cookie, userAgent string) *Client {
	return shubapkg.NewClientWithOptions(nil, cookie, userAgent)
}

func NewClientWithHTMLFetcher(fetcher HTMLFetcher, cookie, userAgent string) *Client {
	return shubapkg.NewClientWithHTMLFetcher(fetcher, cookie, userAgent)
}

func NewCDPFetcher(endpoint string) *CDPFetcher {
	return shubapkg.NewCDPFetcher(endpoint)
}

func NewSandboxCDPFetcher(apiBaseURL string, sandboxID string) *SandboxCDPFetcher {
	return shubapkg.NewSandboxCDPFetcher(apiBaseURL, sandboxID)
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	_, ok := ParseURL(rawURL)
	return ok
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	parts, ok := ParseURL(input.URL)
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	if parts.Kind == shubapkg.ContentTypeChapter {
		return h.probeChapter(input.URL, parts)
	}
	return h.probeNovel(input.URL, parts)
}

func (h *Handler) Resolve(ctx context.Context, input contentdownload.ResolveInput) (*contentdownload.ResolvedRequest, error) {
	return novelutil.ResolveInlineHTML(ctx, PlatformID, input, h.Probe)
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	return novelutil.HTMLPlan(PlatformID), nil
}

func (h *Handler) probeNovel(sourceURL string, parts parsedURL) (*contentdownload.Probe, error) {
	var result *NovelFetchResult
	var novel *Novel
	var err error
	if fetcher, ok := h.Fetcher.(novelFetcher); ok {
		result, err = fetcher.FetchNovel(parts.Canonical)
		if err != nil {
			return nil, fmt.Errorf("fetch 69shuba novel: %w", err)
		}
		novel = result.Novel
	} else {
		novel, err = h.Fetcher.FetchNovelChapters(parts.Canonical)
		if err != nil {
			return nil, fmt.Errorf("fetch 69shuba novel: %w", err)
		}
	}
	novel.BookID = novelutil.FirstNonEmpty(novel.BookID, parts.BookID)
	novel.URL = novelutil.FirstNonEmpty(novel.URL, parts.Canonical)
	contentID := novelutil.FirstNonEmpty(novel.BookID, parts.BookID)
	title := novelutil.FirstNonEmpty(novel.Title, "69shuba_"+contentID)
	description := novelutil.FirstNonEmpty(novel.Description, novelutil.Description(novel.Category, novel.Status))
	bodyHTML := novelutil.RenderBookHTML("69书吧", novelutil.Book{
		Title:       title,
		URL:         novelutil.FirstNonEmpty(novel.URL, parts.Canonical),
		Author:      novel.Author,
		Category:    novel.Category,
		Status:      novel.Status,
		BookID:      contentID,
		Description: description,
		CoverURL:    novel.CoverURL,
		Tags:        novel.Tags,
		Chapters:    shubaChapters(novel.Chapters),
	})
	internal := map[string]any{"novel": novel}
	if result != nil {
		internal["probe_pipeline"] = shubaProbePipeline(result)
	}
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: parts.Canonical,
		ContentID:    contentID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:       PlatformID,
			Type:           shubapkg.ContentTypeNovel,
			ID:             contentID,
			Title:          title,
			Description:    description,
			Author:         novel.Author,
			URL:            parts.Canonical,
			SourceURL:      parts.Canonical,
			AuthorNickname: novel.Author,
			CoverURL:       novel.CoverURL,
		}, novel, map[string]any{
			"book_id":          contentID,
			"category":         novel.Category,
			"status":           novel.Status,
			"word_count":       novel.WordCount,
			"update_time":      novel.UpdateTime,
			"latest_chapter":   novel.LatestChapter,
			"chapter_count":    len(novel.Chapters),
			"full_catalog_url": novel.FullCatalogURL,
			"source_url":       parts.Canonical,
		}, ProbeOutput{
			Format:       "html",
			ContentType:  shubapkg.ContentTypeNovel,
			Title:        title,
			SourceURL:    parts.Canonical,
			CanonicalURL: parts.Canonical,
			BodyHTML:     bodyHTML,
		}.Map()),
		Variants: []contentdownload.Variant{
			novelutil.HTMLVariant("目录 HTML", shubapkg.ContentTypeNovel),
		},
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: internal,
	}, nil
}

func (h *Handler) probeChapter(sourceURL string, parts parsedURL) (*contentdownload.Probe, error) {
	chapter, err := h.Fetcher.FetchChapterContent(parts.Canonical)
	if err != nil {
		return nil, fmt.Errorf("fetch 69shuba chapter: %w", err)
	}
	contentID := novelutil.FirstNonEmpty(joinID(parts.BookID, parts.ChapterID), parts.ChapterID)
	title := novelutil.FirstNonEmpty(chapter.Title, "69shuba_"+contentID)
	bodyHTML := novelutil.RenderChapterHTML("69书吧", title, parts.Canonical, chapter.Content)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: parts.Canonical,
		ContentID:    contentID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:  PlatformID,
			Type:      shubapkg.ContentTypeChapter,
			ID:        contentID,
			Title:     title,
			URL:       parts.Canonical,
			SourceURL: parts.Canonical,
		}, chapter, map[string]any{
			"book_id":    parts.BookID,
			"chapter_id": parts.ChapterID,
			"source_url": parts.Canonical,
		}, ProbeOutput{
			Format:       "html",
			ContentType:  shubapkg.ContentTypeChapter,
			Title:        title,
			SourceURL:    parts.Canonical,
			CanonicalURL: parts.Canonical,
			BodyHTML:     bodyHTML,
		}.Map()),
		Variants: []contentdownload.Variant{
			novelutil.HTMLVariant("章节 HTML", shubapkg.ContentTypeChapter),
		},
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: map[string]any{"chapter": chapter},
	}, nil
}

func ParseURL(rawURL string) (parsedURL, bool) {
	parts, ok := shubapkg.ParseURL(rawURL)
	if !ok {
		return parsedURL{}, false
	}
	return parsedURL{
		Kind:      parts.Kind,
		BookID:    parts.BookID,
		ChapterID: parts.ChapterID,
		Canonical: parts.Canonical,
	}, true
}

func shubaChapters(chapters []Chapter) []novelutil.Chapter {
	out := make([]novelutil.Chapter, 0, len(chapters))
	for _, chapter := range chapters {
		out = append(out, novelutil.Chapter{Index: chapter.Index, Title: chapter.Title, URL: chapter.URL})
	}
	return out
}

func shubaProbePipeline(result *NovelFetchResult) []map[string]any {
	if result == nil {
		return nil
	}
	nodes := make([]map[string]any, 0, 2)
	if result.SourceNovel != nil {
		nodes = append(nodes, map[string]any{
			"id":     "fetch_source_html",
			"type":   "fetch_html",
			"label":  "获取书籍页面 HTML",
			"status": "completed",
			"output": map[string]any{
				"url":           result.SourceURL,
				"html_size":     len(result.SourceHTML),
				"title":         result.SourceNovel.Title,
				"chapter_count": len(result.SourceNovel.Chapters),
				"body_html":     result.SourceParsedHTML,
			},
		})
	}
	if result.FullCatalogNovel != nil {
		nodes = append(nodes, map[string]any{
			"id":     "fetch_full_catalog",
			"type":   "fetch_full_catalog",
			"label":  "获取完整目录页面 HTML",
			"status": "completed",
			"output": map[string]any{
				"url":           result.FullCatalogURL,
				"html_size":     len(result.FullCatalogHTML),
				"title":         result.FullCatalogNovel.Title,
				"chapter_count": len(result.FullCatalogNovel.Chapters),
				"body_html":     result.FullCatalogParsedHTML,
			},
		})
	}
	return nodes
}

func joinID(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, strings.TrimSpace(part))
		}
	}
	return strings.Join(out, "_")
}
