//go:build sunnynet
// +build sunnynet

package main

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/andybalholm/brotli"
	"github.com/qtgolang/SunnyNet/SunnyNet"
	"github.com/qtgolang/SunnyNet/src/public"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// 配置
const (
	// WebSocket 转发配置
	wsSourceHost = "remoteapi.weixin.qq.com"       // 前端连接的假域名
	wsTargetHost = "192.168.1.118"   // 实际 WebSocket 服务器
	wsTargetPort = 2022              // 实际 WebSocket 端口

	// 代理端口
	proxyPort = 2023
)

func main() {
	sunny := SunnyNet.NewSunny()

	// 设置回调: HTTP, TCP, WebSocket, UDP
	sunny.SetGoCallback(handleHTTP, nil, handleWebSocket, nil)

	// 启动代理
	sunny.SetPort(proxyPort).Start()
	if sunny.Error != nil {
		log.Fatalf("启动代理失败: %v", sunny.Error)
	}

	fmt.Printf("SunnyNet 代理已启动，端口: %d\n", proxyPort)
	fmt.Printf("WebSocket 转发: wss://%s -> ws://%s:%d\n", wsSourceHost, wsTargetHost, wsTargetPort)

	// Windows 进程代理
	ok := sunny.OpenDrive(0)
	if !ok {
		fmt.Println("进程代理驱动启动失败，请以管理员身份运行")
	} else {
		sunny.ProcessAddName("WeChatAppEx.exe")
		fmt.Println("已添加进程代理: WeChatAppEx.exe")
	}

	// 等待退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\n正在关闭...")
	sunny.Close()
}

func handleHTTP(conn SunnyNet.ConnHTTP) {
	u := conn.URL()
	parsed, err := url.Parse(u)
	if err != nil {
		return
	}
	host := parsed.Hostname()

	switch conn.Type() {
	case public.HttpSendRequest:
		// 拦截 wss://kf.qq.com 的 WebSocket 升级请求，转发到实际服务器
		if strings.EqualFold(host, wsSourceHost) {
			// 关键：使用 ws:// 协议，SunnyNet 会自动处理 WebSocket 升级
			targetURL := fmt.Sprintf("ws://%s:%d%s", wsTargetHost, wsTargetPort, parsed.Path)
			if parsed.RawQuery != "" {
				targetURL += "?" + parsed.RawQuery
			}

			conn.UpdateURL(targetURL)
			log.Printf("[HTTP] WebSocket 转发: %s -> %s", u, targetURL)
		}

	case public.HttpResponseOK:
		// 拦截 channels.weixin.qq.com 的 HTML 响应，注入脚本
		if strings.Contains(host, "channels.weixin.qq.com") {
			contentType := conn.GetResponseHeader().Get("Content-Type")
			if strings.Contains(strings.ToLower(contentType), "text/html") {
				injectScript(conn)
			}
		}
	}
}

func handleWebSocket(conn SunnyNet.ConnWebSocket) {
	switch conn.Type() {
	case public.WebsocketUserSend:
		// 客户端 -> 服务器
		body := conn.Body()
		log.Printf("[WS] 客户端 -> 服务器: %d bytes", len(body))
		_ = conn.SendToServer(conn.MessageType(), body)

	case public.WebsocketServerSend:
		// 服务器 -> 客户端
		body := conn.Body()
		log.Printf("[WS] 服务器 -> 客户端: %d bytes", len(body))
		_ = conn.SendToClient(conn.MessageType(), body)
	}
}

func injectScript(conn SunnyNet.ConnHTTP) {
	body := conn.GetResponseBody()

	// 解压缩
	enc := strings.ToLower(conn.GetResponseHeader().Get("Content-Encoding"))
	body = decompress(body, enc)

	// 解码字符集
	ct := strings.ToLower(conn.GetResponseHeader().Get("Content-Type"))
	body = decodeCharset(body, ct)

	// 注入脚本
	script := fmt.Sprintf(`
<script>
console.log("Injected by SunnyNet");
var ws = new WebSocket("wss://%s/ws/channels");
ws.onopen = function() {
    console.log("Connected to remote API");
};
ws.onmessage = function(evt) {
    console.log("Message from remote: " + evt.data);
};
ws.onerror = function(err) {
    console.error("WebSocket error:", err);
};
ws.onclose = function() {
    console.log("WebSocket closed");
};
</script>
`, wsSourceHost)

	newBody := string(body) + script

	// 移除压缩相关头，设置正确的 Content-Type
	hdr := conn.GetResponseHeader()
	hdr.Del("Content-Encoding")
	hdr.Del("Content-Length")
	hdr.Set("Content-Type", "text/html; charset=utf-8")

	conn.SetResponseBodyIO(io.NopCloser(bytes.NewBufferString(newBody)))
	log.Printf("[HTTP] 已注入脚本到: %s", conn.URL())
}

func decompress(body []byte, encoding string) []byte {
	if strings.Contains(encoding, "br") {
		r := brotli.NewReader(bytes.NewReader(body))
		if rb, err := io.ReadAll(r); err == nil {
			return rb
		}
	} else if strings.Contains(encoding, "gzip") {
		r, err := gzip.NewReader(bytes.NewReader(body))
		if err == nil {
			defer r.Close()
			if rb, err := io.ReadAll(r); err == nil {
				return rb
			}
		}
	} else if strings.Contains(encoding, "deflate") {
		r, err := zlib.NewReader(bytes.NewReader(body))
		if err == nil {
			defer r.Close()
			if rb, err := io.ReadAll(r); err == nil {
				return rb
			}
		}
	}
	return body
}

func decodeCharset(body []byte, contentType string) []byte {
	cs := ""
	if idx := strings.Index(contentType, "charset="); idx != -1 {
		cs = strings.Trim(strings.TrimSpace(contentType[idx+8:]), "\"'")
	}

	if cs != "" && cs != "utf-8" && cs != "utf8" {
		switch {
		case strings.Contains(cs, "gbk"):
			if rb, err := simplifiedchinese.GBK.NewDecoder().Bytes(body); err == nil {
				return rb
			}
		case strings.Contains(cs, "gb2312"):
			if rb, err := simplifiedchinese.HZGB2312.NewDecoder().Bytes(body); err == nil {
				return rb
			}
		case strings.Contains(cs, "gb18030"):
			if rb, err := simplifiedchinese.GB18030.NewDecoder().Bytes(body); err == nil {
				return rb
			}
		}
	} else if !utf8.Valid(body) {
		if rb, err := simplifiedchinese.GB18030.NewDecoder().Bytes(body); err == nil {
			return rb
		}
	}
	return body
}
