package webpage

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"

	"wx_channel/pkg/clawreq"
	"wx_channel/pkg/cookies"
)

const default_request_timeout = 30 * time.Second

// Client fetches generic web pages with a browser fingerprint and optionally
// attaches cookies loaded from persistent storage for the requested domain.
type Client struct {
	claw_client     *clawreq.Client
	claw_client_err error
	cookie_provider *cookies.Reader
}

// NewClient creates a generic web page client.
func NewClient(cookie_provider *cookies.Reader) *Client {
	claw_client, claw_client_err := clawreq.New(clawreq.Config{
		Profile:         clawreq.ProfileChrome,
		Timeout:         default_request_timeout,
		FollowRedirects: true,
	})
	return &Client{
		claw_client:     claw_client,
		claw_client_err: claw_client_err,
		cookie_provider: cookie_provider,
	}
}

// Close releases idle HTTP resources.
func (c *Client) Close() {
	if c == nil || c.claw_client == nil {
		return
	}
	c.claw_client.CloseIdleConnections()
}

// Fetch retrieves and extracts a web page.
func (c *Client) Fetch(raw_url string) (*Page, error) {
	return c.FetchContext(context.Background(), raw_url)
}

// FetchContext retrieves and extracts a web page with cancellation support.
func (c *Client) FetchContext(fetch_context context.Context, raw_url string) (*Page, error) {
	if c == nil {
		return nil, errors.New("webpage client is not initialized")
	}
	if c.claw_client == nil {
		if c.claw_client_err != nil {
			return nil, fmt.Errorf("initialize webpage clawreq client: %w", c.claw_client_err)
		}
		return nil, errors.New("webpage clawreq client is not initialized")
	}
	request_url, err := parse_request_url(raw_url)
	if err != nil {
		return nil, err
	}
	if fetch_context == nil {
		fetch_context = context.Background()
	}

	request_options := []clawreq.RequestOption{
		clawreq.WithHeaders(map[string]string{
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
			"Cache-Control":   "max-age=0",
			"Sec-Fetch-Site":  "none",
		}),
	}
	cookie_header, err := c.cookie_for_domain(request_url.Hostname())
	if err != nil {
		return nil, err
	}
	if cookie_header != "" {
		request_options = append(request_options, clawreq.WithCookie(cookie_header))
	}

	response, err := c.claw_client.Get(fetch_context, request_url.String(), request_options...)
	if err != nil {
		return nil, fmt.Errorf("fetch webpage with clawreq: url=%q: %w", request_url.String(), err)
	}
	if response == nil {
		return nil, errors.New("fetch webpage with clawreq: empty response")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("webpage returned HTTP %d", response.StatusCode)
	}
	if err := validate_content_type(response.ContentType()); err != nil {
		return nil, err
	}
	html_text, err := response.Text()
	if err != nil {
		return nil, fmt.Errorf("decode webpage response: %w", err)
	}
	final_url := strings.TrimSpace(response.FinalURL)
	if final_url == "" {
		final_url = request_url.String()
	}
	page, err := parse_page(html_text, request_url.String(), final_url)
	if err != nil {
		return nil, err
	}
	page.StatusCode = response.StatusCode
	page.ContentType = strings.TrimSpace(response.ContentType())
	return page, nil
}

func parse_request_url(raw_url string) (*url.URL, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return nil, errors.New("网页 URL 不能为空")
	}
	request_url, err := url.Parse(raw_url)
	if err != nil || request_url.Hostname() == "" {
		return nil, fmt.Errorf("无法解析网页 URL: %s", raw_url)
	}
	request_url.Scheme = strings.ToLower(request_url.Scheme)
	if request_url.Scheme != "http" && request_url.Scheme != "https" {
		return nil, fmt.Errorf("网页 URL 仅支持 HTTP/HTTPS: %s", raw_url)
	}
	return request_url, nil
}

func (c *Client) cookie_for_domain(domain string) (string, error) {
	if c == nil || c.cookie_provider == nil {
		return "", nil
	}
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return "", nil
	}
	cookie_header, err := c.cookie_provider.HeaderForDomain(domain)
	if errors.Is(err, cookies.ErrCookieNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s cookies: %w", domain, err)
	}
	return strings.TrimSpace(cookie_header), nil
}

func validate_content_type(content_type string) error {
	content_type = strings.TrimSpace(content_type)
	if content_type == "" {
		return nil
	}
	media_type, _, err := mime.ParseMediaType(content_type)
	if err != nil {
		return fmt.Errorf("parse webpage content type %q: %w", content_type, err)
	}
	media_type = strings.ToLower(media_type)
	if media_type == "text/html" || media_type == "application/xhtml+xml" || strings.HasPrefix(media_type, "text/") {
		return nil
	}
	return fmt.Errorf("URL 返回的内容不是网页: %s", media_type)
}

func parse_page(html_text string, requested_url string, final_url string) (*Page, error) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(html_text))
	if err != nil {
		return nil, fmt.Errorf("parse webpage HTML: %w", err)
	}
	final_url = normalized_absolute_url(final_url, requested_url)
	if final_url == "" {
		final_url = requested_url
	}

	canonical_url := resolved_attribute(document, final_url, `link[rel="canonical"]`, "href")
	title := first_non_empty(
		meta_content(document, `meta[property="og:title"]`),
		meta_content(document, `meta[name="twitter:title"]`),
		normalize_text(document.Find("title").First().Text()),
		normalize_text(document.Find("h1").First().Text()),
	)
	description := first_non_empty(
		meta_content(document, `meta[name="description"]`),
		meta_content(document, `meta[property="og:description"]`),
		meta_content(document, `meta[name="twitter:description"]`),
	)
	author := first_non_empty(
		meta_content(document, `meta[name="author"]`),
		meta_content(document, `meta[property="article:author"]`),
	)
	site_name := first_non_empty(
		meta_content(document, `meta[property="og:site_name"]`),
		meta_content(document, `meta[name="application-name"]`),
	)
	language, _ := document.Find("html").First().Attr("lang")
	image_url := first_non_empty(
		resolved_meta_content(document, final_url, `meta[property="og:image"]`),
		resolved_meta_content(document, final_url, `meta[name="twitter:image"]`),
	)
	favicon_url := first_non_empty(
		resolved_favicon_url(document, final_url),
		default_favicon_url(final_url),
	)
	publish_time := first_page_publish_time(document)

	content_selection := select_page_content(document)
	content_html, err := clean_page_content(content_selection, final_url)
	if err != nil {
		return nil, err
	}
	content_document, err := goquery.NewDocumentFromReader(strings.NewReader(content_html))
	if err != nil {
		return nil, fmt.Errorf("parse cleaned webpage HTML: %w", err)
	}
	text_content := normalize_text(content_document.Text())
	if description == "" {
		description = truncate_text(text_content, 280)
	}
	if title == "" {
		if parsed_final_url, parse_err := url.Parse(final_url); parse_err == nil {
			title = parsed_final_url.Hostname()
		}
	}
	if image_url == "" {
		image_url = resolved_attribute(content_document, final_url, "img[src]", "src")
	}

	markdown := ""
	if content_html != "" {
		converter := htmltomarkdown.NewConverter(final_url, true, nil)
		markdown, err = converter.ConvertString(content_html)
		if err != nil {
			return nil, fmt.Errorf("convert webpage HTML to Markdown: %w", err)
		}
		markdown = strings.TrimSpace(markdown)
	}
	if markdown == "" {
		markdown = text_content
	}

	return &Page{
		URL:          first_non_empty(canonical_url, final_url, requested_url),
		RequestedURL: strings.TrimSpace(requested_url),
		FinalURL:     strings.TrimSpace(final_url),
		CanonicalURL: canonical_url,
		Title:        title,
		Description:  description,
		Author:       author,
		SiteName:     site_name,
		Language:     strings.TrimSpace(language),
		ImageURL:     image_url,
		FaviconURL:   favicon_url,
		PublishTime:  publish_time,
		HTML:         content_html,
		Text:         text_content,
		Markdown:     markdown,
	}, nil
}

func first_page_publish_time(document *goquery.Document) *int64 {
	if document == nil {
		return nil
	}
	time_element := document.Find("time,div[datetime]").First()
	if time_element.Length() == 0 {
		return nil
	}
	time_value := strings.TrimSpace(time_element.AttrOr("datetime", ""))
	if time_value == "" && goquery.NodeName(time_element) == "time" {
		time_value = normalize_text(time_element.Text())
	}
	return parse_page_publish_time(time_value)
}

func parse_page_publish_time(value string) *int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if numeric_time, err := strconv.ParseInt(value, 10, 64); err == nil && numeric_time > 0 {
		publish_time := numeric_time
		switch {
		case numeric_time >= 100000000000000000:
			publish_time /= 1000000
		case numeric_time >= 100000000000000:
			publish_time /= 1000
		case numeric_time < 100000000000:
			publish_time *= 1000
		}
		return &publish_time
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05-0700",
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
	} {
		parsed_time, err := time.Parse(layout, value)
		if err == nil {
			publish_time := parsed_time.UnixMilli()
			return &publish_time
		}
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"Jan 2, 2006",
		"January 2, 2006",
	} {
		parsed_time, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			publish_time := parsed_time.UnixMilli()
			return &publish_time
		}
	}
	return nil
}

func resolved_favicon_url(document *goquery.Document, base_url string) string {
	if document == nil {
		return ""
	}
	best_url := ""
	best_score := -1
	document.Find("link[rel][href]").Each(func(_ int, link *goquery.Selection) {
		relation, _ := link.Attr("rel")
		score := favicon_relation_score(relation)
		if score < 0 {
			return
		}
		if icon_type, exists := link.Attr("type"); exists && strings.EqualFold(strings.TrimSpace(icon_type), "image/svg+xml") {
			score += 100
		}
		if sizes, exists := link.Attr("sizes"); exists {
			score += favicon_size_score(sizes)
		}
		href, _ := link.Attr("href")
		icon_url := normalized_absolute_url(href, base_url)
		if icon_url != "" && score > best_score {
			best_url = icon_url
			best_score = score
		}
	})
	return best_url
}

func favicon_relation_score(relation string) int {
	score := -1
	for _, token := range strings.Fields(strings.ToLower(strings.TrimSpace(relation))) {
		switch token {
		case "icon":
			if score < 300 {
				score = 300
			}
		case "apple-touch-icon", "apple-touch-icon-precomposed":
			if score < 200 {
				score = 200
			}
		case "mask-icon":
			if score < 100 {
				score = 100
			}
		}
	}
	return score
}

func favicon_size_score(sizes string) int {
	best_score := 0
	for _, size := range strings.Fields(strings.ToLower(strings.TrimSpace(sizes))) {
		if size == "any" {
			return 80
		}
		parts := strings.SplitN(size, "x", 2)
		if len(parts) != 2 {
			continue
		}
		width, width_err := strconv.Atoi(parts[0])
		height, height_err := strconv.Atoi(parts[1])
		if width_err != nil || height_err != nil || width <= 0 || height <= 0 {
			continue
		}
		dimension := width
		if height < dimension {
			dimension = height
		}
		score := dimension / 16
		if score > 64 {
			score = 64
		}
		if score > best_score {
			best_score = score
		}
	}
	return best_score
}

func default_favicon_url(base_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(base_url))
	if err != nil || parsed_url.Host == "" {
		return ""
	}
	scheme := strings.ToLower(parsed_url.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	return (&url.URL{Scheme: scheme, Host: parsed_url.Host, Path: "/favicon.ico"}).String()
}

func select_page_content(document *goquery.Document) *goquery.Selection {
	selectors := []string{
		"article",
		"main",
		`[role="main"]`,
		".article-content",
		".post-content",
		".entry-content",
		"#content",
		".content",
	}
	var best_selection *goquery.Selection
	best_score := -1
	for _, selector := range selectors {
		document.Find(selector).EachWithBreak(func(_ int, selection *goquery.Selection) bool {
			score := page_content_score(selection)
			if score > best_score {
				best_selection = selection
				best_score = score
			}
			return true
		})
	}
	if best_selection == nil || best_score == 0 {
		body := document.Find("body").First()
		if body.Length() > 0 {
			return body
		}
		return document.Selection
	}
	return best_selection
}

func page_content_score(selection *goquery.Selection) int {
	if selection == nil {
		return 0
	}
	clone := selection.Clone()
	clone.Find(removable_content_selector).Remove()
	return utf8.RuneCountInString(normalize_text(clone.Text()))
}

func meta_content(document *goquery.Document, selector string) string {
	value, _ := document.Find(selector).First().Attr("content")
	return normalize_text(value)
}

func resolved_meta_content(document *goquery.Document, base_url string, selector string) string {
	return normalized_absolute_url(meta_content(document, selector), base_url)
}

type attribute_finder interface {
	Find(selector string) *goquery.Selection
}

func resolved_attribute(finder attribute_finder, base_url string, selector string, attribute string) string {
	value, _ := finder.Find(selector).First().Attr(attribute)
	return normalized_absolute_url(value, base_url)
}

func normalized_absolute_url(reference string, base_url string) string {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return ""
	}
	parsed_reference, err := url.Parse(reference)
	if err != nil {
		return ""
	}
	if !parsed_reference.IsAbs() {
		parsed_base, parse_err := url.Parse(strings.TrimSpace(base_url))
		if parse_err != nil || parsed_base.Scheme == "" || parsed_base.Hostname() == "" {
			return ""
		}
		parsed_reference = parsed_base.ResolveReference(parsed_reference)
	}
	scheme := strings.ToLower(parsed_reference.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	parsed_reference.Scheme = scheme
	return parsed_reference.String()
}

func normalize_text(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func truncate_text(value string, max_length int) string {
	value = normalize_text(value)
	if max_length <= 0 || utf8.RuneCountInString(value) <= max_length {
		return value
	}
	runes := []rune(value)
	for len(runes) > 0 && unicode.IsSpace(runes[len(runes)-1]) {
		runes = runes[:len(runes)-1]
	}
	return string(runes[:max_length]) + "…"
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if value = normalize_text(value); value != "" {
			return value
		}
	}
	return ""
}
