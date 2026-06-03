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

func CreateOfficialAccountInterceptorPlugin(cfg *OfficialAccountConfig, files *frontend.ChannelInjectedFiles) *proxy.Plugin {
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
				variables := buildOfficialAccountVariables(html)
				csp := ctx.GetResponseHeader("Content-Security-Policy-Report-Only")
				script_attr := ""
				if match := cspNonceReg.FindStringSubmatch(csp); len(match) > 1 {
					script_attr = fmt.Sprintf(` nonce="%s" reportloaderror`, match[1])
				}
				inserted_scripts := ""
				if cfg.DebugShowError {
					/** 全局错误捕获并展示弹窗 */
					script_error := fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSError)
					inserted_scripts += script_error
				}
				cfg_byte, _ := json.Marshal(cfg)
				script_config := fmt.Sprintf(`<script%s>var __wx_channels_config__ = %s</script>`, script_attr, string(cfg_byte))
				inserted_scripts += script_config
				variable_byte, _ := json.Marshal(variables)
				script_variable := fmt.Sprintf(`<script%s>var WXVariable = %s;</script>`, script_attr, string(variable_byte))
				inserted_scripts += script_variable
				inserted_scripts += fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSMitt)
				inserted_scripts += fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSTimelessReactive)
				inserted_scripts += fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSTimelessUtils)
				inserted_scripts += fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSTimelessUI)
				inserted_scripts += fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSTimelessKit)
				inserted_scripts += fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSTimelessHeadless)
				inserted_scripts += fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSTimelessIcons)
				inserted_scripts += fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSTimelessProviderWeb)
				inserted_scripts += fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSFloatingUICore)
				inserted_scripts += fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSFloatingUIDOM)
				inserted_scripts += fmt.Sprintf(`<style>%s</style>`, files.CSSWeui)
				inserted_scripts += fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSWeui)
				inserted_scripts += fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSWui)
				inserted_scripts += fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSEventBus)
				inserted_scripts += fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSUtils)
				inserted_scripts += fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSComponents)
				inserted_scripts += fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSDownloader)
				inserted_scripts += fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSWechatOfficialAccount)
				if cfg.PagespyEnabled {
					/** 在线调试 */
					script_pagespy := fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSPageSpy)
					script_pagespy2 := fmt.Sprintf(`<script%s>%s</script>`, script_attr, files.JSDebug)
					inserted_scripts += script_pagespy + script_pagespy2
				}
				html = strings.Replace(html, "</body>", inserted_scripts+"</body>", 1)
				ctx.SetResponseBody(html)
				return
			}
		},
	}
}
