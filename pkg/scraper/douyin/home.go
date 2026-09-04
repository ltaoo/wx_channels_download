package douyin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"

	"wx_channel/pkg/minib"
)

const (
	douyin_home_timeout       = 2 * time.Minute
	douyin_home_attempts      = 3
	douyin_home_result_marker = `[data-e2e="user-post-list"] a[href*="/video/"], [data-e2e="user-post-list"] a[href*="/note/"]`
	douyin_home_wait_marker   = douyin_home_result_marker + `, [data-e2e="error-page"]`
)

// HomeResult contains the rendered profile page and its embedded bootstrap data.
type HomeResult struct {
	HTML        string          `json:"html"`
	SSRData     string          `json:"ssr_data"`
	InitialData json.RawMessage `json:"initial_data"`
}

// FetchHome renders a Douyin user homepage using persistent cookies.
func (c *Client) FetchHome(id string) (*HomeResult, error) {
	if c == nil {
		return nil, fmt.Errorf("douyin client is nil")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("douyin home id is empty")
	}

	browser, err := minib.NewMiniBrowser(douyin_home_timeout, c.cookie_reader)
	if err != nil {
		return nil, fmt.Errorf("douyin home: create minib browser: %w", err)
	}
	defer browser.Close()

	page_url := "https://www.douyin.com/user/" + url.PathEscape(id)
	var page *minib.Page
	for attempt := 1; attempt <= douyin_home_attempts; attempt++ {
		page, err = browser.Navigate(context.Background(), page_url, nil, minib.NavigateOptions{
			DisableCache:    true,
			WaitForSelector: douyin_home_wait_marker,
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
	return &HomeResult{HTML: clean_html, SSRData: ssr_data, InitialData: initial_data}, nil
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
