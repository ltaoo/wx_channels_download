package weibo

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

func TestNewImageProxyRequestSetsWeiboHeaders(t *testing.T) {
	inbound := httptest.NewRequest(http.MethodGet, "/weibo/proxy", nil)
	inbound.Header.Set("If-None-Match", `"weibo-etag"`)
	inbound.Header.Set("Range", "bytes=0-1023")

	req, err := newImageProxyRequest(
		context.Background(),
		"http://wx2.sinaimg.cn/orj360/72d02bably1ie9t8fjukdj20nm0q9wmb.jpg",
		inbound,
	)
	if err != nil {
		t.Fatalf("newImageProxyRequest: %v", err)
	}
	if req.URL.Scheme != "https" {
		t.Fatalf("scheme = %q", req.URL.Scheme)
	}
	if req.Header.Get("Referer") != imageProxyReferer {
		t.Fatalf("Referer = %q", req.Header.Get("Referer"))
	}
	if req.Header.Get("User-Agent") != defaultUserAgent {
		t.Fatalf("User-Agent = %q", req.Header.Get("User-Agent"))
	}
	if req.Header.Get("Sec-Fetch-Dest") != "image" {
		t.Fatalf("Sec-Fetch-Dest = %q", req.Header.Get("Sec-Fetch-Dest"))
	}
	if req.Header.Get("Priority") != "u=1, i" {
		t.Fatalf("Priority = %q", req.Header.Get("Priority"))
	}
	if !strings.Contains(req.Header.Get("Accept"), "image/webp") {
		t.Fatalf("Accept = %q", req.Header.Get("Accept"))
	}
	if req.Header.Get("If-None-Match") != `"weibo-etag"` {
		t.Fatalf("If-None-Match = %q", req.Header.Get("If-None-Match"))
	}
	if req.Header.Get("Range") != "bytes=0-1023" {
		t.Fatalf("Range = %q", req.Header.Get("Range"))
	}
}

func TestNewImageProxyRequestRejectsNonWeiboImageHosts(t *testing.T) {
	for _, rawURL := range []string{
		"https://example.com/image.jpg",
		"https://notsinaimg.cn/image.jpg",
		"file:///tmp/image.jpg",
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
					"ETag":         {`"weibo-img-etag"`},
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
	targetURL := "http://wx2.sinaimg.cn/orj360/72d02bably1ie9t8fjukdj20nm0q9wmb.jpg"
	ctx.Request = httptest.NewRequest(http.MethodGet, "/weibo/proxy?url="+url.QueryEscape(targetURL), nil)
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
	if w.Header().Get("ETag") != `"weibo-img-etag"` {
		t.Fatalf("ETag = %q", w.Header().Get("ETag"))
	}
	if proxiedRequest == nil {
		t.Fatal("upstream request was not sent")
	}
	if proxiedRequest.URL.String() != strings.Replace(targetURL, "http://", "https://", 1) {
		t.Fatalf("upstream URL = %q", proxiedRequest.URL.String())
	}
	if proxiedRequest.Header.Get("Referer") != imageProxyReferer {
		t.Fatalf("Referer = %q", proxiedRequest.Header.Get("Referer"))
	}
	if proxiedRequest.Header.Get("If-None-Match") != `"browser-etag"` {
		t.Fatalf("If-None-Match = %q", proxiedRequest.Header.Get("If-None-Match"))
	}
}
