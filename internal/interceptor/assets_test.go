package interceptor

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wx_channel/internal/interceptor/proxy"
)

type staticAssetContext struct {
	req     *proxy.ContextReq
	status  int
	headers map[string]string
	body    string
	mocked  bool
}

func (ctx *staticAssetContext) Req() *proxy.ContextReq {
	return ctx.req
}

func (ctx *staticAssetContext) Res() *proxy.ContextRes {
	return &proxy.ContextRes{}
}

func (ctx *staticAssetContext) Mock(status int, headers map[string]string, body string) {
	ctx.status = status
	ctx.headers = headers
	ctx.body = body
	ctx.mocked = true
}

func (ctx *staticAssetContext) GetResponseHeader(string) string {
	return ""
}

func (ctx *staticAssetContext) SetResponseHeader(string, string) {}

func (ctx *staticAssetContext) SetResponseBody(string) {}

func (ctx *staticAssetContext) GetResponseBody() ([]byte, error) {
	return nil, nil
}

func (ctx *staticAssetContext) SetStatusCode(int) {}

func TestMockChannelStaticAssetLibUsesThirtyDayStrongCache(t *testing.T) {
	files := newTestChannelInjectedFiles(t, map[string]string{
		"lib/mitt.umd.js": "window.mitt = {};",
	})
	ctx := newStaticAssetContext("/__wx_channels_assets/lib/mitt.umd.js", nil)

	if !mockChannelStaticAsset(ctx, ctx.req.URL.Path, files) {
		t.Fatal("expected lib asset to be mocked")
	}
	if ctx.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", ctx.status, http.StatusOK)
	}
	if got := ctx.headers["Cache-Control"]; got != channelLibAssetCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", got, channelLibAssetCacheControl)
	}
	if got := ctx.body; got != "window.mitt = {};" {
		t.Fatalf("body = %q, want lib asset content", got)
	}
}

func TestMockChannelStaticAssetShadcnCSSStripsCascadeLayers(t *testing.T) {
	files := newTestChannelInjectedFiles(t, map[string]string{
		"lib/timeless/0.28.0/timeless.shadcn.css": `@layer theme{:root{--spacing:.25rem}}@layer utilities{.tt-py-1{padding-block:calc(var(--spacing) * 1)}}@layer components;`,
	})
	ctx := newStaticAssetContext("/__wx_channels_assets/lib/timeless/0.28.0/timeless.shadcn.css", nil)

	if !mockChannelStaticAsset(ctx, ctx.req.URL.Path, files) {
		t.Fatal("expected shadcn CSS asset to be mocked")
	}
	if strings.Contains(ctx.body, "@layer") {
		t.Fatalf("body should not contain cascade layer wrappers: %q", ctx.body)
	}
	if !strings.Contains(ctx.body, ":root{--spacing:.25rem}") {
		t.Fatalf("body does not contain unwrapped theme variables: %q", ctx.body)
	}
	if !strings.Contains(ctx.body, ".tt-py-1{padding-block:calc(var(--spacing) * 1)}") {
		t.Fatalf("body does not contain unwrapped utility rule: %q", ctx.body)
	}
}

func TestStripTopLevelCascadeLayersPreservesNestedAtRules(t *testing.T) {
	css := `/* keep */@layer utilities{@media (hover:hover){.tt-hover\:tt-bg-zinc-800:hover{background:#27272a}}.tt-content{content:"}"}}.plain{display:block}`

	got := stripTopLevelCascadeLayers(css)

	if strings.Contains(got, "@layer") {
		t.Fatalf("got %q, want layer wrappers stripped", got)
	}
	if !strings.Contains(got, `@media (hover:hover){.tt-hover\:tt-bg-zinc-800:hover{background:#27272a}}`) {
		t.Fatalf("got %q, want nested @media preserved", got)
	}
	if !strings.Contains(got, `.tt-content{content:"}"}`) {
		t.Fatalf("got %q, want strings with braces preserved", got)
	}
	if !strings.Contains(got, `.plain{display:block}`) {
		t.Fatalf("got %q, want unlayered rules preserved", got)
	}
}

func TestMockChannelStaticAssetSrcUsesETagRevalidation(t *testing.T) {
	body := "console.log('src');"
	files := newTestChannelInjectedFiles(t, map[string]string{
		"src/utils.js": body,
	})
	ctx := newStaticAssetContext("/__wx_channels_assets/src/utils.js", nil)

	if !mockChannelStaticAsset(ctx, ctx.req.URL.Path, files) {
		t.Fatal("expected src asset to be mocked")
	}
	etag := channelStaticAssetETag([]byte(body))
	if ctx.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", ctx.status, http.StatusOK)
	}
	if got := ctx.headers["Cache-Control"]; got != channelSrcAssetCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", got, channelSrcAssetCacheControl)
	}
	if got := ctx.headers["ETag"]; got != etag {
		t.Fatalf("ETag = %q, want %q", got, etag)
	}

	cachedCtx := newStaticAssetContext("/__wx_channels_assets/src/utils.js", http.Header{
		"If-None-Match": []string{etag},
	})
	if !mockChannelStaticAsset(cachedCtx, cachedCtx.req.URL.Path, files) {
		t.Fatal("expected cached src asset to be mocked")
	}
	if cachedCtx.status != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", cachedCtx.status, http.StatusNotModified)
	}
	if cachedCtx.body != "" {
		t.Fatalf("body = %q, want empty 304 body", cachedCtx.body)
	}
}

func TestChannelAssetsBaseURLUsesAPIConfig(t *testing.T) {
	tests := []struct {
		name     string
		proto    string
		host     string
		port     int
		expected string
	}{
		{
			name:     "default local api",
			proto:    "http",
			host:     "127.0.0.1",
			port:     2022,
			expected: "http://127.0.0.1:2022/__wx_channels_assets",
		},
		{
			name:     "host already includes port",
			proto:    "https",
			host:     "example.test:443",
			port:     2022,
			expected: "https://example.test:443/__wx_channels_assets",
		},
		{
			name:     "wildcard host becomes loopback",
			proto:    "http",
			host:     "0.0.0.0",
			port:     2022,
			expected: "http://127.0.0.1:2022/__wx_channels_assets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ChannelAssetsBaseURL(tt.proto, tt.host, tt.port); got != tt.expected {
				t.Fatalf("ChannelAssetsBaseURL() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func newTestChannelInjectedFiles(t *testing.T, assets map[string]string) *ChannelInjectedFiles {
	t.Helper()

	injectDir := t.TempDir()
	for rel, content := range assets {
		fullPath := filepath.Join(injectDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", fullPath, err)
		}
	}
	return NewChannelInjectedFiles(injectDir)
}

func newStaticAssetContext(pathname string, header http.Header) *staticAssetContext {
	if header == nil {
		header = make(http.Header)
	}
	return &staticAssetContext{
		req: &proxy.ContextReq{
			URL: &proxy.ContextURL{
				Path:     pathname,
				Hostname: func() string { return "channels.weixin.qq.com" },
			},
			Header: header,
		},
	}
}
