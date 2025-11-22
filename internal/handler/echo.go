package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"wx_channel/config"
	"wx_channel/internal/application"
	"wx_channel/pkg/echo"
	"wx_channel/pkg/util"
)

func HandleHttpRequestEcho(biz *application.Biz, cfg *config.Config) func(conn *echo.EchoConn) {
	return func(conn *echo.EchoConn) {
		parsed_url, err := conn.URL()
		if err != nil {
			fmt.Printf("URL解析失败: %v\n", err)
			return
		}
		hostname := parsed_url.Hostname()
		path := parsed_url.Path
		v := "?t=" + biz.Version

		if conn.IsBeforeRequest() {
			// @todo js_main 等资源也可以通过这种方式加载，而不是内嵌到 html
			if util.Includes(path, "jszip") {
				headers := http.Header{}
				headers.Set("Content-Type", "application/javascript")
				headers.Set("__debug", "local_file")
				conn.ResponseWithoutRequest(200, biz.Files.JSZip, headers)
				return
			}
			if util.Includes(path, "FileSaver.min") {
				headers := http.Header{}
				headers.Set("Content-Type", "application/javascript")
				headers.Set("__debug", "local_file")
				conn.ResponseWithoutRequest(200, biz.Files.JSFileSaver, headers)
				return
			}
			if path == "/__wx_channels_api/profile" {
				var data ChannelMediaProfile
				request_body := conn.GetRequestBody()
				err := json.Unmarshal(request_body, &data)
				if err != nil {
					fmt.Println("[ECHO]handler", err.Error())
				}
				fmt.Printf("\n打开了视频\n%s\n", data.Title)
				headers := http.Header{}
				headers.Set("Content-Type", "application/json")
				headers.Set("__debug", "fake_resp")
				conn.ResponseWithoutRequest(200, []byte("{}"), headers)
				return
			}
			if path == "/__wx_channels_api/tip" {
				var data FrontendTip
				request_body := conn.GetRequestBody()
				if err := json.Unmarshal(request_body, &data); err != nil {
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
				headers := http.Header{}
				headers.Set("Content-Type", "application/json")
				headers.Set("__debug", "fake_resp")
				conn.ResponseWithoutRequest(200, []byte("{}"), headers)
				return
			}
		}
		if conn.IsAfterRequest() {
			header := conn.GetResponseHeader()
			resp_content_type := strings.ToLower(header.Get("Content-Type"))

			if resp_content_type == "text/html; charset=utf-8" {
				// fmt.Println(hostname, path)
				if hostname == "channels.weixin.qq.com" {
					resp_body, err := conn.GetResponseBody()
					if err != nil {
						return
					}
					header.Set("__debug", "append_script")

					html := string(resp_body)
					script_reg1 := regexp.MustCompile(`src="([^"]{1,})\.js"`)
					html = script_reg1.ReplaceAllString(html, `src="$1.js`+v+`"`)
					script_reg2 := regexp.MustCompile(`href="([^"]{1,})\.js"`)
					html = script_reg2.ReplaceAllString(html, `href="$1.js`+v+`"`)
					inserted_scripts := fmt.Sprintf(`<script>%s</script>`, biz.Files.JSUtils)
					script_config := fmt.Sprintf(`<script>var __wx_channels_config__ = {"defaultHighest":%t};</script>`, cfg.DownloadDefaultHighest)
					inserted_scripts += script_config
					if biz.Debug {
						/** 全局错误捕获 */
						script_error := fmt.Sprintf(`<script>%s</script>`, biz.Files.JSError)
						inserted_scripts += script_error
						/** 在线调试 */
						script_pagespy := fmt.Sprintf(`<script>%s</script>`, biz.Files.JSPageSpy)
						script_pagespy2 := fmt.Sprintf(`<script>%s</script>`, biz.Files.JSDebug)
						inserted_scripts += script_pagespy + script_pagespy2
					}
					if path == "/web/pages/feed" || path == "/web/pages/home" {
						/** 下载逻辑 */
						script_main := fmt.Sprintf(`<script>%s</script>`, biz.Files.JSMain)
						inserted_scripts += script_main
						html = strings.Replace(html, "<head>", "<head>\n"+inserted_scripts, 1)
						if path == "/web/pages/home" {
							fmt.Println("1. 视频号首页 html 注入 js 成功")
						}
						if path == "/web/pages/feed" {
							fmt.Println("1. 视频详情页 html 注入 js 成功")
						}
					}
					if path == "/web/pages/live" {
						script_live_main := fmt.Sprintf(`<script>%s</script>`, biz.Files.JSLiveMain)
						inserted_scripts += script_live_main
						html = strings.Replace(html, "<head>", "<head>\n"+inserted_scripts, 1)
						fmt.Println("1. 直播详情页 html 注入 js 成功")
					}
					conn.ModifyResponseBody([]byte(html))
					return
				}
				return
			}
			if resp_content_type == "application/javascript" {
				if util.Includes(path, "wasm_video_decode") {
					return
				}
				resp_body, err := conn.GetResponseBody()
				if err != nil {
					return
				}
				header.Set("__debug", "replace_script")

				js_script := string(resp_body)
				dep_reg := regexp.MustCompile(`"js/([^"]{1,})\.js"`)
				from_reg := regexp.MustCompile(`from {0,1}"([^"]{1,})\.js"`)
				lazy_import_reg := regexp.MustCompile(`import\("([^"]{1,})\.js"\)`)
				import_reg := regexp.MustCompile(`import {0,1}"([^"]{1,})\.js"`)
				js_script = from_reg.ReplaceAllString(js_script, `from"$1.js`+v+`"`)
				js_script = dep_reg.ReplaceAllString(js_script, `"js/$1.js`+v+`"`)
				js_script = lazy_import_reg.ReplaceAllString(js_script, `import("$1.js`+v+`")`)
				js_script = import_reg.ReplaceAllString(js_script, `import"$1.js`+v+`"`)

				if util.Includes(path, "/t/wx_fed/finder/web/web-finder/res/js/index.publish") {
					regexp1 := regexp.MustCompile(`this.sourceBuffer.appendBuffer\(h\),`)
					replace_str1 := `(() => {
									if (window.__wx_channels_store__) {
									window.__wx_channels_store__.buffers.push(h);
									}
									})(),this.sourceBuffer.appendBuffer(h),`
					if regexp1.MatchString(js_script) {
						fmt.Println("2. 视频播放 js 修改成功")
					}
					js_script = regexp1.ReplaceAllString(js_script, replace_str1)
					regexp2 := regexp.MustCompile(`if\(f.cmd===re.MAIN_THREAD_CMD.AUTO_CUT`)
					replace_str2 := `if(f.cmd==="CUT"){
										if (window.__wx_channels_store__ && __wx_channels_store__.profile) {
										console.log("CUT", f, __wx_channels_store__.profile.key);
										window.__wx_channels_store__.keys[__wx_channels_store__.profile.key]=f.decryptor_array;
										}
									}
									if(f.cmd===re.MAIN_THREAD_CMD.AUTO_CUT`
					js_script = regexp2.ReplaceAllString(js_script, replace_str2)
					conn.ModifyResponseBody([]byte(js_script))
					return
				}
				update_media_profile_text := `var profile = media.mediaType !== 4 ? {
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
									fetch("/__wx_channels_api/profile", {
										method: "POST",
										headers: {
											"Content-Type": "application/json"
										},
										body: JSON.stringify(profile)
									});
								})();`
				if util.Includes(path, "/t/wx_fed/finder/web/web-finder/res/js/virtual_svg-icons-register") {
					regexp1 := regexp.MustCompile(`async finderGetCommentDetail\((\w+)\)\{(.*?)\}async`)
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
				}async`, update_media_profile_text)
					if regexp1.MatchString(js_script) {
						fmt.Println("3.视频读取 js 修改成功")
					}
					js_script = regexp1.ReplaceAllString(js_script, replace_str1)
					regex2 := regexp.MustCompile(`i.default={dialog`)
					replace_str2 := `i.default=window.window.__wx_channels_tip__={dialog`
					js_script = regex2.ReplaceAllString(js_script, replace_str2)

					regexp3 := regexp.MustCompile(`async finderGetLiveInfo\((\w+)\)\{(.*?)\}async`)
					replace_str3 := fmt.Sprintf(`async finderGetLiveInfo($1) {
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
					fetch("/__wx_channels_api/profile", {
						method: "POST",
						headers: {
							"Content-Type": "application/json"
						},
						body: JSON.stringify(profile)
					});
					return feedResult;
				}async`)
					if regexp3.MatchString(js_script) {
						fmt.Println("4.直播读取 js 修改成功")
					}
					js_script = regexp3.ReplaceAllString(js_script, replace_str3)
					conn.ModifyResponseBody([]byte(js_script))
					return
				}
				if util.Includes(path, "vuexStores.publish") {
					regex1 := regexp.MustCompile(`goToNextFlowFeed:rs`)
					replace_str1 := fmt.Sprintf(`goToNextFlowFeed:async function(v){
									await rs(v);
									setTimeout(() => {
									var data_object = Zt.value.feed;
									var media = data_object.objectDesc.media[0];
									%v
									if (window.__insert_download_btn_to_home_page) {
				__insert_download_btn_to_home_page();
									}
									}, 0);
									}`, update_media_profile_text)
					js_script = regex1.ReplaceAllString(js_script, replace_str1)
					conn.ModifyResponseBody([]byte(js_script))
					return
				}
				if util.Includes(path, "/t/wx_fed/finder/web/web-finder/res/js/FeedDetail.publish") {
					regex := regexp.MustCompile(`,"投诉"\)]`)
					replace_str := `,"投诉"),...(() => {
						if (window.__wx_channels_store__ && window.__wx_channels_store__.profile) {
							return window.__wx_channels_store__.profile.spec.map((sp) => {
								return f("div",{class:"context-item",role:"button",onClick:() => __wx_channels_handle_click_download__(sp)},sp.fileFormat);
							});
						}
					return [];
					})(),f("div",{class:"context-item",role:"button",onClick:()=>__wx_channels_handle_click_download__()},"原始视频"),f("div",{class:"context-item",role:"button",onClick:__wx_channels_download_cur__},"当前视频"),f("div",{class:"context-item",role:"button",onClick:__wx_channels_handle_print_download_command},"打印下载命令"),f("div",{class:"context-item",role:"button",onClick:()=>__wx_channels_handle_download_cover()},"下载封面"),f("div",{class:"context-item",role:"button",onClick:__wx_channels_handle_copy__},"复制页面链接")]`

					js_script = regex.ReplaceAllString(js_script, replace_str)
					conn.ModifyResponseBody([]byte(js_script))
					return
				}
				if util.Includes(path, "worker_release") {
					regex := regexp.MustCompile(`fmp4Index:p.fmp4Index`)
					replace_str := `decryptor_array:p.decryptor_array,fmp4Index:p.fmp4Index`
					js_script = regex.ReplaceAllString(js_script, replace_str)
					conn.ModifyResponseBody([]byte(js_script))
					return
				}
				conn.ModifyResponseBody([]byte(js_script))
			}
		}
	}
}
