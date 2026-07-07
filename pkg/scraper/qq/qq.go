package qq

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	htmlpkg "html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/dop251/goja"
)

var (
	piniaRE    = regexp.MustCompile(`window\.__PINIA__\s*=\s*([^<]+)<`)
	emTagRE    = regexp.MustCompile(`</?em>`)
	htmlTagRE  = regexp.MustCompile(`<[^>]+>`)
	countryMap = map[string]string{"大陆": "CN", "普通话": "CN", "内地": "CN", "中国": "CN", "韩国": "KR", "美国": "US"}
	sourceMap  = map[string]string{"腾讯": "qq", "qq": "qq", "youku": "youku", "iqiyi": "iqiyi", "mgtv": "mgtv"}
)

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
		UserAgent:  "Mozilla/5.0 (iPhone; CPU iPhone OS 16_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.43(0x18002b2c) NetType/WIFI Language/zh_CN",
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
	body := map[string]any{
		"version":       "",
		"clientType":    1,
		"filterValue":   "firstTabid=150",
		"uuid":          "9E7C14F7-167B-4F3B-A91E-F607CAC48DE9",
		"retry":         0,
		"query":         keyword,
		"pagenum":       0,
		"pagesize":      20,
		"queryFrom":     4,
		"sceneId":       21,
		"searchDatakey": "",
		"isneedQc":      true,
		"preQid":        "XWGTmIWICDodZWq1UJCk1SISj8KjtV4UMAivnNt21Wr6bf4RJB_hwQ",
		"adClientInfo":  "",
		"extraInfo": map[string]any{
			"isNewMarkLabel": "",
		},
		"platform": "23",
	}
	var resp searchResponse
	if err := c.postJSON(ctx, "https://pbaccess.video.qq.com/trpc.videosearch.mobile_search.HttpMobileRecall/MbSearchHttp", body, &resp); err != nil {
		return nil, err
	}
	return parseSearchResponse(resp)
}

func (c *Client) FetchTVProfile(ctx context.Context, idOrURL string) (*TVProfile, error) {
	rawURL := idOrURL
	if !strings.HasPrefix(rawURL, "http") {
		rawURL = "https://v.qq.com/x/cover/" + idOrURL + ".html"
	}
	html, err := c.getText(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	return ParseTVProfileHTML(idOrURL, html)
}

func (c *Client) FetchSeasonProfile(ctx context.Context, idOrURL string) (*SeasonProfile, error) {
	rawURL := idOrURL
	if !strings.HasPrefix(rawURL, "http") {
		rawURL = "https://v.qq.com/x/cover/" + idOrURL + ".html"
	}
	html, err := c.getText(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	return ParseSeasonProfileHTML(html)
}

func (c *Client) FetchVideoDetailPage(ctx context.Context, rawURL string) (*VideoDetailPage, error) {
	pageURL, ok := ParseVideoPageURL(rawURL)
	if !ok {
		return nil, fmt.Errorf("unsupported qq video URL: %s", rawURL)
	}
	if pageURL.CID == "" || pageURL.VID == "" {
		return nil, fmt.Errorf("qq video URL requires both cid and vid: %s", rawURL)
	}
	guid := newVQQDeviceGUID()
	apiURL := buildVQQPageAPIURL(guid)
	payload := buildVQQPagePayload(pageURL, guid)
	var resp pageServiceResponse
	if err := c.postVQQPageJSON(ctx, apiURL, payload, &resp, guid); err != nil {
		return nil, err
	}
	return parseVideoDetailPageResponse(pageURL, apiURL, resp)
}

func ParseVideoPageURL(rawURL string) (VideoPageURL, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return VideoPageURL{}, false
	}
	u, err := url.Parse(rawURL)
	if err != nil || u == nil {
		return VideoPageURL{}, false
	}
	if u.Scheme == "" {
		u, err = url.Parse("https://" + rawURL)
		if err != nil || u == nil {
			return VideoPageURL{}, false
		}
	}
	host := strings.ToLower(u.Hostname())
	if host != "v.qq.com" && host != "m.v.qq.com" {
		return VideoPageURL{}, false
	}

	out := VideoPageURL{
		Raw: rawURL,
		CID: strings.TrimSpace(u.Query().Get("cid")),
		VID: strings.TrimSpace(u.Query().Get("vid")),
	}
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i < len(segments); i++ {
		if segments[i] != "cover" {
			continue
		}
		cidIndex := i + 1
		if cidIndex < len(segments) && segments[cidIndex] == "x" {
			cidIndex++
		}
		if cidIndex < len(segments) && out.CID == "" {
			out.CID = trimHTMLSuffix(segments[cidIndex])
		}
		vidIndex := cidIndex + 1
		if vidIndex < len(segments) && out.VID == "" {
			out.VID = trimHTMLSuffix(segments[vidIndex])
		}
	}
	switch {
	case out.CID != "" && out.VID != "":
		out.Canonical = "https://v.qq.com/x/cover/" + out.CID + "/" + out.VID + ".html"
	case out.CID != "":
		out.Canonical = "https://v.qq.com/x/cover/" + out.CID + ".html"
	default:
		out.Canonical = u.String()
	}
	return out, out.CID != "" || out.VID != ""
}

type searchResponse struct {
	Data struct {
		ErrCode    int    `json:"errcode"`
		ErrMessage string `json:"errmsg"`
		NormalList struct {
			ItemList []struct {
				Doc struct {
					ID string `json:"id"`
				} `json:"doc"`
				VideoInfo struct {
					Year      int    `json:"year"`
					Title     string `json:"title"`
					Area      string `json:"area"`
					TypeName  string `json:"typeName"`
					ImgURL    string `json:"imgUrl"`
					PlaySites []struct {
						UIType       int    `json:"uiType"`
						ShowName     string `json:"showName"`
						EnName       string `json:"enName"`
						TotalEpisode int    `json:"totalEpisode"`
					} `json:"playSites"`
				} `json:"videoInfo"`
			} `json:"itemList"`
		} `json:"normalList"`
	} `json:"data"`
}

func parseSearchResponse(resp searchResponse) (*SearchResult, error) {
	if resp.Data.ErrCode != 0 {
		return nil, fmt.Errorf(resp.Data.ErrMessage)
	}
	out := SearchResult{List: make([]SearchItem, 0, len(resp.Data.NormalList.ItemList))}
	for _, item := range resp.Data.NormalList.ItemList {
		playSite := struct {
			UIType       int
			ShowName     string
			EnName       string
			TotalEpisode int
		}{}
		if len(item.VideoInfo.PlaySites) > 0 {
			playSite.UIType = item.VideoInfo.PlaySites[0].UIType
			playSite.EnName = item.VideoInfo.PlaySites[0].EnName
		}
		mediaType := "tv"
		if playSite.UIType == 3 {
			mediaType = "movie"
		}
		out.List = append(out.List, SearchItem{
			ID:            item.Doc.ID,
			Name:          emTagRE.ReplaceAllString(item.VideoInfo.Title, ""),
			OriginalName:  "",
			Overview:      "",
			PosterPath:    item.VideoInfo.ImgURL,
			BackdropPath:  "",
			FirstAirDate:  fmt.Sprint(item.VideoInfo.Year),
			OriginCountry: filterEmpty([]string{countryMap[item.VideoInfo.Area]}),
			Type:          mediaType,
			Source:        firstNonEmpty(sourceMap[playSite.EnName], playSite.EnName),
		})
	}
	return &out, nil
}

func ParseTVProfileHTML(id string, html string) (*TVProfile, error) {
	state, err := parsePinia(html)
	if err != nil {
		return nil, err
	}
	seasons := make([]Season, 0, len(state.EpisodeMain.EpTabs))
	for i, tab := range state.EpisodeMain.EpTabs {
		seasons = append(seasons, Season{
			ID:           tab.PageContext,
			Name:         tab.Text,
			PosterPath:   "",
			SeasonNumber: i + 1,
		})
	}
	return &TVProfile{
		ID:              id,
		Name:            state.Global.CoverInfo.Title,
		Overview:        state.Global.CoverInfo.Description,
		PosterPath:      "",
		BackdropPath:    "",
		Seasons:         seasons,
		NumberOfSeasons: len(seasons),
	}, nil
}

func ParseSeasonProfileHTML(html string) (*SeasonProfile, error) {
	state, err := parsePinia(html)
	if err != nil {
		return nil, err
	}
	cover := state.Global.CoverInfo
	var episodes []Episode
	if len(state.EpisodeMain.ListData) > 0 && len(state.EpisodeMain.ListData[0].List) > 0 {
		for _, item := range state.EpisodeMain.ListData[0].List[0] {
			episodes = append(episodes, Episode{
				ID:            item.VID,
				Name:          item.Title,
				Overview:      "",
				Thumbnail:     item.Pic,
				AirDate:       item.PublishDate,
				EpisodeNumber: item.Index,
				Duration:      item.Duration,
			})
		}
	}
	return &SeasonProfile{
		ID:              cover.CoverID,
		Name:            cover.Title,
		Overview:        cover.Description,
		PosterPath:      "",
		BackdropPath:    "",
		AirDate:         cover.PublishDate,
		SeasonNumber:    0,
		Genres:          filterEmpty([]string{cover.TypeName}),
		OriginCountry:   filterEmpty([]string{countryMap[cover.AreaName]}),
		NumberOfEpisode: atoi(cover.EpisodeAll),
		Episodes:        episodes,
		Persons:         []Person{},
	}, nil
}

func parseVideoDetailPageResponse(pageURL VideoPageURL, apiURL string, resp pageServiceResponse) (*VideoDetailPage, error) {
	if resp.Ret != 0 {
		return nil, fmt.Errorf("qq getPage ret=%d msg=%s", resp.Ret, resp.Msg)
	}
	if pageURL.CID == "" {
		pageURL.CID = strings.TrimSpace(resp.Data.OtherPageInfo["cid"])
	}
	if pageURL.VID == "" {
		pageURL.VID = strings.TrimSpace(resp.Data.OtherPageInfo["vid"])
	}
	if pageURL.Canonical == "" && pageURL.CID != "" && pageURL.VID != "" {
		pageURL.Canonical = "https://v.qq.com/x/cover/" + pageURL.CID + "/" + pageURL.VID + ".html"
	}
	detail := &VideoDetailPage{
		URL:      pageURL,
		APIURL:   apiURL,
		CID:      firstNonEmpty(pageURL.CID, resp.Data.OtherPageInfo["cid"]),
		VID:      firstNonEmpty(pageURL.VID, resp.Data.OtherPageInfo["vid"]),
		Episodes: []VideoEpisode{},
	}
	seenEpisodes := map[string]bool{}
	currentEpisodeIndex := -1
	walkPageCards(resp.Data.CardList, func(card pageCard) {
		params := card.Params
		if len(params) == 0 {
			return
		}
		if card.Type == "pc_introduction" {
			applyIntroParams(detail, params)
		}
		if card.Type != "pc_web_episode_list" {
			return
		}
		vid := strings.TrimSpace(params["vid"])
		if vid == "" || seenEpisodes[vid] {
			return
		}
		seenEpisodes[vid] = true
		episode := videoEpisodeFromParams(params)
		detail.Episodes = append(detail.Episodes, episode)
		if episode.VID == detail.VID {
			currentEpisodeIndex = len(detail.Episodes) - 1
		}
	})
	if currentEpisodeIndex >= 0 {
		detail.CurrentEpisode = &detail.Episodes[currentEpisodeIndex]
	}
	if detail.Title == "" {
		return nil, fmt.Errorf("qq getPage missing title")
	}
	return detail, nil
}

func applyIntroParams(detail *VideoDetailPage, params map[string]string) {
	detail.Title = firstNonEmpty(detail.Title, params["title"], params["mod_title"])
	detail.Description = firstNonEmpty(detail.Description, params["cover_description"], params["video_description"])
	detail.Year = firstNonEmpty(detail.Year, params["year"], params["cover_year"], params["show_year"])
	detail.AreaName = firstNonEmpty(detail.AreaName, params["area_name"])
	detail.TypeName = firstNonEmpty(detail.TypeName, params["main_genres"], params["sub_genre"])
	detail.DetailInfo = firstNonEmpty(detail.DetailInfo, cleanRichText(params["detail_info"]), cleanRichText(params["normal_matrix_info"]))
	detail.UpdateNotifyDesc = firstNonEmpty(detail.UpdateNotifyDesc, params["update_notify_desc"], params["holly_online_time"])
	detail.Hot = firstNonEmpty(detail.Hot, params["hotval"], params["hot_num"])
	detail.Score = firstNonEmpty(detail.Score, parseVideoScore(params["score_info"]))
	detail.CoverURL = firstNonEmpty(detail.CoverURL, params["new_pic_hz"])
	detail.VerticalCoverURL = firstNonEmpty(detail.VerticalCoverURL, params["new_pic_vt"])
	detail.TitleImageURL = firstNonEmpty(detail.TitleImageURL, params["title_image_url"])
	if detail.EpisodeAll == 0 {
		detail.EpisodeAll = atoi(params["episode_all"])
	}
	detail.Genres = appendUniqueStrings(detail.Genres, parseGenreList(params["poi_infos"])...)
	detail.Genres = appendUniqueStrings(detail.Genres, params["main_genres"], params["sub_genre"])
}

func videoEpisodeFromParams(params map[string]string) VideoEpisode {
	cid := strings.TrimSpace(params["cid"])
	vid := strings.TrimSpace(params["vid"])
	return VideoEpisode{
		CID:           cid,
		VID:           vid,
		Title:         firstNonEmpty(params["title"], params["c_title_output"]),
		PlayTitle:     firstNonEmpty(params["play_title"], params["union_title"]),
		UnionTitle:    params["union_title"],
		Subtitle:      params["video_subtitle"],
		ImageURL:      params["image_url"],
		PublishDate:   params["publish_date"],
		EpisodeNumber: firstNonZero(atoi(params["title"]), atoi(params["c_title_output"])),
		Duration:      atoi(params["duration"]),
		IsTrailer:     params["is_trailer"] == "1",
		URL:           episodeURL(cid, vid),
	}
}

func walkPageCards(cards []pageCard, visit func(pageCard)) {
	for _, card := range cards {
		visit(card)
		keys := make([]string, 0, len(card.ChildrenList))
		for key := range card.ChildrenList {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			left, right := atoi(keys[i]), atoi(keys[j])
			if left == right {
				return keys[i] < keys[j]
			}
			return left < right
		})
		for _, key := range keys {
			walkPageCards(card.ChildrenList[key].Cards, visit)
		}
	}
}

func buildVQQPageAPIURL(guid string) string {
	values := url.Values{}
	values.Set("vdevice_guid", guid)
	values.Set("video_appid", "3000010")
	values.Set("vversion_name", "8.5.96")
	values.Set("vversion_platform", "2")
	return "https://pbaccess.video.qq.com/trpc.vector_layout.page_view.PageService/getPage?" + values.Encode()
}

func buildVQQPagePayload(pageURL VideoPageURL, guid string) map[string]any {
	return map[string]any{
		"page_params": map[string]string{
			"ad_wechat_authorization_status":   "0",
			"req_from":                         "web_vsite",
			"ad_exp_ids":                       "",
			"pc_sdk_version":                   "",
			"pc_oaid":                          "",
			"new_mark_label_enabled":           "1",
			"pc_device_info":                   "",
			"support_pc_yyb_mobile_app_engine": "0",
			"pc_wegame_version":                "",
			"cid":                              pageURL.CID,
			"history_vid":                      "",
			"vid":                              pageURL.VID,
			"is_pc_new_detail_page":            "0",
			"is_from_web_flyflow":              "1",
			"lid":                              "",
		},
		"page_bypass_params": map[string]any{
			"params": map[string]string{
				"caller_id":   "3000010",
				"platform_id": "2",
			},
			"scene":            "desk_detail",
			"app_version":      "",
			"abtest_bypass_id": guid,
		},
		"page_context": map[string]any{},
	}
}

func parsePinia(html string) (*piniaState, error) {
	match := piniaRE.FindStringSubmatch(html)
	if len(match) < 2 {
		return nil, fmt.Errorf("missing __PINIA__")
	}
	vm := goja.New()
	value, err := vm.RunString("(" + match[1] + ")")
	if err != nil {
		return nil, err
	}
	if err := vm.Set("__qq_pinia", value); err != nil {
		return nil, err
	}
	jsonValue, err := vm.RunString("JSON.stringify(__qq_pinia)")
	if err != nil {
		return nil, err
	}
	var state piniaState
	if err := json.Unmarshal([]byte(jsonValue.String()), &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (c *Client) getText(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	c.setHeaders(req)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return string(body), nil
}

func (c *Client) postJSON(ctx context.Context, rawURL string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return json.Unmarshal(respBody, out)
}

func (c *Client) postVQQPageJSON(ctx context.Context, rawURL string, body any, out any, guid string) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	c.setVQQPageHeaders(req, guid)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode qq getPage response: %w", err)
	}
	return nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Referer", "https://m.v.qq.com/")
}

func (c *Client) setVQQPageHeaders(req *http.Request, guid string) {
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://v.qq.com")
	req.Header.Set("Referer", "https://v.qq.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36")
	if guid != "" {
		req.AddCookie(&http.Cookie{Name: "video_guid", Value: guid})
		req.AddCookie(&http.Cookie{Name: "qq_domain_video_guid_verify", Value: guid})
		req.AddCookie(&http.Cookie{Name: "video_platform", Value: "2"})
	}
}

func filterEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
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

func atoi(value string) int {
	n := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			continue
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func trimHTMLSuffix(value string) string {
	value = strings.TrimSpace(value)
	return strings.TrimSuffix(value, ".html")
}

func episodeURL(cid string, vid string) string {
	if cid == "" || vid == "" {
		return ""
	}
	return "https://v.qq.com/x/cover/" + cid + "/" + vid + ".html"
}

func cleanRichText(value string) string {
	value = htmlpkg.UnescapeString(value)
	value = htmlTagRE.ReplaceAllString(value, "")
	return strings.Join(strings.Fields(value), " ")
}

func parseVideoScore(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var scoreInfo struct {
		VideoScore string `json:"video_score"`
	}
	if err := json.Unmarshal([]byte(value), &scoreInfo); err != nil {
		return ""
	}
	return strings.TrimSpace(scoreInfo.VideoScore)
}

func parseGenreList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var genres []string
	if err := json.Unmarshal([]byte(value), &genres); err == nil {
		return genres
	}
	return []string{value}
}

func appendUniqueStrings(values []string, additions ...string) []string {
	seen := make(map[string]bool, len(values)+len(additions))
	for _, value := range values {
		seen[value] = true
	}
	for _, addition := range additions {
		addition = strings.TrimSpace(addition)
		if addition == "" || seen[addition] {
			continue
		}
		values = append(values, addition)
		seen[addition] = true
	}
	return values
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func newVQQDeviceGUID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return hex.EncodeToString(buf[:])
	}
	return fmt.Sprintf("%016x", time.Now().UnixNano())
}
