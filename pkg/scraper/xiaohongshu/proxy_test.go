package xiaohongshu

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNewImageProxyRequestSetsXiaohongshuHeaders(t *testing.T) {
	inbound := httptest.NewRequest(http.MethodGet, "/xiaohongshu/proxy", nil)
	inbound.Header.Set("If-Modified-Since", "Tue, 24 Feb 2026 03:17:52 GMT")
	inbound.Header.Set("If-None-Match", `"e9fb23a9559a5575928fc150482bf6a4"`)

	req, err := newImageProxyRequest(
		context.Background(),
		"https://sns-webpic-qc.xhscdn.com/202606171314/b7d4a51f7626a1e35ef7c73afc636aea/1000g00828hl4n50fm0004a00ht18ies9r78aokg!nc_n_webp_mw_1",
		inbound,
	)
	if err != nil {
		t.Fatalf("newImageProxyRequest: %v", err)
	}
	if req.Header.Get("Referer") != SourceURL {
		t.Fatalf("Referer = %q", req.Header.Get("Referer"))
	}
	if req.Header.Get("User-Agent") != DefaultUserAgent() {
		t.Fatalf("User-Agent = %q", req.Header.Get("User-Agent"))
	}
	if req.Header.Get("Sec-Fetch-Dest") != "image" {
		t.Fatalf("Sec-Fetch-Dest = %q", req.Header.Get("Sec-Fetch-Dest"))
	}
	if !strings.Contains(req.Header.Get("Accept"), "image/webp") {
		t.Fatalf("Accept = %q", req.Header.Get("Accept"))
	}
	if req.Header.Get("If-None-Match") != `"e9fb23a9559a5575928fc150482bf6a4"` {
		t.Fatalf("If-None-Match = %q", req.Header.Get("If-None-Match"))
	}
}

func TestNewImageProxyRequestRejectsNonXiaohongshuCDNHosts(t *testing.T) {
	for _, rawURL := range []string{
		"https://example.com/image.webp",
		"https://notxhscdn.com/image.webp",
		"file:///tmp/image.webp",
	} {
		if _, err := newImageProxyRequest(context.Background(), rawURL, nil); err == nil {
			t.Fatalf("newImageProxyRequest(%q) returned nil error", rawURL)
		}
	}
}

func TestHandleImageProxy(t *testing.T) {
	oldClient := imageProxyHTTPClient
	defer func() {
		imageProxyHTTPClient = oldClient
	}()

	var proxiedRequest *http.Request
	imageProxyHTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			proxiedRequest = req
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": {"image/webp"},
					"ETag":         {`"img-etag"`},
				},
				Body:          io.NopCloser(strings.NewReader("image-body")),
				ContentLength: int64(len("image-body")),
				Request:       req,
			}, nil
		}),
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	targetURL := "https://sns-webpic-qc.xhscdn.com/202606171314/b7d4a51f7626a1e35ef7c73afc636aea/1000g00828hl4n50fm0004a00ht18ies9r78aokg!nc_n_webp_mw_1"
	ctx.Request = httptest.NewRequest(http.MethodGet, "/xiaohongshu/proxy?url="+url.QueryEscape(targetURL), nil)
	ctx.Request.Header.Set("If-None-Match", `"browser-etag"`)

	HandleImageProxy(ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != "image-body" {
		t.Fatalf("body = %q", w.Body.String())
	}
	if w.Header().Get("Content-Type") != "image/webp" {
		t.Fatalf("Content-Type = %q", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("ETag") != `"img-etag"` {
		t.Fatalf("ETag = %q", w.Header().Get("ETag"))
	}
	if proxiedRequest == nil {
		t.Fatal("upstream request was not sent")
	}
	if proxiedRequest.URL.String() != targetURL {
		t.Fatalf("upstream URL = %q", proxiedRequest.URL.String())
	}
	if proxiedRequest.Header.Get("Referer") != SourceURL {
		t.Fatalf("Referer = %q", proxiedRequest.Header.Get("Referer"))
	}
	if proxiedRequest.Header.Get("If-None-Match") != `"browser-etag"` {
		t.Fatalf("If-None-Match = %q", proxiedRequest.Header.Get("If-None-Match"))
	}
}
