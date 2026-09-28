package wxmp

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/ltaoo/echo"
	"github.com/rs/zerolog"

	"wx_channel/frontend"
)

var (
	csp_nonce_reg = regexp.MustCompile(`'nonce-([^']+)'`)

	// WeChat serves its H5 bundles with a one-year cache header and references
	// them with its own ?v=<build> query, so both the article HTML and the
	// bundles themselves are stamped with the app version to force a refetch
	// through the proxy. The original query is swallowed and replaced; the CDN
	// serves the same bytes either way. Same rules as the wxchannels scraper.
	html_script_src_reg  = regexp.MustCompile(`src="([^"]{1,})\.js(\?[^"]*)?"`)
	html_script_href_reg = regexp.MustCompile(`href="([^"]{1,})\.js(\?[^"]*)?"`)
	js_from_reg          = regexp.MustCompile(`from {0,1}"([^"]{1,})\.js(\?[^"]*)?"`)
	js_dep_reg           = regexp.MustCompile(`"js/([^"]{1,})\.js(\?[^"]*)?"`)
	js_lazy_import_reg   = regexp.MustCompile(`import\("([^"]{1,})\.js(\?[^"]*)?"\)`)
	js_import_reg        = regexp.MustCompile(`import {0,1}"([^"]{1,})\.js(\?[^"]*)?"`)

	// res.wx.qq.com ships the official-account H5 bundles minified in a single
	// line, so FeedObejectHandler.getCommentDetail has to be matched with a
	// regex instead of a parser.
	//
	// Groups: 1/2 = method arguments, 3 = the whole __awaiter(...) expression
	// that the method returns.
	js_mp_get_comment_detail_reg = regexp.MustCompile(
		`getCommentDetail\(([A-Za-z_$][\w$]*),([A-Za-z_$][\w$]*)\)\{return ` +
			`([A-Za-z_$][\w$]*\(this,void 0,void 0,\(function\*\(\)\{(?s:.*?)\}\)\))` +
			`\}`,
	)
)

// InterceptorConfig contains the application values needed by the
// official-account injection rule.
type InterceptorConfig struct {
	Version  string
	Mode     string
	Settings OfficialAccountConfig
}

// NewInterceptorPlugins builds the Echo injection rules owned by the
// official-account scraper.
func NewInterceptorPlugins(cfg InterceptorConfig, logger *zerolog.Logger) []*echo.Plugin {
	if logger == nil {
		nop_logger := zerolog.Nop()
		logger = &nop_logger
	} else {
		component_logger := logger.With().Str("component", "wxmp_scraper").Logger()
		logger = &component_logger
	}
	settings := &cfg.Settings
	asset_base_url := "/__assets"
	url_build := frontend.NewURLBuild(asset_base_url, nil)
	asset_version := cfg.Version
	if asset_version == "" {
		asset_version = "static"
	}
	version_query := url.Values{"v": []string{asset_version}}
	inline_assets := cfg.Mode == "release" || cfg.Mode == "prod"
	inline_asset_options := frontend.StaticAssetMockOptions{
		PlatformPrefix: InjectAssetsPath + "/",
		PlatformFS:     InjectAssets(),
	}
	plugin := &echo.Plugin{
		Match: "qq.com",
		OnRequest: func(ctx *echo.Context) {
			if ctx.Req.URL.Hostname() != "mp.weixin.qq.com" {
				return
			}
			frontend.MockStaticAsset(ctx.Req.URL.Path, ctx.Req.Header, func(status int, headers map[string]string, body string) {
				ctx.Mock(status, headers, body)
			}, frontend.StaticAssetMockOptions{
				PlatformPrefix: InjectAssetsPath + "/",
				PlatformFS:     InjectAssets(),
				UserScriptPath: settings.GlobalScriptPath,
				Logger:         logger,
			})
		},
		OnResponse: func(ctx *echo.Context) {
			response_content_type := strings.ToLower(ctx.GetResponseHeader("Content-Type"))
			hostname := ctx.Req.URL.Hostname()
			if hostname != "mp.weixin.qq.com" || !strings.Contains(response_content_type, "text/html") {
				return
			}
			response_body, err := ctx.GetResponseBody()
			if err != nil {
				return
			}
			html_content := response_body
			// The webview would otherwise reuse the cached bundles and never ask
			// the proxy for them, so the version stamp is what makes the
			// res.wx.qq.com rewrite below reach the page.
			v := "?t=" + cfg.Version
			html_content = html_script_src_reg.ReplaceAllString(html_content, `src="$1.js`+v+`"`)
			html_content = html_script_href_reg.ReplaceAllString(html_content, `href="$1.js`+v+`"`)
			csp := ctx.GetResponseHeader("Content-Security-Policy") + " " + ctx.GetResponseHeader("Content-Security-Policy-Report-Only")
			mp_websocket_url := build_mp_websocket_url(settings)
			rewrite_response_csp(ctx, asset_base_url, mp_websocket_url)
			variables := BuildOfficialAccountVariables(html_content)
			script_attr := ""
			style_attr := ""
			if match := csp_nonce_reg.FindStringSubmatch(csp); len(match) > 1 {
				script_attr = fmt.Sprintf(` nonce="%s" reportloaderror`, match[1])
				style_attr = fmt.Sprintf(` nonce="%s"`, match[1])
			}
			var injected strings.Builder
			append_scripts := func(srcs ...string) {
				if err := frontend.AppendRegisteredScripts(&injected, script_attr, inline_assets, inline_asset_options, srcs...); err != nil {
					logger.Warn().Err(err).Msg("failed to inline registered scripts; using external assets")
				}
			}
			append_stylesheets := func(hrefs ...string) {
				if err := frontend.AppendRegisteredStylesheets(&injected, style_attr, inline_assets, inline_asset_options, hrefs...); err != nil {
					logger.Warn().Err(err).Msg("failed to inline registered stylesheets; using external assets")
				}
			}
			if settings.DebugShowError {
				append_scripts(url_build("/inject/error.js", version_query))
			}
			append_scripts(url_build("/public/timeless/0.33.1/timeless.umd.min.js", version_query))
			append_stylesheets(url_build("/public/timeless/0.33.1/timeless.weui.css", version_query))
			append_scripts(url_build("/public/timeless/0.33.1/timeless.weui.umd.min.js", version_query))
			append_scripts(url_build("/public/timeless/0.33.1/timeless.dom.umd.min.js", version_query))
			append_scripts(url_build("/public/timeless/0.33.1/timeless.web.umd.min.js", version_query))
			append_stylesheets(url_build("/inject/components.css"))
			frontend_config := make(map[string]any, len(variables)+2)
			cfg_byte, _ := json.Marshal(settings)
			_ = json.Unmarshal(cfg_byte, &frontend_config)
			for key, value := range variables {
				frontend_config[key] = value
			}
			api_host := settings.Addr
			if api_host == "" && settings.Hostname != "" {
				api_host = net.JoinHostPort(strings.Trim(settings.Hostname, "[]"), strconv.Itoa(settings.Port))
			}
			api_protocol := strings.TrimSuffix(strings.TrimSpace(settings.Protocol), ":")
			if api_protocol == "" {
				api_protocol = "http"
			}
			frontend_config["version"] = cfg.Version
			frontend_config["assets_base_url"] = asset_base_url
			frontend_config["apiHost"] = api_host
			frontend_config["apiOrigin"] = api_protocol + "://" + api_host
			frontend_config["apiProtocol"] = settings.Protocol
			frontend_config["mpWSURL"] = mp_websocket_url
			frontend_config_byte, _ := json.Marshal(frontend_config)
			frontend.AppendInlineScript(&injected, script_attr, fmt.Sprintf(`window.__d_config = %s;`, frontend_config_byte))
			append_scripts(
				url_build("/inject/eventbus.js", version_query),
				url_build("/public/dl.utils.js", version_query),
				url_build("/public/dl.sdk.js", version_query),
				url_build("/inject/env.js", version_query),
				url_build("/inject/utils.js", version_query),
				url_build("/inject/components.js", version_query),
				url_build("/public/virtual-list-view.js", version_query),
				url_build("/inject/download/model.js", version_query),
				url_build("/inject/download/view.js", version_query),
				asset_url(asset_base_url, "/inject/mp.utils.js", version_query),
				// asset_url(asset_base_url, "/inject/mp.ws.js", version_query),
				asset_url(asset_base_url, "/inject/mp.components.js", version_query),
				asset_url(asset_base_url, "/inject/mp.main.js", version_query),
			)
			if settings.GlobalScriptURL != "" {
				frontend.AppendScripts(&injected, script_attr, settings.GlobalScriptURL)
			}
			if settings.InjectContentScript != "" {
				frontend.AppendInlineScript(&injected, script_attr, settings.InjectContentScript)
			}
			html_content = strings.Replace(html_content, "</body>", injected.String()+"</body>", 1)
			ctx.SetResponseBody(html_content)
		},
	}
	// res.wx.qq.com serves the bundles used by mp.weixin.qq.com pages, so the
	// article page cannot be patched from the HTML hook alone. Like the
	// wxchannels plugin, every bundle gets its imports stamped and the pathname
	// decides the extra rewrites; more pathnames can be handled by adding
	// branches here.
	js_plugin := &echo.Plugin{
		Match: "res.wx.qq.com",
		OnResponse: func(ctx *echo.Context) {
			response_content_type := strings.ToLower(ctx.GetResponseHeader("Content-Type"))
			hostname := ctx.Req.URL.Hostname()
			pathname := ctx.Req.URL.Path
			if hostname != "res.wx.qq.com" || !strings.Contains(response_content_type, "javascript") {
				return
			}
			// The wasm loader is transported untouched: stamping its imports
			// breaks the video decoder. Same exception as the wxchannels scraper.
			if strings.Contains(pathname, "wasm_video_decode") {
				return
			}
			js_script, err := ctx.GetResponseBody()
			if err != nil {
				return
			}
			// Every import of a sibling bundle is stamped with the app version,
			// matching the stamp put on the page's <script> tags, so the whole
			// module graph is refetched through the proxy instead of being
			// served from the webview cache.
			v := "?t=" + cfg.Version
			rewritten := js_script
			rewritten = js_from_reg.ReplaceAllString(rewritten, `from"$1.js`+v+`"`)
			rewritten = js_dep_reg.ReplaceAllString(rewritten, `"js/$1.js`+v+`"`)
			rewritten = js_lazy_import_reg.ReplaceAllString(rewritten, `import("$1.js`+v+`")`)
			rewritten = js_import_reg.ReplaceAllString(rewritten, `import"$1.js`+v+`"`)
			// Article pages embed a 视频号 video by calling
			// FeedObejectHandler.getCommentDetail, which fetches the feed
			// object from /cgi-bin/micromsg-bin/h5_findergetcommentdetail.
			// Chain a .then onto the promise it returns so the feed is
			// emitted before the caller receives it. $1/$2 are the original
			// arguments and $3 the original __awaiter(...) expression, so the
			// original body and the resolved value stay untouched.
			if strings.Contains(pathname, "common_share_video") {
				rewritten = js_mp_get_comment_detail_reg.ReplaceAllString(
					rewritten,
					`getCommentDetail($1,$2){return $3.then(function(result){
						var feed = result && result.object;
						typeof WXU !== "undefined" && WXU.emit("channels:OnFeedProfileLoaded", feed);
						return result;
					})}`,
				)
			}
			if rewritten == js_script {
				return
			}
			logger.Info().
				Str("file", "pkg/scraper/wxmp/interceptor.go").
				Str("pathname", pathname).
				Msg("wxmp interceptor rewrote res.wx.qq.com bundle")
			ctx.SetResponseBody(rewritten)
		},
	}
	return []*echo.Plugin{plugin, js_plugin}
}

func rewrite_response_csp(ctx *echo.Context, asset_base_url string, websocket_url string) {
	for _, header := range []string{"Content-Security-Policy", "Content-Security-Policy-Report-Only"} {
		policy := ctx.GetResponseHeader(header)
		rewritten := frontend.RewriteCSPForLocalAssets(policy, asset_base_url)
		rewritten = frontend.RewriteCSPForWebSocket(rewritten, websocket_url)
		if rewritten != "" && rewritten != policy {
			ctx.SetResponseHeader(header, rewritten)
		}
	}
}

func build_mp_websocket_url(cfg *OfficialAccountConfig) string {
	if cfg == nil {
		return ""
	}
	protocol := cfg.Protocol
	hostname := cfg.Hostname
	port := cfg.Port
	if cfg.RemoteServerEnabled && strings.TrimSpace(cfg.RemoteServerHostname) != "" {
		protocol = cfg.RemoteServerProtocol
		hostname = cfg.RemoteServerHostname
		port = cfg.RemoteServerPort
	}
	hostname = strings.TrimSpace(hostname)
	if hostname == "0.0.0.0" || hostname == "::" || hostname == "[::]" {
		hostname = "127.0.0.1"
	}
	if hostname == "" {
		return ""
	}
	websocket_protocol := "ws"
	switch strings.ToLower(strings.TrimSuffix(strings.TrimSpace(protocol), ":")) {
	case "https", "wss":
		websocket_protocol = "wss"
	}
	host := hostname
	if port > 0 {
		host = net.JoinHostPort(strings.Trim(hostname, "[]"), strconv.Itoa(port))
	}
	return (&url.URL{
		Scheme: websocket_protocol,
		Host:   host,
		Path:   WebsocketPath,
	}).String()
}
