package douban

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

func TestNewImageProxyRequestSetsDoubanHeaders(t *testing.T) {
	inbound := httptest.NewRequest(http.MethodGet, "/douban/proxy", nil)
	inbound.Header.Set("If-None-Match", `"douban-etag"`)

	req, err := newImageProxyRequest(
		context.Background(),
		"http://img1.doubanio.com/view/photo/s_ratio_poster/public/p2186920269.webp",
		"https://movie.douban.com/subject/1393859/",
		inbound,
	)
	if err != nil {
		t.Fatalf("newImageProxyRequest: %v", err)
	}
	if req.URL.Scheme != "https" {
		t.Fatalf("scheme = %q", req.URL.Scheme)
	}
	if req.Header.Get("Referer") != "https://movie.douban.com/subject/1393859/" {
		t.Fatalf("Referer = %q", req.Header.Get("Referer"))
	}
	if req.Header.Get("User-Agent") != imageProxyUserAgent {
		t.Fatalf("User-Agent = %q", req.Header.Get("User-Agent"))
	}
	if req.Header.Get("Sec-Fetch-Dest") != "image" {
		t.Fatalf("Sec-Fetch-Dest = %q", req.Header.Get("Sec-Fetch-Dest"))
	}
	if !strings.Contains(req.Header.Get("Accept"), "image/webp") {
		t.Fatalf("Accept = %q", req.Header.Get("Accept"))
	}
	if req.Header.Get("If-None-Match") != `"douban-etag"` {
		t.Fatalf("If-None-Match = %q", req.Header.Get("If-None-Match"))
	}
}

func TestNewImageProxyRequestRejectsNonDoubanImageHosts(t *testing.T) {
	for _, rawURL := range []string{
		"https://example.com/image.webp",
		"https://notdoubanio.com/image.webp",
		"file:///tmp/image.webp",
	} {
		if _, err := newImageProxyRequest(context.Background(), rawURL, "", nil); err == nil {
			t.Fatalf("newImageProxyRequest(%q) returned nil error", rawURL)
		}
	}
}

func TestNormalizeImageProxyReferer(t *testing.T) {
	if got := normalizeImageProxyReferer("http://movie.douban.com/subject/1393859/"); got != "https://movie.douban.com/subject/1393859/" {
		t.Fatalf("referer = %q", got)
	}
	for _, raw := range []string{"", "https://example.com/", "file:///tmp/a"} {
		if got := normalizeImageProxyReferer(raw); got != defaultImageProxyReferer {
			t.Fatalf("normalizeImageProxyReferer(%q) = %q", raw, got)
		}
	}
}

func TestHandleImageProxy(t *testing.T) {
	oldClient := doubanImageProxyHTTPClient
	defer func() {
		doubanImageProxyHTTPClient = oldClient
	}()

	var proxiedRequest *http.Request
	doubanImageProxyHTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			proxiedRequest = req
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": {"image/webp"},
					"ETag":         {`"douban-img-etag"`},
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
	targetURL := "https://img2.doubanio.com/view/photo/sqxs/public/p737577671.webp"
	referer := "https://movie.douban.com/subject/1393859/"
	ctx.Request = httptest.NewRequest(http.MethodGet, "/douban/proxy?url="+url.QueryEscape(targetURL)+"&referer="+url.QueryEscape(referer), nil)
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
	if w.Header().Get("ETag") != `"douban-img-etag"` {
		t.Fatalf("ETag = %q", w.Header().Get("ETag"))
	}
	if proxiedRequest == nil {
		t.Fatal("upstream request was not sent")
	}
	if proxiedRequest.URL.String() != targetURL {
		t.Fatalf("upstream URL = %q", proxiedRequest.URL.String())
	}
	if proxiedRequest.Header.Get("Referer") != referer {
		t.Fatalf("Referer = %q", proxiedRequest.Header.Get("Referer"))
	}
	if proxiedRequest.Header.Get("If-None-Match") != `"browser-etag"` {
		t.Fatalf("If-None-Match = %q", proxiedRequest.Header.Get("If-None-Match"))
	}
}
