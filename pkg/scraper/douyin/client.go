package douyin

import (
	"fmt"
	"strings"
)

// Client is the Douyin video scraper client.
// Internally prefers DouyinMobile (no cookie required), falling back to DouyinWeb (requires cookie).
type Client struct {
	mobile *DouyinMobileClient
	web    *DouyinWebClient
}

// NewClient creates a new Douyin client.
// cookie is used for web API calls; mobile does not need it.
func NewClient(cookie string) *Client {
	return &Client{
		mobile: NewDouyinMobileClient(),
		web:    NewDouyinWebClient(cookie),
	}
}

// GetVideoInfo retrieves video information.
// Tries mobile scraping first, falls back to web API.
func (c *Client) GetVideoInfo(rawURL string) (*VideoInfo, error) {
	// Try mobile first (no cookie required)
	info, mobileErr := c.mobile.Parse(rawURL)
	if mobileErr == nil {
		info.Source = "mobile"
		return info, nil
	}

	// Mobile failed, try web
	videoID, extractErr := c.web.ExtraVideoId(rawURL)
	if extractErr != nil {
		return nil, fmt.Errorf("douyin: both methods failed: mobile=%v, web_extract=%v", mobileErr, extractErr)
	}

	resp, fetchErr := c.web.FetchVideoProfile(videoID)
	if fetchErr != nil {
		return nil, fmt.Errorf("douyin: mobile failed, web fetch failed: %v", fetchErr)
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
