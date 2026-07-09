package x

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
	BaseURL                    = "https://x.com"
	ContentTypeUserTimeline    = "account"
	defaultUserAgent           = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
	defaultBearerToken         = "AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA"
	defaultGuestActivateURL    = "https://api.x.com/1.1/guest/activate.json"
	defaultUserTweetsOperation = "RyDU3I9VJtPF-Pnl6vrRlw"
)

var (
	ErrUnsupportedURL = errors.New("unsupported x profile url")
	shareURLRE        = regexp.MustCompile(`https?://[^\s"'<>]+`)
	screenNameRE      = regexp.MustCompile(`^[A-Za-z0-9_]{1,15}$`)
	titleProfileRE    = regexp.MustCompile(`(?s)^(.*?)\s*\(@([A-Za-z0-9_]{1,15})\)`)
	bannerUserIDRE    = regexp.MustCompile(`https://pbs\.twimg\.com/profile_banners/([0-9]+)/[0-9]+`)
	restIDRE          = regexp.MustCompile(`"rest_id"\s*:\s*"([0-9]+)"`)
	followersCountRE  = regexp.MustCompile(`"followers_count"\s*:\s*([0-9]+)`)
	friendsCountRE    = regexp.MustCompile(`"friends_count"\s*:\s*([0-9]+)`)
	statusesCountRE   = regexp.MustCompile(`"statuses_count"\s*:\s*([0-9]+)`)
	mediaCountRE      = regexp.MustCompile(`"media_count"\s*:\s*([0-9]+)`)
	unsafeFilenameRE  = regexp.MustCompile(`[\\/:*?"<>|#\n\r]`)
	dotsRE            = regexp.MustCompile(`\.{2,}`)
)

var reservedProfilePaths = map[string]bool{
	"about":         true,
	"account":       true,
	"api":           true,
	"compose":       true,
	"download":      true,
	"explore":       true,
	"graphql":       true,
	"hashtag":       true,
	"help":          true,
	"home":          true,
	"i":             true,
	"intent":        true,
	"jobs":          true,
	"login":         true,
	"logout":        true,
	"messages":      true,
	"notifications": true,
	"oauth":         true,
	"privacy":       true,
	"search":        true,
	"settings":      true,
	"share":         true,
	"signup":        true,
	"tos":           true,
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	HTTPClient            HTTPClient
	BaseURL               string
	GuestActivateURL      string
	UserAgent             string
	Cookie                string
	BearerToken           string
	GuestToken            string
	CSRFToken             string
	UserTweetsOperationID string
}

type ProfileURL struct {
	Username  string `json:"username"`
	Canonical string `json:"canonical"`
}

type TimelineOptions struct {
	Count       int    `json:"count,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
	UserID      string `json:"user_id,omitempty"`
	OperationID string `json:"operation_id,omitempty"`
	Cookie      string `json:"-"`
	BearerToken string `json:"-"`
	GuestToken  string `json:"-"`
	CSRFToken   string `json:"-"`
}

type TimelinePage struct {
	URL          ProfileURL         `json:"url"`
	SourceURL    string             `json:"source_url"`
	PageURL      string             `json:"page_url,omitempty"`
	APIURL       string             `json:"api_url,omitempty"`
	Request      TimelineOptions    `json:"request"`
	Response     UserTweetsResponse `json:"response"`
	RawResponse  json.RawMessage    `json:"raw_response,omitempty"`
	Profile      UserProfile        `json:"profile"`
	Posts        []PostSummary      `json:"posts,omitempty"`
	TopCursor    string             `json:"top_cursor,omitempty"`
	BottomCursor string             `json:"bottom_cursor,omitempty"`
	Warnings     []string           `json:"warnings,omitempty"`
	PageHTML     string             `json:"-"`
}

type UserProfile struct {
	ID              string `json:"id,omitempty"`
	Username        string `json:"username,omitempty"`
	Name            string `json:"name,omitempty"`
	Description     string `json:"description,omitempty"`
	ExternalURL     string `json:"external_url,omitempty"`
	Location        string `json:"location,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
	AvatarURL       string `json:"avatar_url,omitempty"`
	BannerURL       string `json:"banner_url,omitempty"`
	IsBlueVerified  bool   `json:"is_blue_verified,omitempty"`
	Verified        bool   `json:"verified,omitempty"`
	FollowersCount  int    `json:"followers_count,omitempty"`
	FollowingCount  int    `json:"following_count,omitempty"`
	StatusesCount   int    `json:"statuses_count,omitempty"`
	MediaCount      int    `json:"media_count,omitempty"`
	ListedCount     int    `json:"listed_count,omitempty"`
	FavouritesCount int    `json:"favourites_count,omitempty"`
}

type UserSummary struct {
	ID             string `json:"id,omitempty"`
	Username       string `json:"username,omitempty"`
	Name           string `json:"name,omitempty"`
	ProfileURL     string `json:"profile_url,omitempty"`
	AvatarURL      string `json:"avatar_url,omitempty"`
	IsBlueVerified bool   `json:"is_blue_verified,omitempty"`
	Verified       bool   `json:"verified,omitempty"`
}

type PostSummary struct {
	ID            string      `json:"id,omitempty"`
	URL           string      `json:"url,omitempty"`
	Text          string      `json:"text,omitempty"`
	CreatedAt     string      `json:"created_at,omitempty"`
	CreatedTime   int64       `json:"created_time,omitempty"`
	Lang          string      `json:"lang,omitempty"`
	Author        UserSummary `json:"author,omitempty"`
	ImageURLs     []string    `json:"image_urls,omitempty"`
	VideoURLs     []string    `json:"video_urls,omitempty"`
	MediaURLs     []string    `json:"media_urls,omitempty"`
	CoverURL      string      `json:"cover_url,omitempty"`
	ReplyCount    int         `json:"reply_count,omitempty"`
	RetweetCount  int         `json:"retweet_count,omitempty"`
	QuoteCount    int         `json:"quote_count,omitempty"`
	FavoriteCount int         `json:"favorite_count,omitempty"`
	BookmarkCount int         `json:"bookmark_count,omitempty"`
	ViewCount     int         `json:"view_count,omitempty"`
}

type GuestActivateResponse struct {
	GuestToken string `json:"guest_token"`
}

type UserTweetsResponse struct {
	Data   UserTweetsData `json:"data"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

type GraphQLError struct {
	Message string `json:"message,omitempty"`
	Code    int    `json:"code,omitempty"`
}

type UserTweetsData struct {
	User UserTimelineEnvelope `json:"user"`
}

type UserTimelineEnvelope struct {
	Result UserTimelineResult `json:"result"`
}

type UserTimelineResult struct {
	Typename       string           `json:"__typename,omitempty"`
	RestID         string           `json:"rest_id,omitempty"`
	Core           UserCore         `json:"core,omitempty"`
	Legacy         UserLegacy       `json:"legacy,omitempty"`
	Avatar         Avatar           `json:"avatar,omitempty"`
	Location       UserLocation     `json:"location,omitempty"`
	Verification   Verification     `json:"verification,omitempty"`
	IsBlueVerified bool             `json:"is_blue_verified,omitempty"`
	Timeline       TimelineEnvelope `json:"timeline,omitempty"`
}

type TimelineEnvelope struct {
	Timeline Timeline `json:"timeline"`
}

type Timeline struct {
	Instructions []TimelineInstruction `json:"instructions,omitempty"`
	Metadata     TimelineMetadata      `json:"metadata,omitempty"`
}

type TimelineMetadata struct {
	ScribeConfig map[string]any `json:"scribeConfig,omitempty"`
}

type TimelineInstruction struct {
	Type    string          `json:"type,omitempty"`
	Entry   *TimelineEntry  `json:"entry,omitempty"`
	Entries []TimelineEntry `json:"entries,omitempty"`
}

type TimelineEntry struct {
	EntryID   string               `json:"entryId,omitempty"`
	SortIndex string               `json:"sortIndex,omitempty"`
	Content   TimelineEntryContent `json:"content,omitempty"`
}

type TimelineEntryContent struct {
	Typename    string               `json:"__typename,omitempty"`
	EntryType   string               `json:"entryType,omitempty"`
	ItemContent *TimelineItemContent `json:"itemContent,omitempty"`
	Items       []TimelineModuleItem `json:"items,omitempty"`
	Value       string               `json:"value,omitempty"`
	CursorType  string               `json:"cursorType,omitempty"`
}

type TimelineModuleItem struct {
	EntryID string                    `json:"entryId,omitempty"`
	Item    TimelineModuleItemContent `json:"item,omitempty"`
}

type TimelineModuleItemContent struct {
	Content TimelineEntryContent `json:"content,omitempty"`
}

type TimelineItemContent struct {
	Typename         string              `json:"__typename,omitempty"`
	ItemType         string              `json:"itemType,omitempty"`
	TweetDisplayType string              `json:"tweetDisplayType,omitempty"`
	TweetResults     TweetResultEnvelope `json:"tweet_results,omitempty"`
}

type TweetResultEnvelope struct {
	Result *TweetResult `json:"result,omitempty"`
}

type TweetResult struct {
	Typename           string               `json:"__typename,omitempty"`
	RestID             string               `json:"rest_id,omitempty"`
	Tweet              *TweetResult         `json:"tweet,omitempty"`
	Core               TweetCore            `json:"core,omitempty"`
	Legacy             TweetLegacy          `json:"legacy,omitempty"`
	NoteTweet          *NoteTweet           `json:"note_tweet,omitempty"`
	QuotedStatusResult *TweetResultEnvelope `json:"quoted_status_result,omitempty"`
	Views              ViewState            `json:"views,omitempty"`
	Source             string               `json:"source,omitempty"`
}

type TweetCore struct {
	UserResults UserResultEnvelope `json:"user_results,omitempty"`
}

type UserResultEnvelope struct {
	Result *UserResult `json:"result,omitempty"`
}

type UserResult struct {
	Typename       string       `json:"__typename,omitempty"`
	RestID         string       `json:"rest_id,omitempty"`
	ID             string       `json:"id,omitempty"`
	Core           UserCore     `json:"core,omitempty"`
	Legacy         UserLegacy   `json:"legacy,omitempty"`
	Avatar         Avatar       `json:"avatar,omitempty"`
	Location       UserLocation `json:"location,omitempty"`
	Verification   Verification `json:"verification,omitempty"`
	IsBlueVerified bool         `json:"is_blue_verified,omitempty"`
}

type UserCore struct {
	CreatedAt  string `json:"created_at,omitempty"`
	Name       string `json:"name,omitempty"`
	ScreenName string `json:"screen_name,omitempty"`
}

type UserLegacy struct {
	Description      string       `json:"description,omitempty"`
	Entities         UserEntities `json:"entities,omitempty"`
	FollowersCount   int          `json:"followers_count,omitempty"`
	FriendsCount     int          `json:"friends_count,omitempty"`
	FavouritesCount  int          `json:"favourites_count,omitempty"`
	ListedCount      int          `json:"listed_count,omitempty"`
	MediaCount       int          `json:"media_count,omitempty"`
	StatusesCount    int          `json:"statuses_count,omitempty"`
	ProfileBannerURL string       `json:"profile_banner_url,omitempty"`
	URL              string       `json:"url,omitempty"`
}

type UserEntities struct {
	URL URLList `json:"url,omitempty"`
}

type URLList struct {
	URLs []URLEntity `json:"urls,omitempty"`
}

type UserLocation struct {
	Location string `json:"location,omitempty"`
}

type Avatar struct {
	ImageURL string `json:"image_url,omitempty"`
}

type Verification struct {
	Verified bool `json:"verified,omitempty"`
}

type TweetLegacy struct {
	CreatedAt         string        `json:"created_at,omitempty"`
	IDStr             string        `json:"id_str,omitempty"`
	FullText          string        `json:"full_text,omitempty"`
	ConversationIDStr string        `json:"conversation_id_str,omitempty"`
	UserIDStr         string        `json:"user_id_str,omitempty"`
	Lang              string        `json:"lang,omitempty"`
	QuotedStatusIDStr string        `json:"quoted_status_id_str,omitempty"`
	IsQuoteStatus     bool          `json:"is_quote_status,omitempty"`
	FavoriteCount     int           `json:"favorite_count,omitempty"`
	ReplyCount        int           `json:"reply_count,omitempty"`
	RetweetCount      int           `json:"retweet_count,omitempty"`
	QuoteCount        int           `json:"quote_count,omitempty"`
	BookmarkCount     int           `json:"bookmark_count,omitempty"`
	Entities          TweetEntities `json:"entities,omitempty"`
	ExtendedEntities  TweetEntities `json:"extended_entities,omitempty"`
}

type TweetEntities struct {
	Media []MediaEntity `json:"media,omitempty"`
	URLs  []URLEntity   `json:"urls,omitempty"`
}

type URLEntity struct {
	URL         string `json:"url,omitempty"`
	ExpandedURL string `json:"expanded_url,omitempty"`
	DisplayURL  string `json:"display_url,omitempty"`
}

type MediaEntity struct {
	IDStr         string               `json:"id_str,omitempty"`
	MediaKey      string               `json:"media_key,omitempty"`
	Type          string               `json:"type,omitempty"`
	URL           string               `json:"url,omitempty"`
	DisplayURL    string               `json:"display_url,omitempty"`
	ExpandedURL   string               `json:"expanded_url,omitempty"`
	MediaURLHTTPS string               `json:"media_url_https,omitempty"`
	VideoInfo     *VideoInfo           `json:"video_info,omitempty"`
	OriginalInfo  OriginalInfo         `json:"original_info,omitempty"`
	Sizes         map[string]MediaSize `json:"sizes,omitempty"`
}

type OriginalInfo struct {
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}

type MediaSize struct {
	Width  int    `json:"w,omitempty"`
	Height int    `json:"h,omitempty"`
	Resize string `json:"resize,omitempty"`
}

type VideoInfo struct {
	AspectRatio    []int          `json:"aspect_ratio,omitempty"`
	DurationMillis int64          `json:"duration_millis,omitempty"`
	Variants       []VideoVariant `json:"variants,omitempty"`
}

type VideoVariant struct {
	Bitrate     int    `json:"bitrate,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	URL         string `json:"url,omitempty"`
}

type NoteTweet struct {
	IsExpandable     bool             `json:"is_expandable,omitempty"`
	NoteTweetResults NoteTweetResults `json:"note_tweet_results,omitempty"`
}

type NoteTweetResults struct {
	Result NoteTweetResult `json:"result,omitempty"`
}

type NoteTweetResult struct {
	ID   string `json:"id,omitempty"`
	Text string `json:"text,omitempty"`
}

type ViewState struct {
	Count string `json:"count,omitempty"`
	State string `json:"state,omitempty"`
}

type requestAuth struct {
	Cookie      string
	BearerToken string
	GuestToken  string
	CSRFToken   string
}

func NewClient() *Client {
	return &Client{
		HTTPClient:            &http.Client{Timeout: 30 * time.Second},
		BaseURL:               BaseURL,
		GuestActivateURL:      defaultGuestActivateURL,
		UserAgent:             defaultUserAgent,
		BearerToken:           defaultBearerToken,
		UserTweetsOperationID: defaultUserTweetsOperation,
	}
}

func NewClientWithOptions(client HTTPClient, cookie string, bearerToken string, guestToken string, csrfToken string, userAgent string) *Client {
	c := NewClient()
	if client != nil {
		c.HTTPClient = client
	}
	if strings.TrimSpace(cookie) != "" {
		c.Cookie = strings.TrimSpace(cookie)
	}
	if strings.TrimSpace(bearerToken) != "" {
		c.BearerToken = strings.TrimSpace(bearerToken)
	}
	if strings.TrimSpace(guestToken) != "" {
		c.GuestToken = strings.TrimSpace(guestToken)
	}
	if strings.TrimSpace(csrfToken) != "" {
		c.CSRFToken = strings.TrimSpace(csrfToken)
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
	if screenNameRE.MatchString(rawURL) && !reservedProfilePaths[strings.ToLower(rawURL)] {
		return ProfileURL{Username: rawURL, Canonical: CanonicalProfileURL(rawURL)}, true
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ProfileURL{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if !isXHost(host) {
		return ProfileURL{}, false
	}
	if username := strings.TrimPrefix(strings.TrimSpace(parsed.Query().Get("screen_name")), "@"); screenNameRE.MatchString(username) {
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
	if !screenNameRE.MatchString(username) || reservedProfilePaths[strings.ToLower(username)] {
		return ProfileURL{}, false
	}
	if len(segments) >= 2 && strings.EqualFold(segments[1], "status") {
		return ProfileURL{}, false
	}
	return ProfileURL{Username: username, Canonical: CanonicalProfileURL(username)}, true
}

func CanonicalProfileURL(username string) string {
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if username == "" {
		return ""
	}
	return BaseURL + "/" + url.PathEscape(username)
}

func FetchUserTimeline(ctx context.Context, rawURL string, opts TimelineOptions) (*TimelinePage, error) {
	return NewClient().FetchUserTimeline(ctx, rawURL, opts)
}

func (c *Client) FetchUserTimeline(ctx context.Context, rawURL string, opts TimelineOptions) (*TimelinePage, error) {
	target, ok := ParseProfileURL(ExtractShareURL(rawURL))
	if !ok {
		return nil, ErrUnsupportedURL
	}
	if opts.Count <= 0 {
		opts.Count = 20
	}
	if opts.Count > 100 {
		opts.Count = 100
	}
	pageURL := c.profilePageURL(target.Username)
	documentCookie := firstNonEmpty(opts.Cookie, c.Cookie)
	body, finalURL, cookies, err := c.fetchProfilePage(ctx, pageURL, documentCookie)
	if err != nil {
		return nil, err
	}
	page, err := ParseProfilePageHTML(target.Canonical, string(body))
	if err != nil {
		return nil, err
	}
	if page.Profile.ID == "" && strings.TrimSpace(opts.UserID) == "" && documentCookie != "" {
		if fallbackBody, fallbackURL, fallbackCookies, fallbackErr := c.fetchProfilePage(ctx, pageURL, ""); fallbackErr == nil {
			if fallbackPage, parseErr := ParseProfilePageHTML(target.Canonical, string(fallbackBody)); parseErr == nil && fallbackPage.Profile.ID != "" {
				body = fallbackBody
				finalURL = fallbackURL
				cookies = fallbackCookies
				page = fallbackPage
			}
		}
	}
	page.URL = target
	page.SourceURL = rawURL
	page.PageURL = finalURL
	page.PageHTML = string(body)
	page.Request = TimelineOptions{Count: opts.Count, Cursor: opts.Cursor, UserID: opts.UserID, OperationID: opts.OperationID}
	if page.Profile.Username == "" {
		page.Profile.Username = target.Username
	}
	if page.Profile.ID == "" {
		page.Profile.ID = strings.TrimSpace(opts.UserID)
	}
	if page.Profile.ID == "" {
		return nil, fmt.Errorf("fetch x profile: user id not found from %s", target.Canonical)
	}
	auth, err := c.requestAuth(ctx, opts, cookies)
	if err != nil {
		return nil, err
	}
	decoded, apiURL, rawAPI, err := c.fetchUserTweets(ctx, page.Profile.ID, opts, auth, page.URL.Canonical)
	if err != nil {
		return nil, err
	}
	page.APIURL = apiURL
	page.Response = *decoded
	page.RawResponse = rawAPI
	page.Profile = mergeProfile(page.Profile, decoded.UserProfile())
	if page.Profile.ID == "" {
		page.Profile.ID = page.Request.UserID
	}
	if page.Profile.Username == "" {
		page.Profile.Username = target.Username
	}
	page.Posts = decoded.Posts()
	page.TopCursor, page.BottomCursor = decoded.Cursors()
	if len(decoded.Errors) > 0 {
		for _, item := range decoded.Errors {
			if item.Message != "" {
				page.Warnings = append(page.Warnings, item.Message)
			}
		}
	}
	if len(page.Posts) == 0 {
		return nil, fmt.Errorf("fetch x user tweets: empty timeline")
	}
	return page, nil
}

func ParseProfilePageHTML(rawURL string, htmlText string) (*TimelinePage, error) {
	target, _ := ParseProfileURL(rawURL)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	title := cleanTitle(firstNonEmpty(
		metaContent(doc, "property", "og:title"),
		metaContent(doc, "name", "title"),
		doc.Find("title").First().Text(),
	))
	description := strings.TrimSpace(stdhtml.UnescapeString(firstNonEmpty(
		metaContent(doc, "name", "description"),
		metaContent(doc, "property", "og:description"),
	)))
	canonical := normalizeURL(firstNonEmpty(
		metaContent(doc, "property", "og:url"),
		attr(doc.Find(`link[rel="canonical"]`).First(), "href"),
		target.Canonical,
	), target.Canonical)
	if parsed, ok := ParseProfileURL(canonical); ok {
		target = parsed
	}
	name, username := parseTitleProfile(title)
	if username == "" {
		username = firstNonEmpty(target.Username, screenNameFromAppURL(doc))
	}
	avatarURL := normalizeURL(metaContent(doc, "property", "og:image"), canonical)
	bannerURL, userID := parseBanner(htmlText)
	if userID == "" {
		userID = firstRegexpGroup(restIDRE, htmlText)
	}
	page := &TimelinePage{
		URL:       target,
		SourceURL: rawURL,
		PageURL:   canonical,
		Profile: UserProfile{
			ID:             userID,
			Username:       username,
			Name:           firstNonEmpty(name, username),
			Description:    description,
			AvatarURL:      avatarURL,
			BannerURL:      bannerURL,
			FollowersCount: firstRegexpInt(followersCountRE, htmlText),
			FollowingCount: firstRegexpInt(friendsCountRE, htmlText),
			StatusesCount:  firstRegexpInt(statusesCountRE, htmlText),
			MediaCount:     firstRegexpInt(mediaCountRE, htmlText),
		},
		PageHTML: htmlText,
	}
	if page.URL.Canonical == "" {
		page.URL.Canonical = canonical
	}
	if page.Profile.Username == "" && page.URL.Username != "" {
		page.Profile.Username = page.URL.Username
	}
	return page, nil
}

func ParseUserTweetsResponse(body []byte) (*UserTweetsResponse, error) {
	var decoded UserTweetsResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	return &decoded, nil
}

func (r *UserTweetsResponse) UserProfile() UserProfile {
	if r == nil {
		return UserProfile{}
	}
	if profile := r.Data.User.Result.Profile(); profile.ID != "" || profile.Username != "" {
		return profile
	}
	for _, entry := range r.entries() {
		tweet := entry.Tweet()
		if tweet == nil {
			continue
		}
		if user := tweet.User(); user != nil {
			return user.Profile()
		}
	}
	return UserProfile{}
}

func (r *UserTweetsResponse) Posts() []PostSummary {
	if r == nil {
		return nil
	}
	seen := map[string]bool{}
	out := make([]PostSummary, 0)
	for _, entry := range r.entries() {
		tweet := entry.Tweet()
		if tweet == nil {
			continue
		}
		post := SummarizeTweet(*tweet)
		if post.ID == "" || seen[post.ID] {
			continue
		}
		seen[post.ID] = true
		out = append(out, post)
	}
	return out
}

func (r *UserTweetsResponse) Cursors() (top string, bottom string) {
	if r == nil {
		return "", ""
	}
	for _, entry := range r.entries() {
		switch strings.ToLower(entry.Content.CursorType) {
		case "top":
			top = firstNonEmpty(top, entry.Content.Value)
		case "bottom":
			bottom = firstNonEmpty(bottom, entry.Content.Value)
		}
	}
	return top, bottom
}

func (r *UserTweetsResponse) entries() []TimelineEntry {
	if r == nil {
		return nil
	}
	var out []TimelineEntry
	for _, instruction := range r.Data.User.Result.Timeline.Timeline.Instructions {
		if instruction.Entry != nil {
			out = append(out, *instruction.Entry)
		}
		out = append(out, instruction.Entries...)
	}
	return out
}

func (e TimelineEntry) Tweet() *TweetResult {
	if e.Content.ItemContent != nil {
		return e.Content.ItemContent.TweetResults.Result.normalized()
	}
	for _, item := range e.Content.Items {
		if item.Item.Content.ItemContent == nil {
			continue
		}
		if tweet := item.Item.Content.ItemContent.TweetResults.Result.normalized(); tweet != nil {
			return tweet
		}
	}
	return nil
}

func (t *TweetResult) normalized() *TweetResult {
	if t == nil {
		return nil
	}
	if t.Tweet != nil {
		return t.Tweet.normalized()
	}
	if t.Legacy.IDStr == "" && t.RestID == "" {
		return nil
	}
	return t
}

func (t *TweetResult) User() *UserResult {
	if t == nil {
		return nil
	}
	return t.Core.UserResults.Result
}

func (u UserTimelineResult) Profile() UserProfile {
	return UserProfile{
		ID:              u.RestID,
		Username:        u.Core.ScreenName,
		Name:            u.Core.Name,
		Description:     u.Legacy.Description,
		ExternalURL:     firstExpandedURL(u.Legacy.Entities.URL.URLs),
		Location:        u.Location.Location,
		CreatedAt:       u.Core.CreatedAt,
		AvatarURL:       u.Avatar.ImageURL,
		BannerURL:       u.Legacy.ProfileBannerURL,
		IsBlueVerified:  u.IsBlueVerified,
		Verified:        u.Verification.Verified,
		FollowersCount:  u.Legacy.FollowersCount,
		FollowingCount:  u.Legacy.FriendsCount,
		StatusesCount:   u.Legacy.StatusesCount,
		MediaCount:      u.Legacy.MediaCount,
		ListedCount:     u.Legacy.ListedCount,
		FavouritesCount: u.Legacy.FavouritesCount,
	}
}

func (u UserResult) Profile() UserProfile {
	return UserProfile{
		ID:              u.RestID,
		Username:        u.Core.ScreenName,
		Name:            u.Core.Name,
		Description:     u.Legacy.Description,
		ExternalURL:     firstExpandedURL(u.Legacy.Entities.URL.URLs),
		Location:        u.Location.Location,
		CreatedAt:       u.Core.CreatedAt,
		AvatarURL:       u.Avatar.ImageURL,
		BannerURL:       u.Legacy.ProfileBannerURL,
		IsBlueVerified:  u.IsBlueVerified,
		Verified:        u.Verification.Verified,
		FollowersCount:  u.Legacy.FollowersCount,
		FollowingCount:  u.Legacy.FriendsCount,
		StatusesCount:   u.Legacy.StatusesCount,
		MediaCount:      u.Legacy.MediaCount,
		ListedCount:     u.Legacy.ListedCount,
		FavouritesCount: u.Legacy.FavouritesCount,
	}
}

func (u UserResult) Summary() UserSummary {
	profile := u.Profile()
	return UserSummary{
		ID:             profile.ID,
		Username:       profile.Username,
		Name:           profile.Name,
		ProfileURL:     CanonicalProfileURL(profile.Username),
		AvatarURL:      profile.AvatarURL,
		IsBlueVerified: profile.IsBlueVerified,
		Verified:       profile.Verified,
	}
}

func SummarizeTweet(tweet TweetResult) PostSummary {
	normalized := (&tweet).normalized()
	if normalized == nil {
		return PostSummary{}
	}
	tweet = *normalized
	legacy := tweet.Legacy
	id := firstNonEmpty(legacy.IDStr, tweet.RestID)
	user := UserSummary{}
	if result := tweet.User(); result != nil {
		user = result.Summary()
	}
	text := firstNonEmpty(noteTweetText(tweet.NoteTweet), legacy.FullText)
	text = expandTweetText(text, legacy.Entities.URLs)
	imageURLs, videoURLs, mediaURLs := summarizeMedia(firstNonEmptyMedia(legacy.ExtendedEntities.Media, legacy.Entities.Media))
	coverURL := firstString(imageURLs)
	if coverURL == "" {
		coverURL = firstMediaCover(firstNonEmptyMedia(legacy.ExtendedEntities.Media, legacy.Entities.Media))
	}
	return PostSummary{
		ID:            id,
		URL:           StatusURL(user.Username, id),
		Text:          text,
		CreatedAt:     legacy.CreatedAt,
		CreatedTime:   parseTwitterTime(legacy.CreatedAt),
		Lang:          legacy.Lang,
		Author:        user,
		ImageURLs:     imageURLs,
		VideoURLs:     videoURLs,
		MediaURLs:     mediaURLs,
		CoverURL:      coverURL,
		ReplyCount:    legacy.ReplyCount,
		RetweetCount:  legacy.RetweetCount,
		QuoteCount:    legacy.QuoteCount,
		FavoriteCount: legacy.FavoriteCount,
		BookmarkCount: legacy.BookmarkCount,
		ViewCount:     parseCount(tweet.Views.Count),
	}
}

func StatusURL(username string, id string) string {
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	id = strings.TrimSpace(id)
	if username == "" || id == "" {
		return ""
	}
	return BaseURL + "/" + url.PathEscape(username) + "/status/" + url.PathEscape(id)
}

func SanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = unsafeFilenameRE.ReplaceAllString(name, "_")
	name = dotsRE.ReplaceAllString(name, ".")
	name = strings.Trim(name, " ._")
	if name == "" {
		return "x"
	}
	if len([]rune(name)) > 120 {
		runes := []rune(name)
		name = string(runes[:120])
	}
	return name
}

func (c *Client) fetchProfilePage(ctx context.Context, rawURL string, cookie string) ([]byte, string, []*http.Cookie, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, rawURL, nil, err
	}
	c.setDocumentHeaders(req, cookie)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, rawURL, nil, fmt.Errorf("fetch x profile page: %w", err)
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
		return nil, finalURL, resp.Cookies(), fmt.Errorf("fetch x profile page: HTTP %d: %s", resp.StatusCode, trimBody(body))
	}
	return body, finalURL, resp.Cookies(), nil
}

func (c *Client) requestAuth(ctx context.Context, opts TimelineOptions, cookies []*http.Cookie) (requestAuth, error) {
	cookie := mergeCookieHeaders(c.Cookie, opts.Cookie, cookiesHeader(cookies))
	auth := requestAuth{
		Cookie:      cookie,
		BearerToken: firstNonEmpty(opts.BearerToken, c.BearerToken, defaultBearerToken),
		GuestToken:  firstNonEmpty(opts.GuestToken, c.GuestToken, cookieValue(cookie, "gt")),
		CSRFToken:   firstNonEmpty(opts.CSRFToken, c.CSRFToken, cookieValue(cookie, "ct0")),
	}
	if auth.GuestToken == "" && !strings.Contains(cookie, "auth_token=") {
		guestToken, err := c.activateGuest(ctx, auth.BearerToken, cookie)
		if err != nil {
			return auth, err
		}
		auth.GuestToken = guestToken
	}
	return auth, nil
}

func (c *Client) activateGuest(ctx context.Context, bearerToken string, cookie string) (string, error) {
	endpoint := firstNonEmpty(c.GuestActivateURL, defaultGuestActivateURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+firstNonEmpty(bearerToken, defaultBearerToken))
	req.Header.Set("User-Agent", c.userAgent())
	if strings.TrimSpace(cookie) != "" {
		req.Header.Set("Cookie", strings.TrimSpace(cookie))
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("activate x guest token: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("activate x guest token: HTTP %d: %s", resp.StatusCode, trimBody(body))
	}
	var decoded GuestActivateResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", fmt.Errorf("decode x guest token: %w", err)
	}
	if strings.TrimSpace(decoded.GuestToken) == "" {
		return "", fmt.Errorf("activate x guest token: empty guest token")
	}
	return strings.TrimSpace(decoded.GuestToken), nil
}

func (c *Client) fetchUserTweets(ctx context.Context, userID string, opts TimelineOptions, auth requestAuth, referer string) (*UserTweetsResponse, string, json.RawMessage, error) {
	apiURL, err := c.userTweetsURL(userID, opts)
	if err != nil {
		return nil, "", nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, apiURL, nil, err
	}
	c.setAPIHeaders(req, auth, referer)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, apiURL, nil, fmt.Errorf("fetch x user tweets: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apiURL, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apiURL, nil, fmt.Errorf("fetch x user tweets: HTTP %d: %s", resp.StatusCode, trimBody(body))
	}
	decoded, err := ParseUserTweetsResponse(body)
	if err != nil {
		return nil, apiURL, nil, fmt.Errorf("decode x user tweets: %w", err)
	}
	if len(decoded.Errors) > 0 && len(decoded.Posts()) == 0 {
		return nil, apiURL, nil, fmt.Errorf("fetch x user tweets: %s", decoded.Errors[0].Message)
	}
	return decoded, apiURL, json.RawMessage(append([]byte(nil), body...)), nil
}

func (c *Client) profilePageURL(username string) string {
	return strings.TrimRight(firstNonEmpty(c.BaseURL, BaseURL), "/") + "/" + url.PathEscape(strings.TrimPrefix(username, "@"))
}

func (c *Client) userTweetsURL(userID string, opts TimelineOptions) (string, error) {
	base := strings.TrimRight(firstNonEmpty(c.BaseURL, BaseURL), "/")
	operationID := firstNonEmpty(opts.OperationID, c.UserTweetsOperationID, defaultUserTweetsOperation)
	parsed, err := url.Parse(base + "/i/api/graphql/" + url.PathEscape(operationID) + "/UserTweets")
	if err != nil {
		return "", err
	}
	variables := map[string]any{
		"userId":                                 strings.TrimSpace(userID),
		"count":                                  opts.Count,
		"includePromotedContent":                 true,
		"withQuickPromoteEligibilityTweetFields": true,
		"withVoice":                              true,
	}
	if strings.TrimSpace(opts.Cursor) != "" {
		variables["cursor"] = strings.TrimSpace(opts.Cursor)
	}
	variablesJSON, _ := json.Marshal(variables)
	featuresJSON, _ := json.Marshal(userTweetsFeatures())
	fieldTogglesJSON, _ := json.Marshal(map[string]any{"withArticlePlainText": false})
	query := parsed.Query()
	query.Set("variables", string(variablesJSON))
	query.Set("features", string(featuresJSON))
	query.Set("fieldToggles", string(fieldTogglesJSON))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
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

func (c *Client) setAPIHeaders(req *http.Request, auth requestAuth, referer string) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Authorization", "Bearer "+firstNonEmpty(auth.BearerToken, defaultBearerToken))
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", firstNonEmpty(referer, BaseURL+"/"))
	req.Header.Set("Sec-Ch-Ua", `"Google Chrome";v="149", "Chromium";v="149", "Not)A;Brand";v="24"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", c.userAgent())
	req.Header.Set("X-Twitter-Active-User", "yes")
	req.Header.Set("X-Twitter-Client-Language", "en")
	if strings.TrimSpace(auth.GuestToken) != "" {
		req.Header.Set("X-Guest-Token", strings.TrimSpace(auth.GuestToken))
	}
	if strings.TrimSpace(auth.Cookie) != "" {
		req.Header.Set("Cookie", strings.TrimSpace(auth.Cookie))
	}
	if strings.TrimSpace(auth.CSRFToken) != "" {
		req.Header.Set("X-CSRF-Token", strings.TrimSpace(auth.CSRFToken))
	}
	if strings.Contains(auth.Cookie, "auth_token=") {
		req.Header.Set("X-Twitter-Auth-Type", "OAuth2Session")
	}
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

func userTweetsFeatures() map[string]any {
	return map[string]any{
		"rweb_video_screen_enabled":                                               false,
		"rweb_cashtags_enabled":                                                   true,
		"profile_label_improvements_pcf_label_in_post_enabled":                    true,
		"responsive_web_profile_redirect_enabled":                                 false,
		"rweb_tipjar_consumption_enabled":                                         false,
		"verified_phone_label_enabled":                                            false,
		"creator_subscriptions_tweet_preview_api_enabled":                         true,
		"responsive_web_graphql_timeline_navigation_enabled":                      true,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled":       false,
		"premium_content_api_read_enabled":                                        false,
		"communities_web_enable_tweet_community_results_fetch":                    true,
		"c9s_tweet_anatomy_moderator_badge_enabled":                               true,
		"responsive_web_grok_analyze_button_fetch_trends_enabled":                 false,
		"responsive_web_grok_analyze_post_followups_enabled":                      true,
		"rweb_cashtags_composer_attachment_enabled":                               true,
		"responsive_web_jetfuel_frame":                                            true,
		"responsive_web_grok_share_attachment_enabled":                            true,
		"responsive_web_grok_annotations_enabled":                                 true,
		"articles_preview_enabled":                                                true,
		"responsive_web_edit_tweet_api_enabled":                                   true,
		"rweb_conversational_replies_downvote_enabled":                            false,
		"graphql_is_translatable_rweb_tweet_is_translatable_enabled":              true,
		"view_counts_everywhere_api_enabled":                                      true,
		"longform_notetweets_consumption_enabled":                                 true,
		"responsive_web_twitter_article_tweet_consumption_enabled":                true,
		"content_disclosure_indicator_enabled":                                    true,
		"content_disclosure_ai_generated_indicator_enabled":                       true,
		"responsive_web_grok_show_grok_translated_post":                           true,
		"responsive_web_grok_analysis_button_from_backend":                        true,
		"post_ctas_fetch_enabled":                                                 false,
		"freedom_of_speech_not_reach_fetch_enabled":                               true,
		"standardized_nudges_misinfo":                                             true,
		"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
		"longform_notetweets_rich_text_read_enabled":                              true,
		"longform_notetweets_inline_media_enabled":                                false,
		"responsive_web_grok_image_annotation_enabled":                            true,
		"responsive_web_grok_imagine_annotation_enabled":                          true,
		"responsive_web_grok_community_note_auto_translation_is_enabled":          true,
		"responsive_web_enhance_cards_enabled":                                    false,
	}
}

func isXHost(host string) bool {
	switch host {
	case "x.com", "www.x.com", "mobile.x.com", "twitter.com", "www.twitter.com", "mobile.twitter.com":
		return true
	default:
		return false
	}
}

func cleanTitle(title string) string {
	title = strings.TrimSpace(stdhtml.UnescapeString(title))
	for _, sep := range []string{" / X", " on X", " - X", " | X", " / Twitter", " on Twitter"} {
		if idx := strings.Index(title, sep); idx >= 0 {
			title = strings.TrimSpace(title[:idx])
		}
	}
	return title
}

func parseTitleProfile(title string) (name string, username string) {
	title = cleanTitle(title)
	if match := titleProfileRE.FindStringSubmatch(title); len(match) > 2 {
		return strings.TrimSpace(match[1]), strings.TrimSpace(match[2])
	}
	fields := strings.Fields(title)
	for _, field := range fields {
		field = strings.Trim(field, "()[]")
		if strings.HasPrefix(field, "@") && screenNameRE.MatchString(strings.TrimPrefix(field, "@")) {
			username = strings.TrimPrefix(field, "@")
			break
		}
	}
	return strings.TrimSpace(name), strings.TrimSpace(username)
}

func screenNameFromAppURL(doc *goquery.Document) string {
	if doc == nil {
		return ""
	}
	for _, key := range []string{"al:ios:url", "al:android:url"} {
		raw := metaContent(doc, "property", key)
		parsed, err := url.Parse(raw)
		if err != nil || parsed == nil {
			continue
		}
		username := strings.TrimPrefix(strings.TrimSpace(parsed.Query().Get("screen_name")), "@")
		if screenNameRE.MatchString(username) {
			return username
		}
	}
	return ""
}

func parseBanner(htmlText string) (bannerURL string, userID string) {
	match := bannerUserIDRE.FindStringSubmatch(htmlText)
	if len(match) < 2 {
		return "", ""
	}
	return match[0], match[1]
}

func mergeProfile(base UserProfile, override UserProfile) UserProfile {
	if override.ID != "" {
		base.ID = override.ID
	}
	if override.Username != "" {
		base.Username = override.Username
	}
	if override.Name != "" {
		base.Name = override.Name
	}
	if override.Description != "" {
		base.Description = override.Description
	}
	if override.ExternalURL != "" {
		base.ExternalURL = override.ExternalURL
	}
	if override.Location != "" {
		base.Location = override.Location
	}
	if override.CreatedAt != "" {
		base.CreatedAt = override.CreatedAt
	}
	if override.AvatarURL != "" {
		base.AvatarURL = override.AvatarURL
	}
	if override.BannerURL != "" {
		base.BannerURL = override.BannerURL
	}
	base.IsBlueVerified = base.IsBlueVerified || override.IsBlueVerified
	base.Verified = base.Verified || override.Verified
	if override.FollowersCount > 0 {
		base.FollowersCount = override.FollowersCount
	}
	if override.FollowingCount > 0 {
		base.FollowingCount = override.FollowingCount
	}
	if override.StatusesCount > 0 {
		base.StatusesCount = override.StatusesCount
	}
	if override.MediaCount > 0 {
		base.MediaCount = override.MediaCount
	}
	if override.ListedCount > 0 {
		base.ListedCount = override.ListedCount
	}
	if override.FavouritesCount > 0 {
		base.FavouritesCount = override.FavouritesCount
	}
	return base
}

func summarizeMedia(media []MediaEntity) (imageURLs []string, videoURLs []string, mediaURLs []string) {
	seen := map[string]bool{}
	add := func(target *[]string, rawURL string) {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" || seen[rawURL] {
			return
		}
		seen[rawURL] = true
		*target = append(*target, rawURL)
		mediaURLs = append(mediaURLs, rawURL)
	}
	for _, item := range media {
		switch strings.ToLower(item.Type) {
		case "photo":
			add(&imageURLs, item.MediaURLHTTPS)
		case "video", "animated_gif":
			if videoURL := item.BestVideoURL(); videoURL != "" {
				add(&videoURLs, videoURL)
			}
			if item.MediaURLHTTPS != "" && !seen[item.MediaURLHTTPS] {
				seen[item.MediaURLHTTPS] = true
				mediaURLs = append(mediaURLs, item.MediaURLHTTPS)
			}
		default:
			add(&imageURLs, item.MediaURLHTTPS)
		}
	}
	return imageURLs, videoURLs, mediaURLs
}

func firstMediaCover(media []MediaEntity) string {
	for _, item := range media {
		if strings.TrimSpace(item.MediaURLHTTPS) != "" {
			return strings.TrimSpace(item.MediaURLHTTPS)
		}
	}
	return ""
}

func (m MediaEntity) BestVideoURL() string {
	if m.VideoInfo == nil {
		return ""
	}
	bestURL := ""
	bestBitrate := -1
	for _, variant := range m.VideoInfo.Variants {
		if !strings.Contains(strings.ToLower(variant.ContentType), "mp4") || strings.TrimSpace(variant.URL) == "" {
			continue
		}
		if variant.Bitrate > bestBitrate {
			bestBitrate = variant.Bitrate
			bestURL = strings.TrimSpace(variant.URL)
		}
	}
	if bestURL != "" {
		return bestURL
	}
	for _, variant := range m.VideoInfo.Variants {
		if strings.TrimSpace(variant.URL) != "" {
			return strings.TrimSpace(variant.URL)
		}
	}
	return ""
}

func noteTweetText(note *NoteTweet) string {
	if note == nil {
		return ""
	}
	return strings.TrimSpace(note.NoteTweetResults.Result.Text)
}

func expandTweetText(text string, urls []URLEntity) string {
	text = strings.TrimSpace(text)
	for _, item := range urls {
		if item.URL == "" || item.ExpandedURL == "" {
			continue
		}
		text = strings.ReplaceAll(text, item.URL, item.ExpandedURL)
	}
	return text
}

func firstNonEmptyMedia(values ...[]MediaEntity) []MediaEntity {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}

func firstExpandedURL(urls []URLEntity) string {
	for _, item := range urls {
		if strings.TrimSpace(item.ExpandedURL) != "" {
			return strings.TrimSpace(item.ExpandedURL)
		}
		if strings.TrimSpace(item.URL) != "" {
			return strings.TrimSpace(item.URL)
		}
	}
	return ""
}

func parseTwitterTime(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if t, err := time.Parse("Mon Jan 02 15:04:05 -0700 2006", value); err == nil {
		return t.Unix()
	}
	return 0
}

func parseCount(value string) int {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	if value == "" {
		return 0
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 || n > int64(math.MaxInt) {
		return 0
	}
	return int(n)
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
	if strings.HasPrefix(rawURL, "/") {
		base, err := url.Parse(baseURL)
		if err == nil && base != nil && base.Scheme != "" && base.Host != "" {
			base.Path = rawURL
			base.RawQuery = ""
			base.Fragment = ""
			return base.String()
		}
		return strings.TrimRight(BaseURL, "/") + rawURL
	}
	return rawURL
}

func mergeCookieHeaders(values ...string) string {
	out := map[string]string{}
	order := []string{}
	for _, header := range values {
		for _, part := range strings.Split(header, ";") {
			part = strings.TrimSpace(part)
			if part == "" || !strings.Contains(part, "=") {
				continue
			}
			name, value, _ := strings.Cut(part, "=")
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if _, ok := out[name]; !ok {
				order = append(order, name)
			}
			out[name] = strings.TrimSpace(value)
		}
	}
	parts := make([]string, 0, len(order))
	for _, name := range order {
		parts = append(parts, name+"="+out[name])
	}
	return strings.Join(parts, "; ")
}

func cookiesHeader(cookies []*http.Cookie) string {
	if len(cookies) == 0 {
		return ""
	}
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

func firstRegexpGroup(re *regexp.Regexp, text string) string {
	if re == nil {
		return ""
	}
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func firstRegexpInt(re *regexp.Regexp, text string) int {
	return parseCount(firstRegexpGroup(re, text))
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
