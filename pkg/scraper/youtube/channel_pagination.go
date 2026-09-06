package youtube

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ChannelBrowseRequest is the minimum authenticated-or-public youtubei browse
// request derived from the channel page bootstrap configuration.
type ChannelBrowseRequest struct {
	URL     string
	Body    []byte
	Headers http.Header
}

// BuildChannelBrowseRequest creates a continuation request without retaining
// cookies or authorization values from a captured browser session.
func BuildChannelBrowseRequest(page_html string, page_url string, continuation string) (*ChannelBrowseRequest, error) {
	continuation = strings.TrimSpace(continuation)
	if continuation == "" {
		return nil, fmt.Errorf("youtube channel: continuation is empty")
	}
	raw_config, ok, err := ExtractYTCfgJSON([]byte(page_html))
	if err != nil {
		return nil, fmt.Errorf("youtube channel: extract ytcfg: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("youtube channel: ytcfg is missing")
	}
	var config map[string]any
	if err := json.Unmarshal(raw_config, &config); err != nil {
		return nil, fmt.Errorf("youtube channel: decode ytcfg: %w", err)
	}
	api_key := json_config_string(config["INNERTUBE_API_KEY"])
	if api_key == "" {
		return nil, fmt.Errorf("youtube channel: INNERTUBE_API_KEY is missing")
	}
	context_value, ok := config["INNERTUBE_CONTEXT"].(map[string]any)
	if !ok || context_value == nil {
		return nil, fmt.Errorf("youtube channel: INNERTUBE_CONTEXT is missing")
	}
	body, err := json.Marshal(map[string]any{
		"context":      context_value,
		"continuation": continuation,
	})
	if err != nil {
		return nil, fmt.Errorf("youtube channel: encode browse request: %w", err)
	}

	request_url := "https://www.youtube.com/youtubei/v1/browse"
	parsed_page_url, parse_err := url.Parse(strings.TrimSpace(page_url))
	if parse_err == nil && parsed_page_url.Scheme == "https" && youtube_channel_hostname(parsed_page_url.Hostname()) {
		request_url = parsed_page_url.Scheme + "://" + parsed_page_url.Host + "/youtubei/v1/browse"
	}
	parsed_request_url, _ := url.Parse(request_url)
	query := parsed_request_url.Query()
	query.Set("prettyPrint", "false")
	query.Set("key", api_key)
	parsed_request_url.RawQuery = query.Encode()

	client_context, _ := context_value["client"].(map[string]any)
	client_version := json_config_string(config["INNERTUBE_CLIENT_VERSION"])
	if client_version == "" {
		client_version = json_config_string(client_context["clientVersion"])
	}
	client_name := json_config_number_string(config["INNERTUBE_CONTEXT_CLIENT_NAME"])
	if client_name == "" {
		client_name = "1"
	}
	visitor_data := json_config_string(config["VISITOR_DATA"])
	if visitor_data == "" {
		visitor_data = json_config_string(client_context["visitorData"])
	}
	headers := make(http.Header)
	headers.Set("Accept", "*/*")
	headers.Set("Content-Type", "application/json")
	headers.Set("Origin", "https://www.youtube.com")
	if strings.TrimSpace(page_url) != "" {
		headers.Set("Referer", strings.TrimSpace(page_url))
	}
	headers.Set("X-YouTube-Client-Name", client_name)
	if client_version != "" {
		headers.Set("X-YouTube-Client-Version", client_version)
	}
	if visitor_data != "" {
		headers.Set("X-Goog-Visitor-Id", visitor_data)
	}
	return &ChannelBrowseRequest{URL: parsed_request_url.String(), Body: body, Headers: headers}, nil
}

func youtube_channel_hostname(hostname string) bool {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	return hostname == "youtube.com" || strings.HasSuffix(hostname, ".youtube.com")
}

func json_config_string(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func json_config_number_string(value any) string {
	switch typed_value := value.(type) {
	case string:
		return strings.TrimSpace(typed_value)
	case float64:
		return strconv.FormatInt(int64(typed_value), 10)
	case json.Number:
		return typed_value.String()
	default:
		return ""
	}
}
