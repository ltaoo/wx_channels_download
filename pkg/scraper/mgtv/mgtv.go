package mgtv

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	initialStateRE = regexp.MustCompile(`window\.__INITIAL_STATE__=([^<]+)<`)
	playURLRE      = regexp.MustCompile(`^/(?:b|h)/([^/]+)/([^/]+?)(?:\.html)?$`)
	boldTagRE      = regexp.MustCompile(`</?B>`)
	countryMap     = map[string]string{"大陆": "CN", "普通话": "CN", "内地": "CN", "中国": "CN", "韩国": "KR", "美国": "US"}
	mediaTypeMap   = map[string]string{"电影": "movie", "电视剧": "tv", "综艺": "tv", "纪录片": "tv"}
	sourceMap      = map[string]string{"mgtv": "mgtv", "imgo": "mgtv", "芒果TV": "mgtv"}
)

const defaultPCUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"

type Client struct {
	HTTPClient *http.Client
	UserAgent  string
}

type Option func(*Client)

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.HTTPClient = client
	}
}

func NewClient(opts ...Option) *Client {
	c := &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		UserAgent:  defaultPCUserAgent,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	return c
}

func (c *Client) Search(ctx context.Context, keyword string) (*SearchResult, error) {
	var resp searchResponse
	if err := c.getJSON(ctx, "https://mobileso.bz.mgtv.com/msite/search/v2", map[string]string{"q": keyword}, &resp); err != nil {
		return nil, err
	}
	return parseSearchResponse(resp)
}

func (c *Client) FetchTVProfile(ctx context.Context, rawURL string) (*TVProfile, error) {
	clipID, videoID, ok := ParsePlayURL(rawURL)
	if ok {
		return c.FetchVODInfo(ctx, clipID, videoID, rawURL)
	}
	html, err := c.getText(ctx, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return ParseTVProfileHTML(rawURL, html)
}

func (c *Client) FetchVODInfo(ctx context.Context, clipID string, videoID string, sourceURL string) (*TVProfile, error) {
	clipID = strings.TrimSpace(clipID)
	videoID = strings.TrimSpace(videoID)
	if clipID == "" || videoID == "" {
		return nil, fmt.Errorf("mgtv clipID and videoID are required")
	}
	apiURL := BuildVODInfoURL(clipID, videoID, "")
	var resp vodInfoResponse
	if err := c.getJSON(ctx, apiURL, nil, &resp); err != nil {
		return nil, err
	}
	profile, err := parseVODInfoResponse(resp)
	if err != nil {
		return nil, err
	}
	profile.SourceURL = sourceURL
	profile.APIURL = apiURL
	return profile, nil
}

func (c *Client) FetchSeasonProfile(ctx context.Context, _ string) (*Season, error) {
	return nil, fmt.Errorf("mgtv season profile is not implemented in source crawler")
}

func (c *Client) FetchEpisodeProfile(ctx context.Context, _ string) (*Episode, error) {
	return nil, fmt.Errorf("mgtv episode profile is not implemented in source crawler")
}

type searchResponse struct {
	Code int `json:"code"`
	Data struct {
		Contents []struct {
			Type string `json:"type"`
			Name string `json:"name"`
			Data []struct {
				Title  string `json:"title"`
				Desc   any    `json:"desc"`
				Img    string `json:"img"`
				URL    string `json:"url"`
				Source string `json:"source"`
			} `json:"data"`
		} `json:"contents"`
	} `json:"data"`
}

func parseSearchResponse(resp searchResponse) (*SearchResult, error) {
	var list []SearchItem
	for _, content := range resp.Data.Contents {
		if content.Type != "media" || len(content.Data) == 0 {
			continue
		}
		item := content.Data[0]
		desc := descText(item.Desc)
		parts := splitAndTrim(desc, "/")
		if len(parts) < 2 {
			continue
		}
		firstAirDate := ""
		if len(parts) > 2 {
			firstAirDate = parts[2]
		}
		itemURL := item.URL
		if itemURL != "" && !strings.HasPrefix(itemURL, "http") {
			itemURL = "https://m.mgtv.com" + itemURL
		}
		list = append(list, SearchItem{
			ID:            itemURL,
			Name:          boldTagRE.ReplaceAllString(item.Title, ""),
			OriginalName:  "",
			Overview:      "",
			PosterPath:    item.Img,
			BackdropPath:  "",
			FirstAirDate:  firstAirDate,
			OriginCountry: filterEmpty([]string{countryMap[parts[1]]}),
			Type:          mediaTypeMap[strings.TrimPrefix(parts[0], "类型: ")],
			Source:        firstNonEmpty(sourceMap[item.Source], item.Source),
		})
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("media contents not found")
	}
	return &SearchResult{List: list}, nil
}

type initialState struct {
	PlayPage struct {
		VideoInfo struct {
			Type        string `json:"0"`
			Area        string `json:"1"`
			Description string `json:"6"`
			SeriesID    string `json:"seriesId"`
			SeriesName  string `json:"seriesName"`
			Image       string `json:"image"`
			ClipStatus  int    `json:"clipStatus"`
			ClipName    string `json:"clipName"`
			ClipID      string `json:"clipId"`
			VideoIndex  int    `json:"videoIndex"`
			Series      []struct {
				SeasonID int64  `json:"seasonId"`
				ClipID   int64  `json:"clipId"`
				Title    string `json:"title"`
			} `json:"series"`
		} `json:"videoinfo"`
	} `json:"playPage"`
}

type vodInfoResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data vodInfoData `json:"data"`
}

type vodInfoData struct {
	Info struct {
		ShareInfo struct {
			Desc  string `json:"desc"`
			Image string `json:"image"`
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"shareInfo"`
		Video struct {
			VideoID     string `json:"videoId"`
			PartName    string `json:"partName"`
			SerialNo    string `json:"serialno"`
			ReleaseTime string `json:"releaseTime"`
			ContentType string `json:"contentType"`
		} `json:"video"`
		Clip struct {
			ClipID      string `json:"clipId"`
			ClipName    string `json:"clipName"`
			Kind        string `json:"kind"`
			Story       string `json:"story"`
			VImgURL     string `json:"vImgUrl"`
			SerialCount string `json:"serialCount"`
			ContentType string `json:"contentType"`
		} `json:"clip"`
		Template struct {
			Modules []vodModule `json:"modules"`
		} `json:"template"`
	} `json:"info"`
}

type vodModule struct {
	Title string    `json:"title"`
	Media *vodMedia `json:"media"`
}

type vodMedia struct {
	Page struct {
		Total int `json:"total"`
	} `json:"page"`
	List []vodMediaItem `json:"list"`
}

type vodMediaItem struct {
	VideoID     string `json:"videoId"`
	ClipID      string `json:"clipId"`
	SerialNo    string `json:"serialno"`
	Title       string `json:"title"`
	Duration    string `json:"duration"`
	HImgURL     string `json:"hImgUrl"`
	VImgURL     string `json:"vImgUrl"`
	DetailTitle struct {
		Title string `json:"title"`
	} `json:"detailTitle"`
}

func ParsePlayURL(rawURL string) (clipID string, videoID string, ok bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u == nil {
		return "", "", false
	}
	match := playURLRE.FindStringSubmatch(u.Path)
	if len(match) < 3 {
		return "", "", false
	}
	clipID = strings.TrimSuffix(strings.TrimSpace(match[1]), ".html")
	videoID = strings.TrimSuffix(strings.TrimSpace(match[2]), ".html")
	return clipID, videoID, clipID != "" && videoID != ""
}

func BuildVODInfoURL(clipID string, videoID string, deviceID string) string {
	if strings.TrimSpace(deviceID) == "" {
		deviceID = newDeviceID()
	}
	q := url.Values{}
	q.Set("allowedRC", "1")
	q.Set("_support", "10000000")
	q.Set("uuid", deviceID)
	q.Set("ticket", "")
	q.Set("did", deviceID)
	q.Set("device", "pc")
	q.Set("osType", "mac")
	q.Set("osVersion", "10_15")
	q.Set("appVersion", "9.0.2")
	q.Set("platform", "4")
	q.Set("seqId", deviceID)
	q.Set("src", "mgtv")
	q.Set("videoId", strings.TrimSpace(videoID))
	q.Set("clipId", strings.TrimSpace(clipID))
	return "https://mobile-thor.api.mgtv.com/v1/vod/info?" + q.Encode()
}

func parseVODInfoResponse(resp vodInfoResponse) (*TVProfile, error) {
	if resp.Code != 0 && resp.Code != 200 {
		return nil, fmt.Errorf("mgtv vod info error %d: %s", resp.Code, resp.Msg)
	}
	info := resp.Data.Info
	if info.Clip.ClipID == "" && info.Video.VideoID == "" && info.Clip.ClipName == "" {
		return nil, fmt.Errorf("mgtv vod info is empty")
	}
	episodes := episodesFromVODModules(info.Template.Modules, info.Clip.ClipID)
	current := currentEpisode(episodes, info.Video.VideoID)
	if current == nil && info.Video.VideoID != "" {
		episode := Episode{
			ID:            info.Video.VideoID,
			Name:          firstNonEmpty(info.Video.PartName, info.ShareInfo.Title, info.Clip.ClipName),
			URL:           playURL(info.Clip.ClipID, info.Video.VideoID),
			Thumbnail:     firstNonEmpty(info.ShareInfo.Image, info.Clip.VImgURL),
			EpisodeNumber: positiveInt(info.Video.SerialNo, 1),
			AirDate:       dateOnly(info.Video.ReleaseTime),
		}
		current = &episode
		if len(episodes) == 0 {
			episodes = append(episodes, episode)
		}
	}
	profile := &TVProfile{
		Platform:        "mgtv",
		Type:            "tv",
		ID:              info.Clip.ClipID,
		ClipID:          info.Clip.ClipID,
		VideoID:         info.Video.VideoID,
		Name:            firstNonEmpty(info.Clip.ClipName, info.ShareInfo.Title, info.Video.PartName),
		Overview:        firstNonEmpty(info.Clip.Story, info.ShareInfo.Desc),
		PosterPath:      firstNonEmpty(info.Clip.VImgURL, info.ShareInfo.Image),
		BackdropPath:    firstNonEmpty(info.ShareInfo.Image, info.Clip.VImgURL),
		OriginalName:    info.Video.PartName,
		Kind:            info.Clip.Kind,
		CurrentEpisode:  current,
		Seasons:         []Season{{ID: info.Clip.ClipID, Name: info.Clip.ClipName, PosterPath: firstNonEmpty(info.Clip.VImgURL, info.ShareInfo.Image), SeasonNumber: 1, Episodes: episodes}},
		FirstAirDate:    dateOnly(info.Video.ReleaseTime),
		NumberOfSeasons: 1,
		Status:          "",
	}
	return profile, nil
}

func ParseTVProfileHTML(id string, html string) (*TVProfile, error) {
	match := initialStateRE.FindStringSubmatch(html)
	if len(match) < 2 {
		return nil, fmt.Errorf("missing __INITIAL_STATE__")
	}
	var state initialState
	if err := json.Unmarshal([]byte(match[1]), &state); err != nil {
		return nil, err
	}
	info := state.PlayPage.VideoInfo
	return &TVProfile{
		Platform:     "mgtv",
		Type:         "tv",
		ID:           id,
		ClipID:       info.ClipID,
		Name:         firstNonEmpty(info.ClipName, info.SeriesName),
		Overview:     strings.TrimPrefix(info.Description, "简介："),
		PosterPath:   info.Image,
		BackdropPath: "",
		OriginalName: "",
		Seasons: []Season{{
			VoteAverage: 0,
		}},
		FirstAirDate:    "",
		VoteAverage:     0,
		Popularity:      0,
		NumberOfSeasons: 1,
		Status:          "",
	}, nil
}

func (c *Client) getText(ctx context.Context, rawURL string, query map[string]string) (string, error) {
	body, err := c.get(ctx, rawURL, query)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Client) getJSON(ctx context.Context, rawURL string, query map[string]string, out any) error {
	body, err := c.get(ctx, rawURL, query)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

func (c *Client) get(ctx context.Context, rawURL string, query map[string]string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for key, value := range query {
		if value != "" {
			q.Set(key, value)
		}
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	host := strings.ToLower(u.Hostname())
	if host == "api.mgtv.com" || strings.HasSuffix(host, ".api.mgtv.com") {
		req.Header.Set("Origin", "https://www.mgtv.com")
		req.Header.Set("Referer", "https://www.mgtv.com/")
		req.Header.Set("Sec-Fetch-Dest", "empty")
		req.Header.Set("Sec-Fetch-Mode", "cors")
		req.Header.Set("Sec-Fetch-Site", "same-site")
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func descText(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, "/")
	default:
		return fmt.Sprint(v)
	}
}

func splitAndTrim(value string, sep string) []string {
	parts := strings.Split(value, sep)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func filterEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func episodesFromVODModules(modules []vodModule, clipID string) []Episode {
	seen := map[string]bool{}
	var episodes []Episode
	for _, module := range modules {
		if module.Media == nil {
			continue
		}
		for _, item := range module.Media.List {
			if item.VideoID == "" || seen[item.VideoID] {
				continue
			}
			if clipID != "" && item.ClipID != "" && item.ClipID != clipID {
				continue
			}
			seen[item.VideoID] = true
			episodeNumber := positiveInt(item.SerialNo, len(episodes)+1)
			episodes = append(episodes, Episode{
				ID:            item.VideoID,
				Name:          firstNonEmpty(item.Title, item.DetailTitle.Title),
				URL:           playURL(firstNonEmpty(item.ClipID, clipID), item.VideoID),
				Thumbnail:     firstNonEmpty(item.HImgURL, item.VImgURL),
				Duration:      item.Duration,
				EpisodeNumber: episodeNumber,
			})
		}
	}
	return episodes
}

func currentEpisode(episodes []Episode, videoID string) *Episode {
	for i := range episodes {
		if episodes[i].ID == videoID {
			return &episodes[i]
		}
	}
	return nil
}

func playURL(clipID string, videoID string) string {
	if strings.TrimSpace(clipID) == "" || strings.TrimSpace(videoID) == "" {
		return ""
	}
	return fmt.Sprintf("https://www.mgtv.com/b/%s/%s.html", strings.TrimSpace(clipID), strings.TrimSpace(videoID))
}

func positiveInt(value string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func dateOnly(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= len("2006-01-02") {
		return value[:len("2006-01-02")]
	}
	return value
}

func newDeviceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		now := time.Now().UnixNano()
		return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", uint32(now>>32), uint16(now>>16), uint16(now), uint16(now>>48), uint64(now)&0xffffffffffff)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
