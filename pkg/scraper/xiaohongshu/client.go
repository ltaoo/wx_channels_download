// Package xiaohongshu fetches Xiaohongshu note pages.
package xiaohongshu

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
)

const (
	default_request_timeout = 30 * time.Second
	default_response_limit  = int64(8 << 20)
	max_redirect_count      = 10
	default_user_agent      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"
)

var http_url_pattern = regexp.MustCompile(`https?://[a-zA-Z0-9._~:/?#\[\]@!$&'()*+,;=%-]+`)

const trailing_url_punctuation = `.,!?;:)]}"'`

// ExtractURL extracts the first Xiaohongshu URL from a URL or copied share text.
func ExtractURL(content string) (string, error) {
	for _, candidate_url := range http_url_pattern.FindAllString(strings.TrimSpace(content), -1) {
		candidate_url = strings.TrimRight(candidate_url, trailing_url_punctuation)
		parsed_url, err := url.Parse(candidate_url)
		if err == nil && validate_request_url(parsed_url) == nil {
			return candidate_url, nil
		}
	}
	return "", errors.New("xiaohongshu URL not found")
}

// Client fetches the server-rendered HTML of Xiaohongshu note pages. It accepts
// both xhslink.cn share links and direct xiaohongshu.com links.
type Client struct {
	http_client    *http.Client
	response_limit int64
}

// NewClient creates a Xiaohongshu client with a 30-second request timeout.
func NewClient() *Client {
	return &Client{
		http_client: &http.Client{
			Timeout:       default_request_timeout,
			CheckRedirect: stop_automatic_redirect,
		},
		response_limit: default_response_limit,
	}
}

// NewClientWithHTTPClient creates a Xiaohongshu client using http_client.
// The supplied transport, cookie jar, and timeout are retained. Redirects are
// handled explicitly so the empty xhslink.cn response is never returned as the
// note HTML.
func NewClientWithHTTPClient(http_client *http.Client) *Client {
	if http_client == nil {
		return NewClient()
	}
	request_client := &http.Client{
		Transport:     http_client.Transport,
		CheckRedirect: stop_automatic_redirect,
		Jar:           http_client.Jar,
		Timeout:       http_client.Timeout,
	}
	return &Client{
		http_client:    request_client,
		response_limit: default_response_limit,
	}
}

// Close releases idle HTTP connections held by the client.
func (c *Client) Close() {
	if c == nil || c.http_client == nil {
		return
	}
	c.http_client.CloseIdleConnections()
}

// Fetch retrieves the HTML for a Xiaohongshu share or note URL.
func (c *Client) Fetch(raw_url string) (string, error) {
	return c.FetchContext(context.Background(), raw_url)
}

// FetchContext retrieves the HTML for a Xiaohongshu share or note URL with
// cancellation support.
func (c *Client) FetchContext(fetch_context context.Context, raw_url string) (string, error) {
	if c == nil || c.http_client == nil {
		return "", errors.New("xiaohongshu client is not initialized")
	}
	request_url, err := parse_request_url(raw_url)
	if err != nil {
		return "", err
	}
	if fetch_context == nil {
		fetch_context = context.Background()
	}
	select {
	case <-fetch_context.Done():
		return "", fmt.Errorf("xiaohongshu: fetch canceled: %w", fetch_context.Err())
	default:
	}

	response, final_url, err := c.follow_redirect_chain(fetch_context, request_url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	response_limit := c.response_limit
	if response_limit <= 0 {
		response_limit = default_response_limit
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, response_limit+1))
	if err != nil {
		return "", fmt.Errorf("xiaohongshu: read response body: %w", err)
	}
	if int64(len(body)) > response_limit {
		return "", fmt.Errorf("xiaohongshu: response body exceeds %d bytes", response_limit)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("xiaohongshu: HTTP %d for %q: %s", response.StatusCode, final_url.String(), response_preview(body))
	}

	decoded_reader, err := charset.NewReader(bytes.NewReader(body), response.Header.Get("Content-Type"))
	if err != nil {
		return "", fmt.Errorf("xiaohongshu: determine response encoding: %w", err)
	}
	decoded_body, err := io.ReadAll(decoded_reader)
	if err != nil {
		return "", fmt.Errorf("xiaohongshu: decode response body: %w", err)
	}
	return string(decoded_body), nil
}

func (c *Client) follow_redirect_chain(fetch_context context.Context, request_url *url.URL) (*http.Response, *url.URL, error) {
	current_url := request_url
	for redirect_count := 0; ; redirect_count++ {
		request, err := http.NewRequestWithContext(fetch_context, http.MethodGet, current_url.String(), nil)
		if err != nil {
			return nil, nil, fmt.Errorf("xiaohongshu: create request: %w", err)
		}
		set_navigation_headers(request.Header)

		response, err := c.http_client.Do(request)
		if err != nil {
			return nil, nil, fmt.Errorf("xiaohongshu: fetch %q: %w", current_url.String(), err)
		}
		if !is_redirect_status(response.StatusCode) {
			return response, current_url, nil
		}
		if redirect_count >= max_redirect_count {
			response.Body.Close()
			return nil, nil, fmt.Errorf("xiaohongshu: stopped after %d redirects", max_redirect_count)
		}

		location := strings.TrimSpace(response.Header.Get("Location"))
		response.Body.Close()
		if location == "" {
			return nil, nil, fmt.Errorf("xiaohongshu: redirect from %q is missing Location", current_url.String())
		}
		next_url, err := current_url.Parse(location)
		if err != nil {
			return nil, nil, fmt.Errorf("xiaohongshu: invalid redirect Location %q: %w", location, err)
		}
		if err := validate_request_url(next_url); err != nil {
			return nil, nil, fmt.Errorf("xiaohongshu: invalid redirect URL: %w", err)
		}
		current_url = next_url
	}
}

func stop_automatic_redirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

func is_redirect_status(status_code int) bool {
	switch status_code {
	case http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusSeeOther,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

func parse_request_url(raw_url string) (*url.URL, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return nil, errors.New("xiaohongshu URL is empty")
	}
	request_url, err := url.Parse(raw_url)
	if err != nil {
		return nil, fmt.Errorf("xiaohongshu: parse URL: %w", err)
	}
	if err := validate_request_url(request_url); err != nil {
		return nil, err
	}
	return request_url, nil
}

func validate_request_url(request_url *url.URL) error {
	if request_url == nil {
		return errors.New("xiaohongshu URL is nil")
	}
	if !strings.EqualFold(request_url.Scheme, "https") {
		return fmt.Errorf("xiaohongshu: unsupported URL scheme %q", request_url.Scheme)
	}
	host := strings.TrimSuffix(strings.ToLower(request_url.Hostname()), ".")
	if !is_xiaohongshu_host(host) {
		return fmt.Errorf("xiaohongshu: unsupported URL host %q", request_url.Hostname())
	}
	return nil
}

func is_xiaohongshu_host(host string) bool {
	return host == "xhslink.cn" || strings.HasSuffix(host, ".xhslink.cn") ||
		host == "xhslink.com" || strings.HasSuffix(host, ".xhslink.com") ||
		host == "xiaohongshu.com" || strings.HasSuffix(host, ".xiaohongshu.com")
}

func set_navigation_headers(headers http.Header) {
	headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	headers.Set("Accept-Language", "zh-CN,zh;q=0.9")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Pragma", "no-cache")
	headers.Set("Priority", "u=0, i")
	headers.Set("Sec-Ch-Ua", `"Not=A?Brand";v="99", "Google Chrome";v="151", "Chromium";v="151"`)
	headers.Set("Sec-Ch-Ua-Mobile", "?0")
	headers.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	headers.Set("Sec-Fetch-Dest", "document")
	headers.Set("Sec-Fetch-Mode", "navigate")
	headers.Set("Sec-Fetch-Site", "none")
	headers.Set("Sec-Fetch-User", "?1")
	headers.Set("Upgrade-Insecure-Requests", "1")
	headers.Set("User-Agent", default_user_agent)
}

func response_preview(body []byte) string {
	preview := strings.TrimSpace(strings.ToValidUTF8(string(body), "�"))
	if len(preview) > 512 {
		preview = preview[:512]
	}
	return preview
}
