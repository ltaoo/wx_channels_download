package interceptor

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"

	"wx_channel/internal/interceptor/proxy"
	"wx_channel/pkg/util"
)

var (
	// HTML处理相关正则
	scriptSrcReg  = regexp.MustCompile(`src="([^"]{1,})\.js"`)
	scriptHrefReg = regexp.MustCompile(`href="([^"]{1,})\.js"`)

	// JavaScript处理相关正则
	jsDepReg         = regexp.MustCompile(`"js/([^"]{1,})\.js"`)
	jsFromReg        = regexp.MustCompile(`from {0,1}"([^"]{1,})\.js"`)
	jsLazyImportReg  = regexp.MustCompile(`import\("([^"]{1,})\.js"\)`)
	jsImportReg      = regexp.MustCompile(`import {0,1}"([^"]{1,})\.js"`)
	jsExportReg      = regexp.MustCompile(`exports?\s*\{`)
	jsExportBlockReg = regexp.MustCompile(`exports?\s*\{([^}]*)\}`)

	// 特定路径的正则
	jsSourceBufferReg   = regexp.MustCompile(`this.sourceBuffer.appendBuffer\(([a-zA-Z]{1,})\),`)
	jsInitReg           = regexp.MustCompile(`async finderInit\(\)\{(.*?)\}async`)
	jsFeedProfileReg    = regexp.MustCompile(`async finderGetCommentDetail\((\w+)\)\{(.*?)\}async`)
	jsPCFlowReg         = regexp.MustCompile(`async finderPcFlow\((\w+)\)\{(.*?)\}async`)
	jsLiveInfoReg       = regexp.MustCompile(`async finderGetLiveInfo\((\w+)\)\{(.*?)\}async`)
	jsRecommendFeedsReg = regexp.MustCompile(`async finderGetRecommend\((\w+)\)\{(.*?)\}async`)
	jsUserFeedsReg      = regexp.MustCompile(`async finderUserPage\((\w+)\)\{(.*?)\}async`)
	jsFinderPCSearchReg = regexp.MustCompile(`async finderPCSearch\((\w+)\)\{(.*?)\}async`)
	jsFinderSearchReg   = regexp.MustCompile(`async finderSearch\((\w+)\)\{(.*?)\}async`)
	jsGoToPrevFlowReg   = regexp.MustCompile(`goToPrevFlowFeed:([a-zA-Z]{1,})`)
	jsGoToNextFlowReg   = regexp.MustCompile(`goToNextFlowFeed:([a-zA-Z]{1,})`)
)

func CreateChannelInterceptorPlugin(interceptor *Interceptor, files *ChannelInjectedFiles) *proxy.Plugin {
	version := interceptor.Version
	cfg := interceptor.Settings
	variables := interceptor.FrontendVariables
	v := "?t=" + version
	return &proxy.Plugin{
		Match: "qq.com",
		OnRequest: func(ctx proxy.Context) {
			pathname := ctx.Req().URL.Path
			if util.Includes(pathname, "jszip.min") {
				ctx.Mock(200, map[string]string{
					"Content-Type": "application/javascript",
				}, string(files.JSZip))
			}
			if util.Includes(pathname, "FileSaver.min") {
				ctx.Mock(200, map[string]string{
					"Content-Type": "application/javascript",
				}, string(files.JSFileSaver))
			}
			if util.Includes(pathname, "recorder.min") {
				ctx.Mock(200, map[string]string{
					"Content-Type": "application/javascript",
				}, string(files.JSRecorder))
			}
			if pathname == "/__wx_channels_api/profile" {
				var data ChannelMediaProfile
				if err := json.NewDecoder(ctx.Req().Body).Decode(&data); err != nil {
					fmt.Println("[ECHO]handler", err.Error())
				}
				fmt.Printf("\n打开了视频\n%s\n", data.Title)
				ctx.Mock(200, map[string]string{
					"Content-Type": "application/json",
				}, "{}")
			}
			if pathname == "/__wx_channels_api/tip" {
				var data FrontendTip
				if err := json.NewDecoder(ctx.Req().Body).Decode(&data); err != nil {
					fmt.Println("[ECHO]handler", err.Error())
				}
				prefix_text := "[FRONTEND]"
				prefix := data.Prefix
				if prefix == nil {
					prefix = &prefix_text
				}
				if data.End == 1 {
					fmt.Println()
				} else if data.Replace == 1 {
					fmt.Printf("\r\033[K%v%s", *prefix, data.Msg)
				} else if data.IgnorePrefix == 1 {
					fmt.Printf("%s\n", data.Msg)
				} else {
					fmt.Printf("%v%s\n", *prefix, data.Msg)
				}
				ctx.Mock(200, map[string]string{
					"Content-Type": "application/json",
				}, "{}")
			}
			if pathname == "/__wx_channels_api/error" {
				var data FrontendErrorTip
				if err := json.NewDecoder(ctx.Req().Body).Decode(&data); err != nil {
					fmt.Println("[ECHO]handler", err.Error())
				}
				prefix_text := "[FRONTEND ERROR]"
				color.Red(fmt.Sprintf("%v%s\n", prefix_text, data.Msg))
				ctx.Mock(200, map[string]string{
					"Content-Type": "application/json",
				}, "{}")
			}
		},
		OnResponse: func(ctx proxy.Context) {
			resp_content_type := strings.ToLower(ctx.GetResponseHeader("Content-Type"))
			hostname := ctx.Req().URL.Hostname()
			pathname := ctx.Req().URL.Path
			// fmt.Println("response", hostname, pathname, resp_content_type)
			if cfg.ChannelDisableLocationToHome && pathname == "/web/pages/feed" && ctx.Res().StatusCode == 302 {
				u := &url.URL{Scheme: "https", Host: ctx.Req().URL.Hostname(), Path: pathname, RawQuery: ctx.Req().URL.RawQuery}
				q := u.Query()
				q.Set("flow", "2")
				q.Set("fpid", "FinderLike")
				u.RawQuery = q.Encode()
				req, err := http.NewRequest(http.MethodGet, u.String(), nil)
				if err == nil {
					for k, v := range ctx.Req().Header {
						for _, vv := range v {
							req.Header.Add(k, vv)
						}
					}
					req.Header.Del("Accept-Encoding")
					client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 10 * time.Second}
					if resp2, err2 := client.Do(req); err2 == nil {
						defer resp2.Body.Close()
						body2, _ := io.ReadAll(resp2.Body)
						ct := resp2.Header.Get("Content-Type")
						lct := strings.ToLower(ct)
						if ct == "" || strings.Contains(lct, "text/html") {
							ct = "text/html; charset=utf-8"
						}
						ctx.Res().StatusCode = 200
						ctx.Res().Header.Del("Content-Encoding")
						ctx.Res().Header.Del("Content-Length")
						ctx.SetResponseHeader("Content-Type", ct)
						ctx.SetResponseBody(string(body2))
						resp_content_type = strings.ToLower(ct)
					}
				}
			}
			if hostname == "channels.weixin.qq.com" && strings.Contains(resp_content_type, "text/html") {
				resp_body, err := ctx.GetResponseBody()
				if err != nil {
					return
				}
				html := string(resp_body)
				html = scriptSrcReg.ReplaceAllString(html, `src="$1.js`+v+`"`)
				html = scriptHrefReg.ReplaceAllString(html, `href="$1.js`+v+`"`)
				inserted_scripts := ""
				cfg_byte, _ := json.Marshal(cfg)
				script_config := fmt.Sprintf(`<script>var __wx_channels_config__ = %s; var __wx_channels_version__ = "%s";</script>`, string(cfg_byte), version)
				inserted_scripts += script_config
				variable_byte, _ := json.Marshal(variables)
				script_variable := fmt.Sprintf(`<script>var WXVariable = %s;</script>`, string(variable_byte))
				inserted_scripts += script_variable
				inserted_scripts += fmt.Sprintf(`<script>%s</script>`, files.JSMitt)
				inserted_scripts += fmt.Sprintf(`<script>%s</script>`, files.JSEventBus)
				inserted_scripts += fmt.Sprintf(`<script>%s</script>`, files.JSUtils)
				if cfg.DebugShowError {
					/** 全局错误捕获并展示弹窗 */
					script_error := fmt.Sprintf(`<script>%s</script>`, files.JSError)
					inserted_scripts += script_error
				}
				if cfg.PagespyEnabled {
					/** 在线调试 */
					script_pagespy := fmt.Sprintf(`<script>%s</script>`, files.JSPageSpy)
					script_pagespy2 := fmt.Sprintf(`<script>%s</script>`, files.JSDebug)
					inserted_scripts += script_pagespy + script_pagespy2
				}
				inserted_scripts += fmt.Sprintf(`<script>%s</script>`, files.JSFloatingUICore)
				inserted_scripts += fmt.Sprintf(`<script>%s</script>`, files.JSFloatingUIDOM)
				inserted_scripts += fmt.Sprintf(`<style>%s</style>`, files.CSSWeui)
				inserted_scripts += fmt.Sprintf(`<script>%s</script>`, files.JSWeui)
				inserted_scripts += fmt.Sprintf(`<script>%s</script>`, files.JSWui)
				inserted_scripts += fmt.Sprintf(`<script>%s</script>`, files.JSComponents)
				inserted_scripts += fmt.Sprintf(`<script>%s</script>`, files.JSDownloader)
				if cfg.InjectGlobalScript != "" {
					inserted_scripts += fmt.Sprintf(`<script>%s</script>`, cfg.InjectGlobalScript)
				}
				if pathname == "/web/pages/feed" || pathname == "/web/pages/home" {
					/** 核心逻辑 */
					script_main := fmt.Sprintf(`<script>%s</script>`, files.JSMain)
					if cfg.InjectExtraScriptAfterJSMain != "" {
						script_main += fmt.Sprintf(`<script>%s</script>`, cfg.InjectExtraScriptAfterJSMain)
					}
					inserted_scripts += script_main
				}
				if pathname == "/web/pages/live" {
					script_live_main := fmt.Sprintf(`<script>%s</script>`, files.JSLiveMain)
					if cfg.InjectExtraScriptAfterJSMain != "" {
						script_live_main += fmt.Sprintf(`<script>%s</script>`, cfg.InjectExtraScriptAfterJSMain)
					}
					inserted_scripts += script_live_main
				}
				if pathname == "/web/pages/profile" {
					script_contact_main := fmt.Sprintf(`<script>%s</script>`, files.JSContactMain)
					if cfg.InjectExtraScriptAfterJSMain != "" {
						script_contact_main += fmt.Sprintf(`<script>%s</script>`, cfg.InjectExtraScriptAfterJSMain)
					}
					inserted_scripts += script_contact_main
				}
				html = strings.Replace(html, "<head>", "<head>\n"+inserted_scripts, 1)
				ctx.SetResponseBody(html)
				return
			}
			if hostname == "res.wx.qq.com" && strings.Contains(resp_content_type, "application/javascript") {
				if util.Includes(pathname, "wasm_video_decode") {
					return
				}
				resp_body, err := ctx.GetResponseBody()
				if err != nil {
					return
				}
				js_script := string(resp_body)
				js_script = jsFromReg.ReplaceAllString(js_script, `from"$1.js`+v+`"`)
				js_script = jsDepReg.ReplaceAllString(js_script, `"js/$1.js`+v+`"`)
				js_script = jsLazyImportReg.ReplaceAllString(js_script, `import("$1.js`+v+`")`)
				js_script = jsImportReg.ReplaceAllString(js_script, `import"$1.js`+v+`"`)

				if strings.Contains(pathname, "/t/wx_fed/finder/web/web-finder/res/js/index.publish") {
					// 已经废弃了
					buffer_js := `(() => {
					WXU.append_media_buf($1);
					})(),this.sourceBuffer.appendBuffer($1),`
					js_script = jsSourceBufferReg.ReplaceAllString(js_script, buffer_js)
					ctx.SetResponseBody(js_script)
					return
				}
				if util.Includes(pathname, "/t/wx_fed/finder/web/web-finder/res/js/virtual_svg-icons-register") {
					{
						js_init := `async finderInit() {
					var result = await (async () => {
						$1;
					})();
					var data = result.data;
					console.log("before Init", data);
					WXU.emit(WXU.Events.Init, data);
					return result;
				}async`
						js_script = jsInitReg.ReplaceAllString(js_script, js_init)
					}
					{
						js_pc_flow := `async finderPcFlow($1) {
					var result = await (async () => {
						$2;
					})();
					var feeds = result.data.object;
					console.log("before PCFlowLoaded", result.data);
					WXU.emit(WXU.Events.PCFlowLoaded, feeds);
					return result;
				}async`
						js_script = jsPCFlowReg.ReplaceAllString(js_script, js_pc_flow)
					}
					{
						js_recommend_feeds := `async finderGetRecommend($1) {
					var result = await (async () => {
						$2;
					})();
					var feeds = result.data.object;
					// console.log("before RecommendFeedsLoaded", result.data);
					WXU.emit(WXU.Events.RecommendFeedsLoaded, feeds);
					return result;
				}async`
						js_script = jsRecommendFeedsReg.ReplaceAllString(js_script, js_recommend_feeds)
					}
					{
						js_feed_profile := `async finderGetCommentDetail($1) {
					var result = await (async () => {
						$2;
					})();
					var feed = result.data.object;
					console.log("before FeedProfileLoaded", result.data);
					WXU.emit(WXU.Events.FeedProfileLoaded, feed);
					return result;
				}async`
						js_script = jsFeedProfileReg.ReplaceAllString(js_script, js_feed_profile)
					}
					{
						js_finder_pc_search := `async finderPCSearch($1) {
					var result = await (async () => {
						$2;
					})();
					console.log("before finderPCSearch", result, $1);
					return result;
				}async`
						js_script = jsFinderPCSearchReg.ReplaceAllString(js_script, js_finder_pc_search)
					}
					{
						js_finder_search := `async finderSearch($1) {
					var result = await (async () => {
						$2;
					})();
					console.log("before finderSearch", result, $1);
					return result;
				}async`
						js_script = jsFinderSearchReg.ReplaceAllString(js_script, js_finder_search)
					}
					{
						js_user_feed := `async finderUserPage($1) {
					var result = await (async () => {
						$2;
					})();
					var feeds = result.data.object;
					console.log("before UserFeedsLoaded", result.data, $1);
					WXU.emit(WXU.Events.UserFeedsLoaded, feeds);
					return result;
				}async`
						js_script = jsUserFeedsReg.ReplaceAllString(js_script, js_user_feed)
					}
					{
						js_live_profile := `async finderGetLiveInfo($1) {
					var result = await (async () => {
						$2;
					})();
					var live = result.data;
					console.log("before LiveProfileLoaded", result.data);
					WXU.emit(WXU.Events.LiveProfileLoaded, live);
					return result;
				}async`
						js_script = jsLiveInfoReg.ReplaceAllString(js_script, js_live_profile)
					}
					{

						api_methods := "{}"
						if m := jsExportBlockReg.FindStringSubmatch(js_script); len(m) >= 2 {
							items := strings.Split(m[1], ",")
							locals := make([]string, 0, len(items))
							for _, it := range items {
								p := strings.TrimSpace(it)
								if p == "" {
									continue
								}
								idx := strings.Index(p, " as ")
								local := p
								if idx != -1 {
									local = strings.TrimSpace(p[:idx])
								}
								if local != "" && local != " " {
									locals = append(locals, local)
								}
							}
							if len(locals) > 0 {
								api_methods = "{" + strings.Join(locals, ",") + "}"
							}
						}
						api_methods_escaped := strings.ReplaceAll(api_methods, "$", "$$")
						js_wxapi := ";WXU.emit(WXU.Events.APILoaded," + api_methods_escaped + ");export{"
						js_script = jsExportReg.ReplaceAllString(js_script, js_wxapi)
					}
					ctx.SetResponseBody(js_script)
					return
				}
				if util.Includes(pathname, "connect.publish") {
					{
						js_go_next_feed := `goToNextFlowFeed:async function(v){
						await $1(v);
						// yt 来自 flowTab 对应的值
						console.log('goToNextFlowFeed', yt);
						if (!yt || !yt.value.feeds) {
							return;
						}
						var feed = yt.value.feeds[yt.value.currentFeedIndex];
						console.log("before GotoNextFeed", yt, feed);
						WXU.emit(WXU.Events.GotoNextFeed, feed);
					}`
						js_script = jsGoToNextFlowReg.ReplaceAllString(js_script, js_go_next_feed)
					}
					{
						js_go_prev_feed := `goToPrevFlowFeed:async function(v){
						await $1(v);
						console.log('goToPrevFlowFeed', yt);
						if (!yt || !yt.value.feeds) {
							return;
						}
						var feed = yt.value.feeds[yt.value.currentFeedIndex];
						console.log("before GotoPrevFeed", yt, feed);
						WXU.emit(WXU.Events.GotoPrevFeed, feed);
					}`
						js_script = jsGoToPrevFlowReg.ReplaceAllString(js_script, js_go_prev_feed)
					}
					{
						js_wxutil := ";WXU.emit(WXU.Events.UtilsLoaded,{decodeBase64ToUint64String:decodeBase64ToUint64String});export{"
						js_script = jsExportReg.ReplaceAllString(js_script, js_wxutil)
					}
					ctx.SetResponseBody(js_script)
					return
				}
				ctx.SetResponseBody(js_script)
			}
		},
	}
}
