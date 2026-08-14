package bilibili

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

const bilibili_log_preview_limit = 2048

// Client is the Bilibili video scraper client.
type Client struct {
	cookie      string
	http_client *http.Client
	headers     map[string]string
	logger      zerolog.Logger
	request_seq atomic.Uint64
}

// NewClient creates a new Bilibili client.
func NewClient(cookie string) *Client {
	return NewClientWithLogger(cookie, nil)
}

// NewClientWithLogger creates a Bilibili client with structured API diagnostics.
func NewClientWithLogger(cookie string, parent_logger *zerolog.Logger) *Client {
	logger := zerolog.Nop()
	if parent_logger != nil {
		logger = parent_logger.With().Str("component", "bilibili_scraper").Logger()
	}
	return &Client{
		cookie: cookie,
		http_client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
		headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36",
			"Referer":    "https://www.bilibili.com/",
		},
		logger: logger,
	}
}

// Fetch retrieves the structured result for a supported Bilibili URL.
// Bangumi play pages return a BangumiInfo combining PlayURLSSRData with the
// episode page API response; other URL types retain the existing VideoInfo
// list result.
func (c *Client) Fetch(raw_url string) (any, error) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return nil, fmt.Errorf("B站URL不能为空")
	}
	if IsBangumiPlayURL(raw_url) {
		bangumi_info, err := c.GetBangumiInfo(raw_url)
		if err != nil {
			return nil, err
		}
		return bangumi_info, nil
	}
	return c.GetVideoInfo(raw_url, 0)
}

// GetVideoInfo retrieves Bilibili video information.
// Supported URL types: regular videos (BV/AV), bangumi episodes (ep), bangumi seasons (ss), courses (cheese).
// page_num specifies the part/page number; 0 means get all.
func (c *Client) GetVideoInfo(raw_url string, page_num int) ([]*VideoInfo, error) {
	// Follow redirects first (handle b23.tv short links)
	final_url, err := c.resolve_url(raw_url)
	if err != nil {
		final_url = raw_url
	}

	// Regular video BV/AV
	if c.is_common_video(final_url) {
		return c.parse_common_video(final_url, page_num)
	}

	// Bangumi episode ep
	if c.is_bangumi_episode(final_url) {
		return c.parse_bangumi_episode(final_url)
	}

	// Bangumi season ss
	if c.is_bangumi_season(final_url) {
		return c.parse_bangumi_season(final_url)
	}

	// Course cheese
	if c.is_cheese_episode(final_url) {
		return c.parse_cheese_episode(final_url)
	}

	return nil, fmt.Errorf("不支持的B站URL: %s", raw_url)
}

// is_bilibili_url checks whether the URL is a Bilibili domain.
func (c *Client) is_bilibili_url(u string) bool {
	return strings.Contains(u, "bilibili.com") ||
		strings.Contains(u, "b23.tv") ||
		strings.Contains(u, "bili2233.cn")
}

// resolve_url follows short link redirects.
func (c *Client) resolve_url(u string) (string, error) {
	resp, err := c.http_client.Get(u)
	if err != nil {
		return u, err
	}
	defer resp.Body.Close()
	return resp.Request.URL.String(), nil
}

func (c *Client) is_common_video(u string) bool {
	return regexp.MustCompile(`bilibili\.com/(?:video/|festival/[^/?#]+\?(?:[^#]*&)?bvid=)`).MatchString(u)
}

func (c *Client) is_bangumi_episode(u string) bool {
	return regexp.MustCompile(`bilibili\.com/bangumi/play/ep\d+`).MatchString(u)
}

func (c *Client) is_bangumi_season(u string) bool {
	return regexp.MustCompile(`bilibili\.com/bangumi/play/ss\d+`).MatchString(u)
}

func (c *Client) is_cheese_episode(u string) bool {
	return regexp.MustCompile(`bilibili\.com/cheese/play/ep\d+`).MatchString(u)
}

// do_get sends a GET request and parses the JSON response.
func (c *Client) do_get(api_url string, result interface{}) error {
	request_id := c.request_seq.Add(1)
	req, err := http.NewRequest("GET", api_url, nil)
	if err != nil {
		c.logger.Error().
			Err(err).
			Uint64("api_request_id", request_id).
			Str("request_url", api_url).
			Msg("bilibili API: request construction failed")
		return fmt.Errorf("construct bilibili API request: %w", err)
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}

	request_started_at := time.Now()
	c.logger.Info().
		Uint64("api_request_id", request_id).
		Str("method", req.Method).
		Str("api_host", req.URL.Hostname()).
		Str("api_path", req.URL.EscapedPath()).
		Str("request_url", req.URL.String()).
		Msg("bilibili API: request started")

	resp, err := c.http_client.Do(req)
	if err != nil {
		c.logger.Error().
			Err(err).
			Uint64("api_request_id", request_id).
			Str("method", req.Method).
			Str("api_host", req.URL.Hostname()).
			Str("api_path", req.URL.EscapedPath()).
			Str("request_url", req.URL.String()).
			Dur("request_elapsed", time.Since(request_started_at)).
			Msg("bilibili API: request failed")
		return fmt.Errorf("request bilibili API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error().
			Err(err).
			Uint64("api_request_id", request_id).
			Str("method", req.Method).
			Str("api_host", req.URL.Hostname()).
			Str("api_path", req.URL.EscapedPath()).
			Str("request_url", req.URL.String()).
			Int("http_status", resp.StatusCode).
			Str("content_type", resp.Header.Get("Content-Type")).
			Str("content_encoding", resp.Header.Get("Content-Encoding")).
			Int64("content_length", resp.ContentLength).
			Dur("request_elapsed", time.Since(request_started_at)).
			Msg("bilibili API: response body read failed")
		return fmt.Errorf("read bilibili API response: %w", err)
	}

	c.log_api_response(req, resp, body, request_id, time.Since(request_started_at))
	if err := json.Unmarshal(body, result); err != nil {
		c.logger.Error().
			Err(err).
			Uint64("api_request_id", request_id).
			Str("api_host", req.URL.Hostname()).
			Str("api_path", req.URL.EscapedPath()).
			Int("http_status", resp.StatusCode).
			Int("body_bytes", len(body)).
			Str("response_preview", bilibili_log_preview(body)).
			Msg("bilibili API: JSON decode failed")
		return fmt.Errorf("decode bilibili API response: status=%d body_bytes=%d: %w", resp.StatusCode, len(body), err)
	}
	return nil
}

type bilibili_api_envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Msg     string `json:"msg"`
}

func (c *Client) log_api_response(req *http.Request, resp *http.Response, body []byte, request_id uint64, request_elapsed time.Duration) {
	var envelope bilibili_api_envelope
	envelope_err := json.Unmarshal(body, &envelope)
	api_message := strings.TrimSpace(envelope.Message)
	if api_message == "" {
		api_message = strings.TrimSpace(envelope.Msg)
	}

	log_event := c.logger.Info()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || (envelope_err == nil && envelope.Code != 0) {
		log_event = c.logger.Warn()
	}
	log_event.
		Uint64("api_request_id", request_id).
		Str("method", req.Method).
		Str("api_host", req.URL.Hostname()).
		Str("api_path", req.URL.EscapedPath()).
		Str("request_url", req.URL.String()).
		Str("response_url", resp.Request.URL.String()).
		Int("http_status", resp.StatusCode).
		Str("http_status_text", http.StatusText(resp.StatusCode)).
		Str("content_type", resp.Header.Get("Content-Type")).
		Str("content_encoding", resp.Header.Get("Content-Encoding")).
		Int64("content_length", resp.ContentLength).
		Int("body_bytes", len(body)).
		Str("bili_trace_id", bilibili_trace_id(resp.Header)).
		Dur("request_elapsed", request_elapsed).
		Str("response_preview", bilibili_log_preview(body))
	if envelope_err == nil {
		log_event.Int("api_code", envelope.Code).Str("api_message", bilibili_log_preview([]byte(api_message)))
	}
	log_event.Msg("bilibili API: response received")
}

func bilibili_trace_id(headers http.Header) string {
	trace_id := strings.TrimSpace(headers.Get("X-Bili-Trace-Id"))
	if trace_id == "" {
		trace_id = strings.TrimSpace(headers.Get("Bili-Trace-Id"))
	}
	return trace_id
}

func bilibili_log_preview(body []byte) string {
	preview := strings.TrimSpace(strings.ToValidUTF8(string(body), "�"))
	preview_runes := []rune(preview)
	if len(preview_runes) <= bilibili_log_preview_limit {
		return preview
	}
	return string(preview_runes[:bilibili_log_preview_limit]) + "…"
}

// parse_common_video parses a regular video URL.
func (c *Client) parse_common_video(u string, page_num int) ([]*VideoInfo, error) {
	// Extract video_id (BV number)
	re := regexp.MustCompile(`(?:video/|bvid=)([aAbB][vV])([^/?#&]+)`)
	match := re.FindStringSubmatch(u)
	if len(match) < 3 {
		return nil, fmt.Errorf("无法从URL提取BV号: %s", u)
	}
	prefix := match[1]
	video_id := match[2]

	bvid := prefix + video_id

	// AV number needs to be converted to BV first
	if strings.EqualFold(prefix, "AV") {
		aid_str := video_id
		redirect_bvid, err := c.av_to_bv(aid_str)
		if err != nil {
			return nil, err
		}
		bvid = redirect_bvid
	}

	// https://api.bilibili.com/x/web-interface/wbi/view/detail?aid=116968988417376
	// Retrieve video info
	view_url := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?bvid=%s", bvid)
	var view_resp ViewResponse
	if err := c.do_get(view_url, &view_resp); err != nil {
		return nil, fmt.Errorf("获取视频信息失败: %w", err)
	}
	if view_resp.Code != 0 {
		return nil, fmt.Errorf("B站API返回错误: code=%d, msg=%s", view_resp.Code, view_resp.Message)
	}

	// Extract page number
	p_num := page_num
	if p_num == 0 {
		if parsed := parse_page_num(u); parsed > 0 {
			p_num = parsed
		}
	}

	pages := view_resp.Data.Pages
	var results []*VideoInfo

	for idx, page := range pages {
		if p_num > 0 && idx+1 != p_num {
			continue
		}

		// Get playback URL
		play_url := fmt.Sprintf("https://api.bilibili.com/x/player/playurl?otype=json&fnver=0&fnval=0&qn=80&bvid=%s&cid=%d&platform=html5", bvid, page.Cid)
		var play_resp PlayURLResponse
		if err := c.do_get(play_url, &play_resp); err != nil {
			continue
		}
		if play_resp.Code != 0 || len(play_resp.Data.Durl) == 0 {
			continue
		}

		// Pick the largest video stream
		best_durl := play_resp.Data.Durl[0]
		for _, d := range play_resp.Data.Durl {
			if d.Size > best_durl.Size {
				best_durl = d
			}
		}

		title := format_title(view_resp.Data.Title, page.Part, len(pages))
		cover_url := page.FirstFrame()
		if cover_url == "" {
			cover_url = view_resp.Data.Pic
		}

		results = append(results, &VideoInfo{
			URL:      best_durl.URL,
			Title:    title,
			VideoID:  fmt.Sprintf("%s-%d", bvid, page.Cid),
			CoverURL: cover_url,
			Page:     page.Page,
			Source:   "bilibili",
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("未获取到视频播放地址")
	}
	return results, nil
}

// av_to_bv converts an AV number to a BV number.
func (c *Client) av_to_bv(aid string) (string, error) {
	aid_num, err := strconv.ParseInt(aid, 10, 64)
	if err != nil {
		return "", fmt.Errorf("无效的AV号: %s", aid)
	}
	view_url := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?aid=%d", aid_num)
	var view_resp ViewResponse
	if err := c.do_get(view_url, &view_resp); err != nil {
		return "", fmt.Errorf("AV转BV失败: %w", err)
	}
	if view_resp.Code != 0 {
		return "", fmt.Errorf("AV转BV失败: code=%d", view_resp.Code)
	}
	return view_resp.Data.Bvid, nil
}

// parse_bangumi_episode parses a bangumi episode URL.
func (c *Client) parse_bangumi_episode(u string) ([]*VideoInfo, error) {
	re := regexp.MustCompile(`bangumi/play/ep(\d+)`)
	match := re.FindStringSubmatch(u)
	if len(match) < 2 {
		return nil, fmt.Errorf("无法从URL提取ep号: %s", u)
	}
	ep_id := match[1]

	// Get bangumi info
	season_url := fmt.Sprintf("https://api.bilibili.com/pgc/view/web/season?ep_id=%s", ep_id)
	var season_resp PGCSeasonResponse
	if err := c.do_get(season_url, &season_resp); err != nil {
		return nil, fmt.Errorf("获取番剧信息失败: %w", err)
	}
	if season_resp.Code != 0 {
		return nil, fmt.Errorf("番剧API错误: code=%d", season_resp.Code)
	}

	// Collect all episodes
	all_episodes := season_resp.Result.Episodes
	for _, section := range season_resp.Result.Section {
		all_episodes = append(all_episodes, section.Episodes...)
	}

	// Find the target episode
	ep_id_num, _ := strconv.ParseInt(ep_id, 10, 64)
	var target_episode *PGCEpisode
	for i := range all_episodes {
		if all_episodes[i].EpID == ep_id_num {
			target_episode = &all_episodes[i]
			break
		}
	}
	if target_episode == nil {
		return nil, fmt.Errorf("未找到剧集 ep=%s", ep_id)
	}

	// Get playback URL (DASH format)
	play_url := fmt.Sprintf("https://api.bilibili.com/pgc/player/web/v2/playurl?fnval=12240&ep_id=%s", ep_id)
	var play_resp PGCPlayURLResponse
	if err := c.do_get(play_url, &play_resp); err != nil {
		return nil, fmt.Errorf("获取番剧播放地址失败: %w", err)
	}
	if play_resp.Code != 0 {
		return nil, fmt.Errorf("番剧播放API错误: code=%d", play_resp.Code)
	}

	return c.build_dash_results(&play_resp.Result.VideoInfo.Dash, target_episode.title(), target_episode.Cover, ep_id, 1)
}

// parse_bangumi_season parses a bangumi season URL.
func (c *Client) parse_bangumi_season(u string) ([]*VideoInfo, error) {
	re := regexp.MustCompile(`bangumi/play/ss(\d+)`)
	match := re.FindStringSubmatch(u)
	if len(match) < 2 {
		return nil, fmt.Errorf("无法从URL提取ss号: %s", u)
	}
	ss_id := match[1]

	// Get all episodes for the season
	section_url := fmt.Sprintf("https://api.bilibili.com/pgc/web/season/section?season_id=%s", ss_id)
	var section_resp PGCSeasonSectionResponse
	if err := c.do_get(section_url, &section_resp); err != nil {
		return nil, fmt.Errorf("获取番剧季信息失败: %w", err)
	}
	if section_resp.Code != 0 {
		return nil, fmt.Errorf("番剧季API错误: code=%d", section_resp.Code)
	}

	all_episodes := section_resp.Result.MainSection.Episodes
	for _, section := range section_resp.Result.Section {
		all_episodes = append(all_episodes, section.Episodes...)
	}

	var results []*VideoInfo
	for idx, ep := range all_episodes {
		play_url := fmt.Sprintf("https://api.bilibili.com/pgc/player/web/v2/playurl?fnval=12240&ep_id=%d", ep.EpID)
		var play_resp PGCPlayURLResponse
		if err := c.do_get(play_url, &play_resp); err != nil {
			continue
		}
		if play_resp.Code != 0 || len(play_resp.Result.VideoInfo.Dash.Video) == 0 {
			continue
		}

		video_infos, err := c.build_dash_results(&play_resp.Result.VideoInfo.Dash, ep.title(), ep.Cover, strconv.FormatInt(ep.EpID, 10), idx+1)
		if err == nil && len(video_infos) > 0 {
			results = append(results, video_infos[0])
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("未获取到番剧季视频地址")
	}
	return results, nil
}

// parse_cheese_episode parses a course episode URL.
func (c *Client) parse_cheese_episode(u string) ([]*VideoInfo, error) {
	re := regexp.MustCompile(`cheese/play/ep(\d+)`)
	match := re.FindStringSubmatch(u)
	if len(match) < 2 {
		return nil, fmt.Errorf("无法从URL提取课程ep号: %s", u)
	}
	ep_id := match[1]

	season_url := fmt.Sprintf("https://api.bilibili.com/pugv/view/web/season?ep_id=%s", ep_id)
	var season_resp PUGVSeasonResponse
	if err := c.do_get(season_url, &season_resp); err != nil {
		return nil, fmt.Errorf("获取课程信息失败: %w", err)
	}
	if season_resp.Code != 0 {
		return nil, fmt.Errorf("课程API错误: code=%d", season_resp.Code)
	}

	ep_id_num, _ := strconv.ParseInt(ep_id, 10, 64)
	for _, ep := range season_resp.Data.Episodes {
		if ep.ID != ep_id_num {
			continue
		}

		play_url := fmt.Sprintf("https://api.bilibili.com/pugv/player/web/playurl?avid=%d&cid=%d&ep_id=%d&fnval=16&fourk=1",
			ep.Aid, ep.Cid, ep_id_num)
		var play_resp PUGVPlayURLResponse
		if err := c.do_get(play_url, &play_resp); err != nil {
			return nil, fmt.Errorf("获取课程播放地址失败: %w", err)
		}
		if play_resp.Code != 0 {
			return nil, fmt.Errorf("课程播放API错误: code=%d", play_resp.Code)
		}

		return c.build_dash_results(&play_resp.Data.Dash, ep.Title, ep.Cover, ep_id, 1)
	}

	return nil, fmt.Errorf("未找到课程剧集 ep=%s", ep_id)
}

// build_dash_results builds VideoInfo from DASH data, selecting the best video + audio streams.
func (c *Client) build_dash_results(dash *DashInfo, title, cover_url, id string, page int) ([]*VideoInfo, error) {
	if dash == nil || len(dash.Video) == 0 {
		return nil, fmt.Errorf("DASH数据为空")
	}

	// Sort video streams by resolution * file size descending
	type format_item struct {
		url    string
		size   int64
		width  int
		height int
	}
	var video_formats []format_item
	for _, v := range dash.Video {
		if v.BaseURL != "" {
			video_formats = append(video_formats, format_item{
				url:    v.BaseURL,
				size:   v.Size,
				width:  v.Width,
				height: v.Height,
			})
		}
	}
	if len(video_formats) == 0 {
		return nil, fmt.Errorf("无可用的视频流")
	}
	sort.Slice(video_formats, func(i, j int) bool {
		res_i := video_formats[i].width * video_formats[i].height
		res_j := video_formats[j].width * video_formats[j].height
		if res_i != res_j {
			return res_i > res_j
		}
		return video_formats[i].size > video_formats[j].size
	})

	// Sort audio streams by size descending
	var audio_formats []format_item
	for _, a := range dash.Audio {
		if a.BaseURL != "" {
			audio_formats = append(audio_formats, format_item{url: a.BaseURL, size: a.Size})
		}
	}
	sort.Slice(audio_formats, func(i, j int) bool {
		return audio_formats[i].size > audio_formats[j].size
	})

	info := &VideoInfo{
		URL:      video_formats[0].url,
		Title:    sanitize_title(title),
		VideoID:  id,
		CoverURL: cover_url,
		Page:     page,
		Source:   "bilibili",
	}

	if len(audio_formats) > 0 {
		info.AudioURL = audio_formats[0].url
	}

	return []*VideoInfo{info}, nil
}

// format_title formats the video title.
func format_title(main_title, part_title string, total_pages int) string {
	if total_pages > 1 {
		return sanitize_title(part_title)
	}
	return sanitize_title(main_title)
}

// sanitize_title sanitizes the title string.
func sanitize_title(title string) string {
	re := regexp.MustCompile(`[\\/:*?"<>|#\n\r]`)
	title = re.ReplaceAllString(title, "_")
	title = strings.Trim(title, " .")
	if len(title) > 80 {
		title = title[:80]
	}
	return title
}

// parse_page_num extracts the p parameter from a URL.
func parse_page_num(u string) int {
	parsed, err := url.Parse(u)
	if err != nil {
		return 0
	}
	p := parsed.Query().Get("p")
	if p == "" {
		return 0
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return 0
	}
	return n
}

// title returns the best title for a PGC episode
func (e *PGCEpisode) title() string {
	if e.ShareCopy != "" {
		return e.ShareCopy
	}
	if e.ShowTitle != "" {
		return e.ShowTitle
	}
	if e.LongTitle != "" {
		return e.LongTitle
	}
	return e.Title
}

// FirstFrame extracts the first_frame URL from a page (used as cover fallback)
func (p *Page) FirstFrame() string {
	return ""
}
