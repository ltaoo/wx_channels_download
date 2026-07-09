package ttk

import (
	"context"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/novelutil"
)

const (
	PlatformID = "ttk"
	baseURL    = "https://ttks.tw"

	ContentTypeNovel   = "novel"
	ContentTypeChapter = "chapter"

	ArchiveProtocol          = "ttk_archive"
	archiveVariantID         = "html"
	archiveConcurrency       = 5
	metadataNovelFetchResult = "_novel_fetch_result"
)

var (
	defaultHeaders = http.Header{
		"Accept":                    []string{"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8"},
		"Accept-Language":           []string{"zh-CN,zh;q=0.9,en;q=0.8"},
		"Cache-Control":             []string{"no-cache"},
		"Pragma":                    []string{"no-cache"},
		"Sec-Fetch-Dest":            []string{"document"},
		"Sec-Fetch-Mode":            []string{"navigate"},
		"Sec-Fetch-Site":            []string{"same-origin"},
		"Sec-Fetch-User":            []string{"?1"},
		"Upgrade-Insecure-Requests": []string{"1"},
		"User-Agent":                []string{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"},
	}
	chapterHTMLBlockRE = regexp.MustCompile(`(?i)<\s*(br|/p|/div|/section|/article|/li|/h[1-6])\b[^>]*>`)
	htmlTagRE          = regexp.MustCompile(`(?s)<[^>]+>`)
	addBookcaseIDRE    = regexp.MustCompile(`(?:\?|&)bid=(\d+)`)
	imageBookIDRE      = regexp.MustCompile(`/article/image/\d+/(\d+)/`)
	unsafeTitleRE      = regexp.MustCompile(`[《》]`)
)

type Novel struct {
	Title            string    `json:"title"`
	URL              string    `json:"url"`
	Author           string    `json:"author,omitempty"`
	Category         string    `json:"category,omitempty"`
	Status           string    `json:"status,omitempty"`
	BookID           string    `json:"book_id,omitempty"`
	Description      string    `json:"description,omitempty"`
	CoverURL         string    `json:"cover_url,omitempty"`
	LatestChapter    string    `json:"latest_chapter,omitempty"`
	LatestChapterURL string    `json:"latest_chapter_url,omitempty"`
	ChapterCount     int       `json:"chapter_count,omitempty"`
	FullCatalogURL   string    `json:"full_catalog_url,omitempty"`
	Chapters         []Chapter `json:"chapters,omitempty"`
}

type Chapter struct {
	Index int    `json:"index"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type ChapterContent struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type NovelFetchResult struct {
	Novel                 *Novel `json:"novel,omitempty"`
	SourceURL             string `json:"source_url,omitempty"`
	SourceHTML            string `json:"-"`
	SourceNovel           *Novel `json:"source_novel,omitempty"`
	SourceParsedHTML      string `json:"source_parsed_html,omitempty"`
	FullCatalogURL        string `json:"full_catalog_url,omitempty"`
	FullCatalogHTML       string `json:"-"`
	FullCatalogNovel      *Novel `json:"full_catalog_novel,omitempty"`
	FullCatalogParsedHTML string `json:"full_catalog_parsed_html,omitempty"`
}

type ChapterFetchResult struct {
	Chapter    Chapter         `json:"chapter"`
	URL        string          `json:"url"`
	HTML       string          `json:"-"`
	Content    *ChapterContent `json:"content,omitempty"`
	ParsedHTML string          `json:"parsed_html,omitempty"`
	Error      string          `json:"error,omitempty"`
}

type NovelArchiveResult struct {
	Novel    *Novel               `json:"novel,omitempty"`
	Fetch    *NovelFetchResult    `json:"fetch,omitempty"`
	Chapters []ChapterFetchResult `json:"chapters,omitempty"`
}

type NovelArchiveOptions struct {
	Concurrency  int
	AllowPartial bool
	OnChapter    func(done int, total int, chapter ChapterFetchResult)
}

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
		fetcher = NewClient(nil)
	}
	return &Handler{Fetcher: fetcher}
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
	if parts.Kind == ContentTypeChapter {
		return h.probeChapter(input.URL, parts)
	}
	return h.probeNovel(input.URL, parts)
}

func (h *Handler) Resolve(ctx context.Context, input contentdownload.ResolveInput) (*contentdownload.ResolvedRequest, error) {
	sourceURL := novelutil.FirstNonEmpty(input.URL)
	if input.Probe != nil {
		sourceURL = novelutil.FirstNonEmpty(sourceURL, input.Probe.SourceURL, input.Probe.CanonicalURL)
	}
	parts, ok := ParseURL(sourceURL)
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	if parts.Kind == ContentTypeChapter {
		return novelutil.ResolveInlineHTML(ctx, PlatformID, input, h.Probe)
	}
	return h.resolveNovelArchive(ctx, input, parts)
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	if resolved != nil && strings.EqualFold(resolved.Download.Protocol, ArchiveProtocol) {
		return ttkArchivePlan(), nil
	}
	return novelutil.HTMLPlan(PlatformID), nil
}

func (h *Handler) probeNovel(sourceURL string, parts parsedURL) (*contentdownload.Probe, error) {
	var result *NovelFetchResult
	var novel *Novel
	var err error
	if fetcher, ok := h.Fetcher.(novelFetcher); ok {
		result, err = fetcher.FetchNovel(parts.Canonical)
		if err != nil {
			return nil, fmt.Errorf("fetch ttk novel: %w", err)
		}
		novel = result.Novel
	} else {
		novel, err = h.Fetcher.FetchNovelChapters(parts.Canonical)
		if err != nil {
			return nil, fmt.Errorf("fetch ttk novel: %w", err)
		}
	}
	if novel == nil {
		return nil, fmt.Errorf("ttk novel result is empty")
	}
	novel.BookID = novelutil.FirstNonEmpty(novel.BookID, parts.BookID)
	novel.URL = novelutil.FirstNonEmpty(novel.URL, parts.Canonical)
	novel.FullCatalogURL = novelutil.FirstNonEmpty(novel.FullCatalogURL, parts.Canonical)
	contentID := novelutil.FirstNonEmpty(novel.BookID, parts.BookID)
	title := novelutil.FirstNonEmpty(novel.Title, "ttk_"+contentID)
	description := novelutil.FirstNonEmpty(novel.Description, novelutil.Description(novel.Category, novel.Status))
	bodyHTML := BuildNovelHTML(novel)
	internal := map[string]any{"novel": novel}
	if result != nil {
		internal[metadataNovelFetchResult] = result
		internal["probe_pipeline"] = ttkProbePipeline(result)
	}
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: parts.Canonical,
		ContentID:    contentID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:       PlatformID,
			Type:           ContentTypeNovel,
			ID:             contentID,
			Title:          title,
			Description:    description,
			Author:         novel.Author,
			URL:            parts.Canonical,
			SourceURL:      parts.Canonical,
			AuthorNickname: novel.Author,
			CoverURL:       novel.CoverURL,
		}, novel, map[string]any{
			"book_id":            contentID,
			"category":           novel.Category,
			"status":             novel.Status,
			"latest_chapter":     novel.LatestChapter,
			"latest_chapter_url": novel.LatestChapterURL,
			"chapter_count":      len(novel.Chapters),
			"full_catalog_url":   novel.FullCatalogURL,
			"source_url":         parts.Canonical,
		}, ProbeOutput{
			Format:       "html",
			ContentType:  ContentTypeNovel,
			Title:        title,
			SourceURL:    parts.Canonical,
			CanonicalURL: parts.Canonical,
			BodyHTML:     bodyHTML,
		}.Map()),
		Variants: []contentdownload.Variant{ttkArchiveVariant()},
		Defaults: contentdownload.Defaults{VariantID: archiveVariantID},
		Internal: internal,
	}, nil
}

func (h *Handler) resolveNovelArchive(ctx context.Context, input contentdownload.ResolveInput, parts parsedURL) (*contentdownload.ResolvedRequest, error) {
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
	contentID := novelutil.FirstNonEmpty(probe.ContentID, summary.ID, parts.BookID)
	title := novelutil.FirstNonEmpty(summary.Title, contentID)
	sourceURL := novelutil.FirstNonEmpty(probe.SourceURL, input.URL, summary.SourceURL, probe.CanonicalURL, parts.Canonical)
	canonicalURL := novelutil.FirstNonEmpty(probe.CanonicalURL, summary.URL, sourceURL, parts.Canonical)
	filename := novelutil.FirstNonEmpty(input.Options.Filename, title, contentID)
	contentMetadata := cloneAnyMap(contentdownload.ContentMetadataOf(probe.Content))
	contentOutput := contentdownload.ContentOutputOf(probe.Content)
	metadata := cloneAnyMap(contentMetadata)
	metadata["variant_id"] = variant.ID
	metadata["content_type"] = ContentTypeNovel
	metadata["source_url"] = sourceURL
	metadata["canonical_url"] = canonicalURL
	metadata["archive_concurrency"] = archiveConcurrency
	if bodyHTML, _ := contentOutput["body_html"].(string); strings.TrimSpace(bodyHTML) != "" {
		metadata["body_html"] = bodyHTML
	}
	if result := ttkProbeNovelFetchResult(probe); result != nil {
		metadata["source_html_size"] = len(result.SourceHTML)
		metadata["full_catalog_html_size"] = len(result.FullCatalogHTML)
		if result.Novel != nil {
			metadata["chapter_count"] = len(result.Novel.Chapters)
		}
	}
	resolved := &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: canonicalURL,
		ContentID:    contentID,
		Title:        title,
		Filename:     filename,
		Suffix:       "",
		Download: contentdownload.DownloadSpec{
			URL:         "ttk-archive://" + contentID,
			Method:      "GET",
			Protocol:    ArchiveProtocol,
			Connections: archiveConcurrency,
		},
		Files: ttkArchiveFilesFromProbe(probe),
		Labels: map[string]string{
			"platform":     PlatformID,
			"id":           contentID,
			"title":        title,
			"key":          "0",
			"spec":         variant.Spec,
			"suffix":       "",
			"source_url":   canonicalURL,
			"content_type": ContentTypeNovel,
		},
		Metadata: metadata,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:       PlatformID,
			Type:           ContentTypeNovel,
			ID:             contentID,
			Title:          title,
			Description:    summary.Description,
			Author:         novelutil.FirstNonEmpty(summary.Author, summary.AuthorNickname),
			URL:            novelutil.FirstNonEmpty(summary.URL, canonicalURL),
			SourceURL:      novelutil.FirstNonEmpty(summary.SourceURL, canonicalURL, sourceURL),
			AuthorNickname: summary.AuthorNickname,
			CoverURL:       summary.CoverURL,
		}, contentdownload.ContentDataOf(probe.Content), contentMetadata, contentOutput),
		Internal: map[string]any{
			metadataNovelFetchResult: ttkProbeNovelFetchResult(probe),
		},
	}
	resolved.Pipeline = ttkArchivePlan()
	return resolved, nil
}

func (h *Handler) probeChapter(sourceURL string, parts parsedURL) (*contentdownload.Probe, error) {
	chapter, err := h.Fetcher.FetchChapterContent(parts.Canonical)
	if err != nil {
		return nil, fmt.Errorf("fetch ttk chapter: %w", err)
	}
	contentID := joinID(parts.BookID, parts.ChapterID)
	title := novelutil.FirstNonEmpty(chapter.Title, "ttk_"+contentID)
	bodyHTML := BuildChapterHTML(chapter, parts.Canonical)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: parts.Canonical,
		ContentID:    contentID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:  PlatformID,
			Type:      ContentTypeChapter,
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
			ContentType:  ContentTypeChapter,
			Title:        title,
			SourceURL:    parts.Canonical,
			CanonicalURL: parts.Canonical,
			BodyHTML:     bodyHTML,
		}.Map()),
		Variants: []contentdownload.Variant{novelutil.HTMLVariant("章节 HTML", ContentTypeChapter)},
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: map[string]any{"chapter": chapter},
	}, nil
}

func ParseURL(rawURL string) (parsedURL, bool) {
	parsed, ok := novelutil.IsHTTPHost(rawURL, "ttks.tw", "www.ttks.tw")
	if !ok {
		return parsedURL{}, false
	}
	segments := novelutil.SplitPath(parsed)
	if len(segments) < 3 {
		return parsedURL{}, false
	}
	if segments[0] == "novel" && segments[1] == "chapters" {
		bookID := strings.TrimSpace(segments[2])
		if bookID == "" {
			return parsedURL{}, false
		}
		if len(segments) == 3 {
			return parsedURL{
				Kind:      ContentTypeNovel,
				BookID:    bookID,
				Canonical: baseURL + "/novel/chapters/" + bookID + "/index.html",
			}, true
		}
		last := strings.ToLower(strings.TrimSpace(segments[len(segments)-1]))
		if last == "index.html" {
			return parsedURL{
				Kind:      ContentTypeNovel,
				BookID:    bookID,
				Canonical: baseURL + "/novel/chapters/" + bookID + "/index.html",
			}, true
		}
		if strings.HasSuffix(last, ".html") {
			chapterID := strings.TrimSuffix(segments[len(segments)-1], ".html")
			if chapterID == "" {
				return parsedURL{}, false
			}
			return parsedURL{
				Kind:      ContentTypeChapter,
				BookID:    bookID,
				ChapterID: chapterID,
				Canonical: baseURL + parsed.EscapedPath(),
			}, true
		}
	}
	return parsedURL{}, false
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type HTMLFetcher interface {
	FetchHTML(rawURL string, referer string, headers http.Header) (string, error)
}

type HTMLFetchSession interface {
	BeginHTMLFetchSession() (HTMLFetcher, func(error), error)
}

type Client struct {
	HTTPClient  HTTPClient
	HTMLFetcher HTMLFetcher
	UserAgent   string
}

func NewClient(client HTTPClient) *Client {
	if client == nil {
		client = &http.Client{}
	}
	return &Client{
		HTTPClient: client,
		UserAgent:  defaultHeaders.Get("User-Agent"),
	}
}

func NewClientWithHTMLFetcher(fetcher HTMLFetcher, userAgent string) *Client {
	c := NewClient(nil)
	c.HTMLFetcher = fetcher
	c.UserAgent = userAgent
	return c
}

func (c *Client) FetchNovelChapters(rawURL string) (*Novel, error) {
	result, err := c.FetchNovel(rawURL)
	if err != nil {
		return nil, err
	}
	return result.Novel, nil
}

func (c *Client) FetchNovel(rawURL string) (result *NovelFetchResult, err error) {
	sessionClient, done, err := c.beginHTMLFetchSession()
	if err != nil {
		return nil, err
	}
	if done != nil {
		defer func() { done(err) }()
	}
	parts, ok := ParseURL(rawURL)
	if !ok || parts.Kind != ContentTypeNovel {
		return nil, fmt.Errorf("unsupported ttk novel url")
	}
	htmlText, err := sessionClient.fetch(parts.Canonical, baseURL+"/")
	if err != nil {
		return nil, err
	}
	novel, err := ParseNovelHTML(parts.Canonical, htmlText)
	if err != nil {
		return nil, err
	}
	novel.BookID = novelutil.FirstNonEmpty(novel.BookID, parts.BookID)
	novel.URL = novelutil.FirstNonEmpty(novel.URL, parts.Canonical)
	if novel.FullCatalogURL == "" {
		novel.FullCatalogURL = parts.Canonical
	}
	result = &NovelFetchResult{
		Novel:            novel,
		SourceURL:        parts.Canonical,
		SourceHTML:       htmlText,
		SourceNovel:      cloneNovel(novel),
		SourceParsedHTML: BuildNovelHTML(novel),
	}
	if shouldFetchFullCatalog(parts.Canonical, novel) {
		fullHTML, fetchErr := sessionClient.fetch(novel.FullCatalogURL, parts.Canonical)
		if fetchErr != nil {
			return nil, fetchErr
		}
		fullNovel, parseErr := ParseNovelHTML(novel.FullCatalogURL, fullHTML)
		if parseErr != nil {
			return nil, parseErr
		}
		result.FullCatalogURL = novel.FullCatalogURL
		result.FullCatalogHTML = fullHTML
		result.FullCatalogNovel = cloneNovel(fullNovel)
		result.FullCatalogParsedHTML = BuildNovelHTML(fullNovel)
		mergeFullCatalog(novel, fullNovel)
		result.Novel = novel
	} else {
		result.FullCatalogURL = parts.Canonical
		result.FullCatalogHTML = htmlText
		result.FullCatalogNovel = cloneNovel(novel)
		result.FullCatalogParsedHTML = BuildNovelHTML(novel)
	}
	if len(novel.Chapters) == 0 {
		return nil, fmt.Errorf("未找到章节列表")
	}
	return result, nil
}

func (c *Client) FetchChapterContent(rawURL string) (*ChapterContent, error) {
	chapter, err := c.FetchChapter(rawURL)
	if err != nil {
		return nil, err
	}
	return chapter.Content, nil
}

func (c *Client) FetchChapter(rawURL string) (chapter *ChapterFetchResult, err error) {
	sessionClient, done, err := c.beginHTMLFetchSession()
	if err != nil {
		return nil, err
	}
	if done != nil {
		defer func() { done(err) }()
	}
	return sessionClient.fetchChapter(rawURL)
}

func (c *Client) FetchNovelArchive(rawURL string, seed *NovelFetchResult, options NovelArchiveOptions) (archive *NovelArchiveResult, err error) {
	sessionClient, done, err := c.beginHTMLFetchSession()
	if err != nil {
		return nil, err
	}
	if done != nil {
		defer func() { done(err) }()
	}

	result := seed
	if !novelFetchResultHasChapters(result) || strings.TrimSpace(result.SourceHTML) == "" {
		result, err = sessionClient.FetchNovel(rawURL)
		if err != nil {
			return nil, err
		}
	}
	chapters := result.Novel.Chapters
	if len(chapters) == 0 {
		return nil, fmt.Errorf("未找到章节列表")
	}
	concurrency := options.Concurrency
	if concurrency <= 0 {
		concurrency = archiveConcurrency
	}
	if concurrency > len(chapters) {
		concurrency = len(chapters)
	}

	items := make([]ChapterFetchResult, len(chapters))
	jobs := make(chan int)
	results := make(chan chapterArchiveFetchResult, len(chapters))
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				chapter := chapters[index]
				item, fetchErr := sessionClient.fetchChapter(chapter.URL)
				if fetchErr != nil {
					results <- chapterArchiveFetchResult{
						index: index,
						err:   fmt.Errorf("fetch chapter %d %q %s: %w", chapter.Index, chapter.Title, chapter.URL, fetchErr),
					}
					continue
				}
				item.Chapter = chapter
				item.URL = novelutil.FirstNonEmpty(item.URL, chapter.URL)
				if item.Content != nil {
					item.Content.Title = novelutil.FirstNonEmpty(item.Content.Title, chapter.Title)
					item.ParsedHTML = BuildChapterHTML(item.Content, item.URL)
				}
				results <- chapterArchiveFetchResult{index: index, item: *item}
			}
		}()
	}
	go func() {
		for index := range chapters {
			jobs <- index
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	doneCount := 0
	var firstErr error
	for result := range results {
		doneCount++
		if result.err != nil {
			if firstErr == nil {
				firstErr = result.err
			}
			result.item = ChapterFetchResult{
				Chapter: chapters[result.index],
				URL:     chapters[result.index].URL,
				Error:   result.err.Error(),
			}
			items[result.index] = result.item
			if options.OnChapter != nil {
				options.OnChapter(doneCount, len(chapters), result.item)
			}
			continue
		}
		items[result.index] = result.item
		if options.OnChapter != nil {
			options.OnChapter(doneCount, len(chapters), result.item)
		}
	}
	if firstErr != nil && !options.AllowPartial {
		return nil, firstErr
	}
	return &NovelArchiveResult{
		Novel:    cloneNovel(result.Novel),
		Fetch:    result,
		Chapters: items,
	}, nil
}

type chapterArchiveFetchResult struct {
	index int
	item  ChapterFetchResult
	err   error
}

func novelFetchResultHasChapters(result *NovelFetchResult) bool {
	return result != nil && result.Novel != nil && len(result.Novel.Chapters) > 0
}

func (c *Client) fetchChapter(rawURL string) (*ChapterFetchResult, error) {
	parts, ok := ParseURL(rawURL)
	if !ok || parts.Kind != ContentTypeChapter {
		return nil, fmt.Errorf("unsupported ttk chapter url")
	}
	referer := baseURL + "/novel/chapters/" + parts.BookID + "/index.html"
	htmlText, err := c.fetch(parts.Canonical, referer)
	if err != nil {
		return nil, err
	}
	content, err := ParseChapterHTML(htmlText)
	if err != nil {
		return nil, err
	}
	return &ChapterFetchResult{
		Chapter: Chapter{
			Title: content.Title,
			URL:   parts.Canonical,
		},
		URL:        parts.Canonical,
		HTML:       htmlText,
		Content:    content,
		ParsedHTML: BuildChapterHTML(content, parts.Canonical),
	}, nil
}

func (c *Client) beginHTMLFetchSession() (*Client, func(error), error) {
	if c == nil || c.HTMLFetcher == nil {
		return c, nil, nil
	}
	session, ok := c.HTMLFetcher.(HTMLFetchSession)
	if !ok {
		return c, nil, nil
	}
	fetcher, done, err := session.BeginHTMLFetchSession()
	if err != nil {
		return nil, nil, err
	}
	if fetcher == nil {
		return c, done, nil
	}
	clone := *c
	clone.HTMLFetcher = fetcher
	return &clone, done, nil
}

func (c *Client) fetch(rawURL string, referer string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header = c.requestHeaders(referer)
	if c != nil && c.HTMLFetcher != nil {
		return c.HTMLFetcher.FetchHTML(rawURL, referer, req.Header)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusForbidden {
			return "", fmt.Errorf("ttk HTTP 403: request was blocked before browser rendering: %s", strings.TrimSpace(string(body)))
		}
		return "", fmt.Errorf("ttk HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return string(body), nil
}

func (c *Client) requestHeaders(referer string) http.Header {
	headers := defaultHeaders.Clone()
	if c != nil && strings.TrimSpace(c.UserAgent) != "" {
		headers.Set("User-Agent", c.UserAgent)
	}
	if strings.TrimSpace(referer) != "" {
		headers.Set("Referer", referer)
	}
	return headers
}

func (c *Client) httpClient() HTTPClient {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) parseNovelChapters(htmlText string, pageURL string) (*Novel, error) {
	return ParseNovelHTML(pageURL, htmlText)
}

func ParseNovelHTML(pageURL string, htmlText string) (*Novel, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	novel := &Novel{URL: pageURL}
	novel.Title = cleanBookTitle(novelutil.FirstNonEmpty(
		metaContent(doc, "og:novel:book_name"),
		doc.Find(".novel_info h1").First().Text(),
		doc.Find("h1").First().Text(),
		doc.Find("title").First().Text(),
	))
	novel.Author = novelutil.FirstNonEmpty(metaContent(doc, "og:novel:author"), novelInfoValue(doc, "作者：", "作者:"))
	novel.Category = novelutil.FirstNonEmpty(metaContent(doc, "og:novel:category"), novelInfoValue(doc, "類別：", "类别：", "分类：", "類別:", "类别:", "分类:"))
	novel.Status = novelutil.FirstNonEmpty(metaContent(doc, "og:novel:status"), novelInfoValue(doc, "狀態：", "状态：", "狀態:", "状态:"))
	novel.Description = cleanText(novelutil.FirstNonEmpty(
		metaContent(doc, "description"),
		metaContent(doc, "og:description"),
		doc.Find(".description").First().Text(),
	))
	novel.CoverURL = cleanImageURL(novelutil.NormalizeURL(novelutil.FirstNonEmpty(
		metaContent(doc, "og:image"),
		doc.Find(".novel_info amp-img").First().AttrOr("src", ""),
		doc.Find(".novel_info img").First().AttrOr("src", ""),
	), pageURL, baseURL))
	novel.LatestChapter = metaContent(doc, "og:novel:latest_chapter_name")
	novel.LatestChapterURL = novelutil.NormalizeURL(metaContent(doc, "og:novel:latest_chapter_url"), pageURL, baseURL)
	novel.FullCatalogURL = novelutil.NormalizeURL(novelutil.FirstNonEmpty(
		metaContent(doc, "og:novel:read_url"),
		doc.Find(`link[rel="canonical"]`).First().AttrOr("href", ""),
		findCatalogLink(doc),
		pageURL,
	), pageURL, baseURL)
	novel.BookID = novelutil.FirstNonEmpty(extractBookIDFromDoc(doc), slugFromURL(novel.FullCatalogURL), slugFromURL(pageURL))
	novel.Chapters = parseChapterLinks(doc, pageURL)
	novel.ChapterCount = len(novel.Chapters)
	if novel.Title == "" && novel.BookID != "" {
		novel.Title = "ttk_" + novel.BookID
	}
	return novel, nil
}

func ParseChapterHTML(htmlText string) (*ChapterContent, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	content := &ChapterContent{Title: cleanChapterTitle(novelutil.FirstNonEmpty(
		doc.Find("h1").First().Text(),
		doc.Find(".chapter-title, .chapter_title, .title").First().Text(),
		doc.Find("title").First().Text(),
	))}
	for _, selector := range []string{
		".chapter-content",
		".chapter_content",
		".chapter_content_frame",
		".novel-content",
		".novel_content",
		"#content",
		".content",
		".text-content",
		"article",
		".article-content",
		"main",
	} {
		sel := doc.Find(selector).First()
		if sel.Length() == 0 {
			continue
		}
		htmlContent, _ := sel.Html()
		text := cleanChapterContent(htmlToText(htmlContent))
		if text != "" && !looksLikeNavigationOnly(text) {
			content.Content = text
			break
		}
	}
	if strings.TrimSpace(content.Content) == "" {
		return nil, fmt.Errorf("未找到章节正文")
	}
	return content, nil
}

func BuildNovelHTML(novel *Novel) string {
	if novel == nil {
		novel = &Novel{}
	}
	return novelutil.RenderBookHTML("天天看小说", novelutil.Book{
		Title:       novelutil.FirstNonEmpty(novel.Title, novel.BookID, "novel"),
		URL:         novel.URL,
		Author:      novel.Author,
		Category:    novel.Category,
		Status:      novel.Status,
		BookID:      novel.BookID,
		Description: novel.Description,
		CoverURL:    novel.CoverURL,
		Chapters:    ttkChapters(novel.Chapters),
	})
}

func BuildChapterHTML(chapter *ChapterContent, sourceURL string) string {
	if chapter == nil {
		chapter = &ChapterContent{}
	}
	return novelutil.RenderChapterHTML("天天看小说", chapter.Title, sourceURL, chapter.Content)
}

func parseChapterLinks(doc *goquery.Document, pageURL string) []Chapter {
	var best []Chapter
	doc.Find(".chapters_frame").Each(func(_ int, s *goquery.Selection) {
		chapters := parseChapterLinksInSelection(s, pageURL)
		if len(chapters) > len(best) {
			best = chapters
		}
	})
	if len(best) == 0 {
		best = parseChapterLinksInSelection(doc.Selection, pageURL)
	}
	for i := range best {
		best[i].Index = i + 1
	}
	return best
}

func parseChapterLinksInSelection(s *goquery.Selection, pageURL string) []Chapter {
	seen := map[string]bool{}
	var chapters []Chapter
	s.Find(".chapter_cell a, a").Each(func(_ int, link *goquery.Selection) {
		href, exists := link.Attr("href")
		title := cleanText(link.Text())
		if !exists || title == "" {
			return
		}
		chapterURL := novelutil.NormalizeURL(href, pageURL, baseURL)
		parts, ok := ParseURL(chapterURL)
		if !ok || parts.Kind != ContentTypeChapter || seen[parts.Canonical] {
			return
		}
		seen[parts.Canonical] = true
		chapters = append(chapters, Chapter{
			Index: len(chapters) + 1,
			Title: title,
			URL:   parts.Canonical,
		})
	})
	return chapters
}

func ttkChapters(chapters []Chapter) []novelutil.Chapter {
	out := make([]novelutil.Chapter, 0, len(chapters))
	for _, chapter := range chapters {
		out = append(out, novelutil.Chapter{Index: chapter.Index, Title: chapter.Title, URL: chapter.URL})
	}
	return out
}

func shouldFetchFullCatalog(sourceURL string, novel *Novel) bool {
	if novel == nil || strings.TrimSpace(novel.FullCatalogURL) == "" {
		return false
	}
	if strings.TrimRight(novel.FullCatalogURL, "/") == strings.TrimRight(sourceURL, "/") {
		return false
	}
	return true
}

func mergeFullCatalog(dst *Novel, full *Novel) {
	if dst == nil || full == nil {
		return
	}
	dst.Title = novelutil.FirstNonEmpty(dst.Title, full.Title)
	dst.Author = novelutil.FirstNonEmpty(dst.Author, full.Author)
	dst.Category = novelutil.FirstNonEmpty(dst.Category, full.Category)
	dst.Status = novelutil.FirstNonEmpty(dst.Status, full.Status)
	dst.Description = novelutil.FirstNonEmpty(dst.Description, full.Description)
	dst.CoverURL = novelutil.FirstNonEmpty(dst.CoverURL, full.CoverURL)
	dst.LatestChapter = novelutil.FirstNonEmpty(dst.LatestChapter, full.LatestChapter)
	dst.LatestChapterURL = novelutil.FirstNonEmpty(dst.LatestChapterURL, full.LatestChapterURL)
	if len(full.Chapters) > len(dst.Chapters) {
		dst.Chapters = full.Chapters
	}
	dst.ChapterCount = len(dst.Chapters)
}

func cloneNovel(in *Novel) *Novel {
	if in == nil {
		return nil
	}
	out := *in
	if len(in.Chapters) > 0 {
		out.Chapters = append([]Chapter(nil), in.Chapters...)
	}
	return &out
}

func metaContent(doc *goquery.Document, key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	if doc == nil || key == "" {
		return ""
	}
	value := ""
	doc.Find("meta").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		for _, attr := range []string{"property", "name"} {
			if strings.EqualFold(strings.TrimSpace(s.AttrOr(attr, "")), key) {
				value = strings.TrimSpace(s.AttrOr("content", ""))
				return false
			}
		}
		return true
	})
	return value
}

func novelInfoValue(doc *goquery.Document, labels ...string) string {
	if doc == nil {
		return ""
	}
	out := ""
	doc.Find(".novel_info li").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		text := strings.TrimSpace(s.Text())
		for _, label := range labels {
			if strings.Contains(text, label) {
				if linkText := strings.TrimSpace(s.Find("a").First().Text()); linkText != "" {
					out = linkText
				} else {
					out = strings.TrimSpace(strings.TrimPrefix(text, label))
				}
				return false
			}
		}
		return true
	})
	return cleanText(out)
}

func findCatalogLink(doc *goquery.Document) string {
	if doc == nil {
		return ""
	}
	out := ""
	doc.Find("a").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		href := strings.TrimSpace(s.AttrOr("href", ""))
		lower := strings.ToLower(href)
		if strings.Contains(lower, "/novel/chapters/") && strings.Contains(lower, "index.html") {
			out = href
			return false
		}
		return true
	})
	return out
}

func extractBookIDFromDoc(doc *goquery.Document) string {
	if doc == nil {
		return ""
	}
	out := ""
	doc.Find("a[href], amp-img[src], img[src], meta[content]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		for _, attr := range []string{"href", "src", "content"} {
			value := s.AttrOr(attr, "")
			if matches := addBookcaseIDRE.FindStringSubmatch(value); len(matches) > 1 {
				out = matches[1]
				return false
			}
			if matches := imageBookIDRE.FindStringSubmatch(value); len(matches) > 1 {
				out = matches[1]
				return false
			}
		}
		return true
	})
	return out
}

func slugFromURL(rawURL string) string {
	parts := strings.Split(strings.Trim(rawURL, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "chapters" && strings.TrimSpace(parts[i+1]) != "" {
			return strings.TrimSpace(parts[i+1])
		}
	}
	last := strings.TrimSuffix(parts[len(parts)-1], ".html")
	if last == "index" && len(parts) >= 2 {
		return strings.TrimSpace(parts[len(parts)-2])
	}
	return last
}

func cleanBookTitle(title string) string {
	title = cleanText(title)
	if strings.Contains(title, "》") {
		start := strings.Index(title, "《")
		end := strings.Index(title, "》")
		if start >= 0 && end > start {
			return cleanText(title[start+len("《") : end])
		}
	}
	if idx := strings.Index(title, " - "); idx > 0 {
		title = title[:idx]
	}
	return strings.TrimSpace(unsafeTitleRE.ReplaceAllString(title, ""))
}

func cleanChapterTitle(title string) string {
	title = cleanText(title)
	if idx := strings.Index(title, " - "); idx > 0 {
		title = title[:idx]
	}
	return strings.TrimSpace(title)
}

func cleanImageURL(rawURL string) string {
	if idx := strings.Index(rawURL, "?"); idx >= 0 {
		return rawURL[:idx]
	}
	return rawURL
}

func htmlToText(s string) string {
	s = chapterHTMLBlockRE.ReplaceAllString(s, "\n")
	s = htmlTagRE.ReplaceAllString(s, "")
	return cleanText(stdhtml.UnescapeString(s))
}

func cleanText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(stdhtml.UnescapeString(line)), " ")
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func cleanChapterContent(s string) string {
	lines := strings.Split(cleanText(s), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(line, "本網站分享到Facebook"):
			continue
		case strings.Contains(line, "添加書籤") || strings.Contains(line, "返回目錄") || strings.Contains(line, "章節報錯"):
			continue
		case strings.Contains(line, "天天看小說") && strings.Contains(line, "域名"):
			continue
		case strings.Contains(lower, "69ʰ") || strings.Contains(lower, "69hu") || strings.Contains(line, "６９"):
			continue
		default:
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func looksLikeNavigationOnly(s string) bool {
	lines := strings.Split(cleanText(s), "\n")
	if len(lines) > 8 {
		return false
	}
	text := strings.Join(lines, " ")
	return strings.Contains(text, "上一章") && strings.Contains(text, "下一章") && !strings.Contains(text, "本章完")
}

func ttkProbePipeline(result *NovelFetchResult) []map[string]any {
	if result == nil {
		return nil
	}
	nodes := make([]map[string]any, 0, 2)
	if result.SourceNovel != nil {
		nodes = append(nodes, map[string]any{
			"id":     "fetch_book_html",
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
			"label":  "获取全部章节页面 HTML",
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

func ttkArchiveVariant() contentdownload.Variant {
	return contentdownload.Variant{
		ID:    archiveVariantID,
		Type:  "archive",
		Label: "整本 HTML 文件夹",
		Metadata: map[string]any{
			"format":       "directory",
			"content_type": ContentTypeNovel,
			"concurrency":  archiveConcurrency,
		},
	}
}

func ttkArchivePlan() *contentdownload.PipelinePlan {
	return &contentdownload.PipelinePlan{
		Platform: PlatformID,
		Nodes: []contentdownload.PipelineNode{
			{
				ID:    "fetch_book_html",
				Type:  "fetch_html",
				Stage: "fetch",
				Args:  map[string]any{"source": "book"},
			},
			{
				ID:        "fetch_full_catalog",
				Type:      "fetch_full_catalog",
				Stage:     "fetch",
				DependsOn: []string{"fetch_book_html"},
				Args:      map[string]any{"source": "full_catalog"},
			},
			{
				ID:        "download",
				Type:      "download_ttk_archive",
				Stage:     "download",
				DependsOn: []string{"fetch_full_catalog"},
				Args: map[string]any{
					"chapter_source": "fetch_full_catalog",
					"concurrency":    archiveConcurrency,
				},
			},
			{
				ID:        "persist",
				Type:      "persist_artifacts",
				Stage:     "persist",
				DependsOn: []string{"download"},
			},
		},
		Metadata: map[string]any{
			"archive_protocol": ArchiveProtocol,
			"concurrency":      archiveConcurrency,
		},
	}
}

func ttkProbeNovelFetchResult(probe *contentdownload.Probe) *NovelFetchResult {
	if probe == nil || probe.Internal == nil {
		return nil
	}
	result, _ := probe.Internal[metadataNovelFetchResult].(*NovelFetchResult)
	return result
}

func ttkArchiveFilesFromProbe(probe *contentdownload.Probe) []contentdownload.FileNode {
	if result := ttkProbeNovelFetchResult(probe); result != nil {
		return ttkArchiveFilesFromSeed(result, contentdownload.FileNodeStatusPending)
	}
	if probe == nil || probe.Internal == nil {
		return nil
	}
	novel, _ := probe.Internal["novel"].(*Novel)
	return ttkArchiveFilesFromNovel(novel, nil, contentdownload.FileNodeStatusPending)
}

func cloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
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
