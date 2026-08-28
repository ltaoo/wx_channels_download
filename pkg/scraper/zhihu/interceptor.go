package zhihu

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/ltaoo/echo"
	"github.com/rs/zerolog"

	"wx_channel/frontend"
)

// InterceptorConfig contains the application values needed by the Zhihu
// injection rule.
type InterceptorConfig struct {
	Version           string
	GlobalScriptPath  string
	FrontendVariables map[string]any
}

// NewInterceptorPlugins builds the Echo injection rules owned by the Zhihu
// scraper.
func NewInterceptorPlugins(cfg InterceptorConfig, logger *zerolog.Logger) []*echo.Plugin {
	if logger == nil {
		nop_logger := zerolog.Nop()
		logger = &nop_logger
	} else {
		component_logger := logger.With().Str("component", "zhihu_scraper").Logger()
		logger = &component_logger
	}
	asset_base_url := "/__assets"
	url_build := frontend.NewURLBuild(asset_base_url, nil)
	asset_version := cfg.Version
	if asset_version == "" {
		asset_version = "static"
	}
	version_query := url.Values{"v": []string{asset_version}}

	plugin := &echo.Plugin{
		Match: "zhihu.com",
		OnRequest: func(ctx *echo.Context) {
			frontend.MockStaticAsset(ctx.Req.URL.Path, ctx.Req.Header, func(status int, headers map[string]string, body string) {
				ctx.Mock(status, headers, body)
			}, frontend.StaticAssetMockOptions{
				PlatformPrefix: InjectAssetsPath + "/",
				PlatformFS:     InjectAssets(),
				UserScriptPath: cfg.GlobalScriptPath,
				Logger:         logger,
			})
		},
		OnResponse: func(ctx *echo.Context) {
			response_content_type := strings.ToLower(ctx.GetResponseHeader("Content-Type"))
			hostname := ctx.Req.URL.Hostname()

			if hostname != "www.zhihu.com" || !strings.Contains(response_content_type, "text/html") {
				return
			}

			response_body, err := ctx.GetResponseBody()
			if err != nil {
				return
			}
			html := response_body

			var early_injected strings.Builder
			frontend.AppendScripts(&early_injected, "", url_build("/inject/fetch.js", version_query))
			frontend.AppendScripts(&early_injected, "", url_build("/public/timeless/0.32.0/timeless.umd.min.js", version_query))
			frontend.AppendStylesheets(&early_injected, "", url_build("/public/timeless/0.32.0/timeless.weui.css", version_query))
			frontend.AppendScripts(&early_injected, "", url_build("/public/timeless/0.32.0/timeless.weui.umd.min.js", version_query))
			frontend.AppendScripts(&early_injected, "", url_build("/public/timeless/0.32.0/timeless.dom.umd.min.js", version_query))
			frontend.AppendScripts(&early_injected, "", url_build("/public/timeless/0.32.0/timeless.web.umd.min.js", version_query))
			html = strings.Replace(html, "<head>", "<head>"+early_injected.String(), 1)

			var injected strings.Builder
			frontend_config := make(map[string]any, len(cfg.FrontendVariables)+2)
			for key, value := range cfg.FrontendVariables {
				frontend_config[key] = value
			}
			frontend_config["version"] = cfg.Version
			frontend_config["assets_base_url"] = asset_base_url
			frontend_config_byte, _ := json.Marshal(frontend_config)
			frontend.AppendInlineScript(&injected, "", fmt.Sprintf(`window.__d_config = %s;`, frontend_config_byte))
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
			if cfg.GlobalScriptPath != "" {
				frontend.AppendScripts(&injected, "", frontend.UserGlobalScriptAssetPath(cfg.GlobalScriptPath))
			}

			html = strings.Replace(html, "</body>", injected.String()+"</body>", 1)
			ctx.SetResponseBody(html)
		},
	}
	return []*echo.Plugin{plugin}
}
