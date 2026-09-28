// Package x fetches public X/Twitter post pages with minib.
package x

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"wx_channel/pkg/cache"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/minib"
)

const (
	// PlatformID is the stable platform identifier used by the adapter layer.
	PlatformID = "x"
	// DefaultUserAgent is sent when downloading media returned by X.
	DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36"

	default_request_timeout = 30 * time.Second
)

// FetchResult is one post and the main-post video data embedded in X's SSR payload.
type FetchResult struct {
	SourceURL      string   `json:"source_url"`
	ExternalID     string   `json:"external_id"`
	AuthorID       string   `json:"author_id"`
	AuthorName     string   `json:"author_name"`
	AuthorUsername string   `json:"author_username"`
	AuthorAvatar   string   `json:"author_avatar"`
	BodyText       string   `json:"body_text"`
	CoverURL       string   `json:"cover_url"`
	PublishTime    int64    `json:"publish_time"`
	ViewCount      int64    `json:"view_count"`
	LikeCount      int64    `json:"like_count"`
	CommentCount   int64    `json:"comment_count"`
	ShareCount     int64    `json:"share_count"`
	Videos         []Video  `json:"videos"`
	Images         []Image  `json:"images"`
	Article        *Article `json:"article,omitempty"`
}

// Article is the long-form article X renders as the whole post when the
// requested status points at one. Article text lives in its own entity graph,
// not in the tweet's full_text.
type Article struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Text     string `json:"text"`
	Markdown string `json:"markdown,omitempty"`
	CoverURL string `json:"cover_url,omitempty"`
}

// Video is one video attached directly to the requested post.
type Video struct {
	ID             string         `json:"id"`
	Type           string         `json:"type"`
	URL            string         `json:"url"`
	HLSURL         string         `json:"hls_url"`
	CoverURL       string         `json:"cover_url"`
	Width          int            `json:"width"`
	Height         int            `json:"height"`
	DurationMillis int64          `json:"duration_millis"`
	Variants       []VideoVariant `json:"variants"`
}

// VideoVariant is one progressive MP4 or HLS representation advertised by X.
type VideoVariant struct {
	Bitrate     int    `json:"bitrate"`
	ContentType string `json:"content_type"`
	URL         string `json:"url"`
}

// Image is one photo attached to the requested post. X advertises at most four
// photos per post and does not mix them with a video.
type Image struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	AltText string `json:"alt_text,omitempty"`
}

// Client owns the minib session used for one or more X requests.
type Client struct {
	browser         *minib.MiniBrowser
	cookie_provider *cookies.Reader
	file_cache      *cache.CacheProvider
}

// NewClient creates an X client backed by a Chrome-fingerprint minib session.
func NewClient(cookie_providers ...*cookies.Reader) (*Client, error) {
	browser, err := minib.NewMiniBrowser(default_request_timeout, cookie_providers...)
	if err != nil {
		return nil, fmt.Errorf("x: create minib browser: %w", err)
	}
	var cookie_provider *cookies.Reader
	if len(cookie_providers) > 0 {
		cookie_provider = cookie_providers[0]
	}
	return &Client{browser: browser, cookie_provider: cookie_provider}, nil
}

// SetPersistentCache configures the cache used by account-home requests.
func (c *Client) SetPersistentCache(file_cache *cache.CacheProvider) {
	if c != nil {
		c.file_cache = file_cache
	}
}

// Close releases the client's minib session.
func (c *Client) Close() {
	if c == nil || c.browser == nil {
		return
	}
	c.browser.Close()
	c.browser = nil
}

// Fetch retrieves one public X post.
func (c *Client) Fetch(raw_url string) (*FetchResult, error) {
	return c.FetchContext(context.Background(), raw_url)
}

// FetchContext retrieves one public X post with cancellation support.
func (c *Client) FetchContext(fetch_context context.Context, raw_url string) (*FetchResult, error) {
	if c == nil || c.browser == nil {
		return nil, fmt.Errorf("x client is closed")
	}
	status_id, err := ExtractStatusID(raw_url)
	if err != nil {
		return nil, err
	}
	if fetch_context == nil {
		fetch_context = context.Background()
	}
	page, err := c.browser.Navigate(fetch_context, raw_url, nil, minib.NavigateOptions{
		DisableSubresources: true,
		DisableCSS:          true,
		DisableImages:       true,
		DisableMedia:        true,
		DisableJavaScript:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("x: fetch post: %w", err)
	}
	if page.StatusCode < 200 || page.StatusCode >= 300 {
		return nil, fmt.Errorf("x: post returned HTTP %d", page.StatusCode)
	}
	result, err := extract_post(page.HTML, page.Document, status_id, page.URL)
	if err != nil {
		return nil, err
	}
	return normalize_result(result, status_id, page.URL)
}

// ExtractStatusID validates an X/Twitter status URL and returns its decimal ID.
func ExtractStatusID(raw_url string) (string, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil {
		return "", fmt.Errorf("x: parse URL: %w", err)
	}
	if parsed_url.Scheme != "https" {
		return "", fmt.Errorf("x: unsupported URL scheme %q", parsed_url.Scheme)
	}
	host := strings.ToLower(strings.TrimSuffix(parsed_url.Hostname(), "."))
	if !host_matches(host, "x.com") && !host_matches(host, "twitter.com") {
		return "", fmt.Errorf("x: unsupported URL host %q", parsed_url.Hostname())
	}
	path_parts := strings.Split(strings.Trim(parsed_url.EscapedPath(), "/"), "/")
	if len(path_parts) < 3 || path_parts[1] != "status" || !decimal_string(path_parts[2]) {
		return "", fmt.Errorf("x: URL is not a status detail URL")
	}
	return path_parts[2], nil
}

func extract_post(source string, document *html.Node, status_id string, fallback_url string) (*FetchResult, error) {
	metadata := document_metadata(document)
	tweet_key := base64.StdEncoding.EncodeToString([]byte("Tweet:" + status_id))
	if extract_js_string_field(extract_record(source, tweet_key), "rest_id") == status_id {
		return extract_post_records(source, metadata, tweet_key, status_id, fallback_url), nil
	}
	if result, ok := tsr_extract_post(source, metadata, status_id, fallback_url); ok {
		return result, nil
	}
	return nil, fmt.Errorf("x: requested post %s is missing from initialization data", status_id)
}

// extract_post_records reads the older payload, where every record carries an
// `__id:` and fields reference each other by `__ref`.
func extract_post_records(source string, metadata map[string]string, tweet_key string, status_id string, fallback_url string) *FetchResult {
	details_record := extract_record(source, "client:"+tweet_key+":details")
	counts_record := extract_record(source, "client:"+tweet_key+":counts")
	views_record := extract_record(source, "client:"+tweet_key+":views")
	result := &FetchResult{
		SourceURL:    first_non_empty(metadata["og:url"], metadata["canonical"], fallback_url),
		ExternalID:   status_id,
		BodyText:     first_non_empty(metadata["og:description"], extract_js_string_field(details_record, "full_text")),
		CoverURL:     metadata["og:image"],
		PublishTime:  extract_js_int64_field(details_record, "created_at_ms"),
		ViewCount:    extract_js_int64_field(views_record, "count"),
		LikeCount:    extract_js_int64_field(counts_record, "favorite_count"),
		CommentCount: extract_js_int64_field(counts_record, "reply_count"),
		ShareCount:   extract_js_int64_field(counts_record, "retweet_count"),
	}
	result.AuthorUsername = strings.TrimPrefix(metadata["twitter:creator"], "@")
	result.Article = extract_article(source, tweet_key)
	populate_author(source, extract_record(source, "client:"+tweet_key+":core"), result)
	fill_result_from_metadata(result, metadata)
	for media_index := 0; media_index < 4; media_index++ {
		media_key := fmt.Sprintf("client:%s:media_entities2:%d", tweet_key, media_index)
		media_record := extract_record(source, media_key)
		media_type := extract_js_string_field(media_record, "type")
		if media_type == "photo" {
			image := Image{
				ID:      extract_js_string_field(media_record, "id_str"),
				URL:     extract_js_string_field(media_record, "media_url_https"),
				AltText: extract_js_string_field(media_record, "ext_alt_text"),
			}
			original_record := extract_record(source, media_key+":original_info")
			image.Width = int(extract_js_int64_field(original_record, "width"))
			image.Height = int(extract_js_int64_field(original_record, "height"))
			result.Images = append(result.Images, image)
			continue
		}
		if media_type != "video" && media_type != "animated_gif" {
			continue
		}
		video := Video{
			ID:       extract_js_string_field(media_record, "id_str"),
			Type:     media_type,
			CoverURL: extract_js_string_field(media_record, "media_url_https"),
		}
		original_record := extract_record(source, media_key+":original_info")
		video.Width = int(extract_js_int64_field(original_record, "width"))
		video.Height = int(extract_js_int64_field(original_record, "height"))
		video_info_key := media_key + ":video_info"
		video.DurationMillis = extract_js_int64_field(extract_record(source, video_info_key), "duration_millis")
		for variant_index := 0; variant_index < 16; variant_index++ {
			variant_record := extract_record(source, fmt.Sprintf("%s:variants:%d", video_info_key, variant_index))
			variant_url := extract_js_string_field(variant_record, "url")
			if variant_url == "" {
				break
			}
			video.Variants = append(video.Variants, VideoVariant{
				Bitrate:     int(extract_js_int64_field(variant_record, "bitrate")),
				ContentType: extract_js_string_field(variant_record, "content_type"),
				URL:         variant_url,
			})
		}
		result.Videos = append(result.Videos, video)
	}
	return result
}

// fill_result_from_metadata applies the page-level fallbacks both payload shapes
// share: the author and publish time can come from the document meta tags when
// the payload omits them.
func fill_result_from_metadata(result *FetchResult, metadata map[string]string) {
	result.AuthorUsername = first_non_empty(
		result.AuthorUsername,
		strings.TrimPrefix(metadata["twitter:creator"], "@"),
		status_url_username(result.SourceURL),
	)
	result.AuthorName = first_non_empty(result.AuthorName, title_author(metadata["og:title"], result.AuthorUsername))
	if result.PublishTime == 0 {
		if publish_time, err := time.Parse(time.RFC3339Nano, metadata["article:published_time"]); err == nil {
			result.PublishTime = publish_time.UnixMilli()
		}
	}
}

// extract_article follows the tweet -> article entity reference chain that X
// only emits for long-form posts. It returns nil for ordinary tweets.
func extract_article(source string, tweet_key string) *Article {
	article_record := extract_record(source, "client:"+tweet_key+":article")
	if article_record == "" {
		return nil
	}
	results_record := extract_record(source, extract_js_ref_field(article_record, "article_results"))
	entity_record := extract_record(source, extract_js_ref_field(results_record, "result"))
	title := strings.TrimSpace(extract_js_string_field(entity_record, "title"))
	text := first_non_empty(
		extract_js_string_field(entity_record, "plain_text"),
		extract_js_string_field(entity_record, "preview_text"),
	)
	markdown := article_markdown(source, entity_record)
	cover_url := article_cover_url(source, entity_record)
	if title == "" && text == "" && markdown == "" {
		return nil
	}
	return &Article{
		ID:       extract_js_string_field(entity_record, "rest_id"),
		Title:    title,
		Text:     text,
		Markdown: markdown,
		CoverURL: cover_url,
	}
}

func populate_author(source string, tweet_core_record string, result *FetchResult) {
	user_results_key := extract_js_ref_field(tweet_core_record, "user_results")
	user_key := extract_js_ref_field(extract_record(source, user_results_key), "result")
	user_record := extract_record(source, user_key)
	result.AuthorID = extract_js_string_field(user_record, "rest_id")
	user_core_record := extract_record(source, extract_js_ref_field(user_record, "core"))
	result.AuthorName = extract_js_string_field(user_core_record, "name")
	result.AuthorUsername = first_non_empty(result.AuthorUsername, extract_js_string_field(user_core_record, "screen_name"))
	avatar_record := extract_record(source, extract_js_ref_field(user_record, "avatar"))
	result.AuthorAvatar = extract_js_string_field(avatar_record, "image_url")
}

func extract_record(source string, record_id string) string {
	if record_id == "" {
		return ""
	}
	marker := `__id:` + strconv.Quote(record_id)
	marker_index := strings.Index(source, marker)
	if marker_index < 0 {
		return ""
	}
	start_index := strings.LastIndex(source[:marker_index], "{")
	if start_index < 0 {
		return ""
	}
	depth := 0
	in_string := false
	escaped := false
	for byte_index := start_index; byte_index < len(source); byte_index++ {
		character := source[byte_index]
		if in_string {
			if escaped {
				escaped = false
			} else if character == '\\' {
				escaped = true
			} else if character == '"' {
				in_string = false
			}
			continue
		}
		switch character {
		case '"':
			in_string = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[start_index : byte_index+1]
			}
		}
	}
	return ""
}

func extract_js_string_field(record string, field string) string {
	value_index := js_field_value_index(record, field)
	if value_index < 0 || value_index >= len(record) || record[value_index] != '"' {
		return ""
	}
	escaped := false
	for byte_index := value_index + 1; byte_index < len(record); byte_index++ {
		character := record[byte_index]
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' {
			escaped = true
			continue
		}
		if character != '"' {
			continue
		}
		var value string
		if json.Unmarshal([]byte(record[value_index:byte_index+1]), &value) == nil {
			return value
		}
		return ""
	}
	return ""
}

func extract_js_int64_field(record string, field string) int64 {
	value_index := js_field_value_index(record, field)
	if value_index < 0 {
		return 0
	}
	value := record[value_index:]
	if strings.HasPrefix(value, `"`) {
		value = extract_js_string_field(record, field)
	} else {
		end_index := 0
		for end_index < len(value) && value[end_index] >= '0' && value[end_index] <= '9' {
			end_index++
		}
		value = value[:end_index]
	}
	parsed_value, _ := strconv.ParseInt(value, 10, 64)
	return parsed_value
}

func extract_js_ref_field(record string, field string) string {
	value_index := js_field_value_index(record, field)
	if value_index < 0 {
		return ""
	}
	return extract_js_string_field(record[value_index:], "__ref")
}

func js_field_value_index(record string, field string) int {
	marker := field + ":"
	search_index := 0
	for search_index < len(record) {
		field_index := strings.Index(record[search_index:], marker)
		if field_index < 0 {
			return -1
		}
		field_index += search_index
		if field_index == 0 || record[field_index-1] == '{' || record[field_index-1] == ',' {
			value_index := field_index + len(marker)
			for value_index < len(record) && (record[value_index] == ' ' || record[value_index] == '\t' || record[value_index] == '\n' || record[value_index] == '\r') {
				value_index++
			}
			return value_index
		}
		search_index = field_index + len(marker)
	}
	return -1
}

func document_metadata(document *html.Node) map[string]string {
	metadata := make(map[string]string)
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "meta" {
			key := first_non_empty(html_attribute(node, "property"), html_attribute(node, "name"))
			if key != "" && metadata[key] == "" {
				metadata[key] = html_attribute(node, "content")
			}
		}
		if node.Type == html.ElementNode && node.Data == "link" && strings.EqualFold(html_attribute(node, "rel"), "canonical") {
			metadata["canonical"] = html_attribute(node, "href")
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	if document != nil {
		walk(document)
	}
	return metadata
}

func html_attribute(node *html.Node, name string) string {
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, name) {
			return strings.TrimSpace(attribute.Val)
		}
	}
	return ""
}

func status_url_username(raw_url string) string {
	parsed_url, err := url.Parse(raw_url)
	if err != nil {
		return ""
	}
	path_parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
	if len(path_parts) < 3 || path_parts[1] != "status" {
		return ""
	}
	return path_parts[0]
}

func title_author(title string, username string) string {
	suffix := " (@" + username + ") on X"
	if username != "" && strings.HasSuffix(title, suffix) {
		return strings.TrimSuffix(title, suffix)
	}
	return ""
}

func normalize_result(result *FetchResult, status_id string, fallback_url string) (*FetchResult, error) {
	if result == nil || strings.TrimSpace(result.ExternalID) != status_id {
		return nil, fmt.Errorf("x: requested post %s was not returned", status_id)
	}
	result.SourceURL = normalize_source_url(result.SourceURL, fallback_url)
	result.AuthorID = strings.TrimSpace(result.AuthorID)
	result.AuthorName = strings.TrimSpace(result.AuthorName)
	result.AuthorUsername = strings.TrimPrefix(strings.TrimSpace(result.AuthorUsername), "@")
	result.AuthorAvatar = normalize_media_url(result.AuthorAvatar, "pbs.twimg.com")
	result.BodyText = strings.TrimSpace(result.BodyText)
	result.CoverURL = normalize_media_url(result.CoverURL, "pbs.twimg.com")
	result.Article = normalize_article(result.Article)
	valid_videos := make([]Video, 0, len(result.Videos))
	for video_index := range result.Videos {
		video := result.Videos[video_index]
		video.ID = strings.TrimSpace(video.ID)
		video.Type = strings.TrimSpace(video.Type)
		video.CoverURL = normalize_media_url(video.CoverURL, "pbs.twimg.com")
		video.Variants = normalize_variants(video.Variants)
		best_bitrate := -1
		for _, variant := range video.Variants {
			switch {
			case is_hls_variant(variant):
				video.HLSURL = variant.URL
			case strings.EqualFold(variant.ContentType, "video/mp4") && variant.Bitrate > best_bitrate:
				video.URL = variant.URL
				best_bitrate = variant.Bitrate
			}
		}
		if video.URL == "" {
			video.URL = video.HLSURL
		}
		if video.URL != "" {
			valid_videos = append(valid_videos, video)
		}
	}
	result.Videos = valid_videos
	valid_images := make([]Image, 0, len(result.Images))
	seen_image_urls := make(map[string]bool)
	for image_index := range result.Images {
		image := result.Images[image_index]
		image.ID = strings.TrimSpace(image.ID)
		image.AltText = strings.TrimSpace(image.AltText)
		image.URL = normalize_media_url(image.URL, "pbs.twimg.com")
		if image.URL == "" || seen_image_urls[image.URL] {
			continue
		}
		seen_image_urls[image.URL] = true
		valid_images = append(valid_images, image)
	}
	result.Images = valid_images
	if result.BodyText == "" && result.Article == nil && len(result.Videos) == 0 && len(result.Images) == 0 {
		return nil, fmt.Errorf("x: post %s has no text, image, or video", status_id)
	}
	if result.AuthorID == "" {
		result.AuthorID = result.AuthorUsername
	}
	if result.AuthorName == "" {
		result.AuthorName = first_non_empty(result.AuthorUsername, "X user")
	}
	return result, nil
}

func normalize_article(article *Article) *Article {
	if article == nil {
		return nil
	}
	article.ID = strings.TrimSpace(article.ID)
	article.Title = strings.TrimSpace(article.Title)
	article.Text = strings.TrimSpace(article.Text)
	article.Markdown = strings.TrimSpace(article.Markdown)
	article.CoverURL = normalize_media_url(article.CoverURL, "pbs.twimg.com")
	if article.Title == "" && article.Text == "" && article.Markdown == "" {
		return nil
	}
	return article
}

func normalize_variants(variants []VideoVariant) []VideoVariant {
	result := make([]VideoVariant, 0, len(variants))
	seen_urls := make(map[string]bool)
	for _, variant := range variants {
		variant.URL = normalize_media_url(variant.URL, "video.twimg.com")
		variant.ContentType = strings.TrimSpace(variant.ContentType)
		if variant.URL == "" || seen_urls[variant.URL] {
			continue
		}
		seen_urls[variant.URL] = true
		result = append(result, variant)
	}
	return result
}

func is_hls_variant(variant VideoVariant) bool {
	return strings.Contains(strings.ToLower(variant.ContentType), "mpegurl") || strings.Contains(strings.ToLower(variant.URL), ".m3u8")
}

func normalize_source_url(raw_url string, fallback_url string) string {
	for _, candidate := range []string{raw_url, fallback_url} {
		parsed_url, err := url.Parse(strings.TrimSpace(candidate))
		if err != nil || parsed_url.Scheme != "https" {
			continue
		}
		host := strings.ToLower(strings.TrimSuffix(parsed_url.Hostname(), "."))
		if host_matches(host, "x.com") || host_matches(host, "twitter.com") {
			parsed_url.Fragment = ""
			return parsed_url.String()
		}
	}
	return ""
}

func normalize_media_url(raw_url string, expected_host string) string {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || parsed_url.Scheme != "https" {
		return ""
	}
	host := strings.ToLower(strings.TrimSuffix(parsed_url.Hostname(), "."))
	if !host_matches(host, expected_host) {
		return ""
	}
	parsed_url.Fragment = ""
	return parsed_url.String()
}

func host_matches(host string, root string) bool {
	return host == root || strings.HasSuffix(host, "."+root)
}

func decimal_string(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func first_non_empty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
