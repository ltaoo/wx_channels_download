package v2ex

import (
	"bytes"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const (
	BaseURL            = "https://www.v2ex.com"
	ContentTypeTopic   = "topic"
	defaultUserAgent   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"
	maxDescriptionRune = 180
)

var topicPathRegexp = regexp.MustCompile(`^/(?:amp/)?t/([0-9]+)(?:/|$)`)

type TopicURL struct {
	TopicID   string `json:"topic_id"`
	Canonical string `json:"canonical"`
}

type Author struct {
	Name      string `json:"name,omitempty"`
	URL       string `json:"url,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type Reply struct {
	ID          string `json:"id,omitempty"`
	No          int    `json:"no,omitempty"`
	Author      Author `json:"author,omitempty"`
	ContentHTML string `json:"content_html,omitempty"`
	ContentText string `json:"content_text,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
}

type TopicPage struct {
	ID            string   `json:"id"`
	URL           string   `json:"url"`
	CanonicalURL  string   `json:"canonical_url"`
	Title         string   `json:"title"`
	Description   string   `json:"description,omitempty"`
	NodeName      string   `json:"node_name,omitempty"`
	NodeURL       string   `json:"node_url,omitempty"`
	Author        Author   `json:"author,omitempty"`
	ContentHTML   string   `json:"content_html,omitempty"`
	ContentText   string   `json:"content_text,omitempty"`
	PublishedAt   string   `json:"published_at,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	ReplyCount    int      `json:"reply_count,omitempty"`
	ViewCount     int      `json:"view_count,omitempty"`
	FavoriteCount int      `json:"favorite_count,omitempty"`
	Replies       []Reply  `json:"replies,omitempty"`
	PageHTML      string   `json:"-"`
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	HTTPClient HTTPClient
	UserAgent  string
}

func NewClient(client HTTPClient) *Client {
	if client == nil {
		client = http.DefaultClient
	}
	return &Client{HTTPClient: client, UserAgent: defaultUserAgent}
}

func ParseTopicURL(rawURL string) (TopicURL, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return TopicURL{}, false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return TopicURL{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "www.v2ex.com" && host != "v2ex.com" {
		return TopicURL{}, false
	}
	matches := topicPathRegexp.FindStringSubmatch(parsed.EscapedPath())
	if len(matches) < 2 || matches[1] == "" {
		return TopicURL{}, false
	}
	return TopicURL{
		TopicID:   matches[1],
		Canonical: BaseURL + "/t/" + matches[1],
	}, true
}

func (c *Client) FetchTopicPage(rawURL string) (*TopicPage, error) {
	if _, ok := ParseTopicURL(rawURL); !ok {
		return nil, fmt.Errorf("unsupported v2ex topic url")
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Referer", BaseURL+"/")
	req.Header.Set("User-Agent", firstNonEmpty(c.UserAgent, defaultUserAgent))
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("v2ex HTTP %d: %s", resp.StatusCode, rawURL)
	}
	return ParseTopicPage(rawURL, resp.Body)
}

func (c *Client) BuildHTMLFromURL(rawURL string) (string, error) {
	page, err := c.FetchTopicPage(rawURL)
	if err != nil {
		return "", err
	}
	return BuildHTML(page), nil
}

func ParseTopicPage(rawURL string, r io.Reader) (*TopicPage, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return ParseTopicHTML(rawURL, string(body))
}

func ParseTopicHTML(rawURL string, htmlText string) (*TopicPage, error) {
	parts, ok := ParseTopicURL(rawURL)
	if !ok {
		return nil, fmt.Errorf("unsupported v2ex topic url")
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	page := &TopicPage{
		ID:           parts.TopicID,
		URL:          rawURL,
		CanonicalURL: parts.Canonical,
		PageHTML:     htmlText,
	}

	schema := findDiscussionPosting(doc)
	if schema != nil {
		page.Title = schema.Headline
		page.ContentText = strings.TrimSpace(schema.Text)
		page.PublishedAt = schema.DatePublished
		page.ReplyCount = schema.CommentCount
		page.Author = Author{
			Name: strings.TrimSpace(schema.Author.Name),
			URL:  normalizeURL(schema.Author.URL, page.CanonicalURL),
		}
		page.NodeName = strings.TrimSpace(schema.IsPartOf.Name)
		page.NodeURL = normalizeURL(schema.IsPartOf.URL, page.CanonicalURL)
	}

	page.CanonicalURL = firstNonEmpty(
		normalizeURL(attr(doc.Find(`link[rel="canonical"]`).First(), "href"), page.CanonicalURL),
		normalizeURL(metaContent(doc, "property", "og:url"), page.CanonicalURL),
		normalizeURL(schemaURL(schema), page.CanonicalURL),
		page.CanonicalURL,
	)
	if parsedCanonical, ok := ParseTopicURL(page.CanonicalURL); ok {
		page.ID = parsedCanonical.TopicID
		page.CanonicalURL = parsedCanonical.Canonical
	}

	page.Title = cleanTitle(firstNonEmpty(
		page.Title,
		metaContent(doc, "property", "og:title"),
		strings.TrimSpace(doc.Find("#Main .header h1").First().Text()),
		strings.TrimSpace(doc.Find("title").First().Text()),
		"v2ex_"+page.ID,
	))
	page.Description = firstNonEmpty(
		metaContent(doc, "name", "description"),
		metaContent(doc, "property", "og:description"),
		excerpt(page.ContentText),
	)
	if page.Author.AvatarURL == "" {
		page.Author.AvatarURL = normalizeURL(metaContent(doc, "name", "twitter:image"), page.CanonicalURL)
	}
	if page.Author.Name == "" || page.Author.URL == "" || page.Author.AvatarURL == "" {
		authorLink := doc.Find(`#Main .header a[href^="/member/"]`).First()
		if page.Author.Name == "" {
			page.Author.Name = strings.TrimSpace(authorLink.Text())
		}
		if page.Author.URL == "" {
			page.Author.URL = normalizeURL(attr(authorLink, "href"), page.CanonicalURL)
		}
		if page.Author.AvatarURL == "" {
			page.Author.AvatarURL = normalizeURL(attr(doc.Find("#Main .header img.avatar").First(), "src"), page.CanonicalURL)
		}
	}
	if page.PublishedAt == "" {
		page.PublishedAt = strings.TrimSpace(attr(doc.Find("#Main .header span[title]").First(), "title"))
	}
	if page.NodeName == "" || page.NodeURL == "" {
		doc.Find(`#Main .header a[href^="/go/"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
			page.NodeName = strings.TrimSpace(s.Text())
			page.NodeURL = normalizeURL(attr(s, "href"), page.CanonicalURL)
			return false
		})
	}

	page.Tags = parseTags(doc)
	topicBody := doc.Find("#Main .topic_content .markdown_body").First()
	if topicBody.Length() > 0 {
		if bodyHTML, err := topicBody.Html(); err == nil {
			page.ContentHTML = sanitizeFragment(bodyHTML, page.CanonicalURL)
		}
		if page.ContentText == "" {
			page.ContentText = compactText(topicBody.Text())
		}
	}
	if page.ContentHTML == "" && page.ContentText != "" {
		page.ContentHTML = TextToHTML(page.ContentText)
	}
	parseStats(doc, page)
	page.Replies = parseReplies(doc, page.CanonicalURL)
	if len(page.Replies) == 0 && schema != nil {
		page.Replies = schemaReplies(schema, page.CanonicalURL)
	}
	if page.ReplyCount == 0 {
		page.ReplyCount = len(page.Replies)
	}
	return page, nil
}

func BuildHTML(page *TopicPage) string {
	if page == nil {
		return ""
	}
	title := firstNonEmpty(page.Title, "v2ex_"+page.ID, "V2EX Topic")
	sourceURL := firstNonEmpty(page.CanonicalURL, page.URL)
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"zh-CN\">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n<title>")
	b.WriteString(stdhtml.EscapeString(title))
	b.WriteString("</title>\n<style>")
	b.WriteString(`body{margin:0;background:#f5f5f5;color:#222;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;line-height:1.65}main{max-width:860px;margin:0 auto;padding:28px 18px 48px}.panel{background:#fff;border:1px solid #ddd;border-radius:6px;padding:22px;margin-bottom:16px}.meta{color:#666;font-size:14px}.author{display:flex;align-items:center;gap:10px}.avatar{width:42px;height:42px;border-radius:5px}.content img,.reply-body img{max-width:100%;height:auto}.content pre,.reply-body pre{overflow:auto;background:#f7f7f7;padding:12px;border-radius:4px}.tags{display:flex;gap:8px;flex-wrap:wrap}.tag{background:#eef1f5;border-radius:4px;padding:2px 8px;color:#556;text-decoration:none}.reply{border-top:1px solid #eee;padding-top:16px;margin-top:16px}.reply-head{display:flex;justify-content:space-between;gap:12px}.source{word-break:break-all}`)
	b.WriteString("</style>\n</head>\n<body>\n<main>\n")
	b.WriteString("<article class=\"panel\">\n")
	b.WriteString("<p class=\"meta\">V2EX")
	if page.NodeName != "" {
		b.WriteString(" / ")
		if page.NodeURL != "" {
			b.WriteString(`<a href="` + stdhtml.EscapeString(page.NodeURL) + `">` + stdhtml.EscapeString(page.NodeName) + "</a>")
		} else {
			b.WriteString(stdhtml.EscapeString(page.NodeName))
		}
	}
	b.WriteString("</p>\n<h1>")
	b.WriteString(stdhtml.EscapeString(title))
	b.WriteString("</h1>\n")
	writeAuthor(&b, page.Author, page.PublishedAt)
	b.WriteString("<div class=\"content\">\n")
	if strings.TrimSpace(page.ContentHTML) != "" {
		b.WriteString(sanitizeFragment(page.ContentHTML, sourceURL))
	} else {
		b.WriteString(TextToHTML(page.ContentText))
	}
	b.WriteString("\n</div>\n")
	if len(page.Tags) > 0 {
		b.WriteString("<p class=\"tags\">")
		for _, tag := range page.Tags {
			b.WriteString(`<span class="tag">` + stdhtml.EscapeString(tag) + "</span>")
		}
		b.WriteString("</p>\n")
	}
	b.WriteString("<p class=\"meta source\">来源：")
	if sourceURL != "" {
		b.WriteString(`<a href="` + stdhtml.EscapeString(sourceURL) + `">` + stdhtml.EscapeString(sourceURL) + "</a>")
	}
	b.WriteString("</p>\n</article>\n")

	if len(page.Replies) > 0 {
		b.WriteString("<section class=\"panel\">\n<h2>回复")
		if page.ReplyCount > 0 {
			b.WriteString(" (" + strconv.Itoa(page.ReplyCount) + ")")
		}
		b.WriteString("</h2>\n")
		for _, reply := range page.Replies {
			b.WriteString("<article class=\"reply\"")
			if reply.ID != "" {
				b.WriteString(` id="reply-` + stdhtml.EscapeString(reply.ID) + `"`)
			}
			b.WriteString(">\n<div class=\"reply-head\"><div>")
			name := firstNonEmpty(reply.Author.Name, "anonymous")
			if reply.Author.URL != "" {
				b.WriteString(`<a href="` + stdhtml.EscapeString(reply.Author.URL) + `">` + stdhtml.EscapeString(name) + "</a>")
			} else {
				b.WriteString(stdhtml.EscapeString(name))
			}
			b.WriteString("</div><div class=\"meta\">")
			if reply.No > 0 {
				b.WriteString("#" + strconv.Itoa(reply.No))
			}
			if reply.PublishedAt != "" {
				if reply.No > 0 {
					b.WriteString(" · ")
				}
				b.WriteString(stdhtml.EscapeString(reply.PublishedAt))
			}
			b.WriteString("</div></div>\n<div class=\"reply-body\">")
			if strings.TrimSpace(reply.ContentHTML) != "" {
				b.WriteString(sanitizeFragment(reply.ContentHTML, sourceURL))
			} else {
				b.WriteString(TextToHTML(reply.ContentText))
			}
			b.WriteString("</div>\n</article>\n")
		}
		b.WriteString("</section>\n")
	}
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
		b.WriteString("<p>")
		b.WriteString(stdhtml.EscapeString(line))
		b.WriteString("</p>\n")
	}
	return b.String()
}

type schemaDiscussion struct {
	Type                 any                 `json:"@type"`
	Headline             string              `json:"headline"`
	Text                 string              `json:"text"`
	DatePublished        string              `json:"datePublished"`
	CommentCount         int                 `json:"commentCount"`
	URL                  string              `json:"url"`
	Author               schemaPerson        `json:"author"`
	IsPartOf             schemaPage          `json:"isPartOf"`
	Comment              schemaCommentList   `json:"comment"`
	InteractionStatistic []schemaInteraction `json:"interactionStatistic"`
}

type schemaPerson struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type schemaPage struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type schemaComment struct {
	Text          string       `json:"text"`
	DatePublished string       `json:"datePublished"`
	Author        schemaPerson `json:"author"`
}

type schemaCommentList []schemaComment

func (l *schemaCommentList) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil
	}
	if data[0] == '[' {
		var comments []schemaComment
		if err := json.Unmarshal(data, &comments); err != nil {
			return err
		}
		*l = comments
		return nil
	}
	var comment schemaComment
	if err := json.Unmarshal(data, &comment); err != nil {
		return err
	}
	*l = []schemaComment{comment}
	return nil
}

type schemaInteraction struct {
	UserInteractionCount int    `json:"userInteractionCount"`
	InteractionType      string `json:"interactionType"`
}

func findDiscussionPosting(doc *goquery.Document) *schemaDiscussion {
	var found *schemaDiscussion
	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			return true
		}
		var item schemaDiscussion
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return true
		}
		if !schemaTypeContains(item.Type, "DiscussionForumPosting") {
			return true
		}
		found = &item
		return false
	})
	return found
}

func schemaTypeContains(value any, want string) bool {
	switch v := value.(type) {
	case string:
		return strings.EqualFold(v, want)
	case []any:
		for _, item := range v {
			if schemaTypeContains(item, want) {
				return true
			}
		}
	case []string:
		for _, item := range v {
			if strings.EqualFold(item, want) {
				return true
			}
		}
	}
	return false
}

func parseReplies(doc *goquery.Document, pageURL string) []Reply {
	var replies []Reply
	doc.Find(`#Main div.cell[id^="r_"]`).Each(func(i int, s *goquery.Selection) {
		content := s.Find(".reply_content").First()
		if content.Length() == 0 {
			return
		}
		bodyHTML, _ := content.Html()
		authorLink := s.Find(`strong a[href^="/member/"]`).First()
		reply := Reply{
			ID: strings.TrimPrefix(attr(s, "id"), "r_"),
			No: atoi(strings.TrimSpace(s.Find(".no").First().Text())),
			Author: Author{
				Name:      strings.TrimSpace(authorLink.Text()),
				URL:       normalizeURL(attr(authorLink, "href"), pageURL),
				AvatarURL: normalizeURL(attr(s.Find("img.avatar").First(), "src"), pageURL),
			},
			ContentHTML: sanitizeFragment(bodyHTML, pageURL),
			ContentText: compactText(content.Text()),
			PublishedAt: strings.TrimSpace(attr(s.Find(".ago[title]").First(), "title")),
		}
		if reply.No == 0 {
			reply.No = i + 1
		}
		replies = append(replies, reply)
	})
	return replies
}

func schemaReplies(schema *schemaDiscussion, pageURL string) []Reply {
	if schema == nil {
		return nil
	}
	replies := make([]Reply, 0, len(schema.Comment))
	for i, comment := range schema.Comment {
		text := strings.TrimSpace(comment.Text)
		if text == "" {
			continue
		}
		replies = append(replies, Reply{
			No: i + 1,
			Author: Author{
				Name: strings.TrimSpace(comment.Author.Name),
				URL:  normalizeURL(comment.Author.URL, pageURL),
			},
			ContentText: text,
			ContentHTML: TextToHTML(text),
			PublishedAt: comment.DatePublished,
		})
	}
	return replies
}

func sanitizeFragment(fragment string, baseURL string) string {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`<div id="v2ex-fragment">` + fragment + `</div>`))
	if err != nil {
		return TextToHTML(fragment)
	}
	root := doc.Find("#v2ex-fragment")
	root.Find("script,style,iframe,object,embed,form,input,button,textarea").Remove()
	root.Find("*").Each(func(_ int, s *goquery.Selection) {
		node := s.Get(0)
		if node == nil {
			return
		}
		nextAttrs := node.Attr[:0]
		for _, a := range node.Attr {
			key := strings.ToLower(strings.TrimSpace(a.Key))
			if key == "" || strings.HasPrefix(key, "on") || key == "style" || key == "srcset" {
				continue
			}
			switch key {
			case "href":
				if href := sanitizeLink(a.Val, baseURL); href != "" {
					a.Val = href
					nextAttrs = append(nextAttrs, a)
				}
			case "src":
				if src := sanitizeImageSrc(a.Val, baseURL); src != "" {
					a.Val = src
					nextAttrs = append(nextAttrs, a)
				}
			default:
				nextAttrs = append(nextAttrs, a)
			}
		}
		node.Attr = nextAttrs
		if goquery.NodeName(s) == "a" {
			if href := attr(s, "href"); strings.HasPrefix(strings.ToLower(href), "http") {
				s.SetAttr("target", "_blank")
				s.SetAttr("rel", "noopener noreferrer")
			}
		}
	})
	out, err := root.Html()
	if err != nil {
		return TextToHTML(fragment)
	}
	return strings.TrimSpace(out)
}

func sanitizeLink(raw string, baseURL string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "#") {
		return raw
	}
	if strings.HasPrefix(strings.ToLower(raw), "mailto:") {
		return raw
	}
	normalized := normalizeURL(raw, baseURL)
	parsed, err := url.Parse(normalized)
	if err != nil {
		return ""
	}
	if parsed.Scheme == "http" || parsed.Scheme == "https" {
		return normalized
	}
	return ""
}

func sanitizeImageSrc(raw string, baseURL string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "data:image/") {
		return raw
	}
	normalized := normalizeURL(raw, baseURL)
	parsed, err := url.Parse(normalized)
	if err != nil {
		return ""
	}
	if parsed.Scheme == "http" || parsed.Scheme == "https" {
		return normalized
	}
	return ""
}

func normalizeURL(raw string, base string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		return "https:" + raw
	}
	parsed, err := url.Parse(raw)
	if err == nil && parsed.Scheme != "" {
		return parsed.String()
	}
	base = firstNonEmpty(base, BaseURL+"/")
	baseParsed, err := url.Parse(base)
	if err != nil || baseParsed.Scheme == "" || baseParsed.Host == "" {
		baseParsed, _ = url.Parse(BaseURL + "/")
	}
	ref, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return baseParsed.ResolveReference(ref).String()
}

func parseTags(doc *goquery.Document) []string {
	var tags []string
	seen := map[string]bool{}
	doc.Find("#Main a.tag").Each(func(_ int, s *goquery.Selection) {
		tag := compactText(s.Text())
		tag = strings.TrimPrefix(tag, "")
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			return
		}
		seen[tag] = true
		tags = append(tags, tag)
	})
	return tags
}

func parseStats(doc *goquery.Document, page *TopicPage) {
	if page == nil {
		return
	}
	text := compactText(doc.Find("#Main .topic_buttons .topic_stats").First().Text() + " " + doc.Find("#Main .header small.gray").First().Text())
	if text == "" {
		return
	}
	if matches := regexp.MustCompile(`([0-9]+)\s*次点击`).FindStringSubmatch(text); len(matches) > 1 {
		page.ViewCount = atoi(matches[1])
	}
	if matches := regexp.MustCompile(`([0-9]+)\s*人收藏`).FindStringSubmatch(text); len(matches) > 1 {
		page.FavoriteCount = atoi(matches[1])
	}
}

func writeAuthor(b *strings.Builder, author Author, publishedAt string) {
	if author.Name == "" && author.AvatarURL == "" && publishedAt == "" {
		return
	}
	b.WriteString("<div class=\"author\">")
	if author.AvatarURL != "" {
		b.WriteString(`<img class="avatar" src="` + stdhtml.EscapeString(author.AvatarURL) + `" alt="">`)
	}
	b.WriteString("<p class=\"meta\">")
	name := firstNonEmpty(author.Name, "anonymous")
	if author.URL != "" {
		b.WriteString(`<a href="` + stdhtml.EscapeString(author.URL) + `">` + stdhtml.EscapeString(name) + "</a>")
	} else {
		b.WriteString(stdhtml.EscapeString(name))
	}
	if publishedAt != "" {
		b.WriteString(" · " + stdhtml.EscapeString(publishedAt))
	}
	b.WriteString("</p></div>\n")
}

func metaContent(doc *goquery.Document, attrName string, attrValue string) string {
	selector := fmt.Sprintf(`meta[%s="%s"]`, attrName, attrValue)
	return strings.TrimSpace(attr(doc.Find(selector).First(), "content"))
}

func attr(s *goquery.Selection, key string) string {
	if s == nil || s.Length() == 0 {
		return ""
	}
	value, ok := s.Attr(key)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func schemaURL(schema *schemaDiscussion) string {
	if schema == nil {
		return ""
	}
	return schema.URL
}

func cleanTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.TrimSuffix(title, " - V2EX")
	return strings.TrimSpace(title)
}

func excerpt(text string) string {
	text = compactText(text)
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxDescriptionRune {
		return text
	}
	return string(runes[:maxDescriptionRune]) + "..."
}

func compactText(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func atoi(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	n, _ := strconv.Atoi(text)
	return n
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
