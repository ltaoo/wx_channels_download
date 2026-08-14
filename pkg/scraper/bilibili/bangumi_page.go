package bilibili

import (
	"fmt"
	"net/url"
	"strconv"
)

const bangumi_page_api_url = "https://api.bilibili.com/pgc/view/web/ep/page"
const bangumi_season_api_url = "https://api.bilibili.com/pgc/view/web/season"

// BangumiPageResponse is the episode page API response.
type BangumiPageResponse struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Result  BangumiPageResult `json:"result"`
}

// BangumiPageResult contains the paginated sections and target locator.
type BangumiPageResult struct {
	Locator      BangumiPageLocator       `json:"locator"`
	Sections     []BangumiPageSection     `json:"sections"`
	SectionsMeta []BangumiPageSectionMeta `json:"sections_meta"`
}

// BangumiPageLocator identifies the page containing the requested episode.
type BangumiPageLocator struct {
	IndexInPage int   `json:"index_in_page"`
	PageIndex   int   `json:"page_index"`
	SectionID   int64 `json:"section_id"`
	TargetEPID  int64 `json:"target_ep_id"`
}

// BangumiPageSection groups pages of episodes.
type BangumiPageSection struct {
	PageMode  int               `json:"page_mode"`
	Pages     []BangumiPage     `json:"pages"`
	PagesMeta []BangumiPageMeta `json:"pages_meta"`
	SectionID int64             `json:"section_id"`
}

// BangumiPage contains one page of episodes.
type BangumiPage struct {
	Episodes  []BangumiPageEpisode `json:"episodes"`
	PageIndex int                  `json:"page_index"`
}

// BangumiPageMeta describes a page range.
type BangumiPageMeta struct {
	PageIndex int    `json:"page_index"`
	Title     string `json:"title"`
}

// BangumiPageSectionMeta describes a section returned by the page API.
type BangumiPageSectionMeta struct {
	Attr       int     `json:"attr"`
	EpisodeID  int64   `json:"episode_id"`
	EpisodeIDs []int64 `json:"episode_ids"`
	ID         int64   `json:"id"`
	Title      string  `json:"title"`
	Type       int     `json:"type"`
	Type2      int     `json:"type2"`
}

// BangumiPageEpisode contains the page metadata for one episode.
type BangumiPageEpisode struct {
	AID                int64                `json:"aid"`
	Badge              string               `json:"badge"`
	BadgeInfo          BangumiBadgeInfo     `json:"badge_info"`
	BadgeType          int                  `json:"badge_type"`
	BVID               string               `json:"bvid"`
	CID                int64                `json:"cid"`
	Cover              string               `json:"cover"`
	Dimension          BangumiDimension     `json:"dimension"`
	Duration           int64                `json:"duration"`
	EnableVT           bool                 `json:"enable_vt"`
	EpisodeID          int64                `json:"ep_id"`
	From               string               `json:"from"`
	IconFont           BangumiIconFont      `json:"icon_font"`
	ID                 int64                `json:"id"`
	IsViewHide         bool                 `json:"is_view_hide"`
	Link               string               `json:"link"`
	LongTitle          string               `json:"long_title"`
	PublishTime        int64                `json:"pub_time"`
	PV                 int64                `json:"pv"`
	ReleaseDate        string               `json:"release_date"`
	Rights             BangumiEpisodeRights `json:"rights"`
	SectionType        int                  `json:"section_type"`
	ShareCopy          string               `json:"share_copy"`
	ShareURL           string               `json:"share_url"`
	ShortLink          string               `json:"short_link"`
	ShowDRMLoginDialog bool                 `json:"showDrmLoginDialog"`
	ShowTitle          string               `json:"show_title"`
	Skip               BangumiEpisodeSkip   `json:"skip"`
	Stat               BangumiEpisodeStat   `json:"stat"`
	StatForUnity       BangumiStatForUnity  `json:"stat_for_unity"`
	Status             int                  `json:"status"`
	Subtitle           string               `json:"subtitle"`
	Title              string               `json:"title"`
	VID                string               `json:"vid"`
}

// BangumiBadgeInfo contains episode badge colors and text.
type BangumiBadgeInfo struct {
	BackgroundColor      string `json:"bg_color"`
	BackgroundColorNight string `json:"bg_color_night"`
	Text                 string `json:"text"`
}

// BangumiDimension contains the source video's dimensions.
type BangumiDimension struct {
	Height int `json:"height"`
	Rotate int `json:"rotate"`
	Width  int `json:"width"`
}

// BangumiIconFont is a display icon and its formatted text.
type BangumiIconFont struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// BangumiEpisodeRights contains episode playback permissions.
type BangumiEpisodeRights struct {
	AllowDM       int `json:"allow_dm"`
	AllowDownload int `json:"allow_download"`
	AreaLimit     int `json:"area_limit"`
	CacheAuth     int `json:"cache_auth"`
}

// BangumiEpisodeSkip contains opening and ending skip ranges.
type BangumiEpisodeSkip struct {
	Ending  BangumiTimeRange `json:"ed"`
	Opening BangumiTimeRange `json:"op"`
}

// BangumiTimeRange is a start/end range in seconds.
type BangumiTimeRange struct {
	End   int64 `json:"end"`
	Start int64 `json:"start"`
}

// BangumiEpisodeStat contains episode engagement counters.
type BangumiEpisodeStat struct {
	Coin     int64 `json:"coin"`
	Danmakus int64 `json:"danmakus"`
	Likes    int64 `json:"likes"`
	Play     int64 `json:"play"`
	Reply    int64 `json:"reply"`
	VT       int64 `json:"vt"`
}

// BangumiStatForUnity contains counters and their display values.
type BangumiStatForUnity struct {
	Coin    int64                   `json:"coin"`
	Danmaku BangumiStatDisplayValue `json:"danmaku"`
	Likes   int64                   `json:"likes"`
	Reply   int64                   `json:"reply"`
	VT      BangumiStatDisplayValue `json:"vt"`
}

// BangumiStatDisplayValue contains a numeric counter and formatted text.
type BangumiStatDisplayValue struct {
	Icon     string `json:"icon"`
	PureText string `json:"pure_text"`
	Text     string `json:"text"`
	Value    int64  `json:"value"`
}

// TargetEpisode returns the episode selected by the response locator.
func (response *BangumiPageResponse) TargetEpisode() *BangumiPageEpisode {
	if response == nil {
		return nil
	}
	target_ep_id := response.Result.Locator.TargetEPID
	for section_index := range response.Result.Sections {
		section := &response.Result.Sections[section_index]
		for page_index := range section.Pages {
			page := &section.Pages[page_index]
			for episode_index := range page.Episodes {
				episode := &page.Episodes[episode_index]
				if episode.EpisodeID == target_ep_id || (target_ep_id == 0 && episode_index == response.Result.Locator.IndexInPage) {
					return episode
				}
			}
		}
	}
	return nil
}

func (c *Client) fetch_bangumi_page(ep_id int64) (*BangumiPageResponse, error) {
	if ep_id <= 0 {
		return nil, fmt.Errorf("B站番剧页面数据缺少 ep_id")
	}
	query := url.Values{}
	query.Set("ep_id", strconv.FormatInt(ep_id, 10))
	query.Set("web_location", "666.25")
	api_url := bangumi_page_api_url + "?" + query.Encode()
	var response BangumiPageResponse
	if err := c.do_get(api_url, &response); err != nil {
		return nil, fmt.Errorf("获取B站番剧页面数据失败: %w", err)
	}
	if response.Code != 0 {
		return nil, fmt.Errorf("B站番剧页面API错误: code=%d message=%s", response.Code, response.Message)
	}
	if response.TargetEpisode() == nil {
		return nil, fmt.Errorf("B站番剧页面数据中未找到 ep=%d", ep_id)
	}
	return &response, nil
}

func (c *Client) fetch_bangumi_season(ep_id int64) (*PGCSeasonResponse, error) {
	if ep_id <= 0 {
		return nil, fmt.Errorf("B站番剧季度数据缺少 ep_id")
	}
	query := url.Values{}
	query.Set("ep_id", strconv.FormatInt(ep_id, 10))
	api_url := bangumi_season_api_url + "?" + query.Encode()
	var response PGCSeasonResponse
	if err := c.do_get(api_url, &response); err != nil {
		return nil, fmt.Errorf("获取B站番剧季度数据失败: %w", err)
	}
	if response.Code != 0 {
		return nil, fmt.Errorf("B站番剧季度API错误: code=%d message=%s", response.Code, response.Message)
	}
	if response.Result.SeasonID <= 0 {
		return nil, fmt.Errorf("B站番剧季度数据中未找到 season_id")
	}
	return &response, nil
}
