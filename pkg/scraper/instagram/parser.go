package instagram

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/tidwall/gjson"
)

type FetchResult struct {
	SourceURL    string   `json:"source_url"`
	ExternalID   string   `json:"external_id"`
	Shortcode    string   `json:"shortcode"`
	BodyText     string   `json:"body_text"`
	PublishTime  int64    `json:"publish_time"` // Unix milliseconds.
	LikeCount    int64    `json:"like_count"`
	CommentCount int64    `json:"comment_count"`
	Account      *Account `json:"account"`
	Media        []Media  `json:"media"` // Original carousel order; one item for a single image/video.
}

type Account struct {
	ExternalID string `json:"external_id"`
	Username   string `json:"username"`
	Nickname   string `json:"nickname"`
	AvatarURL  string `json:"avatar_url"`
	ProfileURL string `json:"profile_url"`
	Biography  string `json:"biography,omitempty"`
	IsVerified bool   `json:"is_verified"`
	IsPrivate  bool   `json:"is_private"`
}

type Media struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // image or video.
	URL      string `json:"url"`
	CoverURL string `json:"cover_url"`
	Width    int64  `json:"width"`
	Height   int64  `json:"height"`
	AltText  string `json:"alt_text,omitempty"`
}

// ParseFetchResult extracts the requested post from saved or fetched HTML.
func ParseFetchResult(source_url string, page_html string) (*FetchResult, error) {
	shortcode, err := ExtractShortcode(source_url)
	if err != nil {
		return nil, err
	}
	post := find_page_record(page_html, func(record gjson.Result) bool {
		return record.Get("code").String() == shortcode && (record.Get("media_type").Exists() || record.Get("if_not_gated_logged_out").Exists())
	})
	if gated := post.Get("if_not_gated_logged_out"); gated.Exists() {
		post = gated
	}
	if !post.IsObject() {
		return nil, fmt.Errorf("instagram: post %s is missing from embedded page data or unavailable", shortcode)
	}
	account, err := parse_account(post.Get("user"))
	if err != nil {
		return nil, err
	}
	result := &FetchResult{
		SourceURL: "https://www.instagram.com/p/" + shortcode + "/", ExternalID: post.Get("pk").String(), Shortcode: shortcode,
		BodyText: post.Get("caption.text").String(), PublishTime: post.Get("taken_at").Int() * 1000,
		LikeCount: post.Get("like_count").Int(), CommentCount: post.Get("comment_count").Int(), Account: account,
	}
	if result.ExternalID == "" {
		return nil, fmt.Errorf("instagram: post %s has no ID", shortcode)
	}
	items := []gjson.Result{post}
	if post.Get("media_type").Int() == 8 {
		items = post.Get("carousel_media").Array()
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("instagram: post %s contains no media", shortcode)
	}
	for _, item := range items {
		media, err := parse_media(item)
		if err != nil {
			return nil, fmt.Errorf("instagram: post %s media %d: %w", shortcode, len(result.Media)+1, err)
		}
		result.Media = append(result.Media, media)
	}
	return result, nil
}

// ParseAccount finds an exact username in embedded JSON, including a post's author.
func ParseAccount(account_url string, page_html string) (*Account, error) {
	username, err := ExtractUsername(account_url)
	if err != nil {
		return nil, err
	}
	record := find_page_record(page_html, func(record gjson.Result) bool {
		return strings.EqualFold(record.Get("username").String(), username) && record.Get("pk").Exists() && record.Get("full_name").Exists()
	})
	return parse_account(record)
}

func parse_account(record gjson.Result) (*Account, error) {
	username, err := ExtractUsername(record.Get("username").String())
	if err != nil || record.Get("pk").String() == "" {
		return nil, fmt.Errorf("instagram: account data is missing from embedded page data or unavailable")
	}
	avatar_url := record.Get("profile_pic_url").String()
	if avatar_url == "" {
		avatar_url = record.Get("profile_image_uri").String()
	}
	return &Account{
		ExternalID: record.Get("pk").String(), Username: username, Nickname: record.Get("full_name").String(),
		AvatarURL: NormalizeMediaURL(avatar_url), ProfileURL: "https://www.instagram.com/" + username + "/",
		Biography: record.Get("biography").String(), IsVerified: record.Get("is_verified").Bool(), IsPrivate: record.Get("is_private").Bool(),
	}, nil
}

func parse_media(record gjson.Result) (Media, error) {
	media := Media{
		ID: record.Get("pk").String(), Width: record.Get("original_width").Int(), Height: record.Get("original_height").Int(),
		AltText: record.Get("accessibility_caption").String(),
	}
	media.CoverURL = best_candidate(record.Get("image_versions2.candidates"))
	if media.CoverURL == "" {
		media.CoverURL = NormalizeMediaURL(record.Get("display_uri").String())
	}
	switch record.Get("media_type").Int() {
	case 1:
		media.Type, media.URL = "image", media.CoverURL
	case 2:
		media.Type, media.URL = "video", best_candidate(record.Get("video_versions"))
		if media.URL == "" {
			media.URL = NormalizeMediaURL(record.Get("video_url").String())
		}
	default:
		return media, fmt.Errorf("unsupported media type %s", record.Get("media_type").Raw)
	}
	if media.URL == "" {
		return media, fmt.Errorf("missing downloadable %s URL", media.Type)
	}
	return media, nil
}

func best_candidate(candidates gjson.Result) string {
	best_url := ""
	best_area := int64(-1)
	for _, candidate := range candidates.Array() {
		candidate_url := NormalizeMediaURL(candidate.Get("url").String())
		area := candidate.Get("width").Int() * candidate.Get("height").Int()
		// Dimensionless candidates in the supplied page are ordered largest first.
		if candidate_url != "" && area > best_area {
			best_url, best_area = candidate_url, area
		}
	}
	return best_url
}

// NormalizeMediaURL accepts HTTPS media URLs on Instagram CDN domains.
func NormalizeMediaURL(raw_url string) string {
	parsed_url, err := url.Parse(raw_url)
	if err != nil || parsed_url.Scheme != "https" || parsed_url.User != nil || (parsed_url.Port() != "" && parsed_url.Port() != "443") {
		return ""
	}
	host := strings.ToLower(parsed_url.Hostname())
	for _, domain := range []string{"cdninstagram.com", "fbcdn.net"} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return parsed_url.String()
		}
	}
	return ""
}

// find_page_record walks JSON script data only; recommendation and comment records
// cannot match unless they carry the requested post code or account username.
func find_page_record(page_html string, match func(gjson.Result) bool) gjson.Result {
	document, err := goquery.NewDocumentFromReader(strings.NewReader(page_html))
	if err != nil {
		return gjson.Result{}
	}
	var found gjson.Result
	var walk func(gjson.Result) bool
	walk = func(record gjson.Result) bool {
		if record.IsObject() && match(record) {
			found = record
			return false
		}
		if record.IsObject() || record.IsArray() {
			record.ForEach(func(_ gjson.Result, child gjson.Result) bool { return walk(child) })
		}
		return !found.Exists()
	}
	document.Find(`script[type="application/json"]`).EachWithBreak(func(_ int, script *goquery.Selection) bool {
		source := script.Text()
		if gjson.Valid(source) {
			return walk(gjson.Parse(source))
		}
		return true
	})
	return found
}
