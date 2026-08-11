package zhihu

import (
	"fmt"
	"sort"
	"strings"
)

// Querying the request host includes cookies scoped to both www.zhihu.com and
// its parent domain, .zhihu.com.
const zhihu_cookie_domain = "www.zhihu.com"

func (c *Client) log_request(method string, raw_url string, cookie_header string) {
	if c == nil || c.logger == nil {
		return
	}
	cookie_count, cookie_names := summarize_cookie_header(cookie_header)
	c.logger.Info().
		Str("component", "zhihu_scraper").
		Str("method", method).
		Str("url", raw_url).
		Bool("cookie_present", cookie_count > 0).
		Int("cookie_count", cookie_count).
		Strs("cookie_names", cookie_names).
		Msg("zhihu outbound request")
}

func (c *Client) log_response(method string, raw_url string, status_code int) {
	if c == nil || c.logger == nil {
		return
	}
	c.logger.Info().
		Str("component", "zhihu_scraper").
		Str("method", method).
		Str("url", raw_url).
		Int("status_code", status_code).
		Msg("zhihu outbound response")
}

func summarize_cookie_header(cookie_header string) (int, []string) {
	cookie_names := make(map[string]struct{})
	cookie_count := 0
	for _, cookie_part := range strings.Split(cookie_header, ";") {
		cookie_part = strings.TrimSpace(cookie_part)
		if cookie_part == "" {
			continue
		}
		cookie_name, _, has_value := strings.Cut(cookie_part, "=")
		cookie_name = strings.TrimSpace(cookie_name)
		if !has_value || cookie_name == "" {
			continue
		}
		cookie_count++
		cookie_names[cookie_name] = struct{}{}
	}
	unique_names := make([]string, 0, len(cookie_names))
	for cookie_name := range cookie_names {
		unique_names = append(unique_names, cookie_name)
	}
	sort.Strings(unique_names)
	return cookie_count, unique_names
}

func (c *Client) Fetch(rawURL string) (any, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("知乎URL不能为空")
	}

	resolvedURL := ResolveRealURL(rawURL)
	if articleURL, ok := ParseArticleURL(resolvedURL); ok {
		return c.FetchArticlePage(articleURL.Canonical)
	}
	if questionURL, ok := ParseQuestionURL(resolvedURL); ok {
		return c.FetchQuestionPage(questionURL.Canonical)
	}
	if answerURL, ok := ParseAnswerURL(resolvedURL); ok {
		return c.FetchAnswerPage(answerURL.Canonical)
	}
	return nil, fmt.Errorf("不支持的知乎URL: %s", rawURL)
}
