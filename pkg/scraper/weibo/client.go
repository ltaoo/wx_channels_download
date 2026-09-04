// Package weibo renders Weibo pages.
package weibo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"wx_channel/pkg/cookies"
	"wx_channel/pkg/minib"
)

const (
	default_timeout     = 90 * time.Second
	image_proxy_timeout = 30 * time.Second
	desktop_user_agent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"
	detail_title_marker = "<title>微博正文 - 微博</title>"
)

// Client fetches the post-JavaScript HTML of Weibo pages.
type Client struct {
	cookie_provider *cookies.Reader
	proxy_client    *http.Client
}

// IsDetailURL reports whether raw_url is a supported Weibo post URL.
func IsDetailURL(raw_url string) bool {
	_, _, title_marker, err := parse_request_url(raw_url)
	return err == nil && title_marker == detail_title_marker
}

// NewClient creates a Weibo client. A cookie provider is optional, but logged-in
// cookies make the rendered page match an authenticated browser session.
func NewClient(cookie_providers ...*cookies.Reader) *Client {
	var cookie_provider *cookies.Reader
	if len(cookie_providers) > 0 {
		cookie_provider = cookie_providers[0]
	}
	return &Client{
		cookie_provider: cookie_provider,
		proxy_client:    new_image_proxy_client(),
	}
}

// ProxyImg fetches a Weibo image with the headers required by Sina's CDN.
func (c *Client) ProxyImg(raw_url string) (*http.Response, error) {
	return c.ProxyImgContext(context.Background(), raw_url)
}

// ProxyImgContext fetches a Weibo image with cancellation support.
func (c *Client) ProxyImgContext(proxy_context context.Context, raw_url string) (*http.Response, error) {
	if c == nil {
		return nil, errors.New("weibo client is not initialized")
	}
	image_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || image_url == nil {
		return nil, fmt.Errorf("weibo: invalid image URL %q", raw_url)
	}
	if err := validate_image_url(image_url); err != nil {
		return nil, err
	}
	if proxy_context == nil {
		proxy_context = context.Background()
	}
	request, err := http.NewRequestWithContext(proxy_context, http.MethodGet, image_url.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("weibo: create image request: %w", err)
	}
	request.Header.Set("Accept", "image/avif,image/webp,image/apng,image/*,*/*;q=0.8")
	request.Header.Set("Referer", "https://weibo.com/")
	request.Header.Set("User-Agent", desktop_user_agent)
	if c.cookie_provider != nil {
		if cookie_header, cookie_err := c.cookie_provider.HeaderForURL(image_url.String()); cookie_err == nil {
			request.Header.Set("Cookie", cookie_header)
		}
	}
	proxy_client := c.proxy_client
	if proxy_client == nil {
		proxy_client = new_image_proxy_client()
	}
	response, err := proxy_client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("weibo: fetch image: %w", err)
	}
	return response, nil
}

func new_image_proxy_client() *http.Client {
	return &http.Client{
		Timeout: image_proxy_timeout,
		CheckRedirect: func(request *http.Request, _ []*http.Request) error {
			return validate_image_url(request.URL)
		},
	}
}

func validate_image_url(image_url *url.URL) error {
	if image_url == nil || !strings.EqualFold(image_url.Scheme, "https") || image_url.User != nil {
		return errors.New("weibo: unsupported image URL")
	}
	host := strings.ToLower(strings.TrimSuffix(image_url.Hostname(), "."))
	if !allowed_image_host(host, "sinaimg.cn") && !allowed_image_host(host, "sinaimg.com") {
		return fmt.Errorf("weibo: unsupported image host %q", host)
	}
	if image_url.Port() != "" && image_url.Port() != "443" {
		return errors.New("weibo: unsupported image port")
	}
	return nil
}

func allowed_image_host(host string, root string) bool {
	return host == root || strings.HasSuffix(host, "."+root)
}

// Fetch retrieves the rendered HTML for a supported Weibo URL.
func (c *Client) Fetch(raw_url string) (string, error) {
	return c.FetchContext(context.Background(), raw_url)
}

// FetchContext retrieves the rendered HTML for a supported Weibo URL with
// cancellation support.
func (c *Client) FetchContext(fetch_context context.Context, raw_url string) (string, error) {
	if c == nil {
		return "", errors.New("weibo client is not initialized")
	}
	request_url, resource_id, title_marker, err := parse_request_url(raw_url)
	if err != nil {
		return "", err
	}
	if fetch_context == nil {
		fetch_context = context.Background()
	}

	browser, err := minib.NewMiniBrowser(default_timeout, c.cookie_provider)
	if err != nil {
		return "", fmt.Errorf("weibo: create browser: %w", err)
	}
	defer browser.Close()

	navigate_options := minib.NavigateOptions{
		DisableImages:   true,
		DisableMedia:    true,
		ResourceTimeout: 30 * time.Second,
	}
	page, err := browser.Navigate(fetch_context, request_url.String(), navigation_headers(), navigate_options)
	if err != nil {
		return "", fmt.Errorf("weibo: render %s: %w", resource_id, err)
	}
	if is_visitor_page(page.URL) {
		if err := bootstrap_visitor(fetch_context, browser, page.HTML, page.URL, request_url.String()); err != nil {
			return "", err
		}
		page, err = browser.Navigate(fetch_context, request_url.String(), navigation_headers(), navigate_options)
		if err != nil {
			return "", fmt.Errorf("weibo: render %s after visitor bootstrap: %w", resource_id, err)
		}
	}
	if page.StatusCode < http.StatusOK || page.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("weibo: page %s returned HTTP %d", resource_id, page.StatusCode)
	}
	html_text := page.RenderedHTML
	if strings.TrimSpace(html_text) == "" || !strings.Contains(html_text, resource_id) || !strings.Contains(html_text, title_marker) {
		return "", fmt.Errorf("weibo: rendered page %s is incomplete", resource_id)
	}
	return html_text, nil
}

type visitor_response struct {
	Retcode int `json:"retcode"`
	Data    struct {
		Sub  string `json:"sub"`
		Subp string `json:"subp"`
	} `json:"data"`
}

func is_visitor_page(raw_url string) bool {
	page_url, err := url.Parse(raw_url)
	return err == nil && strings.EqualFold(page_url.Hostname(), "passport.weibo.com") && page_url.Path == "/visitor/visitor"
}

func bootstrap_visitor(fetch_context context.Context, browser *minib.MiniBrowser, page_html, page_url, return_url string) error {
	request_id_match := regexp.MustCompile(`var request_id = "([^"]+)"`).FindStringSubmatch(page_html)
	version_match := regexp.MustCompile(`ver=([0-9]+)&request_id=`).FindStringSubmatch(page_html)
	if len(request_id_match) != 2 || len(version_match) != 2 {
		return errors.New("weibo: visitor page is missing bootstrap parameters")
	}
	parsed_page_url, err := url.Parse(page_url)
	if err != nil {
		return fmt.Errorf("weibo: parse visitor URL: %w", err)
	}
	values := url.Values{
		"cb":         {"visitor_gray_callback"},
		"ver":        {version_match[1]},
		"request_id": {request_id_match[1]},
		"tid":        {""},
		"from":       {"weibo"},
		"webdriver":  {"false"},
		"rid":        {fmt.Sprint(time.Now().UnixMilli())},
		"return_url": {return_url},
	}
	visitor_url := parsed_page_url.Scheme + "://" + parsed_page_url.Host + "/visitor/genvisitor2"
	response, err := browser.Request(fetch_context, http.MethodPost, visitor_url, strings.NewReader(values.Encode()), http.Header{
		"Content-Type": {"application/x-www-form-urlencoded"},
		"Origin":       {parsed_page_url.Scheme + "://" + parsed_page_url.Host},
		"Referer":      {page_url},
	})
	if err != nil {
		return fmt.Errorf("weibo: create visitor session: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("weibo: visitor bootstrap returned HTTP %d", response.StatusCode)
	}
	visitor_data, err := parse_visitor_response(response.Body)
	if err != nil {
		return err
	}
	for cookie_name, cookie_value := range map[string]string{"SUB": visitor_data.Data.Sub, "SUBP": visitor_data.Data.Subp} {
		if err := browser.SetCookie(return_url, &http.Cookie{Name: cookie_name, Value: cookie_value, Domain: ".weibo.com", Path: "/", Secure: true}); err != nil {
			return fmt.Errorf("weibo: store visitor cookie %s: %w", cookie_name, err)
		}
	}
	return nil
}

func parse_visitor_response(body []byte) (*visitor_response, error) {
	json_start := bytes.IndexByte(body, '{')
	json_end := bytes.LastIndexByte(body, '}')
	if json_start < 0 || json_end < json_start {
		return nil, errors.New("weibo: invalid visitor bootstrap response")
	}
	var response visitor_response
	if err := json.Unmarshal(body[json_start:json_end+1], &response); err != nil {
		return nil, fmt.Errorf("weibo: decode visitor bootstrap response: %w", err)
	}
	if response.Retcode != 20000000 || response.Data.Sub == "" || response.Data.Subp == "" {
		return nil, fmt.Errorf("weibo: visitor bootstrap failed with retcode %d", response.Retcode)
	}
	return &response, nil
}

func parse_request_url(raw_url string) (*url.URL, string, string, error) {
	request_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || request_url == nil {
		return nil, "", "", fmt.Errorf("weibo: invalid URL %q", raw_url)
	}
	if !strings.EqualFold(request_url.Scheme, "https") || !strings.EqualFold(request_url.Hostname(), "weibo.com") {
		return nil, "", "", fmt.Errorf("weibo: unsupported URL %q", raw_url)
	}
	path_parts := strings.Split(strings.Trim(request_url.EscapedPath(), "/"), "/")
	if len(path_parts) != 2 {
		return nil, "", "", fmt.Errorf("weibo: unsupported URL %q", raw_url)
	}
	resource_id, title_marker := "", ""
	switch {
	case path_parts[0] == "u" && is_decimal(path_parts[1]):
		resource_id, title_marker = path_parts[1], "的个人主页</title>"
	case is_decimal(path_parts[0]) && is_base62(path_parts[1]):
		resource_id, title_marker = path_parts[1], detail_title_marker
	default:
		return nil, "", "", fmt.Errorf("weibo: unsupported URL %q", raw_url)
	}
	request_url.RawQuery = ""
	request_url.Fragment = ""
	return request_url, resource_id, title_marker, nil
}

func is_decimal(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func is_base62(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z')) {
			return false
		}
	}
	return true
}

func navigation_headers() http.Header {
	return http.Header{
		"Accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
		"Accept-Language":           {"zh-CN,zh;q=0.9,en;q=0.8"},
		"Cache-Control":             {"no-cache"},
		"Pragma":                    {"no-cache"},
		"Priority":                  {"u=0, i"},
		"Referer":                   {"https://s.weibo.com/"},
		"Sec-Ch-Ua":                 {`"Not=A?Brand";v="99", "Google Chrome";v="151", "Chromium";v="151"`},
		"Sec-Ch-Ua-Mobile":          {"?0"},
		"Sec-Ch-Ua-Platform":        {`"macOS"`},
		"Sec-Fetch-Dest":            {"document"},
		"Sec-Fetch-Mode":            {"navigate"},
		"Sec-Fetch-Site":            {"same-origin"},
		"Sec-Fetch-User":            {"?1"},
		"Upgrade-Insecure-Requests": {"1"},
		"User-Agent":                {desktop_user_agent},
	}
}
