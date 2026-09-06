package weibo

import (
	"context"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"wx_channel/pkg/scraper/internal/homepage"
)

var home_html_tag_pattern = regexp.MustCompile(`<[^>]+>`)

// HomeItem is one normalized status returned by a Weibo profile timeline.
type HomeItem struct {
	ExternalID   string
	MblogID      string
	Text         string
	CoverURL     string
	PublishTime  int64
	RepostCount  int64
	CommentCount int64
	LikeCount    int64
	HasVideo     bool
	HasPhotos    bool
}

// FetchHomeContext retrieves the first page of one Weibo profile scope.
func (c *Client) FetchHomeContext(fetch_context context.Context, uid string, scope string) ([]HomeItem, error) {
	if c == nil {
		return nil, fmt.Errorf("weibo client is not initialized")
	}
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return nil, fmt.Errorf("Weibo UID is empty")
	}
	scope = strings.TrimSpace(scope)
	if scope != "posts" && scope != "videos" && scope != "photos" {
		return nil, fmt.Errorf("unsupported Weibo home scope: %s", scope)
	}
	query := url.Values{"uid": {uid}, "page": {"1"}, "feature": {"0"}}
	if scope == "videos" {
		query.Set("feature", "4")
	}
	response_json, err := homepage.Request(fetch_context, homepage.RequestOptions{
		URL:   "https://weibo.com/ajax/statuses/mymblog?" + query.Encode(),
		Scope: scope, AccountID: uid, Headers: weibo_home_headers(uid),
		CookieReader: c.cookie_provider, Cache: c.file_cache,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch Weibo profile %s: %w", scope, err)
	}
	return parse_home_response(response_json, scope)
}

type home_status struct {
	ID        string   `json:"idstr"`
	MblogID   string   `json:"mblogid"`
	Text      string   `json:"text_raw"`
	TextHTML  string   `json:"text"`
	CreatedAt string   `json:"created_at"`
	Reposts   int64    `json:"reposts_count"`
	Comments  int64    `json:"comments_count"`
	Attitudes int64    `json:"attitudes_count"`
	PicIDs    []string `json:"pic_ids"`
	PicInfos  map[string]struct {
		Largest struct {
			URL string `json:"url"`
		} `json:"largest"`
		BMiddle struct {
			URL string `json:"url"`
		} `json:"bmiddle"`
	} `json:"pic_infos"`
	PageInfo struct {
		Type    string `json:"type"`
		PagePic struct {
			URL string `json:"url"`
		} `json:"page_pic"`
		MediaInfo struct {
			StreamURL string `json:"stream_url"`
		} `json:"media_info"`
	} `json:"page_info"`
}

func parse_home_response(response_json []byte, scope string) ([]HomeItem, error) {
	var response struct {
		OK      int    `json:"ok"`
		Message string `json:"message"`
		Data    struct {
			List []home_status `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response_json, &response); err != nil {
		return nil, fmt.Errorf("decode Weibo profile %s: %w", scope, err)
	}
	if response.OK != 1 {
		message := strings.TrimSpace(response.Message)
		if message == "" {
			message = "cookies.json 中的微博登录凭证无效或已过期"
		}
		return nil, fmt.Errorf("Weibo profile API rejected request: %s", message)
	}
	items := make([]HomeItem, 0, len(response.Data.List))
	for _, status := range response.Data.List {
		has_video := strings.EqualFold(status.PageInfo.Type, "video") || status.PageInfo.MediaInfo.StreamURL != ""
		has_photos := len(status.PicIDs) > 0 || len(status.PicInfos) > 0
		if scope == "videos" && !has_video || scope == "photos" && !has_photos {
			continue
		}
		items = append(items, normalize_home_status(status, has_video, has_photos))
	}
	return items, nil
}

func normalize_home_status(status home_status, has_video bool, has_photos bool) HomeItem {
	external_id := strings.TrimSpace(status.ID)
	if external_id == "" {
		external_id = strings.TrimSpace(status.MblogID)
	}
	text := strings.TrimSpace(status.Text)
	if text == "" {
		text = strings.Join(strings.Fields(home_html_tag_pattern.ReplaceAllString(stdhtml.UnescapeString(status.TextHTML), " ")), " ")
	}
	if len([]rune(text)) > 160 {
		text = string([]rune(text)[:160])
	}
	if text == "" {
		text = external_id
	}
	cover_url := strings.TrimSpace(status.PageInfo.PagePic.URL)
	if cover_url == "" {
		for _, picture := range status.PicInfos {
			cover_url = strings.TrimSpace(picture.Largest.URL)
			if cover_url == "" {
				cover_url = strings.TrimSpace(picture.BMiddle.URL)
			}
			if cover_url != "" {
				break
			}
		}
	}
	publish_time := int64(0)
	if value, err := time.Parse(time.RubyDate, status.CreatedAt); err == nil {
		publish_time = value.UnixMilli()
	}
	return HomeItem{
		ExternalID: external_id, MblogID: strings.TrimSpace(status.MblogID), Text: text,
		CoverURL: cover_url, PublishTime: publish_time, RepostCount: status.Reposts,
		CommentCount: status.Comments, LikeCount: status.Attitudes,
		HasVideo: has_video, HasPhotos: has_photos,
	}
}

func weibo_home_headers(uid string) http.Header {
	return http.Header{
		"Accept":           {"application/json, text/plain, */*"},
		"Accept-Language":  {"zh-CN,zh;q=0.9,en;q=0.8"},
		"Referer":          {"https://weibo.com/u/" + uid},
		"X-Requested-With": {"XMLHttpRequest"},
	}
}
