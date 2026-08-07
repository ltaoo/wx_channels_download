package zhihu

import (
	"fmt"
	"net/url"
	"strings"

	"wx_channel/frontend"
	"wx_channel/internal/interceptor/proxy"
)

// CreateZhihuInterceptorPlugin creates a proxy plugin that injects the
// zhihu.main.js script into www.zhihu.com pages.
func CreateZhihuInterceptorPlugin(cookie string, assetBaseURL string, version string) *proxy.Plugin {
	urlBuild := frontend.NewURLBuild(assetBaseURL, nil)
	assetVersion := version
	if assetVersion == "" {
		assetVersion = "static"
	}
	versionQuery := url.Values{"v": []string{assetVersion}}

	return &proxy.Plugin{
		Match: "zhihu.com",
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

			var earlyInjected strings.Builder
			frontend.AppendScripts(
				&earlyInjected,
				"",
				urlBuild("/inject/fetch.js"),
			)
			frontend.AppendScripts(&earlyInjected, "", urlBuild("/public/timeless/0.30.0/timeless.umd.min.js", versionQuery))
			frontend.AppendScripts(&earlyInjected, "", urlBuild("/public/timeless/0.30.0/timeless.utils.umd.min.js", versionQuery))
			frontend.AppendStylesheets(&earlyInjected, "", urlBuild("/public/timeless/0.30.0/timeless.weui.css", versionQuery))
			frontend.AppendScripts(&earlyInjected, "", urlBuild("/public/timeless/0.30.0/timeless.weui.umd.min.js", versionQuery))
			frontend.AppendScripts(&earlyInjected, "", urlBuild("/public/timeless/0.30.0/timeless.dom.umd.min.js", versionQuery))
			frontend.AppendScripts(&earlyInjected, "", urlBuild("/public/timeless/0.30.0/timeless.web.umd.min.js", versionQuery))
			html = strings.Replace(html, "<head>", "<head>"+earlyInjected.String(), 1)

			var injected strings.Builder
			frontend.AppendInlineScript(
				&injected,
				"",
				fmt.Sprintf(`window.__d_config = { version: %q, assets_base_url: %q };`, version, assetBaseURL),
			)
			frontend.AppendScripts(
				&injected,
				"",
				urlBuild("/inject/eventbus.js"),
				urlBuild("/inject/env.js"),
				urlBuild("/inject/utils.js"),
				urlBuild("/inject/download/model.js"),
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
