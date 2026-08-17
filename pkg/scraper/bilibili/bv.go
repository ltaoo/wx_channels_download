package bilibili

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"wx_channel/pkg/cache"
)

const (
	bilibili_nav_api         = "https://api.bilibili.com/x/web-interface/nav"
	bilibili_view_detail_api = "https://api.bilibili.com/x/web-interface/wbi/view/detail"
)

var bv_video_path_pattern = regexp.MustCompile(`^/video/(BV[0-9A-Za-z]{10})(?:/|$)`)

var bv_wbi_mixin_key_enc_tab = []int{
	46, 47, 18, 2, 53, 8, 23, 32,
	15, 50, 10, 31, 58, 3, 45, 35,
	27, 43, 5, 49, 33, 9, 42, 19,
	29, 28, 14, 39, 12, 38, 41, 13,
	37, 48, 7, 16, 24, 55, 40, 61,
	26, 17, 0, 1, 60, 51, 30, 4,
	22, 25, 54, 21, 56, 59, 6, 63,
	57, 62, 11, 36, 20, 34, 44, 52,
}

var bv_detail_browser_params = map[string]string{
	"need_view":        "1",
	"isGaiaAvoided":    "false",
	"web_location":     "1315873",
	"dm_img_list":      "[]",
	"dm_img_str":       "V2ViR0wgMS4wIChPcGVuR0wgRVMgMi4wIENocm9taXVtKQ",
	"dm_cover_img_str": "QU5HTEUgKEFwcGxlLCBBTkdMRSBNZXRhbCBSZW5kZXJlcjogQXBwbGUgTTEgUHJvLCBVbnNwZWNpZmllZCBWZXJzaW9uKUdvb2dsZSBJbmMuIChBcHBsZS",
	"dm_img_inter":     `{"ds":[],"wh":[3813,2896,89],"of":[469,938,469]}`,
}

// HTMLCacheFile describes one persisted Bilibili BV page response.
type HTMLCacheFile struct {
	Path string
	Size int64
}

// BVDetailResponse is the signed WBI video detail response.
type BVDetailResponse struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Msg     string       `json:"msg"`
	Data    BVDetailData `json:"data"`
}

// BVDetailData contains the view data returned by wbi/view/detail.
type BVDetailData struct {
	View VideoData `json:"View"`
}

// BVInitialData is the stable subset of window.__INITIAL_STATE__ used for
// publish time and uploader/account metadata.
type BVInitialData struct {
	BVID      string             `json:"bvid"`
	AID       int64              `json:"aid"`
	CID       int64              `json:"cid"`
	VideoData BVInitialVideoData `json:"videoData"`
	UpData    BVInitialUpData    `json:"upData"`
	Raw       json.RawMessage    `json:"-"`
}

type BVInitialVideoData struct {
	BVID    string         `json:"bvid"`
	Pubdate int64          `json:"pubdate"`
	Owner   BVInitialOwner `json:"owner"`
}

type BVInitialOwner struct {
	Mid  int64  `json:"mid"`
	Name string `json:"name"`
	Face string `json:"face"`
}

type BVInitialUpData struct {
	Mid         json.RawMessage `json:"mid"`
	Name        string          `json:"name"`
	Face        string          `json:"face"`
	Sign        string          `json:"sign"`
	Fans        int64           `json:"fans"`
	Description string          `json:"description"`
}

type bv_nav_response struct {
	Code int `json:"code"`
	Data struct {
		WBIImg struct {
			ImgURL string `json:"img_url"`
			SubURL string `json:"sub_url"`
		} `json:"wbi_img"`
	} `json:"data"`
}

// ParseBVURL extracts a BV identifier from a Bilibili video URL. Both full
// URLs and schemeless values such as bilibili.com/video/BV... are accepted.
func ParseBVURL(raw_url string) (string, bool) {
	candidate := strings.TrimSpace(raw_url)
	if candidate == "" {
		return "", false
	}
	if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}
	parsed_url, err := url.Parse(candidate)
	if err != nil {
		return "", false
	}
	host := strings.ToLower(parsed_url.Hostname())
	if host != "bilibili.com" && !strings.HasSuffix(host, ".bilibili.com") {
		return "", false
	}
	matches := bv_video_path_pattern.FindStringSubmatch(parsed_url.EscapedPath())
	if len(matches) != 2 {
		return "", false
	}
	return matches[1], true
}

var bv_initial_state_pattern = regexp.MustCompile(`window\.__INITIAL_STATE__\s*=`)

// ExtractBVInitialDataJSON extracts initial state from HTML or accepts a raw
// JSON object directly for initialdata fixtures.
func ExtractBVInitialDataJSON(page []byte) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(string(page))
	if strings.HasPrefix(trimmed, "{") {
		if !json.Valid([]byte(trimmed)) {
			return nil, fmt.Errorf("B站initialdata不是有效JSON")
		}
		return json.RawMessage(trimmed), nil
	}
	match := bv_initial_state_pattern.FindStringIndex(trimmed)
	if match == nil {
		return nil, fmt.Errorf("B站页面未找到__INITIAL_STATE__")
	}
	start := strings.IndexByte(trimmed[match[1]:], '{')
	if start < 0 {
		return nil, fmt.Errorf("B站initialdata对象起始位置不存在")
	}
	start += match[1]
	depth := 0
	in_string := false
	escaped := false
	for index := start; index < len(trimmed); index++ {
		character := trimmed[index]
		if in_string {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == '"' {
				in_string = false
			}
			continue
		}
		switch character {
		case '"':
			in_string = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				raw_json := trimmed[start : index+1]
				if !json.Valid([]byte(raw_json)) {
					return nil, fmt.Errorf("B站initialdata不是有效JSON")
				}
				return json.RawMessage(raw_json), nil
			}
		}
	}
	return nil, fmt.Errorf("B站initialdata对象未闭合")
}

// ParseBVInitialData parses the Bilibili page initial state.
func ParseBVInitialData(page []byte) (*BVInitialData, error) {
	raw_json, err := ExtractBVInitialDataJSON(page)
	if err != nil {
		return nil, err
	}
	var initial_data BVInitialData
	if err := json.Unmarshal(raw_json, &initial_data); err != nil {
		return nil, fmt.Errorf("解析B站initialdata失败: %w", err)
	}
	initial_data.Raw = append(json.RawMessage(nil), raw_json...)
	return &initial_data, nil
}

func bv_html_cache_relative_path(raw_url string) (string, error) {
	bvid, ok := ParseBVURL(raw_url)
	if !ok {
		return "", fmt.Errorf("不是有效的B站BV视频URL: %s", raw_url)
	}
	return filepath.ToSlash(filepath.Join("bv", bvid, "page.html")), nil
}

func (c *Client) read_cached_bv_html(raw_url string) ([]byte, bool, error) {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() {
		return nil, false, nil
	}
	relative_path, err := bv_html_cache_relative_path(raw_url)
	if err != nil {
		return nil, false, err
	}
	cached_data, read_err := c.file_cache.Read(relative_path)
	if errors.Is(read_err, os.ErrNotExist) {
		return nil, false, nil
	}
	if read_err != nil {
		return nil, false, fmt.Errorf("读取B站HTML缓存失败: %w", read_err)
	}
	if len(cached_data) == 0 {
		_ = c.file_cache.Remove(relative_path)
		return nil, false, nil
	}
	return cached_data, true, nil
}

func (c *Client) write_cached_bv_html(raw_url string, html_data []byte) error {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() {
		return nil
	}
	relative_path, err := bv_html_cache_relative_path(raw_url)
	if err != nil {
		return err
	}
	return c.file_cache.Write(relative_path, html_data)
}

func (c *Client) remove_cached_bv_html(raw_url string) error {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() {
		return nil
	}
	relative_path, err := bv_html_cache_relative_path(raw_url)
	if err != nil {
		return err
	}
	return c.file_cache.Remove(relative_path)
}

// LookupHTMLCache locates the cached BV page for raw_url without performing a
// network request. A nil result means the page is not cached.
func LookupHTMLCache(file_cache *cache.CacheProvider, raw_url string) (*HTMLCacheFile, error) {
	if file_cache == nil || !file_cache.Enabled() {
		return nil, nil
	}
	relative_path, err := bv_html_cache_relative_path(raw_url)
	if err != nil {
		return nil, err
	}
	cache_path, err := file_cache.Path(relative_path)
	if err != nil {
		return nil, err
	}
	file_info, stat_err := file_cache.Stat(relative_path)
	if errors.Is(stat_err, os.ErrNotExist) {
		return nil, nil
	}
	if stat_err != nil {
		return nil, fmt.Errorf("检查B站HTML缓存失败: %w", stat_err)
	}
	if !file_info.Mode().IsRegular() || file_info.Size() <= 0 {
		return nil, nil
	}
	return &HTMLCacheFile{Path: cache_path, Size: file_info.Size()}, nil
}

// ClearHTMLCache removes the cached BV page for raw_url.
func ClearHTMLCache(file_cache *cache.CacheProvider, raw_url string) (bool, error) {
	if file_cache == nil || !file_cache.Enabled() {
		return false, nil
	}
	relative_path, err := bv_html_cache_relative_path(raw_url)
	if err != nil {
		return false, err
	}
	if _, stat_err := file_cache.Stat(relative_path); errors.Is(stat_err, os.ErrNotExist) {
		return false, nil
	} else if stat_err != nil {
		return false, fmt.Errorf("检查B站HTML缓存失败: %w", stat_err)
	}
	if err := file_cache.Remove(relative_path); err != nil {
		return false, fmt.Errorf("清理B站HTML缓存失败: %w", err)
	}
	return true, nil
}

func (c *Client) fetch_bv_initial_data(raw_url string) (*BVInitialData, error) {
	cached_html, cached, err := c.read_cached_bv_html(raw_url)
	if err != nil {
		return nil, err
	}
	if cached {
		initial_data, parse_err := ParseBVInitialData(cached_html)
		if parse_err == nil {
			return initial_data, nil
		}
		_ = c.remove_cached_bv_html(raw_url)
	}

	candidate := strings.TrimSpace(raw_url)
	if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}
	request, err := http.NewRequest(http.MethodGet, candidate, nil)
	if err != nil {
		return nil, fmt.Errorf("创建B站页面请求失败: %w", err)
	}
	if err := c.apply_request_headers(request); err != nil {
		return nil, fmt.Errorf("创建B站页面请求头失败: %w", err)
	}
	response, err := c.http_client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("请求B站页面失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("请求B站页面失败: HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, bangumi_html_limit+1))
	if err != nil {
		return nil, fmt.Errorf("读取B站页面失败: %w", err)
	}
	if len(body) > bangumi_html_limit {
		return nil, fmt.Errorf("B站页面超过大小限制")
	}
	initial_data, err := ParseBVInitialData(body)
	if err != nil {
		return nil, err
	}
	if err := c.write_cached_bv_html(raw_url, body); err != nil {
		return nil, fmt.Errorf("写入B站HTML缓存失败: %w", err)
	}
	return initial_data, nil
}

func bv_wbi_key_from_url(raw_url string) (string, error) {
	parsed_url, err := url.Parse(raw_url)
	if err != nil {
		return "", fmt.Errorf("解析WBI图片地址失败: %w", err)
	}
	file_name := strings.Trim(strings.TrimSpace(parsed_url.Path), "/")
	if slash := strings.LastIndexByte(file_name, '/'); slash >= 0 {
		file_name = file_name[slash+1:]
	}
	key := strings.SplitN(file_name, ".", 2)[0]
	if key == "" {
		return "", fmt.Errorf("WBI图片地址缺少密钥: %s", raw_url)
	}
	return key, nil
}

func bv_build_mixin_key(img_key string, sub_key string) (string, error) {
	source := img_key + sub_key
	if len(source) <= 63 {
		return "", fmt.Errorf("WBI原始密钥长度不足")
	}
	var builder strings.Builder
	for _, index := range bv_wbi_mixin_key_enc_tab {
		builder.WriteByte(source[index])
	}
	return builder.String()[:32], nil
}

func bv_sign_wbi_params(params map[string]string, img_key string, sub_key string, timestamp int64) (url.Values, error) {
	mixin_key, err := bv_build_mixin_key(img_key, sub_key)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(params)+1)
	for key := range params {
		keys = append(keys, key)
	}
	keys = append(keys, "wts")
	sort.Strings(keys)
	values := make(url.Values, len(keys)+1)
	for _, key := range keys {
		value := strconv.FormatInt(timestamp, 10)
		if key != "wts" {
			value = params[key]
		}
		value = strings.Map(func(character rune) rune {
			if strings.ContainsRune("!'()*", character) {
				return -1
			}
			return character
		}, value)
		values.Set(key, value)
	}
	digest := md5.Sum([]byte(values.Encode() + mixin_key))
	values.Set("w_rid", hex.EncodeToString(digest[:]))
	return values, nil
}

func (c *Client) fetch_bv_wbi_keys() (string, string, error) {
	var response bv_nav_response
	if err := c.do_get(bilibili_nav_api, &response); err != nil {
		return "", "", fmt.Errorf("获取B站WBI密钥失败: %w", err)
	}
	if response.Data.WBIImg.ImgURL == "" || response.Data.WBIImg.SubURL == "" {
		return "", "", fmt.Errorf("B站导航接口未返回WBI密钥: code=%d", response.Code)
	}
	img_key, err := bv_wbi_key_from_url(response.Data.WBIImg.ImgURL)
	if err != nil {
		return "", "", err
	}
	sub_key, err := bv_wbi_key_from_url(response.Data.WBIImg.SubURL)
	if err != nil {
		return "", "", err
	}
	return img_key, sub_key, nil
}

// GetBVDetail retrieves the complete signed WBI detail payload for a BV URL.
func (c *Client) GetBVDetail(raw_url string) (*BVDetailResponse, error) {
	bvid, ok := ParseBVURL(raw_url)
	if !ok {
		return nil, fmt.Errorf("不是有效的B站BV视频URL: %s", raw_url)
	}

	view_url := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?bvid=%s", url.QueryEscape(bvid))
	var view_response ViewResponse
	if err := c.do_get(view_url, &view_response); err != nil {
		return nil, fmt.Errorf("获取B站视频基础信息失败: %w", err)
	}
	if view_response.Code != 0 || view_response.Data.Aid <= 0 {
		return nil, fmt.Errorf("获取B站视频基础信息失败: code=%d message=%s", view_response.Code, view_response.Message)
	}

	img_key, sub_key, err := c.fetch_bv_wbi_keys()
	if err != nil {
		return nil, err
	}
	params := make(map[string]string, len(bv_detail_browser_params)+1)
	params["aid"] = strconv.FormatInt(view_response.Data.Aid, 10)
	for key, value := range bv_detail_browser_params {
		params[key] = value
	}
	signed_params, err := bv_sign_wbi_params(params, img_key, sub_key, time.Now().Unix())
	if err != nil {
		return nil, fmt.Errorf("生成B站WBI签名失败: %w", err)
	}

	var detail_response BVDetailResponse
	detail_url := bilibili_view_detail_api + "?" + signed_params.Encode()
	if err := c.do_get(detail_url, &detail_response); err != nil {
		return nil, fmt.Errorf("获取B站视频详情失败: %w", err)
	}
	if detail_response.Code != 0 {
		return nil, fmt.Errorf("B站视频详情接口错误: code=%d message=%s", detail_response.Code, detail_response.Message)
	}
	return &detail_response, nil
}

func (c *Client) parse_bv_video(raw_url string, page_num int) ([]*VideoInfo, error) {
	detail_response, err := c.GetBVDetail(raw_url)
	if err != nil {
		return nil, err
	}
	initial_data, err := c.fetch_bv_initial_data(raw_url)
	if err != nil {
		return nil, fmt.Errorf("获取B站initialdata失败: %w", err)
	}
	view_data := detail_response.Data.View
	bvid := view_data.Bvid
	if bvid == "" {
		bvid, _ = ParseBVURL(raw_url)
	}
	p_num := page_num
	if p_num == 0 {
		p_num = parse_page_num(raw_url)
	}
	var results []*VideoInfo
	for page_index, page := range view_data.Pages {
		if p_num > 0 && page_index+1 != p_num {
			continue
		}
		play_url := fmt.Sprintf("https://api.bilibili.com/x/player/playurl?otype=json&fnver=0&fnval=4048&qn=80&bvid=%s&cid=%d", bvid, page.Cid)
		var play_response PlayURLResponse
		if err := c.do_get(play_url, &play_response); err != nil || play_response.Code != 0 || (len(play_response.Data.Durl) == 0 && len(play_response.Data.Dash.Video) == 0) {
			continue
		}
		best_url := ""
		best_audio_url := ""
		if best_video_stream := best_bv_dash_video_stream(play_response.Data.Dash.Video); best_video_stream != nil {
			best_url = strings.TrimSpace(best_video_stream.BaseURL)
			if best_audio_stream := best_bv_dash_audio_stream(play_response.Data.Dash.Audio); best_audio_stream != nil {
				best_audio_url = strings.TrimSpace(best_audio_stream.BaseURL)
			}
		}
		if best_url == "" && len(play_response.Data.Durl) > 0 {
			best_durl := play_response.Data.Durl[0]
			for _, item := range play_response.Data.Durl[1:] {
				if item.Size > best_durl.Size {
					best_durl = item
				}
			}
			best_url = strings.TrimSpace(best_durl.URL)
		}
		if best_url == "" {
			continue
		}
		results = append(results, &VideoInfo{
			URL:            best_url,
			AudioURL:       best_audio_url,
			Title:          format_title(view_data.Title, page.Part, len(view_data.Pages)),
			VideoID:        fmt.Sprintf("%s-%d", bvid, page.Cid),
			CoverURL:       view_data.Pic,
			Page:           page.Page,
			Source:         "bilibili",
			InitialData:    initial_data,
			SupportFormats: play_response.Data.SupportFormats,
			Dash:           play_response.Data.Dash,
		})
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("未获取到视频播放地址")
	}
	return results, nil
}

func best_bv_dash_video_stream(streams []DashItem) *DashItem {
	var best_stream *DashItem
	for stream_index := range streams {
		stream := &streams[stream_index]
		if strings.TrimSpace(stream.BaseURL) == "" {
			continue
		}
		if best_stream == nil || bv_dash_video_stream_is_better(stream, best_stream) {
			best_stream = stream
		}
	}
	return best_stream
}

func bv_dash_video_stream_is_better(candidate *DashItem, current *DashItem) bool {
	if candidate == nil {
		return false
	}
	if current == nil {
		return true
	}
	candidate_pixels := int64(candidate.Width) * int64(candidate.Height)
	current_pixels := int64(current.Width) * int64(current.Height)
	if candidate_pixels != current_pixels {
		return candidate_pixels > current_pixels
	}
	if candidate.Bandwidth != current.Bandwidth {
		return candidate.Bandwidth > current.Bandwidth
	}
	if candidate.Size != current.Size {
		return candidate.Size > current.Size
	}
	return candidate.ID > current.ID
}

func best_bv_dash_audio_stream(streams []DashItem) *DashItem {
	var best_stream *DashItem
	for stream_index := range streams {
		stream := &streams[stream_index]
		if strings.TrimSpace(stream.BaseURL) == "" {
			continue
		}
		if best_stream == nil || bv_dash_audio_stream_is_better(stream, best_stream) {
			best_stream = stream
		}
	}
	return best_stream
}

func bv_dash_audio_stream_is_better(candidate *DashItem, current *DashItem) bool {
	if candidate == nil {
		return false
	}
	if current == nil {
		return true
	}
	if candidate.Bandwidth != current.Bandwidth {
		return candidate.Bandwidth > current.Bandwidth
	}
	if candidate.Size != current.Size {
		return candidate.Size > current.Size
	}
	return candidate.ID > current.ID
}
