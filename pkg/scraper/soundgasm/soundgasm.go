package soundgasm

import (
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	PlatformID       = "soundgasm"
	BaseURL          = "https://soundgasm.net"
	ContentTypeAudio = "audio"
	defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
)

var (
	ErrUnsupportedURL = errors.New("unsupported soundgasm url")

	mediaRE          = regexp.MustCompile(`(?is)\b(m4a|mp3|oga|ogg|wav)\s*:\s*("([^"\\]*(?:\\.[^"\\]*)*)"|'([^'\\]*(?:\\.[^'\\]*)*)')`)
	tagRE            = regexp.MustCompile(`\[([^\]\r\n]{1,80})\]`)
	htmlBreakRE      = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlBlockEndRE   = regexp.MustCompile(`(?i)</(?:p|div|section|article|li|h[1-6])\s*>`)
	htmlTagRE        = regexp.MustCompile(`<[^>]+>`)
	collapseSpacesRE = regexp.MustCompile(`[ \t\r\f\v]+`)
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	HTTPClient HTTPClient
	UserAgent  string
}

type URLParts struct {
	Username  string `json:"username"`
	Slug      string `json:"slug"`
	Canonical string `json:"canonical"`
}

type Author struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

type Link struct {
	URL  string `json:"url,omitempty"`
	Text string `json:"text,omitempty"`
}

type AudioPage struct {
	ID              string   `json:"id"`
	URL             string   `json:"url"`
	CanonicalURL    string   `json:"canonical_url"`
	Title           string   `json:"title,omitempty"`
	Description     string   `json:"description,omitempty"`
	DescriptionHTML string   `json:"description_html,omitempty"`
	Author          Author   `json:"author,omitempty"`
	AudioURL        string   `json:"audio_url,omitempty"`
	AudioType       string   `json:"audio_type,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	Links           []Link   `json:"links,omitempty"`
	PageHTML        string   `json:"-"`
}

func NewClient(client HTTPClient) *Client {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{HTTPClient: client, UserAgent: defaultUserAgent}
}

func ParseURL(rawURL string) (URLParts, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return URLParts{}, false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return URLParts{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "soundgasm.net" && host != "www.soundgasm.net" {
		return URLParts{}, false
	}
	segments, ok := splitEscapedPath(parsed.EscapedPath())
	if !ok || len(segments) != 3 || strings.ToLower(segments[0]) != "u" {
		return URLParts{}, false
	}
	username := strings.TrimSpace(segments[1])
	slug := strings.TrimSpace(segments[2])
	if username == "" || slug == "" {
		return URLParts{}, false
	}
	return URLParts{
		Username:  username,
		Slug:      slug,
		Canonical: AudioURL(username, slug),
	}, true
}

func AudioURL(username string, slug string) string {
	username = strings.TrimSpace(username)
	slug = strings.TrimSpace(slug)
	if username == "" || slug == "" {
		return ""
	}
	return BaseURL + "/u/" + url.PathEscape(username) + "/" + url.PathEscape(slug)
}

func ProfileURL(username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return ""
	}
	return BaseURL + "/u/" + url.PathEscape(username)
}

func FetchAudioPage(rawURL string) (*AudioPage, error) {
	return NewClient(nil).FetchAudioPage(rawURL)
}

func (c *Client) FetchAudioPage(rawURL string) (*AudioPage, error) {
	parts, ok := ParseURL(rawURL)
	if !ok {
		return nil, ErrUnsupportedURL
	}
	req, err := http.NewRequest(http.MethodGet, parts.Canonical, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Referer", BaseURL+"/")
	req.Header.Set("User-Agent", firstNonEmpty(c.UserAgent, defaultUserAgent))
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch soundgasm page: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("fetch soundgasm page: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	page, err := ParseAudioPage(parts.Canonical, resp.Body)
	if err != nil {
		return nil, err
	}
	page.URL = rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		page.URL = resp.Request.URL.String()
	}
	return page, nil
}

func (c *Client) BuildHTMLFromURL(rawURL string) (string, error) {
	page, err := c.FetchAudioPage(rawURL)
	if err != nil {
		return "", err
	}
	return BuildHTML(page), nil
}

func ParseAudioPage(rawURL string, r io.Reader) (*AudioPage, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return ParseAudioHTML(rawURL, string(body))
}

func ParseAudioHTML(rawURL string, htmlText string) (*AudioPage, error) {
	parts, ok := ParseURL(rawURL)
	if !ok {
		return nil, ErrUnsupportedURL
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	page := &AudioPage{
		ID:           ContentID(parts),
		URL:          rawURL,
		CanonicalURL: parts.Canonical,
		Author: Author{
			Name: parts.Username,
			URL:  ProfileURL(parts.Username),
		},
		PageHTML: htmlText,
	}

	page.Author = parseAuthor(doc, parts)
	page.Title = firstNonEmpty(
		compactText(doc.Find(".jp-title").First().Text()),
		metaContent(doc, "property", "og:title"),
		metaContent(doc, "name", "twitter:title"),
		titleFromSlug(parts.Slug),
	)
	if strings.EqualFold(page.Title, "soundgasm.net") {
		page.Title = titleFromSlug(parts.Slug)
	}

	description := doc.Find(".jp-description").First()
	if description.Length() > 0 {
		if rawHTML, err := description.Html(); err == nil {
			page.DescriptionHTML = sanitizeFragment(rawHTML, page.CanonicalURL)
			page.Description = HTMLToText(page.DescriptionHTML)
		}
		if page.Description == "" {
			page.Description = cleanMultilineText(description.Text())
		}
		page.Links = collectLinks(description, page.CanonicalURL)
	}
	if page.Description == "" {
		page.Description = firstNonEmpty(
			metaContent(doc, "name", "description"),
			metaContent(doc, "property", "og:description"),
			metaContent(doc, "name", "twitter:description"),
		)
	}
	page.Tags = extractTags(page.Description)
	page.AudioURL, page.AudioType = parseAudio(doc, htmlText, page.CanonicalURL)
	if page.AudioURL == "" {
		return nil, fmt.Errorf("missing soundgasm audio url")
	}
	if page.DescriptionHTML == "" && page.Description != "" {
		page.DescriptionHTML = TextToHTML(page.Description)
	}
	return page, nil
}

func ContentID(parts URLParts) string {
	if parts.Username == "" && parts.Slug == "" {
		return ""
	}
	return strings.Trim(strings.TrimSpace(parts.Username)+"_"+strings.TrimSpace(parts.Slug), "_")
}

func (p *AudioPage) ContentType() string {
	if p == nil {
		return ""
	}
	return ContentTypeAudio
}

func (p *AudioPage) ContentID() string {
	if p == nil {
		return ""
	}
	return p.ID
}

func BuildHTML(page *AudioPage) string {
	if page == nil {
		return ""
	}
	title := firstNonEmpty(page.Title, page.ID, "Soundgasm Audio")
	sourceURL := firstNonEmpty(page.CanonicalURL, page.URL)
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"zh-CN\">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n<title>")
	b.WriteString(stdhtml.EscapeString(title))
	b.WriteString("</title>\n<style>")
	b.WriteString(`body{margin:0;background:#f7f3ef;color:#241f1b;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;line-height:1.65}main{max-width:820px;margin:0 auto;padding:28px 18px 48px}.panel{background:#fff;border:1px solid #ded6ce;border-radius:6px;padding:22px;margin-bottom:16px}h1{font-size:28px;line-height:1.25;margin:0 0 12px}.meta{color:#6f655c;font-size:14px}.source,.audio-link{word-break:break-all}.description{white-space:normal}.description img{max-width:100%;height:auto}.tags{display:flex;gap:8px;flex-wrap:wrap;margin-top:16px}.tag{background:#f0ece7;border-radius:4px;padding:2px 8px;color:#5a4e43}audio{width:100%;margin:14px 0}`)
	b.WriteString("</style>\n</head>\n<body>\n<main>\n<section class=\"panel\">\n<h1>")
	b.WriteString(stdhtml.EscapeString(title))
	b.WriteString("</h1>\n")
	if page.Author.Name != "" {
		b.WriteString("<p class=\"meta\">")
		if page.Author.URL != "" {
			b.WriteString(`<a href="` + stdhtml.EscapeString(page.Author.URL) + `">` + stdhtml.EscapeString(page.Author.Name) + "</a>")
		} else {
			b.WriteString(stdhtml.EscapeString(page.Author.Name))
		}
		b.WriteString("</p>\n")
	}
	if page.AudioURL != "" {
		b.WriteString(`<audio controls preload="metadata" src="` + stdhtml.EscapeString(page.AudioURL) + `"></audio>` + "\n")
		b.WriteString(`<p class="meta audio-link">Audio: <a href="` + stdhtml.EscapeString(page.AudioURL) + `">` + stdhtml.EscapeString(page.AudioURL) + "</a></p>\n")
	}
	if sourceURL != "" {
		b.WriteString(`<p class="meta source">Source: <a href="` + stdhtml.EscapeString(sourceURL) + `">` + stdhtml.EscapeString(sourceURL) + "</a></p>\n")
	}
	b.WriteString("</section>\n")
	if page.DescriptionHTML != "" || len(page.Tags) > 0 {
		b.WriteString("<section class=\"panel\">\n")
		if page.DescriptionHTML != "" {
			b.WriteString("<div class=\"description\">")
			b.WriteString(page.DescriptionHTML)
			b.WriteString("</div>\n")
		}
		if len(page.Tags) > 0 {
			b.WriteString("<div class=\"tags\">\n")
			for _, tag := range page.Tags {
				b.WriteString(`<span class="tag">` + stdhtml.EscapeString(tag) + "</span>\n")
			}
			b.WriteString("</div>\n")
		}
		b.WriteString("</section>\n")
	}
	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String()
}

func parseAuthor(doc *goquery.Document, parts URLParts) Author {
	author := Author{Name: parts.Username, URL: ProfileURL(parts.Username)}
	doc.Find("a[href]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		username, profileURL, ok := parseProfileURL(normalizeURL(attr(s, "href"), BaseURL+"/"))
		if !ok {
			return true
		}
		author.Name = firstNonEmpty(compactText(s.Text()), username, author.Name)
		author.URL = firstNonEmpty(profileURL, author.URL)
		return false
	})
	return author
}

func parseProfileURL(rawURL string) (string, string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "soundgasm.net" && host != "www.soundgasm.net" {
		return "", "", false
	}
	segments, ok := splitEscapedPath(parsed.EscapedPath())
	if !ok || len(segments) != 2 || strings.ToLower(segments[0]) != "u" || strings.TrimSpace(segments[1]) == "" {
		return "", "", false
	}
	username := strings.TrimSpace(segments[1])
	return username, ProfileURL(username), true
}

func parseAudio(doc *goquery.Document, htmlText string, baseURL string) (string, string) {
	var audioURL string
	var audioType string
	doc.Find("audio[src], source[src]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		audioURL = normalizeURL(attr(s, "src"), baseURL)
		audioType = mediaTypeFromURL(audioURL)
		if audioType == "" {
			audioType = strings.TrimPrefix(strings.TrimSpace(attr(s, "type")), "audio/")
		}
		return audioURL == ""
	})
	if audioURL != "" {
		return audioURL, audioType
	}
	matches := mediaRE.FindStringSubmatch(htmlText)
	if len(matches) == 0 {
		return "", ""
	}
	audioType = strings.ToLower(strings.TrimSpace(matches[1]))
	raw := firstNonEmpty(matches[3], matches[4])
	audioURL = normalizeURL(unquoteJSString(raw), baseURL)
	if audioType == "" {
		audioType = mediaTypeFromURL(audioURL)
	}
	return audioURL, audioType
}

func unquoteJSString(raw string) string {
	raw = strings.ReplaceAll(raw, `\/`, `/`)
	if value, err := strconv.Unquote(`"` + strings.ReplaceAll(raw, `"`, `\"`) + `"`); err == nil {
		return value
	}
	return raw
}

func collectLinks(sel *goquery.Selection, baseURL string) []Link {
	links := []Link{}
	seen := map[string]bool{}
	sel.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		link := Link{
			URL:  sanitizeLink(attr(a, "href"), baseURL),
			Text: compactText(a.Text()),
		}
		if link.URL == "" || seen[link.URL] {
			return
		}
		seen[link.URL] = true
		links = append(links, link)
	})
	return links
}

func extractTags(text string) []string {
	tags := []string{}
	seen := map[string]bool{}
	for _, matches := range tagRE.FindAllStringSubmatch(text, -1) {
		if len(matches) < 2 {
			continue
		}
		tag := compactText(matches[1])
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	return tags
}

func sanitizeFragment(fragment string, baseURL string) string {
	fragment = strings.TrimSpace(fragment)
	if fragment == "" {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`<div id="soundgasm-fragment">` + fragment + `</div>`))
	if err != nil {
		return TextToHTML(fragment)
	}
	root := doc.Find("#soundgasm-fragment")
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
				if src := sanitizeMediaURL(a.Val, baseURL); src != "" {
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

func sanitizeMediaURL(raw string, baseURL string) string {
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

func mediaTypeFromURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	path := strings.ToLower(parsed.Path)
	switch {
	case strings.HasSuffix(path, ".m4a"):
		return "m4a"
	case strings.HasSuffix(path, ".mp3"):
		return "mp3"
	case strings.HasSuffix(path, ".oga"):
		return "oga"
	case strings.HasSuffix(path, ".ogg"):
		return "ogg"
	case strings.HasSuffix(path, ".wav"):
		return "wav"
	default:
		return ""
	}
}

func HTMLToText(fragment string) string {
	fragment = htmlBreakRE.ReplaceAllString(fragment, "\n")
	fragment = htmlBlockEndRE.ReplaceAllString(fragment, "\n")
	fragment = htmlTagRE.ReplaceAllString(fragment, "")
	return cleanMultilineText(stdhtml.UnescapeString(fragment))
}

func TextToHTML(text string) string {
	text = cleanMultilineText(text)
	if text == "" {
		return ""
	}
	paragraphs := strings.Split(text, "\n\n")
	var b strings.Builder
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("<p>")
		b.WriteString(strings.ReplaceAll(stdhtml.EscapeString(paragraph), "\n", "<br>\n"))
		b.WriteString("</p>")
	}
	return b.String()
}

func cleanMultilineText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = compactText(line)
		if line == "" {
			if len(out) > 0 && !blank {
				out = append(out, "")
				blank = true
			}
			continue
		}
		out = append(out, line)
		blank = false
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func compactText(text string) string {
	text = stdhtml.UnescapeString(strings.TrimSpace(text))
	return collapseSpacesRE.ReplaceAllString(text, " ")
}

func titleFromSlug(slug string) string {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return ""
	}
	slug = strings.ReplaceAll(slug, "-", " ")
	slug = strings.ReplaceAll(slug, "_", " ")
	return compactText(slug)
}

func splitEscapedPath(path string) ([]string, bool) {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil, false
	}
	rawSegments := strings.Split(path, "/")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		value, err := url.PathUnescape(segment)
		if err != nil {
			return nil, false
		}
		segments = append(segments, value)
	}
	return segments, true
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
