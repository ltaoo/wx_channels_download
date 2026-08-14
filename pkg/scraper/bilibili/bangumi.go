package bilibili

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const bangumi_html_limit = 32 << 20

var playurl_ssr_assignment_pattern = regexp.MustCompile(`(?:const|let|var)\s+playurlSSRData\s*=`)
var bangumi_episode_path_pattern = regexp.MustCompile(`^/bangumi/play/ep(\d+)(?:/)?$`)
var bangumi_play_path_pattern = regexp.MustCompile(`^/bangumi/play/[^/]+(?:/)?$`)

// BangumiInfo combines episode page metadata with every DASH stream embedded
// in a Bilibili bangumi play page.
type BangumiInfo struct {
	SourceURL      string              `json:"source_url"`
	Title          string              `json:"title"`
	Description    string              `json:"description"`
	CoverURL       string              `json:"cover_url"`
	EpisodeID      int64               `json:"episode_id"`
	SeasonID       int64               `json:"season_id"`
	SeasonType     int                 `json:"season_type"`
	SeasonTitle    string              `json:"season_title"`
	SeasonCoverURL string              `json:"season_cover_url"`
	AID            int64               `json:"aid"`
	CID            int64               `json:"cid"`
	BVID           string              `json:"bvid"`
	Credits        []BangumiCredit     `json:"credits,omitempty"`
	PlayURLSSRData PlayURLSSRData      `json:"playurl_ssr_data"`
	PageData       BangumiPageResponse `json:"page_data"`
	SeasonData     PGCSeasonResponse   `json:"season_data"`
}

// BangumiCredit is one person-role record parsed from the season API. A person
// may occur more than once when they have multiple jobs in the same series.
type BangumiCredit struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	Character   string `json:"character,omitempty"`
	SourceField string `json:"source_field"`
	Raw         string `json:"raw"`
	SortOrder   int    `json:"sort_order"`
}

// PlayURLSSRData is the JSON value assigned to playurlSSRData in the page.
type PlayURLSSRData struct {
	Status  int                   `json:"status"`
	Message string                `json:"message,omitempty"`
	Data    PlayURLSSRDataPayload `json:"data"`
}

// PlayURLSSRDataPayload holds the playback result returned during SSR.
type PlayURLSSRDataPayload struct {
	Code    int              `json:"code,omitempty"`
	Message string           `json:"message,omitempty"`
	Result  PlayURLSSRResult `json:"result"`
}

// PlayURLSSRResult holds the playback streams and identifiers needed by the
// adapter.
type PlayURLSSRResult struct {
	PlayVideoType        string                      `json:"play_video_type"`
	VideoInfo            BangumiVideoInfo            `json:"video_info"`
	Arc                  BangumiArc                  `json:"arc"`
	PlayViewBusinessInfo BangumiPlayViewBusinessInfo `json:"play_view_business_info"`
}

// BangumiArc identifies the underlying Bilibili archive.
type BangumiArc struct {
	BizType int    `json:"biz_type"`
	AID     int64  `json:"aid"`
	CID     int64  `json:"cid"`
	BVID    string `json:"bvid"`
}

// BangumiPlayViewBusinessInfo contains episode and season identifiers.
type BangumiPlayViewBusinessInfo struct {
	EpisodeInfo BangumiEpisodeInfo `json:"episode_info"`
	SeasonInfo  BangumiSeasonInfo  `json:"season_info"`
}

// BangumiEpisodeInfo identifies the episode represented by the page.
type BangumiEpisodeInfo struct {
	AID       int64 `json:"aid"`
	CID       int64 `json:"cid"`
	EpisodeID int64 `json:"ep_id"`
}

// BangumiSeasonInfo identifies the episode's season.
type BangumiSeasonInfo struct {
	SeasonID   int64 `json:"season_id"`
	SeasonType int   `json:"season_type"`
}

// BangumiVideoInfo contains the DASH manifest embedded in playurlSSRData.
type BangumiVideoInfo struct {
	Quality    int         `json:"quality"`
	Format     string      `json:"format"`
	TimeLength int64       `json:"timelength"`
	Dash       BangumiDash `json:"dash"`
}

// BangumiDash contains all video and audio representations.
type BangumiDash struct {
	Duration      float64             `json:"duration"`
	MinBufferTime float64             `json:"min_buffer_time"`
	Video         []BangumiDashStream `json:"video"`
	Audio         []BangumiDashStream `json:"audio"`
}

// BangumiDashStream is one video or audio DASH representation.
type BangumiDashStream struct {
	ID          int                `json:"id"`
	BaseURL     string             `json:"base_url"`
	BackupURL   []string           `json:"backup_url"`
	MIMEType    string             `json:"mime_type"`
	Codecs      string             `json:"codecs"`
	Bandwidth   int64              `json:"bandwidth"`
	Width       int                `json:"width,omitempty"`
	Height      int                `json:"height,omitempty"`
	FrameRate   string             `json:"frame_rate,omitempty"`
	SegmentBase BangumiSegmentBase `json:"segment_base"`
	Size        int64              `json:"size,omitempty"`
	CodecID     int                `json:"codecid,omitempty"`
	MD5         string             `json:"md5,omitempty"`
}

// BangumiSegmentBase describes byte ranges required to initialize and index a
// DASH representation.
type BangumiSegmentBase struct {
	Initialization string `json:"initialization"`
	IndexRange     string `json:"index_range"`
}

// IsBangumiPlayURL reports whether raw_url points to a Bilibili
// /bangumi/play/<id> page.
func IsBangumiPlayURL(raw_url string) bool {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed_url.Hostname())
	if host != "bilibili.com" && !strings.HasSuffix(host, ".bilibili.com") {
		return false
	}
	return bangumi_play_path_pattern.MatchString(parsed_url.Path)
}

// IsBangumiEpisodeURL reports whether raw_url is a Bilibili bangumi episode
// play URL.
func IsBangumiEpisodeURL(raw_url string) bool {
	if !IsBangumiPlayURL(raw_url) {
		return false
	}
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	return err == nil && bangumi_episode_path_pattern.MatchString(parsed_url.Path)
}

// GetBangumiInfo downloads a Bilibili bangumi episode page and parses its
// server-rendered playurlSSRData value.
func (c *Client) GetBangumiInfo(raw_url string) (*BangumiInfo, error) {
	if !IsBangumiPlayURL(raw_url) {
		return nil, fmt.Errorf("不是有效的B站番剧播放URL: %s", raw_url)
	}

	html_data, final_url, err := c.fetch_bangumi_html(raw_url)
	if err != nil {
		return nil, err
	}
	info, err := ParseBangumiHTML(final_url, html_data)
	if err != nil {
		return nil, fmt.Errorf("解析B站番剧页面失败: %w", err)
	}
	page_data, err := c.fetch_bangumi_page(info.EpisodeID)
	if err != nil {
		return nil, err
	}
	info.PageData = *page_data
	season_data, err := c.fetch_bangumi_season(info.EpisodeID)
	if err != nil {
		return nil, err
	}
	info.SeasonData = *season_data
	apply_bangumi_season_metadata(info)
	apply_bangumi_page_metadata(info)
	return info, nil
}

// ParseBangumiHTML parses episode metadata and playurlSSRData from an already
// downloaded Bilibili HTML document.
func ParseBangumiHTML(source_url string, html_data []byte) (*BangumiInfo, error) {
	assignment_location := playurl_ssr_assignment_pattern.FindIndex(html_data)
	if assignment_location == nil {
		return nil, fmt.Errorf("页面中未找到 playurlSSRData")
	}

	decoder := json.NewDecoder(bytes.NewReader(html_data[assignment_location[1]:]))
	var playurl_ssr_data PlayURLSSRData
	if err := decoder.Decode(&playurl_ssr_data); err != nil {
		return nil, fmt.Errorf("解析 playurlSSRData JSON 失败: %w", err)
	}
	if playurl_ssr_data.Status != http.StatusOK {
		return nil, fmt.Errorf("playurlSSRData 状态异常: status=%d message=%s", playurl_ssr_data.Status, playurl_ssr_data.Message)
	}
	if playurl_ssr_data.Data.Code != 0 {
		return nil, fmt.Errorf("playurlSSRData 返回错误: code=%d message=%s", playurl_ssr_data.Data.Code, playurl_ssr_data.Data.Message)
	}

	dash := playurl_ssr_data.Data.Result.VideoInfo.Dash
	if len(dash.Video) == 0 && len(dash.Audio) == 0 {
		return nil, fmt.Errorf("playurlSSRData 中没有 DASH 音视频流")
	}

	info := &BangumiInfo{
		SourceURL:      strings.TrimSpace(source_url),
		PlayURLSSRData: playurl_ssr_data,
	}
	parse_bangumi_metadata(info, html_data)
	fill_bangumi_identifiers(info)
	return info, nil
}

func (c *Client) fetch_bangumi_html(raw_url string) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, raw_url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("创建B站番剧页面请求失败: %w", err)
	}
	for header_name, header_value := range c.headers {
		req.Header.Set(header_name, header_value)
	}
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}

	resp, err := c.http_client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("抓取B站番剧页面失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("抓取B站番剧页面失败: HTTP %d", resp.StatusCode)
	}

	html_data, err := io.ReadAll(io.LimitReader(resp.Body, bangumi_html_limit+1))
	if err != nil {
		return nil, "", fmt.Errorf("读取B站番剧页面失败: %w", err)
	}
	if len(html_data) > bangumi_html_limit {
		return nil, "", fmt.Errorf("B站番剧页面超过大小限制: %d bytes", bangumi_html_limit)
	}
	return html_data, resp.Request.URL.String(), nil
}

func parse_bangumi_metadata(info *BangumiInfo, html_data []byte) {
	if info == nil {
		return
	}
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(html_data))
	if err != nil {
		return
	}

	page_title := clean_bangumi_title(document.Find("title").First().Text())
	og_title := clean_bangumi_title(document.Find(`meta[property="og:title"]`).First().AttrOr("content", ""))
	og_cover_url := normalize_bangumi_asset_url(document.Find(`meta[property="og:image"]`).First().AttrOr("content", ""))
	info.Title = page_title
	info.Description = strings.TrimSpace(document.Find(`meta[name="description"]`).First().AttrOr("content", ""))
	info.CoverURL = og_cover_url
	info.SeasonTitle = og_title
	info.SeasonCoverURL = og_cover_url
	if info.Title == "" {
		info.Title = og_title
	}
}

func fill_bangumi_identifiers(info *BangumiInfo) {
	if info == nil {
		return
	}
	result := info.PlayURLSSRData.Data.Result
	info.EpisodeID = result.PlayViewBusinessInfo.EpisodeInfo.EpisodeID
	info.SeasonID = result.PlayViewBusinessInfo.SeasonInfo.SeasonID
	info.SeasonType = result.PlayViewBusinessInfo.SeasonInfo.SeasonType
	info.AID = result.Arc.AID
	info.CID = result.Arc.CID
	info.BVID = strings.TrimSpace(result.Arc.BVID)
	if info.AID == 0 {
		info.AID = result.PlayViewBusinessInfo.EpisodeInfo.AID
	}
	if info.CID == 0 {
		info.CID = result.PlayViewBusinessInfo.EpisodeInfo.CID
	}
	if info.EpisodeID == 0 {
		info.EpisodeID = bangumi_episode_id(info.SourceURL)
	}
}

func apply_bangumi_page_metadata(info *BangumiInfo) {
	if info == nil {
		return
	}
	episode := info.PageData.TargetEpisode()
	if episode == nil {
		return
	}
	info.EpisodeID = episode.EpisodeID
	info.AID = episode.AID
	info.CID = episode.CID
	info.BVID = strings.TrimSpace(episode.BVID)
	info.CoverURL = normalize_bangumi_asset_url(episode.Cover)
	for _, title := range []string{episode.ShareCopy, episode.ShowTitle, episode.LongTitle, episode.Title} {
		if title = strings.TrimSpace(title); title != "" {
			info.Title = title
			break
		}
	}
}

func apply_bangumi_season_metadata(info *BangumiInfo) {
	if info == nil {
		return
	}
	season := &info.SeasonData.Result
	if season.SeasonID > 0 {
		info.SeasonID = season.SeasonID
	}
	if season.Type > 0 {
		info.SeasonType = season.Type
	}
	for _, title := range []string{season.Title, season.SeasonTitle} {
		if title = strings.TrimSpace(title); title != "" {
			info.SeasonTitle = title
			break
		}
	}
	if cover_url := normalize_bangumi_asset_url(season.Cover); cover_url != "" {
		info.SeasonCoverURL = cover_url
	}
	if description := strings.TrimSpace(season.Evaluate); description != "" {
		info.Description = description
	}
	info.Credits = parse_bangumi_credits(season.Staff, season.Actors)
}

func parse_bangumi_credits(staff string, actors string) []BangumiCredit {
	credits := make([]BangumiCredit, 0)
	seen_credits := make(map[string]struct{})
	append_credit := func(name string, role string, character string, source_field string, raw string) {
		name = strings.TrimSpace(name)
		role = strings.TrimSpace(role)
		if name == "" || role == "" {
			return
		}
		credit_key := name + "\x00" + role
		if _, exists := seen_credits[credit_key]; exists {
			return
		}
		seen_credits[credit_key] = struct{}{}
		credits = append(credits, BangumiCredit{
			Name:        name,
			Role:        role,
			Character:   strings.TrimSpace(character),
			SourceField: source_field,
			Raw:         strings.TrimSpace(raw),
			SortOrder:   len(credits),
		})
	}

	for _, line := range strings.Split(strings.ReplaceAll(staff, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		role_text, names_text, found := cut_bangumi_credit_line(line)
		if !found {
			continue
		}
		roles := split_bangumi_credit_values(role_text)
		names := split_bangumi_credit_values(names_text)
		for _, name := range names {
			for _, role := range roles {
				append_credit(name, role, "", "staff", line)
			}
		}
	}

	for _, line := range strings.Split(strings.ReplaceAll(actors, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		character := ""
		names_text := line
		if left, right, found := cut_bangumi_credit_line(line); found {
			character = left
			names_text = right
		}
		for _, name := range split_bangumi_credit_values(names_text) {
			append_credit(name, "演员", character, "actors", line)
		}
	}
	return credits
}

func cut_bangumi_credit_line(line string) (string, string, bool) {
	if left, right, found := strings.Cut(line, "："); found {
		return left, right, true
	}
	return strings.Cut(line, ":")
}

func split_bangumi_credit_values(value string) []string {
	parts := strings.FieldsFunc(value, func(char rune) bool {
		switch char {
		case '、', ',', '，', '/', '／', ';', '；':
			return true
		default:
			return false
		}
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func bangumi_episode_id(raw_url string) int64 {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return 0
	}
	match := bangumi_episode_path_pattern.FindStringSubmatch(parsed_url.Path)
	if len(match) < 2 {
		return 0
	}
	episode_id, _ := strconv.ParseInt(match[1], 10, 64)
	return episode_id
}

func clean_bangumi_title(title string) string {
	title = strings.TrimSpace(title)
	for _, suffix := range []string{"-哔哩哔哩_bilibili", "-bilibili-哔哩哔哩"} {
		title = strings.TrimSuffix(title, suffix)
	}
	return strings.TrimSpace(title)
}

func normalize_bangumi_asset_url(raw_url string) string {
	raw_url = strings.TrimSpace(raw_url)
	if strings.HasPrefix(raw_url, "//") {
		return "https:" + raw_url
	}
	if strings.HasPrefix(raw_url, "http://") {
		return "https://" + strings.TrimPrefix(raw_url, "http://")
	}
	return raw_url
}
