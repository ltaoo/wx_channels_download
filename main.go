package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/qtgolang/SunnyNet/SunnyNet"
	"github.com/qtgolang/SunnyNet/public"
)

var Sunny = SunnyNet.NewSunny()
var v = "?t=241101"

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

//go:embed certs/SunnyRoot.cer
var cert_data []byte

//go:embed lib/FileSaver.min.js
var file_saver_js []byte

//go:embed lib/jszip.min.js
var zip_js []byte

type Subject struct {
	CN string
	OU string
	O  string
	L  string
	S  string
	C  string
}
type Certificate struct {
	Thumbprint string
	Subject    Subject
}

func fetchCertificates() ([]Certificate, error) {
	// 获取指定 store 所有证书
	cmd := fmt.Sprintf("Get-ChildItem Cert:\\LocalMachine\\Root")
	ps := exec.Command("powershell.exe", "-Command", cmd)
	output, err2 := ps.CombinedOutput()
	if err2 != nil {
		return nil, errors.New(fmt.Sprintf("安装证书时发生错误，%v\n", err2.Error()))
	}
	var certificates []Certificate
	lines := strings.Split(string(output), "\n")
	// 跳过前两行（列名）
	for i := 2; i < len(lines)-1; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		parts := strings.Split(line, " ")
		if len(parts) >= 2 {
			subject := Subject{}
			for _, part := range parts[1:] {
				part = strings.Replace(part, ",", "", 1)
				kv := strings.Split(part, "=")
				if len(kv) == 2 {
					key := strings.TrimSpace(kv[0])
					value := strings.TrimSpace(kv[1])
					switch key {
					case "CN":
						subject.CN = value
					case "OU":
						subject.OU = value
					case "O":
						subject.O = value
					case "L":
						subject.L = value
					case "S":
						subject.S = value
					case "C":
						subject.C = value
					}
				}
			}
			certificates = append(certificates, Certificate{
				Thumbprint: parts[0],
				Subject:    subject,
			})
		}
	}
	return certificates, nil
}
func checkCertificate(cert_name string) (bool, error) {
	certificates, err := fetchCertificates()
	if err != nil {
		return false, err
	}
	for _, cert := range certificates {
		if cert.Subject.CN == cert_name {
			return true, nil
		}
	}
	return false, nil
}
func removeCertificate() {
	// 删除指定证书
	// Remove-Item "Cert:\LocalMachine\Root\D70CD039051F77C30673B8209FC15EFA650ED52C"
}
func installCertificate() error {
	cert_file, err := os.CreateTemp("", "SunnyRoot.cer")
	if err != nil {
		return errors.New(fmt.Sprintf("没有创建证书的权限，%v\n", err.Error()))
	}
	defer os.Remove(cert_file.Name())
	if _, err := cert_file.Write(cert_data); err != nil {
		return errors.New(fmt.Sprintf("获取证书失败，%v\n", err.Error()))
	}
	if err := cert_file.Close(); err != nil {
		return errors.New(fmt.Sprintf("生成证书失败，%v\n", err.Error()))
	}
	cmd := fmt.Sprintf("Import-Certificate -FilePath '%s' -CertStoreLocation Cert:\\LocalMachine\\Root", cert_file.Name())
	ps := exec.Command("powershell.exe", "-Command", cmd)
	_, err2 := ps.CombinedOutput()
	if err2 != nil {
		return errors.New(fmt.Sprintf("安装证书时发生错误，%v\n", err2.Error()))
	}
	return nil
}

func clear_terminal() {
	cmd := exec.Command("clear")
	if os.Getenv("OS") == "Windows_NT" {
		cmd = exec.Command("cls")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func main() {
	signalChan := make(chan os.Signal, 1)
	// Notify the signal channel on SIGINT (Ctrl+C) and SIGTERM
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-signalChan
		fmt.Printf("\n正在关闭服务...%v\n\n", sig)
		os.Exit(0)
	}()

	existing, err1 := checkCertificate("SunnyNet")
	if err1 != nil {
		fmt.Printf("ERROR %v\v", err1.Error())
		return
	}
	if existing == false {
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		fmt.Println("\n\n正在安装证书...")
		err := installCertificate()
		time.Sleep(3 * time.Second)
		s.Stop()
		if err != nil {
			fmt.Printf("ERROR %v\n", err.Error())
			return
		}
	}
	port := 2023
	Sunny.SetPort(port)
	Sunny.SetGoCallback(HttpCallback, nil, nil, nil)
	err := Sunny.Start().Error
	if err != nil {
		fmt.Printf("ERROR %v\n", err.Error())
		return
	}
	sunny_site := fmt.Sprintf("127.0.0.1:%v", port)
	color.Green(fmt.Sprintf("\n\n服务已正确启动，请打开需要下载的视频号页面进行下载"))
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
			fmt.Println("ERROR 启动进程代理失败")
			return
		}
		Sunny.ProcessAddName("WeChatAppEx.exe")
	} else {
		fmt.Println(fmt.Sprintf("\n\n您还未安装证书，请在浏览器打开 http://%v 并根据说明安装证书\n在安装完成后重新启动此程序即可\n", sunny_site))
	}

	color.White(fmt.Sprintf("\n\n此程序由 ltaoo 制作"))
	time.Sleep(24 * time.Hour)
}

func HttpCallback(Conn *SunnyNet.HttpConn) {
	host := Conn.Request.URL.Hostname()
	path := Conn.Request.URL.Path
	if Conn.Type == public.HttpSendRequest {
		// Conn.Request.Header.Set("Cache-Control", "no-cache")
		Conn.Request.Header.Del("Accept-Encoding")
		if Includes(path, "jszip") {
			// data, err := os.ReadFile("./lib/jszip.min.js")
			// if err != nil {
			// 	fmt.Printf("read file failed, because %v\n", err.Error())
			// 	return
			// }
			headers := http.Header{}
			headers.Set("Content-Type", "application/javascript")
			headers.Set("__debug", "local_file")
			Conn.StopRequest(200, zip_js, headers)
			return
		}
		if Includes(path, "FileSaver.min") {
			// data, err := os.ReadFile("./lib/FileSaver.min.js")
			// if err != nil {
			// 	return
			// }
			headers := http.Header{}
			headers.Set("Content-Type", "application/javascript")
			headers.Set("__debug", "local_file")
			Conn.StopRequest(200, file_saver_js, headers)
			return
		}
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
				async function __wx_channels_download(profile, filename) {
					// console.log("__wx_channels_download");
					const data = profile.data;
					const blob = new Blob(data, { type: 'application/octet-stream' });
					await __wx_load_script("https://res.wx.qq.com/t/wx_fed/cdn_libs/res/jszip.min.js");
					await __wx_load_script("https://res.wx.qq.com/t/wx_fed/cdn_libs/res/FileSaver.min.js");
					const zip = new JSZip();
					zip.file(filename + ".mp4", blob);
					const content = await zip.generateAsync({ type: "blob" });
					saveAs(content, filename + ".zip");
				}
				async function __wx_channels_download2(profile, filename) {
					const url = profile.url;
					// console.log("__wx_channels_download2", url);
					await __wx_load_script("https://res.wx.qq.com/t/wx_fed/cdn_libs/res/jszip.min.js");
					await __wx_load_script("https://res.wx.qq.com/t/wx_fed/cdn_libs/res/FileSaver.min.js");
					const zip = new JSZip();
					const response = await fetch(url);
					const blob = await response.blob();
					zip.file(filename + ".mp4", blob);
					const content = await zip.generateAsync({ type: "blob" });
					saveAs(content, filename + ".zip");
				}
				async function __wx_channels_download3(profile, filename) {
					// console.log("__wx_channels_download3");
					const files = profile.files;
					await __wx_load_script("https://res.wx.qq.com/t/wx_fed/cdn_libs/res/jszip.min.js");
					await __wx_load_script("https://res.wx.qq.com/t/wx_fed/cdn_libs/res/FileSaver.min.js");
					const zip = new JSZip();
					zip.file("contact.txt", JSON.stringify(profile.contact, null, 2));
					const folder = zip.folder("images");
					const fetchPromises = files.map((f) => f.url).map(async (url, index) => {
						const response = await fetch(url);
						const blob = await response.blob();
						folder.file((index + 1) + ".png", blob);
					});
					await Promise.all(fetchPromises);
					const content = await zip.generateAsync({ type: "blob" });
					saveAs(content, filename + ".zip");
				}
				function __wx_load_script(src) {
					return new Promise((resolve, reject) => {
						const script = document.createElement('script');
						script.type = 'text/javascript';
						script.src = src;
						script.onload = resolve;
						script.onerror = reject;
						document.head.appendChild(script);
					});
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
					if (profile && profile.type === "picture") {
						__wx_channels_download3(profile, filename);
						return;
					}
					if (profile && __wx_channels_store__.buffers.length === 0) {
						__wx_channels_download2(profile, filename);
						return;
					}
					profile.data = __wx_channels_store__.buffers;
					__wx_channels_download(profile, filename);
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
					var profile = media.videoType === 0 ? {
						type: "picture",
						id: data_object.id,
						title: data_object.objectDesc.description,
						files: data_object.objectDesc.media,
						contact: data_object.contact
					} : {
						type: "media",
						duration: media.spec[0].durationMs,
						title: data_object.objectDesc.description,
						url: media.url+media.urlToken,
						size: media.fileSize,
						key: media.decodeKey,
						id: data_object.id,
						nonce_id: data_object.objectNonceId,
						nickname: data_object.nickname,
						createtime: data_object.createtime,
						fileFormat: media.spec.map(o => o.fileFormat),
						contact: data_object.contact
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
					regex := regexp.MustCompile(`this.sourceBuffer.appendBuffer\(l\),`)
					replaceStr := `(() => {
if (__wx_channels_store__) {
	__wx_channels_store__.buffers.push(l);
}
})(),this.sourceBuffer.appendBuffer(l),`
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
