package vod

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strings"
	"wx_channel/pkg/hls"
	"wx_channel/pkg/scraper"
)

// SourceProtocol marks deferred adapter inputs, never an engine protocol.
const SourceProtocol = "media"

type Input struct {
	Platform  string          `json:"platform"`
	SourceURL string          `json:"source_url"`
	Episode   scraper.Episode `json:"episode"`
}

type plan_item struct {
	UniqueID string `json:"unique_id"`
	Path     string `json:"path"`
	Key      bool   `json:"key,omitempty"`
	Address  string `json:"-"`
}

// media_plan contains small local manifests and references to Hermes resources.
// Media bytes stay in the engine's files; no intermediate archive is produced.
type media_plan struct {
	Version       int              `json:"version"`
	Info          scraper.PlayInfo `json:"info"`
	VideoManifest string           `json:"video_manifest,omitempty"`
	AudioManifest string           `json:"audio_manifest,omitempty"`
	Items         []plan_item      `json:"items"`
	Fingerprint   string           `json:"fingerprint"`
}

func string_marshal(value any) string { raw, _ := json.Marshal(value); return string(raw) }

func plan_source(owner string, source scraper.Video) (media_plan, error) {
	if source.CheckMediaAddress == nil {
		return media_plan{}, fmt.Errorf("播放源缺少地址校验策略")
	}
	info := source.Info
	if info.Duration <= 0 || math.IsNaN(info.Duration) || math.IsInf(info.Duration, 0) || info.Width <= 0 || info.Height <= 0 || (len(info.Key) != 0 && len(info.Key) != 16) {
		return media_plan{}, fmt.Errorf("媒体源时长、尺寸或密钥无效")
	}
	plan := media_plan{Version: 1, Info: info}
	plan.Info.Address = ""
	var identities []string
	add := func(path, address string, key bool) {
		unique_id := owner
		if len(plan.Items) > 0 {
			unique_id = fmt.Sprintf("%s__part_%05d", owner, len(plan.Items))
		}
		plan.Items = append(plan.Items, plan_item{UniqueID: unique_id, Path: path, Address: address, Key: key})
		link, _ := url.Parse(address)
		// Signed query tokens may change while the same source is resumed.
		link.RawQuery, link.Fragment = "", ""
		identities = append(identities, link.String())
	}
	if source.Playlist == "" {
		if err := source.CheckMediaAddress(info.Address); err != nil {
			return plan, err
		}
		add("input.mp4", info.Address, false)
	} else {
		for _, track := range []struct{ name, text string }{{"video", source.Playlist}, {"audio", source.AudioPlaylist}} {
			if track.text == "" {
				continue
			}
			playlist, err := hls.ParseMedia(track.text)
			if err != nil {
				return plan, err
			}
			for _, resource := range playlist.Resources {
				if err := source.CheckMediaAddress(resource.Address); err != nil {
					return plan, err
				}
			}
			if math.Abs(playlist.Duration-info.Duration) > math.Max(2, info.Duration*0.01) {
				return plan, fmt.Errorf("播放清单时长与完整视频不符，拒绝保存试看或截断片段")
			}
			if track.name == "video" {
				plan.VideoManifest = playlist.Text
			} else {
				plan.AudioManifest = playlist.Text
			}
			for _, resource := range playlist.Resources {
				add(track.name+"/"+resource.Filename, resource.Address, resource.Key)
			}
		}
	}
	plan.Info.SeparateAudio = plan.AudioManifest != ""
	digest := sha256.Sum256([]byte(string_marshal(plan) + string_marshal(identities)))
	plan.Fingerprint = hex.EncodeToString(digest[:])
	return plan, nil
}

func parse_plan(raw string) (media_plan, error) {
	var plan media_plan
	if raw == "" || len(raw) > 20<<20 || json.Unmarshal([]byte(raw), &plan) != nil || plan.Version != 1 || len(plan.Items) == 0 || len(plan.Items) > 20000 {
		return plan, fmt.Errorf("媒体资源计划无效")
	}
	info := plan.Info
	if info.Duration <= 0 || math.IsNaN(info.Duration) || math.IsInf(info.Duration, 0) || info.Width <= 0 || info.Height <= 0 || (len(info.Key) != 0 && len(info.Key) != 16) || plan.Fingerprint == "" {
		return plan, fmt.Errorf("媒体资源元数据无效")
	}
	seen := map[string]bool{}
	for _, item := range plan.Items {
		if !valid_input_path(item.Path) || item.UniqueID == "" || seen[item.Path] || seen["id:"+item.UniqueID] {
			return plan, fmt.Errorf("媒体资源引用无效")
		}
		seen[item.Path], seen["id:"+item.UniqueID] = true, true
	}
	if plan.VideoManifest == "" && (len(plan.Items) != 1 || plan.Items[0].Path != "input.mp4" || plan.AudioManifest != "") {
		return plan, fmt.Errorf("完整文件资源引用无效")
	}
	if plan.VideoManifest != "" && !strings.HasPrefix(plan.VideoManifest, "#EXTM3U") {
		return plan, fmt.Errorf("媒体清单无效")
	}
	return plan, nil
}
