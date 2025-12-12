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
	"github.com/ltaoo/echo"

	"wx_channel/config"
	"wx_channel/pkg/util"
)

var (
	// HTML处理相关正则
	scriptSrcReg  = regexp.MustCompile(`src="([^"]{1,})\.js"`)
	scriptHrefReg = regexp.MustCompile(`href="([^"]{1,})\.js"`)

	// JavaScript处理相关正则
	jsDepReg        = regexp.MustCompile(`"js/([^"]{1,})\.js"`)
	jsFromReg       = regexp.MustCompile(`from {0,1}"([^"]{1,})\.js"`)
	jsLazyImportReg = regexp.MustCompile(`import\("([^"]{1,})\.js"\)`)
	jsImportReg     = regexp.MustCompile(`import {0,1}"([^"]{1,})\.js"`)

	// 特定路径的正则
	jsSourceBufferReg  = regexp.MustCompile(`this.sourceBuffer.appendBuffer\(([a-zA-Z]{1,})\),`)
	jsAutoCutReg       = regexp.MustCompile(`if\(f.cmd===re.MAIN_THREAD_CMD.AUTO_CUT`)
	jsCommentDetailReg = regexp.MustCompile(`async finderGetCommentDetail\((\w+)\)\{(.*?)\}async`)
	jsDialogReg        = regexp.MustCompile(`i.default={dialog`)
	jsLiveInfoReg      = regexp.MustCompile(`async finderGetLiveInfo\((\w+)\)\{(.*?)\}async`)
	jsFeedPageReg      = regexp.MustCompile(`async finderUserPage\((\w+)\)\{(.*?)\}async`)
	jsGoToPrevFlowReg  = regexp.MustCompile(`goToPrevFlowFeed:([a-zA-Z]{1,})`)
	jsGoToNextFlowReg  = regexp.MustCompile(`goToNextFlowFeed:([a-zA-Z]{1,})`)
	jsFmp4IndexReg     = regexp.MustCompile(`fmp4Index:p.fmp4Index`)
	media_profile_js   = `var profile = media.mediaType !== 4 ? {
	type: "picture",
	id: data_object.id,
	title: data_object.objectDesc.description,
	files: data_object.objectDesc.media,
	spec: [],
	contact: data_object.contact
} : {
	type: "media",
	duration: media.spec[0] ? media.spec[0].durationMs : 0,
	spec: media.spec,
	title: data_object.objectDesc.description,
	coverUrl: media.coverUrl,
	url: media.url+media.urlToken,
	size: media.fileSize ? Number(media.fileSize) : 0,
	key: media.decodeKey,
	id: data_object.id,
	nonce_id: data_object.objectNonceId,
	nickname: data_object.nickname,
	createtime: data_object.createtime,
	fileFormat: media.spec.map(o => o.fileFormat),
	contact: data_object.contact
};
(() => {
	if (!window.__wx_channels_store__) {
		return;
	}
	if (window.__wx_channels_store__.profiles.length) {
		var existing = window.__wx_channels_store__.profiles.find(function(v){
			return v.id === profile.id;
		});
		if (existing) {
			return;
		}
	}
	__wx_channels_store__.profile = profile;
	window.__wx_channels_store__.profiles.push(profile);
	setTimeout(() => {
		window.__wx_channels_cur_video = document.querySelector(".feed-video.video-js");
	},800);
	WXE.emit(WXE.Events.FeedProfileLoaded, profile);
	fetch("/__wx_channels_api/profile", {
		method: "POST",
		headers: {
			"Content-Type": "application/json"
		},
		body: JSON.stringify(profile)
	});
})();`
)

func CreateChannelInterceptorPlugin(version string, files *ChannelInjectedFiles, cfg *config.Config) *echo.Plugin {
	v := "?t=" + version
	return &echo.Plugin{
		Match: "qq.com",
		OnRequest: func(ctx *echo.Context) {
			pathname := ctx.Req.URL.Path
			if util.Includes(pathname, "jszip.min") {
				ctx.Mock(200, map[string]string{
					"Content-Type": "application/javascript",
					"__debug":      "local_file",
				}, files.JSZip)
			}
			if util.Includes(pathname, "FileSaver.min") {
				ctx.Mock(200, map[string]string{
					"Content-Type": "application/javascript",
					"__debug":      "local_file",
				}, files.JSFileSaver)
			}
			if util.Includes(pathname, "recorder.min") {
				ctx.Mock(200, map[string]string{
					"Content-Type": "application/javascript",
					"__debug":      "local_file",
				}, files.JSRecorder)
			}
			if pathname == "/__wx_channels_api/profile" {
				var data ChannelMediaProfile
				if err := json.NewDecoder(ctx.Req.Body).Decode(&data); err != nil {
					fmt.Println("[ECHO]handler", err.Error())
				}
				fmt.Printf("\n打开了视频\n%s\n", data.Title)
				ctx.Mock(200, map[string]string{
					"Content-Type": "application/json",
					"__debug":      "fake_resp",
				}, "{}")
			}
			if pathname == "/__wx_channels_api/tip" {
				var data FrontendTip
				if err := json.NewDecoder(ctx.Req.Body).Decode(&data); err != nil {
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
					"__debug":      "fake_resp",
				}, "{}")
			}
			if pathname == "/__wx_channels_api/error" {
				var data FrontendErrorTip
				if err := json.NewDecoder(ctx.Req.Body).Decode(&data); err != nil {
					fmt.Println("[ECHO]handler", err.Error())
				}
				prefix_text := "[FRONTEND ERROR]"
				color.Red(fmt.Sprintf("%v%s\n", prefix_text, data.Msg))
				ctx.Mock(200, map[string]string{
					"Content-Type": "application/json",
					"__debug":      "fake_resp",
				}, "{}")
			}
		},
		OnResponse: func(ctx *echo.Context) {
			resp_content_type := strings.ToLower(ctx.GetResponseHeader("Content-Type"))
			hostname := ctx.Req.URL.Hostname()
			pathname := ctx.Req.URL.Path
			if cfg.ChannelDisableLocationToHome && pathname == "/web/pages/feed" && ctx.Res.StatusCode == 302 {
				u := &url.URL{Scheme: "https", Host: ctx.Req.URL.Hostname(), Path: pathname, RawQuery: ctx.Req.URL.RawQuery}
				q := u.Query()
				q.Set("flow", "2")
				q.Set("fpid", "FinderLike")
				u.RawQuery = q.Encode()
				req, err := http.NewRequest(http.MethodGet, u.String(), nil)
				if err == nil {
					for k, v := range ctx.Req.Header {
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
						ctx.Res.StatusCode = 200
						ctx.Res.Header.Del("Content-Encoding")
						ctx.Res.Header.Del("Content-Length")
						ctx.SetResponseHeader("Content-Type", ct)
						ctx.SetResponseHeader("__debug", "second_fetch")
						ctx.SetResponseBody(string(body2))
						resp_content_type = strings.ToLower(ct)
					}
				}
			}
			if hostname == "channels.weixin.qq.com" && strings.Contains(resp_content_type, "text/html") {
				// fmt.Println(hostname, path)
				resp_body, err := ctx.GetResponseBody()
				if err != nil {
					return
				}
				ctx.SetResponseHeader("__debug", "append_script")
				html := string(resp_body)
				html = scriptSrcReg.ReplaceAllString(html, `src="$1.js`+v+`"`)
				html = scriptHrefReg.ReplaceAllString(html, `href="$1.js`+v+`"`)
				inserted_scripts := ""
				cfg_byte, _ := json.Marshal(cfg)
				script_config := fmt.Sprintf(`<script>var __wx_channels_config__ = %s;</script>`, string(cfg_byte))
				inserted_scripts += script_config
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
				inserted_scripts += fmt.Sprintf(`<script>%s</script>`, files.JSMitt)
				inserted_scripts += fmt.Sprintf(`<script>%s</script>`, files.JSEventBus)
				inserted_scripts += fmt.Sprintf(`<script>%s</script>`, files.JSFloatingUICore)
				inserted_scripts += fmt.Sprintf(`<script>%s</script>`, files.JSFloatingUIDOM)
				inserted_scripts += fmt.Sprintf(`<script>%s</script>`, files.JSWeui)
				inserted_scripts += fmt.Sprintf(`<script>%s</script>`, files.JSComponents)
				inserted_scripts += fmt.Sprintf(`<script>%s</script>`, files.JSUtils)
				if cfg.InjectGlobalScript != "" {
					inserted_scripts += fmt.Sprintf(`<script>%s</script>`, cfg.InjectGlobalScript)
				}
				if pathname == "/web/pages/feed" || pathname == "/web/pages/home" {
					/** 下载逻辑 */
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
				ctx.SetResponseHeader("__debug", "replace_script")
				js_script := string(resp_body)
				js_script = jsFromReg.ReplaceAllString(js_script, `from"$1.js`+v+`"`)
				js_script = jsDepReg.ReplaceAllString(js_script, `"js/$1.js`+v+`"`)
				js_script = jsLazyImportReg.ReplaceAllString(js_script, `import("$1.js`+v+`")`)
				js_script = jsImportReg.ReplaceAllString(js_script, `import"$1.js`+v+`"`)

				if strings.Contains(pathname, "/t/wx_fed/finder/web/web-finder/res/js/index.publish") {
					replace_str1 := `(() => {
									if (window.__wx_channels_store__) {
									window.__wx_channels_store__.buffers.push($1);
									}
									})(),this.sourceBuffer.appendBuffer($1),`
					if jsSourceBufferReg.MatchString(js_script) {
						fmt.Println("2. 视频播放 js 修改成功")
					}
					js_script = jsSourceBufferReg.ReplaceAllString(js_script, replace_str1)
					replace_str2 := `if(f.cmd==="CUT"){
										if (window.__wx_channels_store__ && __wx_channels_store__.profile) {
										console.log("CUT", f, __wx_channels_store__.profile.key);
										window.__wx_channels_store__.keys[__wx_channels_store__.profile.key]=f.decryptor_array;
										}
									}
									if(f.cmd===re.MAIN_THREAD_CMD.AUTO_CUT`
					js_script = jsAutoCutReg.ReplaceAllString(js_script, replace_str2)
					ctx.SetResponseBody(js_script)
					return
				}
				if util.Includes(pathname, "/t/wx_fed/finder/web/web-finder/res/js/virtual_svg-icons-register") {
					replace_str1 := fmt.Sprintf(`async finderGetCommentDetail($1) {
					var feedResult = await (async () => {
						$2;
					})();
					var data_object = feedResult.data.object;
					if (!data_object.objectDesc) {
						return feedResult;
					}
					var media = data_object.objectDesc.media[0];
					%v
					return feedResult;
				}async`, media_profile_js)
					if jsCommentDetailReg.MatchString(js_script) {
						fmt.Println("3.视频读取 js 修改成功")
					}
					js_script = jsCommentDetailReg.ReplaceAllString(js_script, replace_str1)
					if jsFeedPageReg.MatchString(js_script) {
						fmt.Println("5.首页 API js 修改成功")
					}
					replace_str4 := fmt.Sprintf(`async finderUserPage($1) {
					var feedResult = await (async () => {
						$2;
					})();
					var feeds = feedResult.data.object;
					if (!feeds || feeds.length === 0) {
						return feedResult;
					}
					var data_object = feeds[0];
					var media = data_object.objectDesc.media[0];
					if (!home_mounted) {
					%v
					}
					return feedResult;
				}async`, media_profile_js)
					js_script = jsFeedPageReg.ReplaceAllString(js_script, replace_str4)

					replace_str2 := `i.default=window.__wx_channels_tip__={dialog`
					js_script = jsDialogReg.ReplaceAllString(js_script, replace_str2)
					replace_str3 := `async finderGetLiveInfo($1) {
					var feedResult = await (async () => {
						$2;
					})();
					var profile = {
						title: feedResult.data.description || "直播",
						url: feedResult.data.liveInfo.streamUrl,
					};
					if (window.__wx_channels_live_store__) {
						__wx_channels_live_store__.profile = profile;
					}
					WXE.emit(WXE.Events.LiveProfileLoaded, profile);
					fetch("/__wx_channels_api/profile", {
						method: "POST",
						headers: {
							"Content-Type": "application/json"
						},
						body: JSON.stringify(profile)
					});
					return feedResult;
				}async`
					if pathname == "/web/pages/live" && jsLiveInfoReg.MatchString(js_script) {
						fmt.Println("4.直播 API js 修改成功")
					}
					js_script = jsLiveInfoReg.ReplaceAllString(js_script, replace_str3)
					ctx.SetResponseBody(js_script)
					return
				}
				if util.Includes(pathname, "connect.publish") {
					replace_str1 := fmt.Sprintf(`goToNextFlowFeed:async function(v){
									await $1(v);
									setTimeout(() => {
									// 找到 flowTab 对应的值
									console.log('goToNextFlowFeed', yt);
									if (!yt || !yt.value.feeds) {
										return;
									}
									var data_object = yt.value.feeds[yt.value.currentFeedIndex];
									console.log("handle goto next feed", yt, data_object);
									var media = data_object.objectDesc.media[0];
									window.__wx_channels_cur_video = document.querySelector(".feed-video.video-js");
									%v
									if (window.__insert_download_btn_to_home_page) {
				__insert_download_btn_to_home_page();
									}
									}, 0);
									}`, media_profile_js)
					js_script = jsGoToNextFlowReg.ReplaceAllString(js_script, replace_str1)
					replace_str2 := fmt.Sprintf(`goToPrevFlowFeed:async function(v){
									await $1(v);
									setTimeout(() => {
									console.log('goToPrevFlowFeed', yt);
									if (!yt || !yt.value.feeds) {
										return;
									}
									var data_object = yt.value.feeds[yt.value.currentFeedIndex];
									console.log("handle goto prev feed", yt, data_object);
									var media = data_object.objectDesc.media[0];
									window.__wx_channels_cur_video = document.querySelector(".feed-video.video-js");
									%v
									if (window.__insert_download_btn_to_home_page) {
				__insert_download_btn_to_home_page();
									}
									}, 0);
									}`, media_profile_js)
					js_script = jsGoToPrevFlowReg.ReplaceAllString(js_script, replace_str2)
					ctx.SetResponseBody(js_script)
					return
				}
				if util.Includes(pathname, "worker_release") {
					replace_str := `decryptor_array:p.decryptor_array,fmp4Index:p.fmp4Index`
					js_script = jsFmp4IndexReg.ReplaceAllString(js_script, replace_str)
					ctx.SetResponseBody(js_script)
					return
				}
				ctx.SetResponseBody(js_script)
			}
		},
	}
}
