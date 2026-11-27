package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

var query_token_pattern = regexp.MustCompile(`\[\s*"(LSD|DTSGInitialData)"\s*,\s*\[\s*\]\s*,\s*(\{[^{}]*\})`)

// HomeItem is a profile grid entry. Media download URLs require fetching its post.
type HomeItem struct {
	ExternalID string `json:"external_id"`
	Shortcode  string `json:"shortcode"`
	SourceURL  string `json:"source_url"`
	BodyText   string `json:"body_text"`
	CoverURL   string `json:"cover_url"`
	MediaType  int64  `json:"media_type"`
}

type HomeResult struct {
	Account    *Account   `json:"account"`
	Items      []HomeItem `json:"items"`
	NextMarker string     `json:"next_marker,omitempty"`
}

// FetchHomeContents returns the first posts page and its continuation marker.
func (c *Client) FetchHomeContents(account_url string) (*HomeResult, error) {
	return c.FetchHomeContentsContext(context.Background(), account_url)
}

func (c *Client) FetchHomeContentsContext(fetch_context context.Context, account_url string) (*HomeResult, error) {
	return c.FetchHomeContentsPageContext(fetch_context, account_url, "")
}

// FetchHomeContentsPage fetches one page using the preceding result's opaque
// NextMarker. An empty marker fetches the first page.
func (c *Client) FetchHomeContentsPage(account_url string, page_marker string) (*HomeResult, error) {
	return c.FetchHomeContentsPageContext(context.Background(), account_url, page_marker)
}

func (c *Client) FetchHomeContentsPageContext(fetch_context context.Context, account_url string, page_marker string) (*HomeResult, error) {
	if fetch_context == nil {
		fetch_context = context.Background()
	}
	username, err := ExtractUsername(account_url)
	if err != nil {
		return nil, err
	}
	page_html, err := c.fetch_page(fetch_context, "https://www.instagram.com/"+username+"/")
	if err != nil {
		return nil, fmt.Errorf("instagram: fetch homepage bootstrap: %w", err)
	}
	var parse_err error
	if page_marker == "" {
		var result *HomeResult
		result, parse_err = ParseHomeContents(username, page_html)
		if parse_err == nil {
			return result, nil
		}
	}
	form := url.Values{}
	for _, match := range query_token_pattern.FindAllStringSubmatch(page_html, -1) {
		token := gjson.Get(match[2], "token").String()
		if token == "" {
			continue
		}
		if match[1] == "LSD" {
			form.Set("lsd", token)
		} else {
			form.Set("fb_dtsg", token)
			checksum := 0
			for _, char := range token {
				checksum += int(char)
			}
			form.Set("jazoest", "2"+strconv.Itoa(checksum))
		}
	}
	if form.Get("lsd") == "" {
		if parse_err != nil {
			return nil, parse_err
		}
		return nil, fmt.Errorf("instagram: homepage query token is missing from embedded page data")
	}
	if form.Get("jazoest") == "" {
		checksum := 0
		for _, char := range form.Get("lsd") {
			checksum += int(char)
		}
		form.Set("jazoest", "2"+strconv.Itoa(checksum))
	}
	account, account_err := ParseAccount(username, page_html)
	relay_id := ""
	if account_err == nil {
		record := find_page_record(page_html, func(record gjson.Result) bool {
			return record.Get("pk").String() == account.ExternalID && record.Get("id").Type == gjson.String && record.Get("id").String() != ""
		})
		relay_id = record.Get("id").String()
	}
	// Keep the whole traversal anonymous when login cookies select a profile shell.
	if c.cookie_reader != nil && (account_err != nil || (page_marker != "" && relay_id == "")) {
		anonymous_client, err := NewClient()
		if err != nil {
			return nil, err
		}
		defer anonymous_client.Close()
		return anonymous_client.FetchHomeContentsPageContext(fetch_context, account_url, page_marker)
	}
	query_variables := map[string]any{
		"username": username,
		"data": map[string]any{"count": 12, "include_reel_media_seen_timestamp": true,
			"include_relationship_info": true, "latest_besties_reel_media": true, "latest_reel_media": true},
		"__relay_internal__pv__PolarisMultiCaptionCarouselEnabledrelayprovider":  true,
		"__relay_internal__pv__PolarisShortDramaEnabledrelayprovider":            true,
		"__relay_internal__pv__PolarisSEOIsCrawlerrelayprovider":                 false,
		"__relay_internal__pv__PolarisProductTagIndicatorEnabledrelayprovider":   false,
		"__relay_internal__pv__PolarisThreadsStyleCommentsEnabledrelayprovider":  false,
		"__relay_internal__pv__PolarisReelsRecoDebugOverlayEnabledrelayprovider": false,
	}
	query_name, document_id := "PolarisProfilePostsQuery", "30543471925243353"
	query_url := "https://www.instagram.com/graphql/query"
	if page_marker != "" {
		if account_err != nil {
			return nil, account_err
		}
		if relay_id == "" {
			return nil, fmt.Errorf("instagram: homepage Relay account ID is missing")
		}
		query_name, document_id = "PolarisLoggedOutDesktopWWWProfilePostsTabContentQuery_connection", "28904005159232689"
		query_url = "https://www.instagram.com/api/graphql"
		query_variables = map[string]any{
			"id": relay_id, "after": page_marker, "first": 12,
			"__relay_internal__pv__PolarisSEOIsCrawlerrelayprovider": false,
		}
	}
	variables, err := json.Marshal(query_variables)
	if err != nil {
		return nil, fmt.Errorf("instagram: encode homepage query: %w", err)
	}
	for key, value := range map[string]string{
		"__a": "1", "__d": "www", "__user": "0", "__comet_req": "7",
		"fb_api_caller_class": "RelayModern", "fb_api_req_friendly_name": query_name,
		"server_timestamps": "true", "variables": string(variables),
		// shortcut: verified query document ID; update when Instagram changes this query.
		"doc_id": document_id,
	} {
		form.Set(key, value)
	}
	request, err := http.NewRequestWithContext(fetch_context, http.MethodPost, query_url, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("instagram: create homepage query: %w", err)
	}
	request.Header.Set("User-Agent", DefaultUserAgent)
	request.Header.Set("Accept", "*/*")
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://www.instagram.com")
	request.Header.Set("Referer", "https://www.instagram.com/"+username+"/")
	request.Header.Set("X-IG-App-ID", "936619743392459")
	request.Header.Set("X-FB-Friendly-Name", query_name)
	request.Header.Set("X-FB-LSD", form.Get("lsd"))
	request.Header.Set("X-ASBD-ID", "359341")
	request.Header.Set("X-IG-Max-Touch-Points", "0")
	site_data := find_page_record(page_html, func(record gjson.Result) bool {
		return record.Get("csrf_token").Type == gjson.String && record.Get("csrf_token").String() != ""
	})
	if csrf_token := site_data.Get("csrf_token").String(); csrf_token != "" {
		request.Header.Set("X-CSRFToken", csrf_token)
	}
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	request.Header.Set("Sec-Fetch-Mode", "cors")
	request.Header.Set("Sec-Fetch-Dest", "empty")
	query_body, err := c.fetch_request(request)
	if err != nil {
		return nil, fmt.Errorf("instagram: fetch homepage query: %w", err)
	}
	query_body = strings.TrimPrefix(strings.TrimSpace(query_body), "for (;;);")
	if !gjson.Valid(query_body) {
		return nil, fmt.Errorf("instagram: homepage query returned invalid JSON")
	}
	response := gjson.Parse(query_body)
	if code := response.Get("error").Int(); code != 0 {
		return nil, fmt.Errorf("instagram: homepage query failed (code %d)", code)
	}
	const connection = "xdt_api__v1__feed__user_timeline_graphql_connection"
	for _, query_error := range response.Get("errors").Array() {
		if query_error.Get("code").Int() == 1675004 {
			return nil, fmt.Errorf("instagram: homepage query rate limited")
		}
		error_path := query_error.Get("path").Array()
		if page_marker != "" || query_error.Get("severity").String() != "ERROR" || len(error_path) != 6 ||
			error_path[0].String() != connection || error_path[1].String() != "edges" ||
			error_path[3].String() != "node" || error_path[4].String() != "location" || error_path[5].String() != "profile_pic_url" {
			return nil, fmt.Errorf("instagram: homepage query failed")
		}
	}
	connection_path := "data." + connection
	if page_marker != "" {
		if response.Get("data.node.id").String() != relay_id {
			return nil, fmt.Errorf("instagram: homepage query account ID does not match")
		}
		connection_path = "data.node.polaris_ordered_timeline_connection"
	}
	edges := response.Get(connection_path + ".edges")
	if !edges.IsArray() {
		return nil, fmt.Errorf("instagram: homepage query posts are missing or unavailable")
	}
	if account_err != nil {
		account_record := edges.Get("0.node.user")
		if !strings.EqualFold(account_record.Get("username").String(), username) {
			return nil, account_err
		}
		account, err = parse_account(account_record)
		if err != nil {
			return nil, err
		}
	}
	result, err := parse_home_items(account, response.Get(connection_path))
	if err != nil {
		return nil, err
	}
	if page_marker != "" && result.NextMarker == page_marker {
		return nil, fmt.Errorf("instagram: homepage query returned a repeated pagination cursor")
	}
	return result, nil
}

func ParseHomeContents(account_url string, page_html string) (*HomeResult, error) {
	account, err := ParseAccount(account_url, page_html)
	if err != nil {
		return nil, err
	}
	// Account details and grid data arrive in separate Relay records.
	record := find_page_record(page_html, func(record gjson.Result) bool {
		return record.Get("pk").String() == account.ExternalID && record.Get("polaris_ordered_timeline_connection.edges").IsArray()
	})
	if !record.Exists() {
		return nil, fmt.Errorf("instagram: homepage posts are missing from embedded page data or unavailable")
	}
	return parse_home_items(account, record.Get("polaris_ordered_timeline_connection"))
}

func parse_home_items(account *Account, connection gjson.Result) (*HomeResult, error) {
	result := &HomeResult{Account: account, Items: make([]HomeItem, 0)}
	page_info := connection.Get("page_info")
	has_next_page := page_info.Get("has_next_page")
	if has_next_page.Type != gjson.True && has_next_page.Type != gjson.False {
		return nil, fmt.Errorf("instagram: homepage pagination info is missing or invalid")
	}
	if has_next_page.Bool() {
		cursor := page_info.Get("end_cursor")
		if cursor.Type != gjson.String || strings.TrimSpace(cursor.String()) == "" {
			return nil, fmt.Errorf("instagram: homepage next page cursor is missing or invalid")
		}
		result.NextMarker = cursor.String()
	}
	seen_ids := make(map[string]bool)
	for _, edge := range connection.Get("edges").Array() {
		node := edge.Get("node")
		item := HomeItem{ExternalID: node.Get("pk").String(), Shortcode: node.Get("code").String(),
			BodyText: node.Get("caption.text").String(), CoverURL: NormalizeMediaURL(node.Get("display_uri").String()),
			MediaType: node.Get("media_type").Int()}
		if item.CoverURL == "" {
			item.CoverURL = best_candidate(node.Get("image_versions2.candidates"))
		}
		if item.ExternalID == "" || !shortcode_pattern.MatchString(item.Shortcode) || (item.MediaType != 1 && item.MediaType != 2 && item.MediaType != 8) {
			return nil, fmt.Errorf("instagram: invalid homepage post data")
		}
		if seen_ids[item.ExternalID] {
			continue
		}
		seen_ids[item.ExternalID] = true
		item.SourceURL = "https://www.instagram.com/p/" + item.Shortcode + "/"
		result.Items = append(result.Items, item)
	}
	return result, nil
}
