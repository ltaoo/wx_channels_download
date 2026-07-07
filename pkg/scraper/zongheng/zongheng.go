package zongheng

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/pkg/scraper/novelprofile"
)

const (
	PlatformID = "zongheng"
	BaseURL    = "https://www.zongheng.com"
)

var (
	digitsRE          = regexp.MustCompile(`^\d+$`)
	legacyBookPathRE  = regexp.MustCompile(`^/book/(\d+)\.html$`)
	chapterPathRE     = regexp.MustCompile(`/chapter/(\d+)/(\d+)\.html`)
	jsStringCache     = map[string]*regexp.Regexp{}
	jsNumberCache     = map[string]*regexp.Regexp{}
	wordCountInlineRE = regexp.MustCompile(`totalWords:(\d+)`)
	displayWordsRE    = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?\s*万字数)`)
)

type URLParts = novelprofile.URLParts
type Author = novelprofile.Author
type Chapter = novelprofile.Chapter
type BookVolume = novelprofile.Volume
type BookProfile = novelprofile.BookProfile

type Client struct {
	HTTPClient novelprofile.HTTPClient
	BaseURL    string
	UserAgent  string
	Cookie     string
}

func NewClient(client novelprofile.HTTPClient) *Client {
	return &Client{
		HTTPClient: client,
		BaseURL:    BaseURL,
		UserAgent:  novelprofile.DefaultUserAgent,
	}
}

func DefaultUserAgent() string {
	return novelprofile.DefaultUserAgent
}

func ParseURL(rawURL string) (URLParts, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if digitsRE.MatchString(rawURL) {
		return URLParts{BookID: rawURL, Canonical: CanonicalBookURL(rawURL)}, true
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return URLParts{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "www.zongheng.com" && host != "book.zongheng.com" && host != "m.zongheng.com" {
		return URLParts{}, false
	}
	segments := novelprofile.SplitPath(parsed)
	if len(segments) >= 2 && (segments[0] == "detail" || segments[0] == "book") && digitsRE.MatchString(segments[1]) {
		return URLParts{BookID: segments[1], Canonical: CanonicalBookURL(segments[1])}, true
	}
	if match := legacyBookPathRE.FindStringSubmatch(parsed.EscapedPath()); len(match) == 2 {
		return URLParts{BookID: match[1], Canonical: CanonicalBookURL(match[1])}, true
	}
	return URLParts{}, false
}

func CanonicalBookURL(bookID string) string {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return ""
	}
	return BaseURL + "/detail/" + url.PathEscape(bookID)
}

func FetchBookProfile(id string) (*BookProfile, error) {
	return NewClient(nil).FetchBookProfile(id)
}

func (c *Client) FetchBookProfile(id string) (*BookProfile, error) {
	return c.FetchBookProfileContext(context.Background(), id)
}

func (c *Client) FetchBookProfileContext(ctx context.Context, id string) (*BookProfile, error) {
	parts, ok := ParseURL(id)
	if !ok {
		return nil, fmt.Errorf("unsupported zongheng book url: %s", id)
	}
	htmlText, err := novelprofile.FetchHTML(ctx, c.HTTPClient, parts.Canonical, BaseURL+"/", c.UserAgent, c.Cookie)
	if err != nil {
		return nil, err
	}
	return ParseBookProfileHTML(parts.Canonical, htmlText)
}

func ParseBookProfile(reqURL string, body []byte) (*BookProfile, error) {
	htmlText, err := novelprofile.DecodeHTML(body, "")
	if err != nil {
		return nil, err
	}
	return ParseBookProfileHTML(reqURL, htmlText)
}

func ParseBookProfileHTML(reqURL string, htmlText string) (*BookProfile, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	parts, _ := ParseURL(reqURL)
	profile := &BookProfile{
		URL:      novelprofile.FirstNonEmpty(parts.Canonical, reqURL),
		Category: novelprofile.CleanText(novelprofile.MetaContent(doc, "og:novel:category")),
		Status:   novelprofile.CleanText(novelprofile.MetaContent(doc, "og:novel:status")),
		PageHTML: htmlText,
	}
	profile.Title = novelprofile.CleanText(novelprofile.FirstNonEmpty(
		nzhString(htmlText, "bookName"),
		novelprofile.MetaContent(doc, "og:novel:book_name"),
		novelprofile.MetaContent(doc, "og:title"),
		doc.Find(".book-info--title span").First().Text(),
		titleBefore(doc.Find("title").First().Text(), "("),
	))
	profile.Description = novelprofile.FirstNonEmpty(
		novelprofile.HTMLText(nzhString(htmlText, "description")),
		cleanOGDescription(novelprofile.MetaContent(doc, "og:description")),
		novelprofile.CleanText(novelprofile.MetaContent(doc, "description")),
	)
	profile.CoverURL = novelprofile.NormalizeURL(novelprofile.FirstNonEmpty(
		nzhString(htmlText, "bookCover"),
		novelprofile.MetaContent(doc, "og:image"),
		novelprofile.Attr(doc.Find(".book-info--coverImage img").First(), "src"),
	), reqURL, BaseURL)
	profile.Author = Author{
		ID:     nzhNumber(htmlText, "authorId"),
		Name:   novelprofile.CleanText(novelprofile.FirstNonEmpty(nzhString(htmlText, "pseudonym"), novelprofile.MetaContent(doc, "og:novel:author"), doc.Find(".author-info--name").First().Text())),
		URL:    novelprofile.NormalizeURL(doc.Find(".author-info--name").First().AttrOr("href", ""), reqURL, BaseURL),
		Avatar: novelprofile.NormalizeURL(nzhString(htmlText, "authorImage"), reqURL, BaseURL),
	}
	profile.DisplayWordCount = displayWords(doc, htmlText)
	profile.WordCount = nzhNumber(htmlText, "totalWords")
	profile.UpdateTime = novelprofile.CleanText(novelprofile.FirstNonEmpty(
		novelprofile.MetaContent(doc, "og:novel:update_time"),
		nzhString(htmlText, "latestDateMsg"),
	))
	profile.LatestChapter = Chapter{
		ID:    nzhNumber(htmlText, "latestChapterId"),
		Title: novelprofile.CleanText(novelprofile.FirstNonEmpty(nzhString(htmlText, "latestChapterName"), novelprofile.MetaContent(doc, "og:novel:latest_chapter_name"))),
		URL: novelprofile.NormalizeURL(novelprofile.FirstNonEmpty(
			novelprofile.MetaContent(doc, "og:novel:latest_chapter_url"),
			chapterURL(parts.BookID, nzhNumber(htmlText, "latestChapterId")),
		), reqURL, BaseURL),
	}
	profile.Volumes = []BookVolume{{Idx: 1, Title: "详情页章节", Chapters: collectChapters(doc, reqURL, parts.BookID, htmlText, profile.LatestChapter)}}
	if len(profile.Volumes[0].Chapters) == 0 {
		profile.Volumes = nil
	}
	profile.ChapterCount = novelprofile.CountChapters(profile.Volumes)
	if profile.LatestChapter.Title == "" {
		profile.LatestChapter = novelprofile.LastChapter(profile.Volumes)
	}
	if profile.Title == "" {
		return nil, fmt.Errorf("missing zongheng book title")
	}
	return profile, nil
}

func collectChapters(doc *goquery.Document, pageURL, bookID, htmlText string, latest Chapter) []Chapter {
	var chapters []Chapter
	seen := map[string]bool{}
	add := func(chapter Chapter) {
		chapter.Title = novelprofile.CleanText(chapter.Title)
		chapter.URL = novelprofile.NormalizeURL(chapter.URL, pageURL, BaseURL)
		if chapter.Title == "" || chapter.URL == "" || seen[chapter.URL] {
			return
		}
		chapter.Idx = len(chapters) + 1
		seen[chapter.URL] = true
		chapters = append(chapters, chapter)
	}
	add(Chapter{
		ID:    nzhNumber(htmlText, "firstChapterId"),
		Title: nzhString(htmlText, "firstChapterName"),
		URL:   chapterURL(bookID, nzhNumber(htmlText, "firstChapterId")),
	})
	doc.Find("a[href]").Each(func(_ int, item *goquery.Selection) {
		href := novelprofile.NormalizeURL(novelprofile.Attr(item, "href"), pageURL, BaseURL)
		if !chapterPathRE.MatchString(href) {
			return
		}
		match := chapterPathRE.FindStringSubmatch(href)
		id := ""
		if len(match) == 3 {
			id = match[2]
		}
		add(Chapter{ID: id, Title: item.Text(), URL: href})
	})
	add(latest)
	return chapters
}

func nzhString(htmlText, key string) string {
	re := jsStringRE(key)
	match := re.FindStringSubmatch(htmlText)
	if len(match) != 2 {
		return ""
	}
	value, err := strconv.Unquote(`"` + match[1] + `"`)
	if err != nil {
		return strings.TrimSpace(match[1])
	}
	return strings.TrimSpace(value)
}

func nzhNumber(htmlText, key string) string {
	re := jsNumberRE(key)
	match := re.FindStringSubmatch(htmlText)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func jsStringRE(key string) *regexp.Regexp {
	if re := jsStringCache[key]; re != nil {
		return re
	}
	re := regexp.MustCompile(regexp.QuoteMeta(key) + `:"((?:\\.|[^"\\])*)"`)
	jsStringCache[key] = re
	return re
}

func jsNumberRE(key string) *regexp.Regexp {
	if re := jsNumberCache[key]; re != nil {
		return re
	}
	re := regexp.MustCompile(regexp.QuoteMeta(key) + `:(\d+)`)
	jsNumberCache[key] = re
	return re
}

func displayWords(doc *goquery.Document, htmlText string) string {
	body := novelprofile.CleanText(doc.Find("body").Text())
	if match := displayWordsRE.FindStringSubmatch(body); len(match) == 2 {
		return strings.TrimSuffix(match[1], "字数") + "字"
	}
	if value := nzhNumber(htmlText, "totalWords"); value != "" {
		return value + "字"
	}
	if match := wordCountInlineRE.FindStringSubmatch(htmlText); len(match) == 2 {
		return match[1] + "字"
	}
	return ""
}

func cleanOGDescription(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, `content="`)
	value = strings.Trim(value, `"`)
	for _, marker := range []string{"观看小说：", "本站更新及时，无弹窗广告，欢迎光临(m.zongheng.com)观看小说："} {
		if idx := strings.LastIndex(value, marker); idx >= 0 {
			value = value[idx+len(marker):]
			break
		}
	}
	return novelprofile.CleanMultiline(value)
}

func chapterURL(bookID, chapterID string) string {
	bookID = strings.TrimSpace(bookID)
	chapterID = strings.TrimSpace(chapterID)
	if bookID == "" || chapterID == "" {
		return ""
	}
	return "https://read.zongheng.com/chapter/" + bookID + "/" + chapterID + ".html"
}

func titleBefore(value, marker string) string {
	value = strings.TrimSpace(value)
	if idx := strings.Index(value, marker); idx >= 0 {
		return value[:idx]
	}
	return value
}
