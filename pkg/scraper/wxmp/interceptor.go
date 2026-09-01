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

var csp_nonce_reg = regexp.MustCompile(`'nonce-([^']+)'`)

// InterceptorConfig contains the application values needed by the
// official-account injection rule.
type InterceptorConfig struct {
	Version  string
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
			if settings.DebugShowError {
				frontend.AppendScripts(&injected, script_attr, url_build("/inject/error.js", version_query))
			}
			frontend.AppendScripts(&injected, script_attr, url_build("/public/timeless/0.33.0/timeless.umd.min.js", version_query))
			frontend.AppendStylesheets(&injected, style_attr, url_build("/public/timeless/0.33.0/timeless.weui.css", version_query))
			frontend.AppendScripts(&injected, script_attr, url_build("/public/timeless/0.33.0/timeless.weui.umd.min.js", version_query))
			frontend.AppendScripts(&injected, script_attr, url_build("/public/timeless/0.33.0/timeless.dom.umd.min.js", version_query))
			frontend.AppendScripts(&injected, script_attr, url_build("/public/timeless/0.33.0/timeless.web.umd.min.js", version_query))
			frontend.AppendStylesheets(&injected, style_attr, url_build("/inject/components.css"))
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
			frontend.AppendScripts(
				&injected,
				script_attr,
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
				asset_url(asset_base_url, "/inject/mp.ws.js", version_query),
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
	return []*echo.Plugin{plugin}
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
