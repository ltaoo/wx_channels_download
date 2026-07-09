package qidian

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	nethtml "golang.org/x/net/html"

	mqidianpkg "wx_channel/pkg/scraper/mqidian"
)

var (
	brRE             = regexp.MustCompile(`(?i)<br\s*/?>`)
	bookIDPathRE     = regexp.MustCompile(`/book/(\d+)`)
	authorIDPathRE   = regexp.MustCompile(`/author/(\d+)`)
	chapterCountRE   = regexp.MustCompile(`共\s*([0-9]+)\s*章`)
	volumeSuffixRE   = regexp.MustCompile(`\s*·\s*共\s*\d+\s*章.*$`)
	latestPrefixRE   = regexp.MustCompile(`^\s*最新章节[:：]\s*`)
	updatePrefixRE   = regexp.MustCompile(`^\s*更新时间[:：]\s*`)
	numberWithUnitRE = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*(万|亿)?`)
	publishTimeRE    = regexp.MustCompile(`首发时间[:：]\s*([0-9]{4}-[0-9]{2}-[0-9]{2}\s+[0-9]{2}:[0-9]{2}:[0-9]{2})`)
	wordCountRE      = regexp.MustCompile(`章节字数[:：]\s*([0-9]+)`)
	jsKeyRE          = regexp.MustCompile(`([{\[,]\s*)([A-Za-z_$][A-Za-z0-9_$]*)\s*:`)
	trailingCommaRE  = regexp.MustCompile(`,\s*([}\]])`)
)

type Client struct {
	HTTPClient *http.Client
	BaseURL    string
	UserAgent  string
	Cookie     string
}

func NewClient(client *http.Client) *Client {
	if client == nil {
		client = &http.Client{}
	}
	return &Client{
		HTTPClient: client,
		BaseURL:    BaseURL,
		UserAgent:  DefaultUserAgent(),
	}
}

func DefaultUserAgent() string {
	return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
}

func ParseURL(rawURL string) (URLParts, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return URLParts{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "qidian.com" && host != "www.qidian.com" && host != "book.qidian.com" {
		return URLParts{}, false
	}
	segments := splitPath(parsed)
	if len(segments) >= 2 && (segments[0] == "book" || segments[0] == "info") && strings.TrimSpace(segments[1]) != "" {
		bookID := strings.TrimSpace(segments[1])
		return URLParts{
			BookID:    bookID,
			Canonical: CanonicalBookURL(bookID),
		}, true
	}
	return URLParts{}, false
}

func CanonicalBookURL(bookID string) string {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return ""
	}
	return BaseURL + "/book/" + url.PathEscape(bookID) + "/"
}

func FetchBookProfile(id string) (*BookProfile, error) {
	return NewClient(nil).FetchBookProfile(id)
}

func (c *Client) FetchBookProfile(id string) (*BookProfile, error) {
	return c.FetchBookProfileContext(context.Background(), id)
}

func (c *Client) FetchBookProfileContext(ctx context.Context, id string) (*BookProfile, error) {
	bookID := strings.TrimSpace(id)
	if parts, ok := ParseURL(bookID); ok {
		bookID = parts.BookID
	}
	if bookID == "" {
		return nil, fmt.Errorf("missing qidian book id")
	}
	reqURL := strings.TrimRight(firstNonEmpty(c.BaseURL, BaseURL), "/") + "/book/" + url.PathEscape(bookID) + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Referer", BaseURL+"/")
	req.Header.Set("User-Agent", firstNonEmpty(c.UserAgent, DefaultUserAgent()))
	if strings.TrimSpace(c.Cookie) != "" {
		req.Header.Set("Cookie", c.Cookie)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, reqURL)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	profile, err := ParseBookProfile(reqURL, body)
	if err == nil {
		c.supplementMobileCatalog(ctx, bookID, profile)
		return profile, nil
	}
	if shouldUseMobileFallback(resp.StatusCode, body) {
		if fallback, fallbackErr := c.fetchMobileBookProfileContext(ctx, bookID); fallbackErr == nil {
			return fallback, nil
		}
	}
	return nil, err
}

func (c *Client) ParseBookProfile(reqURL string, r io.Reader) (*BookProfile, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return ParseBookProfile(reqURL, body)
}

func ParseBookProfile(reqURL string, body []byte) (*BookProfile, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	profile, err := parseHTMLBookProfile(reqURL, doc)
	if err != nil {
		return nil, err
	}
	if raw, err := ExtractPageJSON(body); err == nil {
		profile.PageContextJSON = raw
		mergePageJSON(profile, raw)
	}
	profile.PageHTML = string(body)
	return profile, nil
}

func shouldUseMobileFallback(statusCode int, body []byte) bool {
	if statusCode == http.StatusAccepted {
		return true
	}
	text := string(body)
	return strings.Contains(text, "/probe.js") ||
		strings.Contains(text, "/probev3.js") ||
		strings.Contains(text, "x-waf-captcha-referer")
}

func (c *Client) fetchMobileBookProfileContext(ctx context.Context, bookID string) (*BookProfile, error) {
	client := mqidianpkg.NewClient(c.HTTPClient)
	client.UserAgent = mqidianpkg.DefaultUserAgent()
	client.Cookie = c.Cookie
	profile, err := client.FetchBookProfileContext(ctx, bookID)
	if err != nil {
		return nil, err
	}
	out := bookProfileFromMobile(bookID, profile)
	c.supplementMobileCatalog(ctx, bookID, out)
	return out, nil
}

func (c *Client) supplementMobileCatalog(ctx context.Context, bookID string, profile *BookProfile) {
	if !needsCompleteCatalog(profile) {
		return
	}
	client := mqidianpkg.NewClient(c.HTTPClient)
	client.UserAgent = mqidianpkg.DefaultUserAgent()
	client.Cookie = c.Cookie
	catalog, err := client.FetchBookCatalogContext(ctx, bookID)
	if err != nil {
		return
	}
	catalogProfile := bookProfileFromMobile(bookID, catalog)
	if countChapters(catalogProfile.Volumes) <= countChapters(profile.Volumes) {
		return
	}
	profile.Volumes = catalogProfile.Volumes
	if catalogProfile.ChapterCount > 0 {
		profile.ChapterCount = catalogProfile.ChapterCount
	} else {
		profile.ChapterCount = countChapters(catalogProfile.Volumes)
	}
	if profile.LatestChapter.Title == "" {
		profile.LatestChapter = catalogProfile.LatestChapter
	}
}

func bookProfileFromMobile(bookID string, src *mqidianpkg.BookProfile) *BookProfile {
	if src == nil {
		return nil
	}
	out := &BookProfile{
		URL:              CanonicalBookURL(bookID),
		Title:            src.Title,
		Description:      src.Description,
		Slogan:           src.Slogan,
		CoverURL:         src.CoverURL,
		LatestUpdateAt:   src.LatestUpdateAt,
		Tags:             append([]string(nil), src.Tags...),
		ChapterCount:     src.ChapterCount,
		WordCount:        src.WordCount,
		DisplayWordCount: src.DisplayWordCount,
		Category:         src.Category,
		SubCategory:      src.SubCategory,
		Status:           src.Status,
		Author: Author{
			ID:     src.Author.ID,
			Name:   src.Author.Name,
			URL:    desktopAuthorURL(src.Author.ID, src.Author.URL),
			Avatar: src.Author.Avatar,
			Desc:   src.Author.Desc,
		},
		PageContextJSON: append(json.RawMessage(nil), src.PageContextJSON...),
		PageHTML:        src.PageHTML,
	}
	out.LatestChapter = chapterFromMobile(src.LatestChapter)
	for _, volume := range src.Volumes {
		outVolume := BookVolume{Idx: volume.Idx, Title: volume.Title}
		for _, chapter := range volume.Chapters {
			outVolume.Chapters = append(outVolume.Chapters, chapterFromMobile(chapter))
		}
		out.Volumes = append(out.Volumes, outVolume)
	}
	return out
}

func chapterFromMobile(src mqidianpkg.Chapter) Chapter {
	return Chapter{
		Idx:         src.Idx,
		Title:       src.Title,
		URL:         desktopChapterURL(src.URL),
		Locked:      src.Locked,
		WordCount:   src.WordCount,
		PublishedAt: src.PublishedAt,
	}
}

func desktopChapterURL(value string) string {
	value = normalizeQidianURL(value)
	if strings.HasPrefix(value, "https://m.qidian.com/chapter/") {
		return strings.Replace(value, "https://m.qidian.com/chapter/", BaseURL+"/chapter/", 1)
	}
	if strings.HasPrefix(value, "http://m.qidian.com/chapter/") {
		return strings.Replace(value, "http://m.qidian.com/chapter/", BaseURL+"/chapter/", 1)
	}
	return value
}

func desktopAuthorURL(authorID string, fallback string) string {
	authorID = strings.TrimSpace(authorID)
	if authorID != "" {
		return BaseURL + "/author/" + url.PathEscape(authorID) + "/"
	}
	return normalizeQidianURL(fallback)
}

func ExtractPageJSON(body []byte) (json.RawMessage, error) {
	rawObject, err := extractAssignedObject(string(body), "g_data.pageJson")
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeJSObject(rawObject)
	if err != nil {
		return nil, err
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, normalized); err != nil {
		return nil, fmt.Errorf("invalid qidian page json: %w", err)
	}
	return json.RawMessage(append([]byte(nil), compact.Bytes()...)), nil
}

func parseHTMLBookProfile(reqURL string, doc *goquery.Document) (*BookProfile, error) {
	bookID := firstNonEmpty(bookIDFromURL(reqURL), bookIDFromDocument(doc))
	profile := &BookProfile{
		URL: CanonicalBookURL(bookID),
	}
	if profile.URL == "" {
		profile.URL = normalizeQidianURL(firstNonEmpty(metaContent(doc, "og:url"), reqURL))
	}
	profile.Title = cleanText(firstNonEmpty(
		doc.Find("#bookName").First().Text(),
		metaContent(doc, "og:novel:book_name"),
		metaContent(doc, "og:title"),
		titleFromDocument(doc),
	))
	profile.Slogan = cleanText(doc.Find(".book-info .intro, p.intro").First().Text())
	profile.Description = firstNonEmpty(
		selectionHTMLText(doc.Find("#book-intro-detail").First()),
		selectionHTMLText(doc.Find(".intro-detail p").First()),
		cleanText(metaContent(doc, "og:description")),
		cleanText(doc.Find(`meta[name="description"]`).AttrOr("content", "")),
	)
	profile.CoverURL = normalizeQidianURL(firstNonEmpty(
		doc.Find("#bookImg img").AttrOr("src", ""),
		doc.Find(".book-detail-img img").AttrOr("src", ""),
		metaContent(doc, "og:image"),
	))
	profile.Author = parseAuthor(doc)
	profile.Category, profile.SubCategory = parseCategories(doc)
	profile.Status = cleanText(firstNonEmpty(
		metaContent(doc, "og:novel:status"),
		firstBookAttributeSpan(doc),
	))
	profile.Tags = qidianTags(doc, profile)
	profile.DisplayWordCount = cleanText(doc.Find(".book-info .count em").First().Text())
	profile.WordCount = parseChineseNumber(profile.DisplayWordCount)
	profile.LatestChapter = parseLatestChapter(doc)
	profile.LatestUpdateAt = parseQidianTime(firstNonEmpty(
		metaContent(doc, "og:novel:update_time"),
		strings.TrimSpace(updatePrefixRE.ReplaceAllString(doc.Find(".book-meta .update-time, .update-time").First().Text(), "")),
	))
	profile.Volumes = parseVolumes(doc)
	chapterCount := chapterCountFromText(doc.Find(".catalog-header-desc").First().Text())
	if chapterCount == 0 {
		chapterCount = countChapters(profile.Volumes)
	}
	profile.ChapterCount = chapterCount
	if profile.LatestChapter.Title == "" {
		profile.LatestChapter = lastChapter(profile.Volumes)
	}
	if profile.Title == "" {
		return nil, fmt.Errorf("missing qidian book title")
	}
	if profile.URL == "" {
		return nil, fmt.Errorf("missing qidian book url")
	}
	return profile, nil
}

func parseAuthor(doc *goquery.Document) Author {
	authorURL := normalizeQidianURL(firstNonEmpty(
		doc.Find(".author-intro .writer-name").AttrOr("href", ""),
		doc.Find("#authorId").AttrOr("href", ""),
		metaContent(doc, "og:novel:author_link"),
	))
	authorID := cleanText(firstNonEmpty(
		doc.Find("#authorId").AttrOr("data-authorid", ""),
		idFromPath(authorIDPathRE, authorURL),
	))
	name := cleanText(firstNonEmpty(
		doc.Find(".author-intro .writer-name").First().Text(),
		strings.TrimPrefix(doc.Find(".book-meta .author").First().Text(), "作者:"),
		metaContent(doc, "og:novel:author"),
	))
	return Author{
		ID:     authorID,
		Name:   name,
		URL:    authorURL,
		Avatar: normalizeQidianURL(doc.Find(".author-img img").AttrOr("src", "")),
		Desc:   cleanText(doc.Find(".author-intro .outer-intro p").First().Text()),
	}
}

func parseCategories(doc *goquery.Document) (string, string) {
	var categories []string
	doc.Find(".book-attribute a").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		value := cleanText(s.Text())
		if value != "" {
			categories = append(categories, value)
		}
		return len(categories) < 2
	})
	if len(categories) == 0 {
		value := cleanText(metaContent(doc, "og:novel:category"))
		if value != "" {
			categories = append(categories, value)
		}
	}
	category := ""
	subCategory := ""
	if len(categories) > 0 {
		category = categories[0]
	}
	if len(categories) > 1 {
		subCategory = categories[1]
	} else if value := cleanText(metaContent(doc, "og:novel:category")); value != "" && value != category {
		subCategory = value
	}
	return category, subCategory
}

func firstBookAttributeSpan(doc *goquery.Document) string {
	var out string
	doc.Find(".book-attribute span").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		value := cleanText(s.Text())
		if value == "" || value == "·" {
			return true
		}
		out = value
		return false
	})
	return out
}

func qidianTags(doc *goquery.Document, profile *BookProfile) []string {
	var tags []string
	add := func(values ...string) {
		for _, value := range values {
			value = cleanText(value)
			if value == "" || value == "·" {
				continue
			}
			exists := false
			for _, existing := range tags {
				if existing == value {
					exists = true
					break
				}
			}
			if !exists {
				tags = append(tags, value)
			}
		}
	}
	add(profile.Status, profile.Category, profile.SubCategory)
	doc.Find(".book-attribute span, .book-attribute a, .all-label a, .tag-wrap a, .book-label a").Each(func(_ int, s *goquery.Selection) {
		add(s.Text())
	})
	return tags
}

func parseLatestChapter(doc *goquery.Document) Chapter {
	latest := doc.Find(".book-latest-chapter").First()
	title := cleanText(latestPrefixRE.ReplaceAllString(latest.Text(), ""))
	chapterURL := normalizeQidianURL(latest.AttrOr("href", ""))
	if title == "" {
		title = cleanText(metaContent(doc, "og:novel:latest_chapter_name"))
		chapterURL = normalizeQidianURL(metaContent(doc, "og:novel:latest_chapter_url"))
	}
	if title == "" {
		link := doc.Find("#catalogContinue .link").First()
		title = cleanText(link.Text())
		chapterURL = normalizeQidianURL(doc.Find("#catalogContinue a").First().AttrOr("href", ""))
	}
	return Chapter{Title: title, URL: chapterURL}
}

func parseVolumes(doc *goquery.Document) []BookVolume {
	var volumes []BookVolume
	chapterIdx := 0
	doc.Find("#allCatalog .catalog-volume, .catalog-all .catalog-volume").Each(func(i int, s *goquery.Selection) {
		volume := BookVolume{
			Idx:   i + 1,
			Title: volumeTitle(s.Find(".volume-name").First()),
		}
		s.Find(".volume-chapters .chapter-item, .chapter-list .chapter-item").Each(func(_ int, cs *goquery.Selection) {
			link := cs.Find(".chapter-name, a").First()
			title := cleanText(link.Text())
			if title == "" {
				return
			}
			chapterIdx++
			chapter := Chapter{
				Idx:    chapterIdx,
				Title:  title,
				URL:    normalizeQidianURL(link.AttrOr("href", "")),
				Locked: cs.Find(".chapter-locked").Length() > 0,
			}
			if t := parseChapterPublishedAt(link.AttrOr("title", "")); !t.IsZero() {
				chapter.PublishedAt = t
			}
			chapter.WordCount = parseChapterWordCount(link.AttrOr("title", ""))
			volume.Chapters = append(volume.Chapters, chapter)
		})
		if len(volume.Chapters) > 0 || volume.Title != "" {
			volumes = append(volumes, volume)
		}
	})
	return volumes
}

func volumeTitle(s *goquery.Selection) string {
	if s == nil || s.Length() == 0 {
		return ""
	}
	var parts []string
	s.Contents().EachWithBreak(func(_ int, node *goquery.Selection) bool {
		if len(node.Nodes) == 0 {
			return true
		}
		if node.Nodes[0].Type == nethtml.TextNode {
			text := cleanText(node.Text())
			if text != "" {
				parts = append(parts, text)
			}
			return true
		}
		return false
	})
	title := cleanText(strings.Join(parts, " "))
	if title == "" {
		title = cleanText(s.Text())
	}
	title = cleanText(volumeSuffixRE.ReplaceAllString(title, ""))
	return title
}

func mergePageJSON(profile *BookProfile, raw json.RawMessage) {
	if profile == nil || len(raw) == 0 {
		return
	}
	var page struct {
		BookID     json.Number `json:"bookId"`
		SignStatus string      `json:"signStatus"`
		AuthorInfo struct {
			AuthorID   string `json:"authorId"`
			AuthorName string `json:"authorName"`
			Avatar     string `json:"avatar"`
		} `json:"authorInfo"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&page); err != nil {
		return
	}
	bookID := page.BookID.String()
	if profile.URL == "" && bookID != "" {
		profile.URL = CanonicalBookURL(bookID)
	}
	profile.Author.ID = firstNonEmpty(profile.Author.ID, page.AuthorInfo.AuthorID)
	profile.Author.Name = firstNonEmpty(profile.Author.Name, page.AuthorInfo.AuthorName)
	profile.Author.Avatar = firstNonEmpty(profile.Author.Avatar, page.AuthorInfo.Avatar)
	if page.SignStatus != "" {
		profile.Tags = appendUnique(profile.Tags, page.SignStatus)
	}
}

func extractAssignedObject(source string, name string) (string, error) {
	idx := strings.Index(source, name)
	if idx < 0 {
		return "", fmt.Errorf("missing qidian page json")
	}
	eq := strings.Index(source[idx:], "=")
	if eq < 0 {
		return "", fmt.Errorf("missing qidian page json assignment")
	}
	startSearch := idx + eq + 1
	startRel := strings.Index(source[startSearch:], "{")
	if startRel < 0 {
		return "", fmt.Errorf("missing qidian page json object")
	}
	start := startSearch + startRel
	end, ok := matchingBrace(source, start)
	if !ok {
		return "", fmt.Errorf("unterminated qidian page json object")
	}
	return source[start : end+1], nil
}

func matchingBrace(source string, start int) (int, bool) {
	depth := 0
	var quote byte
	escaped := false
	lineComment := false
	blockComment := false
	for i := start; i < len(source); i++ {
		c := source[i]
		if lineComment {
			if c == '\n' || c == '\r' {
				lineComment = false
			}
			continue
		}
		if blockComment {
			if c == '*' && i+1 < len(source) && source[i+1] == '/' {
				blockComment = false
				i++
			}
			continue
		}
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' || c == '`' {
			quote = c
			continue
		}
		if c == '/' && i+1 < len(source) {
			switch source[i+1] {
			case '/':
				lineComment = true
				i++
				continue
			case '*':
				blockComment = true
				i++
				continue
			}
		}
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

func normalizeJSObject(raw string) ([]byte, error) {
	raw = stripJSComments(raw)
	converted, err := convertSingleQuotedStrings(raw)
	if err != nil {
		return nil, err
	}
	converted = jsKeyRE.ReplaceAllString(converted, `${1}"${2}":`)
	for {
		next := trailingCommaRE.ReplaceAllString(converted, `$1`)
		if next == converted {
			break
		}
		converted = next
	}
	if !json.Valid([]byte(converted)) {
		return nil, fmt.Errorf("invalid qidian page json")
	}
	return []byte(converted), nil
}

func stripJSComments(source string) string {
	var out strings.Builder
	var quote byte
	escaped := false
	lineComment := false
	blockComment := false
	for i := 0; i < len(source); i++ {
		c := source[i]
		if lineComment {
			if c == '\n' || c == '\r' {
				lineComment = false
				out.WriteByte(c)
			}
			continue
		}
		if blockComment {
			if c == '*' && i+1 < len(source) && source[i+1] == '/' {
				blockComment = false
				i++
			}
			continue
		}
		if quote != 0 {
			out.WriteByte(c)
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' || c == '`' {
			quote = c
			out.WriteByte(c)
			continue
		}
		if c == '/' && i+1 < len(source) {
			switch source[i+1] {
			case '/':
				lineComment = true
				i++
				continue
			case '*':
				blockComment = true
				i++
				continue
			}
		}
		out.WriteByte(c)
	}
	return out.String()
}

func convertSingleQuotedStrings(source string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(source); i++ {
		c := source[i]
		switch c {
		case '\'':
			value, next, err := readSingleQuotedString(source, i)
			if err != nil {
				return "", err
			}
			out.WriteString(strconv.Quote(value))
			i = next
		case '"':
			next := copyQuotedString(&out, source, i, '"')
			i = next
		default:
			out.WriteByte(c)
		}
	}
	return out.String(), nil
}

func readSingleQuotedString(source string, start int) (string, int, error) {
	var out strings.Builder
	escaped := false
	for i := start + 1; i < len(source); i++ {
		c := source[i]
		if escaped {
			switch c {
			case 'n':
				out.WriteByte('\n')
			case 'r':
				out.WriteByte('\r')
			case 't':
				out.WriteByte('\t')
			default:
				out.WriteByte(c)
			}
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '\'' {
			return out.String(), i, nil
		}
		out.WriteByte(c)
	}
	return "", 0, fmt.Errorf("unterminated qidian page json string")
}

func copyQuotedString(out *strings.Builder, source string, start int, quote byte) int {
	out.WriteByte(source[start])
	escaped := false
	for i := start + 1; i < len(source); i++ {
		c := source[i]
		out.WriteByte(c)
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == quote {
			return i
		}
	}
	return len(source) - 1
}

func bookIDFromDocument(doc *goquery.Document) string {
	for _, value := range []string{
		doc.Find("#bookImg").AttrOr("data-bid", ""),
		doc.Find("#readBtn").AttrOr("data-bid", ""),
		doc.Find("#bookCatalogSection").AttrOr("data-bid", ""),
		metaContent(doc, "og:url"),
		metaContent(doc, "og:novel:read_url"),
	} {
		if id := idFromPath(bookIDPathRE, normalizeQidianURL(value)); id != "" {
			return id
		}
		if strings.TrimSpace(value) != "" && allDigits(strings.TrimSpace(value)) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func bookIDFromURL(rawURL string) string {
	if parts, ok := ParseURL(rawURL); ok {
		return parts.BookID
	}
	return idFromPath(bookIDPathRE, rawURL)
}

func idFromPath(re *regexp.Regexp, value string) string {
	match := re.FindStringSubmatch(value)
	if len(match) >= 2 {
		return match[1]
	}
	return ""
}

func metaContent(doc *goquery.Document, property string) string {
	if doc == nil || property == "" {
		return ""
	}
	return strings.TrimSpace(doc.Find(`meta[property="`+property+`"]`).AttrOr("content", ""))
}

func titleFromDocument(doc *goquery.Document) string {
	title := cleanText(doc.Find("title").First().Text())
	if title == "" {
		return ""
	}
	for _, sep := range []string{"(", "（", "-"} {
		if idx := strings.Index(title, sep); idx > 0 {
			return title[:idx]
		}
	}
	return title
}

func selectionHTMLText(s *goquery.Selection) string {
	if s == nil || s.Length() == 0 {
		return ""
	}
	raw, err := s.Html()
	if err != nil {
		return cleanText(s.Text())
	}
	return htmlFragmentText(raw)
}

func htmlFragmentText(fragment string) string {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return ""
	}
	fragment = brRE.ReplaceAllString(fragment, "\n")
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div>" + fragment + "</div>"))
	if err != nil {
		return normalizeMultilineText(stdhtml.UnescapeString(fragment))
	}
	return normalizeMultilineText(doc.Find("div").First().Text())
}

func normalizeMultilineText(value string) string {
	value = stdhtml.UnescapeString(value)
	lines := strings.Split(value, "\n")
	var out []string
	for _, line := range lines {
		line = cleanText(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func cleanText(value string) string {
	value = stdhtml.UnescapeString(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}

func parseChineseNumber(value string) int64 {
	match := numberWithUnitRE.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) == 0 {
		return 0
	}
	number, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0
	}
	switch match[2] {
	case "亿":
		number *= 100000000
	case "万":
		number *= 10000
	}
	return int64(number + 0.5)
}

func chapterCountFromText(value string) int {
	match := chapterCountRE.FindStringSubmatch(cleanText(value))
	if len(match) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(match[1])
	return n
}

func parseChapterPublishedAt(value string) time.Time {
	match := publishTimeRE.FindStringSubmatch(value)
	if len(match) < 2 {
		return time.Time{}
	}
	return parseQidianTime(match[1])
}

func parseChapterWordCount(value string) int64 {
	match := wordCountRE.FindStringSubmatch(value)
	if len(match) < 2 {
		return 0
	}
	n, _ := strconv.ParseInt(match[1], 10, 64)
	return n
}

func parseQidianTime(value string) time.Time {
	value = cleanText(updatePrefixRE.ReplaceAllString(value, ""))
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

func countChapters(volumes []BookVolume) int {
	total := 0
	for _, volume := range volumes {
		total += len(volume.Chapters)
	}
	return total
}

func needsCompleteCatalog(profile *BookProfile) bool {
	if profile == nil {
		return false
	}
	parsedChapters := countChapters(profile.Volumes)
	if profile.ChapterCount > 0 && parsedChapters < profile.ChapterCount {
		return true
	}
	return len(profile.Volumes) == 1 && cleanText(profile.Volumes[0].Title) == "最近更新"
}

func lastChapter(volumes []BookVolume) Chapter {
	for i := len(volumes) - 1; i >= 0; i-- {
		chapters := volumes[i].Chapters
		if len(chapters) > 0 {
			return chapters[len(chapters)-1]
		}
	}
	return Chapter{}
}

func normalizeQidianURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	if strings.HasPrefix(value, "//") {
		return "https:" + value
	}
	if strings.HasPrefix(value, "/") {
		return BaseURL + value
	}
	return value
}

func appendUnique(values []string, value string) []string {
	value = cleanText(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func splitPath(parsed *url.URL) []string {
	if parsed == nil {
		return nil
	}
	pathValue := strings.Trim(parsed.EscapedPath(), "/")
	if pathValue == "" {
		return nil
	}
	return strings.Split(pathValue, "/")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
