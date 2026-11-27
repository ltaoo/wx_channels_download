// Adapted from the local dramadl project; see ../NOTICE and ../LICENSE.
package iqiyi

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"wx_channel/pkg/scraper"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

const iqiyi_version = "13.062.22175"
const iqiyi_user_agent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36"

var iqiyi_path_pattern = regexp.MustCompile(`^/(v|a)_([a-z0-9]+)\.html$`)
var iqiyi_init_pattern = regexp.MustCompile(`QiyiPlayerProphetData\s*=\s*(\{)`)

type iqiyi_episode struct {
	TVID        sourceutil.TextValue `json:"qipu_id"`
	Order       int                  `json:"album_order"`
	Title       string               `json:"title"`
	SubTitle    string               `json:"subtitle"`
	PageURL     string               `json:"page_url"`
	PayMark     int                  `json:"pay_mark"`
	ContentType sourceutil.TextValue `json:"content_type"`
}

type iqiyi_series struct {
	ID       string
	Title    string
	profile  scraper.Profile
	episodes []iqiyi_episode
}

func is_iqiyi_url(value string) bool {
	link, err := url.Parse(value)
	return err == nil && (strings.EqualFold(link.Hostname(), "iqiyi.com") || strings.HasSuffix(strings.ToLower(link.Hostname()), ".iqiyi.com"))
}

func iqiyi_page(value string) (string, string, bool, error) {
	link, err := url.Parse(strings.TrimSpace(value))
	if err != nil || len(value) > 4096 || !is_iqiyi_url(value) || link.User != nil || (link.Scheme != "https" && link.Scheme != "http") || link.Port() != "" {
		return "", "", false, errors.New("请输入爱奇艺 v_ 播放页或 a_ 专辑页链接")
	}
	match := iqiyi_path_pattern.FindStringSubmatch(link.Path)
	if match == nil {
		return "", "", false, errors.New("爱奇艺链接需要为 /v_编号.html 或 /a_编号.html")
	}
	number, err := strconv.ParseUint(match[2], 36, 64)
	if err != nil {
		return "", "", false, errors.New("爱奇艺链接编号无效")
	}
	// The official playback page encodes its entity ID as base 36 XOR this constant.
	number ^= 0x75706971676c
	if number < 900000 {
		number = 100 * (number + 900000)
	}
	if number == 0 {
		return "", "", false, errors.New("爱奇艺链接编号无效")
	}
	link.Scheme, link.Host, link.RawQuery, link.Fragment = "https", "www.iqiyi.com", "", ""
	return link.String(), strconv.FormatUint(number, 10), match[1] == "a", nil
}

func iqiyi_md5(value string) string {
	digest := md5.Sum([]byte(value))
	return hex.EncodeToString(digest[:])
}

func iqiyi_sign(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+values.Get(key))
	}
	return strings.ToUpper(iqiyi_md5(strings.Join(parts, "&") + "&secret_key=howcuteitis"))
}

func iqiyi_vf(path_query string) string {
	return iqiyi_md5(path_query + "ulc2h7tka0mdrf2lkb1n6m6mulc2htbn")
}

func iqiyi_dash_url(tvid, device_id, pck, uid string, now int64) string {
	timestamp := strconv.FormatInt(now, 10)
	fields := [][2]string{
		{"tvid", tvid}, {"bid", "800"}, {"vid", ""}, {"src", "01010031010000000000"}, {"vt", "0"}, {"rs", "1"}, {"uid", uid}, {"ori", "pcw"},
		{"ps", "1"}, {"k_uid", device_id}, {"pt", "0"}, {"d", "0"}, {"s", ""}, {"lid", "0"}, {"cf", "0"}, {"ct", "0"},
		{"authKey", iqiyi_md5(iqiyi_md5("") + timestamp + tvid)}, {"k_tag", "1"}, {"dfp", ""}, {"locale", "zh_cn"}, {"pck", pck},
		{"k_err_retries", "0"}, {"up", ""}, {"qd_v", "a1"}, {"tm", timestamp}, {"k_ft1", "706436220846084"}, {"k_ft4", "1162321298202628"},
		{"k_ft5", "150994945"}, {"k_ft7", "4"}, {"fr_300", "120_120_120_120_120_120"}, {"fr_500", "120_120_120_120_120_120"},
		{"fr_600", "120_120_120_120_120_120"}, {"fr_800", "120_120_120_120_120_120"}, {"fr_1020", "120_120_120_120_120_120"},
		{"bop", url.QueryEscape(`{"version":"10.0","dfp":"","b_ft1":28}`)}, {"sr", "1"}, {"ost", "0"}, {"ut", "0"},
	}
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, url.QueryEscape(field[0])+"="+url.QueryEscape(field[1]))
	}
	query := strings.Join(parts, "&")
	return "https://cache.video.iqiyi.com/dash?" + query + "&vf=" + iqiyi_vf("/dash?"+query)
}

func iqiyi_episode_rows(raw json.RawMessage, album_id string, depth int) ([]iqiyi_episode, error) {
	if depth > 8 {
		return nil, errors.New("爱奇艺分集列表层级异常")
	}
	var array []json.RawMessage
	if json.Unmarshal(raw, &array) == nil {
		var rows []iqiyi_episode
		for _, entry := range array {
			var row iqiyi_episode
			if json.Unmarshal(entry, &row) == nil && row.TVID != "" {
				rows = append(rows, row)
				continue
			}
			nested, err := iqiyi_episode_rows(entry, album_id, depth+1)
			if err != nil {
				return nil, err
			}
			rows = append(rows, nested...)
		}
		return rows, nil
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil {
		return nil, errors.New("爱奇艺分集列表结构无效")
	}
	for _, key := range []string{"videos", "feature_paged", "data"} {
		if nested, ok := object[key]; ok {
			if key == "videos" {
				var group_id sourceutil.TextValue
				if id, exists := object["entity_id"]; exists {
					if json.Unmarshal(id, &group_id) != nil {
						return nil, errors.New("爱奇艺选集分组编号无效")
					}
					if group_id != "" && string(group_id) != album_id {
						return nil, nil
					}
				}
				var remote string
				// Other language/story tabs may contain a lazy selector URL instead of episodes.
				if json.Unmarshal(nested, &remote) == nil {
					return nil, nil
				}
			}
			if key != "feature_paged" {
				return iqiyi_episode_rows(nested, album_id, depth+1)
			}
			var pages map[string]json.RawMessage
			if json.Unmarshal(nested, &pages) != nil {
				return nil, errors.New("爱奇艺分集分页无效")
			}
			keys := make([]string, 0, len(pages))
			for key := range pages {
				keys = append(keys, key)
			}
			sort.Slice(keys, func(i, j int) bool {
				a, _ := strconv.Atoi(strings.Split(keys[i], "-")[0])
				b, _ := strconv.Atoi(strings.Split(keys[j], "-")[0])
				return a < b
			})
			var rows []iqiyi_episode
			for _, key := range keys {
				page, err := iqiyi_episode_rows(pages[key], album_id, depth+1)
				if err != nil {
					return nil, err
				}
				rows = append(rows, page...)
			}
			return rows, nil
		}
	}
	return nil, nil
}

func (client *backend) album_episode(ctx context.Context, album_id, page string) (string, string, error) {
	if canonical, tvid, album, err := iqiyi_page(page); err == nil && !album {
		return canonical, tvid, nil
	}
	values := url.Values{"entityId": {album_id}, "appVersion": {iqiyi_version}, "deviceId": {client.device_id}, "timestamp": {fmt.Sprint(time.Now().Unix())}}
	values.Set("sign", iqiyi_sign(values))
	body, err := client.GetText(ctx, "https://mesh.if.iqiyi.com/tvg/entity_play_url?"+values.Encode())
	if err != nil {
		return "", "", err
	}
	var payload struct {
		Data struct {
			Type string               `json:"resolve_type"`
			TVID sourceutil.TextValue `json:"episode_id"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(body), &payload) != nil || (payload.Data.Type != "episode" && payload.Data.Type != "album_episode") || !id_pattern.MatchString(string(payload.Data.TVID)) {
		return "", "", errors.New("爱奇艺专辑未返回可播放的正片入口")
	}
	tvid := string(payload.Data.TVID)
	number, err := strconv.ParseUint(tvid, 10, 64)
	if err != nil {
		return "", "", errors.New("爱奇艺返回无效播放编号")
	}
	return "https://www.iqiyi.com/v_" + strconv.FormatUint(number^0x75706971676c, 36) + ".html", tvid, nil
}

func (client *backend) series(ctx context.Context, page, entity_id string, album bool) (iqiyi_series, error) {
	expected_album := ""
	if !album {
		body, err := client.GetText(ctx, "https://mesh.if.iqiyi.com/player/lw/lwplay/accelerator.js?apiVer=3&tvId="+entity_id)
		if err != nil {
			return iqiyi_series{}, err
		}
		match := iqiyi_init_pattern.FindStringSubmatchIndex(body)
		if match == nil {
			return iqiyi_series{}, errors.New("爱奇艺初始化未返回视频身份")
		}
		var init struct {
			TVID      sourceutil.TextValue `json:"tvid"`
			VideoInfo struct {
				TVID      sourceutil.TextValue `json:"tvId"`
				AlbumID   sourceutil.TextValue `json:"albumId"`
				Effective *bool                `json:"effective"`
			} `json:"videoInfo"`
		}
		if json.NewDecoder(strings.NewReader(body[match[2]:])).Decode(&init) != nil || string(init.TVID) != entity_id || string(init.VideoInfo.TVID) != entity_id || (init.VideoInfo.Effective != nil && !*init.VideoInfo.Effective) {
			return iqiyi_series{}, errors.New("爱奇艺初始化与请求的视频不匹配或视频已下架")
		}
		expected_album = string(init.VideoInfo.AlbumID)
	} else {
		expected_album = entity_id
	}
	values := url.Values{"entity_id": {entity_id}, "device_id": {client.device_id}, "auth_cookie": {sourceutil.CookieValue(client.Cookie, "P00001")}, "pcv": {iqiyi_version}, "app_version": {iqiyi_version}, "ext": {""}, "app_mode": {"standard"}, "scale": {"125"}, "timestamp": {fmt.Sprint(time.Now().UnixMilli())}, "src": {"pca_tvg"}, "os": {""}, "conduit_id": {""}}
	values.Set("sign", iqiyi_sign(values))
	body, err := client.GetText(ctx, "https://mesh.if.iqiyi.com/tvg/v2/lw/base_info?"+values.Encode())
	if err != nil {
		return iqiyi_series{}, err
	}
	var payload struct {
		StatusCode sourceutil.TextValue `json:"status_code"`
		Data       struct {
			Base struct {
				ID          sourceutil.TextValue `json:"qipu_id"`
				Title       string               `json:"title"`
				PageURL     string               `json:"page_url"`
				Order       int                  `json:"current_video_order"`
				PayMark     int                  `json:"play_show_status"`
				SourceType  string               `json:"album_source_type"`
				Description string               `json:"desc"`
				ImageURL    string               `json:"image_url"`
				BackdropURL string               `json:"horizontal_image_url"`
				Year        sourceutil.TextValue `json:"current_video_year"`
				ChannelID   int                  `json:"channel_id"`
				Total       int                  `json:"total_episode"`
				UpdateInfo  struct {
					Status   string `json:"update_status"`
					Finished *bool  `json:"album_finish"`
				} `json:"update_info"`
			} `json:"base_data"`
			Template struct {
				Tabs []struct {
					Blocks []struct {
						ID   string          `json:"bk_id"`
						Type string          `json:"bk_type"`
						Data json.RawMessage `json:"data"`
					} `json:"blocks"`
				} `json:"tabs"`
			} `json:"template"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(body), &payload) != nil || (payload.StatusCode != "" && payload.StatusCode != "0") || !id_pattern.MatchString(string(payload.Data.Base.ID)) || payload.Data.Base.Title == "" {
		return iqiyi_series{}, errors.New("爱奇艺未返回有效剧集信息，可能需要登录或已下架")
	}
	base := payload.Data.Base
	if expected_album != "" && expected_album != "0" && string(base.ID) != expected_album {
		return iqiyi_series{}, errors.New("爱奇艺剧集编号与请求不匹配")
	}
	var rows []iqiyi_episode
	if len(payload.Data.Template.Tabs) > 0 {
		for index, block := range payload.Data.Template.Tabs[0].Blocks {
			if block.ID == "selector_bk" || block.Type == "album_episodes" || (index == 2 && block.ID == "" && block.Type == "") {
				if len(block.Data) > 0 {
					rows, err = iqiyi_episode_rows(block.Data, string(base.ID), 0)
					if err != nil {
						return iqiyi_series{}, err
					}
				}
				break
			}
		}
	}
	if len(rows) == 0 {
		if album {
			resolved_page, tvid, resolve_err := client.album_episode(ctx, entity_id, base.PageURL)
			if resolve_err != nil {
				return iqiyi_series{}, resolve_err
			}
			resolved_client := *client
			resolved_client.Referer = resolved_page
			resolved, resolve_err := resolved_client.series(ctx, resolved_page, tvid, false)
			if resolve_err != nil {
				return iqiyi_series{}, resolve_err
			}
			if resolved.ID != entity_id {
				return iqiyi_series{}, errors.New("爱奇艺专辑跳转后的剧集编号不匹配")
			}
			return resolved, nil
		}
		if base.SourceType != "-1" {
			return iqiyi_series{}, errors.New("爱奇艺未提供整剧分集列表，不能将当前一集当成全集")
		}
		canonical, tvid, _, parse_err := iqiyi_page(base.PageURL)
		if parse_err != nil || (!album && tvid != entity_id) {
			return iqiyi_series{}, errors.New("爱奇艺未返回可确认的分集列表")
		}
		rows = []iqiyi_episode{{TVID: sourceutil.TextValue(tvid), Order: 1, Title: base.Title, PageURL: canonical, PayMark: base.PayMark}}
	}
	seen := make(map[sourceutil.TextValue]bool)
	episodes := make([]iqiyi_episode, 0, len(rows))
	found := album
	for _, row := range rows {
		if row.ContentType != "" && row.ContentType != "1" {
			continue
		}
		if !id_pattern.MatchString(string(row.TVID)) {
			return iqiyi_series{}, errors.New("爱奇艺分集编号无效")
		}
		if seen[row.TVID] {
			continue
		}
		seen[row.TVID] = true
		if row.Order <= 0 {
			row.Order = len(episodes) + 1
		}
		if row.PageURL != "" {
			canonical, tvid, _, err := iqiyi_page(row.PageURL)
			if err != nil || tvid != string(row.TVID) {
				return iqiyi_series{}, errors.New("爱奇艺分集链接与编号不匹配")
			}
			row.PageURL = canonical
		}
		if string(row.TVID) == entity_id {
			found = true
		}
		episodes = append(episodes, row)
	}
	if !found {
		return iqiyi_series{}, errors.New("爱奇艺分集列表不包含请求的视频")
	}
	sort.SliceStable(episodes, func(i, j int) bool { return episodes[i].Order < episodes[j].Order })
	for index, row := range episodes {
		if row.Order != index+1 {
			return iqiyi_series{}, errors.New("爱奇艺分集列表缺页或集数重复，拒绝按错误顺序下载")
		}
	}
	if base.UpdateInfo.Finished != nil && *base.UpdateInfo.Finished && base.Total > len(episodes) {
		return iqiyi_series{}, errors.New("爱奇艺已完结剧集的正片列表缺集，不能当成完整目录")
	}
	profile := scraper.Profile{ID: string(base.ID), Kind: "video", Title: sourceutil.CleanText(base.Title), Description: sourceutil.CleanText(base.Description), CoverURL: sourceutil.PublicImage(base.ImageURL), BackdropURL: sourceutil.PublicImage(base.BackdropURL), EpisodeCount: len(episodes), AvailableEpisodeCount: len(episodes), Tips: sourceutil.CleanText(base.UpdateInfo.Status)}
	if len(episodes) > 1 || base.ChannelID == 2 {
		profile.Kind = "series"
	}
	if base.SourceType == "-1" || base.ChannelID == 1 {
		profile.Kind = "movie"
	}
	if base.Total > 0 {
		profile.EpisodeCount = base.Total
	}
	if year, err := strconv.Atoi(string(base.Year)); err == nil && year >= 1800 && year <= 2200 {
		profile.ReleaseYear = year
	}
	if base.UpdateInfo.Finished != nil {
		profile.Status = "连载中"
		if *base.UpdateInfo.Finished {
			profile.Status = "已完结"
		}
	}
	profile.Metadata, _ = json.Marshal(map[string]any{"iqiyi_album_id": base.ID, "channel_id": base.ChannelID, "album_source_type": base.SourceType, "release_year": profile.ReleaseYear})
	return iqiyi_series{ID: string(base.ID), Title: profile.Title, profile: profile, episodes: episodes}, nil
}

func (client *backend) playlist(ctx context.Context, episode iqiyi_episode) (string, scraper.PlayInfo, error) {
	if episode.PayMark > 0 && client.Cookie == "" {
		return "", scraper.PlayInfo{}, errors.New("该集需要登录或会员权限，请在平台配置中提供有权限账号的 Cookie")
	}
	address := iqiyi_dash_url(string(episode.TVID), client.device_id, sourceutil.CookieValue(client.Cookie, "P00001"), sourceutil.CookieValue(client.Cookie, "P00003"), time.Now().UnixMilli())
	body, err := client.GetText(ctx, address)
	if err != nil {
		return "", scraper.PlayInfo{}, err
	}
	rows, err := read_dash_response(body, string(episode.TVID))
	if err != nil {
		return "", scraper.PlayInfo{}, err
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Bid > rows[j].Bid })
	for _, row := range rows {
		if row.M3U8 == "" || row.Format == "dash" {
			continue
		}
		duration := sourceutil.Numeric(row.Duration)
		if duration <= 0 {
			continue
		}
		parsed, err := source_policy.ParseHLS(row.M3U8)
		if err != nil {
			return "", scraper.PlayInfo{}, err
		}
		if math.Abs(parsed.Duration-duration) > math.Max(2, duration*0.01) {
			return "", scraper.PlayInfo{}, errors.New("爱奇艺清单时长与完整分集不匹配，拒绝下载试看片段")
		}
		width, height := 1, 1
		if parts := strings.Split(row.Size, "x"); len(parts) == 2 {
			w, e1 := strconv.Atoi(parts[0])
			h, e2 := strconv.Atoi(parts[1])
			if e1 == nil && e2 == nil && w > 0 && h > 0 {
				width, height = w, h
			}
		}
		return row.M3U8, scraper.PlayInfo{Duration: duration, Width: width, Height: height}, nil
	}
	return "", scraper.PlayInfo{}, errors.New("该集未提供可下载的完整 HLS，可能需要登录、会员或使用了不支持的 DRM/DASH")
}
