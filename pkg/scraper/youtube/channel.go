package youtube

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ChannelContent is one item discovered in a YouTube channel tab.
type ChannelContent struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	CoverURL  string `json:"cover_url"`
	Type      string `json:"type"`
	Published string `json:"published"`
}

// ChannelContentPage contains one channel tab page and its opaque continuation
// token. An empty NextMarker means YouTube did not advertise another page.
type ChannelContentPage struct {
	Items      []ChannelContent `json:"items"`
	NextMarker string           `json:"next_marker,omitempty"`
}

// ParseChannelContents extracts channel videos, Shorts, playlists, podcasts,
// and community posts from ytInitialData without running the YouTube app.
func ParseChannelContents(page_html string, scope string) ([]ChannelContent, error) {
	page, err := ParseChannelContentsPage(page_html, scope)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

// ParseChannelContentsPage extracts the first page of a channel tab.
func ParseChannelContentsPage(page_html string, scope string) (*ChannelContentPage, error) {
	raw_json, ok, err := extract_json_by_regexp([]byte(page_html), initial_data_re)
	if err != nil {
		return nil, fmt.Errorf("youtube channel: extract ytInitialData: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("youtube channel: ytInitialData is missing")
	}
	page, err := parse_channel_contents_json(raw_json, scope)
	if err != nil {
		return nil, fmt.Errorf("youtube channel: decode ytInitialData: %w", err)
	}
	return page, nil
}

// ParseChannelContinuation extracts a youtubei browse continuation response.
func ParseChannelContinuation(response_json []byte, scope string) (*ChannelContentPage, error) {
	page, err := parse_channel_contents_json(response_json, scope)
	if err != nil {
		return nil, fmt.Errorf("youtube channel: decode browse continuation: %w", err)
	}
	return page, nil
}

func parse_channel_contents_json(raw_json []byte, scope string) (*ChannelContentPage, error) {
	var root any
	if err := json.Unmarshal(raw_json, &root); err != nil {
		return nil, err
	}
	if root_map, ok := root.(map[string]any); ok {
		if error_value, exists := root_map["error"]; exists {
			return nil, youtube_channel_api_error(error_value)
		}
	}
	items := make([]ChannelContent, 0)
	seen := make(map[string]struct{})
	var walk func(any)
	walk = func(value any) {
		switch typed_value := value.(type) {
		case map[string]any:
			for key, child := range typed_value {
				var item ChannelContent
				var matched bool
				switch key {
				case "videoRenderer", "gridVideoRenderer", "playlistVideoRenderer":
					if scope == "videos" || scope == "streams" || scope == "shorts" {
						item, matched = youtube_video_renderer(child)
					}
				case "reelItemRenderer", "shortsLockupViewModel":
					if scope == "shorts" {
						item, matched = youtube_short_renderer(child)
					}
				case "playlistRenderer", "gridPlaylistRenderer":
					if scope == "playlists" || scope == "podcasts" {
						item, matched = youtube_playlist_renderer(child, scope)
					}
				case "backstagePostRenderer":
					if scope == "community" {
						item, matched = youtube_post_renderer(child)
					}
				case "lockupViewModel":
					item, matched = youtube_lockup_renderer(child, scope)
				}
				if matched && scope == "streams" && item.Type == "video" {
					item.Type = "live"
				}
				if matched && item.ID != "" {
					identity := item.Type + ":" + item.ID
					if _, exists := seen[identity]; !exists {
						seen[identity] = struct{}{}
						items = append(items, item)
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range typed_value {
				walk(child)
			}
		}
	}
	walk(root)
	return &ChannelContentPage{Items: items, NextMarker: youtube_content_continuation(root)}, nil
}

func youtube_channel_api_error(value any) error {
	error_map, _ := value.(map[string]any)
	message := nested_string(error_map, "message")
	if message == "" {
		message = "unknown API error"
	}
	return fmt.Errorf("browse API returned an error: %s", message)
}

func youtube_content_continuation(root any) string {
	best_token := ""
	best_score := -1
	var walk func(any, []string)
	walk = func(value any, path []string) {
		switch typed_value := value.(type) {
		case map[string]any:
			if renderer, ok := typed_value["continuationItemRenderer"].(map[string]any); ok {
				token := nested_string(renderer, "continuationEndpoint", "continuationCommand", "token")
				if token != "" {
					score := youtube_continuation_path_score(path)
					if score > best_score || (score == best_score && len(token) > len(best_token)) {
						best_score = score
						best_token = token
					}
				}
			}
			for key, child := range typed_value {
				walk(child, append(path, key))
			}
		case []any:
			for _, child := range typed_value {
				walk(child, path)
			}
		}
	}
	walk(root, nil)
	return best_token
}

func youtube_continuation_path_score(path []string) int {
	score := 0
	for _, key := range path {
		switch key {
		case "richGridRenderer":
			score += 100
		case "appendContinuationItemsAction", "reloadContinuationItemsCommand":
			score += 200
		case "continuationItems":
			score += 50
		case "header", "engagementPanel":
			score -= 100
		}
	}
	return score
}

func youtube_lockup_renderer(value any, scope string) (ChannelContent, bool) {
	renderer, ok := value.(map[string]any)
	if !ok {
		return ChannelContent{}, false
	}
	if !youtube_lockup_matches_scope(json_string(renderer["contentType"]), scope) {
		return ChannelContent{}, false
	}
	content_id := json_string(renderer["contentId"])
	if content_id == "" {
		return ChannelContent{}, false
	}
	title := nested_string(renderer, "metadata", "lockupMetadataViewModel", "title", "content")
	content_type := "video"
	source_url := "https://www.youtube.com/watch?v=" + content_id
	if scope == "playlists" || scope == "podcasts" {
		content_type = "collection"
		if scope == "podcasts" {
			content_type = "podcast"
		}
		source_url = "https://www.youtube.com/playlist?list=" + content_id
	} else if scope == "community" {
		content_type = "post"
		source_url = "https://www.youtube.com/post/" + content_id
	} else if scope == "shorts" {
		source_url = "https://www.youtube.com/shorts/" + content_id
	}
	return ChannelContent{ID: content_id, Title: title, URL: source_url, CoverURL: youtube_thumbnail(renderer["contentImage"]), Type: content_type}, true
}

func youtube_lockup_matches_scope(content_type string, scope string) bool {
	if content_type == "" {
		return scope == "videos" || scope == "streams" || scope == "playlists" || scope == "podcasts"
	}
	switch scope {
	case "videos", "streams":
		return content_type == "LOCKUP_CONTENT_TYPE_VIDEO"
	case "podcasts":
		return content_type == "LOCKUP_CONTENT_TYPE_PODCAST"
	case "playlists":
		return content_type == "LOCKUP_CONTENT_TYPE_PLAYLIST" || content_type == "LOCKUP_CONTENT_TYPE_COURSE" || content_type == "LOCKUP_CONTENT_TYPE_PODCAST"
	default:
		return false
	}
}

func youtube_video_renderer(value any) (ChannelContent, bool) {
	renderer, ok := value.(map[string]any)
	if !ok {
		return ChannelContent{}, false
	}
	video_id := json_string(renderer["videoId"])
	if video_id == "" {
		return ChannelContent{}, false
	}
	return ChannelContent{
		ID: video_id, Title: youtube_text(renderer["title"]),
		URL:      "https://www.youtube.com/watch?v=" + video_id,
		CoverURL: youtube_thumbnail(renderer["thumbnail"]), Type: "video",
		Published: youtube_text(renderer["publishedTimeText"]),
	}, true
}

func youtube_short_renderer(value any) (ChannelContent, bool) {
	renderer, ok := value.(map[string]any)
	if !ok {
		return ChannelContent{}, false
	}
	video_id := json_string(renderer["videoId"])
	if video_id == "" {
		video_id = nested_string(renderer, "onTap", "innertubeCommand", "reelWatchEndpoint", "videoId")
	}
	if video_id == "" {
		video_id = nested_string(renderer, "navigationEndpoint", "reelWatchEndpoint", "videoId")
	}
	if video_id == "" {
		return ChannelContent{}, false
	}
	title := youtube_text(renderer["headline"])
	if title == "" {
		title = youtube_text(renderer["title"])
	}
	if title == "" {
		title = nested_string(renderer, "overlayMetadata", "primaryText", "content")
	}
	thumbnail := renderer["thumbnail"]
	if thumbnail == nil {
		thumbnail = renderer["thumbnailViewModel"]
	}
	if thumbnail == nil {
		thumbnail = renderer["onTap"]
	}
	return ChannelContent{
		ID: video_id, Title: title, URL: "https://www.youtube.com/shorts/" + video_id,
		CoverURL: youtube_thumbnail(thumbnail), Type: "video",
	}, true
}

func youtube_playlist_renderer(value any, scope string) (ChannelContent, bool) {
	renderer, ok := value.(map[string]any)
	if !ok {
		return ChannelContent{}, false
	}
	playlist_id := json_string(renderer["playlistId"])
	if playlist_id == "" {
		return ChannelContent{}, false
	}
	content_type := "collection"
	if scope == "podcasts" {
		content_type = "podcast"
	}
	return ChannelContent{
		ID: playlist_id, Title: youtube_text(renderer["title"]),
		URL:      "https://www.youtube.com/playlist?list=" + playlist_id,
		CoverURL: youtube_thumbnail(renderer["thumbnail"]), Type: content_type,
	}, true
}

func youtube_post_renderer(value any) (ChannelContent, bool) {
	renderer, ok := value.(map[string]any)
	if !ok {
		return ChannelContent{}, false
	}
	post_id := json_string(renderer["postId"])
	if post_id == "" {
		return ChannelContent{}, false
	}
	return ChannelContent{
		ID: post_id, Title: youtube_text(renderer["contentText"]),
		URL:      "https://www.youtube.com/post/" + post_id,
		CoverURL: youtube_thumbnail(renderer["backstageAttachment"]), Type: "post",
		Published: youtube_text(renderer["publishedTimeText"]),
	}, true
}

func youtube_text(value any) string {
	object, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	if simple_text := json_string(object["simpleText"]); simple_text != "" {
		return simple_text
	}
	runs, _ := object["runs"].([]any)
	var text strings.Builder
	for _, run_value := range runs {
		run, _ := run_value.(map[string]any)
		run_text, _ := run["text"].(string)
		text.WriteString(run_text)
	}
	return strings.TrimSpace(text.String())
}

func youtube_thumbnail(value any) string {
	var find func(any) string
	find = func(current any) string {
		switch typed_value := current.(type) {
		case map[string]any:
			if thumbnails, ok := typed_value["thumbnails"].([]any); ok {
				for index := len(thumbnails) - 1; index >= 0; index-- {
					thumbnail, _ := thumbnails[index].(map[string]any)
					if raw_url := json_string(thumbnail["url"]); raw_url != "" {
						return raw_url
					}
				}
			}
			if sources, ok := typed_value["sources"].([]any); ok {
				for index := len(sources) - 1; index >= 0; index-- {
					source, _ := sources[index].(map[string]any)
					if raw_url := json_string(source["url"]); raw_url != "" {
						return raw_url
					}
				}
			}
			for _, child := range typed_value {
				if raw_url := find(child); raw_url != "" {
					return raw_url
				}
			}
		case []any:
			for _, child := range typed_value {
				if raw_url := find(child); raw_url != "" {
					return raw_url
				}
			}
		}
		return ""
	}
	return find(value)
}

func nested_string(value map[string]any, path ...string) string {
	var current any = value
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = object[key]
	}
	return json_string(current)
}

func json_string(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}
