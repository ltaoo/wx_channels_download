package ciweimao

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/pkg/scraper/novelprofile"
)

const (
	PlatformID = "ciweimao"
	BaseURL    = "https://www.ciweimao.com"
)

var (
	digitsRE      = regexp.MustCompile(`^\d+$`)
	chapterPathRE = regexp.MustCompile(`/chapter/(\d+)`)
	wordCountRE   = regexp.MustCompile(`总字数：\s*([0-9.万]+)`)
	latestRE      = regexp.MustCompile(`最后更新：\s*(.*?)\s*(?:\[\s*(.*?)\s*])?$`)
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
	if host != "www.ciweimao.com" && host != "wap.ciweimao.com" {
		return URLParts{}, false
	}
	segments := novelprofile.SplitPath(parsed)
	if len(segments) >= 2 && segments[0] == "book" && digitsRE.MatchString(segments[1]) {
		return URLParts{BookID: segments[1], Canonical: CanonicalBookURL(segments[1])}, true
	}
	return URLParts{}, false
}

func CanonicalBookURL(bookID string) string {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return ""
	}
	return BaseURL + "/book/" + url.PathEscape(bookID)
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
		return nil, fmt.Errorf("unsupported ciweimao book url: %s", id)
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
	titleNode := doc.Find(".book-info h1.title").First().Clone()
	titleNode.Find("span").Remove()
	profile := &BookProfile{
		URL:         novelprofile.FirstNonEmpty(parts.Canonical, reqURL),
		Title:       novelprofile.CleanText(novelprofile.FirstNonEmpty(titleNode.Text(), novelprofile.MetaContent(doc, "og:novel:book_name"))),
		Description: novelprofile.SelectionText(doc.Find(".book-desc").First()),
		CoverURL:    novelprofile.NormalizeURL(novelprofile.FirstNonEmpty(novelprofile.Attr(doc.Find(".cover img").First(), "src"), novelprofile.MetaContent(doc, "og:image")), reqURL, BaseURL),
		Category:    novelprofile.CleanText(novelprofile.MetaContent(doc, "og:novel:category")),
		Status:      novelprofile.CleanText(doc.Find(".book-info .update-state").First().Text()),
		PageHTML:    htmlText,
	}
	profile.Author = Author{
		Name: novelprofile.CleanText(novelprofile.FirstNonEmpty(novelprofile.MetaContent(doc, "og:novel:author"), doc.Find(".book-info h1.title span a").First().Text())),
		URL:  novelprofile.NormalizeURL(novelprofile.Attr(doc.Find(".book-info h1.title span a").First(), "href"), reqURL, BaseURL),
	}
	profile.Tags = tags(doc)
	profile.WordCount = wordCount(doc)
	profile.LatestChapter = latestChapter(doc, reqURL)
	profile.UpdateTime = profile.LatestChapter.PublishedAt
	profile.Volumes = volumes(doc, reqURL)
	profile.ChapterCount = novelprofile.CountChapters(profile.Volumes)
	if profile.LatestChapter.Title == "" {
		profile.LatestChapter = novelprofile.LastChapter(profile.Volumes)
	}
	if profile.Title == "" {
		return nil, fmt.Errorf("missing ciweimao book title")
	}
	return profile, nil
}

func tags(doc *goquery.Document) []string {
	var out []string
	doc.Find(".label-box .label").Each(func(_ int, item *goquery.Selection) {
		out = novelprofile.AddUnique(out, item.Text())
	})
	return out
}

func wordCount(doc *goquery.Document) string {
	text := novelprofile.CleanText(doc.Find(".book-info .book-grade").First().Text())
	if match := wordCountRE.FindStringSubmatch(text); len(match) == 2 {
		return match[1]
	}
	return ""
}

func latestChapter(doc *goquery.Document, pageURL string) Chapter {
	item := doc.Find(".book-info .update-time").First()
	text := novelprofile.CleanText(item.Text())
	text = strings.TrimPrefix(text, "最后更新：")
	match := latestRE.FindStringSubmatch("最后更新：" + text)
	title := text
	publishedAt := ""
	if len(match) == 3 {
		title = strings.TrimSpace(match[1])
		publishedAt = strings.TrimSpace(match[2])
	}
	href := novelprofile.NormalizeURL(novelprofile.Attr(item.ParentFiltered("a[href]"), "href"), pageURL, BaseURL)
	id := ""
	if match := chapterPathRE.FindStringSubmatch(href); len(match) == 2 {
		id = match[1]
	}
	return Chapter{
		ID:          id,
		Title:       novelprofile.CleanText(title),
		URL:         href,
		PublishedAt: publishedAt,
	}
}

func volumes(doc *goquery.Document, pageURL string) []BookVolume {
	var out []BookVolume
	current := -1
	ensureVolume := func(title string) {
		title = novelprofile.CleanText(novelprofile.FirstNonEmpty(title, "默认卷"))
		out = append(out, BookVolume{Idx: len(out) + 1, Title: title})
		current = len(out) - 1
	}
	doc.Find("#J_book_chapter_list").Children().Each(func(_ int, item *goquery.Selection) {
		switch goquery.NodeName(item) {
		case "h4":
			ensureVolume(item.Text())
		case "ul":
			if current < 0 {
				ensureVolume("默认卷")
			}
			item.Find("a[href]").Each(func(_ int, link *goquery.Selection) {
				href := novelprofile.NormalizeURL(novelprofile.Attr(link, "href"), pageURL, BaseURL)
				match := chapterPathRE.FindStringSubmatch(href)
				if len(match) != 2 {
					return
				}
				title := novelprofile.CleanText(link.Text())
				if title == "" {
					return
				}
				out[current].Chapters = append(out[current].Chapters, Chapter{
					Idx:   len(out[current].Chapters) + 1,
					ID:    match[1],
					Title: title,
					URL:   href,
				})
			})
		}
	})
	return out
}
