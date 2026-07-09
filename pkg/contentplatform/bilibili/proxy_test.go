package bilibili

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

func TestNewImageProxyRequestSetsBilibiliHeaders(t *testing.T) {
	inbound := httptest.NewRequest(http.MethodGet, "/bilibili/proxy", nil)
	inbound.Header.Set("If-None-Match", `"face-etag"`)

	req, err := newImageProxyRequest(
		context.Background(),
		"http://i0.hdslb.com/bfs/face/6f7ef32a25419744130c42f63ff6b11c624f19e3.jpg",
		inbound,
	)
	if err != nil {
		t.Fatalf("newImageProxyRequest: %v", err)
	}
	if req.URL.Scheme != "https" {
		t.Fatalf("scheme = %q", req.URL.Scheme)
	}
	if req.Header.Get("Referer") != defaultWebBaseURL+"/" {
		t.Fatalf("Referer = %q", req.Header.Get("Referer"))
	}
	if req.Header.Get("User-Agent") != defaultUserAgent {
		t.Fatalf("User-Agent = %q", req.Header.Get("User-Agent"))
	}
	if req.Header.Get("Sec-Fetch-Dest") != "image" {
		t.Fatalf("Sec-Fetch-Dest = %q", req.Header.Get("Sec-Fetch-Dest"))
	}
	if !strings.Contains(req.Header.Get("Accept"), "image/webp") {
		t.Fatalf("Accept = %q", req.Header.Get("Accept"))
	}
	if req.Header.Get("If-None-Match") != `"face-etag"` {
		t.Fatalf("If-None-Match = %q", req.Header.Get("If-None-Match"))
	}
}

func TestNewImageProxyRequestRejectsNonBilibiliImageHosts(t *testing.T) {
	for _, rawURL := range []string{
		"https://example.com/image.jpg",
		"https://not-hdslb.com/image.jpg",
		"file:///tmp/image.jpg",
	} {
		if _, err := newImageProxyRequest(context.Background(), rawURL, nil); err == nil {
			t.Fatalf("newImageProxyRequest(%q) returned nil error", rawURL)
		}
	}
}

func TestHandleImageProxy(t *testing.T) {
	oldClient := bilibiliImageProxyHTTPClient
	defer func() {
		bilibiliImageProxyHTTPClient = oldClient
	}()

	var proxiedRequest *http.Request
	bilibiliImageProxyHTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			proxiedRequest = req
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": {"image/jpeg"},
					"ETag":         {`"bili-img-etag"`},
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
	targetURL := "http://i0.hdslb.com/bfs/face/6f7ef32a25419744130c42f63ff6b11c624f19e3.jpg"
	ctx.Request = httptest.NewRequest(http.MethodGet, "/bilibili/proxy?url="+url.QueryEscape(targetURL), nil)
	ctx.Request.Header.Set("If-None-Match", `"browser-etag"`)

	HandleImageProxy(ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != "image-body" {
		t.Fatalf("body = %q", w.Body.String())
	}
	if w.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("Content-Type = %q", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("ETag") != `"bili-img-etag"` {
		t.Fatalf("ETag = %q", w.Header().Get("ETag"))
	}
	if proxiedRequest == nil {
		t.Fatal("upstream request was not sent")
	}
	if proxiedRequest.URL.String() != strings.Replace(targetURL, "http://", "https://", 1) {
		t.Fatalf("upstream URL = %q", proxiedRequest.URL.String())
	}
	if proxiedRequest.Header.Get("Referer") != defaultWebBaseURL+"/" {
		t.Fatalf("Referer = %q", proxiedRequest.Header.Get("Referer"))
	}
	if proxiedRequest.Header.Get("If-None-Match") != `"browser-etag"` {
		t.Fatalf("If-None-Match = %q", proxiedRequest.Header.Get("If-None-Match"))
	}
}
