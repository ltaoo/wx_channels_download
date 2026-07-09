package sfacg

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
	PlatformID = "sfacg"
	BaseURL    = "https://book.sfacg.com"
)

var (
	digitsRE      = regexp.MustCompile(`^\d+$`)
	chapterPathRE = regexp.MustCompile(`/Novel/(\d+)/(\d+)/(\d+)/?`)
	wordStatusRE  = regexp.MustCompile(`字数：([^\[]+)(?:\[([^\]]+)])?`)
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
	if strings.ToLower(parsed.Hostname()) != "book.sfacg.com" {
		return URLParts{}, false
	}
	segments := novelprofile.SplitPath(parsed)
	if len(segments) >= 2 && strings.EqualFold(segments[0], "Novel") && digitsRE.MatchString(segments[1]) {
		return URLParts{BookID: segments[1], Canonical: CanonicalBookURL(segments[1])}, true
	}
	return URLParts{}, false
}

func CanonicalBookURL(bookID string) string {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return ""
	}
	return BaseURL + "/Novel/" + url.PathEscape(bookID) + "/"
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
		return nil, fmt.Errorf("unsupported sfacg book url: %s", id)
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
		URL:         novelprofile.FirstNonEmpty(parts.Canonical, reqURL),
		Title:       novelprofile.CleanText(doc.Find(".summary-content h1.title .text").First().Text()),
		Description: novelprofile.SelectionText(doc.Find(".summary-content .introduce").First()),
		CoverURL:    novelprofile.NormalizeURL(novelprofile.Attr(doc.Find(".summary-pic img").First(), "src"), reqURL, BaseURL),
		PageHTML:    htmlText,
	}
	profile.Author = Author{
		Name:   novelprofile.CleanText(doc.Find(".author-name span").First().Text()),
		Avatar: novelprofile.NormalizeURL(novelprofile.Attr(doc.Find(".author-mask img").First(), "src"), reqURL, BaseURL),
	}
	parseInfoRows(doc, profile)
	profile.Tags = tags(doc)
	if latest := latestChapter(doc, reqURL, parts.BookID); latest.Title != "" {
		profile.LatestChapter = latest
		profile.Volumes = []BookVolume{{Idx: 1, Title: "详情页预读", Chapters: []Chapter{latest}}}
		profile.ChapterCount = 1
	}
	if profile.Title == "" {
		return nil, fmt.Errorf("missing sfacg book title")
	}
	return profile, nil
}

func parseInfoRows(doc *goquery.Document, profile *BookProfile) {
	doc.Find(".count-detail .text").Each(func(_ int, item *goquery.Selection) {
		text := novelprofile.CleanText(item.Text())
		switch {
		case strings.HasPrefix(text, "类型："):
			profile.Category = strings.TrimSpace(strings.TrimPrefix(text, "类型："))
		case strings.HasPrefix(text, "更新："):
			profile.UpdateTime = strings.TrimSpace(strings.TrimPrefix(text, "更新："))
		case strings.HasPrefix(text, "字数："):
			match := wordStatusRE.FindStringSubmatch(text)
			if len(match) >= 2 {
				profile.WordCount = strings.TrimSpace(match[1])
			}
			if len(match) >= 3 {
				profile.Status = strings.TrimSpace(match[2])
			}
		}
	})
}

func tags(doc *goquery.Document) []string {
	var out []string
	doc.Find(".tag-list .text").Each(func(_ int, item *goquery.Selection) {
		out = novelprofile.AddUnique(out, item.Text())
	})
	return out
}

func latestChapter(doc *goquery.Document, pageURL, bookID string) Chapter {
	link := doc.Find(".chapter-title a.link[href]").First()
	href := novelprofile.NormalizeURL(novelprofile.Attr(link, "href"), pageURL, BaseURL)
	match := chapterPathRE.FindStringSubmatch(href)
	if len(match) != 4 || (bookID != "" && match[1] != bookID) {
		return Chapter{}
	}
	return Chapter{
		Idx:   1,
		ID:    match[3],
		Title: novelprofile.CleanText(link.Text()),
		URL:   href,
	}
}
