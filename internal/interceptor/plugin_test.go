package interceptor

import (
	"net/http"
	"strings"
	"testing"

	"wx_channel/internal/interceptor/proxy"
)

type channelPluginContext struct {
	req  *proxy.ContextReq
	res  *proxy.ContextRes
	body string
}

func (ctx *channelPluginContext) Req() *proxy.ContextReq {
	return ctx.req
}

func (ctx *channelPluginContext) Res() *proxy.ContextRes {
	return ctx.res
}

func (ctx *channelPluginContext) Mock(int, map[string]string, string) {}

func (ctx *channelPluginContext) GetResponseHeader(key string) string {
	return ctx.res.Header.Get(key)
}

func (ctx *channelPluginContext) SetResponseHeader(key string, val string) {
	ctx.res.Header.Set(key, val)
}

func (ctx *channelPluginContext) SetResponseBody(body string) {
	ctx.body = body
}

func (ctx *channelPluginContext) GetResponseBody() ([]byte, error) {
	return []byte(ctx.body), nil
}

func (ctx *channelPluginContext) SetStatusCode(code int) {
	ctx.res.StatusCode = code
}

func TestChannelInjectsShadcnCSSInlineAndOtherAssetsFromSameOrigin(t *testing.T) {
	cfg := &InterceptorConfig{
		APIServerProtocol: "http",
		APIServerHostname: "127.0.0.1",
		APIServerPort:     2022,
		APIServerAddr:     "127.0.0.1:2022",
	}
	files := newTestChannelInjectedFiles(t, map[string]string{
		"lib/timeless/0.28.0/timeless.shadcn.css": `@layer utilities{.tt-py-1{padding-block:calc(var(--spacing) * 1)}}`,
	})
	plugins := CreateChannelInterceptorPlugins(&Interceptor{
		Version:           "test-version",
		Settings:          cfg,
		FrontendVariables: map[string]any{},
	}, files)
	ctx := &channelPluginContext{
		req: &proxy.ContextReq{
			URL: &proxy.ContextURL{
				Path:     "/web/pages/feed",
				Hostname: func() string { return "channels.weixin.qq.com" },
			},
			Header: make(http.Header),
		},
		res: &proxy.ContextRes{
			Header: http.Header{},
		},
		body: "<!doctype html><html><head></head><body><p>feed</p></body></html>",
	}
	ctx.res.Header.Set("Content-Type", "text/html; charset=utf-8")

	plugins[0].OnResponse(ctx)

	if !strings.Contains(ctx.body, `<style>.tt-py-1{padding-block:calc(var(--spacing) * 1)}</style>`) {
		t.Fatalf("channel HTML does not contain inline shadcn CSS:\n%s", ctx.body)
	}
	if strings.Contains(ctx.body, "timeless.shadcn.css") {
		t.Fatalf("channel HTML should inline shadcn CSS instead of linking it:\n%s", ctx.body)
	}
	if strings.Contains(ctx.body, "@layer") {
		t.Fatalf("inline shadcn CSS should not contain cascade layer wrappers:\n%s", ctx.body)
	}
	componentsCSS := `href="/__wx_channels_assets/src/components.css"`
	if !strings.Contains(ctx.body, `<link rel="stylesheet" `+componentsCSS+`>`) {
		t.Fatalf("channel HTML does not contain components CSS stylesheet link:\n%s", ctx.body)
	}
	if strings.Contains(ctx.body, "127.0.0.1:2022/__wx_channels_assets") {
		t.Fatalf("channel HTML should use same-origin asset URLs:\n%s", ctx.body)
	}
	if !strings.Contains(ctx.body, `assetsBaseURL: "/__wx_channels_assets"`) {
		t.Fatalf("channel HTML does not override runtime asset base URL:\n%s", ctx.body)
	}
	cssIdx := strings.Index(ctx.body, ".tt-py-1")
	jsIdx := strings.Index(ctx.body, "timeless.weui.umd.min.js?v=test-version")
	envOverrideIdx := strings.Index(ctx.body, `assetsBaseURL: "/__wx_channels_assets"`)
	envScriptIdx := strings.Index(ctx.body, "/__wx_channels_assets/src/env.js")
	utilsScriptIdx := strings.Index(ctx.body, "/__wx_channels_assets/src/utils.js")
	channelsScriptIdx := strings.Index(ctx.body, "/__wx_channels_assets/src/channels.js")
	feedScriptIdx := strings.Index(ctx.body, "/__wx_channels_assets/src/feed.js")
	if cssIdx < 0 || jsIdx < 0 {
		t.Fatalf("expected both weui CSS and JS assets in injected HTML:\n%s", ctx.body)
	}
	if cssIdx > jsIdx {
		t.Fatal("weui CSS should be injected before weui JS")
	}
	if envOverrideIdx < 0 || envScriptIdx < 0 || envOverrideIdx > envScriptIdx {
		t.Fatalf("runtime asset base URL override should be injected before env.js:\n%s", ctx.body)
	}
	if channelsScriptIdx < 0 {
		t.Fatalf("channel HTML does not include channels.js:\n%s", ctx.body)
	}
	if utilsScriptIdx < 0 || channelsScriptIdx < utilsScriptIdx {
		t.Fatalf("channels.js should be injected after utils.js:\n%s", ctx.body)
	}
	if feedScriptIdx < 0 || channelsScriptIdx > feedScriptIdx {
		t.Fatalf("channels.js should be injected before page script:\n%s", ctx.body)
	}
}
