package kuaishou

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"wx_channel/pkg/scraper/internal/homepage"
)

const profile_feed_url = "https://www.kuaishou.com/rest/v/profile/feed"

// FetchUserFeedContext fetches the first page of works on a Kuaishou profile.
func (c *Client) FetchUserFeedContext(fetch_context context.Context, user_id string) ([]Feed, error) {
	if c == nil {
		return nil, fmt.Errorf("kuaishou client is not initialized")
	}
	user_id = strings.TrimSpace(user_id)
	if user_id == "" {
		return nil, fmt.Errorf("Kuaishou user ID is empty")
	}
	cookie_header, err := c.home_cookie_header()
	if err != nil {
		return nil, err
	}
	kww := cookie_header_value(cookie_header, "kwfv1")
	if kww == "" {
		return nil, fmt.Errorf("cookies.json 缺少快手 kwfv1 凭证")
	}
	request_body, _ := json.Marshal(map[string]string{"user_id": user_id, "pcursor": "", "page": "profile"})
	response_json, err := homepage.Request(fetch_context, homepage.RequestOptions{
		URL: profile_feed_url, Method: http.MethodPost, Body: request_body,
		Scope: "posts", AccountID: user_id,
		Headers: http.Header{
			"Accept":          {"application/json, text/plain, */*"},
			"Cookie":          {cookie_header},
			"Content-Type":    {"application/json"},
			"Origin":          {"https://www.kuaishou.com"},
			"Referer":         {"https://www.kuaishou.com/profile/" + url.PathEscape(user_id)},
			"profile_referer": {""},
			"kww":             {kww},
		},
		CookieReader: c.cookie_provider, Cache: c.file_cache,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch Kuaishou profile feed: %w", err)
	}
	return parse_user_feed_response(response_json)
}

func parse_user_feed_response(response_json []byte) ([]Feed, error) {
	var response struct {
		Result   int    `json:"result"`
		ErrorMsg string `json:"error_msg"`
		Feeds    []Feed `json:"feeds"`
	}
	if err := json.Unmarshal(response_json, &response); err != nil {
		return nil, fmt.Errorf("decode Kuaishou profile feed: %w", err)
	}
	if response.Result != 1 {
		message := strings.TrimSpace(response.ErrorMsg)
		if message == "" {
			message = "cookies.json 中的快手凭证无效或 kwfv1 已过期"
		}
		return nil, fmt.Errorf("Kuaishou profile API returned %d: %s", response.Result, message)
	}
	return response.Feeds, nil
}

func (c *Client) home_cookie_header() (string, error) {
	if strings.TrimSpace(c.cookie) != "" {
		return strings.TrimSpace(c.cookie), nil
	}
	if c.cookie_provider == nil {
		return "", fmt.Errorf("缺少 cookies.json 快手凭证")
	}
	cookie_header, err := c.cookie_provider.HeaderForURL(profile_feed_url)
	if err != nil {
		return "", fmt.Errorf("读取 cookies.json 快手凭证失败: %w", err)
	}
	return cookie_header, nil
}

func cookie_header_value(cookie_header string, name string) string {
	for _, field := range strings.Split(cookie_header, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(field), "=")
		if ok && key == name {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
