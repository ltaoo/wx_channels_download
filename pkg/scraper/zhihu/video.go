package zhihu

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"wx_channel/pkg/minib"
)

const video_play_info_endpoint = "https://www.zhihu.com/api/v4/video/play_info"

type VideoPlayInfo struct {
	ZA        VideoPlayZA     `json:"za"`
	VideoPlay VideoPlay       `json:"video_play"`
	UIConfig  json.RawMessage `json:"ui_config"`
	Template  json.RawMessage `json:"template"`
}

type VideoPlayZA struct {
	ContentID    string `json:"content_id"`
	ContentType  int    `json:"content_type"`
	ContentToken string `json:"content_token"`
}

type VideoPlay struct {
	ID           string            `json:"id"`
	DefaultCover string            `json:"default_cover"`
	IsPaid       bool              `json:"is_paid"`
	IsTrial      bool              `json:"is_trial"`
	Meta         VideoPlayMeta     `json:"meta"`
	BeginFrame   map[string]string `json:"begin_frame"`
	Playlist     VideoPlayPlaylist `json:"playlist"`
}

type VideoPlayMeta struct {
	MIME       string              `json:"mime"`
	Duration   float64             `json:"duration"`
	Resolution VideoPlayResolution `json:"resolution"`
	HDRType    string              `json:"hdr_type"`
}

type VideoPlayResolution struct {
	Quality string `json:"quality"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

type VideoPlayPlaylist struct {
	MP4 []VideoPlayVariant `json:"mp4"`
}

type VideoPlayVariant struct {
	Key        int      `json:"key"`
	Name       string   `json:"name"`
	Label      string   `json:"label"`
	Type       int      `json:"type"`
	Quality    string   `json:"quality"`
	Format     string   `json:"format"`
	Codec      string   `json:"codec"`
	HDRType    string   `json:"hdr_type"`
	MaxBitrate int      `json:"maxbitrate"`
	Bitrate    int      `json:"bitrate"`
	Duration   float64  `json:"duration"`
	Channels   int      `json:"channels"`
	SampleRate int      `json:"sample_rate"`
	Width      int      `json:"width"`
	Height     int      `json:"height"`
	Size       int64    `json:"size"`
	FPS        int      `json:"fps"`
	URL        []string `json:"url"`
}

type video_play_info_request struct {
	ContentID      string `json:"content_id"`
	ContentTypeStr string `json:"content_type_str"`
	VideoID        string `json:"video_id"`
	SceneCode      string `json:"scene_code"`
	IsOnlyVideo    bool   `json:"is_only_video"`
}

type video_har_archive struct {
	Log struct {
		Entries []struct {
			Request struct {
				Method string `json:"method"`
				URL    string `json:"url"`
			} `json:"request"`
			Response struct {
				Status  int `json:"status"`
				Content struct {
					Text     string `json:"text"`
					Encoding string `json:"encoding"`
				} `json:"content"`
			} `json:"response"`
		} `json:"entries"`
	} `json:"log"`
}

type lens_video_response struct {
	Playlist    map[string]lens_video_variant `json:"playlist"`
	PlaylistV2  map[string]lens_video_variant `json:"playlist_v2"`
	CoverURL    string                        `json:"cover_url"`
	Title       string                        `json:"title"`
	Watermarked int                           `json:"watermarked"`
}

type lens_video_variant struct {
	MaxBitrate float64 `json:"maxbitrate"`
	Format     string  `json:"format"`
	Height     int     `json:"height"`
	Channels   int     `json:"channels"`
	Duration   float64 `json:"duration"`
	Bitrate    float64 `json:"bitrate"`
	Size       int64   `json:"size"`
	PlayURL    string  `json:"play_url"`
	Width      int     `json:"width"`
	SampleRate int     `json:"sample_rate"`
	FPS        float64 `json:"fps"`
}

func (c *Client) FetchVideoPlayInfo(content_id string, content_type string, video_id string, scene_code string, referer string) (*VideoPlayInfo, error) {
	content_id = strings.TrimSpace(content_id)
	content_type = strings.TrimSpace(content_type)
	video_id = strings.TrimSpace(video_id)
	scene_code = strings.TrimSpace(scene_code)
	if content_id == "" || content_type == "" || video_id == "" || scene_code == "" {
		return nil, fmt.Errorf("zhihu video play info request is incomplete")
	}

	info, browser_err := c.fetch_video_play_info_with_minib(content_id, content_type, video_id, referer)
	if browser_err == nil {
		return info, nil
	}
	info, direct_err := c.fetch_video_play_info_direct(content_id, content_type, video_id, scene_code, referer)
	if direct_err == nil {
		return info, nil
	}
	return nil, fmt.Errorf("execute Zhihu video player with minib: %v; legacy video API fallback: %w", browser_err, direct_err)
}

func (c *Client) fetch_video_play_info_with_minib(content_id string, content_type string, video_id string, referer string) (*VideoPlayInfo, error) {
	referer = strings.TrimSpace(referer)
	if referer == "" {
		return nil, fmt.Errorf("Zhihu video player referer is empty")
	}
	if err := c.ensure_pcweb_zse_cookie(referer, content_id, content_type); err != nil {
		return nil, err
	}
	zse_cookie, err := c.read_pcweb_zse_cookie()
	if err != nil {
		return nil, err
	}
	if zse_cookie == "" {
		return nil, fmt.Errorf("Zhihu video player has no __zse_ck after challenge")
	}

	timeout := 120 * time.Second
	if c != nil && c.http_client != nil && c.http_client.Timeout > 0 {
		timeout = c.http_client.Timeout
	}
	browser, err := minib.NewMiniBrowser(timeout)
	if err != nil {
		return nil, fmt.Errorf("create minib for Zhihu video player: %w", err)
	}
	defer browser.Close()
	if err := set_pcweb_zse_cookie(browser, referer, zse_cookie); err != nil {
		return nil, err
	}

	navigation_request, err := http.NewRequest(http.MethodGet, referer, nil)
	if err != nil {
		return nil, err
	}
	set_pcweb_desktop_document_headers(navigation_request, "none", "")
	navigation_ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	page, err := browser.Navigate(navigation_ctx, referer, navigation_request.Header, minib.NavigateOptions{
		CaptureHAR:        true,
		DisableCache:      true,
		DisableCSS:        true,
		DisableImages:     true,
		DisableMedia:      true,
		JavaScriptTimeout: min_duration(10*time.Second, timeout),
		ResourceTimeout:   min_duration(20*time.Second, timeout),
		WaitUntil:         minib.WaitUntilLoad,
		HARMaxBodyBytes:   32 << 20,
	})
	if err != nil {
		return nil, fmt.Errorf("navigate Zhihu video player: %w", err)
	}
	if page.StatusCode < 200 || page.StatusCode >= 300 {
		return nil, fmt.Errorf("Zhihu video player document status %d", page.StatusCode)
	}
	har_data, err := page.HAR()
	if err != nil {
		return nil, err
	}
	info, request_status, err := video_play_info_from_har(har_data, video_id)
	if err == nil {
		return info, nil
	}
	if request_status != 0 {
		return nil, fmt.Errorf("Zhihu video player request status %d: %w", request_status, err)
	}
	if len(page.ScriptFailures) != 0 {
		return nil, fmt.Errorf("Zhihu video player did not request video %s (scripts=%d fetch=%d xhr=%d; first script error: %v)", video_id, page.ExecutedScripts, len(page.FetchRequests), len(page.XHRRequests), page.ScriptFailures[0].Err)
	}
	return nil, fmt.Errorf("Zhihu video player did not request video %s (scripts=%d fetch=%d xhr=%d)", video_id, page.ExecutedScripts, len(page.FetchRequests), len(page.XHRRequests))
}

func (c *Client) ensure_pcweb_zse_cookie(referer string, content_id string, content_type string) error {
	cookie_value, err := c.read_pcweb_zse_cookie()
	if err != nil || cookie_value != "" {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(content_type)) {
	case "article":
		article_url, ok := ParseArticleURL(referer)
		if !ok {
			return fmt.Errorf("invalid Zhihu Article video referer %q", referer)
		}
		if article_url.ArticleID != content_id {
			return fmt.Errorf("Zhihu Article video content id %q does not match referer id %q", content_id, article_url.ArticleID)
		}
		_, err = c.pcweb_desktop_document(article_url.Canonical, article_url.ArticleID, "Article", pcweb_has_article)
	case "answer":
		answer_url, ok := ParseAnswerURL(referer)
		if !ok {
			return fmt.Errorf("invalid Zhihu Answer video referer %q", referer)
		}
		if answer_url.AnswerID != content_id {
			return fmt.Errorf("Zhihu Answer video content id %q does not match referer id %q", content_id, answer_url.AnswerID)
		}
		_, err = c.pcweb_desktop_document(answer_url.Canonical, answer_url.AnswerID, "Answer", pcweb_has_answer)
	default:
		return fmt.Errorf("full Zhihu video navigation is unsupported for content type %q", content_type)
	}
	if err != nil {
		return fmt.Errorf("prepare Zhihu video challenge state: %w", err)
	}
	cookie_value, err = c.read_pcweb_zse_cookie()
	if err != nil {
		return err
	}
	if cookie_value == "" {
		return fmt.Errorf("Zhihu challenge completed without a reusable __zse_ck")
	}
	return nil
}

func video_play_info_from_har(har_data []byte, video_id string) (*VideoPlayInfo, int, error) {
	var archive video_har_archive
	if err := json.Unmarshal(har_data, &archive); err != nil {
		return nil, 0, fmt.Errorf("decode minib HAR: %w", err)
	}
	request_status := 0
	for _, entry := range archive.Log.Entries {
		if !video_play_info_har_url(entry.Request.URL, video_id) {
			continue
		}
		request_status = entry.Response.Status
		if entry.Response.Status < 200 || entry.Response.Status >= 300 {
			continue
		}
		body := []byte(entry.Response.Content.Text)
		if strings.EqualFold(entry.Response.Content.Encoding, "base64") {
			decoded, err := base64.StdEncoding.DecodeString(entry.Response.Content.Text)
			if err != nil {
				return nil, request_status, fmt.Errorf("decode video response body: %w", err)
			}
			body = decoded
		}
		if len(body) == 0 {
			return nil, request_status, fmt.Errorf("minib HAR omitted the video response body")
		}
		var info VideoPlayInfo
		if err := json.Unmarshal(body, &info); err != nil {
			return nil, request_status, fmt.Errorf("decode Zhihu video player response: %w", err)
		}
		if strings.TrimSpace(info.VideoPlay.ID) == "" && strings.EqualFold(parsed_hostname(entry.Request.URL), "lens.zhihu.com") {
			var lens_response lens_video_response
			if err := json.Unmarshal(body, &lens_response); err != nil {
				return nil, request_status, fmt.Errorf("decode Zhihu Lens video response: %w", err)
			}
			info = video_play_info_from_lens(video_id, lens_response)
		}
		if strings.TrimSpace(info.VideoPlay.ID) != video_id {
			return nil, request_status, fmt.Errorf("Zhihu video player returned video %q instead of %q", info.VideoPlay.ID, video_id)
		}
		if len(info.VideoPlay.Playlist.MP4) == 0 {
			return nil, request_status, fmt.Errorf("Zhihu video player returned no MP4 playlist for %q", video_id)
		}
		return &info, request_status, nil
	}
	return nil, request_status, fmt.Errorf("video metadata request is absent from minib HAR")
}

func video_play_info_from_lens(video_id string, response lens_video_response) VideoPlayInfo {
	qualities := make([]string, 0, len(response.Playlist))
	for quality := range response.Playlist {
		qualities = append(qualities, quality)
	}
	sort.SliceStable(qualities, func(left int, right int) bool {
		left_variant := response.Playlist[qualities[left]]
		right_variant := response.Playlist[qualities[right]]
		left_pixels := int64(left_variant.Width) * int64(left_variant.Height)
		right_pixels := int64(right_variant.Width) * int64(right_variant.Height)
		if left_pixels != right_pixels {
			return left_pixels < right_pixels
		}
		return left_variant.Bitrate < right_variant.Bitrate
	})
	variants := make([]VideoPlayVariant, 0, len(qualities))
	best_variant := VideoPlayVariant{}
	for _, quality := range qualities {
		lens_variant := response.Playlist[quality]
		if strings.TrimSpace(lens_variant.PlayURL) == "" {
			continue
		}
		name, label, quality_index := lens_video_quality_labels(quality, lens_variant.Height)
		codec, codec_index := lens_video_codec(lens_variant.PlayURL)
		variant := VideoPlayVariant{
			Key:        codec_index*1000 + quality_index,
			Name:       name,
			Label:      label,
			Quality:    quality,
			Format:     first_non_empty_video_value(lens_variant.Format, "mp4"),
			Codec:      codec,
			HDRType:    "SDR",
			MaxBitrate: int(lens_variant.MaxBitrate),
			Bitrate:    int(lens_variant.Bitrate),
			Duration:   lens_variant.Duration,
			Channels:   lens_variant.Channels,
			SampleRate: lens_variant.SampleRate,
			Width:      lens_variant.Width,
			Height:     lens_variant.Height,
			Size:       lens_variant.Size,
			FPS:        int(lens_variant.FPS),
			URL:        []string{lens_variant.PlayURL},
		}
		variants = append(variants, variant)
		best_variant = variant
	}
	info := VideoPlayInfo{}
	info.VideoPlay.ID = video_id
	info.VideoPlay.DefaultCover = response.CoverURL
	info.VideoPlay.Playlist.MP4 = variants
	if len(variants) != 0 {
		info.VideoPlay.Meta = VideoPlayMeta{
			MIME:     "video/" + best_variant.Format,
			Duration: best_variant.Duration,
			Resolution: VideoPlayResolution{
				Quality: best_variant.Quality,
				Width:   best_variant.Width,
				Height:  best_variant.Height,
			},
			HDRType: best_variant.HDRType,
		}
	}
	return info
}

func lens_video_quality_labels(quality string, height int) (string, string, int) {
	switch strings.ToUpper(strings.TrimSpace(quality)) {
	case "LD":
		return "360P", "流畅 360P", 10
	case "SD":
		return "480P", "标清 480P", 11
	case "HD":
		return "720P", "高清 720P", 12
	case "FHD":
		return "1080P", "超清 1080P", 13
	default:
		name := strings.ToUpper(strings.TrimSpace(quality))
		if height > 0 {
			name = fmt.Sprintf("%dP", height)
		}
		return name, name, 99
	}
}

func lens_video_codec(play_url string) (string, int) {
	parsed_url, err := url.Parse(play_url)
	if err == nil {
		codec_value := strings.ToLower(parsed_url.Query().Get("c"))
		if strings.Contains(codec_value, "hevc") || strings.Contains(codec_value, "h265") {
			return "H265", 21
		}
	}
	return "H264", 20
}

func parsed_hostname(raw_url string) string {
	parsed_url, err := url.Parse(raw_url)
	if err != nil {
		return ""
	}
	return parsed_url.Hostname()
}

func first_non_empty_video_value(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func video_play_info_har_url(raw_url string, video_id string) bool {
	parsed_url, err := url.Parse(raw_url)
	if err != nil {
		return false
	}
	path := strings.TrimRight(parsed_url.EscapedPath(), "/")
	if strings.EqualFold(parsed_url.Hostname(), "lens.zhihu.com") && strings.HasSuffix(path, "/api/v4/videos/"+url.PathEscape(video_id)) {
		return true
	}
	return strings.EqualFold(parsed_url.Hostname(), "www.zhihu.com") &&
		strings.HasSuffix(path, "/api/v4/video/play_info") &&
		strings.HasPrefix(parsed_url.Query().Get("r"), video_id)
}

func min_duration(left time.Duration, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}

func (c *Client) fetch_video_play_info_direct(content_id string, content_type string, video_id string, scene_code string, referer string) (*VideoPlayInfo, error) {
	endpoint, err := url.Parse(video_play_info_endpoint)
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("r", video_id+content_id)
	endpoint.RawQuery = query.Encode()
	request_data, err := json.Marshal(video_play_info_request{
		ContentID:      content_id,
		ContentTypeStr: content_type,
		VideoID:        video_id,
		SceneCode:      scene_code,
		IsOnlyVideo:    true,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint.String(), bytes.NewReader(request_data))
	if err != nil {
		return nil, err
	}
	set_zhihu_video_play_info_headers(req, referer)
	cookie_header := c.cookie(endpoint.String())
	if cookie_header != "" {
		req.Header.Set("Cookie", cookie_header)
	}
	c.log_request(http.MethodPost, endpoint.String(), cookie_header)
	resp, err := c.http_client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	c.log_response(http.MethodPost, endpoint.String(), resp.StatusCode)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("zhihu video play info status %d body=%s", resp.StatusCode, debug_snippet(body))
	}

	var info VideoPlayInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("decode zhihu video play info: %w", err)
	}
	if strings.TrimSpace(info.VideoPlay.ID) == "" {
		return nil, fmt.Errorf("zhihu video play info has no video id")
	}
	return &info, nil
}

func set_zhihu_video_play_info_headers(req *http.Request, referer string) {
	if strings.TrimSpace(referer) == "" {
		referer = SourceURL
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://www.zhihu.com")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", referer)
	req.Header.Set("Sec-CH-UA", `"Not=A?Brand";v="99", "Google Chrome";v="151", "Chromium";v="151"`)
	req.Header.Set("Sec-CH-UA-Mobile", "?0")
	req.Header.Set("Sec-CH-UA-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", zhihu_user_agent)
	req.Header.Set("X-API-Version", "3.0.91")
	req.Header.Set("X-App-Za", "OS=Web")
	req.Header.Set("X-Requested-With", "fetch")
}

func VideoDownloadHeaders(referer string) map[string]string {
	headers := map[string]string{
		"Accept":          "video/mp4,video/webm,video/*,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"Sec-Fetch-Dest":  "video",
		"Sec-Fetch-Mode":  "no-cors",
		"User-Agent":      zhihu_user_agent,
	}
	if referer = strings.TrimSpace(referer); referer != "" {
		headers["Referer"] = referer
	}
	return headers
}
