// Adapted from the local dramadl project; see ../NOTICE and ../LICENSE.
package youku

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"wx_channel/pkg/scraper"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

func (client *backend) youku_video(ctx context.Context, id string) (scraper.Video, error) {
	original_cookie := client.Cookie
	defer func() { client.Cookie = original_cookie }()
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	if sourceutil.CookieValue(client.Cookie, "__ysuid") == "" {
		client.Cookie = strings.TrimPrefix(client.Cookie+"; __ysuid="+strconv.FormatInt(time.Now().Unix(), 10)+strings.ReplaceAll(sourceutil.RandomUUID(), "-", "")[:3], "; ")
	}
	payload, err := client.load_playback(ctx, id)
	if err != nil {
		return scraper.Video{}, err
	}
	video := payload.Data.Video
	if !playback_matches(payload, id) {
		return scraper.Video{}, errors.New("优酷响应缺少可核对的视频身份或编号不匹配")
	}
	duration := sourceutil.Numeric(video.Seconds)
	streams := payload.Data.Stream
	sort.SliceStable(streams, func(i, j int) bool {
		if sourceutil.Numeric(streams[i].Height) != sourceutil.Numeric(streams[j].Height) {
			return sourceutil.Numeric(streams[i].Height) > sourceutil.Numeric(streams[j].Height)
		}
		if sourceutil.Numeric(streams[i].Width) != sourceutil.Numeric(streams[j].Width) {
			return sourceutil.Numeric(streams[i].Width) > sourceutil.Numeric(streams[j].Width)
		}
		return sourceutil.Numeric(streams[i].Size) > sourceutil.Numeric(streams[j].Size)
	})
	var last_error error
	for _, stream := range streams {
		if stream.URL == "" || stream.Channel == "tail" || stream.Channel == "head" || stream.Channel == "trial" || (stream.DRMType != "" && stream.DRMType != "default" && stream.DRMType != "none" && stream.DRMType != "0") {
			continue
		}
		width, height := int(sourceutil.Numeric(stream.Width)), int(sourceutil.Numeric(stream.Height))
		if width <= 0 || height <= 0 || duration <= 0 {
			continue
		}
		if ms := sourceutil.Numeric(stream.Duration); ms > 0 && (ms/1000 < duration-2 || ms/1000 > duration+2) {
			continue
		}
		address := strings.Replace(stream.URL, "http://", "https://", 1)
		playlist, audio_playlist, err := client.LoadHLSTracks(ctx, address, 0)
		if err == nil {
			err = source_policy.CheckPlaylistDuration(playlist, duration)
			if err == nil && audio_playlist != "" {
				err = source_policy.CheckPlaylistDuration(audio_playlist, duration)
			}
		}
		if err != nil {
			last_error = err
			continue
		}
		codec := "h264"
		if value := string(stream.H265); value == "1" || value == "true" || value == `"1"` {
			codec = "hevc"
		}
		return scraper.Video{UserAgent: youku_user_agent, Referer: client.Referer, Profile: youku_profile(payload.Data.Show), Title: video.Title, Playlist: playlist, AudioPlaylist: audio_playlist, Info: scraper.PlayInfo{Address: address, Duration: duration, Width: width, Height: height, Codec: codec}}, nil
	}
	if last_error != nil {
		return scraper.Video{}, fmt.Errorf("优酷未提供可用完整 HLS：%w", last_error)
	}
	return scraper.Video{}, errors.New("优酷没有可下载的完整片源，可能仅允许试看、需要账号权限或使用不支持的 DRM")
}

type youku_playback struct {
	Error json.RawMessage `json:"error"`
	Data  struct {
		Error json.RawMessage `json:"error"`
		Show  json.RawMessage `json:"show"`
		Video struct {
			ID       sourceutil.TextValue `json:"id"`
			EncodeID string               `json:"encodeid"`
			Title    string               `json:"title"`
			Seconds  sourceutil.TextValue `json:"seconds"`
		} `json:"video"`
		Stream []struct {
			URL      string               `json:"m3u8_url"`
			DRMType  string               `json:"drm_type"`
			H265     json.RawMessage      `json:"h265"`
			Width    sourceutil.TextValue `json:"width"`
			Height   sourceutil.TextValue `json:"height"`
			Size     sourceutil.TextValue `json:"size"`
			Duration sourceutil.TextValue `json:"milliseconds_video"`
			Channel  string               `json:"channel_type"`
		} `json:"stream"`
	} `json:"data"`
}

func youku_error_code(raw json.RawMessage) string {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" || value == "{}" || value == "0" {
		return ""
	}
	var failure struct {
		Code sourceutil.TextValue `json:"code"`
	}
	if json.Unmarshal(raw, &failure) != nil {
		return "unknown"
	}
	code := string(failure.Code)
	if code == "0" {
		return ""
	}
	if code == "" || len(code) > 32 || !regexp.MustCompile(`^-?[0-9A-Za-z_]+$`).MatchString(code) {
		return "unknown"
	}
	return code
}

func youku_profile(raw json.RawMessage) *scraper.Profile {
	var show struct {
		ID       sourceutil.TextValue `json:"id"`
		Title    string               `json:"title"`
		Category json.RawMessage      `json:"category"`
	}
	if json.Unmarshal(raw, &show) != nil || show.ID == "" {
		return nil
	}
	kind := "series"
	category := ""
	_ = json.Unmarshal(show.Category, &category)
	if category == "" {
		var value struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(show.Category, &value)
		category = value.Name
	}
	if category == "电影" || strings.EqualFold(category, "movie") {
		kind = "movie"
	}
	return &scraper.Profile{ID: string(show.ID), Kind: kind, Title: show.Title, Metadata: raw}
}
