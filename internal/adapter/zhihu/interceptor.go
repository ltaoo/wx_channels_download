package zhihuadapter

import (
	"fmt"
	"net/url"
	"strings"

	"wx_channel/frontend"
	"wx_channel/internal/config"
	"wx_channel/internal/interceptor"
	"wx_channel/internal/interceptor/proxy"
	"wx_channel/pkg/scraper/zhihu"
)

// InterceptorPluginConfig owns the zhihu scraper configuration used
// by the local interceptor.
type InterceptorPluginConfig struct {
	version            string
	global_script_path string
}

// NewConfig creates an InterceptorPluginConfig from the application config.
func NewConfig(cfg *config.Config) *InterceptorPluginConfig {
	if cfg == nil {
		return &InterceptorPluginConfig{}
	}
	return &InterceptorPluginConfig{
		version:            cfg.Version,
		global_script_path: cfg.GlobalScriptPath,
	}
}

// GetPlugins returns zhihu injection and callback plugins wired to
// adapter-owned persistence and browse events.
func (c *InterceptorPluginConfig) GetPlugins() []interface{} {
	if c == nil {
		return nil
	}

	plugin := create_zhihu_interceptor_plugin(c.version, c.global_script_path)
	if plugin == nil {
		return nil
	}
	return []interface{}{plugin}
}

// create_zhihu_interceptor_plugin injects zhihu.main.js into www.zhihu.com pages.
func create_zhihu_interceptor_plugin(version, global_script_path string) *proxy.Plugin {
	asset_base_url := "/__assets"
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
				PlatformPrefix: static_assets_path + "/",
				PlatformFS:     zhihu.InjectAssets(),
				UserScriptPath: global_script_path,
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
				url_build("/inject/fetch.js", version_query),
			)
			frontend.AppendScripts(&early_injected, "", url_build("/public/timeless/0.31.4/timeless.umd.min.js", version_query))
			frontend.AppendStylesheets(&early_injected, "", url_build("/public/timeless/0.31.4/timeless.weui.css", version_query))
			frontend.AppendScripts(&early_injected, "", url_build("/public/timeless/0.31.4/timeless.weui.umd.min.js", version_query))
			frontend.AppendScripts(&early_injected, "", url_build("/public/timeless/0.31.4/timeless.dom.umd.min.js", version_query))
			frontend.AppendScripts(&early_injected, "", url_build("/public/timeless/0.31.4/timeless.web.umd.min.js", version_query))
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
				url_build("/inject/eventbus.js", version_query),
				url_build("/public/dl.utils.js", version_query),
				url_build("/public/dl.sdk.js", version_query),
				url_build("/inject/env.js", version_query),
				url_build("/inject/utils.js", version_query),
			)
			frontend.AppendScripts(
				&injected,
				"",
				url_build("/inject/download/model.js", version_query),
				asset_url(asset_base_url, "/inject/zhihu.main.js", version_query),
			)
			if global_script_path != "" {
				frontend.AppendScripts(&injected, "", frontend.UserGlobalScriptAssetPath(global_script_path))
			}

			html = strings.Replace(html, "</body>", injected.String()+"</body>", 1)
			ctx.SetResponseBody(html)
		},
	}
}
