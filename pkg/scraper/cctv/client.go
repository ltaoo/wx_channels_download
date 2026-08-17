package cctv

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html/charset"
)

const (
	vc_key                = "47899B86370B879139C08EA3B5E88267"
	default_request_limit = int64(8 << 20)
	page_info_callback    = "contentid"
)

var (
	guid_patterns = []*regexp.Regexp{
		regexp.MustCompile(`(?is)\bitemguid\s*=\s*["']([0-9a-f]{32})["']`),
		regexp.MustCompile(`(?is)\bguid\s*=\s*["']([0-9a-f]{32})["']`),
		regexp.MustCompile(`(?is)([0-9a-f]{32})-300\.jpg`),
	}
	title_patterns = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<meta[^>]+property\s*=\s*["']og:title["'][^>]+content\s*=\s*["']([^"']+)["']`),
		regexp.MustCompile(`(?is)<meta[^>]+content\s*=\s*["']([^"']+)["'][^>]+property\s*=\s*["']og:title["']`),
		regexp.MustCompile(`(?is)<title[^>]*>\s*(.*?)\s*</title>`),
	}
	description_patterns = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<meta[^>]+(?:name|property)\s*=\s*["'](?:description|og:description)["'][^>]+content\s*=\s*["']([^"']*)["']`),
		regexp.MustCompile(`(?is)<meta[^>]+content\s*=\s*["']([^"']*)["'][^>]+(?:name|property)\s*=\s*["'](?:description|og:description)["']`),
	}
	keyword_patterns = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<meta[^>]+name\s*=\s*["']keywords["'][^>]+content\s*=\s*["']([^"']*)["']`),
		regexp.MustCompile(`(?is)<meta[^>]+content\s*=\s*["']([^"']*)["'][^>]+name\s*=\s*["']keywords["']`),
	}
	content_id_patterns = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<meta[^>]+name\s*=\s*["']contentid["'][^>]+content\s*=\s*["']([^"']+)["']`),
		regexp.MustCompile(`(?is)<meta[^>]+content\s*=\s*["']([^"']+)["'][^>]+name\s*=\s*["']contentid["']`),
	}
	cmstid_patterns = []*regexp.Regexp{
		regexp.MustCompile(`(?is)\bsub_column_id\s*=\s*["']([^"']+)["']`),
		regexp.MustCompile(`(?is)\bvideo_CHANNEL\s*=\s*["']([^"']+)["']`),
	}
)

// Client retrieves CCTV video pages and their VDN metadata.
type Client struct {
	http_client    *http.Client
	uid            string
	video_info_api string
	page_info_api  string
	now            func() time.Time
	request_limit  int64
	user_agent     string
}

// NewClient creates a CCTV scraper client with a 20-second request timeout.
func NewClient() *Client {
	return NewClientWithHTTPClient(nil)
}

// NewClientWithHTTPClient creates a CCTV scraper client using http_client.
func NewClientWithHTTPClient(http_client *http.Client) *Client {
	if http_client == nil {
		http_client = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{
		http_client:    http_client,
		uid:            DefaultUID,
		video_info_api: VideoInfoAPIURL,
		page_info_api:  PageInfoAPIURL,
		now:            time.Now,
		request_limit:  default_request_limit,
		user_agent:     DefaultUserAgent,
	}
}

// SetUID sets the UID used to sign subsequent VDN requests. An empty value
// restores DefaultUID.
func (c *Client) SetUID(uid string) {
	if c == nil {
		return
	}
	uid = strings.TrimSpace(uid)
	if uid == "" {
		uid = DefaultUID
	}
	c.uid = uid
}

// Fetch retrieves a CCTV page and its structured VDN video information.
func (c *Client) Fetch(page_url string) (*FetchResult, error) {
	return c.FetchContext(context.Background(), page_url)
}

// GetVideoInfo is an alias of Fetch.
func (c *Client) GetVideoInfo(page_url string) (*FetchResult, error) {
	return c.Fetch(page_url)
}

// FetchContext retrieves a CCTV page and its structured VDN video information
// using fetch_context for cancellation.
func (c *Client) FetchContext(fetch_context context.Context, page_url string) (*FetchResult, error) {
	if c == nil {
		return nil, fmt.Errorf("cctv client is nil")
	}
	page_url = strings.TrimSpace(page_url)
	if page_url == "" {
		return nil, fmt.Errorf("CCTV URL 不能为空")
	}
	if fetch_context == nil {
		fetch_context = context.Background()
	}
	if _, err := parse_http_url(page_url); err != nil {
		return nil, fmt.Errorf("无效的 CCTV URL: %w", err)
	}

	page_headers := http.Header{
		"User-Agent": []string{c.user_agent},
		"Referer":    []string{"https://v.cctv.com/"},
	}
	page_html, err := c.fetch_text(fetch_context, page_url, page_headers)
	if err != nil {
		return nil, fmt.Errorf("获取 CCTV 视频页面失败: %w", err)
	}
	page_info, err := ExtractPageInfo(page_html)
	if err != nil {
		return nil, fmt.Errorf("解析 CCTV 视频页面失败: %w", err)
	}

	page_info_url := ""
	var page_info_response PageInfoResponse
	if page_info.CMSTID != "" {
		page_info_url, err = build_page_info_url(c.page_info_api, page_info.CMSTID)
		if err != nil {
			return nil, err
		}
		page_info_headers := http.Header{
			"User-Agent": []string{c.user_agent},
			"Accept":     []string{"*/*"},
			"Referer":    []string{page_url},
		}
		page_info_text, fetch_err := c.fetch_text(fetch_context, page_info_url, page_info_headers)
		if fetch_err != nil {
			return nil, fmt.Errorf("获取 CCTV 页面信息失败: %w", fetch_err)
		}
		page_info_response, err = ParsePageInfoResponse(page_info_text)
		if err != nil {
			return nil, fmt.Errorf("解析 CCTV 页面信息失败: %w", err)
		}
	}

	api_url, err := build_video_info_url(c.video_info_api, page_info.GUID, c.uid, c.now().Unix(), DefaultVN)
	if err != nil {
		return nil, err
	}
	api_headers := http.Header{
		"User-Agent": []string{c.user_agent},
		"Accept":     []string{"*/*"},
		"Origin":     []string{"https://v.cctv.com"},
		"Referer":    []string{"https://v.cctv.com/"},
	}
	api_text, err := c.fetch_text(fetch_context, api_url, api_headers)
	if err != nil {
		return nil, fmt.Errorf("获取 CCTV 视频信息失败: %w", err)
	}
	var video_info VideoInfo
	if err := json.Unmarshal([]byte(api_text), &video_info); err != nil {
		return nil, fmt.Errorf("解析 CCTV 视频信息失败: %w", err)
	}

	return &FetchResult{
		PageURL:         page_url,
		PageTitle:       page_info.Title,
		PageDescription: page_info.Description,
		PageKeywords:    page_info.Keywords,
		PageContentID:   page_info.ContentID,
		CMSTID:          page_info.CMSTID,
		PageInfoURL:     page_info_url,
		PageInfo:        page_info_response,
		PID:             page_info.GUID,
		APIURL:          api_url,
		Data:            video_info,
	}, nil
}

// GetVideoInfo retrieves one CCTV video using a default client.
func GetVideoInfo(page_url string) (*FetchResult, error) {
	return NewClient().Fetch(page_url)
}

// ExtractPageInfo extracts the video GUID and title from CCTV page HTML.
func ExtractPageInfo(page_html string) (PageInfo, error) {
	guid, err := extract_first(guid_patterns, page_html, "video guid")
	if err != nil {
		return PageInfo{}, err
	}
	title, err := extract_first(title_patterns, page_html, "page title")
	if err != nil {
		return PageInfo{}, err
	}
	return PageInfo{
		GUID:        strings.ToLower(guid),
		Title:       title,
		Description: extract_optional(description_patterns, page_html),
		Keywords:    extract_optional(keyword_patterns, page_html),
		ContentID:   extract_optional(content_id_patterns, page_html),
		CMSTID:      extract_optional(cmstid_patterns, page_html),
	}, nil
}

// ParsePageInfoResponse parses the contentid(...) JSONP returned by CCTV's
// media-page API.
func ParsePageInfoResponse(response_text string) (PageInfoResponse, error) {
	response_text = strings.TrimSpace(response_text)
	prefix := page_info_callback + "("
	if strings.HasPrefix(response_text, prefix) {
		response_text = strings.TrimSpace(strings.TrimPrefix(response_text, prefix))
		response_text = strings.TrimSuffix(response_text, ";")
		response_text = strings.TrimSpace(response_text)
		if !strings.HasSuffix(response_text, ")") {
			return PageInfoResponse{}, fmt.Errorf("CCTV 页面信息 JSONP 结尾无效")
		}
		response_text = strings.TrimSpace(strings.TrimSuffix(response_text, ")"))
	}
	var response PageInfoResponse
	if err := json.Unmarshal([]byte(response_text), &response); err != nil {
		return PageInfoResponse{}, err
	}
	if response.Code != http.StatusOK {
		return PageInfoResponse{}, fmt.Errorf("CCTV 页面信息 API 返回 code=%d msg=%s", response.Code, strings.TrimSpace(response.Msg))
	}
	return response, nil
}

// MakeVC calculates the uppercase VDN request signature.
func MakeVC(tsp string, uid string, vn string) string {
	hash := md5.Sum([]byte(tsp + vn + vc_key + uid))
	return strings.ToUpper(hex.EncodeToString(hash[:]))
}

// BuildVideoInfoURL builds a signed VDN URL using DefaultVN.
func BuildVideoInfoURL(pid string, uid string, tsp int64) (string, error) {
	return build_video_info_url(VideoInfoAPIURL, pid, uid, tsp, DefaultVN)
}

// BuildPageInfoURL builds the media-page API URL for cmstid.
func BuildPageInfoURL(cmstid string) (string, error) {
	return build_page_info_url(PageInfoAPIURL, cmstid)
}

func build_page_info_url(api_url string, cmstid string) (string, error) {
	cmstid = strings.TrimSpace(cmstid)
	if cmstid == "" {
		return "", fmt.Errorf("CCTV 页面 cmstid 为空")
	}
	parsed_api_url, err := parse_http_url(api_url)
	if err != nil {
		return "", fmt.Errorf("CCTV 页面信息 API 无效: %w", err)
	}
	query := parsed_api_url.Query()
	query.Set("cmstid", cmstid)
	query.Set("cb", page_info_callback)
	parsed_api_url.RawQuery = query.Encode()
	return parsed_api_url.String(), nil
}

func build_video_info_url(api_url string, pid string, uid string, tsp int64, vn string) (string, error) {
	pid = strings.ToLower(strings.TrimSpace(pid))
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(pid) {
		return "", fmt.Errorf("CCTV 视频 GUID 无效: %q", pid)
	}
	uid = strings.TrimSpace(uid)
	if uid == "" {
		uid = DefaultUID
	}
	parsed_api_url, err := parse_http_url(api_url)
	if err != nil {
		return "", fmt.Errorf("CCTV 视频信息 API 无效: %w", err)
	}
	tsp_text := fmt.Sprintf("%d", tsp)
	query := parsed_api_url.Query()
	query.Set("pid", pid)
	query.Set("client", "flash")
	query.Set("im", "0")
	query.Set("tsp", tsp_text)
	query.Set("vn", vn)
	query.Set("vc", MakeVC(tsp_text, uid, vn))
	query.Set("uid", uid)
	query.Set("wlan", "")
	parsed_api_url.RawQuery = query.Encode()
	return parsed_api_url.String(), nil
}

func (c *Client) fetch_text(fetch_context context.Context, request_url string, headers http.Header) (string, error) {
	req, err := http.NewRequestWithContext(fetch_context, http.MethodGet, request_url, nil)
	if err != nil {
		return "", err
	}
	req.Header = headers.Clone()
	resp, err := c.http_client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	limit := c.request_limit
	if limit <= 0 {
		limit = default_request_limit
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return "", err
	}
	if int64(len(body)) > limit {
		return "", fmt.Errorf("response body exceeds %d bytes", limit)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		preview := strings.TrimSpace(strings.ToValidUTF8(string(body), "�"))
		if len(preview) > 512 {
			preview = preview[:512]
		}
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, preview)
	}

	decoded_reader, err := charset.NewReader(bytes.NewReader(body), resp.Header.Get("Content-Type"))
	if err != nil {
		return "", err
	}
	decoded_body, err := io.ReadAll(decoded_reader)
	if err != nil {
		return "", err
	}
	return string(decoded_body), nil
}

func extract_first(patterns []*regexp.Regexp, text string, name string) (string, error) {
	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(text)
		if len(matches) < 2 {
			continue
		}
		value := strings.TrimSpace(html.UnescapeString(matches[1]))
		if value != "" {
			return value, nil
		}
	}
	return "", fmt.Errorf("could not find %s", name)
}

func extract_optional(patterns []*regexp.Regexp, text string) string {
	value, _ := extract_first(patterns, text, "optional value")
	return value
}

func parse_http_url(raw_url string) (*url.URL, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return nil, err
	}
	if parsed_url.Scheme != "http" && parsed_url.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %q", parsed_url.Scheme)
	}
	if parsed_url.Hostname() == "" {
		return nil, fmt.Errorf("URL host is empty")
	}
	return parsed_url, nil
}
