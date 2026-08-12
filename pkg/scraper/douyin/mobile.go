package douyin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/rs/zerolog"
)

// DouyinMobileClient is the Douyin mobile scraper.
type DouyinMobileClient struct {
	logger zerolog.Logger
}

var router_pattern = regexp.MustCompile(`(?s)window\._ROUTER_DATA\s*=\s*(.*?)</script>`)

// NewDouyinMobileClient creates a new Douyin mobile scraper.
func NewDouyinMobileClient() *DouyinMobileClient {
	return NewDouyinMobileClientWithLogger(nil)
}

// NewDouyinMobileClientWithLogger creates a mobile scraper with diagnostics.
func NewDouyinMobileClientWithLogger(parent_logger *zerolog.Logger) *DouyinMobileClient {
	return &DouyinMobileClient{
		logger: new_component_logger(parent_logger, "douyin_mobile"),
	}
}

// Parse resolves a Douyin share link and returns its raw _ROUTER_DATA JSON.
// Supports v.douyin.com short links and iesdouyin.com links.
func (c *DouyinMobileClient) Parse(raw_url string) (json.RawMessage, error) {
	started_at := time.Now()
	c.logger.Info().Str("url", raw_url).Msg("douyin mobile: started")

	if !canParse(raw_url) {
		parse_err := fmt.Errorf("不支持的URL: %s", raw_url)
		c.logger.Error().Err(parse_err).Msg("douyin mobile: URL rejected")
		return nil, parse_err
	}

	ua := userAgents[len(userAgents)-1]

	final_url, err := resolveRedirects(raw_url, ua, &c.logger)
	if err != nil {
		c.logger.Error().
			Err(err).
			Dur("elapsed", time.Since(started_at)).
			Msg("douyin mobile: redirect resolution failed")
		return nil, fmt.Errorf("解析重定向失败: %v", err)
	}
	c.logger.Info().
		Str("final_url", final_url).
		Msg("douyin mobile: redirects resolved")

	video_id := parseVideoID(final_url)
	iesd_url := fmt.Sprintf("https://www.iesdouyin.com/share/video/%s", video_id)

	req, err := http.NewRequest("GET", iesd_url, nil)
	if err != nil {
		c.logger.Error().
			Err(err).
			Str("page_url", iesd_url).
			Msg("douyin mobile: page request construction failed")
		return nil, fmt.Errorf("构造页面请求失败: %v", err)
	}
	req.Header.Set("User-Agent", ua)

	request_started_at := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.logger.Error().
			Err(err).
			Str("page_url", iesd_url).
			Dur("request_elapsed", time.Since(request_started_at)).
			Msg("douyin mobile: page request failed")
		return nil, fmt.Errorf("请求页面失败: %v", err)
	}
	defer resp.Body.Close()

	html, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error().
			Err(err).
			Int("http_status", resp.StatusCode).
			Str("content_type", resp.Header.Get("Content-Type")).
			Str("content_encoding", resp.Header.Get("Content-Encoding")).
			Int64("content_length", resp.ContentLength).
			Str("response_url", resp.Request.URL.String()).
			Msg("douyin mobile: response body read failed")
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}
	c.logger.Info().
		Int("http_status", resp.StatusCode).
		Str("content_type", resp.Header.Get("Content-Type")).
		Str("content_encoding", resp.Header.Get("Content-Encoding")).
		Int64("content_length", resp.ContentLength).
		Int("body_bytes", len(html)).
		Str("response_url", resp.Request.URL.String()).
		Dur("request_elapsed", time.Since(request_started_at)).
		Msg("douyin mobile: page response received")
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		c.logger.Error().
			Int("http_status", resp.StatusCode).
			Str("content_type", resp.Header.Get("Content-Type")).
			Str("content_encoding", resp.Header.Get("Content-Encoding")).
			Int64("content_length", resp.ContentLength).
			Int("body_bytes", len(html)).
			Str("body_preview", log_body_preview(html)).
			Msg("douyin mobile: page returned unexpected HTTP status")
		return nil, fmt.Errorf("页面返回异常状态码: %d", resp.StatusCode)
	}

	router_json, err := extract_router_json(html)
	if err != nil {
		c.logger.Error().
			Int("http_status", resp.StatusCode).
			Str("content_type", resp.Header.Get("Content-Type")).
			Str("content_encoding", resp.Header.Get("Content-Encoding")).
			Int64("content_length", resp.ContentLength).
			Int("body_bytes", len(html)).
			Str("body_preview", log_body_preview(html)).
			Msg("douyin mobile: _ROUTER_DATA not found")
		return nil, err
	}
	c.logger.Info().
		Str("video_id", video_id).
		Int("router_data_bytes", len(router_json)).
		Dur("elapsed", time.Since(started_at)).
		Msg("douyin mobile: raw router JSON fetched")

	return router_json, nil
}

func extract_router_json(html []byte) (json.RawMessage, error) {
	matches := router_pattern.FindSubmatch(html)
	if len(matches) < 2 {
		return nil, fmt.Errorf("未找到_ROUTER_DATA")
	}
	return json.RawMessage(bytes.TrimSpace(matches[1])), nil
}
