package youtube

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"wx_channel/pkg/scraper/internal/homepage"
)

// FetchChannelContentsPage retrieves and parses one channel tab page.
func (c *Client) FetchChannelContentsPage(fetch_context context.Context, channel_url string, scope string, page_marker string) (*ChannelContentPage, error) {
	if c == nil {
		return nil, fmt.Errorf("youtube client is nil")
	}
	channel_url = strings.TrimRight(strings.TrimSpace(channel_url), "/")
	if channel_url == "" {
		return nil, fmt.Errorf("YouTube channel URL is empty")
	}
	scope = strings.TrimSpace(scope)
	page_url := channel_url + "/" + scope
	document_html, err := homepage.Fetch(fetch_context, homepage.FetchOptions{
		URL: page_url, Scope: scope, AccountID: channel_url,
		DisableSubresources: true, DisableJavaScript: true,
		CookieReader: c.CookieReader, Cache: c.Cache,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch YouTube channel %s: %w", scope, err)
	}
	if strings.TrimSpace(page_marker) == "" {
		return ParseChannelContentsPage(document_html, scope)
	}
	browse_request, err := BuildChannelBrowseRequest(document_html, page_url, page_marker)
	if err != nil {
		return nil, err
	}
	response_json, err := homepage.Request(fetch_context, homepage.RequestOptions{
		URL: browse_request.URL, Method: http.MethodPost, Body: browse_request.Body,
		Scope: scope, AccountID: channel_url, Headers: browse_request.Headers,
		CookieReader: c.CookieReader, Cache: c.Cache,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch YouTube channel %s continuation: %w", scope, err)
	}
	return ParseChannelContinuation(response_json, scope)
}
