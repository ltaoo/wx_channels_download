package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"wx_channel/internal/application"
	"wx_channel/pkg/echo"
	"wx_channel/pkg/util"
)

func HandleHttpRequestEcho(biz *application.Biz) func(conn *echo.EchoConn) {
	return func(conn *echo.EchoConn) {
		parsed_url, err := conn.URL()
		if err != nil {
			fmt.Printf("URL解析失败: %v\n", err)
			return
		}
		if err != nil {
			fmt.Printf("URL解析失败: %v\n", err)
			return
		}
		hostname := parsed_url.Hostname()
		path := parsed_url.Path
		var v = "?t=" + biz.Version
		if conn.IsBeforeRequest() {
			// conn.GetRequestHeader().Del("Accept-Encoding")
			if util.Includes(path, "jszip") {
				headers := http.Header{}
				headers.Set("Content-Type", "application/javascript")
				headers.Set("__debug", "local_file")
				// conn.StopRequest(200, biz.Files.JSZip, headers)
				conn.ResponseWithoutRequest(200, biz.Files.JSZip, headers)
				return
			}
			if util.Includes(path, "FileSaver.min") {
				headers := http.Header{}
				headers.Set("Content-Type", "application/javascript")
				headers.Set("__debug", "local_file")
				// conn.StopRequest(200, file_saver_js, headers)
				conn.ResponseWithoutRequest(200, biz.Files.JSZip, headers)
				return
			}
			if path == "/__wx_channels_api/profile" {
				var data ChannelMediaProfile
				request_body := conn.GetRequestBody()
				err := json.Unmarshal(request_body, &data)
				if err != nil {
					fmt.Println(err.Error())
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
				err := json.Unmarshal(request_body, &data)
				if err != nil {
					fmt.Println(err.Error())
				}
				if data.End == 1 {
					fmt.Println()
				} else if data.Replace == 1 {
					fmt.Printf("\r\033[K[FRONTEND]%s", data.Msg)
				} else {
					fmt.Printf("[FRONTEND]%s\n", data.Msg)
				}
				headers := http.Header{}
				headers.Set("Content-Type", "application/json")
				headers.Set("__debug", "fake_resp")
				conn.ResponseWithoutRequest(200, []byte("{}"), headers)
				return
			}
		}
		if conn.IsAfterRequest() {
			content_type := strings.ToLower(conn.GetResponseHeader().Get("Content-Type"))
			if conn.GetResponseBody() != nil {
				request_body := conn.GetResponseBody()
				// if content_type == "text/css" {
				// 	Conn.Response.Body = io.NopCloser(bytes.NewBuffer(Body))
				// 	return
				// }
				// if content_type == "image/svg+xml" {
				// 	Conn.Response.Body = io.NopCloser(bytes.NewBuffer(Body))
				// 	return
				// }
				// if content_type == "image/jpeg" {
				// 	Conn.Response.Body = io.NopCloser(bytes.NewBuffer(Body))
				// 	return
				// }
				// if content_type == "image/jpg" {
				// 	Conn.Response.Body = io.NopCloser(bytes.NewBuffer(Body))
				// 	return
				// }
				// if content_type == "image/png" {
				// 	Conn.Response.Body = io.NopCloser(bytes.NewBuffer(Body))
				// 	return
				// }
				// if content_type == "image/gif" {
				// 	Conn.Response.Body = io.NopCloser(bytes.NewBuffer(Body))
				// 	return
				// }
				// if content_type == "video/mp4" {
				// 	Conn.Response.Body = io.NopCloser(bytes.NewBuffer(Body))
				// 	return
				// }
				// if path == "/web/report-perf" {
				// 	Conn.Response.Body = io.NopCloser(bytes.NewBuffer(Body))
				// 	return
				// }
				// fmt.Println("HttpCallback", Conn.Type, host, path)
				// fmt.Println("Response ContentType is", content_type)
				if content_type == "text/html; charset=utf-8" {
					html := string(request_body)
					script_reg1 := regexp.MustCompile(`src="([^"]{1,})\.js"`)
					html = script_reg1.ReplaceAllString(html, `src="$1.js`+v+`"`)
					script_reg2 := regexp.MustCompile(`href="([^"]{1,})\.js"`)
					html = script_reg2.ReplaceAllString(html, `href="$1.js`+v+`"`)
					conn.GetResponseHeader().Set("__debug", "append_script")
					if hostname == "channels.weixin.qq.com" && (path == "/web/pages/feed" || path == "/web/pages/home") {
						inserted_scripts := ""
						if biz.Debug {
							/** 全局错误捕获 */
							script_error := fmt.Sprintf(`<script>%s</script>`, biz.Files.JSError)
							inserted_scripts = script_error
							/** 在线调试 */
							script_pagespy := fmt.Sprintf(`<script>%s</script>`, biz.Files.JSPageSpy1)
							script_pagespy2 := fmt.Sprintf(`<script>%s</script>`, biz.Files.JSPageSpy2)
							inserted_scripts += script_pagespy + script_pagespy2
						}
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
						// conn.ModifyResponseBody(io.NopCloser(bytes.NewBuffer([]byte(html))))
						conn.ModifyResponseBody([]byte(html))
						return
					}
					// conn.SetResponseBodyIO(io.NopCloser(bytes.NewBuffer([]byte(html))))
					conn.ModifyResponseBody([]byte(html))
					return
				}
				if content_type == "application/javascript" {
					content := string(request_body)
					dep_reg := regexp.MustCompile(`"js/([^"]{1,})\.js"`)
					from_reg := regexp.MustCompile(`from {0,1}"([^"]{1,})\.js"`)
					lazy_import_reg := regexp.MustCompile(`import\("([^"]{1,})\.js"\)`)
					import_reg := regexp.MustCompile(`import {0,1}"([^"]{1,})\.js"`)
					content = from_reg.ReplaceAllString(content, `from"$1.js`+v+`"`)
					content = dep_reg.ReplaceAllString(content, `"js/$1.js`+v+`"`)
					content = lazy_import_reg.ReplaceAllString(content, `import("$1.js`+v+`")`)
					content = import_reg.ReplaceAllString(content, `import"$1.js`+v+`"`)
					conn.GetResponseHeader().Set("__debug", "replace_script")

					if util.Includes(path, "/t/wx_fed/finder/web/web-finder/res/js/index.publish") {
						regexp1 := regexp.MustCompile(`this.sourceBuffer.appendBuffer\(h\),`)
						replace_str1 := `(() => {
if (window.__wx_channels_store__) {
window.__wx_channels_store__.buffers.push(h);
}
})(),this.sourceBuffer.appendBuffer(h),`
						if regexp1.MatchString(content) {
							fmt.Println("2. 视频播放 js 修改成功")
						}
						content = regexp1.ReplaceAllString(content, replace_str1)
						regexp2 := regexp.MustCompile(`if\(f.cmd===re.MAIN_THREAD_CMD.AUTO_CUT`)
						replace_str2 := `if(f.cmd==="CUT"){
	if (window.__wx_channels_store__ && __wx_channels_store__.profile) {
	console.log("CUT", f, __wx_channels_store__.profile.key);
	window.__wx_channels_store__.keys[__wx_channels_store__.profile.key]=f.decryptor_array;
	}
}
if(f.cmd===re.MAIN_THREAD_CMD.AUTO_CUT`
						content = regexp2.ReplaceAllString(content, replace_str2)
						// conn.SetResponseBodyIO(io.NopCloser(bytes.NewBuffer([]byte(content))))
						conn.ModifyResponseBody([]byte(content))
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
				})();
				`
					if util.Includes(path, "/t/wx_fed/finder/web/web-finder/res/js/virtual_svg-icons-register") {
						regexp1 := regexp.MustCompile(`async finderGetCommentDetail\((\w+)\)\{return(.*?)\}async`)
						replace_str1 := fmt.Sprintf(`async finderGetCommentDetail($1) {
					var feedResult = await$2;
					var data_object = feedResult.data.object;
					if (!data_object.objectDesc) {
						return feedResult;
					}
					var media = data_object.objectDesc.media[0];
					%v
					return feedResult;
				}async`, update_media_profile_text)
						if regexp1.MatchString(content) {
							fmt.Println("3.视频读取 js 修改成功")
						}
						content = regexp1.ReplaceAllString(content, replace_str1)
						regex2 := regexp.MustCompile(`i.default={dialog`)
						replace_str2 := `i.default=window.window.__wx_channels_tip__={dialog`
						content = regex2.ReplaceAllString(content, replace_str2)
						// conn.SetResponseBodyIO(io.NopCloser(bytes.NewBuffer([]byte(content))))
						conn.ModifyResponseBody([]byte(content))
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
						content = regex1.ReplaceAllString(content, replace_str1)

						// conn.SetResponseBodyIO(io.NopCloser(bytes.NewBuffer([]byte(content))))
						conn.ModifyResponseBody([]byte(content))
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
						content = regex.ReplaceAllString(content, replace_str)
						// conn.SetResponseBodyIO(io.NopCloser(bytes.NewBuffer([]byte(content))))
						conn.ModifyResponseBody([]byte(content))
						return
					}
					if util.Includes(path, "worker_release") {
						regex := regexp.MustCompile(`fmp4Index:p.fmp4Index`)
						replace_str := `decryptor_array:p.decryptor_array,fmp4Index:p.fmp4Index`
						content = regex.ReplaceAllString(content, replace_str)
						// conn.SetResponseBodyIO(io.NopCloser(bytes.NewBuffer([]byte(content))))
						conn.ModifyResponseBody([]byte(content))
						return
					}
					// conn.SetResponseBodyIO(io.NopCloser(bytes.NewBuffer([]byte(content))))
					conn.ModifyResponseBody([]byte(content))
					return
				}
				// conn.SetResponseBodyIO(io.NopCloser(bytes.NewBuffer([]byte(request_body))))
				conn.ModifyResponseBody([]byte(request_body))
			}
		}
	}
}
