package xiaohongshu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"wx_channel/pkg/minib"
)

const home_fetch_timeout = 90 * time.Second

// FetchHomeContents fetches and parses a Xiaohongshu profile tab.
func (c *Client) FetchHomeContents(raw_url string, scope string) (*HomeContentList, error) {
	return c.FetchHomeContentsContext(context.Background(), raw_url, scope)
}

// FetchHomeContentsContext fetches and parses a Xiaohongshu profile tab with
// cancellation support.
func (c *Client) FetchHomeContentsContext(fetch_context context.Context, raw_url string, scope string) (*HomeContentList, error) {
	if c == nil {
		return nil, fmt.Errorf("xiaohongshu client is nil")
	}
	scope, query_tab, err := normalize_home_scope(scope)
	if err != nil {
		return nil, err
	}
	profile_url, err := normalize_profile_url(raw_url)
	if err != nil {
		return nil, err
	}
	parsed_url, _ := url.Parse(profile_url)
	query := parsed_url.Query()
	query.Set("tab", query_tab)
	parsed_url.RawQuery = query.Encode()
	page_url := parsed_url.String()

	document_html, err := c.fetch_home_html(fetch_context, page_url, scope)
	if err != nil {
		return nil, err
	}
	result, err := ParseHomeContents(document_html, scope)
	if err != nil {
		return nil, err
	}
	result.Source = page_url
	return result, nil
}

func home_items_are_redacted(items []HomeNoteItem) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if first_non_empty(item.NoteCard.NoteID, item.ID) != "" {
			return false
		}
	}
	return true
}

func (c *Client) fetch_home_html(fetch_context context.Context, page_url string, scope string) (string, error) {
	if fetch_context == nil {
		fetch_context = context.Background()
	}
	fetch_context, cancel_fetch := context.WithTimeout(fetch_context, home_fetch_timeout)
	defer cancel_fetch()
	browser, err := minib.NewMiniBrowser(home_fetch_timeout, c.cookie_reader)
	if err != nil {
		return "", fmt.Errorf("xiaohongshu home: create browser: %w", err)
	}
	defer browser.Close()

	page, err := browser.Navigate(fetch_context, page_url, nil, minib.NavigateOptions{
		DisableSubresources: true,
		DisableImages:       true,
		DisableMedia:        true,
		DisableJavaScript:   true,
		ResourceTimeout:     20 * time.Second,
	})
	if err != nil {
		if cached_html, cache_err := c.read_home_cache(page_url, scope); cache_err == nil && cached_html != "" {
			return cached_html, nil
		}
		return "", fmt.Errorf("xiaohongshu home: navigate: %w", err)
	}
	if page.StatusCode < http.StatusOK || page.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("xiaohongshu home: upstream returned HTTP %d", page.StatusCode)
	}
	document_html := strings.TrimSpace(page.RenderedHTML)
	if document_html == "" {
		document_html = strings.TrimSpace(page.HTML)
	}
	if document_html == "" {
		return "", fmt.Errorf("xiaohongshu home: page returned empty HTML")
	}
	if err := c.write_home_cache(page_url, scope, document_html); err != nil {
		return "", fmt.Errorf("xiaohongshu home: cache page: %w", err)
	}
	return document_html, nil
}

func normalize_profile_url(raw_url string) (string, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || parsed_url == nil {
		return "", fmt.Errorf("xiaohongshu home: invalid profile URL")
	}
	if err := validate_request_url(parsed_url); err != nil {
		return "", err
	}
	if !strings.HasPrefix(parsed_url.Path, "/user/profile/") {
		return "", fmt.Errorf("xiaohongshu home: unsupported profile URL")
	}
	return parsed_url.String(), nil
}

func (c *Client) write_home_cache(page_url string, scope string, document_html string) error {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() {
		return nil
	}
	return c.file_cache.Write(home_cache_path(page_url, scope), []byte(document_html))
}

func (c *Client) read_home_cache(page_url string, scope string) (string, error) {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() {
		return "", nil
	}
	data, err := c.file_cache.Read(home_cache_path(page_url, scope))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func home_cache_path(page_url string, scope string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(scope) + "\x00" + strings.TrimSpace(page_url)))
	return "home/" + hex.EncodeToString(digest[:]) + ".html"
}
