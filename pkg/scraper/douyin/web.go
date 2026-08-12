package douyin

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// DouyinWebClient is the Douyin web scraper (fetches via API, requires cookie).
type DouyinWebClient struct {
	cookie string
	logger zerolog.Logger
}

// NewDouyinWebClient creates a new Douyin web scraper.
func NewDouyinWebClient(cookie string) *DouyinWebClient {
	return NewDouyinWebClientWithLogger(cookie, nil)
}

// NewDouyinWebClientWithLogger creates a web scraper with diagnostics.
func NewDouyinWebClientWithLogger(cookie string, parent_logger *zerolog.Logger) *DouyinWebClient {
	return &DouyinWebClient{
		cookie: cookie,
		logger: new_component_logger(parent_logger, "douyin_web"),
	}
}

// FetchVideoProfile retrieves video details by aweme_id.
func (c *DouyinWebClient) FetchVideoProfile(aweme_id string) (*DouyinWebVideoProfileResp, error) {
	started_at := time.Now()
	c.logger.Info().
		Str("video_id", aweme_id).
		Bool("cookie_configured", strings.TrimSpace(c.cookie) != "").
		Msg("douyin web: API request started")

	params := make(map[string]string)
	for k, v := range defaultParams {
		params[k] = v
	}
	params["aweme_id"] = aweme_id
	params["msToken"] = ""

	ab := NewABogus("")
	a_bogus := ab.GetValue(params, paramOrder, "GET", 0, 0, nil, nil, nil)

	headers := map[string]interface{}{
		"Accept-Language": "zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2",
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.212 Safari/537.36",
		"Referer":         "https://www.douyin.com/",
		"Cookie":          c.cookie,
	}

	search := queryStringify(params, paramOrder) + "&a_bogus=" + a_bogus
	api_url := "https://www.douyin.com/aweme/v1/web/aweme/detail/?" + search

	client := NewHttpClient("GET", api_url, map[string]string{}, headers)
	resp, err := client.Request()
	if err != nil {
		c.logger.Error().
			Err(err).
			Str("video_id", aweme_id).
			Dur("elapsed", time.Since(started_at)).
			Msg("douyin web: API HTTP request failed")
		return nil, err
	}
	c.logger.Info().
		Str("video_id", aweme_id).
		Int("http_status", resp.status_code).
		Str("content_type", resp.content_type).
		Str("content_encoding", resp.content_encoding).
		Int64("content_length", resp.content_length).
		Int("body_bytes", len(resp.body)).
		Str("response_url", resp.request_url).
		Dur("elapsed", time.Since(started_at)).
		Msg("douyin web: API response received")
	if resp.status_code < http.StatusOK || resp.status_code >= http.StatusMultipleChoices {
		c.logger.Error().
			Str("video_id", aweme_id).
			Int("http_status", resp.status_code).
			Str("content_type", resp.content_type).
			Str("content_encoding", resp.content_encoding).
			Int64("content_length", resp.content_length).
			Int("body_bytes", len(resp.body)).
			Str("body_preview", log_body_preview(resp.body)).
			Msg("douyin web: API returned unexpected HTTP status")
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.status_code)
	}

	var result DouyinWebVideoProfileResp
	if err := resp.ToJSON(&result); err != nil {
		c.logger.Error().
			Err(err).
			Str("video_id", aweme_id).
			Int("http_status", resp.status_code).
			Str("content_type", resp.content_type).
			Str("content_encoding", resp.content_encoding).
			Int64("content_length", resp.content_length).
			Int("body_bytes", len(resp.body)).
			Str("body_preview", log_body_preview(resp.body)).
			Msg("douyin web: API JSON decode failed")
		return nil, err
	}
	c.logger.Info().
		Str("video_id", aweme_id).
		Int("douyin_status_code", result.StatusCode).
		Dur("elapsed", time.Since(started_at)).
		Msg("douyin web: API response decoded")
	return &result, nil
}

// ExtraVideoId extracts the video ID from text or URL.
func (c *DouyinWebClient) ExtraVideoId(content string) (string, error) {
	started_at := time.Now()
	douyin_url, err := ExtractURL(content)
	if err != nil {
		c.logger.Error().Err(err).Msg("douyin web: URL extraction failed")
		return "", err
	}

	parsed_url, err := url.Parse(douyin_url)
	if err != nil {
		c.logger.Error().Err(err).Str("url", douyin_url).Msg("douyin web: URL parse failed")
		return "", err
	}
	if strings.EqualFold(parsed_url.Hostname(), "v.douyin.com") {
		c.logger.Info().Str("short_url", douyin_url).Msg("douyin web: resolving short URL")
		return c.ShortLinkToFullURL(douyin_url)
	}

	video_path_pattern := regexp.MustCompile(`/(?:video|share/video)/(\d+)`)
	matched := video_path_pattern.FindStringSubmatch(parsed_url.Path)
	if len(matched) > 1 {
		c.logger.Info().
			Str("video_id", matched[1]).
			Dur("elapsed", time.Since(started_at)).
			Msg("douyin web: video ID extracted from URL")
		return matched[1], nil
	}

	extract_err := fmt.Errorf("failed to extract video id from URL")
	c.logger.Error().
		Err(extract_err).
		Str("url", douyin_url).
		Dur("elapsed", time.Since(started_at)).
		Msg("douyin web: video ID extraction failed")
	return "", extract_err
}

// ShortLinkToFullURL converts a short link to a video ID.
func (c *DouyinWebClient) ShortLinkToFullURL(short_link string) (string, error) {
	started_at := time.Now()
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Head(short_link)
	if err != nil {
		c.logger.Error().
			Err(err).
			Str("short_url", short_link).
			Dur("elapsed", time.Since(started_at)).
			Msg("douyin web: short URL request failed")
		return "", err
	}
	defer resp.Body.Close()

	location := resp.Header.Get("Location")
	c.logger.Info().
		Str("short_url", short_link).
		Int("http_status", resp.StatusCode).
		Str("location", location).
		Dur("elapsed", time.Since(started_at)).
		Msg("douyin web: short URL response received")
	if resp.StatusCode >= http.StatusMultipleChoices && resp.StatusCode < http.StatusBadRequest {
		if location == "" {
			redirect_err := fmt.Errorf("redirect location is empty")
			c.logger.Error().
				Err(redirect_err).
				Str("short_url", short_link).
				Int("http_status", resp.StatusCode).
				Msg("douyin web: short URL redirect is invalid")
			return "", redirect_err
		}
		parsed_url, err := url.Parse(location)
		if err != nil {
			c.logger.Error().
				Err(err).
				Str("location", location).
				Msg("douyin web: redirect URL parse failed")
			return "", err
		}
		path := parsed_url.Path
		re := regexp.MustCompile(`/(\d+)/?$`)
		matches := re.FindStringSubmatch(path)
		if len(matches) > 1 {
			c.logger.Info().
				Str("video_id", matches[1]).
				Msg("douyin web: video ID resolved from short URL")
			return matches[1], nil
		}
		extract_err := fmt.Errorf("video id not found in redirect URL")
		c.logger.Error().
			Err(extract_err).
			Str("location", location).
			Msg("douyin web: redirect URL has no video ID")
		return "", extract_err
	}
	status_err := fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	c.logger.Error().
		Err(status_err).
		Str("short_url", short_link).
		Int("http_status", resp.StatusCode).
		Msg("douyin web: short URL returned unexpected status")
	return "", status_err
}
