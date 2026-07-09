package novelsource

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

const defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"

type Client struct {
	Source     Source
	HTTPClient HTTPClient
	Cookie     string
	UserAgent  string
}

func NewClient(source Source, client HTTPClient) *Client {
	if client == nil {
		client = http.DefaultClient
	}
	return &Client{
		Source:     source.normalized(),
		HTTPClient: client,
		UserAgent:  defaultUserAgent,
	}
}

func NewClientWithOptions(source Source, client HTTPClient, cookie, userAgent string) *Client {
	c := NewClient(source, client)
	c.Cookie = strings.TrimSpace(cookie)
	if strings.TrimSpace(userAgent) != "" {
		c.UserAgent = strings.TrimSpace(userAgent)
	}
	return c
}

func (c *Client) FetchNovelChapters(rawURL string) (*Novel, error) {
	result, err := c.FetchNovel(rawURL)
	if err != nil {
		return nil, err
	}
	return result.Novel, nil
}

func (c *Client) FetchNovel(rawURL string) (*NovelFetchResult, error) {
	if c == nil {
		return nil, fmt.Errorf("novel source client is nil")
	}
	source := c.Source.normalized()
	parts, ok := source.ParseURL(rawURL)
	if !ok || parts.Kind != ContentTypeNovel {
		return nil, fmt.Errorf("unsupported %s novel url: %s", source.ID, rawURL)
	}
	htmlText, err := c.fetch(parts.Canonical, source.BaseURL+"/")
	if err != nil {
		return nil, err
	}
	novel, err := source.ParseNovelHTML(parts.Canonical, htmlText)
	if err != nil {
		return nil, err
	}
	novel.BookID = firstNonEmpty(novel.BookID, parts.BookID)
	novel.URL = firstNonEmpty(novel.URL, parts.Canonical)
	result := &NovelFetchResult{
		Novel:            novel,
		SourceURL:        parts.Canonical,
		SourceHTML:       htmlText,
		SourceNovel:      cloneNovel(novel),
		SourceParsedHTML: BuildNovelHTML(source, novel),
	}
	catalogURL := firstNonEmpty(novel.FullCatalogURL, source.CatalogURL(novel.BookID))
	if shouldFetchCatalog(parts.Canonical, catalogURL, novel) {
		fullHTML, fetchErr := c.fetch(catalogURL, parts.Canonical)
		if fetchErr != nil {
			return nil, fetchErr
		}
		fullNovel, parseErr := source.ParseNovelHTML(catalogURL, fullHTML)
		if parseErr != nil {
			return nil, parseErr
		}
		if len(fullNovel.Chapters) > 0 {
			result.FullCatalogURL = catalogURL
			result.FullCatalogHTML = fullHTML
			result.FullCatalogNovel = cloneNovel(fullNovel)
			result.FullCatalogParsedHTML = BuildNovelHTML(source, fullNovel)
			mergeFullCatalog(novel, fullNovel)
			result.Novel = novel
		}
	}
	if len(result.Novel.Chapters) == 0 {
		return nil, fmt.Errorf("%s 未找到章节列表", source.Name)
	}
	return result, nil
}

func (c *Client) FetchChapterContent(rawURL string) (*ChapterContent, error) {
	result, err := c.FetchChapter(rawURL)
	if err != nil {
		return nil, err
	}
	return result.Content, nil
}

func (c *Client) FetchChapter(rawURL string) (*ChapterFetchResult, error) {
	if c == nil {
		return nil, fmt.Errorf("novel source client is nil")
	}
	source := c.Source.normalized()
	parts, ok := source.ParseURL(rawURL)
	if !ok || parts.Kind != ContentTypeChapter {
		return nil, fmt.Errorf("unsupported %s chapter url: %s", source.ID, rawURL)
	}
	referer := source.BaseURL + "/"
	if parts.BookID != "" {
		if catalogURL := source.CatalogURL(parts.BookID); catalogURL != "" {
			referer = catalogURL
		}
	}
	htmlText, err := c.fetch(parts.Canonical, referer)
	if err != nil {
		return nil, err
	}
	content, err := source.ParseChapterHTML(htmlText)
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
		ParsedHTML: BuildChapterHTML(source, content, parts.Canonical),
	}, nil
}

func (c *Client) BuildHTMLFromURL(rawURL string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("novel source client is nil")
	}
	parts, ok := c.Source.ParseURL(rawURL)
	if !ok {
		return "", fmt.Errorf("unsupported %s url: %s", c.Source.ID, rawURL)
	}
	if parts.Kind == ContentTypeChapter {
		chapter, err := c.FetchChapterContent(parts.Canonical)
		if err != nil {
			return "", err
		}
		return BuildChapterHTML(c.Source, chapter, parts.Canonical), nil
	}
	novel, err := c.FetchNovelChapters(parts.Canonical)
	if err != nil {
		return "", err
	}
	return BuildNovelHTML(c.Source, novel), nil
}

func (c *Client) fetch(rawURL string, referer string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header = c.requestHeaders(referer)
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
		return "", fmt.Errorf("%s HTTP %d: %s", c.Source.ID, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return DecodeHTML(body, resp.Header.Get("Content-Type"))
}

func (c *Client) requestHeaders(referer string) http.Header {
	headers := http.Header{}
	headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	headers.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Pragma", "no-cache")
	headers.Set("Upgrade-Insecure-Requests", "1")
	headers.Set("User-Agent", firstNonEmpty(c.UserAgent, defaultUserAgent))
	if strings.TrimSpace(referer) != "" {
		headers.Set("Referer", strings.TrimSpace(referer))
	}
	if strings.TrimSpace(c.Cookie) != "" {
		headers.Set("Cookie", strings.TrimSpace(c.Cookie))
	}
	return headers
}

func (c *Client) httpClient() HTTPClient {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func DecodeHTML(body []byte, contentType string) (string, error) {
	if utf8.Valid(body) {
		return string(body), nil
	}
	encoding, _, _ := charset.DetermineEncoding(body, contentType)
	decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(body), encoding.NewDecoder()))
	if err == nil {
		return string(decoded), nil
	}
	return string(body), err
}

func BuildNovelHTML(source Source, novel *Novel) string {
	source = source.normalized()
	if novel == nil {
		novel = &Novel{}
	}
	var b strings.Builder
	writeHTMLHead(&b, firstNonEmpty(novel.Title, novel.BookID, "novel"))
	b.WriteString("<main>\n")
	b.WriteString("<h1>" + stdhtml.EscapeString(firstNonEmpty(novel.Title, novel.BookID, "novel")) + "</h1>\n")
	b.WriteString("<dl>\n")
	writeTerm(&b, "平台", firstNonEmpty(source.Name, source.ID))
	writeTerm(&b, "作者", novel.Author)
	writeTerm(&b, "分类", novel.Category)
	writeTerm(&b, "状态", novel.Status)
	writeTerm(&b, "字数", novel.WordCount)
	writeTerm(&b, "更新", novel.UpdateTime)
	writeTerm(&b, "最新章节", novel.LatestChapter)
	writeTerm(&b, "来源", novel.URL)
	writeTerm(&b, "章节数", fmt.Sprint(len(novel.Chapters)))
	b.WriteString("</dl>\n")
	if strings.TrimSpace(novel.Description) != "" {
		b.WriteString("<section><h2>简介</h2>\n")
		b.WriteString(TextToHTML(novel.Description))
		b.WriteString("</section>\n")
	}
	if len(novel.Chapters) > 0 {
		b.WriteString("<section><h2>目录</h2><ol>\n")
		for _, chapter := range novel.Chapters {
			title := firstNonEmpty(chapter.Title, fmt.Sprintf("第 %d 章", chapter.Index))
			if chapter.URL != "" {
				b.WriteString(`<li><a href="` + stdhtml.EscapeString(chapter.URL) + `">` + stdhtml.EscapeString(title) + "</a></li>\n")
			} else {
				b.WriteString("<li>" + stdhtml.EscapeString(title) + "</li>\n")
			}
		}
		b.WriteString("</ol></section>\n")
	}
	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String()
}

func BuildChapterHTML(source Source, chapter *ChapterContent, sourceURL string) string {
	source = source.normalized()
	if chapter == nil {
		chapter = &ChapterContent{}
	}
	var b strings.Builder
	writeHTMLHead(&b, firstNonEmpty(chapter.Title, "chapter"))
	b.WriteString("<main>\n")
	b.WriteString("<h1>" + stdhtml.EscapeString(firstNonEmpty(chapter.Title, "chapter")) + "</h1>\n")
	b.WriteString("<dl>\n")
	writeTerm(&b, "平台", firstNonEmpty(source.Name, source.ID))
	writeTerm(&b, "来源", sourceURL)
	b.WriteString("</dl>\n")
	b.WriteString("<article>\n")
	b.WriteString(TextToHTML(chapter.Content))
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
		b.WriteString("<p>" + stdhtml.EscapeString(line) + "</p>\n")
	}
	return b.String()
}

func NormalizeURL(rawURL string, pageURL string, baseURL string) string {
	return normalizeURL(rawURL, pageURL, baseURL)
}

func joinBaseURL(baseURL string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if parsed, ok := parseHTTPURL(value); ok {
		return parsed.String()
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasPrefix(value, "//") {
		if parsed, err := url.Parse(baseURL); err == nil && parsed.Scheme != "" {
			return parsed.Scheme + ":" + value
		}
		return "https:" + value
	}
	if strings.HasPrefix(value, "/") {
		return baseURL + value
	}
	return baseURL + "/" + value
}

func writeHTMLHead(b *strings.Builder, title string) {
	b.WriteString("<!doctype html>\n<html>\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<title>" + stdhtml.EscapeString(title) + "</title>\n")
	b.WriteString("</head>\n<body>\n")
}

func writeTerm(b *strings.Builder, key string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	b.WriteString("<dt>" + stdhtml.EscapeString(key) + "</dt><dd>" + stdhtml.EscapeString(value) + "</dd>\n")
}
