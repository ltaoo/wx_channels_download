package novelprofile

import (
	"context"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/pkg/scraper/novelsource"
)

const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"

var (
	brRE         = regexp.MustCompile(`(?i)<br\s*/?>`)
	blockCloseRE = regexp.MustCompile(`(?i)</(p|div|li|tr|h[1-6])>`)
	tagRE        = regexp.MustCompile(`<[^>]+>`)
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type URLParts struct {
	BookID    string
	Canonical string
}

type Author struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name"`
	URL    string `json:"url,omitempty"`
	Avatar string `json:"avatar,omitempty"`
	Desc   string `json:"desc,omitempty"`
}

type Chapter struct {
	Idx         int    `json:"idx"`
	ID          string `json:"id,omitempty"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Locked      bool   `json:"locked,omitempty"`
	WordCount   string `json:"word_count,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
}

type Volume struct {
	Idx      int       `json:"idx"`
	Title    string    `json:"title"`
	Chapters []Chapter `json:"chapters"`
}

type BookProfile struct {
	URL              string   `json:"url"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	CoverURL         string   `json:"cover_url"`
	Tags             []string `json:"tags,omitempty"`
	Category         string   `json:"category,omitempty"`
	SubCategory      string   `json:"sub_category,omitempty"`
	Status           string   `json:"status,omitempty"`
	Author           Author   `json:"author"`
	WordCount        string   `json:"word_count,omitempty"`
	DisplayWordCount string   `json:"display_word_count,omitempty"`
	UpdateTime       string   `json:"update_time,omitempty"`
	LatestChapter    Chapter  `json:"latest_chapter"`
	ChapterCount     int      `json:"chapter_count"`
	Volumes          []Volume `json:"volumes,omitempty"`
	PageHTML         string   `json:"-"`
}

func FetchHTML(ctx context.Context, client HTTPClient, rawURL, referer, userAgent, cookie string) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", FirstNonEmpty(userAgent, DefaultUserAgent))
	if strings.TrimSpace(referer) != "" {
		req.Header.Set("Referer", strings.TrimSpace(referer))
	}
	if strings.TrimSpace(cookie) != "" {
		req.Header.Set("Cookie", strings.TrimSpace(cookie))
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	text, decodeErr := DecodeHTML(body, resp.Header.Get("Content-Type"))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(text))
	}
	return text, decodeErr
}

func DecodeHTML(body []byte, contentType string) (string, error) {
	return novelsource.DecodeHTML(body, contentType)
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func CleanText(value string) string {
	value = stdhtml.UnescapeString(value)
	value = strings.ReplaceAll(value, "\u00a0", " ")
	value = strings.ReplaceAll(value, "\u3000", " ")
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}

func CleanMultiline(value string) string {
	value = stdhtml.UnescapeString(value)
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.ReplaceAll(value, "\u00a0", " ")
	lines := strings.Split(value, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = CleanText(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func HTMLText(fragment string) string {
	fragment = brRE.ReplaceAllString(fragment, "\n")
	fragment = blockCloseRE.ReplaceAllString(fragment, "\n")
	fragment = tagRE.ReplaceAllString(fragment, "")
	return CleanMultiline(fragment)
}

func SelectionText(sel *goquery.Selection) string {
	if sel == nil || sel.Length() == 0 {
		return ""
	}
	if htmlText, err := sel.Html(); err == nil && strings.TrimSpace(htmlText) != "" {
		return HTMLText(htmlText)
	}
	return CleanMultiline(sel.Text())
}

func MetaContent(doc *goquery.Document, key string) string {
	key = strings.TrimSpace(key)
	if doc == nil || key == "" {
		return ""
	}
	var out string
	doc.Find("meta").EachWithBreak(func(_ int, item *goquery.Selection) bool {
		for _, attrName := range []string{"name", "property", "itemprop"} {
			if strings.EqualFold(strings.TrimSpace(Attr(item, attrName)), key) {
				out = strings.TrimSpace(Attr(item, "content"))
				return false
			}
		}
		return true
	})
	return out
}

func Attr(sel *goquery.Selection, name string) string {
	if sel == nil || sel.Length() == 0 {
		return ""
	}
	value, _ := sel.Attr(name)
	return strings.TrimSpace(value)
}

func NormalizeURL(rawValue, pageURL, baseURL string) string {
	rawValue = strings.TrimSpace(rawValue)
	if rawValue == "" {
		return ""
	}
	if strings.HasPrefix(rawValue, "//") {
		return "https:" + rawValue
	}
	parsed, err := url.Parse(rawValue)
	if err != nil {
		return rawValue
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	base := strings.TrimSpace(pageURL)
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/"
	}
	baseURLParsed, err := url.Parse(base)
	if err != nil {
		return rawValue
	}
	return baseURLParsed.ResolveReference(parsed).String()
}

func SplitPath(parsed *url.URL) []string {
	if parsed == nil {
		return nil
	}
	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	out := parts[:0]
	for _, part := range parts {
		part, err := url.PathUnescape(part)
		if err != nil {
			part = strings.TrimSpace(part)
		}
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func AddUnique(values []string, value string) []string {
	value = CleanText(value)
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

func CountChapters(volumes []Volume) int {
	total := 0
	for _, volume := range volumes {
		total += len(volume.Chapters)
	}
	return total
}

func LastChapter(volumes []Volume) Chapter {
	for i := len(volumes) - 1; i >= 0; i-- {
		chapters := volumes[i].Chapters
		if len(chapters) > 0 {
			return chapters[len(chapters)-1]
		}
	}
	return Chapter{}
}
