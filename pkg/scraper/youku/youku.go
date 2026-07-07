package youku

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const appKey = "24679788"

var (
	initialDataRE  = regexp.MustCompile(`window\.__INITIAL_DATA__\s*=\s*([^;]+);`)
	hrefRE         = regexp.MustCompile(`href="([^"]+)" data-spm="dplaybutton"`)
	titleRE        = regexp.MustCompile(`<div class="show-name">([^<]+)</div>`)
	posterRE       = regexp.MustCompile(`background-image:url\(([^)]+)\)`)
	infoRE         = regexp.MustCompile(`<div class="show-feature">([^<]+)</div>`)
	date8RE        = regexp.MustCompile(`^([0-9]{4})([0-9]{2})([0-9]{2})$`)
	countryMap     = map[string]string{"大陆": "CN", "普通话": "CN", "内地": "CN", "中国": "CN", "韩国": "KR", "美国": "US"}
	mediaTypeMap   = map[string]string{"电影": "movie", "电视剧": "tv", "综艺": "tv", "纪录片": "tv"}
	mediaGenreMap  = map[string]string{"真人秀": "真人秀", "综艺": "真人秀", "剧情": "剧情", "犯罪": "犯罪", "动作": "动作"}
	mediaSourceMap = map[string]string{"优酷": "youku", "youku": "youku", "腾讯": "qq", "爱奇艺": "iqiyi", "mgtv": "mgtv", "imgo": "mgtv"}
)

type Client struct {
	HTTPClient *http.Client
	UserAgent  string
	Cookie     string
	Token      string
	Expired    time.Time
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
	jar, _ := cookiejar.New(nil)
	c := &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second, Jar: jar},
		UserAgent:  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 30 * time.Second, Jar: jar}
	}
	return c
}

func Sign(token string, timestamp int64, data string) string {
	sum := md5.Sum([]byte(fmt.Sprintf("%s&%d&%s&%s", token, timestamp, appKey, data)))
	return hex.EncodeToString(sum[:])
}

func (c *Client) FetchToken(ctx context.Context) (*Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, "https://acs.youku.com/h5/mtop.ykrec.recommendservice.recommend/1.0/", nil)
	if err != nil {
		return nil, err
	}
	query := req.URL.Query()
	query.Set("jsv", "2.6.1")
	query.Set("appKey", appKey)
	query.Set("t", "1701224407059")
	query.Set("sign", "0319217691a8eae04657abd83140bb11")
	query.Set("api", "mtop.ykrec.RecommendService.recommend")
	query.Set("v", "1.0")
	query.Set("dataType", "json")
	query.Set("type", "originaljson")
	query.Set("data", `{"appid":"14177","mtopParams":"{\"count\":\"1\",\"channel\":\"PC\",\"fr\":\"pc\",\"app_source\":\"main_page\",\"x_utdid\":\"XlQcF5xQrCcCAWoLKdGqIOhS\"}","utdid":"XlQcF5xQrCcCAWoLKdGqIOhS"}`)
	req.URL.RawQuery = query.Encode()
	c.setHeaders(req, "https://www.youku.com/")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var mToken, mTokenEnc string
	var expired time.Time
	for _, cookie := range resp.Cookies() {
		switch cookie.Name {
		case "_m_h5_tk":
			mToken = cookie.Value
			expired = cookie.Expires
		case "_m_h5_tk_enc":
			mTokenEnc = cookie.Value
		}
	}
	if mToken == "" {
		return nil, fmt.Errorf("missing _m_h5_tk cookie")
	}
	token := strings.SplitN(mToken, "_", 2)[0]
	c.Cookie = fmt.Sprintf("isI18n=false; _m_h5_tk=%s; _m_h5_tk_enc=%s", mToken, mTokenEnc)
	c.Token = token
	c.Expired = expired
	return &Token{Cookie: c.Cookie, Token: token, Expired: expired}, nil
}

func (c *Client) Initialize(ctx context.Context) error {
	if c.Token == "" || c.Expired.IsZero() || time.Now().After(c.Expired) {
		_, err := c.FetchToken(ctx)
		return err
	}
	return nil
}

func (c *Client) FetchProfilePage(ctx context.Context, rawURL string) (*ProfileData, error) {
	html, err := c.getText(ctx, rawURL, nil, rawURL)
	if err != nil {
		return nil, err
	}
	return ParseProfilePage(html)
}

func ParseProfilePage(html string) (*ProfileData, error) {
	initialData, err := extractInitialData(html)
	if err != nil {
		return nil, fmt.Errorf("missing __INITIAL_DATA__")
	}
	var info PageInfo
	if err := json.Unmarshal([]byte(initialData), &info); err != nil {
		return nil, err
	}
	profile := info.ProfileData()
	return &profile, nil
}

func (c *Client) FetchProfileWithSeasons(ctx context.Context, rawURL string) (*ProfileWithSeasons, error) {
	profile, err := c.FetchProfilePage(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	result, err := FormatSeasonProfile(*profile)
	if err != nil {
		return nil, err
	}
	result.Platform = "youku"
	return result, nil
}

func FormatSeasonProfile(profile ProfileData) (*ProfileWithSeasons, error) {
	baseNode := findNode(profile.Nodes, 10001)
	if baseNode == nil {
		return nil, fmt.Errorf("missing base node")
	}
	extra := profile.Data.Extra
	payload := Season{
		ID:            extra.ShowID,
		Name:          extra.ShowName,
		Overview:      "",
		PosterPath:    extra.ShowImgV,
		AirDate:       formatDateString(extra.ShowReleaseTime),
		Genres:        filterEmpty([]string{mediaGenreMap[extra.ShowCategory]}),
		OriginCountry: []string{},
		Persons:       []Person{},
	}
	if intro := introNode(baseNode); intro != nil {
		payload.Overview = intro.Desc
		parts := strings.Split(intro.IntroSubTitle, "·")
		if len(parts) > 0 {
			payload.OriginCountry = filterEmpty([]string{countryMap[strings.TrimSpace(parts[0])]})
		}
	}
	persons := personNodes(baseNode)
	episodes := episodeNodes(baseNode)
	seasons := seasonNodes(baseNode)
	typeMap := map[string]string{"电影": "movie", "综艺": "season"}
	if len(seasons) == 0 {
		seasons = []Season{{
			ID:            payload.ID,
			Name:          payload.Name,
			Overview:      payload.Overview,
			PosterPath:    payload.PosterPath,
			AirDate:       payload.AirDate,
			Episodes:      episodes,
			Genres:        payload.Genres,
			OriginCountry: payload.OriginCountry,
			Persons:       persons,
		}}
	} else {
		for i := range seasons {
			if seasons[i].ID == payload.ID {
				seasons[i].Name = payload.Name
				seasons[i].Overview = payload.Overview
				seasons[i].PosterPath = payload.PosterPath
				seasons[i].AirDate = payload.AirDate
				seasons[i].Episodes = episodes
				seasons[i].Persons = persons
				seasons[i].Genres = payload.Genres
				seasons[i].OriginCountry = payload.OriginCountry
			}
		}
	}
	return &ProfileWithSeasons{
		Type:         firstNonEmpty(typeMap[extra.VideoCategory], "season"),
		ID:           payload.ID,
		Name:         payload.Name,
		Overview:     payload.Overview,
		PosterPath:   payload.PosterPath,
		BackdropPath: "",
		OriginalName: "",
		Seasons:      seasons,
	}, nil
}

func (c *Client) Search(ctx context.Context, keyword string) (*SearchResult, error) {
	html, err := c.getText(ctx, "https://search.youku.com/search_video", map[string]string{"keyword": keyword}, "https://www.youku.com/")
	if err != nil {
		return nil, err
	}
	return ParseSearchHTML(html)
}

func ParseSearchHTML(html string) (*SearchResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	var list []SearchItem
	doc.Find(".h5-show-card-wrapper").Each(func(_ int, card *goquery.Selection) {
		line, err := card.Html()
		if err != nil || line == "" {
			return
		}
		info := regexpGroup(infoRE, line)
		if info == "" {
			return
		}
		segments := splitAndTrim(info, "·")
		if len(segments) < 2 {
			return
		}
		airDate := segments[0]
		tag := segments[1]
		originCountry := ""
		if len(segments) > 2 {
			originCountry = countryMap[segments[2]]
		}
		sourceText := strings.TrimSpace(card.Find(".show-sourcename").Text())
		list = append(list, SearchItem{
			ID:            regexpGroup(hrefRE, line),
			Name:          regexpGroup(titleRE, line),
			OriginalName:  "",
			Overview:      "",
			PosterPath:    regexpGroup(posterRE, line),
			BackdropPath:  "",
			FirstAirDate:  airDate,
			OriginCountry: filterEmpty([]string{originCountry}),
			Type:          mediaTypeMap[tag],
			Source:        firstNonEmpty(mediaSourceMap[sourceText], "youku"),
		})
	})
	return &SearchResult{List: list}, nil
}

func (c *Client) FetchEpisodeProfileAPI(ctx context.Context, episodeID string, seasonID any) (*ProfileData, error) {
	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}
	data := map[string]any{
		"ms_codes": "2019030100",
		"params": mustJSONString(map[string]any{
			"biz":              "new_detail_web2",
			"componentVersion": "3",
			"debug":            0,
			"gray":             0,
			"ip":               "183.129.167.42",
			"platform":         "pc",
			"scene":            "web_page",
			"showId":           seasonID,
			"source":           "pcNoPrev",
			"userId":           0,
			"utdid":            "TyNmHYeaUmcCAXPuK4Jgz6e+",
			"videoId":          episodeID,
		}),
		"system_info": mustJSONString(map[string]any{
			"os":            "pc",
			"device":        "pc",
			"ver":           "1.0.0",
			"appPackageKey": "pcweb",
			"appPackageId":  "pcweb",
		}),
	}
	return c.fetchMTopProfile(ctx, data)
}

func (c *Client) fetchMTopProfile(ctx context.Context, data map[string]any) (*ProfileData, error) {
	dataText := mustJSONString(data)
	timestamp := time.Now().UnixMilli()
	query := map[string]string{
		"jsv":      "2.6.1",
		"appKey":   appKey,
		"t":        strconv.FormatInt(timestamp, 10),
		"sign":     Sign(c.Token, timestamp, dataText),
		"api":      "mtop.youku.columbus.gateway.new.execute",
		"type":     "originaljson",
		"v":        "1.0",
		"ecode":    "1",
		"dataType": "json",
		"data":     dataText,
	}
	var resp mtopResponse
	if err := c.getJSON(ctx, "https://acs.youku.com/h5/mtop.youku.columbus.gateway.new.execute/1.0/", query, &resp, "https://www.youku.com/"); err != nil {
		return nil, err
	}
	holder, ok := resp.Data["2019030100"]
	if !ok {
		return nil, fmt.Errorf(strings.Join(resp.Ret, "; "))
	}
	return &holder.Data, nil
}

func (c *Client) getText(ctx context.Context, rawURL string, query map[string]string, referer string) (string, error) {
	body, err := c.get(ctx, rawURL, query, referer)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Client) getJSON(ctx context.Context, rawURL string, query map[string]string, out any, referer string) error {
	body, err := c.get(ctx, rawURL, query, referer)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

func (c *Client) get(ctx context.Context, rawURL string, query map[string]string, referer string) ([]byte, error) {
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
	c.setHeaders(req, referer)
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

func (c *Client) setHeaders(req *http.Request, referer string) {
	req.Header.Set("Accept", "application/json,text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("User-Agent", c.UserAgent)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	if c.Cookie != "" {
		req.Header.Set("Cookie", c.Cookie)
	}
}

func extractInitialData(html string) (string, error) {
	const marker = "window.__INITIAL_DATA__"
	index := strings.Index(html, marker)
	if index < 0 {
		match := initialDataRE.FindStringSubmatch(html)
		if len(match) < 2 {
			return "", fmt.Errorf("missing __INITIAL_DATA__")
		}
		return strings.TrimSpace(match[1]), nil
	}
	valueStart := strings.Index(html[index:], "=")
	if valueStart < 0 {
		return "", fmt.Errorf("missing __INITIAL_DATA__ assignment")
	}
	pos := index + valueStart + 1
	for pos < len(html) && (html[pos] == ' ' || html[pos] == '\n' || html[pos] == '\r' || html[pos] == '\t') {
		pos++
	}
	if pos >= len(html) || html[pos] != '{' {
		match := initialDataRE.FindStringSubmatch(html[index:])
		if len(match) < 2 {
			return "", fmt.Errorf("missing __INITIAL_DATA__ object")
		}
		return strings.TrimSpace(match[1]), nil
	}
	depth := 0
	inString := false
	escaped := false
	for i := pos; i < len(html); i++ {
		ch := html[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return html[pos : i+1], nil
			}
		}
	}
	return "", fmt.Errorf("unterminated __INITIAL_DATA__ object")
}

func findNode(nodes []Node, typ int) *Node {
	for i := range nodes {
		if nodes[i].Type == typ {
			return &nodes[i]
		}
	}
	return nil
}

func introNode(base *Node) *NodeData {
	a := findNode(base.Nodes, 20009)
	if a == nil {
		return nil
	}
	b := findNode(a.Nodes, 20010)
	if b == nil {
		return nil
	}
	return &b.Data
}

func personNodes(base *Node) []Person {
	personType := map[string]string{"主持人": "host", "嘉宾": "guest", "导演": "director", "主演": "main_charetors", "演员": "actor"}
	a := findNode(base.Nodes, 20009)
	if a == nil {
		return nil
	}
	var out []Person
	for _, node := range a.Nodes {
		if node.Type != 10011 {
			continue
		}
		out = append(out, Person{
			ID:         firstNonEmpty(personIDString(node.Data.PersonID), nodeIDString(node.ID)),
			Name:       node.Data.Title,
			Avatar:     node.Data.Img,
			Character:  []string{},
			Department: personType[node.Data.Subtitle],
		})
	}
	return out
}

func episodeNodes(base *Node) []Episode {
	a := findNode(base.Nodes, 10013)
	if a == nil {
		return nil
	}
	out := make([]Episode, 0, len(a.Nodes))
	for _, node := range a.Nodes {
		if node.Data.VideoType != "" && node.Data.VideoType != "正片" {
			continue
		}
		out = append(out, Episode{
			ID:            firstNonEmpty(actionValue(node.Data.Action), node.Data.ActionValue, nodeIDString(node.ID)),
			Name:          node.Data.Title,
			Thumbnail:     node.Data.Img,
			EpisodeNumber: firstPositiveInt(node.Data.Rank, len(out)+1),
			AirDate:       formatEpisodeAirDate(node.Data.Stage),
		})
	}
	return out
}

func seasonNodes(base *Node) []Season {
	a := findNode(base.Nodes, 10013)
	if a == nil {
		return nil
	}
	out := make([]Season, 0, len(a.Data.Series))
	for _, item := range a.Data.Series {
		out = append(out, Season{
			ID:            item.ShowID,
			Name:          item.Title,
			Overview:      "",
			PosterPath:    "",
			AirDate:       "",
			Genres:        []string{},
			OriginCountry: []string{},
			Persons:       []Person{},
		})
	}
	return out
}

func actionValue(action *Action) string {
	if action == nil {
		return ""
	}
	return action.Value
}

func nodeIDString(id int64) string {
	if id == 0 {
		return ""
	}
	return strconv.FormatInt(id, 10)
}

func personIDString(value any) string {
	if value == nil {
		return ""
	}
	if number, ok := value.(float64); ok && number == float64(int64(number)) {
		return strconv.FormatInt(int64(number), 10)
	}
	return fmt.Sprint(value)
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func formatEpisodeAirDate(value any) string {
	if value == nil {
		return ""
	}
	text := valueString(value)
	if date8RE.MatchString(text) || len(text) >= 10 && text[4] == '-' {
		return formatDateString(text)
	}
	return ""
}

func valueString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func formatDateString(value string) string {
	value = strings.TrimSpace(value)
	if match := date8RE.FindStringSubmatch(value); len(match) == 4 {
		return match[1] + "-" + match[2] + "-" + match[3]
	}
	if len(value) >= 10 && value[4] == '-' {
		return value[:10]
	}
	return value
}

func filterEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && value != "<nil>" {
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

func regexpGroup(re *regexp.Regexp, text string) string {
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func mustJSONString(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
