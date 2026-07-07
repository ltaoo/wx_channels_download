package telegram

import (
	"context"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	BaseURL            = "https://t.me"
	ContentTypeChannel = "channel"
	ContentTypeMessage = "message"
	defaultUserAgent   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
	maxExcerptRunes    = 180
)

var (
	ErrUnsupportedURL = errors.New("unsupported telegram url")

	shareURLRE          = regexp.MustCompile(`https?://[^\s"'<>]+`)
	usernameRE          = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{3,31}$`)
	messageIDRE         = regexp.MustCompile(`^[0-9]+$`)
	backgroundImageURL  = regexp.MustCompile(`(?i)background-image\s*:\s*url\((?:"([^"]+)"|'([^']+)'|([^)]*))\)`)
	htmlBreakRE         = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlBlockEndRE      = regexp.MustCompile(`(?i)</(?:p|div|section|article|li|h[1-6])\s*>`)
	htmlTagRE           = regexp.MustCompile(`<[^>]+>`)
	collapseWhitespace  = regexp.MustCompile(`[ \t\r\f\v]+`)
	privateTelegramPath = map[string]bool{
		"addemoji":    true,
		"addstickers": true,
		"boost":       true,
		"c":           true,
		"contact":     true,
		"dl":          true,
		"iv":          true,
		"joinchat":    true,
		"login":       true,
		"m":           true,
		"proxy":       true,
		"share":       true,
		"+":           true,
	}
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	HTTPClient HTTPClient
	UserAgent  string
	Cookie     string
}

type TelegramURL struct {
	Username  string `json:"username,omitempty"`
	MessageID int    `json:"message_id,omitempty"`
	Canonical string `json:"canonical"`
	WebURL    string `json:"web_url"`
}

type Channel struct {
	Username     string            `json:"username,omitempty"`
	Title        string            `json:"title,omitempty"`
	Description  string            `json:"description,omitempty"`
	URL          string            `json:"url,omitempty"`
	WebURL       string            `json:"web_url,omitempty"`
	AvatarURL    string            `json:"avatar_url,omitempty"`
	Verified     bool              `json:"verified,omitempty"`
	Counters     map[string]int64  `json:"counters,omitempty"`
	CounterTexts map[string]string `json:"counter_texts,omitempty"`
}

type Message struct {
	ChannelUsername string       `json:"channel_username,omitempty"`
	ID              int          `json:"id,omitempty"`
	Post            string       `json:"post,omitempty"`
	URL             string       `json:"url,omitempty"`
	WebURL          string       `json:"web_url,omitempty"`
	AuthorName      string       `json:"author_name,omitempty"`
	AuthorURL       string       `json:"author_url,omitempty"`
	AuthorAvatarURL string       `json:"author_avatar_url,omitempty"`
	ContentHTML     string       `json:"content_html,omitempty"`
	ContentText     string       `json:"content_text,omitempty"`
	MediaType       string       `json:"media_type,omitempty"`
	Media           []Media      `json:"media,omitempty"`
	Links           []Link       `json:"links,omitempty"`
	LinkPreview     *LinkPreview `json:"link_preview,omitempty"`
	ViewCount       int64        `json:"view_count,omitempty"`
	PublishedAt     string       `json:"published_at,omitempty"`
	Edited          bool         `json:"edited,omitempty"`
}

type Media struct {
	Type         string `json:"type,omitempty"`
	URL          string `json:"url,omitempty"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	Title        string `json:"title,omitempty"`
	Duration     string `json:"duration,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
}

type Link struct {
	URL  string `json:"url,omitempty"`
	Text string `json:"text,omitempty"`
}

type LinkPreview struct {
	URL         string `json:"url,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
}

type Page struct {
	URL          TelegramURL `json:"url"`
	SourceURL    string      `json:"source_url,omitempty"`
	CanonicalURL string      `json:"canonical_url,omitempty"`
	Channel      Channel     `json:"channel,omitempty"`
	Messages     []Message   `json:"messages,omitempty"`
	PageHTML     string      `json:"-"`
}

func NewClient(client HTTPClient) *Client {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{HTTPClient: client, UserAgent: defaultUserAgent}
}

func CanParse(rawURL string) bool {
	_, ok := ParseURL(ExtractShareURL(rawURL))
	return ok
}

func ExtractShareURL(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if strings.HasPrefix(text, "@") {
		return text
	}
	for _, match := range shareURLRE.FindAllString(text, -1) {
		match = strings.Trim(match, " \t\r\n，。；;、.,!?！？")
		if _, ok := ParseURL(match); ok {
			return match
		}
	}
	if _, ok := ParseURL(text); ok {
		return text
	}
	return ""
}

func ParseURL(rawURL string) (TelegramURL, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return TelegramURL{}, false
	}
	if strings.HasPrefix(rawURL, "@") {
		username := strings.TrimPrefix(rawURL, "@")
		return buildTelegramURL(username, 0, ""), isValidUsername(username)
	}
	if !strings.Contains(rawURL, "://") && isValidUsername(strings.TrimPrefix(rawURL, "@")) {
		username := strings.TrimPrefix(rawURL, "@")
		return buildTelegramURL(username, 0, ""), true
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return TelegramURL{}, false
	}
	if parsed.Scheme == "tg" {
		return parseTGResolveURL(parsed)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return TelegramURL{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "t.me" && host != "www.t.me" && host != "telegram.me" && host != "www.telegram.me" && host != "telegram.dog" && host != "www.telegram.dog" {
		return TelegramURL{}, false
	}

	parts := splitEscapedPath(parsed.EscapedPath())
	if len(parts) == 0 {
		return TelegramURL{}, false
	}
	offset := 0
	if strings.EqualFold(parts[0], "s") {
		offset = 1
	}
	if len(parts) <= offset {
		return TelegramURL{}, false
	}
	username := parts[offset]
	if privateTelegramPath[strings.ToLower(username)] || strings.HasPrefix(username, "+") || !isValidUsername(username) {
		return TelegramURL{}, false
	}

	messageID := 0
	if len(parts) > offset+1 {
		if !messageIDRE.MatchString(parts[offset+1]) {
			return TelegramURL{}, false
		}
		id, err := strconv.Atoi(parts[offset+1])
		if err != nil || id <= 0 {
			return TelegramURL{}, false
		}
		messageID = id
	}
	return buildTelegramURL(username, messageID, parsed.RawQuery), true
}

func (c *Client) FetchPage(ctx context.Context, rawURL string) (*Page, error) {
	shareURL := ExtractShareURL(rawURL)
	if shareURL == "" {
		return nil, ErrUnsupportedURL
	}
	parts, ok := ParseURL(shareURL)
	if !ok {
		return nil, ErrUnsupportedURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parts.WebURL, nil)
	if err != nil {
		return nil, err
	}
	setHeaders(req, c.userAgent(), c.Cookie)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch telegram page: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("fetch telegram page: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	page, err := ParsePageHTML(shareURL, string(body))
	if err != nil {
		return nil, err
	}
	if resp.Request != nil && resp.Request.URL != nil {
		page.SourceURL = resp.Request.URL.String()
	}
	return page, nil
}

func FetchPage(ctx context.Context, rawURL string) (*Page, error) {
	return NewClient(nil).FetchPage(ctx, rawURL)
}

func ParsePage(rawURL string, r io.Reader) (*Page, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return ParsePageHTML(rawURL, string(body))
}

func ParsePageHTML(rawURL string, htmlText string) (*Page, error) {
	parts, ok := ParseURL(rawURL)
	if !ok {
		return nil, ErrUnsupportedURL
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	page := &Page{
		URL:          parts,
		SourceURL:    rawURL,
		CanonicalURL: parts.Canonical,
		PageHTML:     htmlText,
	}
	page.Channel = parseChannel(doc, parts, parts.WebURL)
	page.Messages = parseMessages(doc, parts.WebURL, page.Channel.Username)
	if parts.MessageID > 0 {
		filtered := page.Messages[:0]
		for _, message := range page.Messages {
			if message.ID == parts.MessageID {
				filtered = append(filtered, message)
			}
		}
		page.Messages = filtered
	}
	if page.Channel.Username == "" {
		page.Channel.Username = parts.Username
	}
	if page.Channel.URL == "" && page.Channel.Username != "" {
		page.Channel.URL = BaseURL + "/" + page.Channel.Username
	}
	if page.Channel.WebURL == "" && page.Channel.Username != "" {
		page.Channel.WebURL = BaseURL + "/s/" + page.Channel.Username
	}
	return page, nil
}

func (p *Page) ContentType() string {
	if p == nil {
		return ""
	}
	if p.URL.MessageID > 0 || len(p.Messages) == 1 {
		return ContentTypeMessage
	}
	return ContentTypeChannel
}

func (p *Page) ContentID() string {
	if p == nil {
		return ""
	}
	username := p.Channel.Username
	if username == "" {
		username = p.URL.Username
	}
	if p.URL.MessageID > 0 {
		return fmt.Sprintf("%s_%d", username, p.URL.MessageID)
	}
	if len(p.Messages) == 1 && p.Messages[0].ID > 0 {
		return fmt.Sprintf("%s_%d", username, p.Messages[0].ID)
	}
	return username
}

func PageTitle(page *Page) string {
	if page == nil {
		return ""
	}
	channelTitle := firstNonEmpty(page.Channel.Title, page.Channel.Username)
	if page.ContentType() == ContentTypeMessage && len(page.Messages) > 0 {
		message := page.Messages[0]
		excerpt := excerpt(message.ContentText, 72)
		if excerpt != "" {
			return firstNonEmpty(channelTitle+" #"+strconv.Itoa(message.ID)+": "+excerpt, excerpt)
		}
		if channelTitle != "" && message.ID > 0 {
			return channelTitle + " #" + strconv.Itoa(message.ID)
		}
	}
	return firstNonEmpty(channelTitle, page.URL.Username, "telegram")
}

func PageDescription(page *Page) string {
	if page == nil {
		return ""
	}
	if page.ContentType() == ContentTypeMessage && len(page.Messages) > 0 {
		return excerpt(page.Messages[0].ContentText, maxExcerptRunes)
	}
	return firstNonEmpty(page.Channel.Description, excerpt(messagesText(page.Messages), maxExcerptRunes))
}

func FirstMediaURL(page *Page) string {
	if page == nil {
		return ""
	}
	for _, message := range page.Messages {
		for _, media := range message.Media {
			if media.ThumbnailURL != "" {
				return media.ThumbnailURL
			}
			if media.URL != "" && media.Type == "photo" {
				return media.URL
			}
		}
		if message.LinkPreview != nil && message.LinkPreview.ImageURL != "" {
			return message.LinkPreview.ImageURL
		}
	}
	return page.Channel.AvatarURL
}

func BuildHTML(page *Page) string {
	if page == nil {
		return ""
	}
	title := firstNonEmpty(PageTitle(page), page.ContentID(), "Telegram")
	sourceURL := firstNonEmpty(page.CanonicalURL, page.SourceURL, page.URL.Canonical)
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"zh-CN\">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n<title>")
	b.WriteString(stdhtml.EscapeString(title))
	b.WriteString("</title>\n<style>")
	b.WriteString(`body{margin:0;background:#eef2f5;color:#17212b;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;line-height:1.62}main{max-width:860px;margin:0 auto;padding:28px 18px 48px}.panel{background:#fff;border:1px solid #dbe3ea;border-radius:6px;padding:20px;margin-bottom:16px}.channel{display:flex;gap:14px;align-items:flex-start}.avatar{width:56px;height:56px;border-radius:50%;object-fit:cover}.meta{color:#64748b;font-size:14px}.source{word-break:break-all}.message{border-top:1px solid #e6edf3;padding-top:16px;margin-top:16px}.message:first-child{border-top:0;padding-top:0;margin-top:0}.message-head{display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap}.message-body img,.media img,.preview img{max-width:100%;height:auto;border-radius:6px}.media video{max-width:100%;border-radius:6px;background:#000}.media{display:grid;gap:10px;margin-top:12px}.preview{border-left:3px solid #4aa3df;padding-left:12px;margin-top:12px}.preview-title{font-weight:600}.counters{display:flex;gap:12px;flex-wrap:wrap}`)
	b.WriteString("</style>\n</head>\n<body>\n<main>\n")
	writeChannelHTML(&b, page, sourceURL)
	b.WriteString("<section class=\"panel\">\n")
	if len(page.Messages) == 0 {
		b.WriteString("<p class=\"meta\">No public messages were found on this Telegram page.</p>\n")
	}
	for _, message := range page.Messages {
		writeMessageHTML(&b, message)
	}
	b.WriteString("</section>\n</main>\n</body>\n</html>\n")
	return b.String()
}

func parseChannel(doc *goquery.Document, parts TelegramURL, pageURL string) Channel {
	channel := Channel{
		Username: parts.Username,
		URL:      BaseURL + "/" + parts.Username,
		WebURL:   BaseURL + "/s/" + parts.Username,
	}
	channel.Title = cleanTitle(firstNonEmpty(
		compactText(doc.Find(".tgme_channel_info_header_title span").First().Text()),
		compactText(doc.Find(".tgme_header_title span").First().Text()),
		metaContent(doc, "property", "og:title"),
		strings.TrimSpace(doc.Find("title").First().Text()),
		parts.Username,
	))
	channel.Description = firstNonEmpty(
		compactText(doc.Find(".tgme_channel_info_description").First().Text()),
		metaContent(doc, "property", "og:description"),
		metaContent(doc, "name", "twitter:description"),
	)
	channel.AvatarURL = normalizeURL(firstNonEmpty(
		attr(doc.Find(".tgme_channel_info_header img").First(), "src"),
		attr(doc.Find(".tgme_header_info img").First(), "src"),
		metaContent(doc, "property", "og:image"),
		metaContent(doc, "property", "twitter:image"),
	), pageURL)
	if username := usernameFromSelection(doc.Find(".tgme_channel_info_header_username a").First()); username != "" {
		channel.Username = username
		channel.URL = BaseURL + "/" + username
		channel.WebURL = BaseURL + "/s/" + username
	}
	channel.Verified = doc.Find(".tgme_channel_info_header_labels .verified-icon, .tgme_header_labels .verified-icon").Length() > 0
	channel.Counters = map[string]int64{}
	channel.CounterTexts = map[string]string{}
	doc.Find(".tgme_channel_info_counter").Each(func(_ int, s *goquery.Selection) {
		key := normalizeCounterKey(s.Find(".counter_type").First().Text())
		valueText := compactText(s.Find(".counter_value").First().Text())
		if key == "" || valueText == "" {
			return
		}
		channel.Counters[key] = parseAbbrevNumber(valueText)
		channel.CounterTexts[key] = valueText
	})
	if len(channel.Counters) == 0 {
		channel.Counters = nil
	}
	if len(channel.CounterTexts) == 0 {
		channel.CounterTexts = nil
	}
	return channel
}

func parseMessages(doc *goquery.Document, pageURL string, fallbackUsername string) []Message {
	var messages []Message
	doc.Find(".tgme_widget_message.js-widget_message, .tgme_widget_message[data-post]").Each(func(_ int, s *goquery.Selection) {
		message := parseMessage(s, pageURL, fallbackUsername)
		if message.ID > 0 || message.ContentHTML != "" || len(message.Media) > 0 {
			messages = append(messages, message)
		}
	})
	sort.SliceStable(messages, func(i, j int) bool {
		return messages[i].ID < messages[j].ID
	})
	return messages
}

func parseMessage(s *goquery.Selection, pageURL string, fallbackUsername string) Message {
	post := strings.TrimSpace(attr(s, "data-post"))
	username, messageID := parsePost(post)
	if username == "" {
		username = fallbackUsername
	}
	message := Message{
		ChannelUsername: username,
		ID:              messageID,
		Post:            post,
	}
	if username != "" && messageID > 0 {
		message.URL = BuildMessageURL(username, messageID)
		message.WebURL = BuildMessageWebURL(username, messageID)
	}
	message.AuthorName = compactText(s.Find(".tgme_widget_message_owner_name, .tgme_widget_message_author_name").First().Text())
	message.AuthorURL = normalizeURL(attr(s.Find(".tgme_widget_message_owner_name, .tgme_widget_message_author_name").First(), "href"), pageURL)
	message.AuthorAvatarURL = normalizeURL(attr(s.Find(".tgme_widget_message_user_photo img").First(), "src"), pageURL)

	textSel := s.Find(".tgme_widget_message_text.js-message_text, .tgme_widget_message_text").First()
	if textSel.Length() > 0 {
		message.ContentHTML, message.ContentText = cleanContentSelection(textSel, pageURL)
		message.Links = collectLinks(textSel, pageURL)
	}
	message.Media = collectMedia(s, pageURL)
	message.LinkPreview = parseLinkPreview(s.Find("a.tgme_widget_message_link_preview").First(), pageURL)
	if message.LinkPreview != nil && message.LinkPreview.URL != "" {
		message.Links = appendUniqueLink(message.Links, Link{URL: message.LinkPreview.URL, Text: firstNonEmpty(message.LinkPreview.Title, message.LinkPreview.URL)})
	}
	message.MediaType = inferMediaType(message.Media, message.LinkPreview, message.ContentText)
	message.ViewCount = parseAbbrevNumber(s.Find(".tgme_widget_message_views").First().Text())
	message.PublishedAt = strings.TrimSpace(attr(s.Find("a.tgme_widget_message_date time[datetime], time.time[datetime]").First(), "datetime"))
	message.Edited = strings.Contains(strings.ToLower(compactText(s.Find(".tgme_widget_message_meta").Text())), "edited")
	if message.ID == 0 {
		message.ID = messageIDFromDateHref(s, pageURL)
		if message.ID > 0 && username != "" {
			message.URL = BuildMessageURL(username, message.ID)
			message.WebURL = BuildMessageWebURL(username, message.ID)
		}
	}
	if message.ContentText == "" && len(message.Media) > 0 {
		message.ContentText = "Telegram media message"
	}
	if message.ContentHTML == "" && message.ContentText != "" {
		message.ContentHTML = TextToHTML(message.ContentText)
	}
	return message
}

func collectMedia(s *goquery.Selection, pageURL string) []Media {
	var media []Media
	seen := map[string]bool{}
	add := func(item Media) {
		key := firstNonEmpty(item.URL, item.ThumbnailURL)
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		media = append(media, item)
	}

	s.Find(".tgme_widget_message_photo_wrap").Each(func(_ int, photo *goquery.Selection) {
		imageURL := normalizeURL(extractBackgroundURL(attr(photo, "style")), pageURL)
		add(Media{
			Type:         "photo",
			URL:          imageURL,
			ThumbnailURL: imageURL,
			Width:        parseStylePixels(attr(photo, "style"), "width"),
			Height:       parseStylePixels(attr(photo, "style"), "height"),
		})
	})
	s.Find(".tgme_widget_message_video_player, .tgme_widget_message_roundvideo_player").Each(func(_ int, player *goquery.Selection) {
		videoURL := normalizeURL(attr(player.Find("video[src]").First(), "src"), pageURL)
		thumbnailURL := normalizeURL(extractBackgroundURL(attr(player.Find(".tgme_widget_message_video_thumb, .tgme_widget_message_roundvideo_thumb").First(), "style")), pageURL)
		add(Media{
			Type:         "video",
			URL:          videoURL,
			ThumbnailURL: thumbnailURL,
			Duration:     compactText(player.Find(".message_video_duration, .message_roundvideo_duration").First().Text()),
			Width:        parseStylePixels(attr(player.Find(".tgme_widget_message_video_wrap").First(), "style"), "width"),
		})
	})
	s.Find("a.tgme_widget_message_document, .tgme_widget_message_document_wrap a[href]").Each(func(_ int, doc *goquery.Selection) {
		docURL := normalizeURL(attr(doc, "href"), pageURL)
		add(Media{
			Type:  "document",
			URL:   docURL,
			Title: compactText(doc.Find(".tgme_widget_message_document_title").First().Text()),
		})
	})
	return media
}

func parseLinkPreview(s *goquery.Selection, pageURL string) *LinkPreview {
	if s == nil || s.Length() == 0 {
		return nil
	}
	preview := &LinkPreview{
		URL:         normalizeURL(attr(s, "href"), pageURL),
		SiteName:    compactText(s.Find(".link_preview_site_name").First().Text()),
		Title:       compactText(s.Find(".link_preview_title").First().Text()),
		Description: compactText(s.Find(".link_preview_description").First().Text()),
		ImageURL:    normalizeURL(extractBackgroundURL(attr(s.Find(".link_preview_image").First(), "style")), pageURL),
	}
	if preview.URL == "" && preview.Title == "" && preview.Description == "" && preview.ImageURL == "" {
		return nil
	}
	return preview
}

func cleanContentSelection(sel *goquery.Selection, pageURL string) (string, string) {
	sel.Find("script,style").Remove()
	sel.Find("[onclick]").RemoveAttr("onclick")
	sel.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		if href := normalizeURL(attr(a, "href"), pageURL); href != "" {
			a.SetAttr("href", href)
		}
		a.SetAttr("rel", "noopener")
	})
	sel.Find("img[src]").Each(func(_ int, img *goquery.Selection) {
		if src := normalizeURL(attr(img, "src"), pageURL); src != "" {
			img.SetAttr("src", src)
		}
	})
	bodyHTML, _ := sel.Html()
	bodyHTML = strings.TrimSpace(bodyHTML)
	return bodyHTML, HTMLToText(bodyHTML)
}

func collectLinks(sel *goquery.Selection, pageURL string) []Link {
	links := []Link{}
	seen := map[string]bool{}
	sel.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		link := Link{
			URL:  normalizeURL(attr(a, "href"), pageURL),
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

func appendUniqueLink(links []Link, link Link) []Link {
	if link.URL == "" {
		return links
	}
	for _, existing := range links {
		if existing.URL == link.URL {
			return links
		}
	}
	return append(links, link)
}

func inferMediaType(media []Media, preview *LinkPreview, text string) string {
	for _, item := range media {
		if item.Type == "video" {
			return "Video"
		}
	}
	for _, item := range media {
		if item.Type == "photo" {
			return "Photo"
		}
	}
	for _, item := range media {
		if item.Type == "document" {
			return "Document"
		}
	}
	if preview != nil {
		return "Link"
	}
	if strings.TrimSpace(text) != "" {
		return "Text"
	}
	return ""
}

func writeChannelHTML(b *strings.Builder, page *Page, sourceURL string) {
	channel := page.Channel
	b.WriteString("<section class=\"panel channel\">\n")
	if channel.AvatarURL != "" {
		b.WriteString(`<img class="avatar" src="` + stdhtml.EscapeString(channel.AvatarURL) + `" alt="">` + "\n")
	}
	b.WriteString("<div>\n<h1>")
	b.WriteString(stdhtml.EscapeString(firstNonEmpty(channel.Title, channel.Username, "Telegram")))
	b.WriteString("</h1>\n")
	if channel.Username != "" {
		b.WriteString(`<p class="meta"><a href="` + stdhtml.EscapeString(channel.URL) + `">@` + stdhtml.EscapeString(channel.Username) + "</a></p>\n")
	}
	if channel.Description != "" {
		b.WriteString("<p>")
		b.WriteString(stdhtml.EscapeString(channel.Description))
		b.WriteString("</p>\n")
	}
	if len(channel.CounterTexts) > 0 {
		keys := make([]string, 0, len(channel.CounterTexts))
		for key := range channel.CounterTexts {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		b.WriteString("<p class=\"meta counters\">")
		for _, key := range keys {
			b.WriteString("<span>")
			b.WriteString(stdhtml.EscapeString(channel.CounterTexts[key] + " " + key))
			b.WriteString("</span>")
		}
		b.WriteString("</p>\n")
	}
	if sourceURL != "" {
		b.WriteString(`<p class="meta source">Source: <a href="` + stdhtml.EscapeString(sourceURL) + `">` + stdhtml.EscapeString(sourceURL) + "</a></p>\n")
	}
	b.WriteString("</div>\n</section>\n")
}

func writeMessageHTML(b *strings.Builder, message Message) {
	b.WriteString(`<article class="message" id="message-`)
	b.WriteString(strconv.Itoa(message.ID))
	b.WriteString("\">\n<header class=\"message-head\"><div>")
	if message.URL != "" {
		b.WriteString(`<a href="` + stdhtml.EscapeString(message.URL) + `">#` + strconv.Itoa(message.ID) + "</a>")
	} else if message.ID > 0 {
		b.WriteString("#" + strconv.Itoa(message.ID))
	}
	if message.PublishedAt != "" {
		b.WriteString(` <time datetime="` + stdhtml.EscapeString(message.PublishedAt) + `">` + stdhtml.EscapeString(message.PublishedAt) + "</time>")
	}
	b.WriteString("</div><div class=\"meta\">")
	if message.ViewCount > 0 {
		b.WriteString(strconv.FormatInt(message.ViewCount, 10) + " views")
	}
	b.WriteString("</div></header>\n")
	if message.ContentHTML != "" {
		b.WriteString("<div class=\"message-body\">")
		b.WriteString(message.ContentHTML)
		b.WriteString("</div>\n")
	}
	if len(message.Media) > 0 {
		b.WriteString("<div class=\"media\">\n")
		for _, media := range message.Media {
			switch media.Type {
			case "video":
				if media.URL != "" {
					b.WriteString(`<video controls src="` + stdhtml.EscapeString(media.URL) + `"`)
					if media.ThumbnailURL != "" {
						b.WriteString(` poster="` + stdhtml.EscapeString(media.ThumbnailURL) + `"`)
					}
					b.WriteString("></video>\n")
				} else if media.ThumbnailURL != "" {
					b.WriteString(`<img src="` + stdhtml.EscapeString(media.ThumbnailURL) + `" alt="">` + "\n")
				}
			case "photo":
				if media.URL != "" {
					b.WriteString(`<img src="` + stdhtml.EscapeString(media.URL) + `" alt="">` + "\n")
				}
			default:
				if media.URL != "" {
					b.WriteString(`<p><a href="` + stdhtml.EscapeString(media.URL) + `">` + stdhtml.EscapeString(firstNonEmpty(media.Title, media.Type, media.URL)) + "</a></p>\n")
				}
			}
		}
		b.WriteString("</div>\n")
	}
	if message.LinkPreview != nil {
		writeLinkPreviewHTML(b, message.LinkPreview)
	}
	b.WriteString("</article>\n")
}

func writeLinkPreviewHTML(b *strings.Builder, preview *LinkPreview) {
	b.WriteString("<div class=\"preview\">\n")
	if preview.SiteName != "" {
		b.WriteString(`<div class="meta">` + stdhtml.EscapeString(preview.SiteName) + "</div>\n")
	}
	if preview.ImageURL != "" {
		b.WriteString(`<img src="` + stdhtml.EscapeString(preview.ImageURL) + `" alt="">` + "\n")
	}
	if preview.Title != "" {
		b.WriteString(`<div class="preview-title">`)
		if preview.URL != "" {
			b.WriteString(`<a href="` + stdhtml.EscapeString(preview.URL) + `">`)
		}
		b.WriteString(stdhtml.EscapeString(preview.Title))
		if preview.URL != "" {
			b.WriteString("</a>")
		}
		b.WriteString("</div>\n")
	}
	if preview.Description != "" {
		b.WriteString("<p>" + stdhtml.EscapeString(preview.Description) + "</p>\n")
	}
	b.WriteString("</div>\n")
}

func BuildMessageURL(username string, messageID int) string {
	username = strings.TrimSpace(strings.TrimPrefix(username, "@"))
	if username == "" || messageID <= 0 {
		return ""
	}
	return BaseURL + "/" + username + "/" + strconv.Itoa(messageID)
}

func BuildMessageWebURL(username string, messageID int) string {
	username = strings.TrimSpace(strings.TrimPrefix(username, "@"))
	if username == "" || messageID <= 0 {
		return ""
	}
	return BaseURL + "/s/" + username + "/" + strconv.Itoa(messageID)
}

func TextToHTML(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var b strings.Builder
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		b.WriteString("<p>")
		b.WriteString(stdhtml.EscapeString(line))
		b.WriteString("</p>\n")
	}
	return b.String()
}

func HTMLToText(fragment string) string {
	if strings.TrimSpace(fragment) == "" {
		return ""
	}
	text := htmlBreakRE.ReplaceAllString(fragment, "\n")
	text = htmlBlockEndRE.ReplaceAllString(text, "\n")
	text = htmlTagRE.ReplaceAllString(text, "")
	text = stdhtml.UnescapeString(text)
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = compactText(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func parseTGResolveURL(parsed *url.URL) (TelegramURL, bool) {
	if parsed == nil || parsed.Host != "resolve" {
		return TelegramURL{}, false
	}
	username := parsed.Query().Get("domain")
	if !isValidUsername(username) {
		return TelegramURL{}, false
	}
	messageID := 0
	if rawPost := parsed.Query().Get("post"); rawPost != "" {
		id, err := strconv.Atoi(rawPost)
		if err != nil || id <= 0 {
			return TelegramURL{}, false
		}
		messageID = id
	}
	return buildTelegramURL(username, messageID, ""), true
}

func buildTelegramURL(username string, messageID int, rawQuery string) TelegramURL {
	username = strings.TrimSpace(strings.TrimPrefix(username, "@"))
	canonical := BaseURL + "/" + username
	webURL := BaseURL + "/s/" + username
	if messageID > 0 {
		id := strconv.Itoa(messageID)
		canonical += "/" + id
		webURL += "/" + id
	}
	if rawQuery != "" {
		webURL += "?" + rawQuery
	}
	return TelegramURL{Username: username, MessageID: messageID, Canonical: canonical, WebURL: webURL}
}

func splitEscapedPath(pathValue string) []string {
	pathValue = strings.Trim(pathValue, "/")
	if pathValue == "" {
		return nil
	}
	rawParts := strings.Split(pathValue, "/")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		value, err := url.PathUnescape(part)
		if err != nil {
			value = part
		}
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, value)
		}
	}
	return parts
}

func isValidUsername(username string) bool {
	username = strings.TrimSpace(strings.TrimPrefix(username, "@"))
	return usernameRE.MatchString(username)
}

func setHeaders(req *http.Request, ua, cookie string) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Referer", BaseURL+"/")
	if strings.TrimSpace(cookie) != "" {
		req.Header.Set("Cookie", cookie)
	}
}

func (c *Client) httpClient() HTTPClient {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c *Client) userAgent() string {
	if c != nil && strings.TrimSpace(c.UserAgent) != "" {
		return c.UserAgent
	}
	return defaultUserAgent
}

func attr(s *goquery.Selection, name string) string {
	value, _ := s.Attr(name)
	return strings.TrimSpace(value)
}

func metaContent(doc *goquery.Document, attrName string, attrValue string) string {
	selector := fmt.Sprintf(`meta[%s="%s"]`, attrName, attrValue)
	return strings.TrimSpace(attr(doc.Find(selector).First(), "content"))
}

func usernameFromSelection(s *goquery.Selection) string {
	text := strings.TrimSpace(strings.TrimPrefix(s.Text(), "@"))
	if isValidUsername(text) {
		return text
	}
	href := attr(s, "href")
	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}
	parts := splitEscapedPath(parsed.EscapedPath())
	if len(parts) > 0 && isValidUsername(parts[0]) {
		return parts[0]
	}
	return ""
}

func parsePost(post string) (string, int) {
	parts := strings.Split(strings.TrimSpace(post), "/")
	if len(parts) != 2 || !isValidUsername(parts[0]) || !messageIDRE.MatchString(parts[1]) {
		return "", 0
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0
	}
	return parts[0], id
}

func messageIDFromDateHref(s *goquery.Selection, pageURL string) int {
	href := normalizeURL(attr(s.Find("a.tgme_widget_message_date").First(), "href"), pageURL)
	parts, ok := ParseURL(href)
	if !ok {
		return 0
	}
	return parts.MessageID
}

func normalizeURL(href, pageURL string) string {
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
	base, err := url.Parse(pageURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		base, _ = url.Parse(BaseURL + "/")
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(parsed).String()
}

func extractBackgroundURL(style string) string {
	matches := backgroundImageURL.FindStringSubmatch(style)
	if len(matches) == 0 {
		return ""
	}
	for _, match := range matches[1:] {
		match = strings.TrimSpace(match)
		if match != "" {
			return stdhtml.UnescapeString(match)
		}
	}
	return ""
}

func parseStylePixels(style, key string) int {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, item := range strings.Split(style, ";") {
		parts := strings.SplitN(item, ":", 2)
		if len(parts) != 2 || strings.ToLower(strings.TrimSpace(parts[0])) != key {
			continue
		}
		value := strings.TrimSpace(strings.TrimSuffix(parts[1], "px"))
		n, _ := strconv.Atoi(value)
		return n
	}
	return 0
}

func parseAbbrevNumber(value string) int64 {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\u00a0", ""))
	value = strings.ReplaceAll(value, ",", "")
	value = strings.ToLower(value)
	if value == "" {
		return 0
	}
	multiplier := float64(1)
	switch {
	case strings.HasSuffix(value, "k"):
		multiplier = 1000
		value = strings.TrimSuffix(value, "k")
	case strings.HasSuffix(value, "m"):
		multiplier = 1000000
		value = strings.TrimSuffix(value, "m")
	case strings.HasSuffix(value, "b"):
		multiplier = 1000000000
		value = strings.TrimSuffix(value, "b")
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return int64(math.Round(n * multiplier))
}

func normalizeCounterKey(value string) string {
	value = strings.ToLower(compactText(value))
	value = strings.TrimSuffix(value, ":")
	return strings.ReplaceAll(value, " ", "_")
}

func cleanTitle(title string) string {
	title = compactText(title)
	title = strings.TrimSuffix(title, " - Telegram")
	title = strings.TrimSuffix(title, " – Telegram")
	return strings.TrimSpace(title)
}

func compactText(text string) string {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = collapseWhitespace.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func excerpt(text string, limit int) string {
	text = compactText(strings.ReplaceAll(text, "\n", " "))
	if text == "" || limit <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "..."
}

func messagesText(messages []Message) string {
	var parts []string
	for _, message := range messages {
		if strings.TrimSpace(message.ContentText) != "" {
			parts = append(parts, message.ContentText)
		}
	}
	return strings.Join(parts, "\n")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
