package ttk

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/pkg/cache"
	"wx_channel/pkg/clawreq"
	"wx_channel/pkg/cookies"
)

const ttk_cookie_domain = ".ttks.tw"

// Client fetches TTK novel profiles and chapter contents.
type Client struct {
	claw_client      *clawreq.Client
	claw_client_err  error
	cookie           string
	cookie_provider  *cookies.Reader
	user_agent       string
	progress_handler FetchProgressHandler
	file_cache       *cache.CacheProvider
}

// NewClient creates a TTK client. When provided, cookie_provider is queried
// for the latest .ttks.tw cookies before every outbound request.
func NewClient(cookie_providers ...*cookies.Reader) *Client {
	return NewClientWithOptions("", "", cookie_providers...)
}

// NewClientWithCookie creates a TTK client with a Cookie header.
func NewClientWithCookie(cookie string, cookie_providers ...*cookies.Reader) *Client {
	return NewClientWithOptions(cookie, "", cookie_providers...)
}

// NewClientWithOptions creates a TTK client with optional Cookie and
// User-Agent headers. An explicit Cookie takes precedence over cookies read
// from cookie_provider.
func NewClientWithOptions(cookie string, user_agent string, cookie_providers ...*cookies.Reader) *Client {
	claw_client, claw_client_err := clawreq.New(clawreq.Config{
		Profile:         clawreq.ProfileChrome,
		Timeout:         30 * time.Second,
		FollowRedirects: true,
	})
	var cookie_provider *cookies.Reader
	if len(cookie_providers) > 0 {
		cookie_provider = cookie_providers[0]
	}
	return &Client{
		claw_client:     claw_client,
		claw_client_err: claw_client_err,
		cookie:          strings.TrimSpace(cookie),
		cookie_provider: cookie_provider,
		user_agent:      strings.TrimSpace(user_agent),
	}
}

// SetCookieProvider configures the persistent cookie provider used by future
// requests. The provider is retained so updated cookies are read on demand.
func (c *Client) SetCookieProvider(cookie_provider *cookies.Reader) {
	if c == nil {
		return
	}
	c.cookie_provider = cookie_provider
}

// Fetch fetches a TTK novel profile and every chapter in directory order.
func (c *Client) Fetch(params FetchParams) (fetch_result any, fetch_err error) {
	if c == nil {
		return nil, errors.New("ttk client is not initialized")
	}
	raw_url := strings.TrimSpace(params.URL)
	fetch_context := params.Context
	if fetch_context == nil {
		fetch_context = context.Background()
	}
	progress := FetchProgress{
		RequestID: strings.TrimSpace(params.RequestID),
		Platform:  "ttk",
		URL:       raw_url,
		Stage:     FetchStageStart,
		Status:    FetchStatusRunning,
		Message:   "正在准备获取 TTK 小说",
	}
	c.report_progress(progress)
	defer func() {
		if fetch_err == nil {
			return
		}
		if errors.Is(fetch_err, ErrFetchInterrupted) || errors.Is(fetch_err, context.Canceled) {
			progress.Stage = FetchStageInterrupted
			progress.Status = FetchStatusInterrupted
			progress.Message = "已中断获取 TTK 小说"
		} else {
			progress.Stage = FetchStageFailed
			progress.Status = FetchStatusFailed
			progress.Message = "获取 TTK 小说失败"
		}
		progress.Error = fetch_err.Error()
		c.report_progress(progress)
	}()

	book_id, err := validate_novel_url(raw_url)
	if err != nil {
		return nil, err
	}
	progress.BookID = book_id
	if err := check_fetch_interrupted(fetch_context); err != nil {
		return nil, err
	}
	progress.Stage = FetchStageProfile
	progress.Message = "正在获取小说信息和章节目录"
	c.report_progress(progress)

	profile_response, err := c.fetch_novel_chapters(fetch_context, raw_url, params.ForceRefresh)
	if err != nil {
		return nil, fmt.Errorf("fetch profile: %w", normalize_fetch_error(err))
	}
	if profile_response == nil || profile_response.Data == nil {
		return nil, errors.New("fetch profile: empty response")
	}
	profile := profile_response.Data
	profile.URL = raw_url
	progress.BookTitle = profile.Title
	progress.Cached = profile_response.Cached
	if profile_response.Cached {
		progress.CacheHits++
	}

	total := len(profile.Chapters)
	chapters := make([]TtkFetchedChapter, 0, total)
	progress.Stage = FetchStageDirectory
	progress.Status = FetchStatusCompleted
	progress.Total = total
	progress.Message = fmt.Sprintf("章节目录获取完成，共 %d 章", total)
	progress.Profile = profile
	if profile_response.Cached {
		progress.Message = fmt.Sprintf("已复用章节目录缓存，共 %d 章", total)
	}
	c.report_progress(progress)

	for chapter_index, chapter := range profile.Chapters {
		if err := check_fetch_interrupted(fetch_context); err != nil {
			return nil, err
		}
		current_index := chapter_index + 1
		progress.Stage = FetchStageChapter
		progress.Status = FetchStatusRunning
		progress.Current = chapter_index
		progress.Percent = fetch_percent(progress.Current, total)
		progress.ChapterID = chapter_id_from_url(chapter.URL)
		progress.ChapterTitle = chapter.Title
		progress.Cached = false
		progress.Chapter = nil
		progress.Message = fmt.Sprintf("正在获取章节 %d/%d：%s", current_index, total, chapter.Title)
		c.report_progress(progress)

		chapter_response, fetch_err := c.fetch_chapter_content(
			fetch_context,
			raw_url,
			chapter.URL,
			raw_url,
			params.ForceRefresh,
		)
		if fetch_err != nil {
			return nil, fmt.Errorf("fetch chapter %q: %w", chapter.Title, normalize_fetch_error(fetch_err))
		}
		if chapter_response == nil || chapter_response.Data == nil {
			return nil, fmt.Errorf("fetch chapter %q: empty response", chapter.Title)
		}

		chapter_content := chapter_response.Data
		chapter_title := strings.TrimSpace(chapter_content.Title)
		if chapter_title == "" {
			chapter_title = chapter.Title
		}
		fetched_chapter := TtkFetchedChapter{
			Index:   current_index,
			URL:     normalize_url(chapter.URL, raw_url),
			Title:   chapter_title,
			Content: chapter_content.Content,
		}
		chapters = append(chapters, fetched_chapter)
		progress.Status = FetchStatusCompleted
		progress.Current = current_index
		progress.Percent = fetch_percent(current_index, total)
		progress.ChapterTitle = chapter_title
		progress.Cached = chapter_response.Cached
		progress.Chapter = &fetched_chapter
		if chapter_response.Cached {
			progress.CacheHits++
			progress.Message = fmt.Sprintf("已复用章节缓存 %d/%d：%s", current_index, total, chapter_title)
		} else {
			progress.Message = fmt.Sprintf("章节获取完成 %d/%d：%s", current_index, total, chapter_title)
		}
		c.report_progress(progress)
	}

	progress.Stage = FetchStageComplete
	progress.Status = FetchStatusCompleted
	progress.Current = total
	progress.Percent = 100
	progress.ChapterID = ""
	progress.ChapterTitle = ""
	progress.Profile = nil
	progress.Chapter = nil
	progress.Message = fmt.Sprintf("小说获取完成，共 %d 章", total)
	progress.Cached = false
	c.report_progress(progress)
	return &TtkFetchResult{Profile: profile, Chapters: chapters}, nil
}

// FetchNovelChapters fetches and parses a TTK novel directory.
func (c *Client) FetchNovelChapters(raw_url string) (*TtkNovel, error) {
	return c.FetchNovelChaptersContext(context.Background(), raw_url)
}

// FetchNovelChaptersContext fetches and parses a TTK novel directory with
// cancellation support.
func (c *Client) FetchNovelChaptersContext(fetch_context context.Context, raw_url string) (*TtkNovel, error) {
	if fetch_context == nil {
		fetch_context = context.Background()
	}
	response, err := c.fetch_novel_chapters(fetch_context, raw_url, false)
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, errors.New("ttk novel response is empty")
	}
	return response.Data, nil
}

// Close releases HTTP resources.
func (c *Client) Close() {
	if c == nil {
		return
	}
	if c.claw_client != nil {
		c.claw_client.CloseIdleConnections()
	}
}

// SetProgressHandler configures an optional progress callback for Fetch.
func (c *Client) SetProgressHandler(progress_handler FetchProgressHandler) {
	if c == nil {
		return
	}
	c.progress_handler = progress_handler
}

func (c *Client) report_progress(progress FetchProgress) {
	if c == nil || c.progress_handler == nil {
		return
	}
	func() {
		defer func() {
			_ = recover()
		}()
		c.progress_handler(progress)
	}()
}

func fetch_percent(current int, total int) float64 {
	if total <= 0 || current <= 0 {
		return 0
	}
	if current >= total {
		return 100
	}
	return float64(current) * 100 / float64(total)
}

func check_fetch_interrupted(fetch_context context.Context) error {
	if fetch_context == nil {
		return nil
	}
	select {
	case <-fetch_context.Done():
		return fmt.Errorf("%w: %v", ErrFetchInterrupted, fetch_context.Err())
	default:
		return nil
	}
}

func normalize_fetch_error(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, ErrFetchInterrupted) {
		return fmt.Errorf("%w: %v", ErrFetchInterrupted, err)
	}
	return err
}

func (c *Client) fetch_novel_chapters(fetch_context context.Context, raw_url string, force_refresh bool) (*TtkResp[*TtkNovel], error) {
	raw_url = strings.TrimSpace(raw_url)
	if _, err := validate_novel_url(raw_url); err != nil {
		return nil, err
	}
	document, cached, err := c.fetch_document(
		fetch_context,
		raw_url,
		raw_url,
		strings.TrimRight(ttk_base_url, "/")+"/",
		force_refresh,
	)
	if err != nil {
		return nil, err
	}
	novel, err := c.parse_novel_chapters_document(document, raw_url)
	if err != nil {
		_ = c.remove_cached_html(raw_url, raw_url)
		return nil, err
	}
	return &TtkResp[*TtkNovel]{Data: novel, Cached: cached}, nil
}

func (c *Client) parse_novel_chapters(html_str string, page_url string) (*TtkNovel, error) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(html_str))
	if err != nil {
		return nil, err
	}
	return c.parse_novel_chapters_document(document, page_url)
}

func (c *Client) parse_novel_chapters_document(document *goquery.Document, page_url string) (*TtkNovel, error) {
	if document == nil {
		return nil, errors.New("ttk novel document is nil")
	}
	novel := &TtkNovel{
		URL:      page_url,
		Chapters: make([]TtkChapter, 0),
	}

	novel.Title = first_text(document, []string{
		"h1",
		".novel-title",
		".book-title",
		`meta[property="og:title"]`,
		"title",
	})
	novel.Title = clean_title(novel.Title)

	novel.Author = first_text(document, []string{
		`meta[name="og:novel:author"]`,
		`meta[property="og:novel:author"]`,
		".author",
		".novel-author",
		".book-author",
		`a[href*="/author/"]`,
	})
	novel.Author = clean_author(novel.Author)

	novel.CoverURL = normalize_url(first_text(document, []string{
		`meta[name="og:image"]`,
		`meta[property="og:image"]`,
	}), page_url)
	if novel.CoverURL == "" {
		novel.CoverURL = normalize_url(first_attribute(document, []string{
			`.novel-cover img`,
			`.book-cover img`,
			`amp-img[src*="/files/article/image/"]`,
		}, "src"), page_url)
	}

	seen_urls := make(map[string]bool)
	append_chapters := func(chapter_links *goquery.Selection) {
		chapter_links.Each(func(_ int, selection *goquery.Selection) {
			href, exists := selection.Attr("href")
			if !exists {
				return
			}
			title := strings.TrimSpace(selection.Text())
			if title == "" {
				return
			}
			full_url := normalize_url(href, page_url)
			if full_url == "" || seen_urls[full_url] {
				return
			}
			seen_urls[full_url] = true
			novel.Chapters = append(novel.Chapters, TtkChapter{
				Index: len(novel.Chapters) + 1,
				Title: title,
				URL:   full_url,
			})
		})
	}

	chapter_links := find_ttk_chapter_links(document)
	if chapter_links != nil {
		append_chapters(chapter_links)
	}
	chapter_selectors := []string{
		"#chapters_frame .chapter_cell a",
		"#chapter_list a",
		".chapter-list a",
		".chapters a",
		".catalog a",
		".book-chapters a",
	}
	for _, selector := range chapter_selectors {
		if len(novel.Chapters) > 0 {
			break
		}
		append_chapters(document.Find(selector))
	}

	if novel.Title == "" {
		return nil, errors.New("ttk novel title is empty")
	}
	return novel, nil
}

func find_ttk_chapter_links(document *goquery.Document) *goquery.Selection {
	chapter_frames := document.Find(".chapters_frame")
	var full_chapter_frame *goquery.Selection
	chapter_frames.EachWithBreak(func(_ int, chapter_frame *goquery.Selection) bool {
		chapter_heading := strings.TrimSpace(chapter_frame.PrevAllFiltered(".chapters_title").First().Text())
		if strings.Contains(chapter_heading, "全部章節") || strings.Contains(chapter_heading, "全部章节") {
			full_chapter_frame = chapter_frame
			return false
		}
		return true
	})

	if full_chapter_frame == nil && chapter_frames.Length() > 1 {
		full_chapter_frame = chapter_frames.Eq(1)
	}
	if full_chapter_frame == nil && chapter_frames.Length() == 1 {
		full_chapter_frame = chapter_frames.First()
	}
	if full_chapter_frame == nil {
		return nil
	}

	chapter_links := full_chapter_frame.Find(".chapter_cell a")
	if chapter_links.Length() == 0 {
		chapter_links = full_chapter_frame.Find("a")
	}
	return chapter_links
}

// FetchChapterContent fetches and parses a TTK chapter page.
func (c *Client) FetchChapterContent(raw_url string) (*TtkChapterContent, error) {
	return c.FetchChapterContentContext(context.Background(), raw_url)
}

// FetchChapterContentContext fetches and parses a TTK chapter page with
// cancellation support.
func (c *Client) FetchChapterContentContext(fetch_context context.Context, raw_url string) (*TtkChapterContent, error) {
	if fetch_context == nil {
		fetch_context = context.Background()
	}
	response, err := c.fetch_chapter_content(fetch_context, "", raw_url, ttk_base_url+"/", false)
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, errors.New("ttk chapter response is empty")
	}
	return response.Data, nil
}

func (c *Client) fetch_chapter_content(
	fetch_context context.Context,
	source_url string,
	raw_url string,
	referer string,
	force_refresh bool,
) (*TtkResp[*TtkChapterContent], error) {
	raw_url = strings.TrimSpace(raw_url)
	if err := validate_ttk_url(raw_url); err != nil {
		return nil, err
	}
	document, cached, err := c.fetch_document(fetch_context, source_url, raw_url, referer, force_refresh)
	if err != nil {
		return nil, err
	}
	content, err := c.parse_chapter_content_document(document)
	if err != nil {
		_ = c.remove_cached_html(source_url, raw_url)
		return nil, err
	}
	return &TtkResp[*TtkChapterContent]{Data: content, Cached: cached}, nil
}

func (c *Client) parse_chapter_content(html_str string) (*TtkChapterContent, error) {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(html_str))
	if err != nil {
		return nil, err
	}
	return c.parse_chapter_content_document(document)
}

func (c *Client) parse_chapter_content_document(document *goquery.Document) (*TtkChapterContent, error) {
	if document == nil {
		return nil, errors.New("ttk chapter document is nil")
	}
	content := &TtkChapterContent{}
	content.Title = clean_title(first_text(document, []string{
		"h1",
		".chapter-title",
		".title",
		`meta[property="og:title"]`,
		"title",
	}))

	content_selectors := []string{
		".chapter-content",
		".content",
		".novel-content",
		"#content",
		".text-content",
		"article",
		".article-content",
	}
	for _, selector := range content_selectors {
		selection := document.Find(selector).First()
		if selection.Length() == 0 {
			continue
		}
		selection.Find("script, style, noscript, iframe, .ads, .ad, #ads, .banner, .footer, .next, .prev").Remove()
		html_content, _ := selection.Html()
		text := html_fragment_text(html_content)
		if text != "" {
			content.Content = text
			break
		}
	}

	if content.Content == "" {
		return nil, errors.New("ttk chapter content is empty")
	}
	return content, nil
}

func (c *Client) fetch_document(
	fetch_context context.Context,
	source_url string,
	request_url string,
	referer string,
	force_refresh bool,
) (*goquery.Document, bool, error) {
	if c == nil {
		return nil, false, errors.New("ttk client is not initialized")
	}
	if err := check_fetch_interrupted(fetch_context); err != nil {
		return nil, false, err
	}
	if !force_refresh {
		cached_data, cached, err := c.read_cached_html(source_url, request_url)
		if err != nil {
			return nil, false, err
		}
		if cached {
			document, parse_err := goquery.NewDocumentFromReader(bytes.NewReader(cached_data))
			if parse_err != nil {
				_ = c.remove_cached_html(source_url, request_url)
				return nil, false, fmt.Errorf("parse cached ttk html: %w", parse_err)
			}
			return document, true, nil
		}
	}

	html_text, err := c.fetch_html(fetch_context, request_url, referer)
	if err != nil {
		return nil, false, err
	}
	html_data := []byte(html_text)
	if err := c.write_cached_html(source_url, request_url, html_data); err != nil {
		return nil, false, err
	}
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(html_data))
	if err != nil {
		_ = c.remove_cached_html(source_url, request_url)
		return nil, false, err
	}
	return document, false, nil
}

func (c *Client) fetch_html(fetch_context context.Context, request_url string, referer string) (string, error) {
	if c == nil {
		return "", errors.New("ttk client is not initialized")
	}
	if c.claw_client == nil {
		if c.claw_client_err != nil {
			return "", fmt.Errorf("initialize ttk clawreq client: %w", c.claw_client_err)
		}
		return "", errors.New("ttk clawreq client is not initialized")
	}
	request_options := []clawreq.RequestOption{
		clawreq.WithHeaders(map[string]string{
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
			"Cache-Control":   "max-age=0",
			"Sec-Fetch-Site":  "same-origin",
		}),
	}
	cookie_header, err := c.resolve_cookie()
	if err != nil {
		return "", err
	}
	if cookie_header != "" {
		request_options = append(request_options, clawreq.WithCookie(cookie_header))
	}
	if c.user_agent != "" {
		request_options = append(request_options, clawreq.WithHeader("User-Agent", c.user_agent))
	}
	if referer = strings.TrimSpace(referer); referer != "" {
		request_options = append(request_options, clawreq.WithReferer(referer))
	}
	if err := ttk_outbound_request_limiter.wait(fetch_context); err != nil {
		return "", fmt.Errorf("wait for ttk request interval: %w", err)
	}

	response, err := c.claw_client.Get(fetch_context, strings.TrimSpace(request_url), request_options...)
	if err != nil {
		return "", fmt.Errorf("fetch ttk page with clawreq: url=%q: %w", request_url, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("ttk returned HTTP %d", response.StatusCode)
	}
	return response.Text()
}

func (c *Client) resolve_cookie() (string, error) {
	if c == nil {
		return "", errors.New("ttk client is not initialized")
	}
	if cookie_header := strings.TrimSpace(c.cookie); cookie_header != "" {
		return cookie_header, nil
	}
	if c.cookie_provider == nil {
		return "", nil
	}
	cookie_header, err := c.cookie_provider.HeaderForDomain(ttk_cookie_domain)
	if errors.Is(err, cookies.ErrCookieNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s cookies: %w", ttk_cookie_domain, err)
	}
	return strings.TrimSpace(cookie_header), nil
}

func normalize_url(reference string, page_url string) string {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return ""
	}
	parsed_reference, err := url.Parse(reference)
	if err != nil {
		return reference
	}
	parsed_base, err := url.Parse(strings.TrimSpace(page_url))
	if err != nil || parsed_base.Scheme == "" || parsed_base.Host == "" {
		parsed_base, err = url.Parse(ttk_base_url)
		if err != nil {
			return reference
		}
	}
	return parsed_base.ResolveReference(parsed_reference).String()
}

func validate_novel_url(raw_url string) (string, error) {
	parsed_url, err := parse_ttk_url(raw_url)
	if err != nil {
		return "", err
	}
	if strings.Trim(parsed_url.Path, "/") == "" {
		return "", ErrUnsupportedURL
	}
	return book_id_from_url(parsed_url), nil
}

func validate_ttk_url(raw_url string) error {
	_, err := parse_ttk_url(raw_url)
	return err
}

func parse_ttk_url(raw_url string) (*url.URL, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || parsed_url.Scheme == "" || parsed_url.Host == "" {
		return nil, ErrUnsupportedURL
	}
	if parsed_url.Scheme != "http" && parsed_url.Scheme != "https" {
		return nil, ErrUnsupportedURL
	}
	if !is_ttk_host(parsed_url.Hostname()) {
		return nil, ErrUnsupportedURL
	}
	return parsed_url, nil
}

func is_ttk_host(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "ttks.tw" || host == "www.ttks.tw"
}

func book_id_from_url(parsed_url *url.URL) string {
	if parsed_url == nil {
		return ""
	}
	path_parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
	for path_index := len(path_parts) - 1; path_index >= 0; path_index-- {
		if value := safe_identifier(path_parts[path_index]); value != "" {
			return value
		}
	}
	return safe_identifier(parsed_url.Hostname())
}

func chapter_id_from_url(raw_url string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return ""
	}
	return book_id_from_url(parsed_url)
}

func safe_identifier(value string) string {
	var builder strings.Builder
	last_dash := false
	for _, character := range strings.TrimSpace(value) {
		switch {
		case character >= 'a' && character <= 'z':
			builder.WriteRune(character)
			last_dash = false
		case character >= 'A' && character <= 'Z':
			builder.WriteRune(character + ('a' - 'A'))
			last_dash = false
		case character >= '0' && character <= '9':
			builder.WriteRune(character)
			last_dash = false
		case character == '_' || character == '-':
			builder.WriteRune(character)
			last_dash = character == '-'
		default:
			if !last_dash {
				builder.WriteRune('-')
				last_dash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-_")
}

func first_text(document *goquery.Document, selectors []string) string {
	for _, selector := range selectors {
		selection := document.Find(selector).First()
		if selection.Length() == 0 {
			continue
		}
		if strings.HasPrefix(selector, "meta[") {
			if content := strings.TrimSpace(selection.AttrOr("content", "")); content != "" {
				return content
			}
			continue
		}
		if text := strings.TrimSpace(selection.Text()); text != "" {
			return text
		}
	}
	return ""
}

func first_attribute(document *goquery.Document, selectors []string, attribute_name string) string {
	if document == nil {
		return ""
	}
	for _, selector := range selectors {
		selection := document.Find(selector).First()
		if selection.Length() == 0 {
			continue
		}
		if value := strings.TrimSpace(selection.AttrOr(attribute_name, "")); value != "" {
			return value
		}
	}
	return ""
}

func clean_title(value string) string {
	value = html.UnescapeString(strings.TrimSpace(value))
	for _, separator := range []string{" - TT看書", " - TT看书", "_TT看書", "_TT看书", " | TT看書", " | TT看书"} {
		if title, _, found := strings.Cut(value, separator); found {
			value = strings.TrimSpace(title)
		}
	}
	return value
}

func clean_author(value string) string {
	value = html.UnescapeString(strings.TrimSpace(value))
	for _, prefix := range []string{"作者：", "作者:", "作 者：", "作 者:"} {
		value = strings.TrimSpace(strings.TrimPrefix(value, prefix))
	}
	return value
}

func html_fragment_text(value string) string {
	value = normalize_html_breaks(value)
	value = strip_html_tags(value)
	value = html.UnescapeString(value)
	return clean_content_text(value)
}

func normalize_html_breaks(value string) string {
	replacements := map[string]string{
		"<br>":   "\n",
		"<br/>":  "\n",
		"<br />": "\n",
		"<BR>":   "\n",
		"<BR/>":  "\n",
		"<BR />": "\n",
		"</p>":   "\n",
		"</P>":   "\n",
		"</div>": "\n",
		"</DIV>": "\n",
		"&nbsp;": " ",
		"&#160;": " ",
		"\u00a0": " ",
	}
	for old_value, new_value := range replacements {
		value = strings.ReplaceAll(value, old_value, new_value)
	}
	return value
}

func strip_html_tags(value string) string {
	var builder strings.Builder
	in_tag := false
	for _, character := range value {
		if character == '<' {
			in_tag = true
			continue
		}
		if character == '>' {
			in_tag = false
			continue
		}
		if !in_tag {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func clean_content_text(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	cleaned_lines := make([]string, 0, len(lines))
	previous_empty := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(cleaned_lines) > 0 && !previous_empty {
				cleaned_lines = append(cleaned_lines, "")
				previous_empty = true
			}
			continue
		}
		cleaned_lines = append(cleaned_lines, line)
		previous_empty = false
	}
	for len(cleaned_lines) > 0 && cleaned_lines[len(cleaned_lines)-1] == "" {
		cleaned_lines = cleaned_lines[:len(cleaned_lines)-1]
	}
	return strings.Join(cleaned_lines, "\n")
}
