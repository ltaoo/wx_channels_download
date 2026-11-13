// Adapted from the local dramadl project; see ../NOTICE and ../LICENSE.
package vqq

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"wx_channel/pkg/scraper"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

func tencent_ckey(id, page, guid string, timestamp int64) string {
	prefix := func(value string) string {
		if len(value) > 48 {
			return value[:48]
		}
		return value
	}
	payload := fmt.Sprintf("%s|%d|mg3c3b04ba|3.5.57|%s|10901|%s|%s||Mozilla|Netscape|Windows x86_64|00|", id, timestamp, guid, prefix(page), prefix(strings.ToLower(sourceutil.UserAgent)))
	checksum := 0
	for _, value := range payload {
		checksum += int(value)
	}
	data := []byte(fmt.Sprintf("|%d|%s", checksum, payload))
	data = append(data, bytes.Repeat([]byte{' '}, (16-len(data)%16)%16)...)
	key := []byte{0x4f, 0x6b, 0xda, 0xa3, 0x9e, 0x2f, 0x8c, 0xb0, 0x7f, 0x5e, 0x72, 0x2d, 0x9e, 0xde, 0xf3, 0x14}
	iv := []byte{0x01, 0x50, 0x4a, 0xf3, 0x56, 0xe6, 0x19, 0xcf, 0x2e, 0x42, 0xbb, 0xa6, 0x8c, 0x3f, 0x70, 0xf9}
	block, _ := aes.NewCipher(key)
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(data, data)
	return strings.ToUpper(hex.EncodeToString(data))
}

func (client *backend) tencent_episodes(ctx context.Context, cover_id, video_id string) ([]online_episode, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	body, _ := json.Marshal(map[string]any{
		"page_params":        map[string]any{"ad_wechat_authorization_status": "0", "req_from": "web_vsite", "ad_exp_ids": "", "pc_sdk_version": "", "pc_oaid": "", "new_mark_label_enabled": "1", "pc_device_info": "", "support_pc_yyb_mobile_app_engine": "0", "pc_wegame_version": "", "cid": cover_id, "history_vid": nil, "vid": video_id, "is_pc_new_detail_page": "0", "is_from_web_flyflow": "1"},
		"page_bypass_params": map[string]any{"params": map[string]string{"caller_id": "3000010", "platform_id": "2"}, "scene": "desk_detail", "app_version": "", "abtest_bypass_id": strings.ReplaceAll(sourceutil.RandomUUID(), "-", "")[:16]},
		"page_context":       map[string]string{},
	})
	response, err := client.Request(ctx, "https://pbaccess.video.qq.com/trpc.vector_layout.page_view.PageService/getPage?video_appid=3000010&vversion_name=8.5.96&vversion_platform=2", body, true, "")
	if err != nil {
		return nil, "", err
	}
	text, err := sourceutil.ReadVideoResponse(response)
	if err != nil {
		return nil, "", err
	}
	var payload struct {
		Ret  int `json:"ret"`
		Data struct {
			Cards []tencent_card `json:"CardList"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(text), &payload) != nil || payload.Ret != 0 {
		return nil, "", errors.New("腾讯分集接口拒绝请求或返回无效数据")
	}
	title := ""
	rows := make(map[int]online_episode)
	seen_ids := make(map[string]int)
	regular_ids := make(map[string]bool)
	tips := ""
	unloaded_tabs := false
	for _, group := range payload.Data.Cards {
		if group.Type == "pc_introduction" {
			profile, err := tencent_profile(cover_id, group)
			if err != nil {
				return nil, "", err
			}
			title, client.Profile = profile.Title, &profile
			continue
		}
		if group.Type != "pc_web_episode_list" {
			continue
		}
		tips = sourceutil.CleanText(group.Params["sub_title"])
		if tabs_text := group.Params["tabs"]; tabs_text != "" && tabs_text != "[]" {
			var tabs []struct {
				Selected bool `json:"selected"`
			}
			if json.Unmarshal([]byte(tabs_text), &tabs) != nil {
				return nil, "", errors.New("腾讯分集分页信息无效")
			}
			for _, tab := range tabs {
				unloaded_tabs = unloaded_tabs || !tab.Selected
			}
		}
		if video_id == "" && (group.Params["has_next"] == "true" || group.Params["has_front"] == "true" || (group.Params["tabs"] != "" && group.Params["tabs"] != "[]" && group.Params["is_all_data"] != "true")) {
			return nil, "", errors.New("腾讯分集列表存在未加载分页，请使用具体分集链接，避免漏下或错序")
		}
		for _, child := range group.Children {
			for _, item := range child.Cards {
				params := item.Params
				if (video_id == "" && params["is_trailer"] != "0") || (params["cid"] != "" && params["cid"] != cover_id) {
					continue
				}
				id := params["vid"]
				order, err := strconv.Atoi(params["title"])
				if err == nil && order > 0 && params["is_trailer"] == "0" && tencent_id_pattern.MatchString(id) {
					regular_ids[id] = true
				}
				if err != nil && (params["title"] == "正片" || params["title"] == "完整版") {
					order, err = 0, nil
				}
				if video_id != "" {
					if id != video_id {
						continue
					}
					if err != nil {
						order = 0
					}
					err = nil
				}
				if err != nil || order < 0 || order > 10000 || !tencent_id_pattern.MatchString(id) {
					continue
				}
				duration, err := strconv.ParseFloat(params["duration"], 64)
				if err != nil || duration <= 0 {
					continue
				}
				if previous, ok := rows[order]; ok && previous.ID != id {
					return nil, "", errors.New("腾讯同集出现多个视频版本，拒绝混合下载")
				}
				if previous, ok := seen_ids[id]; ok && previous != order {
					return nil, "", errors.New("腾讯视频编号对应多个集数")
				}
				badge := ""
				if params["is_trailer"] == "1" {
					badge = "预告片"
				} else if params["title"] == "先导片" || params["title"] == "预告片" || params["title"] == "花絮" {
					badge = params["title"]
				}
				rows[order] = online_episode{ID: id, Order: order, Duration: duration, Title: sourceutil.CleanText(sourceutil.FirstText(params["full_title"], params["c_title_output"], params["union_title"], params["play_title"], params["title"])), LongTitle: sourceutil.CleanText(params["play_title"]), CoverURL: sourceutil.PublicImage(params["image_url"]), Badge: badge}
				seen_ids[id] = order
			}
		}
	}
	episodes := make([]online_episode, 0, len(rows))
	for _, row := range rows {
		episodes = append(episodes, row)
	}
	sort.Slice(episodes, func(i, j int) bool { return episodes[i].Order < episodes[j].Order })
	if len(episodes) == 0 {
		return nil, "", errors.New("腾讯没有返回可核对的正片分集列表")
	}
	if video_id == "" {
		// is_all_data describes the selected tab, not the entire series.
		if unloaded_tabs {
			return nil, "", errors.New("腾讯分集目录还有未加载的分页，请使用具体分集链接，避免遗漏正片")
		}
		for index, row := range episodes {
			if len(episodes) == 1 && row.Order == 0 {
				continue
			}
			if row.Order != index+1 {
				return nil, "", errors.New("腾讯正片分集缺页或不连续，请使用具体分集链接")
			}
		}
	}
	if client.Profile != nil {
		client.Profile.AvailableEpisodeCount = len(regular_ids)
		if client.Profile.EpisodeCount == 0 {
			client.Profile.EpisodeCount = len(regular_ids)
		}
		client.Profile.Tips = sourceutil.FirstText(tips, client.Profile.Tips)
	}
	return episodes, title, nil
}

type tencent_info struct {
	Preview json.RawMessage `json:"preview"`
	Status  string          `json:"s"`
	Formats struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"fi"`
	} `json:"fl"`
	Videos struct {
		Items []struct {
			ID       string               `json:"vid"`
			Title    string               `json:"ti"`
			Duration sourceutil.TextValue `json:"td"`
			Width    sourceutil.TextValue `json:"vw"`
			Height   sourceutil.TextValue `json:"vh"`
			HEVC     sourceutil.TextValue `json:"hevc"`
			DRM      sourceutil.TextValue `json:"drm"`
			Preview  json.RawMessage      `json:"preview"`
			URLs     struct {
				Items []struct {
					URL string `json:"url"`
					HLS *struct {
						Path string `json:"pt"`
						Name string `json:"pname"`
					} `json:"hls"`
				} `json:"ui"`
			} `json:"ul"`
		} `json:"vi"`
	} `json:"vl"`
}

func (client *backend) tencent_info(ctx context.Context, cover_id, id, quality string) (tencent_info, error) {
	guid := strings.ReplaceAll(sourceutil.RandomUUID(), "-", "")[:16]
	// Prefer AVC for MP4 playback compatibility. Preserve the returned codec
	// below in case the server still selects HEVC, which needs an hvc1 tag.
	params := url.Values{
		"vid": {id}, "cid": {cover_id}, "cKey": {tencent_ckey(id, client.Referer, guid, time.Now().Unix())}, "encryptVer": {"8.1"},
		"spcaptiontype": {"0"}, "sphls": {"2"}, "dtype": {"3"}, "defn": {quality}, "spsrt": {"2"}, "sphttps": {"1"}, "otype": {"json"}, "spwm": {"1"}, "hevclv": {"0"}, "drm": {"40"}, "spvideo": {"4"}, "spsfrhdr": {"100"},
		"host": {"v.qq.com"}, "referer": {"v.qq.com"}, "ehost": {client.Referer}, "appVer": {"3.5.57"}, "platform": {"10901"}, "guid": {guid}, "flowid": {strings.ReplaceAll(sourceutil.RandomUUID(), "-", "")},
	}
	if client.Cookie != "" {
		token := map[string]string{"main_login": "qq"}
		for _, name := range []string{"openid", "appid", "access_token", "vuserid", "vusession"} {
			token[name] = sourceutil.CookieValue(client.Cookie, "vqq_"+name)
		}
		raw, _ := json.Marshal(token)
		params.Set("logintoken", string(raw))
	}
	text, err := client.GetText(ctx, "https://h5vv6.video.qq.com/getvinfo?"+params.Encode())
	if err != nil {
		return tencent_info{}, err
	}
	text = strings.TrimSpace(text)
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(text, "QZOutputJson=")), ";"))
	var info tencent_info
	if json.Unmarshal([]byte(text), &info) != nil || len(info.Videos.Items) != 1 || (info.Status != "" && info.Status != "o") {
		return tencent_info{}, errors.New("腾讯未授权播放或接口响应无效，请检查账号权限、地区或稍后重试")
	}
	if info.Videos.Items[0].ID != id {
		return tencent_info{}, errors.New("腾讯播放响应的视频编号不匹配")
	}
	return info, nil
}

func preview_flag(raw json.RawMessage, full_duration float64) bool {
	switch strings.ToLower(strings.TrimSpace(string(raw))) {
	case "", "null", "0", "false", `"0"`, `"false"`:
		return false
	}
	var seconds sourceutil.TextValue
	if json.Unmarshal(raw, &seconds) == nil && sourceutil.Numeric(seconds) > 0 && full_duration > 0 && sourceutil.Numeric(seconds) >= full_duration-1 {
		return false
	}
	return true
}

func (client *backend) tencent_video(ctx context.Context, cover_id string, episode online_episode) (scraper.Video, error) {
	client.small_ranges = false
	id := episode.ID
	if cover_id != "" {
		client.Referer = "https://v.qq.com/x/cover/" + cover_id + "/" + id + ".html"
	} else {
		client.Referer = "https://v.qq.com/x/page/" + id + ".html"
	}
	initial, err := client.tencent_info(ctx, cover_id, id, "hd")
	if err != nil {
		return scraper.Video{}, err
	}
	available := map[string]bool{"hd": true}
	for _, format := range initial.Formats.Items {
		available[format.Name] = true
	}
	var last_error error
	for _, quality := range []string{"uhd", "fhd", "shd", "hd", "sd", "ld"} {
		if !available[quality] {
			continue
		}
		info := initial
		if quality != "hd" {
			info, err = client.tencent_info(ctx, cover_id, id, quality)
			if err != nil {
				last_error = err
				continue
			}
		}
		row := info.Videos.Items[0]
		if preview_flag(info.Preview, episode.Duration) || preview_flag(row.Preview, episode.Duration) || sourceutil.Numeric(row.DRM) != 0 {
			last_error = errors.New("腾讯返回试看或不支持的 DRM 片源，请检查账号权限")
			continue
		}
		duration := sourceutil.Numeric(row.Duration)
		if episode.Duration > 0 {
			if duration < episode.Duration-2 || duration > episode.Duration+2 {
				last_error = errors.New("腾讯播放时长与正片分集不一致，拒绝保存试看")
				continue
			}
		}
		width, height := int(sourceutil.Numeric(row.Width)), int(sourceutil.Numeric(row.Height))
		codec := ""
		switch row.HEVC {
		case "0":
			codec = "h264"
		case "1":
			codec = "hevc"
		}
		if duration <= 0 || width <= 0 || height <= 0 {
			last_error = errors.New("腾讯片源缺少完整时长或尺寸")
			continue
		}
		var candidates []scraper.Video
		for _, entry := range row.URLs.Items {
			address := entry.URL
			if entry.HLS != nil {
				address += sourceutil.FirstText(entry.HLS.Path, entry.HLS.Name)
			} else {
				parsed, _ := url.Parse(address)
				if parsed == nil || !strings.HasSuffix(strings.ToLower(parsed.Path), ".m3u8") {
					continue
				}
			}
			playlist, err := client.LoadHLS(ctx, address, 0)
			if err == nil {
				err = source_policy.CheckPlaylistDuration(playlist, duration)
			}
			if err != nil {
				last_error = err
				continue
			}
			candidates = append(candidates, scraper.Video{Title: row.Title, Playlist: playlist, Info: scraper.PlayInfo{Address: address, Codec: codec, Width: width, Height: height, Duration: duration}})
		}
		for _, candidate := range candidates {
			if client.probe_hls(ctx, candidate.Playlist) {
				if client.small_ranges {
					candidate.Transport = &range_transport{client: client}
				}
				return candidate, nil
			}
		}
		if len(candidates) > 0 {
			last_error = errors.New("腾讯候选 CDN 未连续返回媒体数据，请稍后重试")
		}
	}
	if last_error != nil {
		return scraper.Video{}, fmt.Errorf("腾讯未提供可用完整 HLS：%w", last_error)
	}
	return scraper.Video{}, errors.New("腾讯未提供可下载的完整 HLS，可能需要账号权限或使用不支持的片源格式")
}

func (client *backend) probe_hls(ctx context.Context, text string) bool {
	playlist, err := source_policy.ParseHLS(text)
	if err != nil {
		return false
	}
	for _, resource := range playlist.Resources {
		if resource.Key {
			continue
		}
		probe_ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		response, err := client.Response(probe_ctx, resource.Address, false)
		if err != nil {
			cancel()
			return false
		}
		size, err := io.CopyN(io.Discard, response.Body, 32<<10)
		response.Body.Close()
		cancel()
		if size > 0 && (err == nil || err == io.EOF) {
			return true
		}
		if size > 0 {
			probe_ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			response, err := client.Request(probe_ctx, resource.Address, nil, false, fmt.Sprintf("bytes=0-%d", tencent_range_size-1))
			if err != nil {
				return false
			}
			defer response.Body.Close()
			start, end, total, err := parse_content_range(response.Header.Get("Content-Range"))
			if err != nil || response.StatusCode != 206 || start != 0 || end != min_int64(total-1, tencent_range_size-1) {
				return false
			}
			size, err := io.Copy(io.Discard, io.LimitReader(response.Body, tencent_range_size+1))
			if err == nil && size == end+1 {
				client.small_ranges = true
				return true
			}
		}
		return false
	}
	return false
}
