package youku

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"wx_channel/pkg/scraper"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

// SeriesDetails is an authenticated catalogue result, independent of media
// permissions. Episodes contains only the currently listed regular episodes.
type SeriesDetails struct {
	SourceURL       string            `json:"source_url"`
	Profile         scraper.Profile   `json:"profile"`
	Episodes        []scraper.Episode `json:"episodes"`
	ListingComplete bool              `json:"listing_complete"`
}

type catalogue_node struct {
	ID    sourceutil.TextValue `json:"id"`
	Type  int                  `json:"type"`
	More  bool                 `json:"more"`
	Data  json.RawMessage      `json:"data"`
	Nodes []catalogue_node     `json:"nodes"`
}

type catalogue_extra struct {
	ShowID           string               `json:"showId"`
	ShowLongID       sourceutil.TextValue `json:"showLongId"`
	ShowName         string               `json:"showName"`
	ShowCategory     string               `json:"showCategory"`
	ShowImage        string               `json:"showImg"`
	ShowVertical     string               `json:"showImgV"`
	ShowReleaseTime  string               `json:"showReleaseTime"`
	ShowSubtitle     string               `json:"showSubtitle"`
	EpisodeTotal     int                  `json:"episodeTotal"`
	EpisodeLast      int                  `json:"episodeLast"`
	Completed        *bool                `json:"completed"`
	VideoID          string               `json:"videoId"`
	VideoTitle       string               `json:"videoTitle"`
	VideoImage       string               `json:"videoImg"`
	VideoPublishTime string               `json:"videoPublishTime"`
	VideoStage       sourceutil.TextValue `json:"showVideoStage"`
	Duration         float64              `json:"duration"`
}

type catalogue_page struct {
	extra        catalogue_extra
	profile      scraper.Profile
	episodes     []scraper.Episode
	more         bool
	next_session string
}

func (client *backend) series_details(ctx context.Context, page, show_id, video_id string) (*SeriesDetails, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	session, err := client.new_mtop_session(page, "")
	if err != nil {
		return nil, err
	}
	api_url, _ := url.Parse(playback_api)
	utid := cookie_value(session.HTTPClient.Jar, api_url, "cna")
	system_info, _ := json.Marshal(map[string]any{"appPackageKey": "pcweb", "appPackageId": "pcweb", "appkey": playback_app_key, "device": "pc", "os": "pc", "ver": "1.0.0.0", "pid": "", "zx": 0, "zx_list": "", "userAgent": youku_user_agent, "disableUserRec": "0"})
	var first catalogue_page
	episodes := make(map[string]scraper.Episode)
	orders := make(map[int]string)
	seen_sessions := make(map[string]bool)
	next_session := ""
	for page_index := 0; page_index < 100; page_index++ {
		params := map[string]any{"biz": "kuflix_pc_detail", "scene": "web_page", "componentVersion": "3", "ip": "", "debug": 0, "utdid": utid, "platform": "pc", "nextSession": next_session, "gray": 0, "appPackageKey": "pcweb"}
		if video_id != "" {
			params["videoId"] = video_id
		}
		if show_id != "" {
			params["showId"] = show_id
		}
		if next_session != "" {
			params["scene"] = "component"
		}
		encoded_params, _ := json.Marshal(params)
		data, _ := json.Marshal(map[string]string{"ms_codes": "2019030100", "params": string(encoded_params), "system_info": string(system_info)})
		raw, err := session.request_mtop(ctx, "mtop.youku.columbus.gateway.new.execute", "1.0", data)
		if err != nil {
			return nil, err
		}
		current, err := parse_catalogue_page(raw, show_id, video_id, page_index == 0)
		if err != nil {
			return nil, err
		}
		if page_index == 0 {
			first = current
			show_id = current.extra.ShowID
		} else if current.extra.ShowLongID != "" && current.extra.ShowLongID != first.extra.ShowLongID {
			return nil, errors.New("优酷分集分页期间剧集身份发生变化")
		} else if current.extra.EpisodeTotal > 0 && current.extra.EpisodeTotal != first.extra.EpisodeTotal {
			return nil, errors.New("优酷分集分页期间总集数发生变化")
		}
		added := 0
		for _, episode := range current.episodes {
			if prior, ok := episodes[episode.ID]; ok {
				if prior.Order != episode.Order {
					return nil, errors.New("优酷同一分集出现不同集序")
				}
				continue
			}
			if previous, ok := orders[episode.Order]; ok && previous != episode.ID {
				return nil, errors.New("优酷选集存在相同集序的不同视频，无法确认正片版本")
			}
			episodes[episode.ID], orders[episode.Order] = episode, episode.ID
			added++
		}
		if len(episodes) > 5000 {
			return nil, errors.New("优酷正片数量异常")
		}
		if !current.more {
			return finish_series_details(first, episodes, video_id)
		}
		if added == 0 || current.next_session == "" || seen_sessions[current.next_session] {
			return nil, errors.New("优酷分集分页未继续推进，不能将不完整列表当成全集")
		}
		seen_sessions[current.next_session] = true
		next_session = current.next_session
	}
	return nil, errors.New("优酷分集分页超过限制")
}

func parse_catalogue_page(raw []byte, show_id, video_id string, first bool) (catalogue_page, error) {
	var envelope map[string]struct {
		Success bool           `json:"success"`
		Data    catalogue_node `json:"data"`
	}
	var page catalogue_page
	if json.Unmarshal(raw, &envelope) != nil || !envelope["2019030100"].Success {
		return page, errors.New("优酷剧集详情接口未返回成功数据")
	}
	root := envelope["2019030100"].Data
	var root_data struct {
		Extra catalogue_extra `json:"extra"`
	}
	if json.Unmarshal(root.Data, &root_data) != nil {
		return page, errors.New("优酷剧集详情数据无效")
	}
	page.extra = root_data.Extra
	if first && (!youku_show_id_pattern.MatchString(page.extra.ShowID) || page.extra.ShowName == "") {
		return page, errors.New("优酷详情缺少可核对的剧集身份")
	}
	if page.extra.EpisodeTotal < 0 || page.extra.EpisodeTotal > 5000 || page.extra.EpisodeLast < 0 || page.extra.EpisodeLast > 5000 {
		return page, errors.New("优酷声明的剧集集数无效")
	}
	if show_id != "" && page.extra.ShowID != "" && show_id != page.extra.ShowID {
		return page, errors.New("优酷详情与请求的剧集编号不匹配")
	}
	if video_id != "" && page.extra.VideoID != "" && video_id != page.extra.VideoID {
		return page, errors.New("优酷详情与请求的视频编号不匹配")
	}
	if show_id == "" {
		show_id = page.extra.ShowID
	}
	page.profile = scraper.Profile{ID: sourceutil.FirstText(string(page.extra.ShowLongID), show_id), Kind: "video", Title: sourceutil.CleanText(page.extra.ShowName), CoverURL: catalogue_image(page.extra.ShowVertical), BackdropURL: catalogue_image(page.extra.ShowImage), ReleaseDate: catalogue_date(page.extra.ShowReleaseTime), EpisodeCount: page.extra.EpisodeTotal, Tagline: sourceutil.CleanText(page.extra.ShowSubtitle), Homepage: "https://v.youku.com/v_nextstage/id_" + show_id + ".html"}
	switch page.extra.ShowCategory {
	case "电影":
		page.profile.Kind = "movie"
	case "电视剧", "动漫", "综艺", "少儿":
		page.profile.Kind = "series"
	}
	if page.profile.Kind == "video" && page.extra.EpisodeTotal > 1 {
		page.profile.Kind = "series"
	}
	if page.extra.Completed != nil {
		page.profile.Status = "连载中"
		if *page.extra.Completed {
			page.profile.Status = "已完结"
		}
	}
	components := 0
	var walk func(catalogue_node, int) error
	walk = func(node catalogue_node, depth int) error {
		if depth > 8 {
			return errors.New("优酷剧集详情层级异常")
		}
		switch node.Type {
		case 10001:
			for _, child := range node.Nodes {
				if err := walk(child, depth+1); err != nil {
					return err
				}
			}
		case 20009:
			read_catalogue_intro(&page.profile, node)
		case 10215:
			if len(page.profile.Credits) == 0 {
				read_catalogue_intro(&page.profile, node)
			}
		case 10013:
			components++
			if components > 1 {
				return errors.New("优酷详情包含多个选集组件，无法确认正片列表")
			}
			var data struct {
				Session  json.RawMessage `json:"session"`
				Subtitle string          `json:"subtitle"`
			}
			if json.Unmarshal(node.Data, &data) != nil {
				return errors.New("优酷选集组件无效")
			}
			page.profile.Tips = sourceutil.CleanText(data.Subtitle)
			page.more = node.More
			if page.more {
				if len(data.Session) > 64<<10 {
					return errors.New("优酷分集分页参数过大")
				}
				session, err := mtop_json_object(data.Session)
				if err != nil {
					return errors.New("优酷分集列表缺少有效分页参数")
				}
				page.next_session = string(session)
			}
			for _, child := range node.Nodes {
				episode, regular, err := read_catalogue_episode(child, show_id)
				if err != nil {
					return err
				}
				if regular {
					page.episodes = append(page.episodes, episode)
				}
			}
		}
		return nil
	}
	if err := walk(root, 0); err != nil {
		return page, err
	}
	if components == 0 && page.profile.Kind == "movie" && youku_id_pattern.MatchString(page.extra.VideoID) {
		page.episodes = []scraper.Episode{{ID: page.extra.VideoID, SourceURL: "https://v.youku.com/v_show/id_" + page.extra.VideoID + ".html", Title: sourceutil.CleanText(page.extra.VideoTitle), CoverURL: catalogue_image(page.extra.VideoImage), Duration: page.extra.Duration}}
	} else if components == 0 {
		return page, errors.New("优酷未返回可确认的正片选集组件")
	}
	return page, nil
}

func read_catalogue_intro(profile *scraper.Profile, node catalogue_node) {
	seen := make(map[string]bool)
	for _, child := range node.Nodes {
		switch child.Type {
		case 20010:
			var data struct {
				Title    string `json:"introTitle"`
				Subtitle string `json:"introSubTitle"`
				Desc     string `json:"desc"`
				Image    string `json:"showImgV"`
				Update   string `json:"updateInfo"`
			}
			if json.Unmarshal(child.Data, &data) == nil {
				profile.Title = sourceutil.CleanText(sourceutil.FirstText(data.Title, profile.Title))
				profile.Description = sourceutil.CleanText(data.Desc)
				profile.CoverURL = sourceutil.FirstText(catalogue_image(data.Image), profile.CoverURL)
				profile.Tips = sourceutil.CleanText(data.Update)
				parts := strings.Split(data.Subtitle, "·")
				for index, part := range parts {
					part = strings.TrimSpace(part)
					if year, err := strconv.Atoi(part); err == nil && len(part) == 4 && year >= 1800 && year <= 2200 {
						profile.ReleaseYear = year
						if index > 0 && parts[index-1] != "" {
							profile.Countries = []string{strings.TrimSpace(parts[index-1])}
						}
						for _, genre := range parts[index+1:] {
							if genre = sourceutil.CleanText(genre); genre != "" {
								profile.Genres = append(profile.Genres, genre)
							}
						}
						break
					}
				}
			}
		case 10011, 10216:
			var data struct {
				ID       sourceutil.TextValue `json:"personId"`
				Name     string               `json:"title"`
				Subtitle string               `json:"subtitle"`
				Image    string               `json:"img"`
			}
			if json.Unmarshal(child.Data, &data) != nil || data.Name == "" || data.Subtitle == "" {
				continue
			}
			credit := scraper.Credit{ID: sourceutil.FirstText(string(data.ID), string(child.ID)), Name: sourceutil.CleanText(data.Name), AvatarURL: catalogue_image(data.Image), SortOrder: len(profile.Credits)}
			switch {
			case data.Subtitle == "导演":
				credit.Role = "director"
			case data.Subtitle == "编剧":
				credit.Role = "writer"
			case strings.HasPrefix(data.Subtitle, "饰"):
				credit.Role, credit.Character = "actor", sourceutil.CleanText(strings.TrimPrefix(data.Subtitle, "饰"))
			case data.Subtitle == "主演" || data.Subtitle == "演员":
				credit.Role = "actor"
			default:
				credit.Role = sourceutil.CleanText(data.Subtitle)
			}
			key := credit.ID + "\x00" + credit.Name + "\x00" + credit.Role + "\x00" + credit.Character
			if !seen[key] {
				profile.Credits = append(profile.Credits, credit)
				seen[key] = true
			}
		}
	}
}

func read_catalogue_episode(node catalogue_node, show_id string) (scraper.Episode, bool, error) {
	var data struct {
		Title       string               `json:"title"`
		Subtitle    string               `json:"subtitle"`
		Image       string               `json:"img"`
		Stage       sourceutil.TextValue `json:"stage"`
		VideoType   string               `json:"videoType"`
		EpisodeInfo struct {
			Regular bool `json:"isZp"`
		} `json:"episodeInfo"`
		Action struct {
			Type  string `json:"type"`
			Value string `json:"value"`
			Extra struct {
				ShowID string `json:"showId"`
			} `json:"extra"`
		} `json:"action"`
		Mark struct {
			Data struct {
				Text string `json:"text"`
				Icon string `json:"iconfont"`
			} `json:"data"`
		} `json:"mark"`
	}
	if json.Unmarshal(node.Data, &data) != nil {
		return scraper.Episode{}, false, errors.New("优酷分集数据无效")
	}
	if !data.EpisodeInfo.Regular || data.VideoType != "正片" {
		return scraper.Episode{}, false, nil
	}
	order, err := strconv.Atoi(string(data.Stage))
	if err != nil || order < 1 || order > 5000 || data.Action.Type != "JUMP_TO_VIDEO" || !youku_id_pattern.MatchString(data.Action.Value) {
		return scraper.Episode{}, false, errors.New("优酷正片编号、集序或跳转类型无效")
	}
	if data.Action.Extra.ShowID != "" && data.Action.Extra.ShowID != show_id {
		return scraper.Episode{}, false, errors.New("优酷正片所属剧集与详情不匹配")
	}
	if node.ID != "" {
		identity := youku_playback{}
		identity.Data.Video.ID = node.ID
		if !playback_matches(identity, data.Action.Value) {
			return scraper.Episode{}, false, errors.New("优酷正片编码编号与数值编号不匹配")
		}
	}
	return scraper.Episode{ID: data.Action.Value, SourceURL: "https://v.youku.com/v_show/id_" + data.Action.Value + ".html", Title: sourceutil.CleanText(data.Title), Order: order, CoverURL: catalogue_image(data.Image), Description: sourceutil.CleanText(data.Subtitle), Badge: sourceutil.CleanText(sourceutil.FirstText(data.Mark.Data.Text, data.Mark.Data.Icon))}, true, nil
}

func finish_series_details(first catalogue_page, episodes map[string]scraper.Episode, video_id string) (*SeriesDetails, error) {
	if len(episodes) == 0 {
		return nil, errors.New("优酷剧集尚未提供正片列表")
	}
	if video_id != "" {
		if _, ok := episodes[video_id]; !ok {
			return nil, errors.New("优酷正片列表不包含请求的视频")
		}
	}
	result := &SeriesDetails{SourceURL: first.profile.Homepage, Profile: first.profile, ListingComplete: true}
	for _, episode := range episodes {
		if episode.ID == first.extra.VideoID {
			episode.ReleaseDate = catalogue_date(first.extra.VideoPublishTime)
			if episode.Duration == 0 && first.extra.Duration > 0 {
				episode.Duration = first.extra.Duration
			}
		}
		result.Episodes = append(result.Episodes, episode)
	}
	sort.Slice(result.Episodes, func(i, j int) bool { return result.Episodes[i].Order < result.Episodes[j].Order })
	if result.Profile.Kind == "series" {
		for index, episode := range result.Episodes {
			if episode.Order != index+1 {
				return nil, errors.New("优酷正片列表存在缺集，不能当成完整分集列表")
			}
		}
		if first.extra.EpisodeLast > len(episodes) || (first.extra.Completed != nil && *first.extra.Completed && first.extra.EpisodeTotal > len(episodes)) || (first.extra.EpisodeTotal > 0 && len(episodes) > first.extra.EpisodeTotal) {
			return nil, errors.New("优酷正片列表与平台声明的集数不一致")
		}
	}
	result.Profile.AvailableEpisodeCount = len(episodes)
	result.Profile.Metadata, _ = json.Marshal(map[string]any{"youku_show_id": first.extra.ShowID, "youku_show_long_id": first.extra.ShowLongID, "episodes": result.Episodes, "listing_complete": true})
	return result, nil
}

func catalogue_date(value string) string {
	if len(value) < 10 {
		return ""
	}
	if _, err := time.Parse("2006-01-02", value[:10]); err != nil {
		return ""
	}
	return value[:10]
}

func catalogue_image(value string) string {
	address := sourceutil.PublicImage(value)
	if strings.HasPrefix(address, "http://") {
		address = "https://" + strings.TrimPrefix(address, "http://")
	}
	return address
}
