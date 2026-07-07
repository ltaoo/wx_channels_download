package weibo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	BaseURL                 = "https://weibo.com"
	ContentTypeUserTimeline = "account"
	defaultUserAgent        = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
	defaultClientVersion    = "3.0.0"
)

var (
	ErrUnsupportedURL = errors.New("unsupported weibo user url")
	shareURLRE        = regexp.MustCompile(`https?://[^\s"'<>]+`)
	numericIDRE       = regexp.MustCompile(`^[0-9]+$`)
	htmlTagRE         = regexp.MustCompile(`<[^>]+>`)
	unsafeFilenameRE  = regexp.MustCompile(`[\\/:*?"<>|#\n\r]`)
	dotsRE            = regexp.MustCompile(`\.{2,}`)
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	HTTPClient HTTPClient
	BaseURL    string
	UserAgent  string
	Cookie     string
}

type UserURL struct {
	UID       string `json:"uid"`
	Canonical string `json:"canonical"`
}

type TimelineOptions struct {
	Page    int    `json:"page,omitempty"`
	Feature int    `json:"feature,omitempty"`
	Cookie  string `json:"-"`
}

type TimelinePage struct {
	URL         UserURL          `json:"url"`
	SourceURL   string           `json:"source_url"`
	APIURL      string           `json:"api_url"`
	Request     TimelineOptions  `json:"request"`
	Response    TimelineResponse `json:"response"`
	RawResponse json.RawMessage  `json:"raw_response,omitempty"`
	User        User             `json:"user,omitempty"`
	Posts       []PostSummary    `json:"posts,omitempty"`
	SinceID     string           `json:"since_id,omitempty"`
	Total       int              `json:"total,omitempty"`
}

type TimelineResponse struct {
	OK      int          `json:"ok,omitempty"`
	Message string       `json:"msg,omitempty"`
	Data    TimelineData `json:"data"`
}

type TimelineData struct {
	SinceID string   `json:"since_id,omitempty"`
	List    []Status `json:"list,omitempty"`
	Total   int      `json:"total,omitempty"`
}

type Status struct {
	CreatedAt        string         `json:"created_at,omitempty"`
	ID               int64          `json:"id,omitempty"`
	IDStr            string         `json:"idstr,omitempty"`
	MID              string         `json:"mid,omitempty"`
	MblogID          string         `json:"mblogid,omitempty"`
	User             User           `json:"user,omitempty"`
	Text             string         `json:"text,omitempty"`
	TextRaw          string         `json:"text_raw,omitempty"`
	Source           string         `json:"source,omitempty"`
	PicIDs           []string       `json:"pic_ids,omitempty"`
	PicNum           int            `json:"pic_num,omitempty"`
	PicInfos         map[string]Pic `json:"pic_infos,omitempty"`
	PageInfo         map[string]any `json:"page_info,omitempty"`
	RetweetedStatus  *Status        `json:"retweeted_status,omitempty"`
	RepostsCount     int            `json:"reposts_count,omitempty"`
	CommentsCount    int            `json:"comments_count,omitempty"`
	AttitudesCount   int            `json:"attitudes_count,omitempty"`
	RegionName       string         `json:"region_name,omitempty"`
	IsLongText       bool           `json:"isLongText,omitempty"`
	RegionNameSnake  string         `json:"regionName,omitempty"`
	CreatedTimestamp int64          `json:"created_timestamp,omitempty"`
}

type User struct {
	ID              int64  `json:"id,omitempty"`
	IDStr           string `json:"idstr,omitempty"`
	ScreenName      string `json:"screen_name,omitempty"`
	ProfileImageURL string `json:"profile_image_url,omitempty"`
	ProfileURL      string `json:"profile_url,omitempty"`
	Verified        bool   `json:"verified,omitempty"`
	VerifiedType    int    `json:"verified_type,omitempty"`
	AvatarLarge     string `json:"avatar_large,omitempty"`
	AvatarHD        string `json:"avatar_hd,omitempty"`
	Description     string `json:"description,omitempty"`
	Location        string `json:"location,omitempty"`
	Gender          string `json:"gender,omitempty"`
	FollowersCount  int    `json:"followers_count,omitempty"`
	FriendsCount    int    `json:"friends_count,omitempty"`
	StatusesCount   int    `json:"statuses_count,omitempty"`
}

type Pic struct {
	Thumbnail PicVariant `json:"thumbnail,omitempty"`
	Bmiddle   PicVariant `json:"bmiddle,omitempty"`
	Large     PicVariant `json:"large,omitempty"`
	Original  PicVariant `json:"original,omitempty"`
	Largest   PicVariant `json:"largest,omitempty"`
	MW2000    PicVariant `json:"mw2000,omitempty"`
	ObjectID  string     `json:"object_id,omitempty"`
	PicID     string     `json:"pic_id,omitempty"`
	Type      string     `json:"type,omitempty"`
	PicStatus int        `json:"pic_status,omitempty"`
}

type PicVariant struct {
	URL     string `json:"url,omitempty"`
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
	CutType int    `json:"cut_type,omitempty"`
	Type    string `json:"type,omitempty"`
}

type UserSummary struct {
	ID         string `json:"id,omitempty"`
	ScreenName string `json:"screen_name,omitempty"`
	ProfileURL string `json:"profile_url,omitempty"`
	AvatarURL  string `json:"avatar_url,omitempty"`
	Verified   bool   `json:"verified,omitempty"`
}

type PostSummary struct {
	ID             string      `json:"id,omitempty"`
	MID            string      `json:"mid,omitempty"`
	MblogID        string      `json:"mblogid,omitempty"`
	URL            string      `json:"url,omitempty"`
	Text           string      `json:"text,omitempty"`
	CreatedAt      string      `json:"created_at,omitempty"`
	CreatedTime    int64       `json:"created_time,omitempty"`
	Source         string      `json:"source,omitempty"`
	RegionName     string      `json:"region_name,omitempty"`
	Author         UserSummary `json:"author,omitempty"`
	PicURLs        []string    `json:"pic_urls,omitempty"`
	CoverURL       string      `json:"cover_url,omitempty"`
	PicNum         int         `json:"pic_num,omitempty"`
	RepostsCount   int         `json:"repost_count,omitempty"`
	CommentsCount  int         `json:"comment_count,omitempty"`
	AttitudesCount int         `json:"like_count,omitempty"`
}

func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		BaseURL:    BaseURL,
		UserAgent:  defaultUserAgent,
	}
}

func NewClientWithOptions(client HTTPClient, cookie, userAgent string) *Client {
	c := NewClient()
	if client != nil {
		c.HTTPClient = client
	}
	if strings.TrimSpace(cookie) != "" {
		c.Cookie = strings.TrimSpace(cookie)
	}
	if strings.TrimSpace(userAgent) != "" {
		c.UserAgent = strings.TrimSpace(userAgent)
	}
	return c
}

func CanParse(rawURL string) bool {
	_, ok := ParseUserURL(rawURL)
	return ok
}

func ExtractShareURL(text string) string {
	text = strings.TrimSpace(text)
	for _, match := range shareURLRE.FindAllString(text, -1) {
		match = strings.Trim(match, " \t\r\n，。；;、.,!?！？")
		if CanParse(match) {
			return match
		}
	}
	if CanParse(text) {
		return text
	}
	return ""
}

func ParseUserURL(rawURL string) (UserURL, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return UserURL{}, false
	}
	if numericIDRE.MatchString(rawURL) {
		return UserURL{UID: rawURL, Canonical: CanonicalUserURL(rawURL)}, true
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return UserURL{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "weibo.com" && host != "www.weibo.com" && host != "m.weibo.cn" && !strings.HasSuffix(host, ".weibo.com") {
		return UserURL{}, false
	}
	if uid := strings.TrimSpace(parsed.Query().Get("uid")); numericIDRE.MatchString(uid) {
		return UserURL{UID: uid, Canonical: CanonicalUserURL(uid)}, true
	}
	segments := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		return UserURL{}, false
	}
	if numericIDRE.MatchString(segments[0]) {
		return UserURL{UID: segments[0], Canonical: CanonicalUserURL(segments[0])}, true
	}
	if len(segments) >= 2 && (segments[0] == "u" || segments[0] == "profile") {
		uid, err := url.PathUnescape(segments[1])
		if err == nil && numericIDRE.MatchString(uid) {
			return UserURL{UID: uid, Canonical: CanonicalUserURL(uid)}, true
		}
	}
	return UserURL{}, false
}

func CanonicalUserURL(uid string) string {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return ""
	}
	return BaseURL + "/u/" + url.PathEscape(uid)
}

func FetchUserTimeline(ctx context.Context, rawURL string, opts TimelineOptions) (*TimelinePage, error) {
	return NewClient().FetchUserTimeline(ctx, rawURL, opts)
}

func (c *Client) FetchUserTimeline(ctx context.Context, rawURL string, opts TimelineOptions) (*TimelinePage, error) {
	userURL, ok := ParseUserURL(rawURL)
	if !ok {
		return nil, ErrUnsupportedURL
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.Feature < 0 {
		opts.Feature = 0
	}
	apiURL, err := c.mymblogURL(userURL.UID, opts.Page, opts.Feature)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	c.setMymblogHeaders(req, userURL.Canonical, opts.Cookie)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch weibo mymblog: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch weibo mymblog: HTTP %d: %s", resp.StatusCode, trimBody(body))
	}
	var decoded TimelineResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decode weibo mymblog: %w", err)
	}
	if decoded.OK == 0 && len(decoded.Data.List) == 0 {
		message := strings.TrimSpace(decoded.Message)
		if message == "" {
			message = "empty response"
		}
		return nil, fmt.Errorf("fetch weibo mymblog: %s", message)
	}
	if len(decoded.Data.List) == 0 {
		message := strings.TrimSpace(decoded.Message)
		if message == "" {
			message = "empty timeline list; weibo cookie may be required"
		}
		return nil, fmt.Errorf("fetch weibo mymblog: %s", message)
	}
	page := &TimelinePage{
		URL:         userURL,
		SourceURL:   rawURL,
		APIURL:      apiURL,
		Request:     TimelineOptions{Page: opts.Page, Feature: opts.Feature},
		Response:    decoded,
		RawResponse: json.RawMessage(append([]byte(nil), body...)),
		SinceID:     decoded.Data.SinceID,
		Total:       decoded.Data.Total,
	}
	page.User = firstTimelineUser(decoded.Data.List, userURL.UID)
	page.Posts = SummarizeStatuses(decoded.Data.List)
	return page, nil
}

func (c *Client) mymblogURL(uid string, page int, feature int) (string, error) {
	base := strings.TrimRight(firstNonEmpty(c.BaseURL, BaseURL), "/")
	parsed, err := url.Parse(base + "/ajax/statuses/mymblog")
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("uid", uid)
	query.Set("page", strconv.Itoa(page))
	query.Set("feature", strconv.Itoa(feature))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *Client) setMymblogHeaders(req *http.Request, referer string, optionCookie string) {
	ua := firstNonEmpty(c.UserAgent, defaultUserAgent)
	cookie := firstNonEmpty(optionCookie, c.Cookie)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Client-Version", defaultClientVersion)
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", referer)
	req.Header.Set("Sec-Ch-Ua", `"Google Chrome";v="149", "Chromium";v="149", "Not)A;Brand";v="24"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", ua)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
		if xsrf := cookieValue(cookie, "XSRF-TOKEN"); xsrf != "" {
			req.Header.Set("X-XSRF-TOKEN", xsrf)
		}
	}
}

func (c *Client) httpClient() HTTPClient {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return NewClient().HTTPClient
}

func firstTimelineUser(statuses []Status, fallbackUID string) User {
	for _, status := range statuses {
		if status.User.IDStr != "" || status.User.ID > 0 || status.User.ScreenName != "" {
			user := status.User
			if user.IDStr == "" && user.ID > 0 {
				user.IDStr = strconv.FormatInt(user.ID, 10)
			}
			return user
		}
	}
	return User{IDStr: fallbackUID}
}

func SummarizeStatuses(statuses []Status) []PostSummary {
	if len(statuses) == 0 {
		return nil
	}
	out := make([]PostSummary, 0, len(statuses))
	for _, status := range statuses {
		out = append(out, SummarizeStatus(status))
	}
	return out
}

func SummarizeStatus(status Status) PostSummary {
	id := firstNonEmpty(status.IDStr, status.MID)
	if id == "" && status.ID > 0 {
		id = strconv.FormatInt(status.ID, 10)
	}
	createdTime := parseWeiboTime(status.CreatedAt)
	picURLs := status.PicURLs()
	return PostSummary{
		ID:             id,
		MID:            firstNonEmpty(status.MID, id),
		MblogID:        status.MblogID,
		URL:            StatusURL(status.User.IDStr, status.MblogID, id),
		Text:           status.PlainText(),
		CreatedAt:      status.CreatedAt,
		CreatedTime:    createdTime,
		Source:         status.Source,
		RegionName:     firstNonEmpty(status.RegionName, status.RegionNameSnake),
		Author:         status.User.Summary(),
		PicURLs:        picURLs,
		CoverURL:       firstString(picURLs),
		PicNum:         firstPositive(status.PicNum, len(picURLs)),
		RepostsCount:   status.RepostsCount,
		CommentsCount:  status.CommentsCount,
		AttitudesCount: status.AttitudesCount,
	}
}

func (s Status) PlainText() string {
	text := firstNonEmpty(s.TextRaw, s.Text)
	text = htmlTagRE.ReplaceAllString(text, "")
	text = stdhtml.UnescapeString(text)
	return strings.TrimSpace(text)
}

func (s Status) PicURLs() []string {
	if len(s.PicInfos) == 0 {
		return nil
	}
	ids := s.PicIDs
	if len(ids) == 0 {
		ids = make([]string, 0, len(s.PicInfos))
		for id := range s.PicInfos {
			ids = append(ids, id)
		}
	}
	out := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		pic, ok := s.PicInfos[id]
		if !ok {
			continue
		}
		rawURL := pic.BestURL()
		if rawURL == "" || seen[rawURL] {
			continue
		}
		seen[rawURL] = true
		out = append(out, rawURL)
	}
	return out
}

func (p Pic) BestURL() string {
	return firstNonEmpty(
		p.Original.URL,
		p.Largest.URL,
		p.MW2000.URL,
		p.Large.URL,
		p.Bmiddle.URL,
		p.Thumbnail.URL,
	)
}

func (u User) Summary() UserSummary {
	id := u.IDStr
	if id == "" && u.ID > 0 {
		id = strconv.FormatInt(u.ID, 10)
	}
	profileURL := NormalizeWeiboURL(u.ProfileURL)
	if profileURL == "" && id != "" {
		profileURL = CanonicalUserURL(id)
	}
	return UserSummary{
		ID:         id,
		ScreenName: u.ScreenName,
		ProfileURL: profileURL,
		AvatarURL:  firstNonEmpty(u.AvatarHD, u.AvatarLarge, u.ProfileImageURL),
		Verified:   u.Verified,
	}
}

func NormalizeWeiboURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if strings.HasPrefix(rawURL, "//") {
		return "https:" + rawURL
	}
	if strings.HasPrefix(rawURL, "/") {
		return BaseURL + rawURL
	}
	return rawURL
}

func StatusURL(uid string, mblogID string, id string) string {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return ""
	}
	if strings.TrimSpace(mblogID) != "" {
		return BaseURL + "/" + url.PathEscape(uid) + "/" + url.PathEscape(mblogID)
	}
	if strings.TrimSpace(id) != "" {
		return BaseURL + "/" + url.PathEscape(uid) + "/" + url.PathEscape(id)
	}
	return ""
}

func SanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = unsafeFilenameRE.ReplaceAllString(name, "_")
	name = dotsRE.ReplaceAllString(name, ".")
	name = strings.Trim(name, " ._")
	if name == "" {
		return "weibo"
	}
	if len([]rune(name)) > 120 {
		runes := []rune(name)
		name = string(runes[:120])
	}
	return name
}

func parseWeiboTime(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if t, err := time.Parse("Mon Jan 02 15:04:05 -0700 2006", value); err == nil {
		return t.Unix()
	}
	return 0
}

func cookieValue(cookieHeader string, name string) string {
	req := http.Request{Header: http.Header{"Cookie": []string{cookieHeader}}}
	for _, cookie := range req.Cookies() {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func trimBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len([]rune(text)) > 300 {
		return string([]rune(text)[:300])
	}
	return text
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

func firstString(values []string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
