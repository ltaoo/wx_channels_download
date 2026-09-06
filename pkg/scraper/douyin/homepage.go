package douyin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/html"

	"wx_channel/pkg/minib"
)

const (
	douyin_home_timeout       = time.Minute
	douyin_home_attempts      = 2
	douyin_home_cache_ttl     = 10 * time.Minute
	douyin_home_result_marker = `[data-e2e="user-post-list"] a[href*="/video/"], [data-e2e="user-post-list"] a[href*="/note/"]`
	douyin_home_wait_marker   = douyin_home_result_marker + `, [data-e2e="error-page"]`
)

// HomeResult contains the rendered profile page and its embedded bootstrap data.
type HomeResult struct {
	HTML            string          `json:"html"`
	SSRData         string          `json:"ssr_data"`
	InitialData     json.RawMessage `json:"initial_data"`
	Contents        []HomeContent   `json:"contents"`
	NextMarker      string          `json:"next_marker,omitempty"`
	PaginationKnown bool            `json:"pagination_known,omitempty"`
}

// HomeContent is one video or image note rendered in a user's works tab.
type HomeContent struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	CoverURL string `json:"cover_url,omitempty"`
}

// HomeContentList is the structured works list from a Douyin user page.
type HomeContentList struct {
	Source     string        `json:"source"`
	OwnerID    string        `json:"owner_id"`
	Scope      string        `json:"scope"`
	Items      []HomeContent `json:"items"`
	NextMarker string        `json:"next_marker,omitempty"`
	HTML       string        `json:"-"`
}

// FetchHomeContents fetches the works rendered on a Douyin /user/{sec_uid}
// page and returns them as structured content items.
func (c *Client) FetchHomeContents(raw_url string) (*HomeContentList, error) {
	return c.FetchHomeContentsPage(raw_url, "")
}

// FetchHomeContentsPage fetches one works page. page_marker is the opaque
// max_cursor returned by the preceding call; an empty marker fetches page one.
func (c *Client) FetchHomeContentsPage(raw_url string, page_marker string) (*HomeContentList, error) {
	owner_id, page_url, err := parse_home_url(raw_url)
	if err != nil {
		return nil, err
	}
	page_marker, err = normalize_home_page_marker(page_marker)
	if err != nil {
		return nil, err
	}
	var home *HomeResult
	if page_marker == "" {
		home, err = c.fetch_home_prefer_api(owner_id, page_url, true)
	} else {
		home, err = c.fetch_home_api_page(owner_id, page_url, page_marker)
	}
	if err != nil {
		return nil, fmt.Errorf("douyin home: works API failed: %w", err)
	}
	return new_home_content_list(page_url, owner_id, home), nil
}

func new_home_content_list(page_url string, owner_id string, home *HomeResult) *HomeContentList {
	if home == nil {
		return &HomeContentList{Source: page_url, OwnerID: owner_id, Scope: "posts"}
	}
	return &HomeContentList{
		Source: page_url, OwnerID: owner_id, Scope: "posts",
		Items: home.Contents, NextMarker: home.NextMarker, HTML: home.HTML,
	}
}

// FetchHome fetches a Douyin user's works, preferring the JSON API and
// falling back to rendered-page extraction.
func (c *Client) FetchHome(id string) (*HomeResult, error) {
	if c == nil {
		return nil, fmt.Errorf("douyin client is nil")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("douyin home id is empty")
	}
	page_url := "https://www.douyin.com/user/" + url.PathEscape(id)
	home, api_err := c.fetch_home_prefer_api(id, page_url, false)
	if api_err == nil {
		return home, nil
	}
	c.logger.Warn().Err(api_err).Str("url", page_url).Msg("douyin home: works API failed, falling back to rendered page")
	home, err := c.fetch_home_live(id, page_url)
	if err != nil {
		return nil, fmt.Errorf("douyin home: works API failed: %v; rendered-page fallback failed: %w", api_err, err)
	}
	if err := c.write_home_cache(page_url, home); err != nil {
		c.logger.Warn().Err(err).Str("url", page_url).Msg("douyin home: write cache failed")
	}
	return home, nil
}

// fetch_home_prefer_api keeps structured home-content requests on the fast
// path. A stale cache is preferable to blocking those requests on the much
// slower rendered-page compatibility fallback.
func (c *Client) fetch_home_prefer_api(id string, page_url string, require_pagination bool) (*HomeResult, error) {
	if cached_home, ok, cache_err := c.read_home_cache(page_url, douyin_home_cache_ttl); cache_err == nil && ok {
		if !require_pagination || cached_home.PaginationKnown {
			return cached_home, nil
		}
	} else if cache_err != nil {
		c.logger.Warn().Err(cache_err).Str("url", page_url).Msg("douyin home: read cache failed")
	}
	home, api_err := c.fetch_home_api(id, page_url)
	if api_err == nil {
		if err := c.write_home_cache(page_url, home); err != nil {
			c.logger.Warn().Err(err).Str("url", page_url).Msg("douyin home: write cache failed")
		}
		return home, nil
	}
	if cached_home, ok, cache_err := c.read_home_cache(page_url, 0); cache_err == nil && ok {
		if !require_pagination || cached_home.PaginationKnown {
			c.logger.Warn().Err(api_err).Str("url", page_url).Msg("douyin home: works API failed, using stale cache")
			return cached_home, nil
		}
	} else if cache_err != nil {
		c.logger.Warn().Err(cache_err).Str("url", page_url).Msg("douyin home: read stale cache failed")
	}
	return nil, api_err
}

func (c *Client) fetch_home_live(id string, page_url string) (*HomeResult, error) {
	browser, err := minib.NewMiniBrowser(douyin_home_timeout, c.cookie_reader)
	if err != nil {
		return nil, fmt.Errorf("douyin home: create minib browser: %w", err)
	}
	defer browser.Close()

	var page *minib.Page
	for attempt := 1; attempt <= douyin_home_attempts; attempt++ {
		page, err = browser.Navigate(context.Background(), page_url, nil, minib.NavigateOptions{
			DisableImages:     true,
			DisableMedia:      true,
			JavaScriptTimeout: 10 * time.Second,
			ResourceTimeout:   8 * time.Second,
			WaitUntil:         minib.WaitUntilDOMContentLoaded,
			WaitForSelector:   douyin_home_wait_marker,
		})
		if err != nil {
			continue
		}
		if terminal_err := home_terminal_error(id, page.RenderedHTML); terminal_err != nil {
			return nil, terminal_err
		}
		if !strings.Contains(page.RenderedHTML, `data-e2e="error-page"`) {
			break
		}
		err = fmt.Errorf("douyin home: page rendered an error state")
	}
	if err != nil {
		return nil, fmt.Errorf("douyin home: navigate after %d attempts: %w", douyin_home_attempts, err)
	}
	if page.StatusCode < 200 || page.StatusCode >= 300 {
		return nil, fmt.Errorf("douyin home: upstream returned HTTP %d", page.StatusCode)
	}

	ssr_data, initial_data, err := extract_home_bootstrap(page.HTML)
	if err != nil {
		return nil, err
	}
	clean_html, err := home_html_without_scripts(page.RenderedHTML)
	if err != nil {
		return nil, err
	}
	contents, err := ParseHomeContents(clean_html)
	if err != nil {
		return nil, err
	}
	return &HomeResult{
		HTML: clean_html, SSRData: ssr_data, InitialData: initial_data,
		Contents: contents,
	}, nil
}

func (c *Client) read_home_cache(page_url string, max_age time.Duration) (*HomeResult, bool, error) {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() {
		return nil, false, nil
	}
	cache_path := douyin_home_cache_path(page_url)
	file_info, err := c.file_cache.Stat(cache_path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if max_age > 0 && time.Since(file_info.ModTime()) > max_age {
		return nil, false, nil
	}
	data, err := c.file_cache.Read(cache_path)
	if err != nil {
		return nil, false, err
	}
	var home HomeResult
	if err := json.Unmarshal(data, &home); err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(home.HTML) == "" && home.Contents == nil {
		return nil, false, nil
	}
	if home.Contents == nil && strings.TrimSpace(home.HTML) != "" {
		contents, err := ParseHomeContents(home.HTML)
		if err != nil {
			return nil, false, err
		}
		home.Contents = contents
	}
	return &home, true, nil
}

func (c *Client) write_home_cache(page_url string, home *HomeResult) error {
	if c == nil || c.file_cache == nil || !c.file_cache.Enabled() || home == nil || (strings.TrimSpace(home.HTML) == "" && home.Contents == nil) {
		return nil
	}
	data, err := json.Marshal(home)
	if err != nil {
		return err
	}
	return c.file_cache.Write(douyin_home_cache_path(page_url), data)
}

func douyin_home_cache_path(page_url string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(page_url)))
	return "home/" + hex.EncodeToString(digest[:]) + ".json"
}

func parse_home_url(raw_url string) (string, string, error) {
	parsed_url, err := url.Parse(strings.TrimSpace(raw_url))
	if err != nil || parsed_url.Hostname() == "" {
		return "", "", fmt.Errorf("douyin home: invalid user URL")
	}
	if parsed_url.Scheme != "https" && parsed_url.Scheme != "http" {
		return "", "", fmt.Errorf("douyin home: unsupported user URL scheme")
	}
	if !strings.EqualFold(parsed_url.Hostname(), "www.douyin.com") {
		return "", "", fmt.Errorf("douyin home: unsupported user URL host %q", parsed_url.Hostname())
	}
	parts := strings.Split(strings.Trim(parsed_url.EscapedPath(), "/"), "/")
	if len(parts) != 2 || parts[0] != "user" {
		return "", "", fmt.Errorf("douyin home: unsupported user URL")
	}
	owner_id, err := url.PathUnescape(parts[1])
	if err != nil || strings.TrimSpace(owner_id) == "" || strings.Contains(owner_id, "/") {
		return "", "", fmt.Errorf("douyin home: invalid user ID")
	}
	owner_id = strings.TrimSpace(owner_id)
	return owner_id, "https://www.douyin.com/user/" + url.PathEscape(owner_id), nil
}

// ParseHomeContents parses the rendered works tab without requiring adapter
// model types.
func ParseHomeContents(document_html string) ([]HomeContent, error) {
	document, err := html.Parse(strings.NewReader(document_html))
	if err != nil {
		return nil, fmt.Errorf("douyin home: parse rendered works: %w", err)
	}
	post_list := home_find_node(document, "data-e2e", "user-post-list")
	if post_list == nil {
		return nil, fmt.Errorf("douyin home: rendered page has no works list")
	}

	contents := make([]HomeContent, 0)
	seen := make(map[string]struct{})
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "a") {
			if content, ok := home_content_from_anchor(node); ok {
				content_key := content.Type + ":" + content.ID
				if _, exists := seen[content_key]; !exists {
					seen[content_key] = struct{}{}
					contents = append(contents, content)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(post_list)
	return contents, nil
}

func home_content_from_anchor(node *html.Node) (HomeContent, bool) {
	raw_url := strings.TrimSpace(html_attribute(node, "href"))
	parsed_url, err := url.Parse(raw_url)
	if err != nil {
		return HomeContent{}, false
	}
	parts := strings.Split(strings.Trim(parsed_url.Path, "/"), "/")
	if len(parts) != 2 || (parts[0] != "video" && parts[0] != "note") || parts[1] == "" {
		return HomeContent{}, false
	}

	image := home_find_element(node, "img")
	title := ""
	cover_url := ""
	if image != nil {
		title = strings.TrimSpace(html_attribute(image, "alt"))
		cover_url = strings.TrimSpace(html_attribute(image, "src"))
		if cover_url == "" {
			cover_url = strings.TrimSpace(html_attribute(image, "data-src"))
		}
	}
	if title == "" {
		title = parts[1]
	}
	return HomeContent{
		ID:       parts[1],
		Type:     parts[0],
		Title:    title,
		URL:      "https://www.douyin.com/" + parts[0] + "/" + url.PathEscape(parts[1]),
		CoverURL: cover_url,
	}, true
}

func home_find_node(node *html.Node, attribute_name string, attribute_value string) *html.Node {
	if node.Type == html.ElementNode && html_attribute(node, attribute_name) == attribute_value {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := home_find_node(child, attribute_name, attribute_value); found != nil {
			return found
		}
	}
	return nil
}

func home_find_element(node *html.Node, name string) *html.Node {
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, name) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := home_find_element(child, name); found != nil {
			return found
		}
	}
	return nil
}

func home_terminal_error(id string, rendered_html string) error {
	if strings.Contains(rendered_html, "用户不存在") {
		return fmt.Errorf("douyin home: 用户 %q 不存在", id)
	}
	return nil
}

func extract_home_bootstrap(document_html string) (string, json.RawMessage, error) {
	document, err := html.Parse(strings.NewReader(document_html))
	if err != nil {
		return "", nil, fmt.Errorf("douyin home: parse source HTML: %w", err)
	}

	var ssr_data strings.Builder
	var initial_data json.RawMessage
	var walk func(*html.Node) error
	walk = func(node *html.Node) error {
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "script") {
			script_text := html_node_text(node)
			if html_attribute(node, "id") == "RENDER_DATA" {
				decoded_data, decode_err := decode_home_initial_data(script_text)
				if decode_err != nil {
					return decode_err
				}
				initial_data = decoded_data
			}
			chunk, ok, chunk_err := extract_home_ssr_chunk(script_text)
			if chunk_err != nil {
				return chunk_err
			}
			if ok {
				ssr_data.WriteString(chunk)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(document); err != nil {
		return "", nil, err
	}
	return ssr_data.String(), initial_data, nil
}

func decode_home_initial_data(script_text string) (json.RawMessage, error) {
	raw_data := []byte(strings.TrimSpace(script_text))
	if !json.Valid(raw_data) {
		decoded_data, err := url.QueryUnescape(string(raw_data))
		if err != nil {
			return nil, fmt.Errorf("douyin home: decode RENDER_DATA: %w", err)
		}
		raw_data = []byte(decoded_data)
	}
	if !json.Valid(raw_data) {
		return nil, fmt.Errorf("douyin home: RENDER_DATA is not valid JSON")
	}
	return append(json.RawMessage(nil), raw_data...), nil
}

func extract_home_ssr_chunk(script_text string) (string, bool, error) {
	const marker = "self.__pace_f.push("
	marker_index := strings.Index(script_text, marker)
	if marker_index < 0 {
		return "", false, nil
	}
	argument := strings.TrimSpace(script_text[marker_index+len(marker):])
	closing_index := strings.LastIndex(argument, ")")
	if closing_index < 0 {
		return "", false, fmt.Errorf("douyin home: malformed __pace_f push")
	}

	var values []json.RawMessage
	if err := json.Unmarshal([]byte(argument[:closing_index]), &values); err != nil {
		return "", false, fmt.Errorf("douyin home: decode __pace_f push: %w", err)
	}
	if len(values) < 2 || string(values[0]) != "1" {
		return "", false, nil
	}
	var chunk string
	if err := json.Unmarshal(values[1], &chunk); err != nil {
		return "", false, fmt.Errorf("douyin home: decode __pace_f chunk: %w", err)
	}
	return chunk, true, nil
}

func home_html_without_scripts(rendered_html string) (string, error) {
	document, err := html.Parse(strings.NewReader(rendered_html))
	if err != nil {
		return "", fmt.Errorf("douyin home: parse rendered HTML: %w", err)
	}
	remove_home_scripts(document)
	var output bytes.Buffer
	if err := html.Render(&output, document); err != nil {
		return "", fmt.Errorf("douyin home: render cleaned HTML: %w", err)
	}
	return output.String(), nil
}

func remove_home_scripts(node *html.Node) {
	for child := node.FirstChild; child != nil; {
		next_child := child.NextSibling
		if child.Type == html.ElementNode && strings.EqualFold(child.Data, "script") {
			node.RemoveChild(child)
		} else {
			remove_home_scripts(child)
		}
		child = next_child
	}
}

func html_node_text(node *html.Node) string {
	var text strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			text.WriteString(child.Data)
		}
	}
	return text.String()
}

func html_attribute(node *html.Node, name string) string {
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, name) {
			return attribute.Val
		}
	}
	return ""
}
