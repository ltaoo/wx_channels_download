package bilibili

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	pathpkg "path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"wx_channel/pkg/scraper/internal/homepage"
)

const (
	user_video_page_size = 30
	max_user_video_page  = 1000000
)

var (
	user_video_url_re = regexp.MustCompile(`^/(\d+)/upload/video/?$`)
	wbi_mixin_table   = []int{
		46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35,
		27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13,
		37, 48, 7, 16, 24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4,
		22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11, 36, 20, 34, 44, 52,
	}
)

// UserVideoItem is one video shown on a Bilibili user's upload page.
type UserVideoItem struct {
	BVID         string `json:"bvid"`
	Title        string `json:"title"`
	Description  string `json:"description,omitempty"`
	CoverURL     string `json:"cover_url,omitempty"`
	CreatedAt    int64  `json:"created_at,omitempty"`
	PlayCount    int64  `json:"play_count,omitempty"`
	CommentCount int64  `json:"comment_count,omitempty"`
}

// UserVideoList is one page from a Bilibili user's /upload/video section.
type UserVideoList struct {
	Source     string          `json:"source"`
	OwnerMID   string          `json:"owner_mid"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
	TotalCount int             `json:"total_count"`
	Items      []UserVideoItem `json:"items"`
	HasNext    bool            `json:"has_next"`
	NextPage   int             `json:"next_page,omitempty"`
}

type user_video_api_response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List struct {
			VList []struct {
				BVID        string `json:"bvid"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Pic         string `json:"pic"`
				Created     int64  `json:"created"`
				Play        int64  `json:"play"`
				Comment     int64  `json:"comment"`
			} `json:"vlist"`
		} `json:"list"`
		Page struct {
			Count int `json:"count"`
			Page  int `json:"pn"`
			Size  int `json:"ps"`
		} `json:"page"`
	} `json:"data"`
}

type wbi_nav_response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		WBIImg struct {
			ImgURL string `json:"img_url"`
			SubURL string `json:"sub_url"`
		} `json:"wbi_img"`
	} `json:"data"`
}

// FetchVideoListOfUser fetches one page of videos from a Bilibili
// space.bilibili.com/{mid}/upload/video URL.
func (c *Client) FetchVideoListOfUser(raw_url string, page int) (*UserVideoList, error) {
	return c.fetch_video_list_of_user(raw_url, page, true)
}

func (c *Client) fetch_video_list_of_user(raw_url string, page int, allow_public_retry bool) (*UserVideoList, error) {
	if c == nil {
		return nil, fmt.Errorf("bilibili client is nil")
	}
	owner_mid, canonical_url, err := parse_user_video_list_url(raw_url, page)
	if err != nil {
		return nil, err
	}

	var nav_response wbi_nav_response
	if err := c.do_get("https://api.bilibili.com/x/web-interface/nav", &nav_response); err != nil {
		return nil, fmt.Errorf("fetch Bilibili WBI key: %w", err)
	}
	img_url := strings.TrimSpace(nav_response.Data.WBIImg.ImgURL)
	sub_url := strings.TrimSpace(nav_response.Data.WBIImg.SubURL)
	if img_url == "" || sub_url == "" {
		return nil, fmt.Errorf("Bilibili WBI key API returned %d: %s", nav_response.Code, nav_response.Message)
	}

	query := url.Values{
		"mid":   []string{owner_mid},
		"pn":    []string{strconv.Itoa(page)},
		"ps":    []string{strconv.Itoa(user_video_page_size)},
		"order": []string{"pubdate"},
	}
	signed_query, err := sign_wbi_query(query, img_url, sub_url, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	var response user_video_api_response
	api_url := "https://api.bilibili.com/x/space/wbi/arc/search?" + signed_query
	if err := c.do_get(api_url, &response); err != nil {
		return nil, fmt.Errorf("fetch Bilibili user videos: %w", err)
	}
	if response.Code == -101 && allow_public_retry && c.has_credentials() {
		return c.without_credentials().fetch_video_list_of_user(raw_url, page, false)
	}
	if response.Code != 0 {
		return nil, fmt.Errorf("Bilibili user video API returned %d: %s", response.Code, response.Message)
	}

	items := make([]UserVideoItem, 0, len(response.Data.List.VList))
	for _, api_item := range response.Data.List.VList {
		bvid := strings.TrimSpace(api_item.BVID)
		if bvid == "" {
			continue
		}
		items = append(items, UserVideoItem{
			BVID:         bvid,
			Title:        strings.TrimSpace(api_item.Title),
			Description:  strings.TrimSpace(api_item.Description),
			CoverURL:     strings.TrimSpace(api_item.Pic),
			CreatedAt:    api_item.Created,
			PlayCount:    api_item.Play,
			CommentCount: api_item.Comment,
		})
	}
	total_count := response.Data.Page.Count
	if total_count < len(items) {
		total_count = len(items)
	}
	has_next := page*user_video_page_size < total_count
	result := &UserVideoList{
		Source: canonical_url, OwnerMID: owner_mid, Page: page,
		PageSize: user_video_page_size, TotalCount: total_count,
		Items: items, HasNext: has_next,
	}
	if has_next {
		result.NextPage = page + 1
	}
	return result, nil
}

func (c *Client) has_credentials() bool {
	return c != nil && (strings.TrimSpace(c.cookie) != "" || c.cookie_provider != nil)
}

func (c *Client) without_credentials() *Client {
	headers := make(map[string]string, len(c.headers))
	for header_name, header_value := range c.headers {
		headers[header_name] = header_value
	}
	return &Client{
		http_client: c.http_client,
		headers:     headers,
		logger:      c.logger,
		file_cache:  c.file_cache,
	}
}

func parse_user_video_list_url(raw_url string, page int) (string, string, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || parsed_url.Hostname() == "" {
		return "", "", fmt.Errorf("invalid Bilibili user video list URL")
	}
	if parsed_url.Scheme != "https" && parsed_url.Scheme != "http" {
		return "", "", fmt.Errorf("unsupported Bilibili user video list URL scheme")
	}
	if !strings.EqualFold(parsed_url.Hostname(), "space.bilibili.com") {
		return "", "", fmt.Errorf("unsupported Bilibili user video list host %q", parsed_url.Hostname())
	}
	matches := user_video_url_re.FindStringSubmatch(parsed_url.EscapedPath())
	if len(matches) != 2 {
		return "", "", fmt.Errorf("unsupported Bilibili user video list URL")
	}
	if page < 1 || page > max_user_video_page {
		return "", "", fmt.Errorf("Bilibili user video page must be between 1 and %d", max_user_video_page)
	}
	owner_mid, err := url.PathUnescape(matches[1])
	if err != nil || strings.TrimSpace(owner_mid) == "" {
		return "", "", fmt.Errorf("invalid Bilibili user MID")
	}
	canonical_url := "https://space.bilibili.com/" + owner_mid + "/upload/video"
	if page > 1 {
		canonical_url += "?page=" + strconv.Itoa(page)
	}
	return owner_mid, canonical_url, nil
}

func sign_wbi_query(query url.Values, img_url string, sub_url string, timestamp int64) (string, error) {
	origin_key := wbi_filename(img_url) + wbi_filename(sub_url)
	if len(origin_key) < 64 {
		return "", fmt.Errorf("Bilibili WBI key is incomplete")
	}
	var mixed_key strings.Builder
	for _, index := range wbi_mixin_table {
		if index < len(origin_key) {
			mixed_key.WriteByte(origin_key[index])
		}
		if mixed_key.Len() == 32 {
			break
		}
	}

	clean_query := url.Values{}
	query.Set("wts", strconv.FormatInt(timestamp, 10))
	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := strings.NewReplacer("!", "", "'", "", "(", "", ")", "", "*", "").Replace(query.Get(key))
		clean_query.Set(key, value)
	}
	encoded_query := clean_query.Encode()
	digest := md5.Sum([]byte(encoded_query + mixed_key.String()))
	clean_query.Set("w_rid", hex.EncodeToString(digest[:]))
	return clean_query.Encode(), nil
}

func wbi_filename(raw_url string) string {
	parsed_url, err := url.Parse(raw_url)
	if err != nil {
		return ""
	}
	name := pathpkg.Base(parsed_url.Path)
	return strings.TrimSuffix(name, pathpkg.Ext(name))
}

const (
	HomeKindVideo      = "video"
	HomeKindArticle    = "article"
	HomeKindCollection = "collection"
	HomeKindPost       = "post"
)

// HomeItem is one normalized entry from a Bilibili account-home scope.
type HomeItem struct {
	ID           string
	Kind         string
	Title        string
	Description  string
	CoverURL     string
	SourceURL    string
	PublishTime  int64
	ViewCount    int64
	CommentCount int64
	LikeCount    int64
	CollectCount int64
}

// FetchHomeContext retrieves the first page of one Bilibili account-home scope.
func (c *Client) FetchHomeContext(fetch_context context.Context, mid string, scope string) ([]HomeItem, error) {
	if c == nil {
		return nil, fmt.Errorf("bilibili client is nil")
	}
	mid = strings.TrimSpace(mid)
	if mid == "" {
		return nil, fmt.Errorf("Bilibili MID is empty")
	}
	scope = strings.TrimSpace(scope)
	if scope == "videos" {
		scope = "video"
	}
	api_url, err := c.home_api_url(fetch_context, mid, scope)
	if err != nil {
		return nil, err
	}
	response_json, err := c.home_request(fetch_context, api_url, scope, mid, false)
	if err != nil {
		return nil, err
	}
	if response_code(response_json) == -101 {
		response_json, err = c.home_request(fetch_context, api_url, scope+"-public", mid, true)
		if err != nil {
			return nil, err
		}
	}
	return parse_home_response(response_json, scope)
}

func (c *Client) home_api_url(fetch_context context.Context, mid string, scope string) (string, error) {
	query := url.Values{}
	switch scope {
	case "video":
		query.Set("mid", mid)
		query.Set("pn", "1")
		query.Set("ps", "30")
		query.Set("order", "pubdate")
		signed_query, err := c.home_wbi_query(fetch_context, query)
		if err != nil {
			return "", err
		}
		return "https://api.bilibili.com/x/space/wbi/arc/search?" + signed_query, nil
	case "dynamic":
		query.Set("host_mid", mid)
		return "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/space?" + query.Encode(), nil
	case "articles":
		query.Set("mid", mid)
		query.Set("pn", "1")
		query.Set("ps", "30")
		query.Set("sort", "publish_time")
		return "https://api.bilibili.com/x/space/article?" + query.Encode(), nil
	case "lists":
		query.Set("mid", mid)
		query.Set("page_num", "1")
		query.Set("page_size", "30")
		return "https://api.bilibili.com/x/polymer/web-space/seasons_series_list?" + query.Encode(), nil
	case "bangumi":
		query.Set("type", "1")
		query.Set("vmid", mid)
		query.Set("pn", "1")
		query.Set("ps", "30")
		return "https://api.bilibili.com/x/space/bangumi/follow/list?" + query.Encode(), nil
	default:
		return "", fmt.Errorf("unsupported Bilibili home scope: %s", scope)
	}
}

func (c *Client) home_wbi_query(fetch_context context.Context, query url.Values) (string, error) {
	nav_json, err := c.home_request(fetch_context, "https://api.bilibili.com/x/web-interface/nav", "wbi", "nav", false)
	if err != nil {
		return "", fmt.Errorf("fetch Bilibili WBI key: %w", err)
	}
	if response_code(nav_json) == -101 {
		nav_json, err = c.home_request(fetch_context, "https://api.bilibili.com/x/web-interface/nav", "wbi-public", "nav", true)
		if err != nil {
			return "", fmt.Errorf("fetch public Bilibili WBI key: %w", err)
		}
	}
	var nav_response wbi_nav_response
	if err := json.Unmarshal(nav_json, &nav_response); err != nil {
		return "", fmt.Errorf("decode Bilibili WBI key: %w", err)
	}
	img_url := strings.TrimSpace(nav_response.Data.WBIImg.ImgURL)
	sub_url := strings.TrimSpace(nav_response.Data.WBIImg.SubURL)
	if nav_response.Code != 0 && (img_url == "" || sub_url == "") {
		return "", fmt.Errorf("Bilibili WBI key API returned %d: %s", nav_response.Code, nav_response.Message)
	}
	return sign_wbi_query(query, img_url, sub_url, time.Now().Unix())
}

func (c *Client) home_request(fetch_context context.Context, api_url string, scope string, account_id string, public bool) ([]byte, error) {
	headers := home_headers(account_id)
	var cookie_provider = c.cookie_provider
	if public {
		cookie_provider = nil
	} else if parsed_url, err := url.Parse(api_url); err == nil {
		if cookie_header, cookie_err := c.resolve_cookie(parsed_url); cookie_err != nil {
			return nil, cookie_err
		} else if cookie_header != "" {
			headers.Set("Cookie", cookie_header)
		}
	}
	return homepage.Request(fetch_context, homepage.RequestOptions{
		URL: api_url, Scope: scope, AccountID: account_id, Headers: headers,
		CookieReader: cookie_provider, Cache: c.file_cache,
	})
}

func home_headers(mid string) http.Header {
	return http.Header{
		"Accept": {"application/json, text/plain, */*"}, "Accept-Language": {"zh-CN,zh;q=0.9,en;q=0.8"},
		"Origin": {"https://space.bilibili.com"}, "Referer": {"https://space.bilibili.com/" + mid},
	}
}

func response_code(response_json []byte) int {
	var envelope struct {
		Code int `json:"code"`
	}
	if json.Unmarshal(response_json, &envelope) != nil {
		return 0
	}
	return envelope.Code
}

func parse_home_response(response_json []byte, scope string) ([]HomeItem, error) {
	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response_json, &envelope); err != nil {
		return nil, fmt.Errorf("decode Bilibili home %s: %w", scope, err)
	}
	if envelope.Code != 0 {
		return nil, fmt.Errorf("Bilibili home %s API returned %d: %s", scope, envelope.Code, envelope.Message)
	}
	switch scope {
	case "video":
		return parse_video_home(envelope.Data)
	case "articles":
		return parse_article_home(envelope.Data)
	case "lists":
		return parse_list_home(envelope.Data)
	case "bangumi":
		return parse_bangumi_home(envelope.Data)
	case "dynamic":
		return parse_dynamic_home(envelope.Data)
	default:
		return nil, fmt.Errorf("unsupported Bilibili home scope: %s", scope)
	}
}

func parse_video_home(data json.RawMessage) ([]HomeItem, error) {
	var payload struct {
		List struct {
			VList []struct {
				BVID        string `json:"bvid"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Pic         string `json:"pic"`
				Created     int64  `json:"created"`
				Play        int64  `json:"play"`
				Comment     int64  `json:"comment"`
			} `json:"vlist"`
		} `json:"list"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	items := make([]HomeItem, 0, len(payload.List.VList))
	for _, item := range payload.List.VList {
		items = append(items, HomeItem{ID: item.BVID, Kind: HomeKindVideo, Title: item.Title, Description: item.Description, CoverURL: item.Pic, SourceURL: "https://www.bilibili.com/video/" + item.BVID, PublishTime: item.Created, ViewCount: item.Play, CommentCount: item.Comment})
	}
	return items, nil
}

func parse_article_home(data json.RawMessage) ([]HomeItem, error) {
	var payload struct {
		Articles []struct {
			ID          int64  `json:"id"`
			Title       string `json:"title"`
			Summary     string `json:"summary"`
			BannerURL   string `json:"banner_url"`
			PublishTime int64  `json:"publish_time"`
			Stats       struct {
				View     int64 `json:"view"`
				Reply    int64 `json:"reply"`
				Like     int64 `json:"like"`
				Favorite int64 `json:"favorite"`
			} `json:"stats"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	items := make([]HomeItem, 0, len(payload.Articles))
	for _, item := range payload.Articles {
		id := strconv.FormatInt(item.ID, 10)
		items = append(items, HomeItem{ID: id, Kind: HomeKindArticle, Title: item.Title, Description: item.Summary, CoverURL: item.BannerURL, SourceURL: "https://www.bilibili.com/read/cv" + id, PublishTime: item.PublishTime, ViewCount: item.Stats.View, CommentCount: item.Stats.Reply, LikeCount: item.Stats.Like, CollectCount: item.Stats.Favorite})
	}
	return items, nil
}

func parse_list_home(data json.RawMessage) ([]HomeItem, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	entries := collect_maps_for_keys(payload, "seasons_list", "series_list")
	items := make([]HomeItem, 0, len(entries))
	seen := make(map[string]struct{})
	for _, entry := range entries {
		id := first_json_string(entry, "season_id", "series_id", "id")
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		items = append(items, HomeItem{ID: "list-" + id, Kind: HomeKindCollection, Title: first_json_string(entry, "name", "title"), CoverURL: first_json_string(entry, "cover"), SourceURL: "https://space.bilibili.com/lists/" + id})
	}
	return items, nil
}

func parse_bangumi_home(data json.RawMessage) ([]HomeItem, error) {
	var payload struct {
		List []map[string]any `json:"list"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	items := make([]HomeItem, 0, len(payload.List))
	for _, entry := range payload.List {
		id := first_json_string(entry, "season_id", "media_id")
		if id == "" {
			continue
		}
		source_url := first_json_string(entry, "url")
		if source_url == "" {
			source_url = "https://www.bilibili.com/bangumi/play/ss" + id
		}
		items = append(items, HomeItem{ID: "bangumi-" + id, Kind: HomeKindCollection, Title: first_json_string(entry, "title"), CoverURL: first_json_string(entry, "cover"), SourceURL: source_url})
	}
	return items, nil
}

func parse_dynamic_home(data json.RawMessage) ([]HomeItem, error) {
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	items := make([]HomeItem, 0, len(payload.Items))
	for _, entry := range payload.Items {
		if bvid := recursive_json_string(entry, "bvid"); bvid != "" {
			items = append(items, HomeItem{ID: bvid, Kind: HomeKindVideo, Title: recursive_json_string(entry, "title"), Description: recursive_json_string(entry, "desc"), CoverURL: recursive_json_string(entry, "cover"), SourceURL: "https://www.bilibili.com/video/" + bvid})
			continue
		}
		id := first_json_string(entry, "id_str", "id")
		if id == "" {
			continue
		}
		text := recursive_json_string(entry, "text")
		items = append(items, HomeItem{ID: "opus-" + id, Kind: HomeKindPost, Title: text, Description: text, CoverURL: recursive_json_string(entry, "cover"), SourceURL: "https://www.bilibili.com/opus/" + id})
	}
	return items, nil
}

func collect_maps_for_keys(value any, keys ...string) []map[string]any {
	key_set := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		key_set[key] = struct{}{}
	}
	result := make([]map[string]any, 0)
	var walk func(any)
	walk = func(current any) {
		switch typed_value := current.(type) {
		case map[string]any:
			for key, child := range typed_value {
				if _, ok := key_set[key]; ok {
					if list, list_ok := child.([]any); list_ok {
						for _, entry := range list {
							if item, item_ok := entry.(map[string]any); item_ok {
								result = append(result, item)
							}
						}
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range typed_value {
				walk(child)
			}
		}
	}
	walk(value)
	return result
}

func first_json_string(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if text := json_value_string(value[key]); text != "" {
			return text
		}
	}
	return ""
}

func recursive_json_string(value any, key string) string {
	switch typed_value := value.(type) {
	case map[string]any:
		if text := json_value_string(typed_value[key]); text != "" {
			return text
		}
		for _, child := range typed_value {
			if text := recursive_json_string(child, key); text != "" {
				return text
			}
		}
	case []any:
		for _, child := range typed_value {
			if text := recursive_json_string(child, key); text != "" {
				return text
			}
		}
	}
	return ""
}

func json_value_string(value any) string {
	switch typed_value := value.(type) {
	case string:
		return strings.TrimSpace(typed_value)
	case float64:
		return strconv.FormatInt(int64(typed_value), 10)
	case json.Number:
		return typed_value.String()
	default:
		return ""
	}
}
