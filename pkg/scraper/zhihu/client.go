package zhihu

import (
	"fmt"
	"strings"
)

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
