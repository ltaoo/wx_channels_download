// Package shuba69 fetches novel profiles and chapter directories from 69shuba.
package shuba69

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog"

	"wx_channel/pkg/cache"
	"wx_channel/pkg/clawreq"
)

var (
	book_id_pattern            = regexp.MustCompile(`/book/(\d+)`)
	book_directory_url_pattern = regexp.MustCompile(`^https?://(?:www\.)?69shuba\.com/book/\d+/$`)
)

// Client fetches and parses 69shuba novel directories.
type Client struct {
	claw_client     *clawreq.Client
	claw_client_err error
	cookie          string
	user_agent      string
	logger          *zerolog.Logger
	work_dir        string
	file_cache      *cache.CacheProvider
}

type fetched_page struct {
	html_text   string
	final_url   string
	cache_entry *HTMLCacheEntry
}

type page_diagnostics struct {
	page_title      string
	h1              string
	html_bytes      int
	html_sha256     string
	text_preview    string
	challenge_kind  string
	selector_counts map[string]int
	expected_url    string
}

// NewClient creates a 69shuba client.
func NewClient() *Client {
	return NewClientWithOptions("", "")
}

// NewClientWithCookie creates a 69shuba client with a Cookie header.
func NewClientWithCookie(cookie string) *Client {
	return NewClientWithOptions(cookie, "")
}

// NewClientWithOptions creates a 69shuba client with optional Cookie and
// User-Agent headers. Cloudflare clearance cookies are tied to the User-Agent
// used when they were issued, so callers may need to provide both values.
func NewClientWithOptions(cookie string, user_agent string) *Client {
	claw_client, claw_client_err := clawreq.New(clawreq.Config{
		Profile:         clawreq.ProfileChrome,
		Timeout:         30 * time.Second,
		FollowRedirects: true,
	})
	return &Client{
		claw_client:     claw_client,
		claw_client_err: claw_client_err,
		cookie:          strings.TrimSpace(cookie),
		user_agent:      strings.TrimSpace(user_agent),
	}
}

// NewClientWithLogger creates a configured client that emits structured
// request and parse diagnostics through logger.
func NewClientWithLogger(cookie string, user_agent string, logger *zerolog.Logger) *Client {
	client := NewClientWithOptions(cookie, user_agent)
	client.logger = logger
	return client
}

// SetLogger configures structured diagnostic logging for subsequent requests.
func (c *Client) SetLogger(logger *zerolog.Logger) {
	if c == nil {
		return
	}
	c.logger = logger
}

// FetchNovelChapters fetches a novel profile and its chapter directory.
// A profile URL looks like https://www.69shuba.com/book/34567.htm. The client
// follows that page's “完整目录” link to fetch the chapter directory.
func (c *Client) FetchNovelChapters(raw_url string) (*Novel, error) {
	return c.FetchNovelChaptersContext(context.Background(), raw_url)
}

// FetchNovelChaptersContext fetches a novel profile and chapter directory with
// cancellation support.
func (c *Client) FetchNovelChaptersContext(fetch_context context.Context, raw_url string) (*Novel, error) {
	if fetch_context == nil {
		fetch_context = context.Background()
	}
	raw_url = normalize_novel_profile_url(raw_url)
	profile_page, err := c.fetch(fetch_context, raw_url, "", "profile")
	if err != nil {
		return nil, err
	}
	profile_url := profile_page.final_url
	if strings.TrimSpace(profile_url) == "" {
		profile_url = strings.TrimSpace(raw_url)
	}
	novel, directory_url, err := c.parse_novel_profile(profile_page.html_text, profile_url, profile_page.cache_entry)
	if err != nil {
		return nil, err
	}
	directory_page, err := c.fetch(fetch_context, directory_url, profile_url, "directory")
	if err != nil {
		return nil, fmt.Errorf("fetch 69shuba complete directory %q discovered from %q: %w", directory_url, profile_url, err)
	}
	directory_page_url := directory_page.final_url
	if strings.TrimSpace(directory_page_url) == "" {
		directory_page_url = directory_url
	}
	return c.parse_chapter_directory(novel, directory_page.html_text, directory_page_url, directory_page.cache_entry)
}

func normalize_novel_profile_url(raw_url string) string {
	raw_url = strings.TrimSpace(raw_url)
	if !book_directory_url_pattern.MatchString(raw_url) {
		return raw_url
	}
	return strings.TrimSuffix(raw_url, "/") + ".htm"
}

func (c *Client) fetch(fetch_context context.Context, raw_url string, referer string, page_kind string) (*fetched_page, error) {
	if c == nil {
		return nil, errors.New("69shuba client is not initialized")
	}
	if c.claw_client == nil {
		if c.claw_client_err != nil {
			return nil, fmt.Errorf("initialize 69shuba clawreq client: %w", c.claw_client_err)
		}
		return nil, errors.New("69shuba clawreq client is not initialized")
	}

	raw_url = strings.TrimSpace(raw_url)
	request_options := []clawreq.RequestOption{
		clawreq.WithHeaders(map[string]string{
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
			"Cache-Control":   "max-age=0",
			"Sec-Fetch-Site":  "same-origin",
		}),
	}
	if c.cookie != "" {
		request_options = append(request_options, clawreq.WithCookie(c.cookie))
	}
	if c.user_agent != "" {
		request_options = append(request_options, clawreq.WithHeader("User-Agent", c.user_agent))
	}
	if referer = strings.TrimSpace(referer); referer != "" {
		request_options = append(request_options, clawreq.WithReferer(referer))
	}

	request_started_at := time.Now()
	if c.logger != nil {
		c.logger.Info().
			Str("http_client", "clawreq").
			Str("url", raw_url).
			Str("page_kind", page_kind).
			Str("referer", referer).
			Bool("has_cookie", c.cookie != "").
			Bool("custom_user_agent", c.user_agent != "").
			Msg("69shuba clawreq: request started")
	}
	response, err := c.claw_client.Get(fetch_context, raw_url, request_options...)
	if err != nil {
		wrapped_err := fmt.Errorf("fetch 69shuba page with clawreq: url=%q: %w", raw_url, err)
		if c.logger != nil {
			c.logger.Error().
				Err(wrapped_err).
				Str("http_client", "clawreq").
				Str("url", raw_url).
				Str("page_kind", page_kind).
				Dur("elapsed", time.Since(request_started_at)).
				Msg("69shuba clawreq: request failed")
		}
		return nil, wrapped_err
	}
	final_url := strings.TrimSpace(response.FinalURL)
	if final_url == "" {
		final_url = raw_url
	}
	html_text, decode_err := response.Text()
	cache_entry, cache_err := c.cache_response(page_kind, raw_url, html_text)
	if cache_err != nil {
		wrapped_err := fmt.Errorf("cache 69shuba %s response: url=%q work_dir=%q: %w", page_kind, raw_url, c.work_dir, cache_err)
		if c.logger != nil {
			c.logger.Error().
				Err(wrapped_err).
				Str("url", raw_url).
				Str("page_kind", page_kind).
				Str("work_dir", c.work_dir).
				Msg("69shuba cache: response persistence failed")
		}
		return nil, wrapped_err
	}
	if cache_entry != nil && c.logger != nil {
		c.logger.Info().
			Str("url", raw_url).
			Str("page_kind", page_kind).
			Str("cache_directory", cache_entry.Directory).
			Str("html_cache_path", cache_entry.HTMLPath).
			Msg("69shuba cache: response persisted")
	}
	if decode_err != nil {
		cache_directory := ""
		html_cache_path := ""
		if cache_entry != nil {
			cache_directory = cache_entry.Directory
			html_cache_path = cache_entry.HTMLPath
		}
		wrapped_err := fmt.Errorf("decode 69shuba clawreq response: url=%q status=%d content_type=%q body_bytes=%d cache_directory=%q html_cache_path=%q: %w", raw_url, response.StatusCode, response.ContentType(), len(response.Body), cache_directory, html_cache_path, decode_err)
		if c.logger != nil {
			c.logger.Error().
				Err(wrapped_err).
				Str("http_client", "clawreq").
				Str("url", raw_url).
				Str("page_kind", page_kind).
				Int("http_status", response.StatusCode).
				Str("content_type", response.ContentType()).
				Int("body_bytes", len(response.Body)).
				Str("cache_directory", cache_directory).
				Str("html_cache_path", html_cache_path).
				Dur("elapsed", time.Since(request_started_at)).
				Msg("69shuba clawreq: response decode failed")
		}
		return nil, wrapped_err
	}
	response_diagnostics := inspect_page(html_text, raw_url, nil)
	cache_directory := ""
	html_cache_path := ""
	if cache_entry != nil {
		cache_directory = cache_entry.Directory
		html_cache_path = cache_entry.HTMLPath
	}
	if c.logger != nil {
		c.logger.Info().
			Str("http_client", "clawreq").
			Str("url", raw_url).
			Str("page_kind", page_kind).
			Str("final_url", final_url).
			Int("http_status", response.StatusCode).
			Str("content_type", response.ContentType()).
			Int("body_bytes", len(response.Body)).
			Str("body_sha256", fmt.Sprintf("%x", sha256.Sum256(response.Body))).
			Str("page_title", response_diagnostics.page_title).
			Str("challenge_kind", response_diagnostics.challenge_kind).
			Str("cache_directory", cache_directory).
			Str("html_cache_path", html_cache_path).
			Dur("elapsed", time.Since(request_started_at)).
			Msg("69shuba clawreq: response received")
	}
	if response.StatusCode != http.StatusOK {
		wrapped_err := fmt.Errorf("fetch 69shuba page with clawreq: url=%q final_url=%q status=%d content_type=%q body_bytes=%d page_title=%q challenge=%q cache_directory=%q html_cache_path=%q", raw_url, final_url, response.StatusCode, response.ContentType(), len(response.Body), response_diagnostics.page_title, response_diagnostics.challenge_kind, cache_directory, html_cache_path)
		if c.logger != nil {
			c.logger.Error().
				Err(wrapped_err).
				Str("http_client", "clawreq").
				Str("url", raw_url).
				Str("page_kind", page_kind).
				Str("final_url", final_url).
				Int("http_status", response.StatusCode).
				Str("content_type", response.ContentType()).
				Int("body_bytes", len(response.Body)).
				Str("page_title", response_diagnostics.page_title).
				Str("challenge_kind", response_diagnostics.challenge_kind).
				Str("text_preview", response_diagnostics.text_preview).
				Str("cache_directory", cache_directory).
				Str("html_cache_path", html_cache_path).
				Msg("69shuba clawreq: unexpected HTTP status")
		}
		return nil, wrapped_err
	}
	return &fetched_page{html_text: html_text, final_url: final_url, cache_entry: cache_entry}, nil
}

func (c *Client) parse_novel_chapters(html_text string, page_url string) (*Novel, error) {
	return c.parse_novel_chapters_with_cache(html_text, page_url, nil)
}

func (c *Client) parse_novel_profile(html_text string, page_url string, cache_entry *HTMLCacheEntry) (*Novel, string, error) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(html_text))
	if err != nil {
		return nil, "", fmt.Errorf("parse 69shuba novel profile: url=%q cache_directory=%q: %w", page_url, cache_directory(cache_entry), err)
	}
	novel := novel_metadata(document, page_url)
	diagnostics := inspect_page(html_text, page_url, document)
	directory_href := ""
	more_button_count := document.Find(".more-btn").Length()
	document.Find(".more-btn").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		button_text := strings.Join(strings.Fields(selection.Text()), "")
		if button_text != "完整目录" {
			return true
		}
		href, exists := selection.Attr("href")
		if !exists || strings.TrimSpace(href) == "" {
			return true
		}
		directory_href = strings.TrimSpace(href)
		return false
	})
	if directory_href == "" {
		parse_err := fmt.Errorf("69shuba complete directory link not found: url=%q selector=%q required_text=%q more_button_count=%d page_title=%q h1=%q challenge=%q cache_directory=%q html_cache_path=%q", page_url, ".more-btn", "完整目录", more_button_count, diagnostics.page_title, diagnostics.h1, diagnostics.challenge_kind, cache_directory(cache_entry), html_cache_path(cache_entry))
		if c.logger != nil {
			c.logger.Error().
				Err(parse_err).
				Str("url", page_url).
				Str("selector", ".more-btn").
				Str("required_text", "完整目录").
				Int("more_button_count", more_button_count).
				Str("page_title", diagnostics.page_title).
				Str("h1", diagnostics.h1).
				Str("challenge_kind", diagnostics.challenge_kind).
				Str("text_preview", diagnostics.text_preview).
				Str("cache_directory", cache_directory(cache_entry)).
				Str("html_cache_path", html_cache_path(cache_entry)).
				Msg("69shuba parser: complete directory link not found")
		}
		return nil, "", parse_err
	}
	directory_url := normalize_url(directory_href, page_url)
	parsed_directory_url, err := url.Parse(directory_url)
	if err != nil || (parsed_directory_url.Scheme != "http" && parsed_directory_url.Scheme != "https") || parsed_directory_url.Host == "" {
		return nil, "", fmt.Errorf("invalid 69shuba complete directory url: profile_url=%q href=%q resolved_url=%q cache_directory=%q", page_url, directory_href, directory_url, cache_directory(cache_entry))
	}
	if c.logger != nil {
		c.logger.Info().
			Str("profile_url", page_url).
			Str("directory_href", directory_href).
			Str("directory_url", directory_url).
			Str("book_id", novel.BookID).
			Str("book_title", novel.Title).
			Str("cache_directory", cache_directory(cache_entry)).
			Msg("69shuba parser: complete directory link discovered")
	}
	return novel, directory_url, nil
}

func (c *Client) parse_novel_chapters_with_cache(html_text string, page_url string, cache_entry *HTMLCacheEntry) (*Novel, error) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(html_text))
	if err != nil {
		return nil, fmt.Errorf("parse 69shuba page: url=%q cache_directory=%q: %w", page_url, cache_directory(cache_entry), err)
	}
	return c.parse_chapter_directory_document(novel_metadata(document, page_url), document, html_text, page_url, cache_entry)
}

func novel_metadata(document *goquery.Document, page_url string) *Novel {
	novel := &Novel{URL: page_url, Chapters: make([]Chapter, 0)}
	if matches := book_id_pattern.FindStringSubmatch(page_url); len(matches) > 1 {
		novel.BookID = matches[1]
	}

	novel.Title = meta_content(document, "og:novel:book_name")
	if novel.Title == "" {
		novel.Title = strings.TrimSpace(document.Find("h1").First().Text())
	}
	if novel.Title == "" {
		novel.Title = strings.TrimSpace(document.Find(".bread-crumbs a").Last().Text())
	}
	novel.Author = meta_content(document, "og:novel:author")
	novel.Category = meta_content(document, "og:novel:category")
	novel.Status = meta_content(document, "og:novel:status")
	cover_src, _ := document.Find(".bookbox img").First().Attr("src")
	cover_src = strings.TrimSpace(cover_src)
	if cover_src != "" {
		novel.CoverURL = normalize_url(cover_src, page_url)
	}
	if novel.CoverURL == "" {
		cover_src = meta_content(document, "og:image")
		if cover_src != "" {
			novel.CoverURL = normalize_url(cover_src, page_url)
		}
	}

	document.Find(".bookinfo .booknav2 p, .bookinfo .booknav2 span").Each(func(_ int, selection *goquery.Selection) {
		text := strings.TrimSpace(selection.Text())
		switch {
		case strings.Contains(text, "作者"):
			novel.Author = value_after_label(text, "作者")
			if link := selection.Find("a").First(); link.Length() > 0 {
				novel.Author = strings.TrimSpace(link.Text())
			}
		case strings.Contains(text, "分类") || strings.Contains(text, "类型"):
			novel.Category = value_after_label(text, "分类", "类型")
			if link := selection.Find("a").First(); link.Length() > 0 {
				novel.Category = strings.TrimSpace(link.Text())
			}
		case strings.Contains(text, "状态"):
			novel.Status = value_after_label(text, "状态")
		}
	})
	return novel
}

func meta_content(document *goquery.Document, property string) string {
	if document == nil {
		return ""
	}
	content, _ := document.Find(fmt.Sprintf(`meta[property=%q]`, property)).First().Attr("content")
	return strings.TrimSpace(content)
}

func (c *Client) parse_chapter_directory(novel *Novel, html_text string, page_url string, cache_entry *HTMLCacheEntry) (*Novel, error) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(html_text))
	if err != nil {
		return nil, fmt.Errorf("parse 69shuba chapter directory: url=%q cache_directory=%q: %w", page_url, cache_directory(cache_entry), err)
	}
	return c.parse_chapter_directory_document(novel, document, html_text, page_url, cache_entry)
}

func (c *Client) parse_chapter_directory_document(novel *Novel, document *goquery.Document, html_text string, page_url string, cache_entry *HTMLCacheEntry) (*Novel, error) {
	if novel == nil {
		return nil, errors.New("69shuba novel profile is nil")
	}
	novel.Chapters = make([]Chapter, 0)

	chapter_selectors := []string{
		".mu_contain ul li a",
		"#chapterlist a",
		".mulu_list li a",
		".catalog li a",
		"#catalog ul li a",
	}
	diagnostics := inspect_page(html_text, page_url, document)
	uses_data_num := false
	for _, selector := range chapter_selectors {
		document.Find(selector).Each(func(_ int, selection *goquery.Selection) {
			href, exists := selection.Attr("href")
			title := strings.TrimSpace(selection.Text())
			if !exists || strings.TrimSpace(href) == "" || title == "" {
				return
			}
			chapter_index := len(novel.Chapters) + 1
			if data_num, valid := chapter_data_num(selection); valid {
				chapter_index = data_num
				uses_data_num = true
			}
			novel.Chapters = append(novel.Chapters, Chapter{
				Index: chapter_index,
				Title: title,
				URL:   normalize_url(href, page_url),
			})
		})
		if len(novel.Chapters) > 0 {
			break
		}
	}
	if uses_data_num {
		sort.SliceStable(novel.Chapters, func(left_index int, right_index int) bool {
			return novel.Chapters[left_index].Index < novel.Chapters[right_index].Index
		})
	}

	if len(novel.Chapters) == 0 {
		selector_counts_json, _ := json.Marshal(diagnostics.selector_counts)
		parse_err := fmt.Errorf("69shuba chapter directory not found: url=%q page_title=%q h1=%q html_bytes=%d html_sha256=%s challenge=%q selector_counts=%s expected_url=%q cache_directory=%q html_cache_path=%q", page_url, diagnostics.page_title, diagnostics.h1, diagnostics.html_bytes, diagnostics.html_sha256, diagnostics.challenge_kind, selector_counts_json, diagnostics.expected_url, cache_directory(cache_entry), html_cache_path(cache_entry))
		if c.logger != nil {
			c.logger.Error().
				Err(parse_err).
				Str("url", page_url).
				Str("page_title", diagnostics.page_title).
				Str("h1", diagnostics.h1).
				Int("html_bytes", diagnostics.html_bytes).
				Str("html_sha256", diagnostics.html_sha256).
				Str("challenge_kind", diagnostics.challenge_kind).
				Interface("selector_counts", diagnostics.selector_counts).
				Str("expected_url", diagnostics.expected_url).
				Str("text_preview", diagnostics.text_preview).
				Str("cache_directory", cache_directory(cache_entry)).
				Str("html_cache_path", html_cache_path(cache_entry)).
				Msg("69shuba parser: chapter directory not found")
		}
		return nil, parse_err
	}
	if c.logger != nil {
		c.logger.Info().
			Str("directory_url", page_url).
			Str("book_id", novel.BookID).
			Str("book_title", novel.Title).
			Str("author", novel.Author).
			Str("category", novel.Category).
			Str("status", novel.Status).
			Int("chapter_count", len(novel.Chapters)).
			Bool("ordered_by_data_num", uses_data_num).
			Interface("selector_counts", diagnostics.selector_counts).
			Str("cache_directory", cache_directory(cache_entry)).
			Msg("69shuba parser: chapter directory parsed")
	}
	return novel, nil
}

func chapter_data_num(selection *goquery.Selection) (int, bool) {
	if selection == nil {
		return 0, false
	}
	data_num_text, exists := selection.Attr("data-num")
	if !exists {
		data_num_text, exists = selection.Closest("[data-num]").Attr("data-num")
	}
	if !exists {
		return 0, false
	}
	data_num, err := strconv.Atoi(strings.TrimSpace(data_num_text))
	if err != nil || data_num <= 0 {
		return 0, false
	}
	return data_num, true
}

func cache_directory(cache_entry *HTMLCacheEntry) string {
	if cache_entry == nil {
		return ""
	}
	return cache_entry.Directory
}

func html_cache_path(cache_entry *HTMLCacheEntry) string {
	if cache_entry == nil {
		return ""
	}
	return cache_entry.HTMLPath
}

func inspect_page(html_text string, page_url string, document *goquery.Document) page_diagnostics {
	if document == nil {
		parsed_document, err := goquery.NewDocumentFromReader(strings.NewReader(html_text))
		if err == nil {
			document = parsed_document
		}
	}
	diagnostics := page_diagnostics{
		html_bytes:      len(html_text),
		html_sha256:     fmt.Sprintf("%x", sha256.Sum256([]byte(html_text))),
		challenge_kind:  detect_challenge(html_text),
		selector_counts: make(map[string]int),
		expected_url:    expected_book_url(page_url),
	}
	if document == nil {
		return diagnostics
	}
	diagnostics.page_title = strings.TrimSpace(document.Find("title").First().Text())
	diagnostics.h1 = strings.TrimSpace(document.Find("h1").First().Text())
	diagnostics.text_preview = truncate_text(strings.Join(strings.Fields(document.Text()), " "), 300)
	for _, selector := range []string{
		".mu_contain ul li a",
		"#chapterlist a",
		".mulu_list li a",
		".catalog li a",
		"#catalog ul li a",
	} {
		diagnostics.selector_counts[selector] = document.Find(selector).Length()
	}
	return diagnostics
}

func detect_challenge(html_text string) string {
	normalized := strings.ToLower(html_text)
	switch {
	case strings.Contains(normalized, "cf-chl-") ||
		strings.Contains(normalized, "challenge-platform") ||
		strings.Contains(normalized, "just a moment..."):
		return "cloudflare"
	case strings.Contains(normalized, "captcha") || strings.Contains(normalized, "验证码"):
		return "captcha"
	case strings.Contains(normalized, "access denied") || strings.Contains(normalized, "访问被拒绝"):
		return "access_denied"
	default:
		return ""
	}
}

func expected_book_url(page_url string) string {
	matches := book_id_pattern.FindStringSubmatch(page_url)
	if len(matches) < 2 {
		return ""
	}
	parsed_url, err := url.Parse(strings.TrimSpace(page_url))
	if err != nil || parsed_url.Host == "" {
		return ""
	}
	parsed_url.Path = "/book/" + matches[1] + "/"
	parsed_url.RawPath = ""
	parsed_url.RawQuery = ""
	parsed_url.Fragment = ""
	expected_url := parsed_url.String()
	if expected_url == strings.TrimSpace(page_url) {
		return ""
	}
	return expected_url
}

func truncate_text(text string, max_runes int) string {
	if max_runes <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= max_runes {
		return string(runes)
	}
	return string(runes[:max_runes]) + "…"
}

func value_after_label(text string, labels ...string) string {
	for _, label := range labels {
		for _, separator := range []string{"：", ":"} {
			prefix := label + separator
			if strings.HasPrefix(text, prefix) {
				return strings.TrimSpace(strings.TrimPrefix(text, prefix))
			}
		}
	}
	return strings.TrimSpace(text)
}

func normalize_url(href string, page_url string) string {
	page, page_err := url.Parse(strings.TrimSpace(page_url))
	reference, reference_err := url.Parse(strings.TrimSpace(href))
	if page_err == nil && reference_err == nil {
		return page.ResolveReference(reference).String()
	}
	return strings.TrimSpace(href)
}
