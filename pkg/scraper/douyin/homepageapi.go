package douyin

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	douyin_home_api_url         = "https://www.douyin.com/aweme/v1/web/aweme/post/"
	douyin_home_api_count       = "18"
	douyin_home_api_timeout     = 15 * time.Second
	douyin_home_user_agent      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36"
	douyin_home_sec_ch_ua       = `"Chromium";v="152", "Not?A_Brand";v="24", "Google Chrome";v="152"`
	douyin_home_sec_ch_platform = `"macOS"`
)

var douyin_home_api_param_order = []string{
	"device_platform", "aid", "channel", "sec_user_id", "max_cursor",
	"locate_query", "show_live_replay_strategy", "need_time_list", "time_list_query",
	"whale_cut_token", "cut_version", "count", "publish_video_strategy_type",
	"from_user_page", "update_version_code", "pc_client_type", "pc_libra_divert",
	"support_h265", "support_dash", "cpu_core_num", "version_code", "version_name",
	"cookie_enabled", "screen_width", "screen_height", "browser_language",
	"browser_platform", "browser_name", "browser_version", "browser_online",
	"engine_name", "engine_version", "os_name", "os_version", "device_memory",
	"platform", "downlink", "effective_type", "round_trip_time", "webid", "uifid",
	"verifyFp", "fp", "msToken", "timestamp",
}

type douyin_home_api_response struct {
	AwemeList  []douyin_home_api_aweme `json:"aweme_list"`
	HasMore    int                     `json:"has_more"`
	MaxCursor  int64                   `json:"max_cursor"`
	StatusCode int                     `json:"status_code"`
	StatusMsg  string                  `json:"status_msg"`
}

type douyin_home_api_aweme struct {
	AwemeID   string                  `json:"aweme_id"`
	AwemeType int                     `json:"aweme_type"`
	Desc      string                  `json:"desc"`
	Images    []douyin_home_api_image `json:"images"`
	Video     douyin_home_api_video   `json:"video"`
}

type douyin_home_api_video struct {
	Cover douyin_home_api_image `json:"cover"`
}

type douyin_home_api_image struct {
	URLList []string `json:"url_list"`
}

func (c *Client) fetch_home_api(owner_id string, page_url string) (*HomeResult, error) {
	return c.fetch_home_api_page(owner_id, page_url, "")
}

func (c *Client) fetch_home_api_page(owner_id string, page_url string, page_marker string) (*HomeResult, error) {
	if c == nil || c.web == nil {
		return nil, fmt.Errorf("douyin home API client is nil")
	}
	started_at := time.Now()
	cookie_header := c.home_cookie_header()
	params := douyin_home_api_page_params(owner_id, cookie_header, page_marker)
	request_url := douyin_home_api_url + "?" + queryStringify(params, douyin_home_api_param_order)
	body, err := c.request_home_api(page_url, request_url, cookie_header)
	if err != nil {
		return nil, err
	}
	return c.decode_home_api_response(owner_id, page_marker, body, started_at)
}

func (c *Client) request_home_api(page_url string, request_url string, cookie_header string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), douyin_home_api_timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, request_url, nil)
	if err != nil {
		return nil, fmt.Errorf("douyin home API: construct request: %w", err)
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Origin", "https://www.douyin.com")
	req.Header.Set("Referer", page_url)
	req.Header.Set("Sec-CH-UA", douyin_home_sec_ch_ua)
	req.Header.Set("Sec-CH-UA-Mobile", "?0")
	req.Header.Set("Sec-CH-UA-Platform", douyin_home_sec_ch_platform)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("User-Agent", douyin_home_user_agent)
	if cookie_header != "" {
		req.Header.Set("Cookie", cookie_header)
	}
	if uifid := home_cookie_value(cookie_header, "UIFID"); uifid != "" {
		req.Header.Set("uifid", uifid)
	}

	resp, err := c.web.request_client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("douyin home API: request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("douyin home API: read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("douyin home API: HTTP %d body=%q", resp.StatusCode, log_body_preview(body))
	}
	return body, nil
}

func (c *Client) decode_home_api_response(owner_id string, page_marker string, body []byte, started_at time.Time) (*HomeResult, error) {
	var result douyin_home_api_response
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("douyin home API: decode response: %w", err)
	}
	if result.StatusCode != 0 {
		return nil, fmt.Errorf("douyin home API: status %d: %s", result.StatusCode, strings.TrimSpace(result.StatusMsg))
	}
	contents := home_contents_from_api_response(result.AwemeList)
	next_marker := ""
	if result.HasMore != 0 && result.MaxCursor > 0 {
		next_marker = strconv.FormatInt(result.MaxCursor, 10)
	}
	c.logger.Info().
		Str("owner_id", owner_id).
		Str("page_marker", page_marker).
		Str("next_marker", next_marker).
		Int("contents", len(contents)).
		Dur("elapsed", time.Since(started_at)).
		Msg("douyin home: works API completed")
	return &HomeResult{
		HTML: home_contents_html(contents), Contents: contents,
		NextMarker: next_marker, PaginationKnown: true,
	}, nil
}

func douyin_home_api_params(owner_id string, cookie_header string) map[string]string {
	return douyin_home_api_page_params(owner_id, cookie_header, "")
}

func douyin_home_api_page_params(owner_id string, cookie_header string, page_marker string) map[string]string {
	max_cursor := page_marker
	if max_cursor == "" {
		max_cursor = "0"
	}
	need_time_list := "0"
	if page_marker == "" {
		need_time_list = "1"
	}
	params := map[string]string{
		"device_platform": "webapp", "aid": "6383", "channel": "channel_pc_web",
		"sec_user_id": owner_id, "max_cursor": max_cursor, "locate_query": "false",
		"show_live_replay_strategy": "1", "need_time_list": need_time_list, "time_list_query": "0",
		"whale_cut_token": "", "cut_version": "1", "count": douyin_home_api_count,
		"publish_video_strategy_type": "2", "from_user_page": "1", "update_version_code": "170400",
		"pc_client_type": "1", "pc_libra_divert": "Mac", "support_h265": "1", "support_dash": "1",
		"cpu_core_num": "10", "version_code": "290100", "version_name": "29.1.0",
		"cookie_enabled": strconv.FormatBool(cookie_header != ""), "screen_width": "1512", "screen_height": "982",
		"browser_language": "zh-CN", "browser_platform": "MacIntel", "browser_name": "Chrome",
		"browser_version": "152.0.0.0", "browser_online": "true", "engine_name": "Blink",
		"engine_version": "152.0.0.0", "os_name": "Mac OS", "os_version": "10.15.7",
		"device_memory": "32", "platform": "PC", "downlink": "10", "effective_type": "4g",
		"round_trip_time": "100", "webid": douyin_home_webid(owner_id, cookie_header),
		"msToken": "", "timestamp": strconv.FormatInt(time.Now().Unix(), 10),
	}
	if uifid := home_cookie_value(cookie_header, "UIFID"); uifid != "" {
		params["uifid"] = uifid
	}
	if verify_fp := home_cookie_value(cookie_header, "s_v_web_id"); verify_fp != "" {
		params["verifyFp"] = verify_fp
		params["fp"] = verify_fp
	}
	return params
}

func normalize_home_page_marker(page_marker string) (string, error) {
	page_marker = strings.TrimSpace(page_marker)
	if page_marker == "" {
		return "", nil
	}
	cursor, err := strconv.ParseUint(page_marker, 10, 64)
	if err != nil || cursor == 0 {
		return "", fmt.Errorf("douyin home: page marker must be a positive integer: %q", page_marker)
	}
	return strconv.FormatUint(cursor, 10), nil
}

func douyin_home_webid(owner_id string, cookie_header string) string {
	seed := home_cookie_value(cookie_header, "UIFID")
	if seed == "" {
		seed = home_cookie_value(cookie_header, "ttwid")
	}
	if seed == "" {
		seed = owner_id
	}
	digest := sha256.Sum256([]byte(seed))
	const webid_base uint64 = 7_000_000_000_000_000_000
	const webid_range uint64 = 1_000_000_000_000_000_000
	return strconv.FormatUint(webid_base+binary.BigEndian.Uint64(digest[:8])%webid_range, 10)
}

func (c *Client) home_cookie_header() string {
	if c != nil && c.cookie_reader != nil {
		cookie_header, err := c.cookie_reader.HeaderForDomain("www.douyin.com")
		if err == nil && strings.TrimSpace(cookie_header) != "" {
			return strings.TrimSpace(cookie_header)
		}
	}
	if c != nil && c.web != nil {
		return strings.TrimSpace(c.web.cookie)
	}
	return ""
}

func home_cookie_value(cookie_header string, name string) string {
	request_url, _ := url.Parse("https://www.douyin.com/")
	req := &http.Request{Header: http.Header{"Cookie": []string{cookie_header}}, URL: request_url}
	for _, cookie := range req.Cookies() {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func home_contents_from_api_response(aweme_list []douyin_home_api_aweme) []HomeContent {
	contents := make([]HomeContent, 0, len(aweme_list))
	seen := make(map[string]struct{}, len(aweme_list))
	for _, aweme := range aweme_list {
		aweme_id := strings.TrimSpace(aweme.AwemeID)
		if aweme_id == "" {
			continue
		}
		content_type := "video"
		cover_urls := aweme.Video.Cover.URLList
		if len(aweme.Images) > 0 || aweme.AwemeType == 68 {
			content_type = "note"
			if len(aweme.Images) > 0 {
				cover_urls = aweme.Images[0].URLList
			}
		}
		content_key := content_type + ":" + aweme_id
		if _, exists := seen[content_key]; exists {
			continue
		}
		seen[content_key] = struct{}{}
		cover_url := ""
		if len(cover_urls) > 0 {
			cover_url = strings.TrimSpace(cover_urls[0])
		}
		contents = append(contents, HomeContent{
			ID: aweme_id, Type: content_type, Title: strings.TrimSpace(aweme.Desc),
			URL:      "https://www.douyin.com/" + content_type + "/" + url.PathEscape(aweme_id),
			CoverURL: cover_url,
		})
	}
	return contents
}

func home_contents_html(contents []HomeContent) string {
	var document strings.Builder
	document.WriteString(`<main><div data-e2e="user-post-list"><ul data-e2e="scroll-list">`)
	for _, content := range contents {
		document.WriteString(`<li><a href="`)
		document.WriteString(stdhtml.EscapeString(content.URL))
		document.WriteString(`"><img src="`)
		document.WriteString(stdhtml.EscapeString(content.CoverURL))
		document.WriteString(`" alt="`)
		document.WriteString(stdhtml.EscapeString(content.Title))
		document.WriteString(`"></a></li>`)
	}
	document.WriteString(`</ul></div></main>`)
	return document.String()
}
