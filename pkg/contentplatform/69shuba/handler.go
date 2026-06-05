package shuba69

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/novelutil"
)

const (
	PlatformID = "69shuba"
	baseURL    = "https://www.69shuba.com"
)

var defaultHeaders = http.Header{
	"Accept":                    []string{"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
	"Accept-Language":           []string{"zh-CN,zh;q=0.9,en;q=0.8"},
	"Cache-Control":             []string{"max-age=0"},
	"Upgrade-Insecure-Requests": []string{"1"},
	"User-Agent":                []string{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"},
}

type Novel struct {
	Title    string    `json:"title"`
	URL      string    `json:"url"`
	Author   string    `json:"author,omitempty"`
	Category string    `json:"category,omitempty"`
	Status   string    `json:"status,omitempty"`
	BookID   string    `json:"book_id,omitempty"`
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
		return nil, fmt.Errorf("fetch 69shuba novel: %w", err)
	}
	novel.BookID = novelutil.FirstNonEmpty(novel.BookID, parts.BookID)
	novel.URL = novelutil.FirstNonEmpty(novel.URL, parts.Canonical)
	title := novelutil.FirstNonEmpty(novel.Title, "69shuba_"+novel.BookID)
	bodyHTML := novelutil.RenderBookHTML("69书吧", novelutil.Book{
		Title:    title,
		URL:      parts.Canonical,
		Author:   novel.Author,
		Category: novel.Category,
		Status:   novel.Status,
		BookID:   novel.BookID,
		Chapters: shubaChapters(novel.Chapters),
	})
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: parts.Canonical,
		ContentID:    novel.BookID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:       PlatformID,
			Type:           "novel",
			ID:             novel.BookID,
			Title:          title,
			Description:    novelutil.Description(novel.Category, novel.Status),
			Author:         novel.Author,
			URL:            parts.Canonical,
			SourceURL:      parts.Canonical,
			AuthorNickname: novel.Author,
		}, novel, map[string]any{
			"book_id":       novel.BookID,
			"category":      novel.Category,
			"status":        novel.Status,
			"chapter_count": len(novel.Chapters),
			"source_url":    parts.Canonical,
		}, map[string]any{
			"format":        "html",
			"content_type":  "novel",
			"title":         title,
			"source_url":    parts.Canonical,
			"canonical_url": parts.Canonical,
			"body_html":     bodyHTML,
		}),
		Variants: []contentdownload.Variant{
			novelutil.HTMLVariant("目录 HTML", "novel"),
		},
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: map[string]any{"novel": novel},
	}, nil
}

func (h *Handler) probeChapter(sourceURL string, parts parsedURL) (*contentdownload.Probe, error) {
	chapter, err := h.Fetcher.FetchChapterContent(parts.Canonical)
	if err != nil {
		return nil, fmt.Errorf("fetch 69shuba chapter: %w", err)
	}
	contentID := novelutil.FirstNonEmpty(parts.BookID+"_"+parts.ChapterID, parts.ChapterID)
	title := novelutil.FirstNonEmpty(chapter.Title, "69shuba_"+contentID)
	bodyHTML := novelutil.RenderChapterHTML("69书吧", title, parts.Canonical, chapter.Content)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: parts.Canonical,
		ContentID:    contentID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:  PlatformID,
			Type:      "chapter",
			ID:        contentID,
			Title:     title,
			URL:       parts.Canonical,
			SourceURL: parts.Canonical,
		}, chapter, map[string]any{
			"book_id":    parts.BookID,
			"chapter_id": parts.ChapterID,
			"source_url": parts.Canonical,
		}, map[string]any{
			"format":        "html",
			"content_type":  "chapter",
			"title":         title,
			"source_url":    parts.Canonical,
			"canonical_url": parts.Canonical,
			"body_html":     bodyHTML,
		}),
		Variants: []contentdownload.Variant{
			novelutil.HTMLVariant("章节 HTML", "chapter"),
		},
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: map[string]any{"chapter": chapter},
	}, nil
}

func ParseURL(rawURL string) (parsedURL, bool) {
	parsed, ok := novelutil.IsHTTPHost(rawURL, "69shuba.com", "www.69shuba.com", "m.69shuba.com")
	if !ok {
		return parsedURL{}, false
	}
	segments := novelutil.SplitPath(parsed)
	if len(segments) >= 2 && segments[0] == "book" {
		canonical := baseURL + "/book/" + segments[1] + "/"
		if len(segments) >= 3 {
			return parsedURL{Kind: "chapter", BookID: segments[1], ChapterID: strings.TrimSuffix(segments[2], ".htm"), Canonical: canonical + segments[2]}, true
		}
		return parsedURL{Kind: "novel", BookID: segments[1], Canonical: canonical}, true
	}
	if len(segments) >= 3 && segments[0] == "txt" {
		return parsedURL{
			Kind:      "chapter",
			BookID:    segments[1],
			ChapterID: strings.TrimSuffix(segments[2], ".htm"),
			Canonical: baseURL + "/txt/" + segments[1] + "/" + segments[2],
		}, true
	}
	return parsedURL{}, false
}

type Client struct {
	httpClient *http.Client
	cookie     string
	userAgent  string
}

func NewClient() *Client {
	return &Client{httpClient: &http.Client{}}
}

func NewClientWithHTTP(client *http.Client) *Client {
	if client == nil {
		client = &http.Client{}
	}
	return &Client{httpClient: client}
}

func NewClientWithOptions(cookie, userAgent string) *Client {
	return &Client{httpClient: &http.Client{}, cookie: cookie, userAgent: userAgent}
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
	req.Header = defaultHeaders.Clone()
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, rawURL)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	encoding, _, _ := charset.DetermineEncoding(body, resp.Header.Get("Content-Type"))
	decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(body), encoding.NewDecoder()))
	if err == nil {
		return string(decoded), nil
	}
	if isGBK(body) {
		return decodeGBK(body)
	}
	return string(body), nil
}

func (c *Client) parseNovelChapters(htmlText string, pageURL string) (*Novel, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	novel := &Novel{URL: pageURL}
	if matches := regexp.MustCompile(`/book/(\d+)`).FindStringSubmatch(pageURL); len(matches) > 1 {
		novel.BookID = matches[1]
	}
	novel.Title = strings.TrimSpace(doc.Find("h1").First().Text())
	if novel.Title == "" {
		novel.Title = strings.TrimSpace(doc.Find(".bread-crumbs a").Last().Text())
	}
	doc.Find(".bookinfo .booknav2 p, .bookinfo .booknav2 span").Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		switch {
		case strings.Contains(text, "作者"):
			novel.Author = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(text, "作者："), "作者:"))
			if a := s.Find("a"); a.Length() > 0 {
				novel.Author = strings.TrimSpace(a.Text())
			}
		case strings.Contains(text, "分类") || strings.Contains(text, "类型"):
			novel.Category = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(text, "分类："), "类型："))
			if a := s.Find("a"); a.Length() > 0 {
				novel.Category = strings.TrimSpace(a.Text())
			}
		case strings.Contains(text, "状态"):
			novel.Status = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(text, "状态："), "状态:"))
		}
	})
	for _, selector := range []string{".mu_contain ul li a", "#chapterlist a", ".mulu_list li a", ".catalog li a", "#catalog ul li a"} {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			title := strings.TrimSpace(s.Text())
			if !exists || title == "" {
				return
			}
			novel.Chapters = append(novel.Chapters, Chapter{Index: i + 1, Title: title, URL: novelutil.NormalizeURL(href, pageURL, baseURL)})
		})
		if len(novel.Chapters) > 0 {
			break
		}
	}
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
	content := &ChapterContent{
		Title: strings.TrimSpace(doc.Find("h1").First().Text()),
	}
	if content.Title == "" {
		content.Title = strings.TrimSpace(doc.Find(".txtnav h1, .chapter-title").First().Text())
	}
	for _, selector := range []string{".txtnav", "#chaptercontent", ".novelcontent", "#novelcontent", ".content", "#content"} {
		sel := doc.Find(selector)
		if sel.Length() == 0 {
			continue
		}
		sel.Find("script, .ads, .ad, #ads, .txtright, .bottom-ad, h1").Remove()
		htmlContent, _ := sel.Html()
		text := htmlToText(htmlContent)
		text = cleanContent(text)
		if text != "" {
			content.Content = text
			break
		}
	}
	return content, nil
}

func shubaChapters(chapters []Chapter) []novelutil.Chapter {
	out := make([]novelutil.Chapter, 0, len(chapters))
	for _, chapter := range chapters {
		out = append(out, novelutil.Chapter{Index: chapter.Index, Title: chapter.Title, URL: chapter.URL})
	}
	return out
}

func htmlToText(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = strings.ReplaceAll(s, "</p>", "\n")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	re := regexp.MustCompile(`<[^>]+>`)
	return strings.TrimSpace(re.ReplaceAllString(s, ""))
}

func cleanContent(s string) string {
	lines := strings.Split(s, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" ||
			strings.Contains(line, "69shuba") ||
			strings.Contains(line, "69书吧") ||
			(strings.Contains(line, "最新章节") && strings.Contains(line, "地址")) {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}

func isGBK(data []byte) bool {
	gbkCount := 0
	for i := 0; i < len(data)-1; i++ {
		if data[i] >= 0x81 && data[i] <= 0xFE && data[i+1] >= 0x40 && data[i+1] <= 0xFE {
			gbkCount++
			i++
		}
	}
	return gbkCount > 10
}

func decodeGBK(data []byte) (string, error) {
	reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewDecoder())
	result, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(result), nil
}
