// Adapted from the local dramadl project; see ../NOTICE and ../LICENSE.
package mgtv

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"wx_channel/pkg/scraper"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

func mgtv_token(device_id string, timestamp int64) string {
	value := fmt.Sprintf("did=%s|pno=1030|ver=0.3.0301|clit=%d", device_id, timestamp)
	encoded := []byte(strings.NewReplacer("+", "_", "/", "~", "=", "-").Replace(base64.StdEncoding.EncodeToString([]byte(value))))
	for left, right := 0, len(encoded)-1; left < right; left, right = left+1, right-1 {
		encoded[left], encoded[right] = encoded[right], encoded[left]
	}
	return string(encoded)
}

func (client *backend) mgtv_video(ctx context.Context, cover_id, id string) (scraper.Video, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	device_id := sourceutil.RandomUUID()
	token := mgtv_token(device_id, time.Now().Unix())
	params := url.Values{"did": {device_id}, "suuid": {sourceutil.RandomUUID()}, "cxid": {""}, "tk2": {token}, "type": {"pch5"}, "video_id": {id}, "_support": {"10000000"}, "auth_mode": {""}, "src": {""}, "abroad": {""}}
	body, err := client.GetText(ctx, "https://pcweb.api.mgtv.com/player/video?"+params.Encode())
	if err != nil {
		return scraper.Video{}, err
	}
	var playback struct {
		Code sourceutil.TextValue `json:"code"`
		Data struct {
			ATC struct {
				PM2 string `json:"pm2"`
			} `json:"atc"`
			DRM struct {
				Type string `json:"drmType"`
			} `json:"drm"`
			Info struct {
				ID          sourceutil.TextValue `json:"video_id"`
				Collection  sourceutil.TextValue `json:"collection_id"`
				Title       string               `json:"title"`
				PartName    string               `json:"partName"`
				Description string               `json:"desc"`
				Thumb       string               `json:"thumb"`
				Duration    sourceutil.TextValue `json:"duration"`
			} `json:"info"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(body), &playback) != nil || playback.Code != "200" {
		return scraper.Video{}, errors.New("芒果 TV 播放接口未授权，请检查账号、地区权限和官网播放状态")
	}
	info := playback.Data.Info
	duration := sourceutil.Numeric(info.Duration)
	if string(info.ID) != id || (info.Collection != "" && string(info.Collection) != cover_id) || duration <= 0 {
		return scraper.Video{}, errors.New("芒果 TV 响应缺少完整时长或视频编号不匹配")
	}
	if drm := playback.Data.DRM.Type; drm != "" && drm != "0" && drm != "none" {
		return scraper.Video{}, errors.New("芒果 TV 片源使用不支持的 DRM")
	}
	if playback.Data.ATC.PM2 == "" || len(playback.Data.ATC.PM2) > 8192 {
		return scraper.Video{}, errors.New("芒果 TV 未返回有效播放凭证")
	}
	params = url.Values{"_support": {"10000000"}, "tk2": {token}, "pm2": {playback.Data.ATC.PM2}, "video_id": {id}, "type": {"pch5"}, "auth_mode": {""}, "src": {""}, "abroad": {""}}
	body, err = client.GetText(ctx, "https://pcweb.api.mgtv.com/player/getSource?"+params.Encode())
	if err != nil {
		return scraper.Video{}, err
	}
	var source struct {
		Code sourceutil.TextValue `json:"code"`
		Data struct {
			Domains []string `json:"stream_domain"`
			Streams []struct {
				URL     string               `json:"url"`
				Bitrate sourceutil.TextValue `json:"filebitrate"`
				VIP     sourceutil.TextValue `json:"vip"`
				Format  string               `json:"fileformat"`
				Codec   string               `json:"videoFormat"`
			} `json:"stream"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(body), &source) != nil || source.Code != "200" {
		return scraper.Video{}, errors.New("芒果 TV 未提供授权片源，请检查账号权限")
	}
	streams := source.Data.Streams
	sort.SliceStable(streams, func(i, j int) bool {
		return sourceutil.Numeric(streams[i].Bitrate) > sourceutil.Numeric(streams[j].Bitrate)
	})
	var last_error error
	for _, stream := range streams {
		if stream.URL == "" || (client.Cookie == "" && stream.VIP != "" && stream.VIP != "0") || (stream.Format != "HLS-TS" && stream.Format != "HLS") || (stream.Codec != "H264" && stream.Codec != "H265") {
			continue
		}
		for _, domain := range source.Data.Domains {
			base, err := url.Parse(domain)
			if err != nil {
				last_error = errors.New("芒果 TV 调度域名无效")
				continue
			}
			address, err := client.HLSAddress(base, stream.URL)
			if err != nil {
				last_error = err
				continue
			}
			playlist, audio_playlist, err := client.mgtv_playlist(ctx, address)
			if err == nil {
				err = source_policy.CheckPlaylistDuration(playlist, duration)
				if err != nil {
					last_error = errors.New("芒果 TV 仅返回试看或截断清单；请在平台配置中提供有该视频播放权限的账号 Cookie")
					break
				}
			}
			if err != nil {
				last_error = err
				continue
			}
			width, height, err := mgtv_dimensions(playlist)
			if err != nil {
				last_error = err
				continue
			}
			if audio_playlist != "" {
				if err := source_policy.CheckPlaylistDuration(audio_playlist, duration); err != nil {
					last_error = err
					continue
				}
			}
			codec := "h264"
			if stream.Codec == "H265" {
				codec = "hevc"
			}
			return scraper.Video{Profile: &scraper.Profile{ID: string(info.Collection), Kind: "series", Title: info.Title, Description: info.Description, CoverURL: sourceutil.PublicImage(info.Thumb)}, Title: sourceutil.FirstText(info.PartName, info.Title), Playlist: playlist, AudioPlaylist: audio_playlist, Info: scraper.PlayInfo{Address: client.PlaylistURL, Duration: duration, Width: width, Height: height, Codec: codec}}, nil
		}
	}
	if last_error != nil {
		return scraper.Video{}, fmt.Errorf("芒果 TV 未提供可用完整 HLS：%w", last_error)
	}
	return scraper.Video{}, errors.New("芒果 TV 没有兼容且允许播放的 HLS 片源，请检查账号权限")
}

func (client *backend) mgtv_playlist(ctx context.Context, address string) (string, string, error) {
	response, err := client.Response(ctx, address, false)
	if err != nil {
		return "", "", err
	}
	body, err := sourceutil.ReadVideoResponse(response)
	if err != nil {
		return "", "", err
	}
	var dispatch struct {
		Status string `json:"status"`
		Info   string `json:"info"`
	}
	if json.Unmarshal([]byte(body), &dispatch) != nil || dispatch.Status != "ok" || dispatch.Info == "" {
		return "", "", errors.New("芒果 TV 调度未返回有效媒体清单")
	}
	return client.LoadHLSTracks(ctx, dispatch.Info, 0)
}

func mgtv_dimensions(playlist string) (int, int, error) {
	width, height := 0, 0
	for _, line := range strings.Split(playlist, "\n") {
		tag, value, _ := strings.Cut(strings.TrimSpace(line), ":")
		switch tag {
		case "#EXT-MGTV-VIDEO-WIDTH":
			width, _ = strconv.Atoi(value)
		case "#EXT-MGTV-VIDEO-HEIGHT":
			height, _ = strconv.Atoi(value)
		}
	}
	if width <= 0 || width > 16384 || height <= 0 || height > 16384 {
		return 0, 0, errors.New("芒果 TV 清单缺少有效视频尺寸")
	}
	return width, height, nil
}
