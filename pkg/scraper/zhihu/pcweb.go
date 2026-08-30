package zhihu

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"wx_channel/pkg/minib"
)

const (
	pcweb_hybrid_user_agent  = "ZhihuHybrid com.zhihu.android/Futureve/10.57.0 Mozilla/5.0"
	pcweb_desktop_user_agent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"
	pcweb_vm_timeout         = 120 * time.Second
	pcweb_zse_cookie_cache   = "zse-ck/cookie.txt"
)

var (
	pcweb_script_name_re = regexp.MustCompile(`^[0-9a-f]{32,128}\.js$`)
	pcweb_meta_after_id  = regexp.MustCompile(`(?is)<meta\b[^>]*\bid\s*=\s*["']zh-zse-ck["'][^>]*\bcontent\s*=\s*["']([^"']+)["']`)
	pcweb_meta_before_id = regexp.MustCompile(`(?is)<meta\b[^>]*\bcontent\s*=\s*["']([^"']+)["'][^>]*\bid\s*=\s*["']zh-zse-ck["']`)
	pcweb_script_src_re  = regexp.MustCompile(`(?is)<script\b[^>]*\bsrc\s*=\s*["']([^"']*/zse-ck/v4/[^"']+\.js)["']`)
)

//go:embed pcweb_runtime.js
var pcweb_vm_runtime []byte

//go:embed pcweb_profile.json
var pcweb_vm_profile []byte

type pcweb_response struct {
	status   int
	body     []byte
	location string
}

type pcweb_document_validator func(body []byte, content_id string) bool

type pcweb_article_api_payload struct {
	ID                          any    `json:"id"`
	Title                       string `json:"title"`
	Content                     string `json:"content"`
	ContentNeedTruncated        bool   `json:"content_need_truncated"`
	ContentNeedTruncatedCamel   bool   `json:"contentNeedTruncated"`
	ForceLoginWhenClickReadMore bool   `json:"force_login_when_click_read_more"`
	ForceLoginWhenReadMoreCamel bool   `json:"forceLoginWhenClickReadMore"`
	Excerpt                     string `json:"excerpt"`
	ImageURL                    string `json:"image_url"`
	ImageURLAlt                 string `json:"imageUrl"`
	Author                      User   `json:"author"`
	CreatedTime                 int64  `json:"created"`
	UpdatedTime                 int64  `json:"updated"`
}

// fetch_pcweb_answer_document first requests the original Answer URL through
// Zhihu's anonymous Hybrid SSR entry. It returns the untouched SSR document.
// The retained desktop zse-ck flow is only used if that stable entry stops
// returning usable Answer data.
func (c *Client) fetch_pcweb_answer_document(raw_url string) ([]byte, error) {
	answer_url, ok := ParseAnswerURL(raw_url)
	if !ok {
		return nil, fmt.Errorf("unsupported zhihu answer url")
	}

	direct, direct_err := c.pcweb_hybrid_request(answer_url.Canonical)
	if direct_err == nil && direct.status >= 200 && direct.status < 300 && pcweb_has_answer(direct.body, answer_url.AnswerID) {
		return direct.body, nil
	}

	desktop_body, desktop_err := c.pcweb_desktop_challenge(answer_url.Canonical, answer_url.AnswerID)
	if desktop_err == nil {
		return desktop_body, nil
	}
	if direct_err != nil {
		return nil, fmt.Errorf("zhihu pcweb hybrid request failed: %v; desktop challenge failed: %w", direct_err, desktop_err)
	}
	direct_status := 0
	if direct != nil {
		direct_status = direct.status
	}
	return nil, fmt.Errorf("zhihu pcweb hybrid status %d did not contain Answer %s; desktop challenge failed: %w", direct_status, answer_url.AnswerID, desktop_err)
}

// fetch_pcweb_article_document follows Zhihu's anonymous Article recovery
// chain. The canonical zhuanlan document must be requested first with the same
// navigation headers used by the web client: it currently carries the complete
// Article while AppView may only expose a login-gated truncated copy.
func (c *Client) fetch_pcweb_article_document(raw_url string) ([]byte, error) {
	article_url, ok := ParseArticleURL(raw_url)
	if !ok {
		return nil, fmt.Errorf("unsupported zhihu article url")
	}
	desktop_body, desktop_err := c.pcweb_desktop_document(
		article_url.Canonical,
		article_url.ArticleID,
		"Article",
		pcweb_has_article,
	)
	if desktop_err == nil {
		return desktop_body, nil
	}

	canonical_response, canonical_err := c.pcweb_article_canonical_request(article_url.Canonical)
	if canonical_err == nil && canonical_response != nil &&
		canonical_response.status >= 200 && canonical_response.status < 300 &&
		pcweb_has_article(canonical_response.body, article_url.ArticleID) {
		return canonical_response.body, nil
	}

	appview_url := pcweb_article_appview_url(article_url.ArticleID)
	appview_response, appview_err := c.pcweb_hybrid_request(appview_url)
	if appview_err == nil && appview_response != nil && appview_response.status >= 200 && appview_response.status < 300 && pcweb_has_article(appview_response.body, article_url.ArticleID) {
		return appview_response.body, nil
	}

	appview_retry, appview_retry_err := c.pcweb_hybrid_request(appview_url)
	if appview_retry_err == nil && appview_retry != nil && appview_retry.status >= 200 && appview_retry.status < 300 && pcweb_has_article(appview_retry.body, article_url.ArticleID) {
		return appview_retry.body, nil
	}
	if appview_retry_err == nil {
		appview_retry_status := 0
		if appview_retry != nil {
			appview_retry_status = appview_retry.status
		}
		appview_retry_err = fmt.Errorf("HTTP %d did not contain Article %s", appview_retry_status, article_url.ArticleID)
	}

	api_body, api_err := c.fetch_pcweb_article_api_document(article_url)
	if api_err == nil {
		return api_body, nil
	}

	canonical_status := 0
	if canonical_response != nil {
		canonical_status = canonical_response.status
	}
	appview_status := 0
	if appview_response != nil {
		appview_status = appview_response.status
	}
	return nil, fmt.Errorf(
		"zhihu canonical Article status %d did not contain complete Article %s (request error: %v); AppView status %d did not contain complete Article (request error: %v); desktop challenge failed: %v; AppView retry failed: %v; Article API fallback failed: %w",
		canonical_status,
		article_url.ArticleID,
		canonical_err,
		appview_status,
		appview_err,
		desktop_err,
		appview_retry_err,
		api_err,
	)
}

func (c *Client) pcweb_article_canonical_request(raw_url string) (*pcweb_response, error) {
	req, err := http.NewRequest(http.MethodGet, raw_url, nil)
	if err != nil {
		return nil, err
	}
	set_pcweb_desktop_document_headers(req, "same-origin", raw_url)
	c.log_request(http.MethodGet, raw_url, "")
	resp, err := c.pcweb_http_client(nil, false).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	c.log_response(http.MethodGet, raw_url, resp.StatusCode)
	return read_pcweb_response(resp, 32<<20)
}

func (c *Client) pcweb_hybrid_request(raw_url string) (*pcweb_response, error) {
	req, err := http.NewRequest(http.MethodGet, raw_url, nil)
	if err != nil {
		return nil, err
	}
	set_pcweb_hybrid_headers(req)
	c.log_request(http.MethodGet, raw_url, "")
	client := c.pcweb_http_client(nil, false)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	c.log_response(http.MethodGet, raw_url, resp.StatusCode)
	return read_pcweb_response(resp, 32<<20)
}

func (c *Client) pcweb_desktop_challenge(raw_url, answer_id string) ([]byte, error) {
	return c.pcweb_desktop_document(raw_url, answer_id, "Answer", pcweb_has_answer)
}

func (c *Client) pcweb_desktop_document(raw_url, content_id, content_kind string, document_validator pcweb_document_validator) ([]byte, error) {
	timeout := 120 * time.Second
	if c != nil && c.http_client != nil && c.http_client.Timeout > 0 {
		timeout = c.http_client.Timeout
	}
	browser, err := minib.NewMiniBrowser(timeout)
	if err != nil {
		return nil, fmt.Errorf("create zhihu minibrowser: %w", err)
	}
	defer browser.Close()
	// Public desktop documents must solve the challenge in a clean anonymous
	// session. Persisted account cookies can contain an expired login state;
	// Zhihu then accepts a newly generated __zse_ck but redirects the otherwise
	// public document to /signin. The authenticated API fallback still uses the
	// configured cookie reader when it is needed.
	cached_zse_cookie, err := c.read_pcweb_zse_cookie()
	if err != nil {
		return nil, err
	}
	if cached_zse_cookie != "" {
		if err := set_pcweb_zse_cookie(browser, raw_url, cached_zse_cookie); err != nil {
			return nil, err
		}
		cached_req, request_err := http.NewRequest(http.MethodGet, raw_url, nil)
		if request_err != nil {
			return nil, request_err
		}
		set_pcweb_desktop_document_headers(cached_req, "none", "")
		c.log_request(http.MethodGet, raw_url, "__zse_ck=<cached>")
		cached_resp, request_err := browser.Get(context.Background(), raw_url, cached_req.Header)
		if request_err != nil {
			return nil, fmt.Errorf("validate cached zhihu zse-ck: %w", request_err)
		}
		c.log_response(http.MethodGet, raw_url, cached_resp.StatusCode)
		if cached_resp.StatusCode >= 200 && cached_resp.StatusCode < 300 && document_validator(cached_resp.Body, content_id) {
			return cached_resp.Body, nil
		}
		// A cached token is useful only when it produces a complete document.
		// In particular, do not keep a token whose response is a signin redirect.
		if err := c.remove_pcweb_zse_cookie(); err != nil {
			return nil, err
		}
	}

	challenge_req, err := http.NewRequest(http.MethodGet, raw_url, nil)
	if err != nil {
		return nil, err
	}
	set_pcweb_desktop_document_headers(challenge_req, "none", "")
	challenge_ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	runtime_hooks := new_pcweb_minib_runtime_hooks()
	c.log_request(http.MethodGet, raw_url, "")
	challenge_page, err := browser.Navigate(challenge_ctx, raw_url, challenge_req.Header, minib.NavigateOptions{
		DisableCache:       true,
		DisableCSS:         true,
		DisableImages:      true,
		DisableMedia:       true,
		JavaScriptTimeout:  timeout,
		ResourceTimeout:    timeout,
		WaitUntil:          minib.WaitUntilLoad,
		RuntimeInitializer: runtime_hooks.initialize(raw_url),
		RuntimeFinalizer:   runtime_hooks.finalize,
		RuntimeCleanup:     runtime_hooks.cleanup,
		UseCustomRuntime:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("navigate zhihu pcweb challenge with minib: %w", err)
	}
	c.log_response(http.MethodGet, raw_url, challenge_page.StatusCode)
	if challenge_page.StatusCode >= 200 && challenge_page.StatusCode < 300 && document_validator([]byte(challenge_page.RenderedHTML), content_id) {
		return []byte(challenge_page.RenderedHTML), nil
	}
	meta, script_url, err := parse_pcweb_challenge([]byte(challenge_page.HTML), raw_url)
	if err != nil {
		return nil, fmt.Errorf("parse zhihu pcweb challenge (HTTP %d): %w", challenge_page.StatusCode, err)
	}
	if len(challenge_page.ScriptFailures) != 0 {
		return nil, fmt.Errorf("execute zhihu pcweb challenge script %s: %v", script_url, challenge_page.ScriptFailures[0].Err)
	}
	cookie_value, err := pcweb_zse_cookie_from_browser(browser, raw_url, meta)
	if err != nil {
		return nil, err
	}
	if !valid_pcweb_zse_cookie(cookie_value) || !strings.HasSuffix(cookie_value, "-"+meta) {
		return nil, errors.New("zhihu minib generated an incomplete __zse_ck")
	}
	if err := set_pcweb_zse_cookie(browser, raw_url, cookie_value); err != nil {
		return nil, err
	}

	retry_req, err := http.NewRequest(http.MethodGet, raw_url, nil)
	if err != nil {
		return nil, err
	}
	set_pcweb_desktop_document_headers(retry_req, "same-origin", raw_url)
	c.log_request(http.MethodGet, raw_url, "__zse_ck=<generated>")
	retry_resp, err := browser.Get(context.Background(), raw_url, retry_req.Header)
	if err != nil {
		return nil, fmt.Errorf("retry zhihu pcweb %s: %w", content_kind, err)
	}
	c.log_response(http.MethodGet, raw_url, retry_resp.StatusCode)
	retry := &pcweb_response{status: retry_resp.StatusCode, body: retry_resp.Body, location: retry_resp.Header.Get("Location")}
	if retry.status == http.StatusForbidden {
		return nil, fmt.Errorf("zhihu rejected minib-generated __zse_ck: HTTP %d body=%s", retry.status, debug_snippet(retry.body))
	}
	if retry.status >= 200 && retry.status < 300 && document_validator(retry.body, content_id) {
		if err := c.write_pcweb_zse_cookie(cookie_value); err != nil {
			return nil, err
		}
		return retry.body, nil
	}
	if retry.location != "" {
		return nil, fmt.Errorf("zhihu pcweb retry status %d location=%s", retry.status, retry.location)
	}
	return nil, fmt.Errorf("zhihu pcweb retry status %d body=%s", retry.status, debug_snippet(retry.body))
}

func pcweb_zse_cookie_from_browser(browser *minib.MiniBrowser, raw_url, meta string) (string, error) {
	cookies, err := browser.Cookies(raw_url)
	if err != nil {
		return "", fmt.Errorf("inspect zhihu minib cookies: %w", err)
	}
	for _, cookie := range cookies {
		if cookie.Name == "__zse_ck" && strings.HasSuffix(cookie.Value, "-"+meta) {
			return cookie.Value, nil
		}
	}
	core_value, execute_err := browser.ExecuteJS(context.Background(), `(function () {
  return typeof __g === "object" && typeof __g.ck === "string" ? __g.ck : "";
})()`)
	if execute_err != nil {
		return "", fmt.Errorf("inspect zhihu minib challenge result: %w", execute_err)
	}
	if cookie_core := strings.TrimSpace(core_value.String()); strings.HasPrefix(cookie_core, "005_") {
		return cookie_core + "-" + meta, nil
	}
	value, execute_err := browser.ExecuteJS(context.Background(), `(function () {
  const written = typeof __writtenCookie === "string" ? __writtenCookie : "";
  const match = written.match(/^__zse_ck=([^;]+)/);
  return match ? match[1] : "";
})()`)
	if execute_err != nil {
		return "", fmt.Errorf("inspect zhihu minib challenge cookie: %w", execute_err)
	}
	if cookie_value := strings.TrimSpace(value.String()); cookie_value != "" {
		return cookie_value, nil
	}
	return "", errors.New("zhihu pcweb challenge did not set __zse_ck in the minib cookie jar")
}

func (c *Client) pcweb_http_client(jar http.CookieJar, no_redirect bool) *http.Client {
	timeout := 120 * time.Second
	var transport http.RoundTripper = zhihu_http_transport()
	if c != nil && c.http_client != nil {
		if c.http_client.Timeout > 0 {
			timeout = c.http_client.Timeout
		}
		if c.http_client.Transport != nil {
			transport = c.http_client.Transport
		}
	}
	client := &http.Client{Timeout: timeout, Transport: transport, Jar: jar}
	if no_redirect {
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}
	return client
}

func read_pcweb_response(resp *http.Response, limit int64) (*pcweb_response, error) {
	if resp == nil {
		return nil, errors.New("zhihu pcweb response is nil")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, err
	}
	return &pcweb_response{
		status:   resp.StatusCode,
		body:     body,
		location: resp.Header.Get("Location"),
	}, nil
}

func set_pcweb_hybrid_headers(req *http.Request) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Priority", "u=0, i")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", pcweb_hybrid_user_agent)
}

func set_pcweb_desktop_document_headers(req *http.Request, fetch_site, referer string) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Priority", "u=0, i")
	req.Header.Set("Sec-CH-UA", `"Not=A?Brand";v="99", "Google Chrome";v="151", "Chromium";v="151"`)
	req.Header.Set("Sec-CH-UA-Mobile", "?0")
	req.Header.Set("Sec-CH-UA-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", fetch_site)
	if fetch_site == "none" {
		req.Header.Set("Sec-Fetch-User", "?1")
	}
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", pcweb_desktop_user_agent)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
}

func set_pcweb_script_headers(req *http.Request, referer string) {
	origin := "https://www.zhihu.com"
	if parsed, err := url.Parse(referer); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		origin = parsed.Scheme + "://" + parsed.Host
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Origin", origin)
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Priority", "u=1")
	req.Header.Set("Referer", referer)
	req.Header.Set("Sec-CH-UA", `"Not=A?Brand";v="99", "Google Chrome";v="151", "Chromium";v="151"`)
	req.Header.Set("Sec-CH-UA-Mobile", "?0")
	req.Header.Set("Sec-CH-UA-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "script")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("User-Agent", pcweb_desktop_user_agent)
}

func pcweb_has_answer(body []byte, answer_id string) bool {
	initial_data, err := ParseInitialData(body)
	if err != nil {
		return false
	}
	answer, ok := initial_data.InitialState.Entities.Answers[answer_id]
	return ok && answer.ID != ""
}

func pcweb_has_article(body []byte, article_id string) bool {
	initial_data, err := ParseInitialData(body)
	if err != nil {
		return false
	}
	article, ok := article_from_initial_data(initial_data, article_id)
	return ok && article_has_complete_content(article)
}

func article_has_complete_content(article Article) bool {
	return strings.TrimSpace(article.ID) != "" &&
		strings.TrimSpace(article.Content) != "" &&
		!article.ContentNeedTruncated
}

func pcweb_article_appview_url(article_id string) string {
	return "https://www.zhihu.com/appview/p/" + url.PathEscape(strings.TrimSpace(article_id))
}

func pcweb_article_api_urls(article_id string) []string {
	base_url := "https://www.zhihu.com/api/v4/articles/" + url.PathEscape(strings.TrimSpace(article_id))
	return []string{
		base_url,
		base_url + "?include=content",
		base_url + "?include=content,author",
	}
}

func set_pcweb_article_api_headers(req *http.Request, referer string) {
	set_pcweb_desktop_document_headers(req, "same-origin", referer)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("X-API-Version", "3.0.91")
	req.Header.Set("X-App-Za", "OS=Web")
	req.Header.Set("X-Requested-With", "fetch")
	req.Header.Del("Sec-Fetch-User")
	req.Header.Del("Upgrade-Insecure-Requests")
}

func (c *Client) fetch_pcweb_article_api_document(article_url ArticleURL) ([]byte, error) {
	failures := make([]string, 0, 3)
	for attempt, api_url := range pcweb_article_api_urls(article_url.ArticleID) {
		req, err := http.NewRequest(http.MethodGet, api_url, nil)
		if err != nil {
			return nil, err
		}
		set_pcweb_article_api_headers(req, article_url.Canonical)
		cookie_header := c.cookie(api_url)
		if cookie_header != "" {
			req.Header.Set("Cookie", cookie_header)
		}
		c.log_request(http.MethodGet, api_url, cookie_header)
		resp, request_err := c.pcweb_http_client(nil, false).Do(req)
		if request_err != nil {
			failures = append(failures, fmt.Sprintf("attempt %d: %v", attempt+1, request_err))
			continue
		}
		if resp == nil {
			failures = append(failures, fmt.Sprintf("attempt %d: empty HTTP response", attempt+1))
			continue
		}
		status_code := resp.StatusCode
		response, read_err := read_pcweb_response(resp, 16<<20)
		resp.Body.Close()
		c.log_response(http.MethodGet, api_url, status_code)
		if read_err != nil {
			failures = append(failures, fmt.Sprintf("attempt %d: %v", attempt+1, read_err))
			continue
		}
		if response.status != http.StatusOK {
			failures = append(failures, fmt.Sprintf("attempt %d: HTTP %d body=%s", attempt+1, response.status, debug_snippet(response.body)))
			continue
		}
		article, decode_err := decode_pcweb_article_api(response.body, article_url.ArticleID)
		if decode_err != nil {
			failures = append(failures, fmt.Sprintf("attempt %d: %v", attempt+1, decode_err))
			continue
		}
		document, render_err := render_pcweb_article_document(article_url, article)
		if render_err == nil {
			return document, nil
		}
		failures = append(failures, fmt.Sprintf("attempt %d: %v", attempt+1, render_err))
	}
	return nil, fmt.Errorf("Zhihu Article API failed after %d attempts: %s", len(failures), strings.Join(failures, "; "))
}

func decode_pcweb_article_api(body []byte, article_id string) (Article, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var payload pcweb_article_api_payload
	if err := decoder.Decode(&payload); err != nil {
		return Article{}, fmt.Errorf("decode Zhihu Article API: %w", err)
	}
	payload_id := pcweb_json_id(payload.ID)
	if payload_id != article_id {
		return Article{}, fmt.Errorf("Zhihu Article API returned unexpected article %q", payload_id)
	}
	if strings.TrimSpace(payload.Content) == "" {
		return Article{}, errors.New("Zhihu Article API returned no HTML content")
	}
	content_need_truncated := payload.ContentNeedTruncated || payload.ContentNeedTruncatedCamel
	if content_need_truncated {
		return Article{}, errors.New("Zhihu Article API returned truncated HTML content")
	}
	return Article{
		ID:                          payload_id,
		Title:                       payload.Title,
		Content:                     payload.Content,
		ContentNeedTruncated:        content_need_truncated,
		ForceLoginWhenClickReadMore: payload.ForceLoginWhenClickReadMore || payload.ForceLoginWhenReadMoreCamel,
		Excerpt:                     payload.Excerpt,
		ImageURL:                    payload.ImageURLAlt,
		ImageURLAlt:                 payload.ImageURL,
		Author:                      payload.Author,
		CreatedTime:                 payload.CreatedTime,
		UpdatedTime:                 payload.UpdatedTime,
	}, nil
}

func pcweb_json_id(value any) string {
	switch typed_value := value.(type) {
	case json.Number:
		return typed_value.String()
	case string:
		return typed_value
	default:
		return ""
	}
}

func render_pcweb_article_document(article_url ArticleURL, article Article) ([]byte, error) {
	initial_data := map[string]any{
		"initialState": map[string]any{
			"entities": map[string]any{
				"articles": map[string]Article{article_url.ArticleID: article},
			},
		},
		"subAppName": "zhuanlan",
		"spanName":   "PostIndex",
	}
	initial_data_json, err := json.Marshal(initial_data)
	if err != nil {
		return nil, fmt.Errorf("encode Zhihu Article initial data: %w", err)
	}
	title := strings.TrimSpace(article.Title)
	if title == "" {
		title = "知乎文章 " + article_url.ArticleID
	}
	document := fmt.Sprintf(`<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>%s - 知乎专栏</title><link rel="canonical" href="%s"></head>
<body><main class="PostIndex"><article data-article-id="%s"><h1>%s</h1><div class="RichText ztext PostIndex-richText">%s</div></article></main>
<script id="js-initialData" type="text/json">%s</script></body></html>`,
		html.EscapeString(title),
		html.EscapeString(article_url.Canonical),
		html.EscapeString(article_url.ArticleID),
		html.EscapeString(title),
		article.Content,
		initial_data_json,
	)
	return []byte(document), nil
}

func parse_pcweb_challenge(body []byte, base_url string) (string, string, error) {
	text := string(body)
	var meta_match []string
	for _, expression := range []*regexp.Regexp{pcweb_meta_after_id, pcweb_meta_before_id} {
		if match := expression.FindStringSubmatch(text); len(match) == 2 {
			meta_match = match
			break
		}
	}
	script_match := pcweb_script_src_re.FindStringSubmatch(text)
	if len(meta_match) != 2 || len(script_match) != 2 {
		return "", "", errors.New("response is not a zse-ck challenge page")
	}
	base, err := url.Parse(base_url)
	if err != nil {
		return "", "", err
	}
	reference, err := url.Parse(script_match[1])
	if err != nil {
		return "", "", err
	}
	return meta_match[1], base.ResolveReference(reference).String(), nil
}

func pcweb_script_cache_path(script_url string) (string, error) {
	parsed, err := url.Parse(script_url)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(parsed.Scheme, "https") || !strings.EqualFold(parsed.Hostname(), "static.zhihu.com") {
		return "", fmt.Errorf("unexpected zse-ck script URL %q", script_url)
	}
	name := filepath.Base(parsed.Path)
	if !pcweb_script_name_re.MatchString(name) {
		return "", fmt.Errorf("unexpected zse-ck script name %q", name)
	}
	return filepath.ToSlash(filepath.Join("zse-ck", name)), nil
}

func set_pcweb_zse_cookie(browser *minib.MiniBrowser, raw_url, value string) error {
	return browser.SetCookie(raw_url, &http.Cookie{
		Name:   "__zse_ck",
		Value:  value,
		Path:   "/",
		Domain: ".zhihu.com",
		Secure: true,
	})
}

func valid_pcweb_zse_cookie(value string) bool {
	if value != strings.TrimSpace(value) || len(value) > 4096 || !strings.HasPrefix(value, "005_") {
		return false
	}
	return (&http.Cookie{Name: "__zse_ck", Value: value}).Valid() == nil
}

func (c *Client) read_pcweb_zse_cookie() (string, error) {
	if c == nil {
		return "", nil
	}
	c.pcweb_zse_mutex.RLock()
	memory_value := c.pcweb_zse_cookie
	c.pcweb_zse_mutex.RUnlock()
	if valid_pcweb_zse_cookie(memory_value) {
		return memory_value, nil
	}
	if c.file_cache == nil || !c.file_cache.Enabled() {
		return "", nil
	}
	cached, err := c.file_cache.Read(pcweb_zse_cookie_cache)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read cached zhihu zse-ck cookie: %w", err)
	}
	value := strings.TrimSpace(string(cached))
	if !valid_pcweb_zse_cookie(value) {
		_ = c.file_cache.Remove(pcweb_zse_cookie_cache)
		return "", nil
	}
	c.pcweb_zse_mutex.Lock()
	c.pcweb_zse_cookie = value
	c.pcweb_zse_mutex.Unlock()
	return value, nil
}

func (c *Client) write_pcweb_zse_cookie(value string) error {
	if c == nil {
		return nil
	}
	if !valid_pcweb_zse_cookie(value) {
		return errors.New("refuse to cache invalid zhihu zse-ck cookie")
	}
	c.pcweb_zse_mutex.Lock()
	c.pcweb_zse_cookie = value
	c.pcweb_zse_mutex.Unlock()
	if c.file_cache == nil || !c.file_cache.Enabled() {
		return nil
	}
	if err := c.file_cache.Write(pcweb_zse_cookie_cache, []byte(value)); err != nil {
		return fmt.Errorf("cache zhihu zse-ck cookie: %w", err)
	}
	return nil
}

func (c *Client) remove_pcweb_zse_cookie() error {
	if c == nil {
		return nil
	}
	c.pcweb_zse_mutex.Lock()
	c.pcweb_zse_cookie = ""
	c.pcweb_zse_mutex.Unlock()
	if c.file_cache == nil || !c.file_cache.Enabled() {
		return nil
	}
	if err := c.file_cache.Remove(pcweb_zse_cookie_cache); err != nil {
		return fmt.Errorf("remove cached zhihu zse-ck cookie: %w", err)
	}
	return nil
}

func (c *Client) prepare_pcweb_challenge_script(browser *minib.MiniBrowser, script_url, referer string) ([]byte, error) {
	cache_path, err := pcweb_script_cache_path(script_url)
	if err != nil {
		return nil, err
	}
	if c != nil && c.file_cache != nil && c.file_cache.Enabled() {
		cached, read_err := c.file_cache.Read(cache_path)
		if read_err == nil && valid_pcweb_challenge_script(cached) {
			return cached, nil
		}
		if read_err != nil && !errors.Is(read_err, os.ErrNotExist) {
			return nil, fmt.Errorf("read cached zhihu zse-ck script: %w", read_err)
		}
		if len(cached) > 0 {
			_ = c.file_cache.Remove(cache_path)
		}
	}

	req, err := http.NewRequest(http.MethodGet, script_url, nil)
	if err != nil {
		return nil, err
	}
	set_pcweb_script_headers(req, referer)
	c.log_request(http.MethodGet, script_url, "")
	resp, err := browser.Get(context.Background(), script_url, req.Header)
	if err != nil {
		return nil, fmt.Errorf("download zhihu zse-ck script: %w", err)
	}
	c.log_response(http.MethodGet, script_url, resp.StatusCode)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download zhihu zse-ck script: HTTP %d", resp.StatusCode)
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "javascript") {
		return nil, errors.New("downloaded zhihu zse-ck response is not JavaScript")
	}
	if len(resp.Body) > 4<<20 {
		return nil, errors.New("downloaded zhihu zse-ck script is too large")
	}
	script := resp.Body
	if !valid_pcweb_challenge_script(script) {
		return nil, errors.New("downloaded zhihu zse-ck script is empty or incomplete")
	}
	if c != nil && c.file_cache != nil && c.file_cache.Enabled() {
		if err := c.file_cache.Write(cache_path, script); err != nil {
			return nil, fmt.Errorf("cache zhihu zse-ck script: %w", err)
		}
	}
	return script, nil
}

func valid_pcweb_challenge_script(script []byte) bool {
	return len(script) > 1024 && bytes.Contains(script, []byte("WebAssembly"))
}

func generate_pcweb_zse_cookie(browser *minib.MiniBrowser, script []byte, script_url, meta, target_url string) (string, error) {
	if browser == nil {
		return "", errors.New("zhihu minibrowser is nil")
	}
	return run_pcweb_goja(browser.JavaScriptRuntime(), script, script_url, meta, target_url)
}
