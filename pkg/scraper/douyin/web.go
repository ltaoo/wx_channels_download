package douyin

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
)

// DouyinWebClient is the Douyin web scraper (fetches via API, requires cookie).
type DouyinWebClient struct {
	cookie string
}

// NewDouyinWebClient creates a new Douyin web scraper.
func NewDouyinWebClient(cookie string) *DouyinWebClient {
	return &DouyinWebClient{cookie: cookie}
}

// FetchVideoProfile retrieves video details by aweme_id.
func (c *DouyinWebClient) FetchVideoProfile(awemeID string) (*DouyinWebVideoProfileResp, error) {
	params := make(map[string]string)
	for k, v := range defaultParams {
		params[k] = v
	}
	params["aweme_id"] = awemeID
	params["msToken"] = ""

	ab := NewABogus("")
	aBogus := ab.GetValue(params, paramOrder, "GET", 0, 0, nil, nil, nil)

	headers := map[string]interface{}{
		"Accept-Language": "zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2",
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.212 Safari/537.36",
		"Referer":         "https://www.douyin.com/",
		"Cookie":          c.cookie,
	}

	search := queryStringify(params, paramOrder) + "&a_bogus=" + aBogus
	apiURL := "https://www.douyin.com/aweme/v1/web/aweme/detail/?" + search

	client := NewHttpClient("GET", apiURL, map[string]string{}, headers)
	resp, err := client.Request()
	if err != nil {
		return nil, err
	}

	var result DouyinWebVideoProfileResp
	if err := resp.ToJSON(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ExtraVideoId extracts the video ID from text or URL.
func (c *DouyinWebClient) ExtraVideoId(content string) (string, error) {
	if content == "" {
		return "", fmt.Errorf("empty content")
	}

	shortLinkRegex := regexp.MustCompile(`https://v\.douyin\.com/([a-zA-Z0-9_]{1,})/`)
	fullLinkRegex := regexp.MustCompile(`https://www\.douyin\.com/video/([0-9]{1,})`)

	if content[:4] != "http" {
		m := shortLinkRegex.FindAllString(content, -1)
		if len(m) != 0 {
			return c.ShortLinkToFullURL(m[0])
		}
		return "", fmt.Errorf("not a valid URL")
	}

	matched := fullLinkRegex.FindStringSubmatch(content)
	if len(matched) != 0 {
		return matched[1], nil
	}

	matched2 := shortLinkRegex.FindStringSubmatch(content)
	if len(matched2) != 0 {
		return matched2[1], nil
	}

	return "", fmt.Errorf("failed to extract video id from URL")
}

// ShortLinkToFullURL converts a short link to a video ID.
func (c *DouyinWebClient) ShortLinkToFullURL(shortLink string) (string, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Head(shortLink)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		rawURL := resp.Header.Get("Location")
		parsedURL, err := url.Parse(rawURL)
		if err != nil {
			return "", err
		}
		path := parsedURL.Path
		re := regexp.MustCompile(`/(\d+)/?$`)
		matches := re.FindStringSubmatch(path)
		if len(matches) > 1 {
			return matches[1], nil
		}
		return "", fmt.Errorf("video id not found in redirect URL")
	}
	return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}
