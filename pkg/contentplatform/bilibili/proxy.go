package bilibili

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

var bilibiliImageProxyHTTPClient = &http.Client{Timeout: 30 * time.Second}

func HandleImageProxy(ctx *gin.Context) {
	targetURL := ctx.Query("url")
	if strings.TrimSpace(targetURL) == "" {
		ctx.String(http.StatusBadRequest, "missing url")
		return
	}

	req, err := newImageProxyRequest(ctx.Request.Context(), targetURL, ctx.Request)
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}

	resp, err := bilibiliImageProxyHTTPClient.Do(req)
	if err != nil {
		ctx.String(http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()

	copyImageProxyHeaders(ctx, resp.Header)
	ctx.Status(resp.StatusCode)
	_, _ = io.Copy(ctx.Writer, resp.Body)
}

func newImageProxyRequest(ctx context.Context, targetURL string, inbound *http.Request) (*http.Request, error) {
	targetURL = strings.ReplaceAll(strings.TrimSpace(targetURL), "&amp;", "&")
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported bilibili image URL scheme: %s", parsed.Scheme)
	}
	if !isAllowedImageProxyHost(parsed.Hostname()) {
		return nil, fmt.Errorf("unsupported bilibili image host: %s", parsed.Hostname())
	}
	if parsed.Scheme == "http" {
		parsed.Scheme = "https"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	setImageProxyHeaders(req)
	copyConditionalImageHeaders(req, inbound)
	return req, nil
}

func setImageProxyHeaders(req *http.Request) {
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", defaultWebBaseURL+"/")
	req.Header.Set("Sec-Fetch-Dest", "image")
	req.Header.Set("Sec-Fetch-Mode", "no-cors")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("User-Agent", defaultUserAgent)
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
	return host == "hdslb.com" ||
		strings.HasSuffix(host, ".hdslb.com") ||
		host == "biliimg.com" ||
		strings.HasSuffix(host, ".biliimg.com")
}
