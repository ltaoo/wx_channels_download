package douyin

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
)

var (
	errUnsupportedProfileURL = errors.New("不支持的抖音作者主页 URL")
	secUserIDRE              = regexp.MustCompile(`^[A-Za-z0-9._-]{20,}$`)
)

type ProfileURL struct {
	SecUserID string `json:"sec_user_id"`
	Canonical string `json:"canonical"`
}

type ProfileOptions struct {
	Count     int               `json:"count,omitempty"`
	MaxCursor int64             `json:"max_cursor,omitempty"`
	Cookie    string            `json:"-"`
	UserAgent string            `json:"user_agent,omitempty"`
	APIURL    string            `json:"api_url,omitempty"`
	PageHTML  string            `json:"-"`
	SkipPage  bool              `json:"skip_page,omitempty"`
	Extra     map[string]string `json:"extra,omitempty"`
}

type ProfilePage struct {
	URL         ProfileURL        `json:"url"`
	SourceURL   string            `json:"source_url,omitempty"`
	PageURL     string            `json:"page_url,omitempty"`
	APIURL      string            `json:"api_url,omitempty"`
	Request     ProfileOptions    `json:"request"`
	User        UserProfile       `json:"user"`
	Response    AwemePostResponse `json:"response"`
	RawResponse json.RawMessage   `json:"raw_response,omitempty"`
	Posts       []AwemeSummary    `json:"posts,omitempty"`
	HasMore     bool              `json:"has_more,omitempty"`
	MinCursor   int64             `json:"min_cursor,omitempty"`
	MaxCursor   int64             `json:"max_cursor,omitempty"`
	Warnings    []string          `json:"warnings,omitempty"`
	PageHTML    string            `json:"-"`
}

type UserProfile struct {
	UID                      string `json:"uid,omitempty"`
	SecUID                   string `json:"sec_uid,omitempty"`
	ShortID                  string `json:"short_id,omitempty"`
	UniqueID                 string `json:"unique_id,omitempty"`
	RealName                 string `json:"real_name,omitempty"`
	Nickname                 string `json:"nickname,omitempty"`
	Signature                string `json:"signature,omitempty"`
	AvatarURL                string `json:"avatar_url,omitempty"`
	Avatar300URL             string `json:"avatar_300_url,omitempty"`
	ShareURL                 string `json:"share_url,omitempty"`
	CoverURL                 string `json:"cover_url,omitempty"`
	FollowerCount            int    `json:"follower_count,omitempty"`
	FollowingCount           int    `json:"following_count,omitempty"`
	MPlatformFollowersCount  int    `json:"mplatform_followers_count,omitempty"`
	AwemeCount               int    `json:"aweme_count,omitempty"`
	FavoritingCount          int    `json:"favoriting_count,omitempty"`
	TotalFavorited           int    `json:"total_favorited,omitempty"`
	Country                  string `json:"country,omitempty"`
	Province                 string `json:"province,omitempty"`
	City                     string `json:"city,omitempty"`
	IPLocation               string `json:"ip_location,omitempty"`
	EnterpriseVerifyReason   string `json:"enterprise_verify_reason,omitempty"`
	CustomVerify             string `json:"custom_verify,omitempty"`
	Secret                   int    `json:"secret,omitempty"`
	RoomID                   string `json:"room_id,omitempty"`
	ContinuationState        int    `json:"continuation_state,omitempty"`
	NeedSpecialFollowerCount bool   `json:"need_special_follower_count,omitempty"`
}

type AwemePostResponse struct {
	StatusCode int     `json:"status_code"`
	StatusMsg  string  `json:"status_msg,omitempty"`
	MinCursor  int64   `json:"min_cursor,omitempty"`
	MaxCursor  int64   `json:"max_cursor,omitempty"`
	HasMore    int     `json:"has_more,omitempty"`
	AwemeList  []Aweme `json:"aweme_list,omitempty"`
	LogPB      any     `json:"log_pb,omitempty"`
	Extra      any     `json:"extra,omitempty"`
}

type Aweme struct {
	AwemeID        string          `json:"aweme_id,omitempty"`
	Desc           string          `json:"desc,omitempty"`
	CreateTime     int64           `json:"create_time,omitempty"`
	Author         AwemeAuthor     `json:"author,omitempty"`
	Music          AwemeMusic      `json:"music,omitempty"`
	Video          AwemeVideo      `json:"video,omitempty"`
	Images         []AwemeImage    `json:"images,omitempty"`
	OriginalImages []AwemeImage    `json:"original_images,omitempty"`
	Statistics     AwemeStatistics `json:"statistics,omitempty"`
	ShareInfo      AwemeShareInfo  `json:"share_info,omitempty"`
	AwemeType      int             `json:"aweme_type,omitempty"`
	Duration       int             `json:"duration,omitempty"`
	Region         string          `json:"region,omitempty"`
	IPAttribution  string          `json:"ip_attribution,omitempty"`
	IsTop          int             `json:"is_top,omitempty"`
}

type AwemeAuthor struct {
	UID                     string        `json:"uid,omitempty"`
	SecUID                  string        `json:"sec_uid,omitempty"`
	ShortID                 string        `json:"short_id,omitempty"`
	UniqueID                string        `json:"unique_id,omitempty"`
	Nickname                string        `json:"nickname,omitempty"`
	Signature               string        `json:"signature,omitempty"`
	AvatarThumb             URLEnvelope   `json:"avatar_thumb,omitempty"`
	AvatarMedium            URLEnvelope   `json:"avatar_medium,omitempty"`
	AvatarLarge             URLEnvelope   `json:"avatar_large,omitempty"`
	CoverURL                []URLEnvelope `json:"cover_url,omitempty"`
	FollowerCount           int           `json:"follower_count,omitempty"`
	FollowingCount          int           `json:"following_count,omitempty"`
	AwemeCount              int           `json:"aweme_count,omitempty"`
	FavoritingCount         int           `json:"favoriting_count,omitempty"`
	TotalFavorited          int           `json:"total_favorited,omitempty"`
	EnterpriseVerifyReason  string        `json:"enterprise_verify_reason,omitempty"`
	CustomVerify            string        `json:"custom_verify,omitempty"`
	Country                 string        `json:"country,omitempty"`
	Province                string        `json:"province,omitempty"`
	City                    string        `json:"city,omitempty"`
	IPLocation              string        `json:"ip_location,omitempty"`
	MPlatformFollowersCount int           `json:"mplatform_followers_count,omitempty"`
}

type AwemeMusic struct {
	IDStr    string      `json:"id_str,omitempty"`
	Title    string      `json:"title,omitempty"`
	Author   string      `json:"author,omitempty"`
	Duration int         `json:"duration,omitempty"`
	PlayURL  URLEnvelope `json:"play_url,omitempty"`
	Cover    URLEnvelope `json:"cover_large,omitempty"`
}

type AwemeVideo struct {
	PlayAddr      URLEnvelope    `json:"play_addr,omitempty"`
	DownloadAddr  URLEnvelope    `json:"download_addr,omitempty"`
	Cover         URLEnvelope    `json:"cover,omitempty"`
	DynamicCover  URLEnvelope    `json:"dynamic_cover,omitempty"`
	OriginCover   URLEnvelope    `json:"origin_cover,omitempty"`
	BitRate       []AwemeBitRate `json:"bit_rate,omitempty"`
	Duration      int            `json:"duration,omitempty"`
	Height        int            `json:"height,omitempty"`
	Width         int            `json:"width,omitempty"`
	Ratio         string         `json:"ratio,omitempty"`
	Format        string         `json:"format,omitempty"`
	PlayAddrH264  URLEnvelope    `json:"play_addr_h264,omitempty"`
	PlayAddrLowBR URLEnvelope    `json:"play_addr_lowbr,omitempty"`
}

type AwemeBitRate struct {
	GearName string      `json:"gear_name,omitempty"`
	Quality  any         `json:"quality_type,omitempty"`
	BitRate  int         `json:"bit_rate,omitempty"`
	PlayAddr URLEnvelope `json:"play_addr,omitempty"`
}

type AwemeImage struct {
	URI                          string   `json:"uri,omitempty"`
	URLList                      []string `json:"url_list,omitempty"`
	DownloadURLList              []string `json:"download_url_list,omitempty"`
	WatermarkFreeDownloadURLList []string `json:"watermark_free_download_url_list,omitempty"`
	Width                        int      `json:"width,omitempty"`
	Height                       int      `json:"height,omitempty"`
}

type AwemeStatistics struct {
	DiggCount    int `json:"digg_count,omitempty"`
	CommentCount int `json:"comment_count,omitempty"`
	CollectCount int `json:"collect_count,omitempty"`
	ShareCount   int `json:"share_count,omitempty"`
	PlayCount    int `json:"play_count,omitempty"`
}

type AwemeShareInfo struct {
	ShareURL       string `json:"share_url,omitempty"`
	ShareTitle     string `json:"share_title,omitempty"`
	ShareDesc      string `json:"share_desc,omitempty"`
	ShareWeiboDesc string `json:"share_weibo_desc,omitempty"`
	BoolPersist    int    `json:"bool_persist,omitempty"`
}

type URLEnvelope struct {
	URI      string   `json:"uri,omitempty"`
	URLList  []string `json:"url_list,omitempty"`
	Width    int      `json:"width,omitempty"`
	Height   int      `json:"height,omitempty"`
	URLKey   string   `json:"url_key,omitempty"`
	DataSize int64    `json:"data_size,omitempty"`
}

type UserSummary struct {
	UID       string `json:"uid,omitempty"`
	SecUID    string `json:"sec_uid,omitempty"`
	UniqueID  string `json:"unique_id,omitempty"`
	Nickname  string `json:"nickname,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type AwemeSummary struct {
	ID           string      `json:"id,omitempty"`
	URL          string      `json:"url,omitempty"`
	ShareURL     string      `json:"share_url,omitempty"`
	Description  string      `json:"description,omitempty"`
	ContentType  string      `json:"content_type,omitempty"`
	CreateTime   int64       `json:"create_time,omitempty"`
	Author       UserSummary `json:"author,omitempty"`
	VideoURL     string      `json:"video_url,omitempty"`
	CoverURL     string      `json:"cover_url,omitempty"`
	ImageURLs    []string    `json:"image_urls,omitempty"`
	Duration     int         `json:"duration,omitempty"`
	DiggCount    int         `json:"digg_count,omitempty"`
	CommentCount int         `json:"comment_count,omitempty"`
	CollectCount int         `json:"collect_count,omitempty"`
	ShareCount   int         `json:"share_count,omitempty"`
	IsTop        bool        `json:"is_top,omitempty"`
}

func CanParseProfile(rawURL string) bool {
	_, ok := ParseProfileURL(rawURL)
	return ok
}

func ExtractProfileURL(text string) string {
	text = strings.TrimSpace(text)
	for _, match := range shareURLRE.FindAllString(text, -1) {
		match = strings.Trim(match, " \t\r\n，。；;、.,!?！？")
		if CanParseProfile(match) {
			return match
		}
	}
	if CanParseProfile(text) {
		return text
	}
	return ""
}

func ParseProfileURL(rawURL string) (ProfileURL, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ProfileURL{}, false
	}
	if secUserIDRE.MatchString(rawURL) {
		return ProfileURL{SecUserID: rawURL, Canonical: CanonicalProfileURL(rawURL)}, true
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ProfileURL{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if !isDouyinHost(host) {
		return ProfileURL{}, false
	}
	for _, key := range []string{"sec_user_id", "sec_uid"} {
		if secUID := strings.TrimSpace(parsed.Query().Get(key)); secUserIDRE.MatchString(secUID) {
			return ProfileURL{SecUserID: secUID, Canonical: CanonicalProfileURL(secUID)}, true
		}
	}
	segments := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(segments) >= 2 {
		switch {
		case segments[0] == "user":
			if secUID, ok := unescapeSecUID(segments[1]); ok {
				return ProfileURL{SecUserID: secUID, Canonical: CanonicalProfileURL(secUID)}, true
			}
		case segments[0] == "share" && segments[1] == "user" && len(segments) >= 3:
			if secUID, ok := unescapeSecUID(segments[2]); ok {
				return ProfileURL{SecUserID: secUID, Canonical: CanonicalProfileURL(secUID)}, true
			}
		}
	}
	return ProfileURL{}, false
}

func CanonicalProfileURL(secUserID string) string {
	secUserID = strings.TrimSpace(secUserID)
	if secUserID == "" {
		return ""
	}
	return strings.TrimRight(BaseURL, "/") + "/user/" + url.PathEscape(secUserID)
}

func FetchUserProfile(ctx context.Context, rawURL string, opts ProfileOptions) (*ProfilePage, error) {
	return NewClient().FetchUserProfile(ctx, rawURL, opts)
}

func (c *Client) FetchUserProfile(ctx context.Context, rawURL string, opts ProfileOptions) (*ProfilePage, error) {
	target, ok := ParseProfileURL(ExtractProfileURL(rawURL))
	if !ok {
		return nil, errUnsupportedProfileURL
	}
	opts = normalizeProfileOptions(opts)
	page := &ProfilePage{
		URL:       target,
		SourceURL: rawURL,
		Request:   profileRequestForOutput(opts),
	}
	cookie := firstNonEmptyString(opts.Cookie, c.cookie())
	if strings.TrimSpace(opts.PageHTML) != "" {
		parsed, err := parseProfilePageHTML(target.Canonical, opts.PageHTML, target.SecUserID)
		if err != nil {
			page.Warnings = append(page.Warnings, "parse_user_page: "+err.Error())
		} else {
			page.User = parsed.User
			page.PageHTML = opts.PageHTML
			page.PageURL = target.Canonical
		}
	} else if !opts.SkipPage {
		body, finalURL, cookies, err := c.fetchProfilePageHTML(ctx, c.profilePageURL(target.SecUserID), opts, cookie)
		if err != nil {
			page.Warnings = append(page.Warnings, "fetch_user_page: "+err.Error())
		} else {
			page.PageHTML = string(body)
			page.PageURL = finalURL
			cookie = mergeCookieHeaders(cookie, cookiesHeader(cookies))
			parsed, err := parseProfilePageHTML(finalURL, string(body), target.SecUserID)
			if err != nil {
				page.Warnings = append(page.Warnings, "parse_user_page: "+err.Error())
			} else {
				page.User = parsed.User
				if parsed.URL.SecUserID != "" {
					page.URL = parsed.URL
				}
			}
		}
	}

	response, apiURL, rawResponse, err := c.fetchAwemePost(ctx, page.URL, opts, cookie)
	if err != nil {
		if page.User.SecUID != "" {
			page.Warnings = append(page.Warnings, "fetch_aweme_post: "+err.Error())
			return page, nil
		}
		return nil, err
	}
	page.APIURL = apiURL
	page.Response = *response
	page.RawResponse = rawResponse
	page.Posts = SummarizeAwemes(response.AwemeList)
	page.HasMore = response.HasMore > 0
	page.MinCursor = response.MinCursor
	page.MaxCursor = response.MaxCursor
	if page.User.SecUID == "" {
		page.User = firstProfileFromAwemes(response.AwemeList, page.URL.SecUserID)
	}
	if page.User.SecUID == "" {
		page.User.SecUID = page.URL.SecUserID
	}
	return page, nil
}

func ParseProfilePageHTML(rawURL string, htmlText string) (*ProfilePage, error) {
	return parseProfilePageHTML(rawURL, htmlText, "")
}

func parseProfilePageHTML(rawURL string, htmlText string, preferredSecUID string) (*ProfilePage, error) {
	target, _ := ParseProfileURL(rawURL)
	user, ok := extractProfileUserFromHTML(htmlText, firstNonEmptyString(preferredSecUID, target.SecUserID))
	if !ok {
		return nil, errors.New("未找到抖音作者信息")
	}
	if user.SecUID == "" && target.SecUserID != "" {
		user.SecUID = target.SecUserID
	}
	if target.SecUserID == "" && user.SecUID != "" {
		target = ProfileURL{SecUserID: user.SecUID, Canonical: CanonicalProfileURL(user.SecUID)}
	}
	return &ProfilePage{
		URL:       target,
		SourceURL: rawURL,
		PageURL:   target.Canonical,
		User:      user,
		PageHTML:  htmlText,
	}, nil
}

func ExtractProfileUserFromHTML(htmlText string) (UserProfile, bool) {
	return extractProfileUserFromHTML(htmlText, "")
}

func extractProfileUserFromHTML(htmlText string, preferredSecUID string) (UserProfile, bool) {
	var first UserProfile
	found := false
	for _, chunk := range extractPaceFlightStrings(htmlText) {
		value, ok := parseFlightJSONChunk(chunk)
		if !ok {
			continue
		}
		if user, ok := findProfileUser(value); ok {
			if preferredSecUID != "" && user.SecUID == preferredSecUID {
				return user, true
			}
			if !found {
				first = user
				found = true
			}
		}
	}
	return first, found
}

func ParseAwemePostResponse(body []byte) (*AwemePostResponse, error) {
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil, errors.New("抖音作品列表响应为空，可能需要 cookie 或签名参数")
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	var decoded AwemePostResponse
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("解析抖音作品列表失败: %w", err)
	}
	if decoded.StatusCode != 0 {
		message := strings.TrimSpace(decoded.StatusMsg)
		if message == "" {
			message = "unknown error"
		}
		return nil, fmt.Errorf("抖音作品列表返回错误: status_code=%d status_msg=%s", decoded.StatusCode, message)
	}
	return &decoded, nil
}

func SummarizeAwemes(awemes []Aweme) []AwemeSummary {
	if len(awemes) == 0 {
		return nil
	}
	out := make([]AwemeSummary, 0, len(awemes))
	for _, aweme := range awemes {
		summary := SummarizeAweme(aweme)
		if summary.ID != "" || summary.VideoURL != "" || len(summary.ImageURLs) > 0 {
			out = append(out, summary)
		}
	}
	return out
}

func SummarizeAweme(aweme Aweme) AwemeSummary {
	id := strings.TrimSpace(aweme.AwemeID)
	imageURLs := aweme.ImageURLs()
	videoURL := aweme.VideoURL()
	contentType := "video"
	if len(imageURLs) > 0 {
		contentType = "image_album"
	} else if videoURL == "" {
		contentType = "post"
	}
	shareURL := normalizeDouyinURL(aweme.ShareInfo.ShareURL)
	return AwemeSummary{
		ID:           id,
		URL:          firstNonEmptyString(shareURL, CanonicalAwemeURL(id, contentType)),
		ShareURL:     shareURL,
		Description:  strings.TrimSpace(aweme.Desc),
		ContentType:  contentType,
		CreateTime:   aweme.CreateTime,
		Author:       aweme.Author.Summary(),
		VideoURL:     videoURL,
		CoverURL:     aweme.CoverURL(),
		ImageURLs:    imageURLs,
		Duration:     firstPositiveInt(aweme.Duration, aweme.Video.Duration),
		DiggCount:    aweme.Statistics.DiggCount,
		CommentCount: aweme.Statistics.CommentCount,
		CollectCount: aweme.Statistics.CollectCount,
		ShareCount:   aweme.Statistics.ShareCount,
		IsTop:        aweme.IsTop > 0,
	}
}

func CanonicalAwemeURL(awemeID string, contentType string) string {
	awemeID = strings.TrimSpace(awemeID)
	if awemeID == "" {
		return ""
	}
	path := "video"
	if contentType == "image_album" {
		path = "note"
	}
	return strings.TrimRight(BaseURL, "/") + "/" + path + "/" + url.PathEscape(awemeID)
}

func (a Aweme) VideoURL() string {
	return firstNonEmptyString(
		a.Video.PlayAddr.FirstURL(),
		a.Video.PlayAddrH264.FirstURL(),
		a.Video.PlayAddrLowBR.FirstURL(),
		a.Video.DownloadAddr.FirstURL(),
	)
}

func (a Aweme) CoverURL() string {
	if url := firstNonEmptyString(a.Video.Cover.FirstURL(), a.Video.OriginCover.FirstURL(), a.Video.DynamicCover.FirstURL()); url != "" {
		return url
	}
	for _, image := range append(a.Images, a.OriginalImages...) {
		if url := image.FirstURL(); url != "" {
			return url
		}
	}
	return ""
}

func (a Aweme) ImageURLs() []string {
	images := a.Images
	if len(images) == 0 {
		images = a.OriginalImages
	}
	out := make([]string, 0, len(images))
	seen := map[string]bool{}
	for _, image := range images {
		rawURL := image.FirstURL()
		if rawURL == "" || seen[rawURL] {
			continue
		}
		seen[rawURL] = true
		out = append(out, rawURL)
	}
	return out
}

func (a AwemeAuthor) Summary() UserSummary {
	return UserSummary{
		UID:       a.UID,
		SecUID:    a.SecUID,
		UniqueID:  firstNonEmptyString(a.UniqueID, a.ShortID),
		Nickname:  a.Nickname,
		AvatarURL: firstNonEmptyString(a.AvatarLarge.FirstURL(), a.AvatarMedium.FirstURL(), a.AvatarThumb.FirstURL()),
	}
}

func (a AwemeAuthor) Profile() UserProfile {
	coverURL := ""
	for _, cover := range a.CoverURL {
		if coverURL = cover.FirstURL(); coverURL != "" {
			break
		}
	}
	return UserProfile{
		UID:                     a.UID,
		SecUID:                  a.SecUID,
		ShortID:                 a.ShortID,
		UniqueID:                a.UniqueID,
		Nickname:                a.Nickname,
		Signature:               a.Signature,
		AvatarURL:               firstNonEmptyString(a.AvatarLarge.FirstURL(), a.AvatarMedium.FirstURL(), a.AvatarThumb.FirstURL()),
		CoverURL:                coverURL,
		FollowerCount:           a.FollowerCount,
		FollowingCount:          a.FollowingCount,
		MPlatformFollowersCount: a.MPlatformFollowersCount,
		AwemeCount:              a.AwemeCount,
		FavoritingCount:         a.FavoritingCount,
		TotalFavorited:          a.TotalFavorited,
		Country:                 a.Country,
		Province:                a.Province,
		City:                    a.City,
		IPLocation:              a.IPLocation,
		EnterpriseVerifyReason:  a.EnterpriseVerifyReason,
		CustomVerify:            a.CustomVerify,
	}
}

func (u URLEnvelope) FirstURL() string {
	for _, rawURL := range u.URLList {
		if rawURL = strings.TrimSpace(rawURL); rawURL != "" {
			return normalizeDouyinURL(rawURL)
		}
	}
	return normalizeDouyinURL(u.URI)
}

func (i AwemeImage) FirstURL() string {
	for _, values := range [][]string{i.WatermarkFreeDownloadURLList, i.DownloadURLList, i.URLList} {
		for _, rawURL := range values {
			if rawURL = strings.TrimSpace(rawURL); rawURL != "" {
				return normalizeDouyinURL(rawURL)
			}
		}
	}
	return normalizeDouyinURL(i.URI)
}

func (c *Client) fetchProfilePageHTML(ctx context.Context, rawURL string, opts ProfileOptions, cookie string) ([]byte, string, []*http.Cookie, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, rawURL, nil, err
	}
	c.setProfilePageHeaders(req, opts, cookie)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, rawURL, nil, fmt.Errorf("请求抖音作者主页失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, rawURL, resp.Cookies(), err
	}
	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, finalURL, resp.Cookies(), fmt.Errorf("请求抖音作者主页失败: HTTP %d: %s", resp.StatusCode, trimBody(body))
	}
	return body, finalURL, resp.Cookies(), nil
}

func (c *Client) fetchAwemePost(ctx context.Context, target ProfileURL, opts ProfileOptions, cookie string) (*AwemePostResponse, string, json.RawMessage, error) {
	apiURL, err := c.awemePostURL(target.SecUserID, opts)
	if err != nil {
		return nil, "", nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, apiURL, nil, err
	}
	c.setAwemePostHeaders(req, target.Canonical, opts, cookie)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, apiURL, nil, fmt.Errorf("请求抖音作品列表失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apiURL, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apiURL, nil, fmt.Errorf("请求抖音作品列表失败: HTTP %d: %s", resp.StatusCode, trimBody(body))
	}
	decoded, err := ParseAwemePostResponse(body)
	if err != nil {
		return nil, apiURL, nil, err
	}
	return decoded, apiURL, json.RawMessage(append([]byte(nil), body...)), nil
}

func (c *Client) awemePostURL(secUserID string, opts ProfileOptions) (string, error) {
	if strings.TrimSpace(opts.APIURL) != "" {
		return normalizeWithBase(opts.APIURL, c.baseURL())
	}
	parsed, err := url.Parse(strings.TrimRight(c.baseURL(), "/") + defaultAwemePostPath)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("device_platform", "webapp")
	query.Set("aid", "6383")
	query.Set("channel", "channel_pc_web")
	query.Set("sec_user_id", secUserID)
	query.Set("max_cursor", strconv.FormatInt(opts.MaxCursor, 10))
	query.Set("locate_query", "false")
	query.Set("show_live_replay_strategy", "1")
	query.Set("need_time_list", "1")
	query.Set("time_list_query", "0")
	query.Set("whale_cut_token", "")
	query.Set("cut_version", "1")
	query.Set("count", strconv.Itoa(opts.Count))
	query.Set("publish_video_strategy_type", "2")
	query.Set("from_user_page", "1")
	query.Set("update_version_code", "170400")
	query.Set("pc_client_type", "1")
	query.Set("pc_libra_divert", "Mac")
	query.Set("support_h265", "1")
	query.Set("support_dash", "0")
	query.Set("cpu_core_num", "10")
	query.Set("version_code", "290100")
	query.Set("version_name", "29.1.0")
	query.Set("cookie_enabled", "true")
	query.Set("screen_width", "1512")
	query.Set("screen_height", "982")
	query.Set("browser_language", "zh-CN")
	query.Set("browser_platform", "MacIntel")
	query.Set("browser_name", "Chrome")
	query.Set("browser_version", "149.0.0.0")
	query.Set("browser_online", "true")
	query.Set("engine_name", "Blink")
	query.Set("engine_version", "149.0.0.0")
	query.Set("os_name", "Mac OS")
	query.Set("os_version", "10.15.7")
	query.Set("device_memory", "32")
	query.Set("platform", "PC")
	query.Set("downlink", "10")
	query.Set("effective_type", "4g")
	query.Set("round_trip_time", "100")
	for key, value := range opts.Extra {
		if strings.TrimSpace(key) != "" {
			query.Set(key, value)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *Client) profilePageURL(secUserID string) string {
	return strings.TrimRight(c.baseURL(), "/") + "/user/" + url.PathEscape(strings.TrimSpace(secUserID))
}

func (c *Client) setProfilePageHeaders(req *http.Request, opts ProfileOptions, cookie string) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Sec-Ch-Ua", `"Google Chrome";v="149", "Chromium";v="149", "Not)A;Brand";v="24"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", c.webUserAgent(opts.UserAgent))
	if strings.TrimSpace(cookie) != "" {
		req.Header.Set("Cookie", strings.TrimSpace(cookie))
	}
}

func (c *Client) setAwemePostHeaders(req *http.Request, referer string, opts ProfileOptions, cookie string) {
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Priority", "u=1, i")
	req.Header.Set("Referer", firstNonEmptyString(referer, BaseURL+"/"))
	req.Header.Set("Sec-Ch-Ua", `"Google Chrome";v="149", "Chromium";v="149", "Not)A;Brand";v="24"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", c.webUserAgent(opts.UserAgent))
	if uifid := firstNonEmptyString(opts.Extra["uifid"], cookieValue(cookie, "UIFID")); uifid != "" {
		req.Header.Set("Uifid", uifid)
	}
	if strings.TrimSpace(cookie) != "" {
		req.Header.Set("Cookie", strings.TrimSpace(cookie))
	}
}

func normalizeProfileOptions(opts ProfileOptions) ProfileOptions {
	if opts.Count <= 0 {
		opts.Count = 18
	}
	if opts.MaxCursor < 0 {
		opts.MaxCursor = 0
	}
	if opts.Extra == nil {
		opts.Extra = map[string]string{}
	}
	return opts
}

func profileRequestForOutput(opts ProfileOptions) ProfileOptions {
	opts.Cookie = ""
	opts.PageHTML = ""
	return opts
}

func firstProfileFromAwemes(awemes []Aweme, fallbackSecUID string) UserProfile {
	for _, aweme := range awemes {
		profile := aweme.Author.Profile()
		if profile.SecUID != "" || profile.UID != "" || profile.Nickname != "" {
			if profile.SecUID == "" {
				profile.SecUID = fallbackSecUID
			}
			return profile
		}
	}
	return UserProfile{SecUID: fallbackSecUID}
}

func extractPaceFlightStrings(htmlText string) []string {
	const prefix = "self.__pace_f.push([1,"
	var chunks []string
	offset := 0
	for {
		idx := strings.Index(htmlText[offset:], prefix)
		if idx < 0 {
			break
		}
		idx += offset + len(prefix)
		for idx < len(htmlText) && (htmlText[idx] == ' ' || htmlText[idx] == '\t' || htmlText[idx] == '\n' || htmlText[idx] == '\r') {
			idx++
		}
		if idx >= len(htmlText) || htmlText[idx] != '"' {
			offset = idx
			continue
		}
		raw, end, ok := scanQuotedString(htmlText, idx)
		if !ok {
			offset = idx + 1
			continue
		}
		if value, err := strconv.Unquote(raw); err == nil {
			chunks = append(chunks, value)
		}
		offset = end
	}
	return chunks
}

func scanQuotedString(text string, start int) (string, int, bool) {
	escaped := false
	for i := start + 1; i < len(text); i++ {
		ch := text[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			return text[start : i+1], i + 1, true
		}
	}
	return "", start, false
}

func parseFlightJSONChunk(chunk string) (any, bool) {
	chunk = strings.TrimSpace(chunk)
	if idx := strings.Index(chunk, ":"); idx > 0 && allDigits(chunk[:idx]) {
		chunk = strings.TrimSpace(chunk[idx+1:])
	}
	if !strings.HasPrefix(chunk, "{") && !strings.HasPrefix(chunk, "[") {
		return nil, false
	}
	decoder := json.NewDecoder(strings.NewReader(chunk))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false
	}
	return value, true
}

func findProfileUser(value any) (UserProfile, bool) {
	switch node := value.(type) {
	case map[string]any:
		if user, ok := profileUserFromCandidate(node); ok {
			return user, true
		}
		for _, child := range node {
			if user, ok := findProfileUser(child); ok {
				return user, true
			}
		}
	case []any:
		for _, child := range node {
			if user, ok := findProfileUser(child); ok {
				return user, true
			}
		}
	}
	return UserProfile{}, false
}

func profileUserFromCandidate(node map[string]any) (UserProfile, bool) {
	if secUID := stringField(node, "secUid", "sec_uid"); secUID != "" {
		return userProfileFromMap(node), true
	}
	if wrapped, ok := node["user"].(map[string]any); ok {
		if user, ok := profileUserFromCandidate(wrapped); ok {
			return user, true
		}
	}
	return UserProfile{}, false
}

func userProfileFromMap(node map[string]any) UserProfile {
	user := UserProfile{
		UID:                     stringField(node, "uid"),
		SecUID:                  stringField(node, "secUid", "sec_uid"),
		ShortID:                 stringField(node, "shortId", "short_id"),
		UniqueID:                stringField(node, "uniqueId", "unique_id"),
		RealName:                stringField(node, "realName", "real_name"),
		Nickname:                stringField(node, "nickname"),
		Signature:               firstNonEmptyString(stringField(node, "signature"), stringField(node, "desc")),
		AvatarURL:               firstNonEmptyString(stringField(node, "avatarUrl", "avatar_url"), firstURLFromMap(nodeMap(node, "avatar_thumb"))),
		Avatar300URL:            stringField(node, "avatar300Url", "avatar_300_url"),
		FollowerCount:           intField(node, "followerCount", "follower_count"),
		FollowingCount:          intField(node, "followingCount", "following_count"),
		MPlatformFollowersCount: intField(node, "mplatformFollowersCount", "mplatform_followers_count"),
		AwemeCount:              intField(node, "awemeCount", "aweme_count"),
		FavoritingCount:         intField(node, "favoritingCount", "favoriting_count"),
		TotalFavorited:          intField(node, "totalFavorited", "total_favorited"),
		Country:                 stringField(node, "country"),
		Province:                stringField(node, "province"),
		City:                    stringField(node, "city"),
		IPLocation:              stringField(node, "ipLocation", "ip_location"),
		EnterpriseVerifyReason:  stringField(node, "enterpriseVerifyReason", "enterprise_verify_reason"),
		CustomVerify:            stringField(node, "customVerify", "custom_verify"),
		Secret:                  intField(node, "secret"),
		RoomID:                  firstNonEmptyString(stringField(node, "roomIdStr", "room_id_str"), stringField(node, "roomId", "room_id")),
		ContinuationState:       intField(node, "continuationState", "continuation_state"),
	}
	if shareInfo := nodeMap(node, "shareInfo", "share_info"); shareInfo != nil {
		user.ShareURL = normalizeDouyinURL(stringField(shareInfo, "shareUrl", "share_url"))
		if user.AvatarURL == "" {
			user.AvatarURL = firstURLFromMap(nodeMap(shareInfo, "shareImageUrl", "share_image_url"))
		}
	}
	if cover := firstCoverURL(nodeMap(node, "coverAndHeadImageInfo", "cover_and_head_image_info")); cover != "" {
		user.CoverURL = cover
	}
	if user.Avatar300URL == "" {
		user.Avatar300URL = user.AvatarURL
	}
	return user
}

func firstCoverURL(node map[string]any) string {
	if node == nil {
		return ""
	}
	list, _ := node["profileCoverList"].([]any)
	if len(list) == 0 {
		list, _ = node["profile_cover_list"].([]any)
	}
	for _, item := range list {
		itemMap, _ := item.(map[string]any)
		if rawURL := firstURLFromMap(nodeMap(itemMap, "coverUrl", "cover_url")); rawURL != "" {
			return rawURL
		}
	}
	return ""
}

func firstURLFromMap(node map[string]any) string {
	if node == nil {
		return ""
	}
	for _, key := range []string{"url_list", "urlList"} {
		if list, ok := node[key].([]any); ok {
			for _, item := range list {
				if rawURL := normalizeDouyinURL(fmt.Sprint(item)); rawURL != "" {
					return rawURL
				}
			}
		}
	}
	return normalizeDouyinURL(stringField(node, "url", "uri"))
}

func nodeMap(node map[string]any, keys ...string) map[string]any {
	if node == nil {
		return nil
	}
	for _, key := range keys {
		if child, ok := node[key].(map[string]any); ok {
			return child
		}
	}
	return nil
}

func stringField(node map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := node[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			value := strings.TrimSpace(stdhtml.UnescapeString(typed))
			if value != "" && value != "$undefined" {
				return value
			}
		case json.Number:
			return typed.String()
		case float64:
			if typed == float64(int64(typed)) {
				return strconv.FormatInt(int64(typed), 10)
			}
			return strconv.FormatFloat(typed, 'f', -1, 64)
		case int:
			return strconv.Itoa(typed)
		case int64:
			return strconv.FormatInt(typed, 10)
		}
	}
	return ""
}

func intField(node map[string]any, keys ...string) int {
	for _, key := range keys {
		value, ok := node[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case json.Number:
			if v, err := typed.Int64(); err == nil {
				return int(v)
			}
		case float64:
			return int(typed)
		case int:
			return typed
		case int64:
			return int(typed)
		case string:
			typed = strings.TrimSpace(typed)
			if typed == "" || typed == "$undefined" {
				continue
			}
			if v, err := strconv.Atoi(typed); err == nil {
				return v
			}
		}
	}
	return 0
}

func (c *Client) baseURL() string {
	if c != nil && strings.TrimSpace(c.BaseURL) != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return BaseURL
}

func (c *Client) cookie() string {
	if c != nil {
		return strings.TrimSpace(c.Cookie)
	}
	return ""
}

func (c *Client) webUserAgent(override string) string {
	if strings.TrimSpace(override) != "" {
		return strings.TrimSpace(override)
	}
	return defaultAPIUserAgent
}

func normalizeWithBase(rawURL string, baseURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("URL 为空")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if parsed.IsAbs() {
		return parsed.String(), nil
	}
	base, err := url.Parse(strings.TrimRight(firstNonEmptyString(baseURL, BaseURL), "/") + "/")
	if err != nil {
		return "", err
	}
	return base.ResolveReference(parsed).String(), nil
}

func normalizeDouyinURL(rawURL string) string {
	rawURL = strings.TrimSpace(stdhtml.UnescapeString(rawURL))
	if rawURL == "" || rawURL == "$undefined" {
		return ""
	}
	if strings.HasPrefix(rawURL, "//") {
		return "https:" + rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err == nil && parsed.IsAbs() {
		return parsed.String()
	}
	if strings.HasPrefix(rawURL, "www.") {
		return "https://" + rawURL
	}
	return rawURL
}

func unescapeSecUID(segment string) (string, bool) {
	secUID, err := url.PathUnescape(segment)
	if err != nil {
		return "", false
	}
	secUID = strings.TrimSpace(secUID)
	return secUID, secUserIDRE.MatchString(secUID)
}

func isDouyinHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "douyin.com" ||
		host == "www.douyin.com" ||
		host == "iesdouyin.com" ||
		host == "www.iesdouyin.com" ||
		host == "v.douyin.com" ||
		strings.HasSuffix(host, ".douyin.com") ||
		strings.HasSuffix(host, ".iesdouyin.com")
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func mergeCookieHeaders(values ...string) string {
	parts := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		for _, part := range strings.Split(value, ";") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			name := part
			if idx := strings.Index(part, "="); idx >= 0 {
				name = strings.TrimSpace(part[:idx])
			}
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "; ")
}

func cookiesHeader(cookies []*http.Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil || cookie.Name == "" {
			continue
		}
		parts = append(parts, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(parts, "; ")
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

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
