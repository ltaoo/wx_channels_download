package iqiyi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	playPageInfoRE  = regexp.MustCompile(`window\.Q\.PageInfo\.playPageInfo\s*=\s*(.+?);`)
	nuxtRE          = regexp.MustCompile(`window\.__NUXT__\s*=\s*(.+?)</script>`)
	videoPageIDRE   = regexp.MustCompile(`/[vwp]_([^/?#]+)\.html`)
	regexpImageSize = regexp.MustCompile(`_[0-9]{3}_[0-9]{3}$`)
	countryMap      = map[string]string{"大陆": "CN", "普通话": "CN", "内地": "CN", "中国": "CN", "韩国": "KR", "美国": "US"}
	mediaTypeMap    = map[string]string{"电影": "movie", "电视剧": "tv", "综艺": "tv", "纪录片": "tv"}
	mediaGenreMap   = map[string]string{"真人秀": "真人秀", "综艺": "真人秀", "剧情": "剧情", "犯罪": "犯罪", "动作": "动作"}
)

const (
	tvidXORKey   uint64 = 0x75706971676c
	maxInt64Uint uint64 = 1<<63 - 1
)

type Client struct {
	HTTPClient *http.Client
	UserAgent  string
	Cookie     string
}

type Option func(*Client)

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.HTTPClient = client
	}
}

func WithCookie(cookie string) Option {
	return func(c *Client) {
		c.Cookie = cookie
	}
}

func NewClient(opts ...Option) *Client {
	c := &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		UserAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	return c
}

func (c *Client) FetchProfilePage(ctx context.Context, rawURL string) (*ProfilePageInfo, error) {
	html, err := c.getText(ctx, rawURL, nil)
	if err != nil {
		if tvid, ok := ParseTVID(rawURL); ok {
			return &ProfilePageInfo{TVID: tvid}, nil
		}
		return nil, err
	}
	profile, err := ParseProfilePage(html)
	if err == nil {
		return profile, nil
	}
	if tvid, ok := ParseTVID(rawURL); ok {
		return &ProfilePageInfo{TVID: tvid}, nil
	}
	return nil, err
}

func ParseProfilePage(html string) (*ProfilePageInfo, error) {
	match := playPageInfoRE.FindStringSubmatch(html)
	if len(match) < 2 {
		return nil, fmt.Errorf("missing playPageInfo")
	}
	var info ProfilePageInfo
	if err := json.Unmarshal([]byte(match[1]), &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *Client) FetchBaseInfo(ctx context.Context, id any) (*BaseInfo, error) {
	base, err := c.fetchBaseInfo(ctx, "https://mesh.if.iqiyi.com/tvg/v2/lw/base_info", BuildLWQuery(map[string]any{"entity_id": id}))
	if err == nil {
		return base, nil
	}
	legacyBase, legacyErr := c.fetchBaseInfo(ctx, "https://mesh.if.iqiyi.com/tvg/pcw/base_info", BuildQuery(map[string]any{"entity_id": id}))
	if legacyErr == nil {
		return legacyBase, nil
	}
	return nil, err
}

func (c *Client) fetchBaseInfo(ctx context.Context, rawURL string, query map[string]any) (*BaseInfo, error) {
	var resp baseInfoResponse
	if err := c.getJSON(ctx, rawURL, query, &resp); err != nil {
		return nil, err
	}
	return parseBaseInfoResponse(resp)
}

func parseBaseInfoResponse(resp baseInfoResponse) (*BaseInfo, error) {
	base := resp.Data.BaseData
	if base.ID == 0 {
		if resp.Msg != "" {
			return nil, fmt.Errorf("empty iqiyi base info: %s", resp.Msg)
		}
		return nil, fmt.Errorf("empty iqiyi base info")
	}
	sources := resp.Data.Template.SeasonSources()
	sort.Slice(sources, func(i, j int) bool { return sources[i].Order < sources[j].Order })
	seasons := make([]Season, 0, len(sources))
	for _, source := range sources {
		id := source.EntityID
		if id == 0 {
			id = base.ID
		}
		episodes := episodesFromVideos(source.Videos)
		airDate := ""
		if len(episodes) > 0 {
			airDate = episodes[0].AirDate
		}
		seasons = append(seasons, Season{
			ID:       id,
			Name:     firstNonEmpty(source.TabName, base.Title),
			AirDate:  airDate,
			Episodes: episodes,
		})
	}
	return &BaseInfo{
		ID:           base.ID,
		ShareURL:     base.ShareURL,
		Title:        base.Title,
		Desc:         base.Desc,
		ImageURL:     base.ImageURL,
		PublishDate:  base.PublishDate,
		TotalEpisode: base.TotalEpisode,
		Seasons:      seasons,
	}, nil
}

func (t baseInfoTemplate) SeasonSources() []seasonSource {
	if len(t.PureData.SourceSelectorBK) > 0 {
		return t.PureData.SourceSelectorBK
	}
	if len(t.PureData.SelectorBK) > 0 {
		return t.PureData.SelectorBK
	}
	for _, tab := range t.Tabs {
		for _, block := range tab.Blocks {
			if block.BKID != "selector_bk" || block.Data.Data == nil {
				continue
			}
			data, err := json.Marshal(block.Data.Data)
			if err != nil {
				continue
			}
			var sources []seasonSource
			if err := json.Unmarshal(data, &sources); err == nil && len(sources) > 0 {
				return sources
			}
		}
	}
	return nil
}

func ParseTVID(rawURL string) (int64, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return 0, false
	}
	parsed, err := url.Parse(rawURL)
	if err == nil && parsed != nil {
		if id, ok := parsePositiveID(parsed.Query().Get("positiveId")); ok {
			return id, true
		}
		if id, ok := parseTVIDPath(parsed.EscapedPath()); ok {
			return id, true
		}
	}
	return parseTVIDPath(rawURL)
}

func parsePositiveID(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	body, err := base64.StdEncoding.DecodeString(withBase64Padding(value))
	if err != nil {
		body, err = base64.RawStdEncoding.DecodeString(value)
	}
	if err != nil {
		return 0, false
	}
	id, err := strconv.ParseInt(strings.TrimSpace(string(body)), 10, 64)
	return id, err == nil && id > 0
}

func parseTVIDPath(value string) (int64, bool) {
	match := videoPageIDRE.FindStringSubmatch(value)
	if len(match) < 2 {
		return 0, false
	}
	encoded, err := strconv.ParseUint(match[1], 36, 64)
	if err != nil {
		return 0, false
	}
	id := encoded ^ tvidXORKey
	if id < 900000 {
		id = 100 * (id + 900000)
	}
	if id == 0 || id > maxInt64Uint {
		return 0, false
	}
	return int64(id), true
}

func (c *Client) FetchProfileWithSeasons(ctx context.Context, rawURL string) (*ProfileWithSeasons, error) {
	profile, err := c.FetchProfilePage(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	base, err := c.FetchBaseInfo(ctx, profile.TVID)
	if err != nil {
		return nil, err
	}
	typeMap := map[int]string{1: "movie", 2: "season"}
	poster := FormatPosterPath(base.ImageURL)["s4"]
	seasons := make([]Season, 0, len(base.Seasons))
	if len(base.Seasons) == 0 && profile.ChannelID == 1 {
		seasons = append(seasons, Season{
			ID:            base.ID,
			Name:          base.Title,
			Overview:      base.Desc,
			PosterPath:    poster,
			AirDate:       yyyyMMdd(base.PublishDate),
			Genres:        genresFromCategories(profile.Categories),
			OriginCountry: countriesFromCategories(profile.Categories, map[int]bool{0: true, 1: true}),
			Persons:       FormatPeople(profile.People),
		})
	} else {
		for _, season := range base.Seasons {
			if season.ID == base.ID {
				latest := ""
				if len(season.Episodes) > 0 {
					latest = yyyyMMdd(season.Episodes[len(season.Episodes)-1].AirDate)
				}
				season.Name = base.Title
				season.Overview = base.Desc
				season.PosterPath = poster
				season.Genres = genresFromCategories(profile.Categories)
				season.OriginCountry = countriesFromCategories(profile.Categories, map[int]bool{1: true})
				season.Persons = FormatPeople(profile.People)
				season.AirDate = latest
			}
			seasons = append(seasons, season)
		}
	}
	return &ProfileWithSeasons{
		Platform:     "iqiyi",
		Type:         firstNonEmpty(typeMap[profile.ChannelID], "season"),
		ID:           base.ID,
		Name:         base.Title,
		Overview:     base.Desc,
		PosterPath:   poster,
		BackdropPath: "",
		OriginalName: "",
		Seasons:      seasons,
	}, nil
}

func (c *Client) Search(ctx context.Context, keyword string) (*SearchResult, error) {
	escaped := url.QueryEscape(keyword)
	rawURL := "https://so.iqiyi.com/so/q_" + escaped
	html, err := c.getText(ctx, rawURL, map[string]any{"source": "input"})
	if err != nil {
		return nil, err
	}
	return ParseSearchHTML(html)
}

func ParseSearchHTML(html string) (*SearchResult, error) {
	match := nuxtRE.FindStringSubmatch(html)
	if len(match) < 2 {
		return nil, fmt.Errorf("window.__NUXT__ not found")
	}
	rawJSON, err := evalJSObjectJSON(match[1])
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data []struct {
			CardData *struct {
				List []struct {
					SiteID      string `json:"siteId"`
					SiteName    string `json:"siteName"`
					Tag         string `json:"tag"`
					Title       string `json:"g_title"`
					MainLink    string `json:"g_main_link"`
					Desc        string `json:"desc"`
					Image       string `json:"g_img"`
					Year        string `json:"year"`
					Region      string `json:"region"`
					ReleaseTime string `json:"releaseTime"`
				} `json:"list"`
			} `json:"cardData"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rawJSON, &payload); err != nil {
		return nil, err
	}
	for _, block := range payload.Data {
		if block.CardData == nil {
			continue
		}
		result := SearchResult{List: make([]SearchItem, 0, len(block.CardData.List))}
		for _, card := range block.CardData.List {
			result.List = append(result.List, SearchItem{
				ID:            card.MainLink,
				Name:          stripEm(card.Title),
				OriginalName:  "",
				Overview:      card.Desc,
				PosterPath:    card.Image,
				BackdropPath:  "",
				FirstAirDate:  card.ReleaseTime,
				OriginCountry: filterEmpty([]string{countryMap[card.Region]}),
				Type:          mediaTypeMap[card.Tag],
				Source:        card.SiteID,
			})
		}
		return &result, nil
	}
	return nil, fmt.Errorf("cardData not found")
}

func (c *Client) FetchSeasonProfile(ctx context.Context, seasonID any) (*SeasonProfile, error) {
	base, err := c.FetchBaseInfo(ctx, seasonID)
	if err != nil {
		return nil, err
	}
	query := BuildQuery(map[string]any{"album_id": seasonID})
	var selector selectorResponse
	if err := c.getJSON(ctx, "https://mesh.if.iqiyi.com/tvg/v2/selector", query, &selector); err != nil {
		return nil, err
	}
	episodes := episodesFromVideos(selector.Data.Videos)
	if len(episodes) == 0 {
		return nil, fmt.Errorf("empty episode list")
	}
	profile, err := c.FetchProfilePage(ctx, episodes[0].ID)
	if err != nil {
		return nil, err
	}
	return &SeasonProfile{
		ID:            base.ID,
		Name:          base.Title,
		OriginalName:  "",
		Overview:      base.Desc,
		PosterPath:    FormatPosterPath(base.ImageURL)["s4"],
		BackdropPath:  "",
		Episodes:      episodes,
		AirDate:       episodes[0].AirDate,
		Genres:        genresFromCategories(profile.Categories),
		OriginCountry: countriesFromCategories(profile.Categories, map[int]bool{1: true}),
		Persons:       FormatPeople(profile.People),
	}, nil
}

func (c *Client) getText(ctx context.Context, rawURL string, query map[string]any) (string, error) {
	body, err := c.get(ctx, rawURL, query)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Client) getJSON(ctx context.Context, rawURL string, query map[string]any, out any) error {
	body, err := c.get(ctx, rawURL, query)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	return decoder.Decode(out)
}

func (c *Client) get(ctx context.Context, rawURL string, query map[string]any) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	for key, value := range query {
		if value != nil {
			q.Set(key, fmt.Sprint(value))
		}
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Referer", "https://so.iqiyi.com/")
	if c.Cookie != "" {
		req.Header.Set("Cookie", c.Cookie)
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

func episodesFromVideos(raw any) []Episode {
	data, err := json.Marshal(raw)
	if err != nil || len(data) == 0 || string(data) == `""` || string(data) == "null" {
		return nil
	}
	var groups []struct {
		Title string      `json:"title"`
		Data  []videoItem `json:"data"`
	}
	if json.Unmarshal(data, &groups) == nil && len(groups) > 0 {
		var out []Episode
		for _, group := range groups {
			for i, item := range group.Data {
				out = append(out, episodeFromVideo(item, i+1))
			}
		}
		return out
	}
	var object struct {
		FeaturePaged map[string][]videoItem `json:"feature_paged"`
		PageKeys     []string               `json:"page_keys"`
	}
	if json.Unmarshal(data, &object) == nil && len(object.FeaturePaged) > 0 {
		var items []videoItem
		for _, values := range object.FeaturePaged {
			items = append(items, values...)
		}
		filtered := items[:0]
		for _, item := range items {
			if item.ContentType == 1 || item.ContentType == 28 {
				filtered = append(filtered, item)
			}
		}
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].AlbumOrder < filtered[j].AlbumOrder })
		out := make([]Episode, 0, len(filtered))
		for i, item := range filtered {
			out = append(out, episodeFromVideo(item, i+1))
		}
		return out
	}
	return nil
}

func episodeFromVideo(item videoItem, fallbackOrder int) Episode {
	order := item.AlbumOrder
	if order == 0 {
		order = fallbackOrder
	}
	name := firstNonEmpty(item.ShortDisplayName, item.Title)
	return Episode{
		ID:            item.PageURL,
		Name:          name,
		AirDate:       item.PublishDate,
		EpisodeNumber: order,
		Thumbnail:     item.ImageURL,
	}
}

func genresFromCategories(categories []Category) []string {
	var out []string
	for _, category := range categories {
		if category.SubType == 2 {
			if genre := mediaGenreMap[category.Name]; genre != "" {
				out = append(out, genre)
			}
		}
	}
	return out
}

func countriesFromCategories(categories []Category, subTypes map[int]bool) []string {
	var out []string
	for _, category := range categories {
		if subTypes[category.SubType] {
			if country := countryMap[category.Name]; country != "" {
				out = append(out, country)
			}
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
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func stripEm(text string) string {
	text = strings.ReplaceAll(text, "<em>", "")
	text = strings.ReplaceAll(text, "</em>", "")
	return text
}

func yyyyMMdd(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == 8 {
		return value[:4] + "-" + value[4:6] + "-" + value[6:8]
	}
	return value
}
