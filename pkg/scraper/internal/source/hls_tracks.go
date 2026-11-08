// Adapted from the local dramadl project; see ../../NOTICE and ../../LICENSE.
package source

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"wx_channel/pkg/hls"
)

func (client *RequestClient) HLSAddress(base *url.URL, value string) (string, error) {
	relative, err := url.Parse(value)
	if err != nil {
		return "", errors.New("HLS 资源地址无效")
	}
	link := base.ResolveReference(relative)
	if client.Policy.UpgradeHTTPS && link.Scheme == "http" {
		link.Scheme = "https"
	}
	address := link.String()
	if err := client.Policy.check_address(address, false); err != nil {
		return "", err
	}
	return address, nil
}

func (client *RequestClient) LoadHLSTracks(ctx context.Context, address string, depth int) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	if depth > 3 {
		return "", "", errors.New("HLS 嵌套超过 3 层")
	}
	response, err := client.Response(ctx, address, false)
	if err != nil {
		return "", "", err
	}
	base := response.Request.URL
	text, err := ReadVideoResponse(response)
	if err != nil {
		return "", "", err
	}
	text = strings.TrimSpace(strings.TrimPrefix(text, "\ufeff"))
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) == 0 || lines[0] != "#EXTM3U" {
		return "", "", errors.New("片源未返回 HLS 清单")
	}
	audio_groups := make(map[string]map[string]string)
	best_url, best_audio := "", ""
	best_bandwidth := int64(-1)
	var variant map[string]string
	media_count := 0
	for index, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "#EXT-X-SESSION-KEY:"):
			return "", "", errors.New("暂不支持 HLS 会话级密钥或 DRM")
		case strings.HasPrefix(line, "#EXT-X-MEDIA:"):
			attrs, err := hls.ParseAttributes(strings.TrimPrefix(line, "#EXT-X-MEDIA:"))
			if err != nil {
				return "", "", err
			}
			if attrs["TYPE"] == "AUDIO" {
				group := attrs["GROUP-ID"]
				if group == "" {
					return "", "", errors.New("HLS 音轨缺少组编号")
				}
				previous, exists := audio_groups[group]
				if !exists || (previous["DEFAULT"] != "YES" && attrs["DEFAULT"] == "YES") {
					audio_groups[group] = attrs
				}
			}
		case strings.HasPrefix(line, "#EXT-X-STREAM-INF:"):
			if variant != nil {
				return "", "", errors.New("HLS 主清单缺少子清单地址")
			}
			variant, err = hls.ParseAttributes(strings.TrimPrefix(line, "#EXT-X-STREAM-INF:"))
			if err != nil {
				return "", "", err
			}
		case line != "" && !strings.HasPrefix(line, "#"):
			address, err := client.HLSAddress(base, line)
			if err != nil {
				return "", "", err
			}
			line = address
			if variant != nil {
				bandwidth, err := strconv.ParseInt(variant["BANDWIDTH"], 10, 64)
				if err != nil || bandwidth <= 0 {
					return "", "", errors.New("HLS 主清单码率无效")
				}
				if bandwidth > best_bandwidth {
					best_url, best_audio, best_bandwidth = address, variant["AUDIO"], bandwidth
				}
				variant = nil
			} else {
				media_count++
			}
		case strings.HasPrefix(line, "#EXT-X-KEY:") || strings.HasPrefix(line, "#EXT-X-MAP:"):
			_, values, _ := strings.Cut(line, ":")
			attrs, err := hls.ParseAttributes(values)
			if err != nil {
				return "", "", err
			}
			if value := attrs["URI"]; value != "" {
				address, err := client.HLSAddress(base, value)
				if err != nil {
					return "", "", err
				}
				line = strings.Replace(line, `URI="`+value+`"`, `URI="`+address+`"`, 1)
			}
		}
		lines[index] = line
	}
	if variant != nil {
		return "", "", errors.New("HLS 主清单不完整")
	}
	if best_url != "" {
		if media_count > 0 {
			return "", "", errors.New("HLS 主清单与媒体清单混用")
		}
		video, audio, err := client.LoadHLSTracks(ctx, best_url, depth+1)
		if err != nil {
			return "", "", err
		}
		if best_audio != "" {
			attrs, exists := audio_groups[best_audio]
			if !exists {
				return "", "", errors.New("HLS 缺少视频引用的音轨组")
			}
			if value := attrs["URI"]; value != "" {
				if audio != "" {
					return "", "", errors.New("HLS 嵌套出现重复音轨")
				}
				address, err := client.HLSAddress(base, value)
				if err != nil {
					return "", "", err
				}
				var nested_audio string
				audio, nested_audio, err = client.LoadHLSTracks(ctx, address, depth+1)
				if err != nil {
					return "", "", err
				}
				if nested_audio != "" {
					return "", "", errors.New("HLS 音轨清单含嵌套音轨")
				}
			}
		}
		client.PlaylistURL = best_url
		return video, audio, nil
	}
	text = strings.Join(lines, "\n")
	if _, err := client.Policy.ParseHLS(text); err != nil {
		return "", "", err
	}
	client.PlaylistURL = base.String()
	return text, "", nil
}
