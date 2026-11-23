// Adapted from dramadl; see ../scraper/NOTICE and ../scraper/LICENSE.
// Package hls parses HLS syntax without network access or source-platform policy.
package hls

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"path"
	"strconv"
	"strings"
)

// Resource describes one unique segment, initialization object or AES-128 key.
// Address retains the original URI, including relative references.
type Resource struct {
	Address  string
	Filename string
	Key      bool
}

// Playlist describes a finite media playlist. Text uses deterministic local
// filenames and Resources maps them back to the original URIs.
type Playlist struct {
	Text      string
	Duration  float64
	Resources []Resource
}

// ParseMedia parses a complete media playlist and rewrites resource references
// to local filenames. Supported inputs have EXT-X-ENDLIST and use clear media
// or AES-128 identity encryption. Master/live playlists must be handled by the
// caller. URI scheme, host, credentials and source permissions are caller policy.
func ParseMedia(text string) (Playlist, error) {
	text = strings.TrimSpace(strings.TrimPrefix(text, "\ufeff"))
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(text) > 8<<20 || len(lines) == 0 || lines[0] != "#EXTM3U" {
		return Playlist{}, errors.New("未返回有效 HLS 清单")
	}
	playlist := Playlist{}
	seen := make(map[string]int)
	add := func(address string, key, initial bool) (string, error) {
		link, err := url.Parse(address)
		if err != nil || address == "" || strings.ContainsAny(address, "\r\n\t") {
			return "", errors.New("HLS 资源 URI 无效")
		}
		if index, exists := seen[address]; exists {
			if playlist.Resources[index].Key != key {
				return "", errors.New("HLS 密钥地址与视频分片冲突")
			}
			return playlist.Resources[index].Filename, nil
		}
		if len(playlist.Resources) >= 10000 {
			return "", errors.New("HLS 资源数量异常")
		}
		extension := strings.ToLower(path.Ext(link.Path))
		if key {
			extension = ".key"
		} else if extension != ".mp4" && extension != ".m4s" && extension != ".m4a" {
			extension = ".ts"
			if initial {
				extension = ".mp4"
			}
		}
		filename := fmt.Sprintf("resource-%05d%s", len(playlist.Resources), extension)
		seen[address] = len(playlist.Resources)
		playlist.Resources = append(playlist.Resources, Resource{Address: address, Filename: filename, Key: key})
		return filename, nil
	}
	ended, pending, segments := false, false, 0
	for index, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case strings.HasPrefix(line, "#EXT-X-STREAM-INF:") || strings.HasPrefix(line, "#EXT-X-MEDIA:"):
			return Playlist{}, errors.New("该片源返回多层 HLS，当前需要直接分片清单")
		case strings.HasPrefix(line, "#EXTINF:"):
			if pending || ended {
				return Playlist{}, errors.New("HLS 分片顺序无效")
			}
			value := strings.Split(strings.TrimPrefix(line, "#EXTINF:"), ",")[0]
			duration, err := strconv.ParseFloat(value, 64)
			// Preserve media bytes in boundary segments rounded to zero seconds.
			if err != nil || duration < 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
				return Playlist{}, errors.New("HLS 分片时长无效")
			}
			playlist.Duration += duration
			pending = true
		case line == "#EXT-X-ENDLIST":
			if pending {
				return Playlist{}, errors.New("HLS 缺少分片地址")
			}
			ended = true
		case strings.HasPrefix(line, "#EXT-X-KEY:") || strings.HasPrefix(line, "#EXT-X-MAP:"):
			tag, values, _ := strings.Cut(line, ":")
			attributes, err := ParseAttributes(values)
			if err != nil {
				return Playlist{}, err
			}
			key := tag == "#EXT-X-KEY"
			if key {
				if attributes["METHOD"] == "NONE" {
					if attributes["URI"] != "" {
						return Playlist{}, errors.New("HLS 明文标记含密钥地址")
					}
					break
				}
				if attributes["METHOD"] != "AES-128" || (attributes["KEYFORMAT"] != "" && attributes["KEYFORMAT"] != "identity") {
					return Playlist{}, errors.New("该 HLS 使用不支持的 DRM 或加密方式")
				}
			}
			address := attributes["URI"]
			if address == "" || !strings.Contains(line, `URI="`+address+`"`) {
				return Playlist{}, errors.New("HLS 缺少有效的资源 URI")
			}
			filename, err := add(address, key, tag == "#EXT-X-MAP")
			if err != nil {
				return Playlist{}, err
			}
			line = strings.Replace(line, `URI="`+address+`"`, `URI="`+filename+`"`, 1)
		case strings.HasPrefix(line, "#"):
			if strings.Contains(line, "URI=") {
				return Playlist{}, errors.New("HLS 包含不支持的远程资源标签")
			}
		default:
			if !pending || ended {
				return Playlist{}, errors.New("HLS 缺少分片时长或结束标记位置无效")
			}
			filename, err := add(line, false, false)
			if err != nil {
				return Playlist{}, err
			}
			line = filename
			pending = false
			segments++
		}
		lines[index] = line
	}
	if !ended || pending || segments == 0 || playlist.Duration <= 0 || math.IsInf(playlist.Duration, 0) {
		return Playlist{}, errors.New("HLS 清单未完整结束或没有有效分片")
	}
	playlist.Text = strings.Join(lines, "\n") + "\n"
	return playlist, nil
}
