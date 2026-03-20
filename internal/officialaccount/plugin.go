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

func CreateOfficialAccountInterceptorPlugin(cfg *OfficialAccountConfig) *proxy.Plugin {
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
				html := string(resp_body)
				csp := ctx.GetResponseHeader("Content-Security-Policy-Report-Only")
				script_attr := ""
				if match := cspNonceReg.FindStringSubmatch(csp); len(match) > 1 {
					script_attr = fmt.Sprintf(` nonce="%s" reportloaderror`, match[1])
				}
				assets_base := fmt.Sprintf("%s://%s/__wx_channels_assets", cfg.Protocol, cfg.Addr)
				d := interceptor.NewAssetDirWithAttr(assets_base, "", script_attr)
				inserted_scripts := ""
				cfg_byte, _ := json.Marshal(cfg)
				inserted_scripts += d.Inline(fmt.Sprintf("var __wx_channels_config__ = %s", string(cfg_byte)), false)
				inserted_scripts += d.Tag("lib/mitt.umd.js")
				inserted_scripts += d.Tag("src/eventbus.js")
				inserted_scripts += d.Tag("src/utils.js")
				inserted_scripts += d.Inline(string(interceptor.CSSWeui), true)
				inserted_scripts += d.Tag("lib/weui.min.js")
				inserted_scripts += d.Tag("lib/floating-ui.core.1.7.4.min.js")
				inserted_scripts += d.Tag("lib/floating-ui.dom.1.7.4.min.js")
				inserted_scripts += d.Tag("lib/wui.umd.js")
				inserted_scripts += d.Tag("src/components.js")
				inserted_scripts += d.Tag("src/officialaccount.js")
				if cfg.DebugShowError {
					/** 全局错误捕获并展示弹窗 */
					inserted_scripts += d.Tag("src/error.js")
				}
				if cfg.PagespyEnabled {
					/** 在线调试 */
					inserted_scripts += d.Tag("lib/pagespy.min.js")
					inserted_scripts += d.Tag("src/pagespy.js")
				}
				html = strings.Replace(html, "</body>", inserted_scripts+"</body>", 1)
				ctx.SetResponseBody(html)
				return
			}
		},
	}
}
