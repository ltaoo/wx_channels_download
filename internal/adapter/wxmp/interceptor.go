package wxmpadapter

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"wx_channel/frontend"
	"wx_channel/internal/config"
	"wx_channel/internal/interceptor"
	"wx_channel/internal/interceptor/proxy"
	"wx_channel/pkg/scraper/wxmp"
)

var csp_nonce_reg = regexp.MustCompile(`'nonce-([^']+)'`)

// InterceptorPluginConfig owns the official-account scraper configuration used
// by the local interceptor.
type InterceptorPluginConfig struct {
	settings *wxmp.OfficialAccountConfig
	version  string
}

func NewConfig(cfg *config.Config) *InterceptorPluginConfig {
	if cfg == nil {
		return &InterceptorPluginConfig{}
	}
	return &InterceptorPluginConfig{
		settings: new_official_account_config(cfg),
		version:  cfg.Version,
	}
}

// GetPlugins returns the official-account injection plugin.
func (c *InterceptorPluginConfig) GetPlugins() []interface{} {
	if c == nil || c.settings == nil {
		return nil
	}

	return []interface{}{
		create_official_account_interceptor_plugin(c.settings, c.version),
	}
}

func create_official_account_interceptor_plugin(cfg *wxmp.OfficialAccountConfig, version string) *proxy.Plugin {
	asset_base_url := "/__assets"
	url_build := frontend.NewURLBuild(asset_base_url, nil)
	asset_version := version
	if asset_version == "" {
		asset_version = "static"
	}
	version_query := url.Values{"v": []string{asset_version}}
	return &proxy.Plugin{
		Match: "qq.com",
		OnRequest: func(ctx proxy.Context) {
			if ctx.Req().URL.Hostname() != "mp.weixin.qq.com" {
				return
			}
			interceptor.MockFrontendStaticAsset(ctx, ctx.Req().URL.Path, interceptor.FrontendStaticAssetMockOptions{
				PlatformPrefix: static_assets_path + "/",
				PlatformFS:     wxmp.InjectAssets(),
				UserScriptPath: cfg.GlobalScriptPath,
			})
		},
		OnResponse: func(ctx proxy.Context) {
			response_content_type := strings.ToLower(ctx.GetResponseHeader("Content-Type"))
			hostname := ctx.Req().URL.Hostname()
			if hostname != "mp.weixin.qq.com" || !strings.Contains(response_content_type, "text/html") {
				return
			}
			response_body, err := ctx.GetResponseBody()
			if err != nil {
				return
			}
			html_content := string(response_body)
			csp := ctx.GetResponseHeader("Content-Security-Policy") + " " + ctx.GetResponseHeader("Content-Security-Policy-Report-Only")
			mp_websocket_url := build_mp_websocket_url(cfg)
			interceptor.RewriteResponseCSPForLocalAssets(ctx, asset_base_url)
			interceptor.RewriteResponseCSPForWebSocket(ctx, mp_websocket_url)
			variables := wxmp.BuildOfficialAccountVariables(html_content)
			script_attr := ""
			style_attr := ""
			if match := csp_nonce_reg.FindStringSubmatch(csp); len(match) > 1 {
				script_attr = fmt.Sprintf(` nonce="%s" reportloaderror`, match[1])
				style_attr = fmt.Sprintf(` nonce="%s"`, match[1])
			}
			var injected strings.Builder
			if cfg.DebugShowError {
				frontend.AppendScripts(&injected, script_attr, url_build("/inject/error.js", version_query))
			}
			frontend.AppendScripts(&injected, script_attr, url_build("/public/timeless/0.31.4/timeless.umd.min.js", version_query))
			frontend.AppendStylesheets(&injected, style_attr, url_build("/public/timeless/0.31.4/timeless.weui.css", version_query))
			frontend.AppendScripts(&injected, script_attr, url_build("/public/timeless/0.31.4/timeless.weui.umd.min.js", version_query))
			frontend.AppendScripts(&injected, script_attr, url_build("/public/timeless/0.31.4/timeless.dom.umd.min.js", version_query))
			frontend.AppendScripts(&injected, script_attr, url_build("/public/timeless/0.31.4/timeless.web.umd.min.js", version_query))
			frontend.AppendStylesheets(&injected, style_attr, url_build("/inject/components.css"))
			frontend_config := make(map[string]any, len(variables)+2)
			cfg_byte, _ := json.Marshal(cfg)
			_ = json.Unmarshal(cfg_byte, &frontend_config)
			for key, value := range variables {
				frontend_config[key] = value
			}
			frontend_config["version"] = version
			frontend_config["assets_base_url"] = asset_base_url
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
			if cfg.GlobalScriptURL != "" {
				frontend.AppendScripts(&injected, script_attr, cfg.GlobalScriptURL)
			}
			if cfg.InjectContentScript != "" {
				frontend.AppendInlineScript(&injected, script_attr, cfg.InjectContentScript)
			}
			html_content = strings.Replace(html_content, "</body>", injected.String()+"</body>", 1)
			ctx.SetResponseBody(html_content)
		},
	}
}

func build_mp_websocket_url(cfg *wxmp.OfficialAccountConfig) string {
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
