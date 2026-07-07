package instagram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	BaseURL                = "https://www.instagram.com"
	ContentTypeUserProfile = "account"
	DefaultAppID           = "936619743392459"
	defaultUserAgent       = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
)

var (
	ErrUnsupportedURL = errors.New("unsupported instagram profile url")
	shareURLRE        = regexp.MustCompile(`https?://[^\s"'<>]+`)
	usernameRE        = regexp.MustCompile(`^[A-Za-z0-9._]{1,30}$`)
	titleUsernameRE   = regexp.MustCompile(`\(@([A-Za-z0-9._]+)\)`)
	appIDRE           = regexp.MustCompile(`(?:"X-IG-App-ID"|appId)"\s*:\s*"?([0-9]+)"?`)
	csrfTokenRE       = regexp.MustCompile(`"csrf_token"\s*:\s*"([^"]+)"`)
	profileIDRE       = regexp.MustCompile(`(?:"profile_id"\s*:\s*"|profilePage_)([0-9]+)`)
	countValueRE      = regexp.MustCompile(`(?i)([0-9]+(?:[.,][0-9]+)?)(?:\s*)(k|m|b|万|亿)?`)
	unsafeFilenameRE  = regexp.MustCompile(`[\\/:*?"<>|#\n\r]`)
	dotsRE            = regexp.MustCompile(`\.{2,}`)
)

var reservedProfilePaths = map[string]bool{
	"about":     true,
	"accounts":  true,
	"api":       true,
	"challenge": true,
	"developer": true,
	"direct":    true,
	"explore":   true,
	"graphql":   true,
	"legal":     true,
	"oauth":     true,
	"p":         true,
	"privacy":   true,
	"reel":      true,
	"reels":     true,
	"stories":   true,
	"terms":     true,
	"tv":        true,
	"web":       true,
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	HTTPClient HTTPClient
	BaseURL    string
	UserAgent  string
	Cookie     string
	AppID      string
}

type ProfileURL struct {
	Username  string `json:"username"`
	Canonical string `json:"canonical"`
}

type ProfileOptions struct {
	Cookie string `json:"-"`
	AppID  string `json:"app_id,omitempty"`
	Count  int    `json:"count,omitempty"`
}

type ProfilePage struct {
	URL         ProfileURL      `json:"url"`
	SourceURL   string          `json:"source_url"`
	APIURL      string          `json:"api_url,omitempty"`
	AppID       string          `json:"app_id,omitempty"`
	CSRFToken   string          `json:"csrf_token,omitempty"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	Profile     UserProfile     `json:"profile"`
	Posts       []PostSummary   `json:"posts,omitempty"`
	RawAPI      json.RawMessage `json:"raw_api,omitempty"`
	Warnings    []string        `json:"warnings,omitempty"`
	PageHTML    string          `json:"-"`
}

type UserProfile struct {
	ID              string `json:"id,omitempty"`
	Username        string `json:"username,omitempty"`
	FullName        string `json:"full_name,omitempty"`
	Biography       string `json:"biography,omitempty"`
	ExternalURL     string `json:"external_url,omitempty"`
	ProfilePicURL   string `json:"profile_pic_url,omitempty"`
	ProfilePicURLHD string `json:"profile_pic_url_hd,omitempty"`
	IsPrivate       bool   `json:"is_private,omitempty"`
	IsVerified      bool   `json:"is_verified,omitempty"`
	FollowersCount  int    `json:"followers_count,omitempty"`
	FollowingCount  int    `json:"following_count,omitempty"`
	MediaCount      int    `json:"media_count,omitempty"`
}

type PostSummary struct {
	ID               string `json:"id,omitempty"`
	Shortcode        string `json:"shortcode,omitempty"`
	URL              string `json:"url,omitempty"`
	Caption          string `json:"caption,omitempty"`
	DisplayURL       string `json:"display_url,omitempty"`
	ThumbnailURL     string `json:"thumbnail_url,omitempty"`
	VideoURL         string `json:"video_url,omitempty"`
	IsVideo          bool   `json:"is_video,omitempty"`
	TakenAtTimestamp int64  `json:"taken_at_timestamp,omitempty"`
	LikeCount        int    `json:"like_count,omitempty"`
	CommentCount     int    `json:"comment_count,omitempty"`
	Width            int    `json:"width,omitempty"`
	Height           int    `json:"height,omitempty"`
}

type WebProfileInfoResponse struct {
	Status  string             `json:"status,omitempty"`
	Message string             `json:"message,omitempty"`
	Data    WebProfileInfoData `json:"data"`
}

type WebProfileInfoData struct {
	User WebProfileUser `json:"user"`
}

type WebProfileUser struct {
	ID                         string        `json:"id,omitempty"`
	Username                   string        `json:"username,omitempty"`
	FullName                   string        `json:"full_name,omitempty"`
	Biography                  string        `json:"biography,omitempty"`
	ExternalURL                string        `json:"external_url,omitempty"`
	ProfilePicURL              string        `json:"profile_pic_url,omitempty"`
	ProfilePicURLHD            string        `json:"profile_pic_url_hd,omitempty"`
	IsPrivate                  bool          `json:"is_private,omitempty"`
	IsVerified                 bool          `json:"is_verified,omitempty"`
	EdgeFollowedBy             CountEnvelope `json:"edge_followed_by,omitempty"`
	EdgeFollow                 CountEnvelope `json:"edge_follow,omitempty"`
	EdgeOwnerToTimelineMedia   TimelineMedia `json:"edge_owner_to_timeline_media,omitempty"`
	EdgeFelixVideoTimeline     TimelineMedia `json:"edge_felix_video_timeline,omitempty"`
	EdgeSavedMedia             TimelineMedia `json:"edge_saved_media,omitempty"`
	EdgeMediaCollections       TimelineMedia `json:"edge_media_collections,omitempty"`
	ShouldShowCategory         any           `json:"should_show_category,omitempty"`
	ShouldShowPublicContacts   any           `json:"should_show_public_contacts,omitempty"`
	BusinessContactMethod      any           `json:"business_contact_method,omitempty"`
	CategoryName               string        `json:"category_name,omitempty"`
	OverallCategoryName        string        `json:"overall_category_name,omitempty"`
	Pronouns                   []string      `json:"pronouns,omitempty"`
	HasARBackedProfilePicture  bool          `json:"has_ar_effects,omitempty"`
	HasBlockedViewer           bool          `json:"has_blocked_viewer,omitempty"`
	HasRequestedViewer         bool          `json:"has_requested_viewer,omitempty"`
	RequestedByViewer          bool          `json:"requested_by_viewer,omitempty"`
	FollowedByViewer           bool          `json:"followed_by_viewer,omitempty"`
	FollowsViewer              bool          `json:"follows_viewer,omitempty"`
	HasClips                   bool          `json:"has_clips,omitempty"`
	HasGuides                  bool          `json:"has_guides,omitempty"`
	HasChannel                 bool          `json:"has_channel,omitempty"`
	HighlightReelCount         int           `json:"highlight_reel_count,omitempty"`
	ConnectedFBPage            any           `json:"connected_fb_page,omitempty"`
	TransparencyProductEnabled bool          `json:"transparency_product_enabled,omitempty"`
}

type CountEnvelope struct {
	Count int `json:"count,omitempty"`
}

type TimelineMedia struct {
	Count    int             `json:"count,omitempty"`
	PageInfo PageInfo        `json:"page_info,omitempty"`
	Edges    []TimelineEdge  `json:"edges,omitempty"`
	Nodes    []TimelineMedia `json:"nodes,omitempty"`
}

type PageInfo struct {
	HasNextPage bool   `json:"has_next_page,omitempty"`
	EndCursor   string `json:"end_cursor,omitempty"`
}

type TimelineEdge struct {
	Node MediaNode `json:"node"`
}

type MediaNode struct {
	ID                    string          `json:"id,omitempty"`
	Shortcode             string          `json:"shortcode,omitempty"`
	DisplayURL            string          `json:"display_url,omitempty"`
	ThumbnailSrc          string          `json:"thumbnail_src,omitempty"`
	VideoURL              string          `json:"video_url,omitempty"`
	IsVideo               bool            `json:"is_video,omitempty"`
	TakenAtTimestamp      int64           `json:"taken_at_timestamp,omitempty"`
	EdgeMediaToCaption    CaptionEnvelope `json:"edge_media_to_caption,omitempty"`
	EdgeLikedBy           CountEnvelope   `json:"edge_liked_by,omitempty"`
	EdgeMediaPreviewLike  CountEnvelope   `json:"edge_media_preview_like,omitempty"`
	EdgeMediaToComment    CountEnvelope   `json:"edge_media_to_comment,omitempty"`
	Dimensions            Dimensions      `json:"dimensions,omitempty"`
	AccessibilityCaption  string          `json:"accessibility_caption,omitempty"`
	Typename              string          `json:"__typename,omitempty"`
	ProductType           string          `json:"product_type,omitempty"`
	Owner                 MediaOwner      `json:"owner,omitempty"`
	CoauthorProducers     []MediaOwner    `json:"coauthor_producers,omitempty"`
	Location              any             `json:"location,omitempty"`
	CommentsDisabled      bool            `json:"comments_disabled,omitempty"`
	CommentsDisabledByAPI bool            `json:"comments_disabled_by_viewer,omitempty"`
}

type CaptionEnvelope struct {
	Edges []CaptionEdge `json:"edges,omitempty"`
}

type CaptionEdge struct {
	Node CaptionNode `json:"node"`
}

type CaptionNode struct {
	Text string `json:"text,omitempty"`
}

type Dimensions struct {
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}

type MediaOwner struct {
	ID       string `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
}

func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		BaseURL:    BaseURL,
		UserAgent:  defaultUserAgent,
		AppID:      DefaultAppID,
	}
}

func NewClientWithOptions(client HTTPClient, cookie string, appID string, userAgent string) *Client {
	c := NewClient()
	if client != nil {
		c.HTTPClient = client
	}
	if strings.TrimSpace(cookie) != "" {
		c.Cookie = strings.TrimSpace(cookie)
	}
	if strings.TrimSpace(appID) != "" {
		c.AppID = strings.TrimSpace(appID)
	}
	if strings.TrimSpace(userAgent) != "" {
		c.UserAgent = strings.TrimSpace(userAgent)
	}
	return c
}

func CanParse(rawURL string) bool {
	_, ok := ParseProfileURL(rawURL)
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

func ParseProfileURL(rawURL string) (ProfileURL, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ProfileURL{}, false
	}
	if strings.HasPrefix(rawURL, "@") {
		rawURL = strings.TrimPrefix(rawURL, "@")
	}
	if usernameRE.MatchString(rawURL) && !reservedProfilePaths[strings.ToLower(rawURL)] {
		return ProfileURL{Username: rawURL, Canonical: CanonicalProfileURL(rawURL)}, true
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ProfileURL{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "instagram.com" && host != "www.instagram.com" && !strings.HasSuffix(host, ".instagram.com") {
		return ProfileURL{}, false
	}
	if username := strings.TrimSpace(parsed.Query().Get("username")); usernameRE.MatchString(username) {
		return ProfileURL{Username: username, Canonical: CanonicalProfileURL(username)}, true
	}
	segments := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		return ProfileURL{}, false
	}
	username, err := url.PathUnescape(segments[0])
	if err != nil {
		return ProfileURL{}, false
	}
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if !usernameRE.MatchString(username) || reservedProfilePaths[strings.ToLower(username)] {
		return ProfileURL{}, false
	}
	return ProfileURL{Username: username, Canonical: CanonicalProfileURL(username)}, true
}

func CanonicalProfileURL(username string) string {
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if username == "" {
		return ""
	}
	return BaseURL + "/" + url.PathEscape(username) + "/"
}

func FetchUserProfile(ctx context.Context, rawURL string, opts ProfileOptions) (*ProfilePage, error) {
	return NewClient().FetchUserProfile(ctx, rawURL, opts)
}

func (c *Client) FetchUserProfile(ctx context.Context, rawURL string, opts ProfileOptions) (*ProfilePage, error) {
	target, ok := ParseProfileURL(ExtractShareURL(rawURL))
	if !ok {
		return nil, ErrUnsupportedURL
	}
	if opts.Count <= 0 {
		opts.Count = 12
	}
	pageURL := c.profilePageURL(target.Username)
	body, finalURL, cookies, err := c.fetchPageHTML(ctx, pageURL, opts)
	if err != nil {
		return nil, err
	}
	page, err := ParseProfilePageHTML(target.Canonical, string(body))
	if err != nil {
		return nil, err
	}
	page.SourceURL = firstNonEmpty(rawURL, finalURL, target.Canonical)
	page.URL = target
	page.PageHTML = string(body)
	if page.Profile.Username == "" {
		page.Profile.Username = target.Username
	}
	if page.URL.Canonical == "" {
		page.URL.Canonical = target.Canonical
	}

	appID := firstNonEmpty(opts.AppID, page.AppID, c.AppID, DefaultAppID)
	page.AppID = appID
	apiCookie := mergeCookieHeaders(c.Cookie, opts.Cookie, cookiesHeader(cookies))
	api, apiURL, rawAPI, err := c.fetchProfileInfo(ctx, target.Username, appID, apiCookie, page.URL.Canonical)
	if err != nil {
		page.Warnings = append(page.Warnings, "web_profile_info: "+err.Error())
		return page, nil
	}
	page.APIURL = apiURL
	page.RawAPI = rawAPI
	page.Profile = mergeProfile(page.Profile, api.Data.User.Profile())
	page.Posts = api.Data.User.Posts()
	if page.Profile.Username == "" {
		page.Profile.Username = target.Username
	}
	if page.Profile.MediaCount == 0 {
		page.Profile.MediaCount = len(page.Posts)
	}
	if page.Title == "" {
		page.Title = firstNonEmpty(page.Profile.FullName, page.Profile.Username)
	}
	if page.Description == "" {
		page.Description = page.Profile.Biography
	}
	return page, nil
}

func ParseProfilePageHTML(rawURL string, htmlText string) (*ProfilePage, error) {
	target, _ := ParseProfileURL(rawURL)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(firstNonEmpty(
		metaContent(doc, "property", "og:title"),
		doc.Find("title").First().Text(),
	))
	description := strings.TrimSpace(firstNonEmpty(
		metaContent(doc, "name", "description"),
		metaContent(doc, "property", "og:description"),
	))
	canonical := normalizeURL(firstNonEmpty(
		attr(doc.Find(`link[rel="canonical"]`).First(), "href"),
		metaContent(doc, "property", "og:url"),
		target.Canonical,
	), target.Canonical)
	if parsed, ok := ParseProfileURL(canonical); ok {
		target = parsed
	}
	fullName, username := parseTitleProfile(title)
	if username == "" {
		username = target.Username
	}
	bio := parseBio(description)
	followers, following, mediaCount := parseProfileCounts(description)
	page := &ProfilePage{
		URL:         target,
		SourceURL:   rawURL,
		AppID:       parseAppID(htmlText),
		CSRFToken:   parseCSRFToken(htmlText),
		Title:       cleanTitle(title),
		Description: description,
		Profile: UserProfile{
			ID:              parseProfileID(htmlText),
			Username:        username,
			FullName:        fullName,
			Biography:       bio,
			ProfilePicURL:   normalizeURL(metaContent(doc, "property", "og:image"), canonical),
			ProfilePicURLHD: normalizeURL(metaContent(doc, "property", "og:image"), canonical),
			FollowersCount:  followers,
			FollowingCount:  following,
			MediaCount:      mediaCount,
		},
		PageHTML: htmlText,
	}
	if page.URL.Canonical == "" {
		page.URL.Canonical = canonical
	}
	if page.Profile.Username == "" && page.URL.Username != "" {
		page.Profile.Username = page.URL.Username
	}
	if page.Profile.FullName == "" {
		page.Profile.FullName = page.Profile.Username
	}
	return page, nil
}

func (c *Client) FetchProfileInfo(ctx context.Context, username string, opts ProfileOptions) (*WebProfileInfoResponse, error) {
	api, _, _, err := c.fetchProfileInfo(ctx, username, firstNonEmpty(opts.AppID, c.AppID, DefaultAppID), firstNonEmpty(opts.Cookie, c.Cookie), CanonicalProfileURL(username))
	return api, err
}

func (c *Client) fetchProfileInfo(ctx context.Context, username string, appID string, cookie string, referer string) (*WebProfileInfoResponse, string, json.RawMessage, error) {
	apiURL := c.profileInfoURL(username)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, apiURL, nil, err
	}
	c.setAPIHeaders(req, appID, cookie, referer)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, apiURL, nil, fmt.Errorf("fetch instagram profile info: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apiURL, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apiURL, nil, fmt.Errorf("fetch instagram profile info: HTTP %d: %s", resp.StatusCode, trimBody(body))
	}
	var decoded WebProfileInfoResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, apiURL, nil, fmt.Errorf("decode instagram profile info: %w", err)
	}
	if decoded.Status != "" && decoded.Status != "ok" {
		return nil, apiURL, nil, fmt.Errorf("instagram profile info status %q: %s", decoded.Status, decoded.Message)
	}
	if strings.TrimSpace(decoded.Data.User.Username) == "" {
		return nil, apiURL, nil, fmt.Errorf("instagram profile info missing user")
	}
	return &decoded, apiURL, json.RawMessage(append([]byte(nil), body...)), nil
}

func (u WebProfileUser) Profile() UserProfile {
	return UserProfile{
		ID:              u.ID,
		Username:        u.Username,
		FullName:        u.FullName,
		Biography:       u.Biography,
		ExternalURL:     u.ExternalURL,
		ProfilePicURL:   u.ProfilePicURL,
		ProfilePicURLHD: firstNonEmpty(u.ProfilePicURLHD, u.ProfilePicURL),
		IsPrivate:       u.IsPrivate,
		IsVerified:      u.IsVerified,
		FollowersCount:  u.EdgeFollowedBy.Count,
		FollowingCount:  u.EdgeFollow.Count,
		MediaCount:      u.EdgeOwnerToTimelineMedia.Count,
	}
}

func (u WebProfileUser) Posts() []PostSummary {
	return SummarizeMediaEdges(u.EdgeOwnerToTimelineMedia.Edges)
}

func SummarizeMediaEdges(edges []TimelineEdge) []PostSummary {
	if len(edges) == 0 {
		return nil
	}
	out := make([]PostSummary, 0, len(edges))
	for _, edge := range edges {
		post := SummarizeMedia(edge.Node)
		if post.ID != "" || post.Shortcode != "" || post.DisplayURL != "" {
			out = append(out, post)
		}
	}
	return out
}

func SummarizeMedia(node MediaNode) PostSummary {
	likeCount := node.EdgeLikedBy.Count
	if likeCount == 0 {
		likeCount = node.EdgeMediaPreviewLike.Count
	}
	shortcode := strings.TrimSpace(node.Shortcode)
	return PostSummary{
		ID:               node.ID,
		Shortcode:        shortcode,
		URL:              PostURL(shortcode),
		Caption:          firstCaption(node.EdgeMediaToCaption),
		DisplayURL:       node.DisplayURL,
		ThumbnailURL:     firstNonEmpty(node.ThumbnailSrc, node.DisplayURL),
		VideoURL:         node.VideoURL,
		IsVideo:          node.IsVideo,
		TakenAtTimestamp: node.TakenAtTimestamp,
		LikeCount:        likeCount,
		CommentCount:     node.EdgeMediaToComment.Count,
		Width:            node.Dimensions.Width,
		Height:           node.Dimensions.Height,
	}
}

func PostURL(shortcode string) string {
	shortcode = strings.TrimSpace(shortcode)
	if shortcode == "" {
		return ""
	}
	return BaseURL + "/p/" + url.PathEscape(shortcode) + "/"
}

func (c *Client) fetchPageHTML(ctx context.Context, rawURL string, opts ProfileOptions) ([]byte, string, []*http.Cookie, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, rawURL, nil, err
	}
	c.setDocumentHeaders(req, firstNonEmpty(opts.Cookie, c.Cookie))
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, rawURL, nil, fmt.Errorf("fetch instagram profile page: %w", err)
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
		return nil, finalURL, resp.Cookies(), fmt.Errorf("fetch instagram profile page: HTTP %d: %s", resp.StatusCode, trimBody(body))
	}
	return body, finalURL, resp.Cookies(), nil
}

func (c *Client) profilePageURL(username string) string {
	return strings.TrimRight(firstNonEmpty(c.BaseURL, BaseURL), "/") + "/" + url.PathEscape(strings.TrimPrefix(username, "@")) + "/"
}

func (c *Client) profileInfoURL(username string) string {
	base := strings.TrimRight(firstNonEmpty(c.BaseURL, BaseURL), "/")
	return base + "/api/v1/users/web_profile_info/?username=" + url.QueryEscape(strings.TrimPrefix(username, "@"))
}

func (c *Client) httpClient() HTTPClient {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return NewClient().HTTPClient
}

func (c *Client) userAgent() string {
	if c != nil && strings.TrimSpace(c.UserAgent) != "" {
		return strings.TrimSpace(c.UserAgent)
	}
	return defaultUserAgent
}

func (c *Client) setDocumentHeaders(req *http.Request, cookie string) {
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
	req.Header.Set("User-Agent", c.userAgent())
	if strings.TrimSpace(cookie) != "" {
		req.Header.Set("Cookie", strings.TrimSpace(cookie))
	}
}

func (c *Client) setAPIHeaders(req *http.Request, appID string, cookie string, referer string) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", firstNonEmpty(referer, BaseURL+"/"))
	req.Header.Set("Sec-Ch-Ua", `"Google Chrome";v="149", "Chromium";v="149", "Not)A;Brand";v="24"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", c.userAgent())
	req.Header.Set("X-ASBD-ID", "129477")
	req.Header.Set("X-IG-App-ID", firstNonEmpty(appID, DefaultAppID))
	req.Header.Set("X-IG-WWW-Claim", "0")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if strings.TrimSpace(cookie) != "" {
		req.Header.Set("Cookie", strings.TrimSpace(cookie))
		if csrf := cookieValue(cookie, "csrftoken"); csrf != "" {
			req.Header.Set("X-CSRFToken", csrf)
		}
	}
}

func parseTitleProfile(title string) (fullName string, username string) {
	title = cleanTitle(title)
	if match := titleUsernameRE.FindStringSubmatch(title); len(match) > 1 {
		username = strings.TrimSpace(match[1])
		fullName = strings.TrimSpace(title[:strings.Index(title, match[0])])
	}
	if username == "" {
		fields := strings.Fields(title)
		for _, field := range fields {
			field = strings.Trim(field, "()[]")
			if strings.HasPrefix(field, "@") && usernameRE.MatchString(strings.TrimPrefix(field, "@")) {
				username = strings.TrimPrefix(field, "@")
				break
			}
		}
	}
	return strings.TrimSpace(fullName), strings.TrimSpace(username)
}

func cleanTitle(title string) string {
	title = strings.TrimSpace(stdhtml.UnescapeString(title))
	for _, sep := range []string{" · Instagram", " - Instagram", " | Instagram"} {
		if idx := strings.Index(title, sep); idx >= 0 {
			title = strings.TrimSpace(title[:idx])
		}
	}
	return title
}

func parseBio(description string) string {
	description = strings.TrimSpace(stdhtml.UnescapeString(description))
	for _, marker := range []string{"：“", ": “", ":\"", ": \""} {
		if idx := strings.Index(description, marker); idx >= 0 {
			bio := strings.TrimSpace(description[idx+len(marker):])
			bio = strings.TrimSuffix(bio, "”")
			bio = strings.TrimSuffix(bio, "\"")
			return strings.TrimSpace(bio)
		}
	}
	return ""
}

func parseProfileCounts(description string) (followers int, following int, mediaCount int) {
	description = strings.ToLower(stdhtml.UnescapeString(description))
	parts := strings.FieldsFunc(description, func(r rune) bool {
		return r == '、' || r == ',' || r == '，' || r == '-' || r == '·' || r == ';' || r == '；'
	})
	for _, part := range parts {
		part = strings.TrimSpace(part)
		value := parseCountValue(part)
		if value == 0 {
			continue
		}
		switch {
		case strings.Contains(part, "followers") || strings.Contains(part, "粉丝") || strings.Contains(part, "位粉"):
			followers = value
		case strings.Contains(part, "following") || strings.Contains(part, "已关注") || strings.Contains(part, "关注"):
			following = value
		case strings.Contains(part, "posts") || strings.Contains(part, "帖子") || strings.Contains(part, "貼文"):
			mediaCount = value
		}
	}
	return followers, following, mediaCount
}

func parseCountValue(text string) int {
	match := countValueRE.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0
	}
	raw := strings.ReplaceAll(match[1], ",", "")
	raw = strings.ReplaceAll(raw, "，", "")
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	switch strings.ToLower(match[2]) {
	case "k":
		value *= 1000
	case "m":
		value *= 1000000
	case "b":
		value *= 1000000000
	case "万":
		value *= 10000
	case "亿":
		value *= 100000000
	}
	if value <= 0 || value > float64(math.MaxInt) {
		return 0
	}
	return int(math.Round(value))
}

func parseAppID(htmlText string) string {
	for _, match := range appIDRE.FindAllStringSubmatch(htmlText, -1) {
		if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
			return strings.TrimSpace(match[1])
		}
	}
	return ""
}

func parseCSRFToken(htmlText string) string {
	if match := csrfTokenRE.FindStringSubmatch(htmlText); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func parseProfileID(htmlText string) string {
	for _, match := range profileIDRE.FindAllStringSubmatch(htmlText, -1) {
		if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
			return strings.TrimSpace(match[1])
		}
	}
	return ""
}

func firstCaption(captions CaptionEnvelope) string {
	for _, edge := range captions.Edges {
		if text := strings.TrimSpace(edge.Node.Text); text != "" {
			return text
		}
	}
	return ""
}

func mergeProfile(base UserProfile, override UserProfile) UserProfile {
	if override.ID != "" {
		base.ID = override.ID
	}
	if override.Username != "" {
		base.Username = override.Username
	}
	if override.FullName != "" {
		base.FullName = override.FullName
	}
	if override.Biography != "" {
		base.Biography = override.Biography
	}
	if override.ExternalURL != "" {
		base.ExternalURL = override.ExternalURL
	}
	if override.ProfilePicURL != "" {
		base.ProfilePicURL = override.ProfilePicURL
	}
	if override.ProfilePicURLHD != "" {
		base.ProfilePicURLHD = override.ProfilePicURLHD
	}
	base.IsPrivate = override.IsPrivate
	base.IsVerified = override.IsVerified
	if override.FollowersCount > 0 {
		base.FollowersCount = override.FollowersCount
	}
	if override.FollowingCount > 0 {
		base.FollowingCount = override.FollowingCount
	}
	if override.MediaCount > 0 {
		base.MediaCount = override.MediaCount
	}
	return base
}

func metaContent(doc *goquery.Document, attrName string, attrValue string) string {
	if doc == nil {
		return ""
	}
	selector := fmt.Sprintf(`meta[%s="%s"]`, attrName, attrValue)
	return attr(doc.Find(selector).First(), "content")
}

func attr(s *goquery.Selection, name string) string {
	if s == nil || s.Length() == 0 {
		return ""
	}
	value, _ := s.Attr(name)
	return strings.TrimSpace(stdhtml.UnescapeString(value))
}

func normalizeURL(rawURL string, baseURL string) string {
	rawURL = strings.TrimSpace(stdhtml.UnescapeString(rawURL))
	if rawURL == "" {
		return ""
	}
	if strings.HasPrefix(rawURL, "//") {
		return "https:" + rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err == nil && parsed.IsAbs() {
		return parsed.String()
	}
	if baseURL == "" {
		baseURL = BaseURL + "/"
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return rawURL
	}
	rel, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return base.ResolveReference(rel).String()
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

func SanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = unsafeFilenameRE.ReplaceAllString(name, "_")
	name = dotsRE.ReplaceAllString(name, ".")
	name = strings.Trim(name, " ._")
	if name == "" {
		return "instagram"
	}
	if len([]rune(name)) > 120 {
		runes := []rune(name)
		name = string(runes[:120])
	}
	return name
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
