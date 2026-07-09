package qidian

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

func TestNewImageProxyRequestSetsQidianAvatarHeaders(t *testing.T) {
	inbound := httptest.NewRequest(http.MethodGet, "/qidian/proxy", nil)
	inbound.Header.Set("If-None-Match", `"avatar-etag"`)
	inbound.Header.Set("If-Modified-Since", "Fri, 19 Jun 2026 00:00:00 GMT")
	inbound.Header.Set("Range", "bytes=0-99")

	req, err := newImageProxyRequest(
		context.Background(),
		"http://ccportrait.yuewen.com/apimg/349573/p_16465333704674701/100",
		inbound,
	)
	if err != nil {
		t.Fatalf("newImageProxyRequest: %v", err)
	}
	if req.URL.Scheme != "https" {
		t.Fatalf("scheme = %q", req.URL.Scheme)
	}
	if req.Header.Get("Referer") != BaseURL+"/" {
		t.Fatalf("Referer = %q", req.Header.Get("Referer"))
	}
	if req.Header.Get("Sec-Fetch-Storage-Access") != "none" {
		t.Fatalf("Sec-Fetch-Storage-Access = %q", req.Header.Get("Sec-Fetch-Storage-Access"))
	}
	if req.Header.Get("Priority") != "u=2, i" {
		t.Fatalf("Priority = %q", req.Header.Get("Priority"))
	}
	if !strings.Contains(req.Header.Get("Accept"), "image/webp") {
		t.Fatalf("Accept = %q", req.Header.Get("Accept"))
	}
	if req.Header.Get("If-None-Match") != "" {
		t.Fatalf("If-None-Match = %q", req.Header.Get("If-None-Match"))
	}
	if req.Header.Get("If-Modified-Since") != "" {
		t.Fatalf("If-Modified-Since = %q", req.Header.Get("If-Modified-Since"))
	}
	if req.Header.Get("Range") != "bytes=0-99" {
		t.Fatalf("Range = %q", req.Header.Get("Range"))
	}
}

func TestNewImageProxyRequestRejectsNonQidianAvatarHosts(t *testing.T) {
	for _, rawURL := range []string{
		"https://example.com/avatar.jpg",
		"https://not-yuewen.com/apimg/349573/p_16465333704674701/100",
		"file:///tmp/avatar.jpg",
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
					"Content-Type": {"image/jpeg"},
					"ETag":         {`"qidian-avatar-etag"`},
				},
				Body:          io.NopCloser(strings.NewReader("avatar-body")),
				ContentLength: int64(len("avatar-body")),
				Request:       req,
			}, nil
		}),
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	targetURL := "http://ccportrait.yuewen.com/apimg/349573/p_16465333704674701/100"
	ctx.Request = httptest.NewRequest(http.MethodGet, "/qidian/proxy?url="+url.QueryEscape(targetURL), nil)
	ctx.Request.Header.Set("If-None-Match", `"browser-etag"`)
	ctx.Request.Header.Set("If-Modified-Since", "Fri, 19 Jun 2026 00:00:00 GMT")
	ctx.Request.Header.Set("Range", "bytes=0-99")

	HandleImageProxy(ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != "avatar-body" {
		t.Fatalf("body = %q", w.Body.String())
	}
	if w.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("Content-Type = %q", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("ETag") != `"qidian-avatar-etag"` {
		t.Fatalf("ETag = %q", w.Header().Get("ETag"))
	}
	if proxiedRequest == nil {
		t.Fatal("upstream request was not sent")
	}
	if proxiedRequest.URL.String() != strings.Replace(targetURL, "http://", "https://", 1) {
		t.Fatalf("upstream URL = %q", proxiedRequest.URL.String())
	}
	if proxiedRequest.Header.Get("Referer") != BaseURL+"/" {
		t.Fatalf("Referer = %q", proxiedRequest.Header.Get("Referer"))
	}
	if proxiedRequest.Header.Get("If-None-Match") != "" {
		t.Fatalf("If-None-Match = %q", proxiedRequest.Header.Get("If-None-Match"))
	}
	if proxiedRequest.Header.Get("If-Modified-Since") != "" {
		t.Fatalf("If-Modified-Since = %q", proxiedRequest.Header.Get("If-Modified-Since"))
	}
	if proxiedRequest.Header.Get("Range") != "bytes=0-99" {
		t.Fatalf("Range = %q", proxiedRequest.Header.Get("Range"))
	}
}
