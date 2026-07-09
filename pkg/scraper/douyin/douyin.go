package douyin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	SourceName             = "抖音"
	BaseURL                = "https://www.douyin.com"
	ContentTypeUserProfile = "account"
	DefaultWebUserAgent    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
	defaultAPIUserAgent    = DefaultWebUserAgent
	defaultAwemePostPath   = "/aweme/v1/web/aweme/post/"
)

var (
	errUnsupportedURL = errors.New("不支持的抖音 URL")
	shareURLRE        = regexp.MustCompile(`https?://[^\s"'<>]+`)
	routerDataRE      = regexp.MustCompile(`(?s)window\._ROUTER_DATA\s*=\s*(.*?)</script>`)
	unsafeFilenameRE  = regexp.MustCompile(`[\\/:*?"<>|#\n\r]`)
	dotsRE            = regexp.MustCompile(`\.{2,}`)
)

type VideoInfo struct {
	URL             string
	Title           string
	VideoID         string
	CoverURL        string
	UserAgent       string
	Source          string
	AuthorID        string
	AuthorSecID     string
	AuthorUsername  string
	AuthorNickname  string
	AuthorAvatarURL string
}

type Progress struct {
	DownloadedBytes int64
	TotalBytes      int64
	Percent         float64
	SpeedBytes      int64
	ETASeconds      int64
}

type ProgressFunc func(progress Progress)

type Client struct {
	HTTPClient       *http.Client
	NoRedirectClient *http.Client
	DownloadClient   *http.Client
	UserAgent        func() string
	BaseURL          string
	Cookie           string
}

func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		NoRedirectClient: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		DownloadClient: &http.Client{
			Timeout: 0,
		},
		UserAgent: RandomUserAgent,
		BaseURL:   BaseURL,
	}
}

func CanParse(rawURL string) bool {
	return strings.Contains(rawURL, "douyin.com") || strings.Contains(rawURL, "iesdouyin.com")
}

func ExtractShareURL(text string) string {
	for _, match := range shareURLRE.FindAllString(strings.TrimSpace(text), -1) {
		match = strings.Trim(match, " \t\r\n，。；;、.,!?！？")
		if CanParse(match) {
			return match
		}
	}
	if CanParse(text) {
		return strings.TrimSpace(text)
	}
	return ""
}

func Parse(ctx context.Context, rawURL string) (*VideoInfo, error) {
	return NewClient().Parse(ctx, rawURL)
}

func FetchFileSize(ctx context.Context, videoURL, userAgent string) (int64, error) {
	return NewClient().FetchFileSize(ctx, videoURL, userAgent)
}

func Download(ctx context.Context, info *VideoInfo, destPath string, onProgress ProgressFunc) error {
	return NewClient().Download(ctx, info, destPath, onProgress)
}

func (c *Client) Parse(ctx context.Context, rawURL string) (*VideoInfo, error) {
	rawURL = ExtractShareURL(rawURL)
	if !CanParse(rawURL) {
		return nil, errUnsupportedURL
	}

	ua := c.userAgent()
	finalURL, err := c.resolveRedirects(ctx, rawURL, ua, 5)
	if err != nil {
		return nil, fmt.Errorf("解析重定向失败: %w", err)
	}

	videoID := parseVideoID(finalURL)
	if videoID == "" {
		return nil, fmt.Errorf("无法解析视频 ID: %s", finalURL)
	}

	pageURL := "https://www.iesdouyin.com/share/video/" + videoID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req, ua)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求页面失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("请求页面失败: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	data, err := extractRouterData(string(body))
	if err != nil {
		return nil, err
	}

	item, err := extractItem(data)
	if err != nil {
		return nil, err
	}

	videoURL, err := extractString(item, "video", "play_addr", "url_list", "0")
	if err != nil {
		return nil, fmt.Errorf("未找到视频 URL: %w", err)
	}
	videoURL = strings.Replace(videoURL, "playwm", "play", 1)

	coverURL, _ := extractString(item, "video", "cover", "url_list", "0")
	desc, _ := extractString(item, "desc")
	if strings.TrimSpace(desc) == "" {
		desc = "douyin_" + videoID
	}
	authorID, _ := extractString(item, "author", "uid")
	authorSecID, _ := extractString(item, "author", "sec_uid")
	authorUsername, _ := extractString(item, "author", "unique_id")
	if authorUsername == "" {
		authorUsername, _ = extractString(item, "author", "short_id")
	}
	authorNickname, _ := extractString(item, "author", "nickname")
	authorAvatarURL, _ := extractString(item, "author", "avatar_thumb", "url_list", "0")

	return &VideoInfo{
		URL:             videoURL,
		Title:           SanitizeFilename(desc),
		VideoID:         videoID,
		CoverURL:        coverURL,
		UserAgent:       ua,
		Source:          SourceName,
		AuthorID:        authorID,
		AuthorSecID:     authorSecID,
		AuthorUsername:  authorUsername,
		AuthorNickname:  authorNickname,
		AuthorAvatarURL: authorAvatarURL,
	}, nil
}

func (c *Client) FetchFileSize(ctx context.Context, videoURL, userAgent string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, videoURL, nil)
	if err != nil {
		return -1, err
	}
	setCommonHeaders(req, fallbackUserAgent(userAgent))

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return -1, fmt.Errorf("获取文件大小失败: HTTP %d", resp.StatusCode)
	}

	length := resp.Header.Get("Content-Length")
	if length == "" {
		return -1, nil
	}
	size, err := strconv.ParseInt(length, 10, 64)
	if err != nil {
		return -1, err
	}
	return size, nil
}

func (c *Client) Download(ctx context.Context, info *VideoInfo, destPath string, onProgress ProgressFunc) error {
	if info == nil {
		return errors.New("视频信息为空")
	}
	if strings.TrimSpace(info.URL) == "" {
		return errors.New("视频下载 URL 为空")
	}
	if strings.TrimSpace(destPath) == "" {
		return errors.New("目标文件路径为空")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.URL, nil)
	if err != nil {
		return err
	}
	setCommonHeaders(req, fallbackUserAgent(info.UserAgent))

	resp, err := c.downloadClient().Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	total := resp.ContentLength
	if total <= 0 {
		total, _ = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	tmpPath := destPath + ".download"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}

	cleanup := true
	defer func() {
		file.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := copyWithProgress(ctx, file, resp.Body, total, onProgress); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("关闭文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}
	cleanup = false
	return nil
}

func (c *Client) resolveRedirects(ctx context.Context, rawURL, userAgent string, maxHops int) (string, error) {
	current := rawURL
	client := c.noRedirectClient()

	for i := 0; i < maxHops; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, current, nil)
		if err != nil {
			return "", err
		}
		setCommonHeaders(req, userAgent)

		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		location := resp.Header.Get("Location")
		resp.Body.Close()
		if location == "" {
			return current, nil
		}

		next, err := resolveLocation(current, location)
		if err != nil {
			return "", err
		}
		current = next
	}
	return current, nil
}

func extractRouterData(pageHTML string) (map[string]interface{}, error) {
	matches := routerDataRE.FindStringSubmatch(pageHTML)
	if len(matches) < 2 {
		return nil, errors.New("未找到 _ROUTER_DATA")
	}

	raw := strings.TrimSpace(html.UnescapeString(matches[1]))
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("解析 _ROUTER_DATA 失败: %w", err)
	}
	return data, nil
}

func extractItem(data map[string]interface{}) (map[string]interface{}, error) {
	loaderData, ok := data["loaderData"].(map[string]interface{})
	if !ok {
		return nil, errors.New("未找到 loaderData")
	}

	for _, key := range []string{"video_(id)/page", "note_(id)/page"} {
		if item, ok := itemFromLoaderEntry(loaderData[key]); ok {
			return item, nil
		}
	}
	for _, entry := range loaderData {
		if item, ok := itemFromLoaderEntry(entry); ok {
			return item, nil
		}
	}
	return nil, errors.New("未找到视频信息")
}

func itemFromLoaderEntry(entry interface{}) (map[string]interface{}, bool) {
	entryMap, ok := entry.(map[string]interface{})
	if !ok {
		return nil, false
	}
	videoInfoRes, ok := entryMap["videoInfoRes"].(map[string]interface{})
	if !ok {
		return nil, false
	}
	itemList, ok := videoInfoRes["item_list"].([]interface{})
	if !ok || len(itemList) == 0 {
		return nil, false
	}
	item, ok := itemList[0].(map[string]interface{})
	return item, ok
}

func extractString(root interface{}, path ...string) (string, error) {
	var current interface{} = root
	for _, part := range path {
		switch node := current.(type) {
		case map[string]interface{}:
			v, ok := node[part]
			if !ok {
				return "", fmt.Errorf("缺少字段 %s", part)
			}
			current = v
		case []interface{}:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(node) {
				return "", fmt.Errorf("数组索引无效 %s", part)
			}
			current = node[idx]
		default:
			return "", fmt.Errorf("字段 %s 类型不匹配", part)
		}
	}

	s, ok := current.(string)
	if !ok {
		return "", errors.New("目标字段不是字符串")
	}
	return s, nil
}

func copyWithProgress(ctx context.Context, dst io.Writer, src io.Reader, total int64, onProgress ProgressFunc) error {
	buf := make([]byte, 32*1024)
	startedAt := time.Now()
	lastAt := startedAt
	lastBytes := int64(0)
	downloaded := int64(0)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := src.Read(buf)
		if n > 0 {
			written, writeErr := dst.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("写入文件失败: %w", writeErr)
			}
			if written != n {
				return io.ErrShortWrite
			}
			downloaded += int64(n)
			now := time.Now()
			if onProgress != nil && (now.Sub(lastAt) >= 200*time.Millisecond || readErr == io.EOF) {
				elapsed := now.Sub(lastAt).Seconds()
				speed := int64(0)
				if elapsed > 0 {
					speed = int64(float64(downloaded-lastBytes) / elapsed)
				}
				onProgress(makeProgress(downloaded, total, speed))
				lastAt = now
				lastBytes = downloaded
			}
		}
		if readErr == io.EOF {
			if onProgress != nil {
				onProgress(makeProgress(downloaded, total, 0))
			}
			return nil
		}
		if readErr != nil {
			return fmt.Errorf("读取文件流失败: %w", readErr)
		}
	}
}

func makeProgress(downloaded, total, speed int64) Progress {
	progress := Progress{
		DownloadedBytes: downloaded,
		TotalBytes:      total,
		SpeedBytes:      speed,
		ETASeconds:      -1,
	}
	if total > 0 {
		progress.Percent = float64(downloaded) / float64(total) * 100
		if speed > 0 && downloaded < total {
			progress.ETASeconds = (total - downloaded) / speed
		}
	}
	return progress
}

func parseVideoID(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, part := range parts {
		if part == "video" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return ""
}

func resolveLocation(baseURL, location string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	next, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(next).String(), nil
}

func setCommonHeaders(req *http.Request, userAgent string) {
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://www.douyin.com/")
}

func (c *Client) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return NewClient().HTTPClient
}

func (c *Client) noRedirectClient() *http.Client {
	if c != nil && c.NoRedirectClient != nil {
		return c.NoRedirectClient
	}
	return NewClient().NoRedirectClient
}

func (c *Client) downloadClient() *http.Client {
	if c != nil && c.DownloadClient != nil {
		return c.DownloadClient
	}
	return NewClient().DownloadClient
}

func (c *Client) userAgent() string {
	if c != nil && c.UserAgent != nil {
		return c.UserAgent()
	}
	return RandomUserAgent()
}

func fallbackUserAgent(userAgent string) string {
	if strings.TrimSpace(userAgent) != "" {
		return userAgent
	}
	return RandomUserAgent()
}

func RandomUserAgent() string {
	osVersions := []string{"15_0", "15_4", "16_0", "16_3", "16_6", "17_0", "17_1", "17_2", "17_3", "17_4", "17_5", "18_0"}
	safariVersions := []string{"604.1", "605.1.15"}
	chromeVersions := []string{"120.0.6099.119", "121.0.6167.178", "122.0.6261.89", "122.0.6261.105", "123.0.6312.58", "124.0.6367.54"}
	edgeVersions := []string{"121.0.2277.107", "122.0.2365.56", "122.0.2365.92", "123.0.2420.65"}
	firefoxVersions := []string{"121.0", "122.0", "123.0", "124.0"}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	osVersion := osVersions[r.Intn(len(osVersions))]
	osPart := "iPhone; CPU iPhone OS " + osVersion + " like Mac OS X"
	webkit := "AppleWebKit/605.1.15 (KHTML, like Gecko)"

	switch r.Intn(4) {
	case 0:
		version := strings.ReplaceAll(osVersions[r.Intn(len(osVersions))], "_", ".")
		return fmt.Sprintf("Mozilla/5.0 (%s) %s Version/%s Mobile/15E148 Safari/%s", osPart, webkit, version, safariVersions[r.Intn(len(safariVersions))])
	case 1:
		return fmt.Sprintf("Mozilla/5.0 (%s) %s CriOS/%s Mobile/15E148 Safari/%s", osPart, webkit, chromeVersions[r.Intn(len(chromeVersions))], safariVersions[r.Intn(len(safariVersions))])
	case 2:
		return fmt.Sprintf("Mozilla/5.0 (%s) %s EdgiOS/%s Version/17.0 Mobile/15E148 Safari/%s", osPart, webkit, edgeVersions[r.Intn(len(edgeVersions))], safariVersions[r.Intn(len(safariVersions))])
	default:
		return fmt.Sprintf("Mozilla/5.0 (%s) %s FxiOS/%s Mobile/15E148 Safari/605.1.15", osPart, webkit, firefoxVersions[r.Intn(len(firefoxVersions))])
	}
}

func SanitizeFilename(name string) string {
	name = unsafeFilenameRE.ReplaceAllString(name, "_")
	name = dotsRE.ReplaceAllString(name, ".")
	name = strings.Trim(name, " .")
	if len([]rune(name)) > 80 {
		name = string([]rune(name)[:80])
	}
	if name == "" {
		return "douyin"
	}
	return name
}
