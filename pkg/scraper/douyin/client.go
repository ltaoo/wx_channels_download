package douyin

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Client is the Douyin video scraper client.
// Fetch prefers raw mobile data and falls back to the structured web API.
type Client struct {
	mobile *DouyinMobileClient
	web    *DouyinWebClient
	logger zerolog.Logger
}

// NewClient creates a new Douyin client.
// cookie is used for web API calls; mobile does not need it.
func NewClient(cookie string) *Client {
	return NewClientWithLogger(cookie, nil)
}

// NewClientWithLogger creates a Douyin client with structured diagnostics.
func NewClientWithLogger(cookie string, parent_logger *zerolog.Logger) *Client {
	logger := new_component_logger(parent_logger, "douyin_scraper")
	return &Client{
		mobile: NewDouyinMobileClientWithLogger(parent_logger),
		web:    NewDouyinWebClientWithLogger(cookie, parent_logger),
		logger: logger,
	}
}

// Fetch retrieves raw mobile JSON, falling back to structured web API data.
func (c *Client) Fetch(raw_url string) (any, error) {
	started_at := time.Now()
	c.logger.Info().
		Int("input_length", len(raw_url)).
		Str("input_preview", log_text_preview(raw_url)).
		Msg("douyin fetch: started")

	douyin_url, extract_url_err := ExtractURL(raw_url)
	if extract_url_err != nil {
		c.logger.Error().
			Err(extract_url_err).
			Dur("elapsed", time.Since(started_at)).
			Msg("douyin fetch: URL extraction failed")
		return nil, fmt.Errorf("douyin: extract URL: %w", extract_url_err)
	}
	c.logger.Info().
		Str("douyin_url", douyin_url).
		Msg("douyin fetch: URL extracted")

	// Try mobile first (no cookie required)
	mobile_started_at := time.Now()
	router_json, mobile_err := c.mobile.Parse(douyin_url)
	if mobile_err == nil {
		c.logger.Info().
			Int("router_data_bytes", len(router_json)).
			Dur("mobile_elapsed", time.Since(mobile_started_at)).
			Dur("elapsed", time.Since(started_at)).
			Msg("douyin fetch: completed with raw mobile JSON")
		return router_json, nil
	}
	c.logger.Warn().
		Err(mobile_err).
		Dur("mobile_elapsed", time.Since(mobile_started_at)).
		Msg("douyin fetch: mobile fetch failed, falling back to web API")

	// Mobile failed, try web
	web_started_at := time.Now()
	video_id, extract_err := c.web.ExtraVideoId(douyin_url)
	if extract_err != nil {
		c.logger.Error().
			Err(extract_err).
			Errs("previous_errors", []error{mobile_err}).
			Dur("web_elapsed", time.Since(web_started_at)).
			Dur("elapsed", time.Since(started_at)).
			Msg("douyin fetch: web video ID extraction failed")
		return nil, fmt.Errorf("douyin: both methods failed: mobile=%v, web_extract=%v", mobile_err, extract_err)
	}
	c.logger.Info().
		Str("video_id", video_id).
		Msg("douyin fetch: web video ID resolved")

	resp, fetch_err := c.web.FetchVideoProfile(video_id)
	if fetch_err != nil {
		c.logger.Error().
			Err(fetch_err).
			Errs("previous_errors", []error{mobile_err}).
			Str("video_id", video_id).
			Dur("web_elapsed", time.Since(web_started_at)).
			Dur("elapsed", time.Since(started_at)).
			Msg("douyin fetch: web API failed")
		return nil, fmt.Errorf("douyin: mobile failed: %v; web fetch failed: %w", mobile_err, fetch_err)
	}

	if resp.StatusCode != 0 {
		c.logger.Error().
			Int("douyin_status_code", resp.StatusCode).
			Str("video_id", video_id).
			Dur("web_elapsed", time.Since(web_started_at)).
			Dur("elapsed", time.Since(started_at)).
			Msg("douyin fetch: web API returned an application error")
		return nil, fmt.Errorf("douyin: mobile failed: %v; web API returned status_code=%d", mobile_err, resp.StatusCode)
	}

	info := convertWebResp(resp)
	c.logger.Info().
		Str("video_id", info.VideoID).
		Dur("web_elapsed", time.Since(web_started_at)).
		Dur("elapsed", time.Since(started_at)).
		Msg("douyin fetch: completed with web API")
	return info, nil
}

// GetVideoInfo retrieves structured video information from the web API.
func (c *Client) GetVideoInfo(raw_url string) (*VideoInfo, error) {
	douyin_url, err := ExtractURL(raw_url)
	if err != nil {
		return nil, fmt.Errorf("douyin: extract URL: %w", err)
	}
	video_id, err := c.web.ExtraVideoId(douyin_url)
	if err != nil {
		return nil, fmt.Errorf("douyin: extract video id: %w", err)
	}
	resp, err := c.web.FetchVideoProfile(video_id)
	if err != nil {
		return nil, fmt.Errorf("douyin: web fetch failed: %w", err)
	}
	if resp.StatusCode != 0 {
		return nil, fmt.Errorf("douyin: web API returned status_code=%d", resp.StatusCode)
	}
	return convertWebResp(resp), nil
}

// convertWebResp converts web API response to VideoInfo.
func convertWebResp(resp *DouyinWebVideoProfileResp) *VideoInfo {
	detail := resp.AwemeDetail
	video := detail.Video

	// Choose best video URL: prefer H264 playback address
	var videoURL string
	if len(video.PlayAddrH264.UrlList) > 0 {
		videoURL = video.PlayAddrH264.UrlList[0]
	} else if len(video.PlayAddr265.UrlList) > 0 {
		videoURL = video.PlayAddr265.UrlList[0]
	} else if len(video.PlayAddr.UrlList) > 0 {
		videoURL = video.PlayAddr.UrlList[0]
	} else {
		// Try bitrate list
		for _, br := range video.BitRate {
			if len(br.PlayAddr.UrlList) > 0 {
				videoURL = br.PlayAddr.UrlList[0]
				break
			}
		}
	}

	// Replace watermarked URL with non-watermarked URL
	videoURL = strings.Replace(videoURL, "playwm", "play", 1)

	// Cover URL
	var coverURL string
	if len(video.OriginCover.UrlList) > 0 {
		coverURL = video.OriginCover.UrlList[0]
	} else if len(video.Cover.UrlList) > 0 {
		coverURL = video.Cover.UrlList[0]
	}

	title := sanitizeFilename(detail.Desc)
	if title == "" {
		title = fmt.Sprintf("douyin_%s", detail.AwemeId)
	}

	return &VideoInfo{
		URL:      videoURL,
		Title:    title,
		VideoID:  detail.AwemeId,
		CoverURL: coverURL,
		Source:   "web",
	}
}
