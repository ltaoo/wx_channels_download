package douban

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultImageProxyReferer = "https://movie.douban.com/"
	imageProxyUserAgent      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
)

var doubanImageProxyHTTPClient = &http.Client{Timeout: 30 * time.Second}

func HandleImageProxy(ctx *gin.Context) {
	targetURL := ctx.Query("url")
	if strings.TrimSpace(targetURL) == "" {
		ctx.String(http.StatusBadRequest, "missing url")
		return
	}

	req, err := newImageProxyRequest(ctx.Request.Context(), targetURL, ctx.Query("referer"), ctx.Request)
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}

	resp, err := doubanImageProxyHTTPClient.Do(req)
	if err != nil {
		ctx.String(http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()

	copyImageProxyHeaders(ctx, resp.Header)
	ctx.Status(resp.StatusCode)
	_, _ = io.Copy(ctx.Writer, resp.Body)
}

func newImageProxyRequest(ctx context.Context, targetURL string, referer string, inbound *http.Request) (*http.Request, error) {
	targetURL = strings.ReplaceAll(strings.TrimSpace(targetURL), "&amp;", "&")
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported douban image URL scheme: %s", parsed.Scheme)
	}
	if !isAllowedImageProxyHost(parsed.Hostname()) {
		return nil, fmt.Errorf("unsupported douban image host: %s", parsed.Hostname())
	}
	if parsed.Scheme == "http" {
		parsed.Scheme = "https"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	setImageProxyHeaders(req, normalizeImageProxyReferer(referer))
	copyConditionalImageHeaders(req, inbound)
	return req, nil
}

func setImageProxyHeaders(req *http.Request, referer string) {
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Priority", "u=2, i")
	req.Header.Set("Referer", referer)
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "image")
	req.Header.Set("Sec-Fetch-Mode", "no-cors")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Sec-Fetch-Storage-Access", "active")
	req.Header.Set("User-Agent", imageProxyUserAgent)
}

func copyConditionalImageHeaders(req *http.Request, inbound *http.Request) {
	if inbound == nil {
		return
	}
	for _, key := range []string{"If-Modified-Since", "If-None-Match", "Range"} {
		if value := inbound.Header.Get(key); value != "" {
			req.Header.Set(key, value)
		}
	}
}

func copyImageProxyHeaders(ctx *gin.Context, headers http.Header) {
	for key, values := range headers {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			ctx.Header(key, value)
		}
	}
	if ctx.Writer.Header().Get("Cache-Control") == "" {
		ctx.Header("Cache-Control", "public, max-age=86400")
	}
}

func normalizeImageProxyReferer(raw string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "&amp;", "&"))
	if raw == "" {
		return defaultImageProxyReferer
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return defaultImageProxyReferer
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return defaultImageProxyReferer
	}
	if !isAllowedRefererHost(parsed.Hostname()) {
		return defaultImageProxyReferer
	}
	if parsed.Scheme == "http" {
		parsed.Scheme = "https"
	}
	return parsed.String()
}

func isHopByHopHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
		return true
	default:
		return false
	}
}

func isAllowedImageProxyHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "doubanio.com" || strings.HasSuffix(host, ".doubanio.com")
}

func isAllowedRefererHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "douban.com" || strings.HasSuffix(host, ".douban.com")
}
