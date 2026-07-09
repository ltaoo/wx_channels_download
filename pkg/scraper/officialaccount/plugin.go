package officialaccount

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"wx_channel/frontend"
	interceptorpkg "wx_channel/internal/interceptor"
	"wx_channel/internal/interceptor/proxy"
)

var cspNonceReg = regexp.MustCompile(`'nonce-([^']+)'`)

func CreateOfficialAccountArticleLoadedPlugin(onArticleLoaded func(profile *interceptorpkg.OfficialAccountArticleProfile)) *proxy.Plugin {
	return &proxy.Plugin{
		Match: "mp.weixin.qq.com",
		OnRequest: func(ctx proxy.Context) {
			if ctx.Req().URL.Path != "/__wx_channels_api/officialaccount/article" {
				return
			}
			body, err := io.ReadAll(ctx.Req().Body)
			if err != nil {
				fmt.Println("[ECHO]handler", err.Error())
			}
			profile, err := interceptorpkg.NewOfficialAccountArticleProfile(json.RawMessage(body))
			if err != nil {
				fmt.Println("[ECHO]handler", err.Error())
			}
			if profile != nil && onArticleLoaded != nil {
				go onArticleLoaded(profile)
			}
			if profile != nil {
				fmt.Printf("\n打开了公众号文章\n%s\n", profile.Title)
			}
			ctx.Mock(200, map[string]string{
				"Content-Type": "application/json",
			}, "{}")
		},
	}
}

func CreateOfficialAccountInterceptorPlugin(cfg *OfficialAccountConfig, files *frontend.ChannelInjectedFiles, version string) *proxy.Plugin {
	assetBaseURL := frontend.ChannelAssetsSameOriginBaseURL()
	return &proxy.Plugin{
		Match: "qq.com",
		OnRequest: func(ctx proxy.Context) {
			if ctx.Req().URL.Hostname() == "mp.weixin.qq.com" && frontend.MockChannelStaticAsset(ctx, ctx.Req().URL.Path, files) {
				return
			}
		},
		OnResponse: func(ctx proxy.Context) {
			resp_content_type := strings.ToLower(ctx.GetResponseHeader("Content-Type"))
			hostname := ctx.Req().URL.Hostname()
			// pathname := ctx.Req().URL.Path
			if cfg.Enabled && hostname == "mp.weixin.qq.com" && strings.Contains(resp_content_type, "text/html") {
				resp_body, err := ctx.GetResponseBody()
				if err != nil {
					return
				}
				html := string(resp_body)
				csp := ctx.GetResponseHeader("Content-Security-Policy") + " " + ctx.GetResponseHeader("Content-Security-Policy-Report-Only")
				variables := buildOfficialAccountVariables(html)
				script_attr := ""
				style_attr := ""
				if match := cspNonceReg.FindStringSubmatch(csp); len(match) > 1 {
					script_attr = fmt.Sprintf(` nonce="%s" reportloaderror`, match[1])
					style_attr = fmt.Sprintf(` nonce="%s"`, match[1])
				}
				var injected strings.Builder
				if cfg.DebugShowError {
					/** 全局错误捕获并展示弹窗 */
					frontend.AppendScriptSrcs(&injected, script_attr, frontend.ChannelInjectAssetURL(assetBaseURL, "error.js"))
				}
				var shadcnCSS []byte
				if files != nil {
					shadcnCSS = files.CSSTimelessShadcn
				}
				frontend.AppendSharedLibAssetsWithInlineShadcnCSS(&injected, assetBaseURL, version, script_attr, style_attr, shadcnCSS)
				frontend.AppendStylesheetHrefs(&injected, style_attr, frontend.ChannelInjectAssetURL(assetBaseURL, "components.css"))
				cfg_byte, _ := json.Marshal(cfg)
				frontend.AppendInlineScript(&injected, script_attr, fmt.Sprintf(`var __wx_channels_config__ = %s; var __wx_channels_version__ = "%s";`, string(cfg_byte), version))
				frontend.AppendInlineScript(&injected, script_attr, fmt.Sprintf(`window.__wx_channels_env__ = Object.assign(window.__wx_channels_env__ || {}, { assetsBaseURL: %q });`, assetBaseURL))
				variable_byte, _ := json.Marshal(variables)
				frontend.AppendInlineScript(&injected, script_attr, fmt.Sprintf(`var WXVariable = %s;`, string(variable_byte)))
				frontend.AppendScriptSrcs(
					&injected,
					script_attr,
					frontend.ChannelInjectAssetURL(assetBaseURL, "eventbus.js"),
					frontend.ChannelInjectAssetURL(assetBaseURL, "env.js"),
					frontend.ChannelInjectAssetURL(assetBaseURL, "utils.js"),
					frontend.ChannelInjectAssetURL(assetBaseURL, "components.js"),
					frontend.ChannelInjectAssetURL(assetBaseURL, "virtual-list-view.js"),
					frontend.ChannelInjectAssetURL(assetBaseURL, "download/core.js"),
					frontend.ChannelInjectAssetURL(assetBaseURL, "officialaccount.js"),
				)
				if cfg.PagespyEnabled {
					/** 在线调试 */
					frontend.AppendScriptSrcs(&injected, script_attr, frontend.ChannelLibAssetURL(assetBaseURL, version, "pagespy.min.js"), frontend.ChannelInjectAssetURL(assetBaseURL, "pagespy.js"))
				}
				html = strings.Replace(html, "</body>", injected.String()+"</body>", 1)
				ctx.SetResponseBody(html)
				return
			}
		},
	}
}
