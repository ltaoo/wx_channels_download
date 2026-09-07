package instagram

import (
	"context"
	"fmt"

	"github.com/tidwall/gjson"
)

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
	Account *Account   `json:"account"`
	Items   []HomeItem `json:"items"`
}

// FetchHomeContents returns the posts embedded in the account homepage.
func (c *Client) FetchHomeContents(account_url string) (*HomeResult, error) {
	return c.FetchHomeContentsContext(context.Background(), account_url)
}

func (c *Client) FetchHomeContentsContext(fetch_context context.Context, account_url string) (*HomeResult, error) {
	username, err := ExtractUsername(account_url)
	if err != nil {
		return nil, err
	}
	page_html, err := c.fetch_page(fetch_context, "https://www.instagram.com/"+username+"/")
	if err != nil {
		return nil, err
	}
	return ParseHomeContents(username, page_html)
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
	result := &HomeResult{Account: account, Items: make([]HomeItem, 0)}
	seen_ids := make(map[string]bool)
	for _, edge := range record.Get("polaris_ordered_timeline_connection.edges").Array() {
		node := edge.Get("node")
		item := HomeItem{ExternalID: node.Get("pk").String(), Shortcode: node.Get("code").String(),
			BodyText: node.Get("caption.text").String(), CoverURL: NormalizeMediaURL(node.Get("display_uri").String()),
			MediaType: node.Get("media_type").Int()}
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
