package instagram

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

func TestNewImageProxyRequestSetsInstagramHeaders(t *testing.T) {
	inbound := httptest.NewRequest(http.MethodGet, "/instagram/proxy", nil)
	inbound.Header.Set("If-None-Match", `"instagram-etag"`)
	inbound.Header.Set("Range", "bytes=0-1023")

	req, err := newImageProxyRequest(
		context.Background(),
		"http://instagram.fsin11-1.fna.fbcdn.net/v/t51.82787-15/image.jpg?oh=test",
		inbound,
	)
	if err != nil {
		t.Fatalf("newImageProxyRequest: %v", err)
	}
	if req.URL.Scheme != "https" {
		t.Fatalf("scheme = %q", req.URL.Scheme)
	}
	if req.Header.Get("Origin") != BaseURL {
		t.Fatalf("Origin = %q", req.Header.Get("Origin"))
	}
	if req.Header.Get("Referer") != defaultImageProxyReferer {
		t.Fatalf("Referer = %q", req.Header.Get("Referer"))
	}
	if req.Header.Get("User-Agent") != defaultUserAgent {
		t.Fatalf("User-Agent = %q", req.Header.Get("User-Agent"))
	}
	if req.Header.Get("Sec-Fetch-Dest") != "image" {
		t.Fatalf("Sec-Fetch-Dest = %q", req.Header.Get("Sec-Fetch-Dest"))
	}
	if req.Header.Get("Sec-Fetch-Mode") != "cors" {
		t.Fatalf("Sec-Fetch-Mode = %q", req.Header.Get("Sec-Fetch-Mode"))
	}
	if !strings.Contains(req.Header.Get("Accept"), "image/webp") {
		t.Fatalf("Accept = %q", req.Header.Get("Accept"))
	}
	if req.Header.Get("If-None-Match") != `"instagram-etag"` {
		t.Fatalf("If-None-Match = %q", req.Header.Get("If-None-Match"))
	}
	if req.Header.Get("Range") != "bytes=0-1023" {
		t.Fatalf("Range = %q", req.Header.Get("Range"))
	}
}

func TestNewImageProxyRequestRejectsNonInstagramImageHosts(t *testing.T) {
	for _, rawURL := range []string{
		"https://example.com/image.jpg",
		"https://notfbcdn.net.evil.test/image.jpg",
		"https://not-cdninstagram.com/image.jpg",
		"file:///tmp/image.jpg",
	} {
		if _, err := newImageProxyRequest(context.Background(), rawURL, nil); err == nil {
			t.Fatalf("newImageProxyRequest(%q) returned nil error", rawURL)
		}
	}
}

func TestHandleImageProxy(t *testing.T) {
	oldClient := instagramImageProxyHTTPClient
	defer func() {
		instagramImageProxyHTTPClient = oldClient
	}()

	var proxiedRequest *http.Request
	instagramImageProxyHTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			proxiedRequest = req
			return &http.Response{
				StatusCode: http.StatusPartialContent,
				Header: http.Header{
					"Content-Type":  {"image/jpeg"},
					"Content-Range": {"bytes 0-9/10"},
					"ETag":          {`"instagram-img-etag"`},
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
	targetURL := "http://scontent.cdninstagram.com/v/t51.2885-15/image.jpg?oh=test"
	ctx.Request = httptest.NewRequest(http.MethodGet, "/instagram/proxy?url="+url.QueryEscape(targetURL), nil)
	ctx.Request.Header.Set("Range", "bytes=0-9")

	HandleImageProxy(ctx)

	if w.Code != http.StatusPartialContent {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != "image-body" {
		t.Fatalf("body = %q", w.Body.String())
	}
	if w.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("Content-Type = %q", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Content-Range") != "bytes 0-9/10" {
		t.Fatalf("Content-Range = %q", w.Header().Get("Content-Range"))
	}
	if w.Header().Get("ETag") != `"instagram-img-etag"` {
		t.Fatalf("ETag = %q", w.Header().Get("ETag"))
	}
	if proxiedRequest == nil {
		t.Fatal("upstream request was not sent")
	}
	if proxiedRequest.URL.String() != strings.Replace(targetURL, "http://", "https://", 1) {
		t.Fatalf("upstream URL = %q", proxiedRequest.URL.String())
	}
	if proxiedRequest.Header.Get("Origin") != BaseURL {
		t.Fatalf("Origin = %q", proxiedRequest.Header.Get("Origin"))
	}
	if proxiedRequest.Header.Get("Referer") != defaultImageProxyReferer {
		t.Fatalf("Referer = %q", proxiedRequest.Header.Get("Referer"))
	}
	if proxiedRequest.Header.Get("Range") != "bytes=0-9" {
		t.Fatalf("Range = %q", proxiedRequest.Header.Get("Range"))
	}
}
