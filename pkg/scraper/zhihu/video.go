package zhihu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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

func (c *Client) FetchVideoPlayInfo(content_id string, content_type string, video_id string, scene_code string, referer string) (*VideoPlayInfo, error) {
	content_id = strings.TrimSpace(content_id)
	content_type = strings.TrimSpace(content_type)
	video_id = strings.TrimSpace(video_id)
	scene_code = strings.TrimSpace(scene_code)
	if content_id == "" || content_type == "" || video_id == "" || scene_code == "" {
		return nil, fmt.Errorf("zhihu video play info request is incomplete")
	}

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
