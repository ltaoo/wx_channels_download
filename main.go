package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
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
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/qtgolang/SunnyNet/SunnyNet"
	"github.com/qtgolang/SunnyNet/public"
)

//go:embed certs/SunnyRoot.cer
var cert_data []byte

//go:embed lib/FileSaver.min.js
var file_saver_js []byte

//go:embed lib/jszip.min.js
var zip_js []byte

//go:embed inject/main.js
var main_js []byte

var Sunny = SunnyNet.NewSunny()
var version = "241216"
var v = "?t=" + version

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

func fetchCertificatesInWindows() ([]Certificate, error) {
	// 获取指定 store 所有证书
	cmd := fmt.Sprintf("Get-ChildItem Cert:\\LocalMachine\\Root")
	ps := exec.Command("powershell.exe", "-Command", cmd)
	output, err2 := ps.CombinedOutput()
	if err2 != nil {
		return nil, errors.New(fmt.Sprintf("获取证书时发生错误，%v\n", err2.Error()))
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
func fetchCertificatesInMacOS() ([]Certificate, error) {
	cmd := exec.Command("security", "find-certificate", "-a")
	output, err2 := cmd.Output()
	if err2 != nil {
		return nil, errors.New(fmt.Sprintf("获取证书时发生错误，%v\n", err2.Error()))
	}
	var certificates []Certificate
	lines := strings.Split(string(output), "\n")
	for i := 0; i < len(lines)-1; i += 13 {
		if lines[i] == "" {
			continue
		}
		// if i > len(lines)-1 {
		// 	continue
		// }
		cenc := lines[i+5]
		ctyp := lines[i+6]
		hpky := lines[i+7]
		labl := lines[i+9]
		subj := lines[i+12]
		re := regexp.MustCompile(`="([^"]{1,})"`)
		// 找到匹配的字符串
		matches := re.FindStringSubmatch(labl)
		if len(matches) < 1 {
			continue
		}
		label := matches[1]
		certificates = append(certificates, Certificate{
			Thumbprint: "",
			Subject: Subject{
				CN: label,
				OU: cenc,
				O:  ctyp,
				L:  hpky,
				S:  subj,
				C:  cenc,
			},
		})
	}
	return certificates, nil
}

func fetchCertificates() ([]Certificate, error) {
	os_env := runtime.GOOS
	switch os_env {
	case "linux":
		fmt.Println("Running on Linux")
	case "darwin":
		return fetchCertificatesInMacOS()
	case "windows":
		return fetchCertificatesInWindows()
	default:
		fmt.Printf("Running on %s\n", os_env)
	}
	return nil, errors.New(fmt.Sprintf("unknown OS\n"))

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
func installCertificateInWindows() error {
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
func installCertificateInMacOS() error {
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
	cmd := fmt.Sprintf("security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain '%s'", cert_file.Name())
	ps := exec.Command("bash", "-c", cmd)
	output, err2 := ps.CombinedOutput()
	if err2 != nil {
		return errors.New(fmt.Sprintf("安装证书时发生错误，%v\n", output))
	}
	return nil
}

func installCertificate() error {
	os_env := runtime.GOOS
	switch os_env {
	case "linux":
		fmt.Println("Running on Linux")
	case "darwin":
		return installCertificateInMacOS()
	case "windows":
		return installCertificateInWindows()
	default:
		fmt.Printf("Running on %s\n", os_env)
	}
	return errors.New(fmt.Sprintf("unknown OS\n"))
}

func enableProxyInMacOS() error {
	cmd1 := exec.Command("networksetup", "-setwebproxy", "Wi-Fi", "127.0.0.1", "2023")
	_, err1 := cmd1.Output()
	if err1 != nil {
		return errors.New(fmt.Sprintf("设置 HTTP 代理失败，%v\n", err1.Error()))
	}
	cmd2 := exec.Command("networksetup", "-setsecurewebproxy", "Wi-Fi", "127.0.0.1", "2023")
	_, err2 := cmd2.Output()
	if err2 != nil {
		return errors.New(fmt.Sprintf("设置 HTTPS 代理失败，%v\n", err2.Error()))
	}
	return nil
}
func disableProxyInMacOS() error {
	cmd1 := exec.Command("networksetup", "-setwebproxystate", "Wi-Fi", "off")
	_, err1 := cmd1.Output()
	if err1 != nil {
		return errors.New(fmt.Sprintf("禁用 HTTP 代理失败，%v\n", err1.Error()))
	}
	cmd2 := exec.Command("networksetup", "-setsecurewebproxystate", "Wi-Fi", "off")
	_, err2 := cmd2.Output()
	if err2 != nil {
		return errors.New(fmt.Sprintf("禁用 HTTPS 代理失败，%v\n", err2.Error()))
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
	os_env := runtime.GOOS
	signalChan := make(chan os.Signal, 1)
	// Notify the signal channel on SIGINT (Ctrl+C) and SIGTERM
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-signalChan
		fmt.Printf("\n正在关闭服务...%v\n\n", sig)
		if os_env == "darwin" {
			disableProxyInMacOS()
		}
		os.Exit(0)
	}()
	fmt.Printf("\nv" + version)
	fmt.Printf("\n问题反馈 https://github.com/ltaoo/wx_channels_download/issues\n")
	existing, err1 := checkCertificate("SunnyNet")
	if err1 != nil {
		fmt.Printf("\nERROR %v\v", err1.Error())
		fmt.Printf("按 Ctrl+C 退出...\n")
		select {}
	}
	if existing == false {
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		fmt.Printf("\n\n正在安装证书...\n")
		err := installCertificate()
		time.Sleep(3 * time.Second)
		s.Stop()
		if err != nil {
			fmt.Printf("\nERROR %v\n", err.Error())
			fmt.Printf("按 Ctrl+C 退出...\n")
			select {}
		}
	}
	port := 2023
	Sunny.SetPort(port)
	Sunny.SetGoCallback(HttpCallback, nil, nil, nil)
	err := Sunny.Start().Error
	if err != nil {
		fmt.Printf("\nERROR %v\n", err.Error())
		fmt.Printf("按 Ctrl+C 退出...\n")
		select {}
	}
	proxy_server := fmt.Sprintf("127.0.0.1:%v", port)
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(&url.URL{
				Scheme: "http",
				Host:   proxy_server,
			}),
		},
	}
	_, err3 := client.Get("https://sunny.io/")
	if err3 == nil {
		if os_env == "windows" {
			ok := Sunny.StartProcess()
			if !ok {
				fmt.Printf("\nERROR 启动进程代理失败\n")
				fmt.Printf("按 Ctrl+C 退出...\n")
				select {}
			}
			Sunny.ProcessAddName("WeChatAppEx.exe")
		}
		if os_env == "darwin" {
			err := enableProxyInMacOS()
			if err != nil {
				fmt.Printf("\nERROR 设置代理失败 %v\n", err.Error())
				fmt.Printf("按 Ctrl+C 退出...\n")
				select {}
			}
		}
		color.Green(fmt.Sprintf("\n\n服务已正确启动，请打开需要下载的视频号页面进行下载"))
	} else {
		fmt.Println(fmt.Sprintf("\n\n您还未安装证书，请在浏览器打开 http://%v 并根据说明安装证书\n在安装完成后重新启动此程序即可\n", proxy_server))
	}
	fmt.Println("\n\n服务正在运行，按 Ctrl+C 退出...")
	select {}
}

type ChannelProfile struct {
	Title string `json:"title"`
}
type FrontendTip struct {
	Msg string `json:"msg"`
}

func HttpCallback(Conn *SunnyNet.HttpConn) {
	host := Conn.Request.URL.Hostname()
	path := Conn.Request.URL.Path
	if Conn.Type == public.HttpSendRequest {
		// Conn.Request.Header.Set("Cache-Control", "no-cache")
		Conn.Request.Header.Del("Accept-Encoding")
		if Includes(path, "jszip") {
			headers := http.Header{}
			headers.Set("Content-Type", "application/javascript")
			headers.Set("__debug", "local_file")
			Conn.StopRequest(200, zip_js, headers)
			return
		}
		if Includes(path, "FileSaver.min") {
			headers := http.Header{}
			headers.Set("Content-Type", "application/javascript")
			headers.Set("__debug", "local_file")
			Conn.StopRequest(200, file_saver_js, headers)
			return
		}
		if path == "/__wx_channels_api/profile" {
			var data ChannelProfile
			body, _ := io.ReadAll(Conn.Request.Body)
			_ = Conn.Request.Body.Close()
			err := json.Unmarshal(body, &data)
			if err != nil {
				fmt.Println(err.Error())
			}
			fmt.Printf("\n%s\n", data.Title)
			headers := http.Header{}
			headers.Set("Content-Type", "application/json")
			headers.Set("__debug", "fake_resp")
			Conn.StopRequest(200, "{}", headers)
			return
		}
		if path == "/__wx_channels_api/tip" {
			var data FrontendTip
			body, _ := io.ReadAll(Conn.Request.Body)
			_ = Conn.Request.Body.Close()
			err := json.Unmarshal(body, &data)
			if err != nil {
				fmt.Println(err.Error())
			}
			fmt.Printf("[FRONTEND]%s\n", data.Msg)
			headers := http.Header{}
			headers.Set("Content-Type", "application/json")
			headers.Set("__debug", "fake_resp")
			Conn.StopRequest(200, "{}", headers)
			return
		}
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
				fmt.Println("\n\n检测到页面打开")
				fmt.Println(path)
				html := string(Body)
				script_reg1 := regexp.MustCompile(`src="([^"]{1,})\.js"`)
				html = script_reg1.ReplaceAllString(html, `src="$1.js`+v+`"`)
				script_reg2 := regexp.MustCompile(`href="([^"]{1,})\.js"`)
				html = script_reg2.ReplaceAllString(html, `href="$1.js`+v+`"`)
				Conn.Response.Header.Set("__debug", "append_script")
				script2 := ""
				// script2 := `<script src="https://debug.funzm.com/target.js"></script>`
				// 				script2 := `<script
				//       crossorigin="anonymous"
				//       src="https://pagespy.jikejishu.com/page-spy/index.min.js"
				//     ></script>
				//     <script
				//       crossorigin="anonymous"
				//       src="https://pagespy.jikejishu.com/plugin/data-harbor/index.min.js"
				//     ></script>
				//     <script
				//       crossorigin="anonymous"
				//       src="https://pagespy.jikejishu.com/plugin/rrweb/index.min.js"
				//     ></script>
				//     <!-- 使用第二步：实例化 PageSpy -->
				//     <script>
				//       window.$harbor = new DataHarborPlugin();
				//       window.$rrweb = new RRWebPlugin();
				//       [window.$harbor, window.$rrweb].forEach((p) => {
				//         PageSpy.registerPlugin(p);
				//       });

				//       // 实例化的参数都是可选的
				//       window.$pageSpy = new PageSpy({
				//         api: "pagespy.jikejishu.com",
				//         clientOrigin: "https://pagespy.jikejishu.com",
				//         project: "React 演示",
				//         autoRender: true,
				//         title: "PageSpy 🤝 React",
				//       });
				//       // 之后即可使用 PageSpy，前往 https://pagespy.jikejishu.com 体验
				//     </script>`

				if host == "channels.weixin.qq.com" && (path == "/web/pages/feed" || path == "/web/pages/home") {
					// Conn.Response.Header.Add("wx-channel-video-download", "1")
					script := fmt.Sprintf(`<script>%s</script>`, main_js)
					html = strings.Replace(html, "<head>", "<head>\n"+script+script2, 1)
					fmt.Println("1. 视频详情页 html 注入 js 成功")
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

				if Includes(path, "/t/wx_fed/finder/web/web-finder/res/js/index.publish") {
					regexp1 := regexp.MustCompile(`this.sourceBuffer.appendBuffer\(h\),`)
					replaceStr1 := `(() => {
if (window.__wx_channels_store__) {
window.__wx_channels_store__.buffers.push(h);
}
})(),this.sourceBuffer.appendBuffer(h),`
					if regexp1.MatchString(content) {
						fmt.Println("2. 视频播放 js 修改成功")
					}
					content = regexp1.ReplaceAllString(content, replaceStr1)
					regexp2 := regexp.MustCompile(`if\(f.cmd===re.MAIN_THREAD_CMD.AUTO_CUT`)
					replaceStr2 := `if(f.cmd===re.MAIN_THREAD_CMD.AUTO_CUT) {
}
if(f.cmd==="CUT"){
	console.log(f);
	if (window.__wx_channels_store__) {
	window.__wx_channels_store__.keys[f.seed]=f.decryptor_array;
	}
}
if(f.cmd===re.MAIN_THREAD_CMD.AUTO_CUT`
					content = regexp2.ReplaceAllString(content, replaceStr2)
					Conn.Response.Body = io.NopCloser(bytes.NewBuffer([]byte(content)))
					return
				}
				if Includes(path, "/t/wx_fed/finder/web/web-finder/res/js/virtual_svg-icons-register") {
					regexp1 := regexp.MustCompile(`async finderGetCommentDetail\((\w+)\)\{return(.*?)\}async`)
					replaceStr1 := `async finderGetCommentDetail($1) {
					var feedResult = await$2;
					var data_object = feedResult.data.object;
					if (!data_object.objectDesc) {
						return feedResult;
					}
					var media = data_object.objectDesc.media[0];
					var profile = media.mediaType !== 4 ? {
						type: "picture",
						id: data_object.id,
						title: data_object.objectDesc.description,
						files: data_object.objectDesc.media,
						spec: [],
						contact: data_object.contact
					} : {
						type: "media",
						duration: media.spec[0].durationMs,
						spec: media.spec,
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
					fetch("/__wx_channels_api/profile", {
						method: "POST",
						headers: {
							"Content-Type": "application/json"
						},
						body: JSON.stringify(profile)
					});
					if (window.__wx_channels_store__) {
					__wx_channels_store__.profile = profile;
					window.__wx_channels_store__.profiles.push(profile);
					}
					return feedResult;
				}async`
					if regexp1.MatchString(content) {
						fmt.Println("3. 视频详情页 js 修改成功")
					}
					content = regexp1.ReplaceAllString(content, replaceStr1)
					regex2 := regexp.MustCompile(`r.default={dialog`)
					replaceStr2 := `r.default=window.window.__wx_channels_tip__={dialog`
					content = regex2.ReplaceAllString(content, replaceStr2)
					regex3 := regexp.MustCompile(`const u=this.storage.getSession`)
					replaceStr3 := `return;const u = this.storage.getSession`
					content = regex3.ReplaceAllString(content, replaceStr3)
					regex4 := regexp.MustCompile(`return this.storage.getSession`)
					replaceStr4 := `return null;return this.storage.getSession`
					content = regex4.ReplaceAllString(content, replaceStr4)
					regex5 := regexp.MustCompile(`this.updateDetail\(o\)`)
					replaceStr5 := `(() => {
					if (Object.keys(o).length===0){
					return;
					}
					var data_object = o;
					var media = data_object.objectDesc.media[0];
					var profile = media.mediaType !== 4 ? {
						type: "picture",
						id: data_object.id,
						title: data_object.objectDesc.description,
						files: data_object.objectDesc.media,
						spec: [],
						contact: data_object.contact
					} : {
						type: "media",
						duration: media.spec[0].durationMs,
						spec: media.spec,
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
					if (window.__wx_channels_store__) {
window.__wx_channels_store__.profiles.push(profile);
					}
					})(),this.updateDetail(o)`
					content = regex5.ReplaceAllString(content, replaceStr5)
					Conn.Response.Body = io.NopCloser(bytes.NewBuffer([]byte(content)))
					return
				}
				if Includes(path, "/t/wx_fed/finder/web/web-finder/res/js/FeedDetail.publish") {
					regex := regexp.MustCompile(`,"投诉"\)]`)
					replaceStr := `,"投诉"),...(() => {
					if (window.__wx_channels_store__ && window.__wx_channels_store__.profile) {
						return window.__wx_channels_store__.profile.spec.map((sp) => {
							return p("div",{class:"context-item",role:"button",onClick:() => __wx_channels_handle_click_download__(sp)},sp.fileFormat);
						});
					}
					})(),p("div",{class:"context-item",role:"button",onClick:()=>__wx_channels_handle_click_download__()},"原始视频"),p("div",{class:"context-item",role:"button",onClick:__wx_channels_download_cur__},"当前视频"),p("div",{class:"context-item",role:"button",onClick:__wx_channels_handle_copy__},"复制链接"),p("div",{class:"context-item",role:"button",onClick:__wx_channels_handle_log__},"下载日志")]`
					content = regex.ReplaceAllString(content, replaceStr)
					Conn.Response.Body = io.NopCloser(bytes.NewBuffer([]byte(content)))
					return
				}
				if Includes(path, "worker_release") {
					regex := regexp.MustCompile(`fmp4Index:p.fmp4Index`)
					replaceStr := `decryptor_array:p.decryptor_array,fmp4Index:p.fmp4Index`
					content = regex.ReplaceAllString(content, replaceStr)
					Conn.Response.Body = io.NopCloser(bytes.NewBuffer([]byte(content)))
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
