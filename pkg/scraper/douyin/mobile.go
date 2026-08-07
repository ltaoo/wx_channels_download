package douyin

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// DouyinMobileClient is the Douyin mobile scraper.
type DouyinMobileClient struct{}

// NewDouyinMobileClient creates a new Douyin mobile scraper.
func NewDouyinMobileClient() *DouyinMobileClient {
	return &DouyinMobileClient{}
}

// Parse resolves a Douyin share link and extracts video info.
// Supports v.douyin.com short links and iesdouyin.com links.
func (c *DouyinMobileClient) Parse(url string) (*VideoInfo, error) {
	if !canParse(url) {
		return nil, fmt.Errorf("不支持的URL: %s", url)
	}

	ua := userAgents[len(userAgents)-1]

	finalURL, err := resolveRedirects(url, ua)
	if err != nil {
		return nil, fmt.Errorf("解析重定向失败: %v", err)
	}

	videoID := parseVideoID(finalURL)
	iesdURL := fmt.Sprintf("https://www.iesdouyin.com/share/video/%s", videoID)

	req, _ := http.NewRequest("GET", iesdURL, nil)
	req.Header.Set("User-Agent", ua)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求页面失败: %v", err)
	}
	defer resp.Body.Close()

	html, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	routerPattern := regexp.MustCompile(`(?s)window\._ROUTER_DATA\s*=\s*(.*?)</script>`)
	matches := routerPattern.FindStringSubmatch(string(html))
	if len(matches) < 2 {
		return nil, fmt.Errorf("未找到_ROUTER_DATA")
	}
	jsonStr := strings.TrimSpace(matches[1])
	jsonStr = strings.ReplaceAll(jsonStr, "\\u002F", "/")

	urlPattern := regexp.MustCompile(`play_addr.*?url_list.*?"(https://[^"]+)"`)
	urlMatch := urlPattern.FindStringSubmatch(jsonStr)
	if len(urlMatch) < 2 {
		return nil, fmt.Errorf("未找到视频URL")
	}
	videoURL := strings.Replace(urlMatch[1], "playwm", "play", 1)

	coverPattern := regexp.MustCompile(`cover.*?url_list.*?"(https?://[^"]+)"`)
	coverMatch := coverPattern.FindStringSubmatch(jsonStr)
	var coverURL string
	if len(coverMatch) > 1 {
		coverURL = coverMatch[1]
	}

	desc := extractByRegex(jsonStr, `"desc"\s*:\s*"([^"]*)"`)
	if desc == "" {
		desc = fmt.Sprintf("douyin_%s", videoID)
	}

	safeTitle := sanitizeFilename(desc)

	return &VideoInfo{
		URL:      videoURL,
		Title:    safeTitle,
		VideoID:  videoID,
		CoverURL: coverURL,
		Source:   "抖音",
	}, nil
}
