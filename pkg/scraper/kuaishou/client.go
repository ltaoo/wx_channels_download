package kuaishou

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	default_request_timeout = 30 * time.Second
	default_response_limit  = int64(8 << 20)
	max_redirect_count      = 10
)

var (
	http_url_pattern = regexp.MustCompile(`https?://[a-zA-Z0-9._~:/?#\[\]@!$&'()*+,;=%-]+`)
	hashtag_pattern  = regexp.MustCompile(`#[^\s#]+`)
)

const trailing_url_punctuation = `.,!?;:)]}"'`

const apollo_state_variable = "window.__APOLLO_STATE__"

// Client resolves Kuaishou links and requests their public video metadata.
type Client struct {
	http_client    *http.Client
	response_limit int64
	cookie         string
}

// NewClient creates a Kuaishou client with a cookie jar and a 30-second
// timeout.
func NewClient() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		http_client: &http.Client{
			Timeout:       default_request_timeout,
			CheckRedirect: stop_automatic_redirect,
			Jar:           jar,
		},
		response_limit: default_response_limit,
	}
}

// NewClientWithHTTPClient creates a Kuaishou client using the supplied
// transport, cookie jar, and timeout while retaining explicit redirect
// handling.
func NewClientWithHTTPClient(http_client *http.Client) *Client {
	if http_client == nil {
		return NewClient()
	}
	jar := http_client.Jar
	if jar == nil {
		jar, _ = cookiejar.New(nil)
	}
	return &Client{
		http_client: &http.Client{
			Transport:     http_client.Transport,
			CheckRedirect: stop_automatic_redirect,
			Jar:           jar,
			Timeout:       http_client.Timeout,
		},
		response_limit: default_response_limit,
	}
}

// Close releases idle HTTP connections held by the client.
func (c *Client) Close() {
	if c == nil || c.http_client == nil {
		return
	}
	c.http_client.CloseIdleConnections()
}

// SetCookie sets an optional Cookie header for kuaishou.com requests.
func (c *Client) SetCookie(cookie string) {
	if c != nil {
		c.cookie = strings.TrimSpace(cookie)
	}
}

// ExtractURL extracts the first supported Kuaishou URL from a URL or copied
// share text.
func ExtractURL(content string) (string, error) {
	for _, candidate_url := range http_url_pattern.FindAllString(strings.TrimSpace(content), -1) {
		candidate_url = strings.TrimRight(candidate_url, trailing_url_punctuation)
		parsed_url, err := url.Parse(candidate_url)
		if err == nil && validate_input_url(parsed_url) == nil {
			return candidate_url, nil
		}
	}
	return "", errors.New("kuaishou URL not found")
}

// Fetch resolves and retrieves one Kuaishou video.
func (c *Client) Fetch(raw_url string) (*FetchResult, error) {
	return c.FetchContext(context.Background(), raw_url)
}

// FetchContext resolves and retrieves one Kuaishou video with cancellation.
func (c *Client) FetchContext(fetch_context context.Context, raw_url string) (*FetchResult, error) {
	if c == nil || c.http_client == nil {
		return nil, errors.New("kuaishou client is not initialized")
	}
	if fetch_context == nil {
		fetch_context = context.Background()
	}
	source_url, err := ExtractURL(raw_url)
	if err != nil {
		return nil, err
	}
	request_url, err := url.Parse(source_url)
	if err != nil {
		return nil, fmt.Errorf("kuaishou: parse URL: %w", err)
	}
	page_url, photo_id, page_html, err := c.resolve_page(fetch_context, request_url)
	if err != nil {
		return nil, err
	}
	feed, err := parse_feed_from_html(page_html, photo_id)
	if err != nil {
		return nil, err
	}
	return validate_fetch_result(&FetchResult{
		SourceURL: source_url,
		PageURL:   page_url.String(),
		PhotoID:   photo_id,
		Feed:      *feed,
	})
}

func (c *Client) resolve_page(fetch_context context.Context, request_url *url.URL) (*url.URL, string, []byte, error) {
	current_url := request_url
	for redirect_count := 0; ; redirect_count++ {
		request, err := http.NewRequestWithContext(fetch_context, http.MethodGet, current_url.String(), nil)
		if err != nil {
			return nil, "", nil, fmt.Errorf("kuaishou: create page request: %w", err)
		}
		set_navigation_headers(request.Header)
		c.set_kuaishou_cookie(request)
		response, err := c.http_client.Do(request)
		if err != nil {
			return nil, "", nil, fmt.Errorf("kuaishou: fetch %q: %w", current_url.String(), err)
		}
		if is_redirect_status(response.StatusCode) {
			if redirect_count >= max_redirect_count {
				response.Body.Close()
				return nil, "", nil, fmt.Errorf("kuaishou: stopped after %d redirects", max_redirect_count)
			}
			location := strings.TrimSpace(response.Header.Get("Location"))
			response.Body.Close()
			if location == "" {
				return nil, "", nil, fmt.Errorf("kuaishou: redirect from %q is missing Location", current_url.String())
			}
			next_url, parse_err := current_url.Parse(location)
			if parse_err != nil {
				return nil, "", nil, fmt.Errorf("kuaishou: invalid redirect Location %q: %w", location, parse_err)
			}
			if validate_err := validate_redirect_url(next_url); validate_err != nil {
				return nil, "", nil, validate_err
			}
			current_url = next_url
			continue
		}

		response_body, read_err := io.ReadAll(io.LimitReader(response.Body, c.response_limit+1))
		response.Body.Close()
		if read_err != nil {
			return nil, "", nil, fmt.Errorf("kuaishou: read page response: %w", read_err)
		}
		if int64(len(response_body)) > c.response_limit {
			return nil, "", nil, fmt.Errorf("kuaishou: page response exceeds %d bytes", c.response_limit)
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			return nil, "", nil, fmt.Errorf("kuaishou: page returned HTTP %d", response.StatusCode)
		}
		photo_id, extract_err := extract_photo_id(current_url)
		if extract_err != nil {
			return nil, "", nil, extract_err
		}
		return current_url, photo_id, response_body, nil
	}
}

// parse_feed_from_html extracts the SSR-rendered video metadata embedded in a
// Kuaishou short-video detail page, avoiding the client-side GraphQL request
// that is frequently rate-limited or slow.
func parse_feed_from_html(page_html []byte, photo_id string) (*Feed, error) {
	document, err := html.Parse(bytes.NewReader(page_html))
	if err != nil {
		return nil, fmt.Errorf("kuaishou: parse page HTML: %w", err)
	}
	feed := &Feed{Type: "VIDEO"}
	feed.Photo.ID = strings.TrimSpace(photo_id)

	var title string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			switch node.Data {
			case "video":
				if feed.Photo.PhotoURL == "" {
					feed.Photo.PhotoURL = html_attribute_value(node, "src")
				}
				if feed.Photo.CoverURL == "" {
					feed.Photo.CoverURL = html_attribute_value(node, "poster")
				}
			case "div":
				if feed.Photo.CoverURL == "" {
					feed.Photo.CoverURL = html_attribute_value(node, "poster")
				}
			case "img":
				if feed.Author.HeaderURL == "" && node_has_class(node, "avatar-img") {
					feed.Author.HeaderURL = html_attribute_value(node, "src")
				}
			case "a":
				href := html_attribute_value(node, "href")
				if feed.Author.ID == "" {
					if author_id, ok := profile_id_from_href(href); ok {
						feed.Author.ID = author_id
					}
				}
				if tag_name, ok := hashtag_from_href(href); ok {
					feed.Tags = append(feed.Tags, Tag{Name: tag_name})
				}
			case "span":
				if feed.Author.Name == "" && node_has_class(node, "profile-user-name-title") {
					feed.Author.Name = strings.TrimSpace(node_text(node))
				}
			case "title":
				if title == "" {
					title = strings.TrimSpace(node_text(node))
				}
			case "script":
				apollo_feed, apollo_err := feed_from_apollo_state(node_text(node), photo_id)
				if apollo_err == nil && apollo_feed != nil {
					merge_feed(feed, apollo_feed)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)

	if feed.Photo.Caption == "" {
		feed.Photo.Caption = caption_from_title(title)
	}
	if len(feed.Tags) == 0 {
		feed.Tags = tags_from_caption(feed.Photo.Caption)
	}
	return feed, nil
}

type apollo_state struct {
	DefaultClient map[string]json.RawMessage `json:"defaultClient"`
}

type apollo_reference struct {
	ID string `json:"id"`
}

type apollo_video_detail struct {
	Type   any                `json:"type"`
	Author apollo_reference   `json:"author"`
	Photo  apollo_reference   `json:"photo"`
	Tags   []apollo_reference `json:"tags"`
}

type apollo_json_value struct {
	JSON json.RawMessage `json:"json"`
}

// feed_from_apollo_state reads the normalized Apollo cache embedded in the
// server-rendered page. Kuaishou leaves the video element's src empty until
// hydration, while the same response already contains photoUrl and manifests
// in this cache.
func feed_from_apollo_state(script_text string, photo_id string) (*Feed, error) {
	assignment_index := strings.Index(script_text, apollo_state_variable)
	if assignment_index < 0 {
		return nil, nil
	}
	state_json := strings.TrimLeft(script_text[assignment_index+len(apollo_state_variable):], " \t\r\n")
	if !strings.HasPrefix(state_json, "=") {
		return nil, errors.New("kuaishou: Apollo state assignment is invalid")
	}
	state_json = strings.TrimLeft(strings.TrimPrefix(state_json, "="), " \t\r\n")
	var state apollo_state
	if err := json.NewDecoder(strings.NewReader(state_json)).Decode(&state); err != nil {
		return nil, fmt.Errorf("kuaishou: decode Apollo state: %w", err)
	}

	photo_key := "VisionVideoDetailPhoto:" + strings.TrimSpace(photo_id)
	photo_data, exists := state.DefaultClient[photo_key]
	if !exists {
		return nil, fmt.Errorf("kuaishou: Apollo state is missing photo %s", photo_id)
	}
	var photo Photo
	if err := json.Unmarshal(photo_data, &photo); err != nil {
		return nil, fmt.Errorf("kuaishou: decode Apollo photo %s: %w", photo_id, err)
	}
	if photo.ID == "" {
		photo.ID = strings.TrimSpace(photo_id)
	}
	merge_apollo_manifests(&photo, photo_data)

	feed := &Feed{Type: "VIDEO", Photo: photo}
	for _, entity_data := range state.DefaultClient {
		var detail apollo_video_detail
		if err := json.Unmarshal(entity_data, &detail); err != nil || detail.Photo.ID != photo_key {
			continue
		}
		if detail.Type != nil {
			feed.Type = detail.Type
		}
		if author_data, ok := state.DefaultClient[detail.Author.ID]; ok {
			_ = json.Unmarshal(author_data, &feed.Author)
		}
		for _, tag_reference := range detail.Tags {
			tag_data, ok := state.DefaultClient[tag_reference.ID]
			if !ok {
				continue
			}
			var tag Tag
			if err := json.Unmarshal(tag_data, &tag); err == nil && strings.TrimSpace(tag.Name) != "" {
				feed.Tags = append(feed.Tags, tag)
			}
		}
		break
	}
	return feed, nil
}

func merge_apollo_manifests(photo *Photo, photo_data json.RawMessage) {
	if photo == nil {
		return
	}
	var media_values struct {
		Manifest      json.RawMessage `json:"manifest"`
		ManifestH265  json.RawMessage `json:"manifestH265"`
		VideoResource json.RawMessage `json:"videoResource"`
	}
	if err := json.Unmarshal(photo_data, &media_values); err != nil {
		return
	}
	if manifest, ok := manifest_from_apollo_json(media_values.Manifest); ok {
		photo.Manifest = manifest
	}
	if manifest, ok := manifest_from_apollo_json(media_values.ManifestH265); ok {
		photo.ManifestH265 = manifest
	}

	var video_resource_value apollo_json_value
	if err := json.Unmarshal(media_values.VideoResource, &video_resource_value); err != nil || len(video_resource_value.JSON) == 0 {
		return
	}
	var video_resource struct {
		H264 Manifest `json:"h264"`
		H265 Manifest `json:"h265"`
		HEVC Manifest `json:"hevc"`
	}
	if err := json.Unmarshal(video_resource_value.JSON, &video_resource); err != nil {
		return
	}
	if manifest_has_url(video_resource.H264) {
		photo.Manifest = video_resource.H264
	}
	if manifest_has_url(video_resource.HEVC) {
		photo.ManifestH265 = video_resource.HEVC
	} else if manifest_has_url(video_resource.H265) {
		photo.ManifestH265 = video_resource.H265
	}
}

func manifest_from_apollo_json(data json.RawMessage) (Manifest, bool) {
	if len(data) == 0 {
		return Manifest{}, false
	}
	var value apollo_json_value
	if err := json.Unmarshal(data, &value); err == nil && len(value.JSON) > 0 {
		data = value.JSON
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil || !manifest_has_url(manifest) {
		return Manifest{}, false
	}
	return manifest, true
}

func merge_feed(base *Feed, overlay *Feed) {
	if base == nil || overlay == nil {
		return
	}
	if overlay.Type != nil {
		base.Type = overlay.Type
	}
	merge_author(&base.Author, overlay.Author)
	merge_photo(&base.Photo, overlay.Photo)
	if len(overlay.Tags) > 0 {
		base.Tags = overlay.Tags
	}
	if overlay.AuthorStatement.Content != "" || overlay.AuthorStatement.Type != nil || overlay.AuthorStatement.RiskStyleType != nil {
		base.AuthorStatement = overlay.AuthorStatement
	}
}

func merge_author(base *Author, overlay Author) {
	if base == nil {
		return
	}
	if overlay.ID != "" {
		base.ID = overlay.ID
	}
	if overlay.Name != "" {
		base.Name = overlay.Name
	}
	if overlay.HeaderURL != "" {
		base.HeaderURL = overlay.HeaderURL
	}
}

func merge_photo(base *Photo, overlay Photo) {
	if base == nil {
		return
	}
	if overlay.ID != "" {
		base.ID = overlay.ID
	}
	if overlay.Duration != 0 {
		base.Duration = overlay.Duration
	}
	if overlay.Caption != "" {
		base.Caption = overlay.Caption
	}
	if overlay.OriginCaption != "" {
		base.OriginCaption = overlay.OriginCaption
	}
	if overlay.LikeCount != 0 {
		base.LikeCount = overlay.LikeCount
	}
	if overlay.ViewCount != 0 {
		base.ViewCount = overlay.ViewCount
	}
	if overlay.CommentCount != 0 {
		base.CommentCount = overlay.CommentCount
	}
	if overlay.RealLikeCount != 0 {
		base.RealLikeCount = overlay.RealLikeCount
	}
	if overlay.CoverURL != "" {
		base.CoverURL = overlay.CoverURL
	}
	if overlay.PhotoURL != "" {
		base.PhotoURL = overlay.PhotoURL
	}
	if overlay.PhotoH265URL != "" {
		base.PhotoH265URL = overlay.PhotoH265URL
	}
	if manifest_has_url(overlay.Manifest) {
		base.Manifest = overlay.Manifest
	}
	if manifest_has_url(overlay.ManifestH265) {
		base.ManifestH265 = overlay.ManifestH265
	}
	if overlay.Timestamp != 0 {
		base.Timestamp = overlay.Timestamp
	}
	if overlay.ExpTag != "" {
		base.ExpTag = overlay.ExpTag
	}
	if overlay.AnimatedCoverURL != "" {
		base.AnimatedCoverURL = overlay.AnimatedCoverURL
	}
	if overlay.VideoRatio != nil {
		base.VideoRatio = overlay.VideoRatio
	}
	if overlay.StereoType != nil {
		base.StereoType = overlay.StereoType
	}
	if overlay.MusicBlocked != nil {
		base.MusicBlocked = overlay.MusicBlocked
	}
	if overlay.RiskTagContent != "" {
		base.RiskTagContent = overlay.RiskTagContent
	}
	if overlay.RiskTagURL != "" {
		base.RiskTagURL = overlay.RiskTagURL
	}
	if overlay.VideoResource != nil {
		base.VideoResource = overlay.VideoResource
	}
}

func html_attribute_value(node *html.Node, name string) string {
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, name) {
			return strings.TrimSpace(attribute.Val)
		}
	}
	return ""
}

func node_has_class(node *html.Node, class string) bool {
	for _, token := range strings.Fields(html_attribute_value(node, "class")) {
		if token == class {
			return true
		}
	}
	return false
}

func node_text(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			builder.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return builder.String()
}

func profile_id_from_href(href string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return "", false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "profile" {
		return "", false
	}
	id := strings.TrimSpace(parts[1])
	if id == "" || id == "undefined" || !valid_path_segment(id) {
		return "", false
	}
	return id, true
}

func hashtag_from_href(href string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return "", false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "hashtag" {
		return "", false
	}
	name := strings.TrimSpace(parts[1])
	if name == "" {
		return "", false
	}
	return name, true
}

func caption_from_title(title string) string {
	title = strings.TrimSpace(title)
	const suffix = "-快手"
	if strings.HasSuffix(title, suffix) {
		return strings.TrimSpace(strings.TrimSuffix(title, suffix))
	}
	return title
}

func tags_from_caption(caption string) []Tag {
	seen := make(map[string]struct{})
	tags := make([]Tag, 0)
	for _, match := range hashtag_pattern.FindAllString(caption, -1) {
		name := strings.TrimSpace(strings.TrimPrefix(match, "#"))
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		tags = append(tags, Tag{Name: name})
	}
	return tags
}

func (c *Client) set_kuaishou_cookie(request *http.Request) {
	if c == nil || request == nil || c.cookie == "" || !is_kuaishou_host(request.URL.Hostname()) {
		return
	}
	request.Header.Set("Cookie", c.cookie)
}

func validate_fetch_result(result *FetchResult) (*FetchResult, error) {
	if result == nil {
		return nil, errors.New("kuaishou fetch result is nil")
	}
	result.PhotoID = strings.TrimSpace(result.PhotoID)
	result.Feed.Photo.ID = strings.TrimSpace(result.Feed.Photo.ID)
	if result.PhotoID == "" {
		result.PhotoID = result.Feed.Photo.ID
	}
	if result.Feed.Photo.ID == "" {
		result.Feed.Photo.ID = result.PhotoID
	}
	if result.PhotoID == "" || result.Feed.Photo.ID != result.PhotoID {
		return nil, errors.New("kuaishou fetch result has an invalid photo id")
	}
	if strings.TrimSpace(result.Feed.Photo.PhotoURL) == "" && strings.TrimSpace(result.Feed.Photo.PhotoH265URL) == "" && !manifest_has_url(result.Feed.Photo.Manifest) && !manifest_has_url(result.Feed.Photo.ManifestH265) {
		return nil, fmt.Errorf("kuaishou photo %s has no downloadable video URL", result.PhotoID)
	}
	return result, nil
}

func manifest_has_url(manifest Manifest) bool {
	for _, adaptation_set := range manifest.AdaptationSet {
		for _, representation := range adaptation_set.Representation {
			if strings.TrimSpace(representation.URL) != "" || len(representation.BackupURL) > 0 {
				return true
			}
		}
	}
	return false
}

func extract_photo_id(page_url *url.URL) (string, error) {
	if page_url == nil {
		return "", errors.New("kuaishou page URL is nil")
	}
	if photo_id := strings.TrimSpace(page_url.Query().Get("photoId")); valid_path_segment(photo_id) {
		return photo_id, nil
	}
	path_parts := strings.Split(strings.Trim(page_url.EscapedPath(), "/"), "/")
	for part_index := 0; part_index+1 < len(path_parts); part_index++ {
		if path_parts[part_index] != "short-video" && path_parts[part_index] != "photo" {
			continue
		}
		photo_id, err := url.PathUnescape(path_parts[part_index+1])
		if err == nil && valid_path_segment(photo_id) {
			return photo_id, nil
		}
	}
	return "", fmt.Errorf("kuaishou: photo id not found in %q", page_url.String())
}

func valid_path_segment(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func validate_input_url(request_url *url.URL) error {
	if request_url == nil || request_url.Hostname() == "" {
		return errors.New("kuaishou URL is invalid")
	}
	scheme := strings.ToLower(strings.TrimSpace(request_url.Scheme))
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("kuaishou: unsupported URL scheme %q", request_url.Scheme)
	}
	host := normalized_host(request_url.Hostname())
	if !is_kuaishou_host(host) {
		return fmt.Errorf("kuaishou: unsupported URL host %q", request_url.Hostname())
	}
	if host == "v.kuaishou.com" || strings.HasPrefix(strings.Trim(request_url.EscapedPath(), "/"), "f/") {
		return nil
	}
	if _, err := extract_photo_id(request_url); err != nil {
		return err
	}
	return nil
}

func validate_redirect_url(request_url *url.URL) error {
	if request_url == nil || request_url.Hostname() == "" {
		return errors.New("kuaishou: redirect URL is invalid")
	}
	scheme := strings.ToLower(strings.TrimSpace(request_url.Scheme))
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("kuaishou: unsupported redirect scheme %q", request_url.Scheme)
	}
	host := normalized_host(request_url.Hostname())
	if is_kuaishou_host(host) || host == "v.m.chenzhongtech.com" {
		return nil
	}
	return fmt.Errorf("kuaishou: unsupported redirect host %q", request_url.Hostname())
}

func normalized_host(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

func is_kuaishou_host(host string) bool {
	host = normalized_host(host)
	return host == "kuaishou.com" || strings.HasSuffix(host, ".kuaishou.com") ||
		host == "kuaishou.cn" || strings.HasSuffix(host, ".kuaishou.cn")
}

func stop_automatic_redirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

func is_redirect_status(status_code int) bool {
	switch status_code {
	case http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusSeeOther,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

func set_navigation_headers(headers http.Header) {
	headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	headers.Set("Accept-Language", "zh-CN,zh;q=0.9")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Pragma", "no-cache")
	headers.Set("Sec-Fetch-Dest", "document")
	headers.Set("Sec-Fetch-Mode", "navigate")
	headers.Set("Sec-Fetch-Site", "none")
	headers.Set("Sec-Fetch-User", "?1")
	headers.Set("Upgrade-Insecure-Requests", "1")
	headers.Set("User-Agent", DefaultUserAgent)
}

type flexible_int64 int64

func (v *flexible_int64) UnmarshalJSON(data []byte) error {
	parsed_value, err := parse_flexible_int(data, 64)
	if err != nil {
		return err
	}
	*v = flexible_int64(parsed_value)
	return nil
}

type flexible_int int

func (v *flexible_int) UnmarshalJSON(data []byte) error {
	parsed_value, err := parse_flexible_int(data, 0)
	if err != nil {
		return err
	}
	*v = flexible_int(parsed_value)
	return nil
}

func parse_flexible_int(data []byte, bit_size int) (int64, error) {
	value := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if value == "" || value == "null" {
		return 0, nil
	}
	parsed_value, err := strconv.ParseInt(value, 10, bit_size)
	if err == nil {
		return parsed_value, nil
	}
	multiplier := float64(1)
	suffix := ""
	switch {
	case strings.HasSuffix(value, "亿"):
		suffix, multiplier = "亿", 100_000_000
	case strings.HasSuffix(value, "万"):
		suffix, multiplier = "万", 10_000
	case strings.HasSuffix(value, "千"):
		suffix, multiplier = "千", 1_000
	case strings.HasSuffix(value, "w"), strings.HasSuffix(value, "W"):
		suffix, multiplier = value[len(value)-1:], 10_000
	case strings.HasSuffix(value, "k"), strings.HasSuffix(value, "K"):
		suffix, multiplier = value[len(value)-1:], 1_000
	}
	if suffix != "" {
		value = strings.TrimSpace(strings.TrimSuffix(value, suffix))
	}
	parsed_float, float_err := strconv.ParseFloat(strings.ReplaceAll(value, ",", ""), 64)
	if float_err != nil {
		return 0, fmt.Errorf("kuaishou: invalid integer %q", value)
	}
	return int64(parsed_float * multiplier), nil
}

type flexible_bool bool

func (v *flexible_bool) UnmarshalJSON(data []byte) error {
	value := strings.ToLower(strings.Trim(strings.TrimSpace(string(data)), `"`))
	switch value {
	case "", "null", "false", "0":
		*v = false
		return nil
	case "true", "1":
		*v = true
		return nil
	default:
		return fmt.Errorf("kuaishou: invalid boolean %q", value)
	}
}

type flexible_strings []string

func (v *flexible_strings) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*v = nil
		return nil
	}
	var values []string
	if err := json.Unmarshal(data, &values); err == nil {
		*v = values
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("kuaishou: invalid URL list: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		*v = nil
	} else {
		*v = []string{value}
	}
	return nil
}

// UnmarshalJSON accepts either a normal manifest object or an escaped JSON
// string containing that object.
func (m *Manifest) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.New("kuaishou: nil manifest")
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) || bytes.Equal(data, []byte(`""`)) {
		*m = Manifest{}
		return nil
	}
	if data[0] == '"' {
		var encoded_manifest string
		if err := json.Unmarshal(data, &encoded_manifest); err != nil {
			return fmt.Errorf("kuaishou: decode manifest string: %w", err)
		}
		data = []byte(encoded_manifest)
	}
	type manifest_alias Manifest
	var decoded_manifest manifest_alias
	if err := json.Unmarshal(data, &decoded_manifest); err != nil {
		return fmt.Errorf("kuaishou: decode manifest: %w", err)
	}
	*m = Manifest(decoded_manifest)
	return nil
}
