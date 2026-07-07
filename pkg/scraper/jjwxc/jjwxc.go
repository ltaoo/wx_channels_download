package jjwxc

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
	PlatformID = "jjwxc"
	BaseURL    = "https://m.jjwxc.net"
)

var (
	digitsRE        = regexp.MustCompile(`^\d+$`)
	chapterPathRE   = regexp.MustCompile(`/book2/(\d+)/(\d+)`)
	titlePrefixRE   = regexp.MustCompile(`^\d+\.\s*`)
	updatePreviewRE = regexp.MustCompile(`\s+\S*……$`)
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
		UserAgent:  DefaultUserAgent(),
	}
}

func DefaultUserAgent() string {
	return "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
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
	switch host {
	case "m.jjwxc.net":
		segments := novelprofile.SplitPath(parsed)
		if len(segments) >= 2 && segments[0] == "book2" && digitsRE.MatchString(segments[1]) {
			return URLParts{BookID: segments[1], Canonical: CanonicalBookURL(segments[1])}, true
		}
	case "www.jjwxc.net", "jjwxc.net":
		if strings.EqualFold(parsed.EscapedPath(), "/onebook.php") {
			bookID := strings.TrimSpace(parsed.Query().Get("novelid"))
			if digitsRE.MatchString(bookID) {
				return URLParts{BookID: bookID, Canonical: CanonicalBookURL(bookID)}, true
			}
		}
	}
	return URLParts{}, false
}

func CanonicalBookURL(bookID string) string {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return ""
	}
	return BaseURL + "/book2/" + url.PathEscape(bookID)
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
		return nil, fmt.Errorf("unsupported jjwxc book url: %s", id)
	}
	htmlText, err := novelprofile.FetchHTML(ctx, c.HTTPClient, parts.Canonical, "https://www.jjwxc.net/", c.UserAgent, c.Cookie)
	if err != nil {
		return nil, err
	}
	return ParseBookProfileHTML(parts.Canonical, htmlText)
}

func ParseBookProfile(reqURL string, body []byte) (*BookProfile, error) {
	htmlText, err := novelprofile.DecodeHTML(body, "text/html; charset=gb18030")
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
		PageHTML: htmlText,
		Category: categoryFromInfo(doc),
	}
	profile.Title = novelprofile.CleanText(novelprofile.FirstNonEmpty(
		doc.Find("[data-novelname]").First().AttrOr("data-novelname", ""),
		titleFromDocument(doc),
	))
	profile.Author = Author{
		Name: novelprofile.CleanText(doc.Find(".authorname-content a").First().Text()),
		URL:  novelprofile.NormalizeURL(novelprofile.Attr(doc.Find(".authorname-content a").First(), "href"), reqURL, BaseURL),
	}
	profile.Description = strings.TrimPrefix(novelprofile.SelectionText(doc.Find("#novelintro").First()), "简介：")
	profile.Description = strings.TrimSpace(profile.Description)
	profile.Tags = tags(doc)
	profile.Status, profile.WordCount = statusAndWords(doc)
	profile.Volumes = []BookVolume{{Idx: 1, Title: "章节列表", Chapters: chapters(doc, reqURL, parts.BookID)}}
	if len(profile.Volumes[0].Chapters) == 0 {
		profile.Volumes = nil
	}
	profile.ChapterCount = novelprofile.CountChapters(profile.Volumes)
	profile.LatestChapter = novelprofile.LastChapter(profile.Volumes)
	if profile.Title == "" {
		return nil, fmt.Errorf("missing jjwxc book title")
	}
	return profile, nil
}

func categoryFromInfo(doc *goquery.Document) string {
	var out string
	doc.Find("#left li").EachWithBreak(func(_ int, item *goquery.Selection) bool {
		text := novelprofile.CleanText(item.Text())
		if strings.HasPrefix(text, "类型：") {
			out = strings.TrimSpace(strings.TrimPrefix(text, "类型："))
			return false
		}
		return true
	})
	return out
}

func statusAndWords(doc *goquery.Document) (string, string) {
	var status, words string
	doc.Find("#left li").EachWithBreak(func(_ int, item *goquery.Selection) bool {
		text := novelprofile.CleanText(item.Text())
		if !strings.HasPrefix(text, "状态：") {
			return true
		}
		value := strings.TrimSpace(strings.TrimPrefix(text, "状态："))
		parts := strings.Split(value, "/")
		if len(parts) > 0 {
			if strings.Contains(parts[len(parts)-1], "字") {
				words = strings.TrimSpace(parts[len(parts)-1])
				parts = parts[:len(parts)-1]
			}
			status = strings.Join(parts, "/")
		}
		return false
	})
	return status, words
}

func tags(doc *goquery.Document) []string {
	var out []string
	doc.Find(`#left a[href^="/assort/"]`).Each(func(_ int, item *goquery.Selection) {
		out = novelprofile.AddUnique(out, item.Text())
	})
	return out
}

func chapters(doc *goquery.Document, pageURL, bookID string) []Chapter {
	var out []Chapter
	seen := map[string]bool{}
	doc.Find("#chapter_list_box a[href]").Each(func(_ int, item *goquery.Selection) {
		href := novelprofile.NormalizeURL(novelprofile.Attr(item, "href"), pageURL, BaseURL)
		match := chapterPathRE.FindStringSubmatch(href)
		if len(match) != 3 || (bookID != "" && match[1] != bookID) || seen[href] {
			return
		}
		title, locked := chapterTitle(item)
		if title == "" {
			return
		}
		seen[href] = true
		out = append(out, Chapter{
			Idx:    len(out) + 1,
			ID:     match[2],
			Title:  title,
			URL:    href,
			Locked: locked,
		})
	})
	return out
}

func chapterTitle(item *goquery.Selection) (string, bool) {
	locked := strings.Contains(item.Text(), "[锁]") || strings.Contains(novelprofile.Attr(item, "class"), "redmanagertext")
	clone := item.Clone()
	clone.Find("span").Remove()
	text := clone.Text()
	for _, sep := range []string{"\u00a0", "&nbsp;"} {
		if idx := strings.Index(text, sep); idx >= 0 {
			text = text[:idx]
		}
	}
	text = titlePrefixRE.ReplaceAllString(novelprofile.CleanText(text), "")
	text = updatePreviewRE.ReplaceAllString(text, "")
	if locked && (text == "[锁]" || strings.Contains(text, "此章节已锁")) {
		text = "[锁定章节]"
	}
	if text == "" && locked {
		text = "[锁定章节]"
	}
	return text, locked
}

func titleFromDocument(doc *goquery.Document) string {
	title := novelprofile.CleanText(doc.Find("title").First().Text())
	title = strings.TrimPrefix(title, "《")
	if idx := strings.Index(title, "》"); idx >= 0 {
		return title[:idx]
	}
	return title
}
