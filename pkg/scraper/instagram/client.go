// Package instagram fetches Instagram posts and account profiles from embedded page data.
package instagram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"wx_channel/pkg/cookies"
)

const (
	PlatformID       = "instagram"
	DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36"
)

var (
	shortcode_pattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	username_pattern  = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.]{0,29}$`)
)

type Client struct {
	http_client   *http.Client
	cookie_reader *cookies.Reader
}

// NewClient creates an HTTP session with optional persistent browser cookies.
func NewClient(cookie_providers ...*cookies.Reader) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("instagram: create cookie jar: %w", err)
	}
	client := &Client{http_client: &http.Client{
		Timeout: 30 * time.Second, Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}
	if len(cookie_providers) > 0 {
		client.cookie_reader = cookie_providers[0]
	}
	return client, nil
}

func (c *Client) Close() {
	if c != nil && c.http_client != nil {
		c.http_client.CloseIdleConnections()
		c.http_client = nil
	}
}

func (c *Client) Fetch(raw_url string) (*FetchResult, error) {
	return c.FetchContext(context.Background(), raw_url)
}

func (c *Client) FetchContext(fetch_context context.Context, raw_url string) (*FetchResult, error) {
	shortcode, err := ExtractShortcode(raw_url)
	if err != nil {
		return nil, err
	}
	page_url := "https://www.instagram.com/p/" + shortcode + "/"
	request_url, _ := url.Parse(strings.TrimSpace(raw_url))
	if request_url.RawQuery != "" {
		page_url += "?" + request_url.RawQuery
	}
	page_html, err := c.fetch_page(fetch_context, page_url)
	if err != nil {
		return nil, err
	}
	return ParseFetchResult(page_url, page_html)
}

// FetchAccount accepts a username or Instagram profile URL.
func (c *Client) FetchAccount(account_url string) (*Account, error) {
	return c.FetchAccountContext(context.Background(), account_url)
}

func (c *Client) FetchAccountContext(fetch_context context.Context, account_url string) (*Account, error) {
	username, err := ExtractUsername(account_url)
	if err != nil {
		return nil, err
	}
	page_html, err := c.fetch_page(fetch_context, "https://www.instagram.com/"+username+"/")
	if err != nil {
		return nil, err
	}
	return ParseAccount(username, page_html)
}

func (c *Client) fetch_page(fetch_context context.Context, page_url string) (string, error) {
	if c == nil || c.http_client == nil {
		return "", fmt.Errorf("instagram: client is closed")
	}
	if fetch_context == nil {
		fetch_context = context.Background()
	}
	request, err := http.NewRequestWithContext(fetch_context, http.MethodGet, page_url, nil)
	if err != nil {
		return "", fmt.Errorf("instagram: create request: %w", err)
	}
	request.Header = http.Header{
		"User-Agent":                {DefaultUserAgent},
		"Accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
		"Accept-Language":           {"zh-CN,zh;q=0.9"},
		"Cache-Control":             {"no-cache"},
		"Pragma":                    {"no-cache"},
		"Priority":                  {"u=0, i"},
		"Sec-Ch-Ua":                 {`"Chromium";v="152", "Not?A_Brand";v="24", "Google Chrome";v="152"`},
		"Sec-Ch-Ua-Mobile":          {"?0"},
		"Sec-Ch-Ua-Platform":        {`"macOS"`},
		"Sec-Fetch-Dest":            {"document"},
		"Sec-Fetch-Mode":            {"navigate"},
		"Sec-Fetch-Site":            {"none"},
		"Sec-Fetch-User":            {"?1"},
		"Upgrade-Insecure-Requests": {"1"},
	}
	if c.cookie_reader != nil {
		cookie_header, err := c.cookie_reader.HeaderForURL(page_url)
		if err != nil && !errors.Is(err, cookies.ErrCookieNotFound) {
			return "", fmt.Errorf("instagram: read cookies: %w", err)
		}
		if cookie_header != "" {
			request.Header.Set("Cookie", cookie_header)
		}
	}
	response, err := c.http_client.Do(request)
	if err != nil {
		return "", fmt.Errorf("instagram: fetch page: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("instagram: page returned HTTP %d", response.StatusCode)
	}
	const response_limit = 16 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, response_limit+1))
	if err != nil {
		return "", fmt.Errorf("instagram: read page: %w", err)
	}
	if len(body) > response_limit {
		return "", fmt.Errorf("instagram: page exceeds %d bytes", response_limit)
	}
	return string(body), nil
}

// ExtractShortcode validates a post, reel, or TV URL, ignoring gallery selection.
func ExtractShortcode(raw_url string) (string, error) {
	path_parts, err := url_path_parts(raw_url)
	if err != nil {
		return "", err
	}
	if len(path_parts) != 2 || (path_parts[0] != "p" && path_parts[0] != "reel" && path_parts[0] != "tv") || !shortcode_pattern.MatchString(path_parts[1]) {
		return "", fmt.Errorf("instagram: URL is not a post URL")
	}
	return path_parts[1], nil
}

func ExtractUsername(raw_url string) (string, error) {
	username := strings.TrimSpace(raw_url)
	if strings.Contains(username, "://") {
		path_parts, err := url_path_parts(username)
		if err != nil {
			return "", err
		}
		if len(path_parts) != 1 {
			return "", fmt.Errorf("instagram: URL is not an account URL")
		}
		username = path_parts[0]
	}
	username = strings.TrimPrefix(username, "@")
	if !username_pattern.MatchString(username) {
		return "", fmt.Errorf("instagram: invalid username")
	}
	switch strings.ToLower(username) {
	case "p", "reel", "reels", "tv", "accounts", "explore", "direct", "stories", "about", "developer", "legal":
		return "", fmt.Errorf("instagram: reserved account path")
	}
	return strings.ToLower(username), nil
}

func url_path_parts(raw_url string) ([]string, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || parsed_url.Scheme != "https" || parsed_url.User != nil || (parsed_url.Port() != "" && parsed_url.Port() != "443") {
		return nil, fmt.Errorf("instagram: invalid HTTPS URL")
	}
	host := strings.ToLower(parsed_url.Hostname())
	if host != "instagram.com" && host != "www.instagram.com" && host != "m.instagram.com" {
		return nil, fmt.Errorf("instagram: unsupported URL host")
	}
	return strings.Split(strings.Trim(parsed_url.EscapedPath(), "/"), "/"), nil
}
