package officialaccount

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"wx_channel/internal/interceptor"
	"wx_channel/internal/interceptor/proxy"
)

var cspNonceReg = regexp.MustCompile(`'nonce-([^']+)'`)

func CreateOfficialAccountInterceptorPlugin(cfg *OfficialAccountConfig, files *interceptor.ChannelInjectedFiles, version string) *proxy.Plugin {
	return &proxy.Plugin{
		Match: "qq.com",
		OnResponse: func(ctx proxy.Context) {
			resp_content_type := strings.ToLower(ctx.GetResponseHeader("Content-Type"))
			hostname := ctx.Req().URL.Hostname()
			// pathname := ctx.Req().URL.Path
			if !cfg.Disabled && hostname == "mp.weixin.qq.com" && strings.Contains(resp_content_type, "text/html") {
				resp_body, err := ctx.GetResponseBody()
				if err != nil {
					return
				}
				variables := map[string]interface{}{}
				html := string(resp_body)
				csp := ctx.GetResponseHeader("Content-Security-Policy-Report-Only")
				script_attr := ""
				style_attr := ""
				if match := cspNonceReg.FindStringSubmatch(csp); len(match) > 1 {
					script_attr = fmt.Sprintf(` nonce="%s" reportloaderror`, match[1])
					style_attr = fmt.Sprintf(` nonce="%s"`, match[1])
				}
				var injected strings.Builder
				if cfg.DebugShowError {
					/** 全局错误捕获并展示弹窗 */
					interceptor.AppendScriptSrcs(&injected, script_attr, interceptor.ChannelSrcAssetURL("error.js"))
				}
				cfg_byte, _ := json.Marshal(cfg)
				interceptor.AppendInlineScript(&injected, script_attr, fmt.Sprintf(`var __wx_channels_config__ = %s; var __wx_channels_version__ = "%s";`, string(cfg_byte), version))
				variable_byte, _ := json.Marshal(variables)
				interceptor.AppendInlineScript(&injected, script_attr, fmt.Sprintf(`var WXVariable = %s;`, string(variable_byte)))
				interceptor.AppendSharedLibAssets(&injected, version, script_attr, style_attr)
				interceptor.AppendScriptSrcs(
					&injected,
					script_attr,
					interceptor.ChannelSrcAssetURL("eventbus.js"),
					interceptor.ChannelSrcAssetURL("utils.js"),
					interceptor.ChannelSrcAssetURL("components.js"),
					interceptor.ChannelSrcAssetURL("downloaderv2.js"),
					interceptor.ChannelSrcAssetURL("officialaccount.js"),
				)
				if cfg.PagespyEnabled {
					/** 在线调试 */
					interceptor.AppendScriptSrcs(&injected, script_attr, interceptor.ChannelLibAssetURL(version, "pagespy.min.js"), interceptor.ChannelSrcAssetURL("pagespy.js"))
				}
				html = strings.Replace(html, "</body>", injected.String()+"</body>", 1)
				ctx.SetResponseBody(html)
				return
			}
		},
	}
}
