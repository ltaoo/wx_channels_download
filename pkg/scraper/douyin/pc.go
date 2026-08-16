package douyin

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	xhtml "golang.org/x/net/html"

	"wx_channel/pkg/cookies"
)

const (
	douyin_pc_iteminfo_url      = "https://www.iesdouyin.com/web/api/v2/aweme/iteminfo/"
	douyin_pc_slidesinfo_url    = "https://www.iesdouyin.com/web/api/v2/aweme/slidesinfo/"
	douyin_pc_request_timeout   = 30 * time.Second
	douyin_pc_request_attempts  = 5
	douyin_pc_default_aid       = "1128"
	douyin_pc_default_token_id  = "douyin_reflow_token"
	douyin_pc_reflow_source     = "reflow_page"
	douyin_pc_mobile_user_agent = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.5 Mobile/15E148 Safari/604.1"
)

// DouyinPCClient resolves a Douyin link and returns its normalized detail JSON.
//
// Despite the name, the current PC-compatible flow is browser-free: it opens
// the mobile share page, reads its SSR data, and calls the mobile iteminfo API
// with the reflow_id expected by Douyin.
type DouyinPCClient struct {
	http_client      *http.Client
	cookie_reader    *cookies.Reader
	request_timeout  time.Duration
	request_attempts int
	logger           zerolog.Logger
}

// NewDouyinPCClient creates a PC-compatible scraper that ignores proxy
// environment variables, matching fetch_detail.py --no-env-proxy.
func NewDouyinPCClient() *DouyinPCClient {
	return NewDouyinPCClientWithLogger(nil)
}

// NewDouyinPCClientWithLogger creates a PC-compatible scraper with diagnostics.
func NewDouyinPCClientWithLogger(parent_logger *zerolog.Logger) *DouyinPCClient {
	return NewDouyinPCClientWithCookieReader(nil, parent_logger)
}

// NewDouyinPCClientWithCookieReader creates a PC-compatible scraper that
// refreshes Douyin cookies from the persistent reader for every request.
func NewDouyinPCClientWithCookieReader(cookie_reader *cookies.Reader, parent_logger *zerolog.Logger) *DouyinPCClient {
	return NewDouyinPCClientWithHTTPClientAndCookieReader(nil, cookie_reader, parent_logger)
}

// NewDouyinPCClientWithHTTPClient creates a PC-compatible scraper using the
// supplied HTTP client. Passing nil creates a direct client that does not use
// HTTP_PROXY or HTTPS_PROXY. The custom-client form is useful when callers need
// their own transport, proxy, cookie jar, or test double.
func NewDouyinPCClientWithHTTPClient(http_client *http.Client, parent_logger *zerolog.Logger) *DouyinPCClient {
	return NewDouyinPCClientWithHTTPClientAndCookieReader(http_client, nil, parent_logger)
}

// NewDouyinPCClientWithHTTPClientAndCookieReader creates a PC-compatible
// scraper with both a custom HTTP client and persistent Douyin cookies.
func NewDouyinPCClientWithHTTPClientAndCookieReader(http_client *http.Client, cookie_reader *cookies.Reader, parent_logger *zerolog.Logger) *DouyinPCClient {
	if http_client == nil {
		http_client = new_douyin_pc_http_client()
	}
	return &DouyinPCClient{
		http_client:      http_client,
		cookie_reader:    cookie_reader,
		request_timeout:  douyin_pc_request_timeout,
		request_attempts: douyin_pc_request_attempts,
		logger:           new_component_logger(parent_logger, "douyin_pc"),
	}
}

// Parse resolves raw_url and returns the same normalized detail object printed
// by fetch_detail.py when no output mode is selected.
func (c *DouyinPCClient) Parse(raw_url string) (json.RawMessage, error) {
	return c.FetchDetail(raw_url)
}

// FetchDetail resolves raw_url and returns normalized detail JSON.
func (c *DouyinPCClient) FetchDetail(raw_url string) (json.RawMessage, error) {
	if c == nil {
		return nil, fmt.Errorf("douyin PC client is nil")
	}
	if c.http_client == nil {
		return nil, fmt.Errorf("douyin PC HTTP client is nil")
	}

	douyin_url, err := ExtractURL(raw_url)
	if err != nil {
		return nil, fmt.Errorf("douyin PC: extract URL: %w", err)
	}

	started_at := time.Now()
	c.logger.Info().Str("url", douyin_url).Msg("douyin PC: detail fetch started")
	page_request_url := douyin_url
	attempt_errors := make([]string, 0, c.request_attempts)

	for attempt := 1; attempt <= c.request_attempts; attempt++ {
		detail, final_url, fetch_err := c.fetch_detail_attempt(page_request_url)
		if final_url != "" {
			page_request_url = final_url
		}
		if fetch_err != nil {
			attempt_errors = append(attempt_errors, fmt.Sprintf("attempt %d: %v", attempt, fetch_err))
			c.logger.Warn().
				Err(fetch_err).
				Int("attempt", attempt).
				Str("page_url", page_request_url).
				Msg("douyin PC: detail attempt failed")
			continue
		}

		raw_detail, marshal_err := json.Marshal(detail)
		if marshal_err != nil {
			return nil, fmt.Errorf("douyin PC: encode detail JSON: %w", marshal_err)
		}
		c.logger.Info().
			Int("attempt", attempt).
			Int("detail_bytes", len(raw_detail)).
			Dur("elapsed", time.Since(started_at)).
			Msg("douyin PC: detail fetch completed")
		return json.RawMessage(raw_detail), nil
	}

	return nil, fmt.Errorf(
		"douyin PC: mobile SSR/iteminfo failed after %d attempts: %s",
		c.request_attempts,
		strings.Join(attempt_errors, " | "),
	)
}

func new_douyin_pc_http_client() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Transport: transport,
		Jar:       jar,
	}
}

func (c *DouyinPCClient) fetch_detail_attempt(page_request_url string) (map[string]any, string, error) {
	page_headers := map[string]string{
		"User-Agent":      douyin_pc_mobile_user_agent,
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9",
	}
	page_body, final_url, err := c.get(page_request_url, page_headers)
	if err != nil {
		return nil, final_url, fmt.Errorf("fetch share page: %w", err)
	}

	router_data, err := decode_douyin_pc_router_data(page_body)
	if err != nil {
		var fallback_err error
		router_data, fallback_err = build_douyin_pc_share_router_data(final_url)
		if fallback_err != nil {
			return nil, final_url, err
		}
	}
	video_page_data, err := find_douyin_pc_video_page(router_data)
	if err != nil {
		return nil, final_url, err
	}

	if detail, normalize_err := normalize_douyin_pc_ssr_detail(router_data, video_page_data); normalize_err == nil {
		return detail, final_url, nil
	}

	detail, err := c.fetch_douyin_pc_iteminfo(
		final_url,
		page_body,
		router_data,
		video_page_data,
	)
	if err != nil {
		return nil, final_url, err
	}
	return detail, final_url, nil
}

// build_douyin_pc_share_router_data reconstructs the small portion of
// _ROUTER_DATA needed by the iteminfo request. Douyin slides pages can fall
// back to client-side rendering and omit _ROUTER_DATA while still including
// the webId and xsstoken elements used by the reflow API.
func build_douyin_pc_share_router_data(page_url string) (map[string]any, error) {
	parsed_url, err := url.Parse(page_url)
	if err != nil {
		return nil, fmt.Errorf("douyin PC: parse share page URL: %w", err)
	}

	path_parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
	share_type := ""
	item_id := ""
	for index := 0; index+1 < len(path_parts); index++ {
		if path_parts[index] != "video" && path_parts[index] != "slides" {
			continue
		}
		candidate_id := path_parts[index+1]
		if !douyin_pc_digits(candidate_id) {
			continue
		}
		share_type = path_parts[index]
		item_id = candidate_id
		break
	}
	if item_id == "" {
		return nil, fmt.Errorf("douyin PC: share page URL did not contain an aweme ID")
	}

	query := make(map[string]any, len(parsed_url.Query()))
	for key, values := range parsed_url.Query() {
		if len(values) > 0 {
			query[key] = values[len(values)-1]
		}
	}
	page_data := map[string]any{
		"itemId":   item_id,
		"lastPath": item_id,
		"query":    query,
		"abParams": map[string]any{
			"select_pool_data": map[string]any{
				"use_new_select_scope": 0,
			},
		},
	}
	return map[string]any{
		"loaderData": map[string]any{
			share_type + "_(id)/page": page_data,
		},
	}, nil
}

func (c *DouyinPCClient) get(request_url string, headers map[string]string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.request_timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, request_url, nil)
	if err != nil {
		return nil, request_url, fmt.Errorf("construct GET request: %w", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if c.cookie_reader != nil {
		cookie_header, cookie_err := c.cookie_reader.HeaderForDomain("www.douyin.com")
		if cookie_err == nil && cookie_header != "" {
			req.Header.Set("Cookie", cookie_header)
		} else if cookie_err != nil && !errors.Is(cookie_err, cookies.ErrCookieNotFound) {
			c.logger.Warn().Err(cookie_err).Msg("douyin PC: failed to read persistent cookies")
		}
	}

	resp, err := c.http_client.Do(req)
	if err != nil {
		return nil, request_url, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, response_request_url(resp, request_url), fmt.Errorf("read response body: %w", err)
	}
	final_url := response_request_url(resp, request_url)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, final_url, fmt.Errorf(
			"unexpected HTTP status %d: body_bytes=%d body_preview=%q",
			resp.StatusCode,
			len(body),
			log_body_preview(body),
		)
	}
	return body, final_url, nil
}

func response_request_url(resp *http.Response, fallback_url string) string {
	if resp != nil && resp.Request != nil && resp.Request.URL != nil {
		return resp.Request.URL.String()
	}
	return fallback_url
}

func decode_douyin_pc_router_data(page_html []byte) (map[string]any, error) {
	raw_router_data, err := extract_router_json(page_html)
	if err != nil {
		return nil, fmt.Errorf("douyin PC: %w", err)
	}

	unescaped_router_data := stdhtml.UnescapeString(string(raw_router_data))
	router_data, err := decode_douyin_pc_json_object([]byte(unescaped_router_data))
	if err != nil {
		return nil, fmt.Errorf("douyin PC: decode _ROUTER_DATA: %w", err)
	}
	return router_data, nil
}

func decode_douyin_pc_json_object(data []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var result map[string]any
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("JSON value is not an object")
	}
	return result, nil
}

func find_douyin_pc_video_page(router_data map[string]any) (map[string]any, error) {
	loader_data, ok := douyin_pc_object(router_data["loaderData"])
	if !ok {
		return nil, fmt.Errorf("douyin PC: SSR router data did not contain loaderData")
	}

	page_keys := make([]string, 0, len(loader_data))
	for key := range loader_data {
		page_keys = append(page_keys, key)
	}
	sort.Strings(page_keys)
	for _, key := range page_keys {
		if !strings.HasSuffix(key, "/page") ||
			(!strings.Contains(key, "video") && !strings.Contains(key, "slides")) {
			continue
		}
		if page_data, page_ok := douyin_pc_object(loader_data[key]); page_ok {
			return page_data, nil
		}
	}
	return nil, fmt.Errorf("douyin PC: SSR router data did not contain video page data")
}

func normalize_douyin_pc_ssr_detail(router_data map[string]any, video_page_data map[string]any) (map[string]any, error) {
	video_info, ok := douyin_pc_object(video_page_data["videoInfoRes"])
	if !ok {
		return nil, fmt.Errorf("SSR video page did not contain videoInfoRes")
	}
	item_list, ok := douyin_pc_array(video_info["item_list"])
	if !ok || len(item_list) == 0 {
		return nil, fmt.Errorf("SSR videoInfoRes did not contain item_list")
	}
	aweme, ok := douyin_pc_object(item_list[0])
	if !ok {
		return nil, fmt.Errorf("SSR item_list[0] was not an object")
	}
	aweme = copy_douyin_pc_object(aweme)
	set_douyin_pc_preview_title(aweme)

	return map[string]any{
		"status_code":  video_info["status_code"],
		"status_msg":   video_info["status_msg"],
		"aweme_detail": aweme,
		"extra":        video_info["extra"],
		"log_pb":       video_info["log_pb"],
		"_source":      "mobile_ssr",
		"_web_id":      video_page_data["webId"],
		"_item_id":     douyin_pc_item_id(video_page_data),
		"_router_data": router_data,
	}, nil
}

func (c *DouyinPCClient) fetch_douyin_pc_iteminfo(
	page_url string,
	page_html []byte,
	router_data map[string]any,
	video_page_data map[string]any,
) (map[string]any, error) {
	web_ids := douyin_pc_web_id_candidates(page_html, video_page_data)
	if len(web_ids) == 0 {
		return nil, fmt.Errorf("douyin PC: SSR HTML did not contain a valid webId")
	}

	attempt_errors := make([]string, 0, len(web_ids))
	for _, web_id := range web_ids {
		params, err := build_douyin_pc_iteminfo_params(page_html, video_page_data, web_id)
		if err != nil {
			attempt_errors = append(attempt_errors, fmt.Sprintf("%s: %v", web_id, err))
			continue
		}

		detail_url := douyin_pc_iteminfo_url + "?" + params.Encode()
		headers := map[string]string{
			"User-Agent":      douyin_pc_mobile_user_agent,
			"Accept":          "application/json, text/plain, */*",
			"Accept-Language": "zh-CN,zh;q=0.9",
			"Content-Type":    "application/json",
			"Referer":         page_url,
		}
		body, _, fetch_err := c.get(detail_url, headers)
		if fetch_err != nil {
			attempt_errors = append(attempt_errors, fmt.Sprintf("%s: %v", web_id, fetch_err))
			continue
		}
		iteminfo_data, decode_err := decode_douyin_pc_json_object(body)
		if decode_err != nil {
			attempt_errors = append(attempt_errors, fmt.Sprintf("%s: decode iteminfo JSON: %v", web_id, decode_err))
			continue
		}
		detail, normalize_err := normalize_douyin_pc_iteminfo_detail(
			router_data,
			video_page_data,
			iteminfo_data,
			web_id,
		)
		if normalize_err != nil {
			attempt_errors = append(attempt_errors, fmt.Sprintf("%s: %v", web_id, normalize_err))
			continue
		}
		if strings.Contains(page_url, "/share/slides/") {
			if enrich_err := c.enrich_douyin_pc_live_photo_images(
				page_url,
				detail,
				video_page_data,
			); enrich_err != nil {
				c.logger.Warn().
					Err(enrich_err).
					Str("page_url", page_url).
					Msg("douyin PC: live photo image videos unavailable")
			}
		}
		return detail, nil
	}

	return nil, fmt.Errorf("douyin PC: mobile iteminfo failed after retries: %s", strings.Join(attempt_errors, " | "))
}

// enrich_douyin_pc_live_photo_images fetches the slides-specific response.
// The regular iteminfo endpoint returns the still images but omits the
// per-image motion video nested under images[].video.
func (c *DouyinPCClient) enrich_douyin_pc_live_photo_images(
	page_url string,
	detail map[string]any,
	video_page_data map[string]any,
) error {
	item_id := douyin_pc_scalar_string(douyin_pc_item_id(video_page_data))
	if item_id == "" {
		return fmt.Errorf("slides page has no aweme ID")
	}
	query, _ := douyin_pc_object(video_page_data["query"])
	params := url.Values{}
	params.Set("aweme_ids", "["+item_id+"]")
	params.Set("aweme_type", douyin_pc_value_or_default(query["aweme_type"], "2"))
	params.Set("aid", douyin_pc_value_or_default(query["from_aid"], "6383"))
	params.Set("request_source", "200")

	headers := map[string]string{
		"User-Agent":      douyin_pc_mobile_user_agent,
		"Accept":          "application/json, text/plain, */*",
		"Accept-Language": "zh-CN,zh;q=0.9",
		"Referer":         page_url,
	}
	body, _, err := c.get(douyin_pc_slidesinfo_url+"?"+params.Encode(), headers)
	if err != nil {
		return fmt.Errorf("fetch slidesinfo: %w", err)
	}
	response, err := decode_douyin_pc_json_object(body)
	if err != nil {
		return fmt.Errorf("decode slidesinfo: %w", err)
	}
	details, ok := douyin_pc_array(response["aweme_details"])
	if !ok || len(details) == 0 {
		return fmt.Errorf("slidesinfo did not contain aweme_details")
	}
	slides_detail, ok := douyin_pc_object(details[0])
	if !ok {
		return fmt.Errorf("slidesinfo aweme_details[0] was not an object")
	}
	slides_images, ok := douyin_pc_array(slides_detail["images"])
	if !ok || len(slides_images) == 0 {
		return fmt.Errorf("slidesinfo did not contain images")
	}
	return merge_douyin_pc_live_photo_images(detail, slides_images)
}

func merge_douyin_pc_live_photo_images(detail map[string]any, slides_images []any) error {
	if len(slides_images) == 0 {
		return fmt.Errorf("slidesinfo did not contain images")
	}
	aweme_detail, ok := douyin_pc_object(detail["aweme_detail"])
	if !ok {
		return fmt.Errorf("normalized detail did not contain aweme_detail")
	}
	item_images, ok := douyin_pc_array(aweme_detail["images"])
	if !ok || len(item_images) == 0 {
		return fmt.Errorf("normalized detail did not contain images")
	}
	item_images_by_uri := make(map[string]map[string]any, len(item_images))
	for _, raw_item_image := range item_images {
		item_image, ok := douyin_pc_object(raw_item_image)
		if !ok {
			continue
		}
		if uri := douyin_pc_scalar_string(item_image["uri"]); uri != "" {
			item_images_by_uri[uri] = item_image
		}
	}
	for index, raw_slide_image := range slides_images {
		slide_image, ok := douyin_pc_object(raw_slide_image)
		if !ok {
			continue
		}
		motion_video, ok := douyin_pc_object(slide_image["video"])
		if !ok || len(motion_video) == 0 {
			continue
		}
		var item_image map[string]any
		if uri := douyin_pc_scalar_string(slide_image["uri"]); uri != "" {
			item_image = item_images_by_uri[uri]
		}
		if item_image == nil && index < len(item_images) {
			item_image, _ = douyin_pc_object(item_images[index])
		}
		if item_image == nil {
			continue
		}
		item_image["video"] = motion_video
		item_image["live_photo_type"] = slide_image["live_photo_type"]
	}
	aweme_detail["images"] = item_images
	return nil
}

func build_douyin_pc_iteminfo_params(page_html []byte, video_page_data map[string]any, web_id string) (url.Values, error) {
	item_id := douyin_pc_item_id(video_page_data)
	if !douyin_pc_truthy(item_id) {
		return nil, fmt.Errorf("SSR video page data did not contain itemId")
	}

	xs_token, err := resolve_douyin_pc_xs_token(page_html, web_id)
	if err != nil {
		return nil, err
	}
	reflow_id, err := encrypt_douyin_pc_reflow_id(xs_token, web_id)
	if err != nil {
		return nil, err
	}

	query, _ := douyin_pc_object(video_page_data["query"])
	ab_params, _ := douyin_pc_object(video_page_data["abParams"])
	select_pool_data, _ := douyin_pc_object(ab_params["select_pool_data"])

	params := url.Values{}
	params.Set("reflow_source", douyin_pc_reflow_source)
	params.Set("web_id", web_id)
	params.Set("device_id", web_id)
	params.Set("aid", douyin_pc_value_or_default(query["from_aid"], douyin_pc_default_aid))
	set_douyin_pc_query_value(params, "from_did", query["did"])
	set_douyin_pc_query_value(params, "user_cip", extract_douyin_pc_element_attribute(page_html, "douyin_reflow_webId", "usercip"))
	params.Set("use_new_select_scope", douyin_pc_value_or_default(select_pool_data["use_new_select_scope"], "0"))
	set_douyin_pc_query_value(params, "item_ids", item_id)
	set_douyin_pc_query_value(params, "aweme_type", query["aweme_type"])
	params.Set("reflow_id", reflow_id)
	set_douyin_pc_query_value(params, "share_scene", query["share_scene"])
	set_douyin_pc_query_value(params, "share_token", query["share_token"])
	set_douyin_pc_query_value(params, "scene_from", query["scene_from"])
	if douyin_pc_scalar_string(ab_params["search_video_mark_abtest"]) == "1" {
		params.Set("reflow_logo_type", "aweme_search_suffix_nickname")
	}
	return params, nil
}

func normalize_douyin_pc_iteminfo_detail(
	router_data map[string]any,
	video_page_data map[string]any,
	iteminfo_data map[string]any,
	web_id string,
) (map[string]any, error) {
	item_list, ok := douyin_pc_array(iteminfo_data["item_list"])
	if !ok || len(item_list) == 0 {
		return nil, fmt.Errorf(
			"mobile iteminfo did not contain item_list (status_code=%v, status_msg=%v)",
			iteminfo_data["status_code"],
			iteminfo_data["status_msg"],
		)
	}
	aweme, ok := douyin_pc_object(item_list[0])
	if !ok {
		return nil, fmt.Errorf("mobile iteminfo item_list[0] was not an object")
	}
	video, ok := douyin_pc_object(aweme["video"])
	if !ok || len(video) == 0 {
		return nil, fmt.Errorf("mobile iteminfo item_list[0] did not contain video info")
	}
	aweme = copy_douyin_pc_object(aweme)
	set_douyin_pc_preview_title(aweme)
	if !douyin_pc_truthy(aweme["preview_title"]) {
		return nil, fmt.Errorf("mobile iteminfo item_list[0] did not contain preview_title/desc")
	}

	return map[string]any{
		"status_code":  iteminfo_data["status_code"],
		"status_msg":   iteminfo_data["status_msg"],
		"aweme_detail": aweme,
		"extra":        iteminfo_data["extra"],
		"filter_list":  iteminfo_data["filter_list"],
		"_source":      "mobile_iteminfo",
		"_web_id":      web_id,
		"_item_id":     douyin_pc_item_id(video_page_data),
		"_router_data": router_data,
	}, nil
}

func douyin_pc_web_id_candidates(page_html []byte, video_page_data map[string]any) []string {
	common_context, _ := douyin_pc_object(video_page_data["commonContext"])
	candidates := []string{
		extract_douyin_pc_element_attribute(page_html, "douyin_reflow_webId", "webId"),
		douyin_pc_scalar_string(video_page_data["webId"]),
		douyin_pc_scalar_string(common_context["webId"]),
	}

	web_ids := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if !valid_douyin_pc_web_id(candidate) {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		web_ids = append(web_ids, candidate)
	}
	return web_ids
}

func valid_douyin_pc_web_id(value string) bool {
	if len(value) < 16 {
		return false
	}
	return douyin_pc_digits(value)
}

func douyin_pc_digits(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range []byte(value) {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func resolve_douyin_pc_xs_token(page_html []byte, web_id string) (string, error) {
	token_element_id := douyin_pc_default_token_id
	new_token_element_id := token_element_id
	new_key_ratio := 0

	raw_tcc_config := extract_douyin_pc_element_attribute(page_html, "douyin_reflow_tcc", "tccconfig")
	if raw_tcc_config != "" {
		if tcc_config, err := decode_douyin_pc_json_object([]byte(raw_tcc_config)); err == nil {
			if token_config, ok := douyin_pc_object(tcc_config["token_encry_cooperation"]); ok {
				if old_key := douyin_pc_scalar_string(token_config["fe_key"]); old_key != "" {
					token_element_id = old_key
				}
				new_token_element_id = douyin_pc_scalar_string(token_config["new_fe_key"])
				if new_token_element_id == "" {
					new_token_element_id = token_element_id
				}
				new_key_ratio = douyin_pc_int(token_config["new_fe_key_ratio"])
			}
		}
	}

	if suffix_value, err := strconv.Atoi(web_id[len(web_id)-3:]); err == nil && suffix_value < new_key_ratio {
		token_element_id = new_token_element_id
	}
	xs_token := extract_douyin_pc_element_attribute(page_html, token_element_id, "xsstoken")
	if xs_token == "" && token_element_id != douyin_pc_default_token_id {
		xs_token = extract_douyin_pc_element_attribute(page_html, douyin_pc_default_token_id, "xsstoken")
	}
	if xs_token == "" {
		return "", fmt.Errorf("SSR HTML did not contain douyin reflow xsstoken")
	}
	return xs_token, nil
}

func encrypt_douyin_pc_reflow_id(xs_token string, web_id string) (string, error) {
	if len(web_id) < aes.BlockSize {
		return "", fmt.Errorf("web_id was too short to build AES key: %s", web_id)
	}
	key := []byte(web_id[:aes.BlockSize])
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create AES cipher: %w", err)
	}

	plaintext := []byte(xs_token)
	padding_size := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded_plaintext := make([]byte, len(plaintext)+padding_size)
	copy(padded_plaintext, plaintext)
	for index := len(plaintext); index < len(padded_plaintext); index++ {
		padded_plaintext[index] = byte(padding_size)
	}

	ciphertext := make([]byte, len(padded_plaintext))
	cipher.NewCBCEncrypter(block, key).CryptBlocks(ciphertext, padded_plaintext)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func extract_douyin_pc_element_attribute(page_html []byte, element_id string, attribute_name string) string {
	tokenizer := xhtml.NewTokenizer(bytes.NewReader(page_html))
	for {
		token_type := tokenizer.Next()
		switch token_type {
		case xhtml.ErrorToken:
			return ""
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			token := tokenizer.Token()
			matched_id := false
			attribute_value := ""
			for _, attribute := range token.Attr {
				if strings.EqualFold(attribute.Key, "id") && attribute.Val == element_id {
					matched_id = true
				}
				if strings.EqualFold(attribute.Key, attribute_name) {
					attribute_value = attribute.Val
				}
			}
			if matched_id {
				return attribute_value
			}
		}
	}
}

func douyin_pc_item_id(video_page_data map[string]any) any {
	if douyin_pc_truthy(video_page_data["itemId"]) {
		return video_page_data["itemId"]
	}
	return video_page_data["lastPath"]
}

func set_douyin_pc_preview_title(aweme map[string]any) {
	if !douyin_pc_truthy(aweme["preview_title"]) && douyin_pc_truthy(aweme["desc"]) {
		aweme["preview_title"] = aweme["desc"]
	}
}

func set_douyin_pc_query_value(params url.Values, key string, value any) {
	if value == nil {
		return
	}
	params.Set(key, douyin_pc_scalar_string(value))
}

func douyin_pc_value_or_default(value any, default_value string) string {
	if !douyin_pc_truthy(value) {
		return default_value
	}
	return douyin_pc_scalar_string(value)
}

func douyin_pc_scalar_string(value any) string {
	switch typed_value := value.(type) {
	case string:
		return typed_value
	case json.Number:
		return typed_value.String()
	case float64:
		return strconv.FormatFloat(typed_value, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed_value), 'f', -1, 32)
	case int:
		return strconv.Itoa(typed_value)
	case int64:
		return strconv.FormatInt(typed_value, 10)
	case bool:
		return strconv.FormatBool(typed_value)
	default:
		return ""
	}
}

func douyin_pc_int(value any) int {
	integer, _ := strconv.Atoi(douyin_pc_scalar_string(value))
	return integer
}

func douyin_pc_truthy(value any) bool {
	switch typed_value := value.(type) {
	case nil:
		return false
	case string:
		return typed_value != ""
	case bool:
		return typed_value
	case json.Number:
		return typed_value.String() != "0" && typed_value.String() != "0.0"
	case float64:
		return typed_value != 0
	case float32:
		return typed_value != 0
	case int:
		return typed_value != 0
	case int64:
		return typed_value != 0
	case []any:
		return len(typed_value) > 0
	case map[string]any:
		return len(typed_value) > 0
	default:
		return true
	}
}

func douyin_pc_object(value any) (map[string]any, bool) {
	object, ok := value.(map[string]any)
	return object, ok && object != nil
}

func douyin_pc_array(value any) ([]any, bool) {
	array, ok := value.([]any)
	return array, ok && array != nil
}

func copy_douyin_pc_object(value map[string]any) map[string]any {
	copied_value := make(map[string]any, len(value)+1)
	for key, item := range value {
		copied_value[key] = item
	}
	return copied_value
}
