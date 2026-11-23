package x

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"wx_channel/pkg/scraper/internal/homepage"
)

var (
	main_script_pattern = regexp.MustCompile(`https://abs\.twimg\.com/responsive-web/client-web/main\.[a-zA-Z0-9_-]+\.js`)
	bearer_pattern      = regexp.MustCompile(`AAAAA[A-Za-z0-9%_-]{60,}`)
	feature_pattern     = regexp.MustCompile(`"([a-zA-Z0-9_]+)"`)
)

// TimelineItem is one normalized post from an X account timeline.
type TimelineItem struct {
	ExternalID   string
	Username     string
	Text         string
	CoverURL     string
	PublishTime  int64
	ViewCount    int64
	LikeCount    int64
	CommentCount int64
	ShareCount   int64
}

// FetchTimelineContext retrieves the first page of one X account timeline.
func (c *Client) FetchTimelineContext(fetch_context context.Context, user_id string, username string, scope string) ([]TimelineItem, error) {
	if c == nil || c.browser == nil {
		return nil, fmt.Errorf("x client is closed")
	}
	if c.cookie_provider == nil {
		return nil, fmt.Errorf("缺少 cookies.json X 凭证")
	}
	user_id = strings.TrimSpace(user_id)
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if user_id == "" || username == "" {
		return nil, fmt.Errorf("X account ID and username are required")
	}
	operation_names := map[string]string{
		"posts": "UserTweets", "replies": "UserTweetsAndReplies",
		"reposts": "UserRepostsTimeline", "media": "UserMedia",
	}
	operation_name, ok := operation_names[strings.TrimSpace(scope)]
	if !ok {
		return nil, fmt.Errorf("unsupported X home scope: %s", scope)
	}
	profile_url := "https://x.com/" + url.PathEscape(username)
	profile_html, err := homepage.Fetch(fetch_context, homepage.FetchOptions{
		URL: profile_url, Scope: "profile", AccountID: user_id,
		DisableSubresources: true, DisableJavaScript: true,
		CookieReader: c.cookie_provider, Cache: c.file_cache,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch X profile entry: %w", err)
	}
	main_script_url := main_script_pattern.FindString(profile_html)
	if main_script_url == "" {
		return nil, fmt.Errorf("X profile is missing main script")
	}
	main_script, err := homepage.Request(fetch_context, homepage.RequestOptions{
		URL: main_script_url, Scope: "client", AccountID: "main",
		CookieReader: c.cookie_provider, Cache: c.file_cache,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch X client definition: %w", err)
	}
	query_id, features, err := operation_info(string(main_script), operation_name)
	if err != nil {
		return nil, err
	}
	bearer := bearer_pattern.FindString(string(main_script))
	if bearer == "" {
		return nil, fmt.Errorf("X client is missing bearer token")
	}
	if decoded_bearer, decode_err := url.QueryUnescape(bearer); decode_err == nil {
		bearer = decoded_bearer
	}
	cookie_header, err := c.cookie_provider.HeaderForURL(profile_url)
	if err != nil {
		return nil, fmt.Errorf("读取 cookies.json X 凭证失败: %w", err)
	}
	csrf_token := cookie_header_value(cookie_header, "ct0")
	if csrf_token == "" || cookie_header_value(cookie_header, "auth_token") == "" {
		return nil, fmt.Errorf("cookies.json 缺少 X 的 ct0 或 auth_token")
	}
	variables_json, _ := json.Marshal(map[string]any{
		"userId": user_id, "count": 20, "includePromotedContent": false,
		"withQuickPromoteEligibilityTweetFields": true, "withVoice": true,
	})
	features_json, _ := json.Marshal(features)
	field_toggles_json, _ := json.Marshal(map[string]bool{"withArticlePlainText": false})
	query := url.Values{
		"variables": {string(variables_json)}, "features": {string(features_json)},
		"fieldToggles": {string(field_toggles_json)},
	}
	api_url := "https://x.com/i/api/graphql/" + query_id + "/" + operation_name + "?" + query.Encode()
	response_json, err := homepage.Request(fetch_context, homepage.RequestOptions{
		URL: api_url, Scope: scope, AccountID: user_id,
		Headers: http.Header{
			"Accept": {"*/*"}, "Authorization": {"Bearer " + bearer}, "Referer": {profile_url},
			"X-Csrf-Token": {csrf_token}, "X-Twitter-Active-User": {"yes"},
			"X-Twitter-Auth-Type": {"OAuth2Session"}, "X-Twitter-Client-Language": {"en"},
		},
		CookieReader: c.cookie_provider, Cache: c.file_cache,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch X profile %s: %w", scope, err)
	}
	return parse_timeline_response(response_json, username)
}

func operation_info(script string, operation_name string) (string, map[string]bool, error) {
	operation_pattern := regexp.MustCompile(`queryId:"([^"]+)",operationName:"` + regexp.QuoteMeta(operation_name) + `"`)
	operation_match := operation_pattern.FindStringSubmatchIndex(script)
	if len(operation_match) < 4 {
		return "", nil, fmt.Errorf("X client is missing %s operation definition", operation_name)
	}
	window_end := operation_match[1] + 20000
	if window_end > len(script) {
		window_end = len(script)
	}
	feature_match := regexp.MustCompile(`featureSwitches:\[([^\]]*)\]`).FindStringSubmatch(script[operation_match[1]:window_end])
	if len(feature_match) < 2 {
		return "", nil, fmt.Errorf("X client is missing %s feature switches", operation_name)
	}
	features := make(map[string]bool)
	for _, name_match := range feature_pattern.FindAllStringSubmatch(feature_match[1], -1) {
		if len(name_match) > 1 {
			features[name_match[1]] = false
		}
	}
	for _, name := range []string{"responsive_web_graphql_timeline_navigation_enabled", "longform_notetweets_consumption_enabled", "rweb_video_screen_enabled"} {
		if _, exists := features[name]; exists {
			features[name] = true
		}
	}
	return script[operation_match[2]:operation_match[3]], features, nil
}

func parse_timeline_response(response_json []byte, fallback_username string) ([]TimelineItem, error) {
	var response any
	if err := json.Unmarshal(response_json, &response); err != nil {
		return nil, fmt.Errorf("decode X profile response: %w", err)
	}
	if response_map, ok := response.(map[string]any); ok {
		if errors_value, exists := response_map["errors"]; exists {
			errors_json, _ := json.Marshal(errors_value)
			return nil, fmt.Errorf("X profile API rejected request: %s", string(errors_json))
		}
	}
	items := make([]TimelineItem, 0)
	seen := make(map[string]struct{})
	append_tweet := func(value any) {
		tweet := unwrap_tweet(value)
		if tweet == nil {
			return
		}
		legacy, _ := tweet["legacy"].(map[string]any)
		tweet_id := json_string(tweet["rest_id"])
		if tweet_id == "" || json_string(legacy["full_text"]) == "" {
			return
		}
		if _, exists := seen[tweet_id]; exists {
			return
		}
		seen[tweet_id] = struct{}{}
		items = append(items, timeline_item(tweet, legacy, fallback_username))
	}
	var walk func(any)
	walk = func(value any) {
		switch typed_value := value.(type) {
		case map[string]any:
			if tweet_results, ok := typed_value["tweet_results"].(map[string]any); ok {
				append_tweet(tweet_results["result"])
			}
			for key, child := range typed_value {
				if key != "tweet_results" {
					walk(child)
				}
			}
		case []any:
			for _, child := range typed_value {
				walk(child)
			}
		}
	}
	walk(response)
	return items, nil
}

func unwrap_tweet(value any) map[string]any {
	tweet, _ := value.(map[string]any)
	for tweet != nil {
		if _, has_legacy := tweet["legacy"].(map[string]any); has_legacy && json_string(tweet["rest_id"]) != "" {
			return tweet
		}
		wrapped_tweet, ok := tweet["tweet"].(map[string]any)
		if !ok {
			return nil
		}
		tweet = wrapped_tweet
	}
	return nil
}

func timeline_item(tweet map[string]any, legacy map[string]any, fallback_username string) TimelineItem {
	username := fallback_username
	if core, ok := tweet["core"].(map[string]any); ok {
		if user_results, ok := core["user_results"].(map[string]any); ok {
			if result, ok := user_results["result"].(map[string]any); ok {
				if user_legacy, ok := result["legacy"].(map[string]any); ok {
					if value := json_string(user_legacy["screen_name"]); value != "" {
						username = value
					}
				}
			}
		}
	}
	publish_time := int64(0)
	if value, err := time.Parse(time.RubyDate, json_string(legacy["created_at"])); err == nil {
		publish_time = value.UnixMilli()
	}
	return TimelineItem{
		ExternalID: json_string(tweet["rest_id"]), Username: username,
		Text: json_string(legacy["full_text"]), CoverURL: recursive_string(legacy["extended_entities"], "media_url_https"),
		PublishTime: publish_time, ViewCount: json_int64(tweet["views"]),
		LikeCount: json_int64(legacy["favorite_count"]), CommentCount: json_int64(legacy["reply_count"]),
		ShareCount: json_int64(legacy["retweet_count"]),
	}
}

func cookie_header_value(cookie_header string, name string) string {
	for _, field := range strings.Split(cookie_header, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(field), "=")
		if ok && key == name {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func json_string(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func json_int64(value any) int64 {
	switch typed_value := value.(type) {
	case float64:
		return int64(typed_value)
	case string:
		var number int64
		_, _ = fmt.Sscan(typed_value, &number)
		return number
	case map[string]any:
		return json_int64(typed_value["count"])
	default:
		return 0
	}
}

func recursive_string(value any, key string) string {
	switch typed_value := value.(type) {
	case map[string]any:
		if text := json_string(typed_value[key]); text != "" {
			return text
		}
		for _, child := range typed_value {
			if text := recursive_string(child, key); text != "" {
				return text
			}
		}
	case []any:
		for _, child := range typed_value {
			if text := recursive_string(child, key); text != "" {
				return text
			}
		}
	}
	return ""
}
