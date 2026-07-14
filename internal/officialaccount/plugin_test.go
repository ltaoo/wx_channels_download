package officialaccount

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wx_channel/internal/interceptor"
	"wx_channel/internal/interceptor/proxy"
)

type officialAccountPluginContext struct {
	req     *proxy.ContextReq
	res     *proxy.ContextRes
	body    string
	status  int
	headers map[string]string
	mocked  bool
}

func (ctx *officialAccountPluginContext) Req() *proxy.ContextReq {
	return ctx.req
}

func (ctx *officialAccountPluginContext) Res() *proxy.ContextRes {
	return ctx.res
}

func (ctx *officialAccountPluginContext) Mock(status int, headers map[string]string, body string) {
	ctx.status = status
	ctx.headers = headers
	ctx.body = body
	ctx.mocked = true
}

func (ctx *officialAccountPluginContext) GetResponseHeader(key string) string {
	return ctx.res.Header.Get(key)
}

func (ctx *officialAccountPluginContext) SetResponseHeader(key string, val string) {
	ctx.res.Header.Set(key, val)
}

func (ctx *officialAccountPluginContext) SetResponseBody(body string) {
	ctx.body = body
}

func (ctx *officialAccountPluginContext) GetResponseBody() ([]byte, error) {
	return []byte(ctx.body), nil
}

func (ctx *officialAccountPluginContext) SetStatusCode(code int) {
	ctx.res.StatusCode = code
}

func TestOfficialAccountInjectsTimelessShadcnCSSInline(t *testing.T) {
	cfg := &OfficialAccountConfig{
		Enabled:  true,
		Protocol: "http",
		Hostname: "127.0.0.1",
		Port:     2022,
	}
	files := newOfficialAccountTestInjectedFiles(t, map[string]string{
		"lib/timeless/0.28.0/timeless.shadcn.css": `@layer utilities{.tt-py-1{padding-block:calc(var(--spacing) * 1)}}`,
	})
	plugin := CreateOfficialAccountInterceptorPlugin(cfg, files, "test-version")
	ctx := &officialAccountPluginContext{
		req: &proxy.ContextReq{
			URL: &proxy.ContextURL{
				Path:     "/s",
				Hostname: func() string { return "mp.weixin.qq.com" },
			},
			Header: make(http.Header),
		},
		res: &proxy.ContextRes{
			Header: http.Header{},
		},
		body: "<!doctype html><html><head></head><body><p>article</p></body></html>",
	}
	ctx.res.Header.Set("Content-Type", "text/html; charset=utf-8")
	ctx.res.Header.Set("Content-Security-Policy", "default-src 'self'; script-src 'nonce-testnonce'; style-src 'self' 'unsafe-inline'")

	plugin.OnResponse(ctx)

	if !strings.Contains(ctx.body, `"officialAccountEnabled":true`) {
		t.Fatalf("official account HTML does not expose officialAccountEnabled=true:\n%s", ctx.body)
	}

	if !strings.Contains(ctx.body, `<style nonce="testnonce">.tt-py-1{padding-block:calc(var(--spacing) * 1)}</style>`) {
		t.Fatalf("official account HTML does not contain inline shadcn CSS:\n%s", ctx.body)
	}
	if strings.Contains(ctx.body, "timeless.shadcn.css") {
		t.Fatalf("official account HTML should inline shadcn CSS instead of linking it:\n%s", ctx.body)
	}
	if strings.Contains(ctx.body, "@layer") {
		t.Fatalf("inline shadcn CSS should not contain cascade layer wrappers:\n%s", ctx.body)
	}
	componentsCSS := `href="/__wx_channels_assets/src/components.css"`
	if !strings.Contains(ctx.body, `<link nonce="testnonce" rel="stylesheet" `+componentsCSS+`>`) {
		t.Fatalf("official account HTML does not contain components CSS stylesheet link:\n%s", ctx.body)
	}
	if strings.Contains(ctx.body, "127.0.0.1:2022/__wx_channels_assets") {
		t.Fatalf("official account HTML should use same-origin asset URLs:\n%s", ctx.body)
	}
	if !strings.Contains(ctx.body, `assetsBaseURL: "/__wx_channels_assets"`) {
		t.Fatalf("official account HTML does not override runtime asset base URL:\n%s", ctx.body)
	}
	cssIdx := strings.Index(ctx.body, ".tt-py-1")
	jsIdx := strings.Index(ctx.body, "timeless.weui.umd.min.js?v=test-version")
	if cssIdx < 0 || jsIdx < 0 {
		t.Fatalf("expected both weui CSS and JS assets in injected HTML:\n%s", ctx.body)
	}
	if cssIdx > jsIdx {
		t.Fatalf("weui CSS should be injected before weui JS")
	}
}

func TestOfficialAccountMocksSameOriginStaticAssets(t *testing.T) {
	files := newOfficialAccountTestInjectedFiles(t, map[string]string{
		"src/components.css": ".wx-components{}",
	})
	plugin := CreateOfficialAccountInterceptorPlugin(&OfficialAccountConfig{}, files, "test-version")
	ctx := &officialAccountPluginContext{
		req: &proxy.ContextReq{
			URL: &proxy.ContextURL{
				Path:     "/__wx_channels_assets/src/components.css",
				Hostname: func() string { return "mp.weixin.qq.com" },
			},
			Header: make(http.Header),
		},
		res: &proxy.ContextRes{
			Header: http.Header{},
		},
	}

	plugin.OnRequest(ctx)

	if !ctx.mocked {
		t.Fatal("expected same-origin static asset request to be mocked")
	}
	if ctx.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", ctx.status, http.StatusOK)
	}
	if got := ctx.headers["Content-Type"]; got != "text/css; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/css", got)
	}
	if ctx.body != ".wx-components{}" {
		t.Fatalf("body = %q, want asset content", ctx.body)
	}
}

func newOfficialAccountTestInjectedFiles(t *testing.T, assets map[string]string) *interceptor.ChannelInjectedFiles {
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
	return interceptor.NewChannelInjectedFiles(injectDir)
}
