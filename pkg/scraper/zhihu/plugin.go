package zhihu

import (
	"fmt"
	"strings"

	"wx_channel/frontend"
	"wx_channel/internal/interceptor/proxy"
)

// CreateZhihuInterceptorPlugin creates a proxy plugin that injects the
// zhihu.main.js script into www.zhihu.com pages and serves platform-owned
// static assets for same-origin requests intercepted by the local proxy.
func CreateZhihuInterceptorPlugin(cookie string, files *frontend.ChannelInjectedFiles, version string) *proxy.Plugin {
	assetBaseURL := frontend.ChannelAssetsSameOriginBaseURL()

	return &proxy.Plugin{
		Match: "zhihu.com",
		OnRequest: func(ctx proxy.Context) {
			if ctx.Req().URL.Hostname() == "www.zhihu.com" {
				if frontend.MockChannelStaticAsset(ctx, ctx.Req().URL.Path, files) || MockStaticAsset(ctx, ctx.Req().URL.Path) {
					return
				}
			}
		},
		OnResponse: func(ctx proxy.Context) {
			respContentType := strings.ToLower(ctx.GetResponseHeader("Content-Type"))
			hostname := ctx.Req().URL.Hostname()

			if hostname != "www.zhihu.com" || !strings.Contains(respContentType, "text/html") {
				return
			}

			respBody, err := ctx.GetResponseBody()
			if err != nil {
				return
			}
			html := string(respBody)

			var injected strings.Builder
			frontend.AppendInlineScript(
				&injected,
				"",
				fmt.Sprintf(`window.__wx_channels_env__ = Object.assign(window.__wx_channels_env__ || {}, { assetsBaseURL: %q });`, assetBaseURL),
			)
			frontend.AppendScriptSrcs(
				&injected,
				"",
				InjectAssetURL(assetBaseURL, "zhihu.main.js"),
			)

			html = strings.Replace(html, "</body>", injected.String()+"</body>", 1)
			ctx.SetResponseBody(html)
		},
	}
}

// CreateZhihuArticleLoadedPlugin creates a no-op plugin for zhihu content.
// Used as a placeholder to satisfy the interceptor plugin interface contract.
func CreateZhihuArticleLoadedPlugin(_ map[string]string) *proxy.Plugin {
	return nil
}
