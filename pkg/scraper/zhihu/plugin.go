package zhihu

import (
	"fmt"
	"net/url"
	"strings"

	"wx_channel/frontend"
	"wx_channel/internal/interceptor"
	"wx_channel/internal/interceptor/proxy"
)

// CreateZhihuInterceptorPlugin creates a proxy plugin that injects the
// zhihu.main.js script into www.zhihu.com pages.
func CreateZhihuInterceptorPlugin(cookie string, asset_base_url string, version string) *proxy.Plugin {
	asset_base_url = "/__assets"
	url_build := frontend.NewURLBuild(asset_base_url, nil)
	asset_version := version
	if asset_version == "" {
		asset_version = "static"
	}
	version_query := url.Values{"v": []string{asset_version}}

	return &proxy.Plugin{
		Match: "zhihu.com",
		OnRequest: func(ctx proxy.Context) {
			interceptor.MockFrontendStaticAsset(ctx, ctx.Req().URL.Path, interceptor.FrontendStaticAssetMockOptions{
				PlatformPrefix: StaticAssetsPath + "/",
				PlatformFS:     Assets.InjectFS,
			})
		},
		OnResponse: func(ctx proxy.Context) {
			resp_content_type := strings.ToLower(ctx.GetResponseHeader("Content-Type"))
			hostname := ctx.Req().URL.Hostname()

			if hostname != "www.zhihu.com" || !strings.Contains(resp_content_type, "text/html") {
				return
			}

			resp_body, err := ctx.GetResponseBody()
			if err != nil {
				return
			}
			html := string(resp_body)

			var early_injected strings.Builder
			frontend.AppendScripts(
				&early_injected,
				"",
				url_build("/inject/fetch.js"),
			)
			frontend.AppendScripts(&early_injected, "", url_build("/public/timeless/0.30.0/timeless.umd.min.js", version_query))
			frontend.AppendScripts(&early_injected, "", url_build("/public/timeless/0.30.0/timeless.utils.umd.min.js", version_query))
			frontend.AppendStylesheets(&early_injected, "", url_build("/public/timeless/0.30.0/timeless.weui.css", version_query))
			frontend.AppendScripts(&early_injected, "", url_build("/public/timeless/0.30.0/timeless.weui.umd.min.js", version_query))
			frontend.AppendScripts(&early_injected, "", url_build("/public/timeless/0.30.0/timeless.dom.umd.min.js", version_query))
			frontend.AppendScripts(&early_injected, "", url_build("/public/timeless/0.30.0/timeless.web.umd.min.js", version_query))
			html = strings.Replace(html, "<head>", "<head>"+early_injected.String(), 1)

			var injected strings.Builder
			frontend.AppendInlineScript(
				&injected,
				"",
				fmt.Sprintf(`window.__d_config = { version: %q, assets_base_url: %q };`, version, asset_base_url),
			)
			frontend.AppendScripts(
				&injected,
				"",
				url_build("/public/mitt.umd.js"),
				url_build("/inject/eventbus.js"),
				url_build("/inject/env.js"),
				url_build("/inject/utils.js"),
				url_build("/inject/download/model.js"),
				InjectAssetURL(asset_base_url, "zhihu.main.js"),
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
