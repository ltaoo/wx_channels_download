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
)

// Client is the Bilibili video scraper client.
type Client struct {
	cookie     string
	httpClient *http.Client
	headers    map[string]string
}

// NewClient creates a new Bilibili client.
func NewClient(cookie string) *Client {
	return &Client{
		cookie: cookie,
		httpClient: &http.Client{
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
	}
}

// GetVideoInfo retrieves Bilibili video information.
// Supported URL types: regular videos (BV/AV), bangumi episodes (ep), bangumi seasons (ss), courses (cheese).
// pageNum specifies the part/page number; 0 means get all.
func (c *Client) GetVideoInfo(rawURL string, pageNum int) ([]*VideoInfo, error) {
	// Follow redirects first (handle b23.tv short links)
	finalURL, err := c.resolveURL(rawURL)
	if err != nil {
		finalURL = rawURL
	}

	// Regular video BV/AV
	if c.isCommonVideo(finalURL) {
		return c.parseCommonVideo(finalURL, pageNum)
	}

	// Bangumi episode ep
	if c.isBangumiEpisode(finalURL) {
		return c.parseBangumiEpisode(finalURL)
	}

	// Bangumi season ss
	if c.isBangumiSeason(finalURL) {
		return c.parseBangumiSeason(finalURL)
	}

	// Course cheese
	if c.isCheeseEpisode(finalURL) {
		return c.parseCheeseEpisode(finalURL)
	}

	return nil, fmt.Errorf("不支持的B站URL: %s", rawURL)
}

// isBilibiliURL checks whether the URL is a Bilibili domain.
func (c *Client) isBilibiliURL(u string) bool {
	return strings.Contains(u, "bilibili.com") ||
		strings.Contains(u, "b23.tv") ||
		strings.Contains(u, "bili2233.cn")
}

// resolveURL follows short link redirects.
func (c *Client) resolveURL(u string) (string, error) {
	resp, err := c.httpClient.Get(u)
	if err != nil {
		return u, err
	}
	defer resp.Body.Close()
	return resp.Request.URL.String(), nil
}

func (c *Client) isCommonVideo(u string) bool {
	return regexp.MustCompile(`bilibili\.com/(?:video/|festival/[^/?#]+\?(?:[^#]*&)?bvid=)`).MatchString(u)
}

func (c *Client) isBangumiEpisode(u string) bool {
	return regexp.MustCompile(`bilibili\.com/bangumi/play/ep\d+`).MatchString(u)
}

func (c *Client) isBangumiSeason(u string) bool {
	return regexp.MustCompile(`bilibili\.com/bangumi/play/ss\d+`).MatchString(u)
}

func (c *Client) isCheeseEpisode(u string) bool {
	return regexp.MustCompile(`bilibili\.com/cheese/play/ep\d+`).MatchString(u)
}

// doGet sends a GET request and parses the JSON response.
func (c *Client) doGet(apiURL string, result interface{}) error {
	req, _ := http.NewRequest("GET", apiURL, nil)
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, result)
}

// parseCommonVideo parses a regular video URL.
func (c *Client) parseCommonVideo(u string, pageNum int) ([]*VideoInfo, error) {
	// Extract video_id (BV number)
	re := regexp.MustCompile(`(?:video/|bvid=)([aAbB][vV])([^/?#&]+)`)
	match := re.FindStringSubmatch(u)
	if len(match) < 3 {
		return nil, fmt.Errorf("无法从URL提取BV号: %s", u)
	}
	prefix := match[1]
	videoID := match[2]

	bvid := prefix + videoID

	// AV number needs to be converted to BV first
	if strings.EqualFold(prefix, "AV") {
		aidStr := videoID
		redirectBVID, err := c.avToBV(aidStr)
		if err != nil {
			return nil, err
		}
		bvid = redirectBVID
	}

	// Retrieve video info
	viewURL := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?bvid=%s", bvid)
	var viewResp ViewResponse
	if err := c.doGet(viewURL, &viewResp); err != nil {
		return nil, fmt.Errorf("获取视频信息失败: %w", err)
	}
	if viewResp.Code != 0 {
		return nil, fmt.Errorf("B站API返回错误: code=%d, msg=%s", viewResp.Code, viewResp.Message)
	}

	// Extract page number
	pNum := pageNum
	if pNum == 0 {
		if parsed := parsePageNum(u); parsed > 0 {
			pNum = parsed
		}
	}

	pages := viewResp.Data.Pages
	var results []*VideoInfo

	for idx, page := range pages {
		if pNum > 0 && idx+1 != pNum {
			continue
		}

		// Get playback URL
		playURL := fmt.Sprintf("https://api.bilibili.com/x/player/playurl?otype=json&fnver=0&fnval=0&qn=80&bvid=%s&cid=%d&platform=html5", bvid, page.Cid)
		var playResp PlayURLResponse
		if err := c.doGet(playURL, &playResp); err != nil {
			continue
		}
		if playResp.Code != 0 || len(playResp.Data.Durl) == 0 {
			continue
		}

		// Pick the largest video stream
		bestDurl := playResp.Data.Durl[0]
		for _, d := range playResp.Data.Durl {
			if d.Size > bestDurl.Size {
				bestDurl = d
			}
		}

		title := formatTitle(viewResp.Data.Title, page.Part, len(pages))
		coverURL := page.FirstFrame()
		if coverURL == "" {
			coverURL = viewResp.Data.Pic
		}

		results = append(results, &VideoInfo{
			URL:      bestDurl.URL,
			Title:    title,
			VideoID:  fmt.Sprintf("%s-%d", bvid, page.Cid),
			CoverURL: coverURL,
			Page:     page.Page,
			Source:   "bilibili",
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("未获取到视频播放地址")
	}
	return results, nil
}

// avToBV converts an AV number to a BV number.
func (c *Client) avToBV(aid string) (string, error) {
	aidNum, err := strconv.ParseInt(aid, 10, 64)
	if err != nil {
		return "", fmt.Errorf("无效的AV号: %s", aid)
	}
	viewURL := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?aid=%d", aidNum)
	var viewResp ViewResponse
	if err := c.doGet(viewURL, &viewResp); err != nil {
		return "", fmt.Errorf("AV转BV失败: %w", err)
	}
	if viewResp.Code != 0 {
		return "", fmt.Errorf("AV转BV失败: code=%d", viewResp.Code)
	}
	return viewResp.Data.Bvid, nil
}

// parseBangumiEpisode parses a bangumi episode URL.
func (c *Client) parseBangumiEpisode(u string) ([]*VideoInfo, error) {
	re := regexp.MustCompile(`bangumi/play/ep(\d+)`)
	match := re.FindStringSubmatch(u)
	if len(match) < 2 {
		return nil, fmt.Errorf("无法从URL提取ep号: %s", u)
	}
	epID := match[1]

	// Get bangumi info
	seasonURL := fmt.Sprintf("https://api.bilibili.com/pgc/view/web/season?ep_id=%s", epID)
	var seasonResp PGCSeasonResponse
	if err := c.doGet(seasonURL, &seasonResp); err != nil {
		return nil, fmt.Errorf("获取番剧信息失败: %w", err)
	}
	if seasonResp.Code != 0 {
		return nil, fmt.Errorf("番剧API错误: code=%d", seasonResp.Code)
	}

	// Collect all episodes
	allEpisodes := seasonResp.Result.Episodes
	for _, section := range seasonResp.Result.Section {
		allEpisodes = append(allEpisodes, section.Episodes...)
	}

	// Find the target episode
	epIDNum, _ := strconv.ParseInt(epID, 10, 64)
	var targetEpisode *PGCEpisode
	for i := range allEpisodes {
		if allEpisodes[i].EpID == epIDNum {
			targetEpisode = &allEpisodes[i]
			break
		}
	}
	if targetEpisode == nil {
		return nil, fmt.Errorf("未找到剧集 ep=%s", epID)
	}

	// Get playback URL (DASH format)
	playURL := fmt.Sprintf("https://api.bilibili.com/pgc/player/web/v2/playurl?fnval=12240&ep_id=%s", epID)
	var playResp PGCPlayURLResponse
	if err := c.doGet(playURL, &playResp); err != nil {
		return nil, fmt.Errorf("获取番剧播放地址失败: %w", err)
	}
	if playResp.Code != 0 {
		return nil, fmt.Errorf("番剧播放API错误: code=%d", playResp.Code)
	}

	return c.buildDASHResults(&playResp.Result.VideoInfo.Dash, targetEpisode.title(), targetEpisode.Cover, epID, 1)
}

// parseBangumiSeason parses a bangumi season URL.
func (c *Client) parseBangumiSeason(u string) ([]*VideoInfo, error) {
	re := regexp.MustCompile(`bangumi/play/ss(\d+)`)
	match := re.FindStringSubmatch(u)
	if len(match) < 2 {
		return nil, fmt.Errorf("无法从URL提取ss号: %s", u)
	}
	ssID := match[1]

	// Get all episodes for the season
	sectionURL := fmt.Sprintf("https://api.bilibili.com/pgc/web/season/section?season_id=%s", ssID)
	var sectionResp PGCSeasonSectionResponse
	if err := c.doGet(sectionURL, &sectionResp); err != nil {
		return nil, fmt.Errorf("获取番剧季信息失败: %w", err)
	}
	if sectionResp.Code != 0 {
		return nil, fmt.Errorf("番剧季API错误: code=%d", sectionResp.Code)
	}

	allEpisodes := sectionResp.Result.MainSection.Episodes
	for _, section := range sectionResp.Result.Section {
		allEpisodes = append(allEpisodes, section.Episodes...)
	}

	var results []*VideoInfo
	for idx, ep := range allEpisodes {
		playURL := fmt.Sprintf("https://api.bilibili.com/pgc/player/web/v2/playurl?fnval=12240&ep_id=%d", ep.EpID)
		var playResp PGCPlayURLResponse
		if err := c.doGet(playURL, &playResp); err != nil {
			continue
		}
		if playResp.Code != 0 || len(playResp.Result.VideoInfo.Dash.Video) == 0 {
			continue
		}

		videoInfos, err := c.buildDASHResults(&playResp.Result.VideoInfo.Dash, ep.title(), ep.Cover, strconv.FormatInt(ep.EpID, 10), idx+1)
		if err == nil && len(videoInfos) > 0 {
			results = append(results, videoInfos[0])
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("未获取到番剧季视频地址")
	}
	return results, nil
}

// parseCheeseEpisode parses a course episode URL.
func (c *Client) parseCheeseEpisode(u string) ([]*VideoInfo, error) {
	re := regexp.MustCompile(`cheese/play/ep(\d+)`)
	match := re.FindStringSubmatch(u)
	if len(match) < 2 {
		return nil, fmt.Errorf("无法从URL提取课程ep号: %s", u)
	}
	epID := match[1]

	seasonURL := fmt.Sprintf("https://api.bilibili.com/pugv/view/web/season?ep_id=%s", epID)
	var seasonResp PUGVSeasonResponse
	if err := c.doGet(seasonURL, &seasonResp); err != nil {
		return nil, fmt.Errorf("获取课程信息失败: %w", err)
	}
	if seasonResp.Code != 0 {
		return nil, fmt.Errorf("课程API错误: code=%d", seasonResp.Code)
	}

	epIDNum, _ := strconv.ParseInt(epID, 10, 64)
	for _, ep := range seasonResp.Data.Episodes {
		if ep.ID != epIDNum {
			continue
		}

		playURL := fmt.Sprintf("https://api.bilibili.com/pugv/player/web/playurl?avid=%d&cid=%d&ep_id=%d&fnval=16&fourk=1",
			ep.Aid, ep.Cid, epIDNum)
		var playResp PUGVPlayURLResponse
		if err := c.doGet(playURL, &playResp); err != nil {
			return nil, fmt.Errorf("获取课程播放地址失败: %w", err)
		}
		if playResp.Code != 0 {
			return nil, fmt.Errorf("课程播放API错误: code=%d", playResp.Code)
		}

		return c.buildDASHResults(&playResp.Data.Dash, ep.Title, ep.Cover, epID, 1)
	}

	return nil, fmt.Errorf("未找到课程剧集 ep=%s", epID)
}

// buildDASHResults builds VideoInfo from DASH data, selecting the best video + audio streams.
func (c *Client) buildDASHResults(dash *DashInfo, title, coverURL, id string, page int) ([]*VideoInfo, error) {
	if dash == nil || len(dash.Video) == 0 {
		return nil, fmt.Errorf("DASH数据为空")
	}

	// Sort video streams by resolution * file size descending
	type formatItem struct {
		url    string
		size   int64
		width  int
		height int
	}
	var videoFormats []formatItem
	for _, v := range dash.Video {
		if v.BaseURL != "" {
			videoFormats = append(videoFormats, formatItem{
				url:    v.BaseURL,
				size:   v.Size,
				width:  v.Width,
				height: v.Height,
			})
		}
	}
	if len(videoFormats) == 0 {
		return nil, fmt.Errorf("无可用的视频流")
	}
	sort.Slice(videoFormats, func(i, j int) bool {
		resI := videoFormats[i].width * videoFormats[i].height
		resJ := videoFormats[j].width * videoFormats[j].height
		if resI != resJ {
			return resI > resJ
		}
		return videoFormats[i].size > videoFormats[j].size
	})

	// Sort audio streams by size descending
	var audioFormats []formatItem
	for _, a := range dash.Audio {
		if a.BaseURL != "" {
			audioFormats = append(audioFormats, formatItem{url: a.BaseURL, size: a.Size})
		}
	}
	sort.Slice(audioFormats, func(i, j int) bool {
		return audioFormats[i].size > audioFormats[j].size
	})

	info := &VideoInfo{
		URL:      videoFormats[0].url,
		Title:    sanitizeTitle(title),
		VideoID:  id,
		CoverURL: coverURL,
		Page:     page,
		Source:   "bilibili",
	}

	if len(audioFormats) > 0 {
		info.AudioURL = audioFormats[0].url
	}

	return []*VideoInfo{info}, nil
}

// formatTitle formats the video title.
func formatTitle(mainTitle, partTitle string, totalPages int) string {
	if totalPages > 1 {
		return sanitizeTitle(partTitle)
	}
	return sanitizeTitle(mainTitle)
}

// sanitizeTitle sanitizes the title string.
func sanitizeTitle(title string) string {
	re := regexp.MustCompile(`[\\/:*?"<>|#\n\r]`)
	title = re.ReplaceAllString(title, "_")
	title = strings.Trim(title, " .")
	if len(title) > 80 {
		title = title[:80]
	}
	return title
}

// parsePageNum extracts the p parameter from a URL.
func parsePageNum(u string) int {
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
