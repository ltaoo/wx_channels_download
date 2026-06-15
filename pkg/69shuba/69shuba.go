package shuba69

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const (
	PlatformID = "69shuba"
	SourceName = "69书吧"
	BaseURL    = "https://www.69shuba.com"

	ContentTypeNovel   = "novel"
	ContentTypeChapter = "chapter"
)

var (
	defaultHeaders = http.Header{
		"Accept":                 []string{"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
		"Accept-Language":        []string{"zh-CN,zh;q=0.9,en;q=0.8"},
		"Cache-Control":          []string{"no-cache"},
		"Pragma":                 []string{"no-cache"},
		"Priority":               []string{"u=0, i"},
		"Sec-Ch-Ua":              []string{`"Google Chrome";v="149", "Chromium";v="149", "Not)A;Brand";v="24"`},
		"Sec-Ch-Ua-Arch":         []string{`"arm"`},
		"Sec-Ch-Ua-Bitness":      []string{`"64"`},
		"Sec-Ch-Ua-Full-Version": []string{`"149.0.7827.103"`},
		"Sec-Ch-Ua-Full-Version-List": []string{
			`"Google Chrome";v="149.0.7827.103", "Chromium";v="149.0.7827.103", "Not)A;Brand";v="24.0.0.0"`,
		},
		"Sec-Ch-Ua-Mobile":   []string{"?0"},
		"Sec-Ch-Ua-Model":    []string{`""`},
		"Sec-Ch-Ua-Platform": []string{`"macOS"`},
		"Sec-Ch-Ua-Platform-Version": []string{
			`"15.7.1"`,
		},
		"Sec-Fetch-Dest":            []string{"document"},
		"Sec-Fetch-Mode":            []string{"navigate"},
		"Sec-Fetch-Site":            []string{"same-origin"},
		"Sec-Fetch-User":            []string{"?1"},
		"Upgrade-Insecure-Requests": []string{"1"},
		"User-Agent":                []string{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"},
	}
	bookPathRE      = regexp.MustCompile(`/book/(\d+)(?:\.htm|/)`)
	bookImageIDRE   = regexp.MustCompile(`/article/image/\d+/(\d+)/`)
	jsStringFieldRE = regexp.MustCompile(`(?s)\b([A-Za-z0-9_]+)\s*:\s*['"]([^'"]*)['"]`)
	wordCountRE     = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?\s*万?字)`)
)

type PageURL struct {
	Kind      string `json:"kind"`
	BookID    string `json:"book_id,omitempty"`
	ChapterID string `json:"chapter_id,omitempty"`
	Canonical string `json:"canonical"`
}

type Novel struct {
	Title          string    `json:"title"`
	URL            string    `json:"url"`
	Author         string    `json:"author,omitempty"`
	Category       string    `json:"category,omitempty"`
	Status         string    `json:"status,omitempty"`
	BookID         string    `json:"book_id,omitempty"`
	Description    string    `json:"description,omitempty"`
	CoverURL       string    `json:"cover_url,omitempty"`
	Tags           []string  `json:"tags,omitempty"`
	WordCount      string    `json:"word_count,omitempty"`
	UpdateTime     string    `json:"update_time,omitempty"`
	LatestChapter  string    `json:"latest_chapter,omitempty"`
	ChapterCount   int       `json:"chapter_count,omitempty"`
	FullCatalogURL string    `json:"full_catalog_url,omitempty"`
	Chapters       []Chapter `json:"chapters,omitempty"`
}

type Chapter struct {
	Index     int    `json:"index"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	UpdatedAt string `json:"updated_at,omitempty"`
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
	Cookie      string
	UserAgent   string
}

func NewClient(client HTTPClient) *Client {
	if client == nil {
		client = &http.Client{}
	}
	return &Client{HTTPClient: client}
}

func NewClientWithOptions(client HTTPClient, cookie, userAgent string) *Client {
	c := NewClient(client)
	c.Cookie = cookie
	c.UserAgent = userAgent
	return c
}

func NewClientWithHTMLFetcher(fetcher HTMLFetcher, cookie, userAgent string) *Client {
	c := NewClient(nil)
	c.HTMLFetcher = fetcher
	c.Cookie = cookie
	c.UserAgent = userAgent
	return c
}

func CanParse(rawURL string) bool {
	_, ok := ParseURL(rawURL)
	return ok
}

func ParseURL(rawURL string) (PageURL, bool) {
	parsed, ok := parseHTTPHost(rawURL, "69shuba.com", "www.69shuba.com", "m.69shuba.com")
	if !ok {
		return PageURL{}, false
	}
	segments := splitPath(parsed)
	if len(segments) < 2 {
		return PageURL{}, false
	}
	switch segments[0] {
	case "book":
		bookID := strings.TrimSuffix(strings.TrimSpace(segments[1]), ".htm")
		if bookID == "" {
			return PageURL{}, false
		}
		if len(segments) >= 3 {
			chapterID := strings.TrimSuffix(strings.TrimSpace(segments[2]), ".htm")
			if chapterID == "" {
				return PageURL{}, false
			}
			return PageURL{
				Kind:      ContentTypeChapter,
				BookID:    bookID,
				ChapterID: chapterID,
				Canonical: BaseURL + "/book/" + bookID + "/" + segments[2],
			}, true
		}
		canonical := BaseURL + "/book/" + bookID + "/"
		if strings.HasSuffix(strings.ToLower(segments[1]), ".htm") {
			canonical = BaseURL + "/book/" + bookID + ".htm"
		}
		return PageURL{Kind: ContentTypeNovel, BookID: bookID, Canonical: canonical}, true
	case "txt":
		if len(segments) < 3 {
			return PageURL{}, false
		}
		bookID := strings.TrimSpace(segments[1])
		chapterID := strings.TrimSuffix(strings.TrimSpace(segments[2]), ".htm")
		if bookID == "" || chapterID == "" {
			return PageURL{}, false
		}
		return PageURL{
			Kind:      ContentTypeChapter,
			BookID:    bookID,
			ChapterID: chapterID,
			Canonical: BaseURL + "/txt/" + bookID + "/" + segments[2],
		}, true
	default:
		return PageURL{}, false
	}
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
		return nil, fmt.Errorf("unsupported 69shuba novel url")
	}
	htmlText, err := sessionClient.fetch(parts.Canonical, novelReferer(parts))
	if err != nil {
		return nil, err
	}
	novel, err := ParseNovelHTML(parts.Canonical, htmlText)
	if err != nil {
		return nil, err
	}
	novel.BookID = firstNonEmpty(novel.BookID, parts.BookID)
	novel.URL = firstNonEmpty(novel.URL, parts.Canonical)
	result = &NovelFetchResult{
		Novel:            novel,
		SourceURL:        parts.Canonical,
		SourceHTML:       htmlText,
		SourceNovel:      cloneNovel(novel),
		SourceParsedHTML: BuildNovelHTML(novel),
	}
	if novel.FullCatalogURL == "" && strings.HasSuffix(parts.Canonical, ".htm") {
		novel.FullCatalogURL = BaseURL + "/book/" + parts.BookID + "/"
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
		if len(fullNovel.Chapters) == 0 {
			return nil, fmt.Errorf("完整目录页面未找到章节列表")
		}
		result.FullCatalogURL = novel.FullCatalogURL
		result.FullCatalogHTML = fullHTML
		result.FullCatalogNovel = cloneNovel(fullNovel)
		result.FullCatalogParsedHTML = BuildNovelHTML(fullNovel)
		mergeFullCatalog(novel, fullNovel)
		result.Novel = novel
	}
	if len(novel.Chapters) == 0 {
		return nil, fmt.Errorf("未找到章节列表")
	}
	return result, nil
}

func (c *Client) FetchChapterContent(rawURL string) (chapter *ChapterContent, err error) {
	sessionClient, done, err := c.beginHTMLFetchSession()
	if err != nil {
		return nil, err
	}
	if done != nil {
		defer func() { done(err) }()
	}
	parts, ok := ParseURL(rawURL)
	if !ok || parts.Kind != ContentTypeChapter {
		return nil, fmt.Errorf("unsupported 69shuba chapter url")
	}
	htmlText, err := sessionClient.fetch(parts.Canonical, BaseURL+"/book/"+parts.BookID+"/")
	if err != nil {
		return nil, err
	}
	return ParseChapterHTML(htmlText)
}

func (c *Client) BuildHTMLFromURL(rawURL string) (string, error) {
	parts, ok := ParseURL(rawURL)
	if !ok {
		return "", fmt.Errorf("unsupported 69shuba url")
	}
	if parts.Kind == ContentTypeChapter {
		chapter, err := c.FetchChapterContent(parts.Canonical)
		if err != nil {
			return "", err
		}
		return BuildChapterHTML(chapter, parts.Canonical), nil
	}
	novel, err := c.FetchNovelChapters(parts.Canonical)
	if err != nil {
		return "", err
	}
	return BuildNovelHTML(novel), nil
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
			return "", fmt.Errorf("69shuba HTTP 403: request was blocked before browser rendering; configure 69shuba.fetcher=cdp or 69shuba.fetcher=sandbox for Cloudflare-protected pages: %s", strings.TrimSpace(string(body)))
		}
		return "", fmt.Errorf("69shuba HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return DecodeHTML(body, resp.Header.Get("Content-Type"))
}

func (c *Client) requestHeaders(referer string) http.Header {
	headers := defaultHeaders.Clone()
	if referer != "" {
		headers.Set("Referer", referer)
	}
	if c != nil && strings.TrimSpace(c.Cookie) != "" {
		headers.Set("Cookie", c.Cookie)
	}
	if c != nil && strings.TrimSpace(c.UserAgent) != "" {
		headers.Set("User-Agent", c.UserAgent)
	}
	return headers
}

func novelReferer(parts PageURL) string {
	if parts.BookID == "" {
		return BaseURL + "/"
	}
	if strings.HasSuffix(strings.ToLower(parts.Canonical), ".htm") {
		return BaseURL + "/book/" + parts.BookID + "/"
	}
	return BaseURL + "/"
}

func (c *Client) httpClient() HTTPClient {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func ParseNovelHTML(pageURL string, htmlText string) (*Novel, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	parts, _ := ParseURL(pageURL)
	novel := &Novel{
		URL:    firstNonEmpty(parts.Canonical, pageURL),
		BookID: firstNonEmpty(parts.BookID, extractBookID(pageURL), extractBookID(htmlText)),
	}
	novel.Title = cleanBookTitle(firstNonEmpty(
		metaContent(doc, "property", "og:novel:book_name"),
		jsField(htmlText, "articlename"),
		strings.TrimSpace(doc.Find(".booknav2 h1").First().Text()),
		strings.TrimSpace(doc.Find("h1").First().Text()),
		strings.TrimSpace(doc.Find("title").First().Text()),
	))
	novel.Author = firstNonEmpty(
		metaContent(doc, "property", "og:novel:author"),
		jsField(htmlText, "author"),
	)
	novel.Category = firstNonEmpty(
		metaContent(doc, "property", "og:novel:category"),
		jsField(htmlText, "sortName"),
	)
	novel.Status = metaContent(doc, "property", "og:novel:status")
	novel.UpdateTime = metaContent(doc, "property", "og:novel:update_time")
	novel.LatestChapter = metaContent(doc, "property", "og:novel:latest_chapter_name")
	novel.CoverURL = normalizeURL(firstNonEmpty(
		metaContent(doc, "property", "og:image"),
		attr(doc.Find(".bookimg2 img, .bookimg img, .book-cover img").First(), "src"),
	), pageURL, BaseURL)
	novel.Description = firstNonEmpty(
		descriptionFromSelection(doc.Find(".navtxt").First()),
		htmlToText(metaContent(doc, "property", "og:description")),
		metaContent(doc, "name", "description"),
	)
	novel.Tags = splitTags(jsField(htmlText, "tags"))
	parseBookNav(doc, novel)
	if novel.BookID == "" {
		novel.BookID = firstNonEmpty(extractBookID(novel.CoverURL), extractBookID(novel.URL))
	}
	novel.FullCatalogURL = findFullCatalogURL(doc, pageURL, novel.BookID)
	novel.Chapters = parseChapterLinks(doc, pageURL)
	if novel.ChapterCount == 0 {
		novel.ChapterCount = len(novel.Chapters)
	}
	if novel.Title == "" && novel.BookID != "" {
		novel.Title = "69shuba_" + novel.BookID
	}
	return novel, nil
}

func ParseChapterHTML(htmlText string) (*ChapterContent, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	content := &ChapterContent{
		Title: cleanTitle(firstNonEmpty(
			strings.TrimSpace(doc.Find(".txtnav h1, .chapter-title, h1").First().Text()),
			strings.TrimSpace(doc.Find("title").First().Text()),
		)),
	}
	for _, selector := range []string{
		"#htmlContent",
		"#chaptercontent",
		".txtnav .content",
		".novelcontent",
		"#novelcontent",
		"#content",
		".chapter-content",
		".content",
		"article",
		".txtnav",
	} {
		sel := doc.Find(selector).First()
		if sel.Length() == 0 {
			continue
		}
		clone := sel.Clone()
		clone.Find("script, style, iframe, ins, .ads, .ad, #ads, .txtright, #txtright, .bottom-ad, .txtinfo, .page1, .jump, h1").Remove()
		htmlContent, _ := clone.Html()
		text := cleanChapterContent(htmlToText(htmlContent))
		if text != "" {
			content.Content = text
			break
		}
	}
	if strings.TrimSpace(content.Content) == "" {
		return nil, fmt.Errorf("未找到章节内容")
	}
	return content, nil
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
	if isGBK(body) {
		return decodeGBK(body)
	}
	return string(body), err
}

func BuildNovelHTML(novel *Novel) string {
	if novel == nil {
		novel = &Novel{}
	}
	var b strings.Builder
	writeHTMLHead(&b, firstNonEmpty(novel.Title, novel.BookID, "novel"))
	b.WriteString("<main>\n")
	b.WriteString("<h1>" + stdhtml.EscapeString(firstNonEmpty(novel.Title, novel.BookID, "novel")) + "</h1>\n")
	b.WriteString("<dl>\n")
	writeTerm(&b, "平台", SourceName)
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
	if len(novel.Tags) > 0 {
		b.WriteString("<section><h2>标签</h2><p>")
		for i, tag := range novel.Tags {
			if i > 0 {
				b.WriteString(" / ")
			}
			b.WriteString(stdhtml.EscapeString(tag))
		}
		b.WriteString("</p></section>\n")
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

func BuildChapterHTML(chapter *ChapterContent, sourceURL string) string {
	if chapter == nil {
		chapter = &ChapterContent{}
	}
	var b strings.Builder
	writeHTMLHead(&b, firstNonEmpty(chapter.Title, "chapter"))
	b.WriteString("<main>\n")
	b.WriteString("<h1>" + stdhtml.EscapeString(firstNonEmpty(chapter.Title, "chapter")) + "</h1>\n")
	b.WriteString("<dl>\n")
	writeTerm(&b, "平台", SourceName)
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

func parseBookNav(doc *goquery.Document, novel *Novel) {
	doc.Find(".booknav2 p, .bookinfo .booknav2 span, .bookbox .booknav2 span").Each(func(_ int, s *goquery.Selection) {
		text := compactText(s.Text())
		switch {
		case strings.Contains(text, "作者"):
			novel.Author = firstNonEmpty(novel.Author, strings.TrimSpace(s.Find("a").First().Text()), afterAnyLabel(text, "作者：", "作者:"))
		case strings.Contains(text, "分类") || strings.Contains(text, "类型"):
			novel.Category = firstNonEmpty(novel.Category, strings.TrimSpace(s.Find("a").First().Text()), afterAnyLabel(text, "分类：", "类型：", "分类:", "类型:"))
		case strings.Contains(text, "状态"):
			novel.Status = firstNonEmpty(novel.Status, afterAnyLabel(text, "状态：", "状态:"))
		case strings.Contains(text, "更新"):
			novel.UpdateTime = firstNonEmpty(novel.UpdateTime, afterAnyLabel(text, "更新：", "更新:"))
		}
		if novel.WordCount == "" {
			if match := wordCountRE.FindString(text); match != "" {
				novel.WordCount = strings.TrimSpace(match)
			}
		}
		if novel.Status == "" && strings.Contains(text, "|") {
			parts := strings.Split(text, "|")
			novel.Status = strings.TrimSpace(parts[len(parts)-1])
		}
	})
	doc.Find(".infolist li").Each(func(_ int, s *goquery.Selection) {
		label := strings.TrimSpace(s.Find("span").First().Text())
		value := strings.TrimSpace(strings.Replace(s.Text(), label, "", 1))
		switch label {
		case "字数":
			novel.WordCount = firstNonEmpty(novel.WordCount, value)
		case "章节数":
			novel.ChapterCount = parseInt(value)
		}
	})
}

func parseChapterLinks(doc *goquery.Document, pageURL string) []Chapter {
	selectors := []string{
		".qustime li a",
		".tabsnav a[href*=\"/txt/\"]",
		"#chapterlist a",
		".mulu_list li a",
		".catalog li a",
		"#catalog ul li a",
		".mu_contain ul li a",
		".chapterlist a",
		".booklist a",
	}
	var chapters []Chapter
	seen := map[string]bool{}
	for _, selector := range selectors {
		doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if !exists || strings.TrimSpace(href) == "" {
				return
			}
			chapterURL := normalizeURL(href, pageURL, BaseURL)
			parts, ok := ParseURL(chapterURL)
			if !ok || parts.Kind != ContentTypeChapter || seen[chapterURL] {
				return
			}
			title := strings.TrimSpace(s.Find("span").First().Text())
			if title == "" {
				title = strings.TrimSpace(s.Text())
				if small := strings.TrimSpace(s.Find("small").First().Text()); small != "" {
					title = strings.TrimSpace(strings.Replace(title, small, "", 1))
				}
			}
			if title == "" {
				return
			}
			updatedAt := strings.TrimSpace(firstNonEmpty(
				attr(s.Parent(), "data-etime"),
				strings.TrimSpace(s.Find("small").First().Text()),
			))
			seen[chapterURL] = true
			chapters = append(chapters, Chapter{
				Index:     len(chapters) + 1,
				Title:     title,
				URL:       chapterURL,
				UpdatedAt: updatedAt,
			})
		})
	}
	return chapters
}

func findFullCatalogURL(doc *goquery.Document, pageURL string, bookID string) string {
	if href := attr(doc.Find("a.more-btn").First(), "href"); href != "" {
		return normalizeURL(href, pageURL, BaseURL)
	}
	var out string
	doc.Find("a").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		text := strings.TrimSpace(s.Text())
		if strings.Contains(text, "完整目录") || strings.Contains(text, "开始阅读") {
			out = normalizeURL(attr(s, "href"), pageURL, BaseURL)
			return false
		}
		return true
	})
	if out == "" && bookID != "" {
		out = BaseURL + "/book/" + bookID + "/"
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
	return strings.HasSuffix(strings.ToLower(sourceURL), ".htm") ||
		len(novel.Chapters) == 0 ||
		(novel.ChapterCount > 0 && len(novel.Chapters) < novel.ChapterCount)
}

func mergeFullCatalog(dst *Novel, full *Novel) {
	if dst == nil || full == nil {
		return
	}
	dst.BookID = firstNonEmpty(dst.BookID, full.BookID)
	dst.Title = firstNonEmpty(dst.Title, full.Title)
	dst.Author = firstNonEmpty(dst.Author, full.Author)
	dst.Category = firstNonEmpty(dst.Category, full.Category)
	dst.Status = firstNonEmpty(dst.Status, full.Status)
	dst.Description = firstNonEmpty(dst.Description, full.Description)
	dst.CoverURL = firstNonEmpty(dst.CoverURL, full.CoverURL)
	dst.WordCount = firstNonEmpty(dst.WordCount, full.WordCount)
	dst.UpdateTime = firstNonEmpty(dst.UpdateTime, full.UpdateTime)
	dst.LatestChapter = firstNonEmpty(dst.LatestChapter, full.LatestChapter)
	if dst.ChapterCount == 0 {
		dst.ChapterCount = full.ChapterCount
	}
	if len(full.Tags) > 0 && len(dst.Tags) == 0 {
		dst.Tags = full.Tags
	}
	if len(full.Chapters) > len(dst.Chapters) {
		dst.Chapters = full.Chapters
	}
}

func cloneNovel(in *Novel) *Novel {
	if in == nil {
		return nil
	}
	out := *in
	if len(in.Tags) > 0 {
		out.Tags = append([]string(nil), in.Tags...)
	}
	if len(in.Chapters) > 0 {
		out.Chapters = append([]Chapter(nil), in.Chapters...)
	}
	return &out
}

func descriptionFromSelection(sel *goquery.Selection) string {
	if sel.Length() == 0 {
		return ""
	}
	clone := sel.Clone()
	clone.Find("script, style").Remove()
	htmlText, _ := clone.Html()
	text := htmlToText(htmlText)
	lines := strings.Split(text, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "小说关键词") {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

func htmlToText(s string) string {
	replacements := map[string]string{
		"<br>":       "\n",
		"<br/>":      "\n",
		"<br />":     "\n",
		"</p>":       "\n",
		"</div>":     "\n",
		"&nbsp;":     " ",
		"&#160;":     " ",
		"\r\n":       "\n",
		"\u00a0":     " ",
		"&amp;nbsp;": " ",
	}
	for old, replacement := range replacements {
		s = strings.ReplaceAll(s, old, replacement)
	}
	re := regexp.MustCompile(`<[^>]+>`)
	return strings.TrimSpace(stdhtml.UnescapeString(re.ReplaceAllString(s, "")))
}

func cleanChapterContent(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "69shuba") ||
			strings.Contains(line, "69书吧") ||
			strings.Contains(line, "最新网址") ||
			(strings.Contains(line, "最新章节") && strings.Contains(line, "地址")) ||
			strings.Contains(line, "请收藏本站") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}

func cleanBookTitle(s string) string {
	s = cleanTitle(s)
	if idx := strings.Index(s, ","); idx > 0 && (strings.Contains(s, "无弹窗") || strings.Contains(s, "最新章节")) {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

func cleanTitle(s string) string {
	s = compactText(s)
	for _, suffix := range []string{"-69书吧", "_69书吧", " - 69书吧"} {
		if idx := strings.Index(s, suffix); idx > 0 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}

func metaContent(doc *goquery.Document, attrName string, key string) string {
	var out string
	doc.Find("meta").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if attr(s, attrName) == key {
			out = strings.TrimSpace(attr(s, "content"))
			return false
		}
		return true
	})
	return out
}

func attr(s *goquery.Selection, name string) string {
	if s == nil || s.Length() == 0 {
		return ""
	}
	value, _ := s.Attr(name)
	return strings.TrimSpace(value)
}

func jsField(htmlText string, field string) string {
	for _, match := range jsStringFieldRE.FindAllStringSubmatch(htmlText, -1) {
		if len(match) >= 3 && match[1] == field {
			return strings.TrimSpace(match[2])
		}
	}
	return ""
}

func extractBookID(value string) string {
	if match := bookPathRE.FindStringSubmatch(value); len(match) > 1 {
		return match[1]
	}
	if match := bookImageIDRE.FindStringSubmatch(value); len(match) > 1 {
		return match[1]
	}
	if match := regexp.MustCompile(`articleid\s*:\s*['"]?(\d+)['"]?`).FindStringSubmatch(value); len(match) > 1 {
		return match[1]
	}
	return ""
}

func splitTags(raw string) []string {
	raw = strings.ReplaceAll(raw, "，", "|")
	raw = strings.ReplaceAll(raw, ",", "|")
	parts := strings.Split(raw, "|")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		out = append(out, part)
	}
	return out
}

func afterAnyLabel(text string, labels ...string) string {
	for _, label := range labels {
		if after, ok := strings.CutPrefix(text, label); ok {
			return strings.TrimSpace(after)
		}
		if idx := strings.Index(text, label); idx >= 0 {
			return strings.TrimSpace(text[idx+len(label):])
		}
	}
	return ""
}

func parseInt(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			continue
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func parseHTTPHost(rawURL string, hosts ...string) (*url.URL, bool) {
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

func splitPath(parsed *url.URL) []string {
	if parsed == nil {
		return nil
	}
	pathValue := strings.Trim(parsed.EscapedPath(), "/")
	if pathValue == "" {
		return nil
	}
	segments := strings.Split(pathValue, "/")
	for i, segment := range segments {
		if decoded, err := url.PathUnescape(segment); err == nil {
			segments[i] = decoded
		}
	}
	return segments
}

func normalizeURL(href, pageURL, baseURL string) string {
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

func compactText(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
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

func writeHTMLHead(b *strings.Builder, title string) {
	b.WriteString("<!doctype html>\n<html lang=\"zh-CN\">\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	b.WriteString("<title>" + stdhtml.EscapeString(title) + "</title>\n")
	b.WriteString("<style>body{font-family:-apple-system,BlinkMacSystemFont,\"Segoe UI\",sans-serif;line-height:1.75;margin:0;background:#f7f7f5;color:#1f2328}main{max-width:860px;margin:0 auto;padding:32px 20px 56px;background:#fff;min-height:100vh}h1{font-size:28px;line-height:1.3;margin:0 0 20px}h2{font-size:20px;margin:28px 0 12px}dl{display:grid;grid-template-columns:max-content 1fr;gap:6px 14px;color:#4f5762}dt{font-weight:600}dd{margin:0;word-break:break-all}p{margin:0 0 14px}ol{padding-left:1.6em}li{margin:6px 0}a{color:#0b65c2;text-decoration:none}a:hover{text-decoration:underline}</style>\n")
	b.WriteString("</head>\n<body>\n")
}

func writeTerm(b *strings.Builder, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" || value == "0" {
		return
	}
	b.WriteString("<dt>" + stdhtml.EscapeString(key) + "</dt><dd>" + stdhtml.EscapeString(value) + "</dd>\n")
}
