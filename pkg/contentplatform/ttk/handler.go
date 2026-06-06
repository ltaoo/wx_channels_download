package ttk

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/novelutil"
)

const (
	PlatformID = "ttk"
	baseURL    = "https://ttks.tw"
)

type Novel struct {
	Title    string    `json:"title"`
	URL      string    `json:"url"`
	Author   string    `json:"author,omitempty"`
	Chapters []Chapter `json:"chapters,omitempty"`
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

type Fetcher interface {
	FetchNovelChapters(url string) (*Novel, error)
	FetchChapterContent(url string) (*ChapterContent, error)
}

type Handler struct {
	Fetcher Fetcher
}

type parsedURL struct {
	Kind      string
	ID        string
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
	if parts.Kind == "chapter" {
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
	novel, err := h.Fetcher.FetchNovelChapters(parts.Canonical)
	if err != nil {
		return nil, fmt.Errorf("fetch ttk novel: %w", err)
	}
	novel.URL = novelutil.FirstNonEmpty(novel.URL, parts.Canonical)
	contentID := novelutil.FirstNonEmpty(parts.ID, slugFromURL(parts.Canonical))
	title := novelutil.FirstNonEmpty(novel.Title, "ttk_"+contentID)
	bodyHTML := novelutil.RenderBookHTML("TTK", novelutil.Book{
		Title:    title,
		URL:      parts.Canonical,
		Author:   novel.Author,
		BookID:   contentID,
		Chapters: ttkChapters(novel.Chapters),
	})
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: parts.Canonical,
		ContentID:    contentID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:       PlatformID,
			Type:           "novel",
			ID:             contentID,
			Title:          title,
			Author:         novel.Author,
			URL:            parts.Canonical,
			SourceURL:      parts.Canonical,
			AuthorNickname: novel.Author,
		}, novel, map[string]any{
			"chapter_count": len(novel.Chapters),
			"source_url":    parts.Canonical,
		}, ProbeOutput{
			Format:       "html",
			ContentType:  "novel",
			Title:        title,
			SourceURL:    parts.Canonical,
			CanonicalURL: parts.Canonical,
			BodyHTML:     bodyHTML,
		}.Map()),
		Variants: []contentdownload.Variant{novelutil.HTMLVariant("目录 HTML", "novel")},
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: map[string]any{"novel": novel},
	}, nil
}

func (h *Handler) probeChapter(sourceURL string, parts parsedURL) (*contentdownload.Probe, error) {
	chapter, err := h.Fetcher.FetchChapterContent(parts.Canonical)
	if err != nil {
		return nil, fmt.Errorf("fetch ttk chapter: %w", err)
	}
	title := novelutil.FirstNonEmpty(chapter.Title, "ttk_"+parts.ID)
	bodyHTML := novelutil.RenderChapterHTML("TTK", title, parts.Canonical, chapter.Content)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: parts.Canonical,
		ContentID:    parts.ID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:  PlatformID,
			Type:      "chapter",
			ID:        parts.ID,
			Title:     title,
			URL:       parts.Canonical,
			SourceURL: parts.Canonical,
		}, chapter, map[string]any{
			"chapter_id": parts.ID,
			"source_url": parts.Canonical,
		}, ProbeOutput{
			Format:       "html",
			ContentType:  "chapter",
			Title:        title,
			SourceURL:    parts.Canonical,
			CanonicalURL: parts.Canonical,
			BodyHTML:     bodyHTML,
		}.Map()),
		Variants: []contentdownload.Variant{novelutil.HTMLVariant("章节 HTML", "chapter")},
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
	if len(segments) == 0 {
		return parsedURL{}, false
	}
	id := strings.TrimSuffix(segments[len(segments)-1], ".html")
	kind := "novel"
	if strings.HasSuffix(strings.ToLower(segments[len(segments)-1]), ".html") || strings.Contains(strings.ToLower(parsed.EscapedPath()), "/chapter/") {
		kind = "chapter"
	}
	return parsedURL{Kind: kind, ID: id, Canonical: baseURL + parsed.EscapedPath()}, true
}

type Client struct {
	HTTPClient *http.Client
	UserAgent  string
}

func NewClient(client *http.Client) *Client {
	if client == nil {
		client = &http.Client{}
	}
	return &Client{
		HTTPClient: client,
		UserAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
	}
}

func (c *Client) FetchNovelChapters(rawURL string) (*Novel, error) {
	htmlText, err := c.fetch(rawURL)
	if err != nil {
		return nil, err
	}
	return c.parseNovelChapters(htmlText, rawURL)
}

func (c *Client) FetchChapterContent(rawURL string) (*ChapterContent, error) {
	htmlText, err := c.fetch(rawURL)
	if err != nil {
		return nil, err
	}
	return c.parseChapterContent(htmlText)
}

func (c *Client) fetch(rawURL string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, rawURL)
	}
	return string(body), nil
}

func (c *Client) parseNovelChapters(htmlText string, pageURL string) (*Novel, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	novel := &Novel{URL: pageURL}
	novel.Title = strings.TrimSpace(doc.Find("h1").First().Text())
	if novel.Title == "" {
		novel.Title = strings.TrimSpace(doc.Find(".novel-title, .book-title, title").First().Text())
	}
	doc.Find(".author, .novel-author").Each(func(_ int, s *goquery.Selection) {
		if author := strings.TrimSpace(s.Text()); author != "" {
			novel.Author = author
		}
	})
	doc.Find("#chapters_frame .chapter_cell a, .chapters_frame .chapter_cell a, .chapter_cell a, .chapter-list a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		title := strings.TrimSpace(s.Text())
		if !exists || title == "" {
			return
		}
		novel.Chapters = append(novel.Chapters, Chapter{Index: i + 1, Title: title, URL: novelutil.NormalizeURL(href, pageURL, baseURL)})
	})
	if len(novel.Chapters) == 0 {
		return nil, fmt.Errorf("未找到章节列表")
	}
	return novel, nil
}

func (c *Client) parseChapterContent(htmlText string) (*ChapterContent, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	content := &ChapterContent{Title: strings.TrimSpace(doc.Find("h1").First().Text())}
	if content.Title == "" {
		content.Title = strings.TrimSpace(doc.Find(".chapter-title, .title").First().Text())
	}
	for _, selector := range []string{".chapter-content", ".content", ".novel-content", "#content", ".text-content", "article", ".article-content"} {
		sel := doc.Find(selector)
		if sel.Length() == 0 {
			continue
		}
		htmlContent, _ := sel.Html()
		text := htmlToText(htmlContent)
		if text != "" {
			content.Content = text
			break
		}
	}
	return content, nil
}

func ttkChapters(chapters []Chapter) []novelutil.Chapter {
	out := make([]novelutil.Chapter, 0, len(chapters))
	for _, chapter := range chapters {
		out = append(out, novelutil.Chapter{Index: chapter.Index, Title: chapter.Title, URL: chapter.URL})
	}
	return out
}

func slugFromURL(rawURL string) string {
	parts := strings.Split(strings.Trim(rawURL, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSuffix(parts[len(parts)-1], ".html")
}

func htmlToText(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = strings.ReplaceAll(s, "</p>", "\n")
	re := regexp.MustCompile(`<[^>]+>`)
	return strings.TrimSpace(re.ReplaceAllString(s, ""))
}
