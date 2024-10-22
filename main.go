package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/qtgolang/SunnyNet/SunnyNet"
	"github.com/qtgolang/SunnyNet/public"
)

var Sunny = SunnyNet.NewSunny()
var v = "?t=241022"

func Includes(str, substr string) bool {
	return strings.Contains(str, substr)
}
func getFreePort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0
	}
	defer listener.Close()
	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port
}
func main() {
	port := 2023
	Sunny.SetPort(port)
	Sunny.SetGoCallback(HttpCallback, nil, nil, nil)
	err := Sunny.Start().Error
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	sunny_site := fmt.Sprintf("127.0.0.1:%v", port)
	fmt.Println(fmt.Sprintf("\nServer is running at port %v", port))
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(&url.URL{
				Scheme: "http",
				Host:   sunny_site,
			}),
		},
	}
	_, err3 := client.Get("https://sunny.io/")
	if err3 == nil {
		ok := Sunny.StartProcess()
		if !ok {
			fmt.Println("启动进程代理失败")
			return
		}
		Sunny.ProcessAddName("WeChatAppEx.exe")
	} else {
		fmt.Println(fmt.Sprintf("\n\n您还未安装证书，请在浏览器打开 http://%v 并根据说明安装证书\n在安装完成后重新启动此程序即可\n", sunny_site))
	}

	// bold := color.New(color.FgGreen).SprintFunc()
	fmt.Println(fmt.Sprintf("\n\n此程序由 ltaoo 制作"))
	fmt.Println(fmt.Sprintf("https://www.zhihu.com/people/ltaoo-46\n\n"))

	fmt.Println(fmt.Sprintf("\n另外此程序大部分参考自以下项目代码"))
	fmt.Println(fmt.Sprintf("https://github.com/kanadeblisst00/WechatVideoSniffer2.0"))
	fmt.Println(fmt.Sprintf("https://github.com/qtgolang/SunnyNet"))
	time.Sleep(24 * time.Hour)
}

func HttpCallback(Conn *SunnyNet.HttpConn) {
	host := Conn.Request.URL.Hostname()
	path := Conn.Request.URL.Path
	if Conn.Type == public.HttpSendRequest {
		// Conn.Request.Header.Set("Cache-Control", "no-cache")
		Conn.Request.Header.Del("Accept-Encoding")
		// 查找主机的 IP 地址
		// _, err := net.LookupHost(host)
		// if err != nil {
		// 	fmt.Printf("无法查找主机 %s: %v\n", host, err)
		// 	Conn.StopRequest(200, "{}", http.Header{
		// 		"content-type": []string{"application/json; charset=UTF-8"},
		// 	})
		// 	return
		// }
	}
	if Conn.Type == public.HttpResponseOK {
		content_type := strings.ToLower(Conn.Response.Header.Get("content-type"))
		if Conn.Response.Body != nil {
			Body, _ := io.ReadAll(Conn.Response.Body)
			_ = Conn.Response.Body.Close()
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
				html := string(Body)
				script_reg1 := regexp.MustCompile(`src="([^"]{1,})\.js"`)
				html = script_reg1.ReplaceAllString(html, `src="$1.js`+v+`"`)
				script_reg2 := regexp.MustCompile(`href="([^"]{1,})\.js"`)
				html = script_reg2.ReplaceAllString(html, `href="$1.js`+v+`"`)
				Conn.Response.Header.Set("__debug", "append_script")

				if host == "channels.weixin.qq.com" && path == "/web/pages/feed" {
					// Conn.Response.Header.Add("wx-channel-video-download", "1")
					script := `<script>
				function __wx_channels_copy(text) {
					const textArea = document.createElement("textarea");
					textArea.value = text;
					textArea.style.cssText = "position: absolute; top: -999px; left: -999px;";
					document.body.appendChild(textArea);
					textArea.select();
					document.execCommand("copy");
					document.body.removeChild(textArea);
				}
				function __wx_channels_download(data, filename) {
					const blob = new Blob(data, { type: 'application/octet-stream' });
					const url = URL.createObjectURL(blob);
					__wx_channels_download2(url, filename);
				}
				function __wx_channels_download2(url, filename) {
					const a = document.createElement('a');
					a.href = url;
					a.download = filename + '.mp4';
					document.body.appendChild(a);
					a.click();
					document.body.removeChild(a);
					URL.revokeObjectURL(url);
				}
				var __wx_channels_store__ = {
					profile: null,
					buffers: [],
				};
				var __wx_channels_video_download_btn__ = document.createElement("div");
				__wx_channels_video_download_btn__.innerHTML = '<div data-v-6548f11a data-v-c2373d00 class="click-box op-item item-gap-combine" role="button" aria-label="下载" style="padding: 4px 4px 4px 4px; --border-radius: 4px; --left: 0; --top: 0; --right: 0; --bottom: 0;"><svg data-v-c2373d00 class="svg-icon icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" fill="currentColor" width="28" height="28"><path d="M213.333333 853.333333h597.333334v-85.333333H213.333333m597.333334-384h-170.666667V128H384v256H213.333333l298.666667 298.666667 298.666667-298.666667z"></path></svg></div>';
				__wx_channels_video_download_btn__ = __wx_channels_video_download_btn__.firstChild;
				__wx_channels_video_download_btn__.onclick = () => {
					var profile = __wx_channels_store__.profile;
					if (!profile) {
						alert("检测不到视频，请将本工具更新到最新版");
						return;
					}
					console.log(__wx_channels_store__);
					var filename = (() => {
						if (profile.title) {
							return profile.title;
						}
						if (profile.id) {
							return profile.id;
						}
						return new Date().valueOf();
					})();
					if (profile && __wx_channels_store__.buffers.length === 0) {
						__wx_channels_download2(profile.url, filename);
						return;
					}
					__wx_channels_download(__wx_channels_store__.buffers, filename);
				};
				var count = 0;
				var __timer = setInterval(() => {
					count += 1;
					if (count >= 10) {
						clearInterval(__timer);
						__timer = null;
						return;
					}
					const $wrap = document.getElementsByClassName("full-opr-wrp layout-row")[0];
					if (!$wrap) {
						return;
					}
					clearInterval(__timer);
					__timer = null;
					$wrap.insertBefore(__wx_channels_video_download_btn__, $wrap.children[$wrap.children.length - 1]);
				}, 1000);
				</script>`
					html = strings.Replace(html, "<head>", "<head>\n"+script, 1)
					Conn.Response.Body = io.NopCloser(bytes.NewBuffer([]byte(html)))
					return
				}
				Conn.Response.Body = io.NopCloser(bytes.NewBuffer([]byte(html)))
				return
			}
			if content_type == "application/javascript" {
				content := string(Body)
				dep_reg := regexp.MustCompile(`"js/([^"]{1,})\.js"`)
				from_reg := regexp.MustCompile(`from {0,1}"([^"]{1,})\.js"`)
				lazy_import_reg := regexp.MustCompile(`import\("([^"]{1,})\.js"\)`)
				import_reg := regexp.MustCompile(`import {0,1}"([^"]{1,})\.js"`)
				content = from_reg.ReplaceAllString(content, `from"$1.js`+v+`"`)
				content = dep_reg.ReplaceAllString(content, `"js/$1.js`+v+`"`)
				content = lazy_import_reg.ReplaceAllString(content, `import("$1.js`+v+`")`)
				content = import_reg.ReplaceAllString(content, `import"$1.js`+v+`"`)
				Conn.Response.Header.Set("__debug", "replace_script")

				if Includes(path, "/t/wx_fed/finder/web/web-finder/res/js/virtual_svg-icons-register") {
					regex := regexp.MustCompile(`async finderGetCommentDetail\((\w+)\)\{return(.*?)\}async`)
					replaceStr := `async finderGetCommentDetail($1) {
					var feedResult = await$2;
					var data_object = feedResult.data.object;
					if (!data_object.objectDesc) {
						return feedResult;
					}
					var media = data_object.objectDesc.media[0];
					var profile = {
						duration: media.spec[0].durationMs,
						title: data_object.objectDesc.description,
						url: media.url+media.urlToken,
						size: media.fileSize,
						key: media.decodeKey,
						id: data_object.id,
						nonce_id: data_object.objectNonceId,
						nickname: data_object.nickname,
						createtime: data_object.createtime,
						fileFormat: media.spec.map(o => o.fileFormat)
					};
					__wx_channels_store__.profile = profile;
					return feedResult;
				}async`
					content = regex.ReplaceAllString(content, replaceStr)
					// hookBody = strings.Replace(hookBody, "js/index.publishBWnoWtFy.js", "js/index.publishBWnoWtFy.js?t="+now, 1)
					// Conn.Response.Header.Set("Cache-Control", "no-cache,no-store,max-age=0")
					// Conn.Response.Header.Set("Expires", "0")
					// Conn.Response.Header.Set("Pragma", "no-cache")
					Conn.Response.Body = io.NopCloser(bytes.NewBuffer([]byte(content)))
					return
				}
				if Includes(path, "/t/wx_fed/finder/web/web-finder/res/js/index") {
					regex := regexp.MustCompile(`this.sourceBuffer.appendBuffer\(o\),`)
					replaceStr := `(() => {
console.log(u);
if (__wx_channels_store__) {
	__wx_channels_store__.buffers.push(o);
}
})(),this.sourceBuffer.appendBuffer(o),`
					hookBody := regex.ReplaceAllString(content, replaceStr)
					Conn.Response.Body = io.NopCloser(bytes.NewBuffer([]byte(hookBody)))
					return
				}
				Conn.Response.Body = io.NopCloser(bytes.NewBuffer([]byte(content)))
				return
			}
			Conn.Response.Body = io.NopCloser(bytes.NewBuffer(Body))
		}

	}
	if Conn.Type == public.HttpRequestFail {
		//请求错误
		// Body := []byte("Hello Sunny Response")
		// Conn.Response = &http.Response{
		// 	Body: io.NopCloser(bytes.NewBuffer(Body)),
		// }
	}
}
func WSCallback(Conn *SunnyNet.WsConn) {
	//捕获到数据可以修改,修改空数据,取消发送/接收
	fmt.Println("WSCallback", Conn.Url)
}
func TcpCallback(Conn *SunnyNet.TcpConn) {
	fmt.Println("TcpCallback", Conn.Type)
	//捕获到数据可以修改,修改空数据,取消发送/接收
	fmt.Println(Conn.Pid, Conn.LocalAddr, Conn.RemoteAddr, Conn.Type, Conn.GetBodyLen())
}
func UdpCallback(Conn *SunnyNet.UDPConn) {
	fmt.Println("UdpCallback", Conn.Type)
	//在 Windows 捕获UDP需要加载驱动,并且设置进程名
	//其他情况需要设置Socket5代理,才能捕获到UDP
	//捕获到数据可以修改,修改空数据,取消发送/接收
	if public.SunnyNetUDPTypeReceive == Conn.Type {
		fmt.Println("接收UDP", Conn.LocalAddress, Conn.RemoteAddress, len(Conn.Data))
	}
	if public.SunnyNetUDPTypeSend == Conn.Type {
		fmt.Println("发送UDP", Conn.LocalAddress, Conn.RemoteAddress, len(Conn.Data))
	}
	if public.SunnyNetUDPTypeClosed == Conn.Type {
		fmt.Println("关闭UDP", Conn.LocalAddress, Conn.RemoteAddress)
	}
}
