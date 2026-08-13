package zhihu

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/zerolog"

	"wx_channel/pkg/cache"
	"wx_channel/pkg/cookies"
)

const (
	Protocol  = "zhihu"
	SourceURL = "https://www.zhihu.com/"
)

var answer_url_re = regexp.MustCompile(`^/question/([0-9]+|undefined)/answer/([0-9]+)$`)
var question_url_re = regexp.MustCompile(`^/question/([0-9]+)$`)
var article_url_re = regexp.MustCompile(`^/p/([0-9]+)$`)

type Client struct {
	http_client     *http.Client
	cookie_reader   *cookies.Reader
	logger          *zerolog.Logger
	file_cache      *cache.CacheProvider
	browser_fetcher BrowserFetcher
	request_timeout time.Duration
	OnProgress      func(downloaded int64)
}

func (c *Client) Fetch(raw_url string) (any, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return nil, fmt.Errorf("知乎URL不能为空")
	}

	resolved_url := ResolveRealURL(raw_url)
	if article_url, ok := ParseArticleURL(resolved_url); ok {
		return c.FetchArticlePage(article_url.Canonical)
	}
	if question_url, ok := ParseQuestionURL(resolved_url); ok {
		return c.FetchQuestionPage(question_url.Canonical)
	}
	if answer_url, ok := ParseAnswerURL(resolved_url); ok {
		return c.FetchAnswerPage(answer_url.Canonical)
	}
	return nil, fmt.Errorf("不支持的知乎URL: %s", raw_url)
}

func ParseAnswerURL(raw_url string) (AnswerURL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return AnswerURL{}, false
	}
	if !strings.EqualFold(parsed.Hostname(), "www.zhihu.com") {
		return AnswerURL{}, false
	}
	matches := answer_url_re.FindStringSubmatch(parsed.EscapedPath())
	if len(matches) != 3 {
		return AnswerURL{}, false
	}
	question_id := matches[1]
	answer_id := matches[2]
	canonical := canonical_answer_url(question_id, answer_id)
	if question_id == "undefined" {
		question_id = ""
	}
	return AnswerURL{QuestionID: question_id, AnswerID: answer_id, Canonical: canonical}, true
}

func ParseQuestionURL(raw_url string) (QuestionURL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return QuestionURL{}, false
	}
	if !strings.EqualFold(parsed.Hostname(), "www.zhihu.com") {
		return QuestionURL{}, false
	}
	matches := question_url_re.FindStringSubmatch(parsed.EscapedPath())
	if len(matches) != 2 {
		return QuestionURL{}, false
	}
	canonical := "https://www.zhihu.com/question/" + matches[1]
	return QuestionURL{QuestionID: matches[1], Canonical: canonical}, true
}

func ParseArticleURL(raw_url string) (ArticleURL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return ArticleURL{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "zhuanlan.zhihu.com" && host != "www.zhihu.com" {
		return ArticleURL{}, false
	}
	matches := article_url_re.FindStringSubmatch(parsed.EscapedPath())
	if len(matches) != 2 {
		return ArticleURL{}, false
	}
	canonical := "https://zhuanlan.zhihu.com/p/" + matches[1]
	return ArticleURL{ArticleID: matches[1], Canonical: canonical}, true
}

func ResolveRealURL(raw_url string) string {
	if strings.HasPrefix(strings.ToLower(raw_url), Protocol+"://") {
		raw_url = raw_url[len(Protocol+"://"):]
		if !strings.HasPrefix(strings.ToLower(raw_url), "http") {
			raw_url = "https://" + raw_url
		}
	}
	return raw_url
}

func canonical_answer_url(question_id, answer_id string) string {
	question_id = strings.TrimSpace(question_id)
	if question_id == "" {
		question_id = "undefined"
	}
	return "https://www.zhihu.com/question/" + question_id + "/answer/" + strings.TrimSpace(answer_id)
}

// NewClient creates a Zhihu scraper client using application-provided
// capabilities. The cookie reader remains owned by the caller and reads the
// latest persistent cookie data on demand.
func NewClient(cookie_reader *cookies.Reader, logger *zerolog.Logger) *Client {
	c := &Client{
		cookie_reader:   cookie_reader,
		logger:          logger,
		http_client:     &http.Client{Timeout: 120 * time.Second},
		request_timeout: 120 * time.Second,
	}
	return c
}

// SetBrowserFetcher configures the real-browser request capability used for
// Zhihu HTML and API responses protected by ZSE.
func (c *Client) SetBrowserFetcher(browser_fetcher BrowserFetcher) {
	if c == nil {
		return
	}
	c.browser_fetcher = browser_fetcher
}

// SetHTTPTimeout overrides the standard HTTP client timeout. It is primarily
// used by lightweight availability checks so startup status cannot hang for the
// longer content-fetch timeout.
func (c *Client) SetHTTPTimeout(timeout time.Duration) {
	if c == nil || timeout <= 0 {
		return
	}
	if c.http_client == nil {
		c.http_client = &http.Client{}
	}
	c.http_client.Timeout = timeout
	c.request_timeout = timeout
}

// cookie returns the current cookie string from the injected persistent reader.
func (c *Client) cookie() string {
	if c != nil && c.cookie_reader != nil {
		cookie_value, err := c.cookie_reader.HeaderForDomain(zhihu_cookie_domain)
		if err == nil {
			return cookie_value
		}
		if !errors.Is(err, cookies.ErrCookieNotFound) {
			log.Printf("zhihu: failed to read persistent cookies: %v", err)
		}
	}
	return ""
}

func (c *Client) FetchAnswerPage(raw_url string) (*AnswerPage, error) {
	answer_url, ok := ParseAnswerURL(ResolveRealURL(raw_url))
	if !ok {
		return nil, fmt.Errorf("unsupported zhihu answer url")
	}
	body, err := c.do_bytes(http.MethodGet, answer_url.Canonical, answer_url.Canonical)
	if err != nil {
		return nil, err
	}
	page, err := parse_answer_page(body, answer_url)
	if err != nil {
		return nil, err
	}
	page.Source = answer_url.Canonical
	if page.Answer.CommentCount > 0 {
		if comments, err := c.fetch_answer_comments(answer_url); err == nil {
			page.Comments = comments
		}
	}
	return page, nil
}

func (c *Client) FetchQuestionPage(raw_url string) (*QuestionPage, error) {
	question_url, ok := ParseQuestionURL(ResolveRealURL(raw_url))
	if !ok {
		return nil, fmt.Errorf("unsupported zhihu question url")
	}
	body, err := c.do_bytes(http.MethodGet, question_url.Canonical, question_url.Canonical)
	if err != nil {
		return nil, err
	}
	page, err := parse_question_page(body, question_url)
	if err != nil {
		return nil, err
	}
	page.Source = question_url.Canonical
	return page, nil
}

func (c *Client) FetchArticlePage(raw_url string) (*ArticlePage, error) {
	article_url, ok := ParseArticleURL(ResolveRealURL(raw_url))
	if !ok {
		return nil, fmt.Errorf("unsupported zhihu article url")
	}
	body, err := c.do_bytes(http.MethodGet, article_url.Canonical, article_url.Canonical)
	if err != nil {
		return nil, err
	}
	page, err := parse_article_page(body, article_url)
	if err != nil {
		return nil, err
	}
	page.Source = article_url.Canonical
	return page, nil
}

func (c *Client) do_bytes(method, raw_url, referer string) ([]byte, error) {
	if method != http.MethodGet {
		return nil, fmt.Errorf("unsupported zhihu browser method %s", method)
	}
	cached_html, cached, err := c.read_cached_html(raw_url)
	if err != nil {
		return nil, fmt.Errorf("read cached zhihu HTML response for %q: %w", raw_url, err)
	}
	if cached {
		if _, parse_err := ParseInitialData(cached_html); parse_err == nil {
			return cached_html, nil
		}
		_ = c.remove_cached_html(raw_url)
	}
	if c.browser_fetcher == nil {
		return nil, ErrBrowserUnavailable
	}
	var request_context context.Context = context.Background()
	cancel_request := func() {}
	if c.request_timeout > 0 {
		request_context, cancel_request = context.WithTimeout(request_context, c.request_timeout)
	}
	defer cancel_request()

	c.log_request(method, raw_url, c.cookie())
	response, err := c.browser_fetcher.Fetch(request_context, BrowserRequest{
		URL:     raw_url,
		Referer: referer,
		Kind:    "html",
		Headers: map[string]string{
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		},
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, fmt.Errorf("zhihu browser returned an empty response")
	}
	c.log_response(method, raw_url, response.StatusCode)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("zhihu browser status %d body=%s", response.StatusCode, debug_snippet(response.Body))
	}
	html_data := response.Body
	if c.OnProgress != nil {
		c.OnProgress(int64(len(html_data)))
	}
	if _, parse_err := ParseInitialData(html_data); parse_err == nil {
		if err := c.write_cached_html(raw_url, html_data); err != nil {
			return nil, fmt.Errorf("cache zhihu HTML response for %q: %w", raw_url, err)
		}
	}
	return html_data, nil
}

func debug_snippet(body []byte) string {
	if len(body) <= 256 {
		return string(body)
	}
	return string(body[:256])
}

func (c *Client) inline_remote_images(content string, referer string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return "", err
	}
	var first_err error
	doc.Find("img[src]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		src, _ := s.Attr("src")
		src = normalize_asset_url(src, referer)
		if src == "" || strings.HasPrefix(src, "data:") {
			return true
		}
		data_uri, err := c.fetch_image_data_uri(src, referer)
		if err != nil {
			if first_err == nil {
				first_err = err
			}
			return true
		}
		s.SetAttr("src", data_uri)
		return true
	})
	if first_err != nil {
		return "", first_err
	}
	out, err := doc.Html()
	if err != nil {
		return "", err
	}
	return "<!doctype html>" + out, nil
}

func (c *Client) InlineRemoteImages(content string, referer string) (string, error) {
	return c.inline_remote_images(content, referer)
}

func (c *Client) LocalizeRemoteVideos(ctx context.Context, content string, referer string, html_path string) (string, error) {
	if strings.TrimSpace(content) == "" || strings.TrimSpace(html_path) == "" {
		return content, nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return "", err
	}
	assets_dir_name := html_assets_dir_name(html_path)
	assets_dir_path := filepath.Join(filepath.Dir(html_path), assets_dir_name)
	downloaded := make(map[string]string)
	var first_err error
	video_index := 0
	doc.Find("video[src], video source[src]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		src, _ := s.Attr("src")
		src = normalize_asset_url(src, referer)
		if src == "" || strings.HasPrefix(src, "data:") || !strings.HasPrefix(src, "http") {
			return true
		}
		local_path, ok := downloaded[src]
		if !ok {
			video_index++
			filename, err := c.download_video(ctx, src, referer, assets_dir_path, video_index)
			if err != nil {
				first_err = err
				return false
			}
			local_path = filepath.ToSlash(filepath.Join(assets_dir_name, filename))
			downloaded[src] = local_path
		}
		s.SetAttr("src", local_path)
		if s.Is("video") {
			ensure_playable_video(s)
		} else {
			s.ParentFiltered("video").Each(func(_ int, video *goquery.Selection) {
				ensure_playable_video(video)
			})
		}
		return true
	})
	if first_err != nil {
		return "", first_err
	}
	out, err := doc.Html()
	if err != nil {
		return "", err
	}
	return "<!doctype html>" + out, nil
}

func ensure_playable_video(s *goquery.Selection) {
	s.SetAttr("controls", "controls")
	if _, ok := s.Attr("preload"); !ok {
		s.SetAttr("preload", "metadata")
	}
	if _, ok := s.Attr("style"); !ok {
		s.SetAttr("style", "max-width:100%;height:auto")
	}
}

func html_assets_dir_name(html_path string) string {
	base := strings.TrimSuffix(filepath.Base(html_path), filepath.Ext(html_path))
	base = strings.TrimSpace(base)
	if base == "" || base == "." {
		base = "zhihu"
	}
	return base + "_files"
}

func (c *Client) download_video(ctx context.Context, raw_url string, referer string, dir string, index int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw_url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "video/mp4,video/webm,video/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36")
	req.Header.Set("Sec-Fetch-Dest", "video")
	req.Header.Set("Sec-Fetch-Mode", "no-cors")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	cookie_header := c.cookie()
	if cookie_header != "" {
		req.Header.Set("Cookie", cookie_header)
	}

	c.log_request(http.MethodGet, raw_url, cookie_header)
	resp, err := c.http_client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	c.log_response(http.MethodGet, raw_url, resp.StatusCode)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("zhihu video status %d body=%s", resp.StatusCode, debug_snippet(body))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	filename := fmt.Sprintf("video_%02d%s", index, video_ext(raw_url, resp.Header.Get("content-type")))
	dest_path := filepath.Join(dir, filename)
	file, err := os.Create(dest_path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := copy_with_client_progress(file, resp.Body, c.OnProgress); err != nil {
		return "", err
	}
	return filename, nil
}

func copy_with_client_progress(dst io.Writer, src io.Reader, on_progress func(int64)) (int64, error) {
	buf := make([]byte, 64*1024)
	var written int64
	for {
		n, read_err := src.Read(buf)
		if n > 0 {
			m, write_err := dst.Write(buf[:n])
			written += int64(m)
			if on_progress != nil && m > 0 {
				on_progress(int64(m))
			}
			if write_err != nil {
				return written, write_err
			}
			if m != n {
				return written, io.ErrShortWrite
			}
		}
		if read_err != nil {
			if read_err == io.EOF {
				return written, nil
			}
			return written, read_err
		}
	}
}

func video_ext(raw_url string, content_type string) string {
	if ext := strings.ToLower(path_ext(raw_url)); valid_media_ext(ext) {
		return ext
	}
	if idx := strings.Index(content_type, ";"); idx >= 0 {
		content_type = strings.TrimSpace(content_type[:idx])
	}
	if exts, err := mime.ExtensionsByType(strings.TrimSpace(content_type)); err == nil {
		for _, ext := range exts {
			if valid_media_ext(ext) {
				return ext
			}
		}
	}
	return ".mp4"
}

func valid_media_ext(ext string) bool {
	switch ext {
	case ".mp4", ".m4v", ".mov", ".webm", ".mkv":
		return true
	default:
		return false
	}
}

func (c *Client) fetch_image_data_uri(raw_url string, referer string) (string, error) {
	body, content_type, err := c.do_image_bytes(raw_url, referer)
	if err != nil {
		return "", err
	}
	if content_type == "" {
		content_type = http.DetectContentType(body)
	}
	if idx := strings.Index(content_type, ";"); idx >= 0 {
		content_type = strings.TrimSpace(content_type[:idx])
	}
	if content_type == "" || content_type == "application/octet-stream" {
		if ext := strings.ToLower(path_ext(raw_url)); ext != "" {
			content_type = mime.TypeByExtension(ext)
		}
	}
	if content_type == "" {
		content_type = "application/octet-stream"
	}
	return "data:" + content_type + ";base64," + base64.StdEncoding.EncodeToString(body), nil
}

func (c *Client) do_image_bytes(raw_url string, referer string) ([]byte, string, error) {
	return c.do_image_bytes_with_http(raw_url, referer)
}

func (c *Client) do_image_bytes_with_http(raw_url string, referer string) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, raw_url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	cookie_header := c.cookie()
	if cookie_header != "" {
		req.Header.Set("Cookie", cookie_header)
	}

	c.log_request(http.MethodGet, raw_url, cookie_header)
	resp, err := c.http_client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	c.log_response(http.MethodGet, raw_url, resp.StatusCode)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, "", fmt.Errorf("zhihu image status %d body=%s", resp.StatusCode, debug_snippet(body))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err == nil && c.OnProgress != nil {
		c.OnProgress(int64(len(body)))
	}
	return body, resp.Header.Get("content-type"), err
}

func parse_answer_page(body []byte, answer_url AnswerURL) (*AnswerPage, error) {
	initial_data, err := ParseInitialData(body)
	if err != nil {
		return nil, err
	}
	answer := initial_data.InitialState.Entities.Answers[answer_url.AnswerID]
	if answer.ID == "" {
		return nil, fmt.Errorf("missing zhihu answer entity")
	}
	question := initial_data.InitialState.Entities.Questions[answer_url.QuestionID]
	if question.ID == "" && answer.Question.ID != "" {
		question = initial_data.InitialState.Entities.Questions[answer.Question.ID]
	}
	if question.ID == "" && len(initial_data.InitialState.Entities.Questions) == 1 {
		for _, candidate := range initial_data.InitialState.Entities.Questions {
			question = candidate
		}
	}
	if question.ID == "" {
		return nil, fmt.Errorf("missing zhihu question entity")
	}
	page_url := answer_url
	if page_url.QuestionID != question.ID {
		page_url.QuestionID = question.ID
		page_url.Canonical = canonical_answer_url(question.ID, answer.ID)
	}
	if page_url.Canonical == "" {
		page_url.Canonical = canonical_answer_url(page_url.QuestionID, page_url.AnswerID)
	}
	return &AnswerPage{
		URL:             page_url,
		Source:          page_url.Canonical,
		PageHTML:        string(body),
		Question:        question,
		Answer:          answer,
		InitialData:     initial_data,
		InitialDataJSON: initial_data.Raw,
	}, nil
}

func parse_question_page(body []byte, question_url QuestionURL) (*QuestionPage, error) {
	initial_data, err := ParseInitialData(body)
	if err != nil {
		return nil, err
	}
	question := initial_data.InitialState.Entities.Questions[question_url.QuestionID]
	if question.ID == "" {
		return nil, fmt.Errorf("missing zhihu question entity")
	}
	return &QuestionPage{
		URL:             question_url,
		Source:          question_url.Canonical,
		PageHTML:        string(body),
		Question:        question,
		InitialData:     initial_data,
		InitialDataJSON: initial_data.Raw,
	}, nil
}

func parse_article_page(body []byte, article_url ArticleURL) (*ArticlePage, error) {
	initial_data, err := ParseInitialData(body)
	if err != nil {
		return nil, err
	}
	article := initial_data.InitialState.Entities.Articles[article_url.ArticleID]
	if article.ID == "" {
		article = initial_data.InitialState.Entities.Posts[article_url.ArticleID]
	}
	if article.ID == "" {
		return nil, fmt.Errorf("missing zhihu article entity")
	}
	return &ArticlePage{
		URL:             article_url,
		Source:          article_url.Canonical,
		PageHTML:        string(body),
		Article:         article,
		InitialData:     initial_data,
		InitialDataJSON: initial_data.Raw,
	}, nil
}

func (c *Client) fetch_answer_comments(answer_url AnswerURL) ([]Comment, error) {
	comments, err := c.fetch_answer_root_comments(answer_url)
	if err == nil {
		return comments, nil
	}
	return c.fetch_answer_comments_v5(answer_url)
}

func (c *Client) fetch_answer_root_comments(answer_url AnswerURL) ([]Comment, error) {
	endpoint := fmt.Sprintf("/api/v4/answers/%s/root_comments?limit=20&offset=0&order=normal&status=open", url.PathEscape(answer_url.AnswerID))
	var comments []Comment
	for endpoint != "" {
		body, err := c.do_api_bytes(endpoint, answer_url.Canonical)
		if err != nil {
			return nil, err
		}
		var response comment_response
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, err
		}
		for _, item := range response.Data {
			comments = append(comments, item.to_comment())
		}
		if response.Paging.IsEnd || response.Paging.Next == "" || len(comments) >= 200 {
			break
		}
		endpoint = endpoint_from_url(response.Paging.Next)
	}
	return comments, nil
}

func (c *Client) fetch_answer_comments_v5(answer_url AnswerURL) ([]Comment, error) {
	endpoint := fmt.Sprintf("/api/v4/comment_v5/answers/%s/root_comment?order_by=score&limit=20&offset=0&status=open", url.PathEscape(answer_url.AnswerID))
	body, err := c.do_api_bytes(endpoint, answer_url.Canonical)
	if err != nil {
		return nil, err
	}
	var response comment_response
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	comments := make([]Comment, 0, len(response.Data))
	for _, item := range response.Data {
		comments = append(comments, item.to_comment())
	}
	return comments, nil
}

func (c *Client) do_api_bytes(endpoint, referer string) ([]byte, error) {
	if !strings.HasPrefix(endpoint, "/") {
		return nil, fmt.Errorf("invalid zhihu api endpoint")
	}
	if c.browser_fetcher == nil {
		return nil, ErrBrowserUnavailable
	}
	api_url := SourceURL + strings.TrimPrefix(endpoint, "/")
	cookie_header := c.cookie()
	headers := map[string]string{
		"Accept":           "*/*",
		"Cache-Control":    "no-cache",
		"Pragma":           "no-cache",
		"X-Requested-With": "fetch",
	}
	// Append x-zse signed headers when d_c0 cookie is available.
	if cookie_header != "" {
		if dc0 := strings.Trim(get_cookie_value(cookie_header, "d_c0"), `"`); dc0 != "" {
			for k, v := range build_signed_header(endpoint, dc0) {
				headers[k] = v
			}
		}
	}
	var request_context context.Context = context.Background()
	cancel_request := func() {}
	if c.request_timeout > 0 {
		request_context, cancel_request = context.WithTimeout(request_context, c.request_timeout)
	}
	defer cancel_request()
	c.log_request(http.MethodGet, api_url, cookie_header)
	response, err := c.browser_fetcher.Fetch(request_context, BrowserRequest{
		URL:     api_url,
		Referer: referer,
		Kind:    "fetch",
		Headers: headers,
	})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, fmt.Errorf("zhihu browser returned an empty API response")
	}
	c.log_response(http.MethodGet, api_url, response.StatusCode)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("zhihu api status %d body=%s", response.StatusCode, debug_snippet(response.Body))
	}
	if c.OnProgress != nil {
		c.OnProgress(int64(len(response.Body)))
	}
	return response.Body, nil
}

func endpoint_from_url(raw_url string) string {
	parsed, err := url.Parse(raw_url)
	if err != nil {
		return ""
	}
	if parsed.Host != "" && !strings.EqualFold(parsed.Hostname(), "www.zhihu.com") {
		return ""
	}
	if parsed.RawQuery == "" {
		return parsed.EscapedPath()
	}
	return parsed.EscapedPath() + "?" + parsed.RawQuery
}

func (p comment_payload) to_comment() Comment {
	created := p.Created
	if created == 0 {
		created = p.CreatedAt
	}
	content := first_non_empty(p.Content, p.ContentTag)
	comment := Comment{
		ID:          raw_id_string(p.ID),
		ContentHTML: content,
		ContentText: html_to_text(content),
		CreatedTime: created,
		Author:      p.Author,
		ReplyTo:     p.ReplyTo,
	}
	for _, child := range p.Child {
		comment.Replies = append(comment.Replies, child.to_comment())
	}
	return comment
}

func raw_id_string(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return number.String()
	}
	return strings.Trim(string(raw), `"`)
}

func best_zhihu_image_src(s *goquery.Selection) string {
	for _, attr := range []string{"data-original", "data-actualsrc", "data-default-watermark-src", "src"} {
		value, ok := s.Attr(attr)
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" || is_placeholder_image(value) {
			continue
		}
		return value
	}
	return ""
}

func FirstImageURL(fragment string, base string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div id=\"wx-zhihu-root\">" + fragment + "</div>"))
	if err != nil {
		return ""
	}
	var image_url string
	doc.Find("#wx-zhihu-root img").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		src := normalize_asset_url(best_zhihu_image_src(s), base)
		if src == "" || strings.HasPrefix(src, "data:") {
			return true
		}
		image_url = src
		return false
	})
	return image_url
}

func is_placeholder_image(raw_url string) bool {
	lower := strings.ToLower(raw_url)
	return strings.Contains(lower, "data:image/svg") ||
		strings.Contains(lower, "placeholder") ||
		strings.Contains(lower, "loading") ||
		strings.Contains(lower, "blank")
}

func html_to_text(fragment string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(fragment))
	if err != nil {
		return strings.TrimSpace(fragment)
	}
	return strings.TrimSpace(doc.Text())
}

func display_name(user User) string {
	return first_non_empty(user.Name, user.URLToken, user.URLTokenSnake, user.ID, "匿名用户")
}

func avatar_url(user User) string {
	return first_non_empty(user.AvatarURL, user.AvatarURLSnake, user.AvatarURLTemplate)
}

func UserDisplayName(user User) string {
	return display_name(user)
}

func UserAvatarURL(user User) string {
	return avatar_url(user)
}

func UserURL(user User) string {
	return author_url(user)
}

func author_url(user User) string {
	token := first_non_empty(user.URLToken, user.URLTokenSnake)
	if token != "" {
		return "https://www.zhihu.com/people/" + url.PathEscape(token)
	}
	if strings.HasPrefix(user.URL, "https://www.zhihu.com/people/") {
		return user.URL
	}
	return ""
}

func normalize_asset_url(raw_url string, base string) string {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" || strings.HasPrefix(raw_url, "data:") {
		return raw_url
	}
	if strings.HasPrefix(raw_url, "//") {
		return "https:" + raw_url
	}
	parsed, err := url.Parse(raw_url)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() {
		if parsed.Scheme == "http" || parsed.Scheme == "https" {
			return parsed.String()
		}
		return ""
	}
	if base == "" {
		return ""
	}
	base_url, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return base_url.ResolveReference(parsed).String()
}

func path_ext(raw_url string) string {
	parsed, err := url.Parse(raw_url)
	if err != nil {
		return ""
	}
	path := parsed.EscapedPath()
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		return path[idx:]
	}
	return ""
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// Querying the request host includes cookies scoped to both www.zhihu.com and
// its parent domain, .zhihu.com.
const zhihu_cookie_domain = "www.zhihu.com"

func (c *Client) log_request(method string, raw_url string, cookie_header string) {
	if c == nil || c.logger == nil {
		return
	}
	cookie_count, cookie_names := summarize_cookie_header(cookie_header)
	c.logger.Info().
		Str("component", "zhihu_scraper").
		Str("method", method).
		Str("url", raw_url).
		Bool("cookie_present", cookie_count > 0).
		Int("cookie_count", cookie_count).
		Strs("cookie_names", cookie_names).
		Msg("zhihu outbound request")
}

func (c *Client) log_response(method string, raw_url string, status_code int) {
	if c == nil || c.logger == nil {
		return
	}
	c.logger.Info().
		Str("component", "zhihu_scraper").
		Str("method", method).
		Str("url", raw_url).
		Int("status_code", status_code).
		Msg("zhihu outbound response")
}

func summarize_cookie_header(cookie_header string) (int, []string) {
	cookie_names := make(map[string]struct{})
	cookie_count := 0
	for _, cookie_part := range strings.Split(cookie_header, ";") {
		cookie_part = strings.TrimSpace(cookie_part)
		if cookie_part == "" {
			continue
		}
		cookie_name, _, has_value := strings.Cut(cookie_part, "=")
		cookie_name = strings.TrimSpace(cookie_name)
		if !has_value || cookie_name == "" {
			continue
		}
		cookie_count++
		cookie_names[cookie_name] = struct{}{}
	}
	unique_names := make([]string, 0, len(cookie_names))
	for cookie_name := range cookie_names {
		unique_names = append(unique_names, cookie_name)
	}
	sort.Strings(unique_names)
	return cookie_count, unique_names
}
