package quanben

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/novelutil"
)

const (
	PlatformID = "quanben"
	baseURL    = "https://www.quanben.io"
)

var defaultHeaders = http.Header{
	"Accept":             []string{"*/*"},
	"Accept-Language":    []string{"zh-CN,zh;q=0.9,en;q=0.8"},
	"Cache-Control":      []string{"no-cache"},
	"Pragma":             []string{"no-cache"},
	"User-Agent":         []string{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"},
	"sec-ch-ua":          []string{`"Google Chrome";v="147", "Not.A/Brand";v="8", "Chromium";v="147"`},
	"sec-ch-ua-mobile":   []string{"?0"},
	"sec-ch-ua-platform": []string{`"macOS"`},
}

type Novel struct {
	Title    string    `json:"title"`
	URL      string    `json:"url"`
	Author   string    `json:"author,omitempty"`
	Category string    `json:"category,omitempty"`
	Status   string    `json:"status,omitempty"`
	BookID   string    `json:"book_id,omitempty"`
	Callback string    `json:"callback,omitempty"`
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
		return nil, fmt.Errorf("fetch quanben novel: %w", err)
	}
	novel.URL = novelutil.FirstNonEmpty(novel.URL, parts.Canonical)
	contentID := novelutil.FirstNonEmpty(novel.BookID, parts.ID)
	title := novelutil.FirstNonEmpty(novel.Title, "quanben_"+contentID)
	bodyHTML := novelutil.RenderBookHTML("全本小说网", novelutil.Book{
		Title:    title,
		URL:      parts.Canonical,
		Author:   novel.Author,
		Category: novel.Category,
		Status:   novel.Status,
		BookID:   contentID,
		Chapters: quanbenChapters(novel.Chapters),
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
			Description:    novelutil.Description(novel.Category, novel.Status),
			Author:         novel.Author,
			URL:            parts.Canonical,
			SourceURL:      parts.Canonical,
			AuthorNickname: novel.Author,
		}, novel, map[string]any{
			"book_id":       novel.BookID,
			"callback":      novel.Callback,
			"category":      novel.Category,
			"status":        novel.Status,
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
		return nil, fmt.Errorf("fetch quanben chapter: %w", err)
	}
	title := novelutil.FirstNonEmpty(chapter.Title, "quanben_"+parts.ID)
	bodyHTML := novelutil.RenderChapterHTML("全本小说网", title, parts.Canonical, chapter.Content)
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
	parsed, ok := novelutil.IsHTTPHost(rawURL, "quanben.io", "www.quanben.io")
	if !ok {
		return parsedURL{}, false
	}
	segments := novelutil.SplitPath(parsed)
	if len(segments) == 0 {
		return parsedURL{}, false
	}
	canonical := baseURL + parsed.EscapedPath()
	id := strings.TrimSuffix(segments[len(segments)-1], ".html")
	kind := "novel"
	if strings.HasSuffix(strings.ToLower(segments[len(segments)-1]), ".html") {
		kind = "chapter"
	}
	return parsedURL{Kind: kind, ID: id, Canonical: canonical}, true
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type pageFetcher interface {
	Fetch(url string) (string, error)
	FetchWithHeaders(url string, headers http.Header) (string, error)
}

type Client struct {
	fetcher pageFetcher
}

func NewClient() *Client {
	return &Client{fetcher: &httpClientImpl{client: &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}}}
}

func NewClientWithHTTP(client HTTPClient) *Client {
	return &Client{fetcher: &httpClientWrapper{client: client}}
}

type httpClientImpl struct {
	client HTTPClient
}

func (c *httpClientImpl) Fetch(rawURL string) (string, error) {
	return c.FetchWithHeaders(rawURL, nil)
}

func (c *httpClientImpl) FetchWithHeaders(rawURL string, extraHeaders http.Header) (string, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header = defaultHeaders.Clone()
	for k, v := range extraHeaders {
		req.Header[k] = v
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, rawURL)
	}
	return string(body), nil
}

type httpClientWrapper struct {
	client HTTPClient
}

func (w *httpClientWrapper) Fetch(rawURL string) (string, error) {
	return w.FetchWithHeaders(rawURL, nil)
}

func (w *httpClientWrapper) FetchWithHeaders(rawURL string, headers http.Header) (string, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header = defaultHeaders.Clone()
	for k, v := range headers {
		req.Header[k] = v
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, rawURL)
	}
	return string(body), nil
}

func (c *Client) FetchNovelChapters(rawURL string) (*Novel, error) {
	htmlText, err := c.fetcher.Fetch(rawURL)
	if err != nil {
		return nil, err
	}
	return c.parseNovelChapters(htmlText, rawURL)
}

func (c *Client) FetchChapterContent(rawURL string) (*ChapterContent, error) {
	htmlText, err := c.fetcher.Fetch(rawURL)
	if err != nil {
		return nil, err
	}
	return c.parseChapterContent(htmlText)
}

func (c *Client) parseNovelChapters(htmlText string, pageURL string) (*Novel, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	novel := &Novel{URL: pageURL}
	novel.Title = strings.TrimSpace(doc.Find("h1").First().Text())
	if novel.Title == "" {
		novel.Title = strings.TrimSpace(doc.Find(".book-title, title").First().Text())
	}
	doc.Find(".list2 p").Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		switch {
		case strings.HasPrefix(text, "作者:"):
			novel.Author = strings.TrimSpace(strings.TrimPrefix(text, "作者:"))
		case strings.HasPrefix(text, "类别:"):
			novel.Category = strings.TrimSpace(strings.TrimPrefix(text, "类别:"))
		case strings.HasPrefix(text, "状态:"):
			novel.Status = strings.TrimSpace(strings.TrimPrefix(text, "状态:"))
		}
	})
	doc.Find("ul.list3 li a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		title := strings.TrimSpace(s.Text())
		if !exists || title == "" {
			return
		}
		novel.Chapters = append(novel.Chapters, Chapter{Index: i + 1, Title: title, URL: novelutil.NormalizeURL(href, pageURL, baseURL)})
	})
	novel.BookID = extractBookID(htmlText)
	novel.Callback = extractCallback(htmlText)
	if chapters, err := c.fetchFullChapterList(pageURL, htmlText); err != nil {
		return nil, err
	} else if len(chapters) > 0 {
		novel.Chapters = chapters
	}
	for i := range novel.Chapters {
		novel.Chapters[i].Index = i + 1
	}
	if len(novel.Chapters) == 0 {
		return nil, fmt.Errorf("the chapters is empty")
	}
	return novel, nil
}

func (c *Client) fetchFullChapterList(pageURL string, htmlText string) ([]Chapter, error) {
	bookID := extractBookID(htmlText)
	callback := extractCallback(htmlText)
	if bookID == "" || callback == "" {
		return nil, nil
	}
	jsonpURL := baseURL + "/index.php?c=book&a=list.jsonp&callback=" + callback + "&book_id=" + bookID + "&b=" + url.QueryEscape(encodeBase64(callback))
	headers := http.Header{
		"Accept":     []string{"*/*"},
		"Referer":    []string{pageURL},
		"User-Agent": defaultHeaders.Values("User-Agent"),
	}
	respText, err := c.fetcher.FetchWithHeaders(jsonpURL, headers)
	if err != nil {
		return nil, err
	}
	if strings.Contains(respText, "参数错误") {
		return nil, fmt.Errorf("参数错误, resp: %s", respText)
	}
	content := jsonpContent(callback, respText)
	if content == "" {
		return nil, fmt.Errorf("no content field in jsonp response")
	}
	chapterRe := regexp.MustCompile(`<a\s+href="([^"]+)"\s+itemprop="url"><span\s+itemprop="name">([^<]+)</span></a>`)
	matches := chapterRe.FindAllStringSubmatch(content, -1)
	chapters := make([]Chapter, 0, len(matches))
	for i, match := range matches {
		if len(match) < 3 {
			continue
		}
		chapters = append(chapters, Chapter{
			Index: i + 1,
			Title: match[2],
			URL:   novelutil.NormalizeURL(match[1], pageURL, baseURL),
		})
	}
	return chapters, nil
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

func extractBookID(htmlText string) string {
	matches := regexp.MustCompile(`load_more\(['"](\d+)['"]\)`).FindStringSubmatch(htmlText)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func extractCallback(htmlText string) string {
	matches := regexp.MustCompile(`var callback=['"]([a-z0-9]+)['"]`).FindStringSubmatch(htmlText)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func encodeBase64(input string) string {
	staticchars := "PXhw7UT1B0a9kQDKZsjIASmOezxYG4CHo5Jyfg2b8FLpEvRr3WtVnlqMidu6cN"
	var encoded strings.Builder
	for i := 0; i < len(input); i++ {
		num0 := strings.Index(staticchars, string(input[i]))
		code := string(input[i])
		if num0 != -1 {
			code = string(staticchars[(num0+3)%62])
		}
		encoded.WriteByte(staticchars[rand.Int63n(62)])
		encoded.WriteString(code)
		encoded.WriteByte(staticchars[rand.Int63n(62)])
	}
	return encoded.String()
}

func jsonpContent(callback, respText string) string {
	if matches := regexp.MustCompile(`` + regexp.QuoteMeta(callback) + `\(\s*(\{[\s\S]*\})\s*\)`).FindStringSubmatch(respText); len(matches) < 2 {
		return ""
	}
	contentRe := regexp.MustCompile(`"content"\s*:\s*"(.*)"\s*,|\"content\"\s*:\s*\"(.*)\"\s*\}`)
	contentMatches := contentRe.FindStringSubmatch(respText)
	if len(contentMatches) < 2 {
		return ""
	}
	content := contentMatches[1]
	if content == "" && len(contentMatches) >= 3 {
		content = contentMatches[2]
	}
	content = strings.ReplaceAll(content, `\"`, `"`)
	content = strings.ReplaceAll(content, `\\n`, "\n")
	content = strings.ReplaceAll(content, `\\t`, "\t")
	content = strings.ReplaceAll(content, `\/`, "/")
	content = strings.ReplaceAll(content, `&lt;`, "<")
	content = strings.ReplaceAll(content, `&gt;`, ">")
	return content
}

func quanbenChapters(chapters []Chapter) []novelutil.Chapter {
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
	re := regexp.MustCompile(`<[^>]+>`)
	return strings.TrimSpace(re.ReplaceAllString(s, ""))
}
