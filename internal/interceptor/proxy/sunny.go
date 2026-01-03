//go:build sunnynet
// +build sunnynet

package proxy

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	h "net/http"
	"net/url"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/andybalholm/brotli"
	"github.com/qtgolang/SunnyNet/SunnyNet"
	"github.com/qtgolang/SunnyNet/src/http"
	"github.com/qtgolang/SunnyNet/src/public"
	"golang.org/x/text/encoding/simplifiedchinese"
)

type SunnyNetProxy struct {
	Sunny    *SunnyNet.Sunny
	callback func(conn SunnyNet.ConnHTTP)
	plugins  []interface{}
}

func NewProxy(cert []byte, private_key []byte) (InnerProxy, error) {
	Sunny := SunnyNet.NewSunny()
	return &SunnyNetProxy{Sunny: Sunny}, nil
}

func (p *SunnyNetProxy) Start(port int) error {
	p.Sunny.SetGoCallback(p.HandleHTTPRequest, nil, p.HandleWS, nil)
	p.Sunny.SetPort(port).Start()
	err := p.Sunny.Error
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		success := p.Sunny.OpenDrive(0)
		if success {
			fmt.Println("进程代理驱动启动成功")
			p.Sunny.ProcessAddName("WeChatAppEx.exe")
		} else {
			fmt.Println("进程代理驱动启动失败，使用系统代理")
		}
	}
	return nil
}

func (p *SunnyNetProxy) AddPlugin(plugin interface{}) {
	switch pl := plugin.(type) {
	case *SunnyNetPlugin:
		p.plugins = append(p.plugins, pl)
	case *Plugin:
		p.plugins = append(p.plugins, toSunnyPlugin(pl))
	}

}

type SunnyNetPlugin struct {
	Match      string
	Target     *SunnyNetTargetConfig
	OnRequest  func(ctx *SunnyNetContext)
	OnResponse func(ctx *SunnyNetContext)
}
type SunnyNetTargetConfig struct {
	Protocol string
	Host     string
	Port     int
}
type SunnyNetContext struct {
	Req               func() *SunnyNetContextReq
	Res               func() *SunnyNetContextRes
	Mock              func(status_code int, headers map[string]string, body []byte)
	GetResponseHeader func(key string) string
	SetResponseHeader func(key, val string)
	SetResponseBody   func(body string)
	GetResponseBody   func() ([]byte, error)
	SetStatusCode     func(code int)
}

type SunnyNetContextReq struct {
	URL  SunnyNetContextURL
	Body []byte
}
type SunnyNetContextURL struct {
	Hostname func() string
	Pathname string
	RawQuery string
}
type SunnyNetContextRes struct {
	Header http.Header
}

type sunnyBridgeContext struct {
	impl *SunnyNetContext
}

func toSunnyPlugin(p *Plugin) *SunnyNetPlugin {
	return &SunnyNetPlugin{
		Match:  p.Match,
		Target: toSunnyTarget(p.Target),
		OnRequest: func(c *SunnyNetContext) {
			if p.OnRequest != nil {
				p.OnRequest(&sunnyBridgeContext{impl: c})
			}
		},
		OnResponse: func(c *SunnyNetContext) {
			if p.OnResponse != nil {
				p.OnResponse(&sunnyBridgeContext{impl: c})
			}
		},
	}
}

func (p *SunnyNetProxy) HandleWS(Conn SunnyNet.ConnWebSocket) {
	switch Conn.Type() {
	case public.WebsocketUserSend:
		_ = Conn.SendToServer(Conn.MessageType(), Conn.Body())
	case public.WebsocketServerSend:
		_ = Conn.SendToClient(Conn.MessageType(), Conn.Body())
	}
}

func toSunnyTarget(t *TargetConfig) *SunnyNetTargetConfig {
	if t == nil {
		return nil
	}
	return &SunnyNetTargetConfig{
		Protocol: t.Protocol,
		Host:     t.Host,
		Port:     t.Port,
	}
}

func (c *sunnyBridgeContext) Req() *ContextReq {
	if c.impl == nil || c.impl.Req == nil {
		return &ContextReq{
			URL: &ContextURL{
				Path:     "",
				Hostname: func() string { return "" },
				RawQuery: "",
			},
			Body:   nil,
			Header: nil,
		}
	}
	r := c.impl.Req()
	return &ContextReq{
		URL: &ContextURL{
			Path:     r.URL.Pathname,
			Hostname: r.URL.Hostname,
			RawQuery: r.URL.RawQuery,
		},
		Body:   bytes.NewReader(r.Body),
		Header: nil,
	}
}

func (c *sunnyBridgeContext) Res() *ContextRes {
	hdr := make(h.Header)
	if c.impl != nil && c.impl.Res != nil {
		if rr := c.impl.Res(); rr != nil {
			// SunnyNet 的 Header 类型与 net/http.Header 不同，保持为空以避免类型不匹配
			_ = rr
		}
	}
	return &ContextRes{
		Body:       nil,
		Header:     hdr,
		StatusCode: 0,
	}
}

func (c *sunnyBridgeContext) Mock(status int, headers map[string]string, body string) {
	if c.impl != nil && c.impl.Mock != nil {
		c.impl.Mock(status, headers, []byte(body))
	}
}

func (c *sunnyBridgeContext) GetResponseHeader(key string) string {
	if c.impl != nil && c.impl.GetResponseHeader != nil {
		return c.impl.GetResponseHeader(key)
	}
	return ""
}

func (c *sunnyBridgeContext) SetResponseHeader(key, val string) {
	if c.impl != nil && c.impl.SetResponseHeader != nil {
		c.impl.SetResponseHeader(key, val)
	}
}

func (c *sunnyBridgeContext) SetResponseBody(body string) {
	if c.impl != nil && c.impl.SetResponseBody != nil {
		c.impl.SetResponseBody(body)
	}
}

func (c *sunnyBridgeContext) SetStatusCode(code int) {
	if c.impl != nil && c.impl.SetStatusCode != nil {
		c.impl.SetStatusCode(code)
	}
}

func (c *sunnyBridgeContext) GetResponseBody() ([]byte, error) {
	if c.impl != nil && c.impl.GetResponseBody != nil {
		return c.impl.GetResponseBody()
	}
	return nil, nil
}

func hostMatches(host, pattern string) bool {
	if pattern == "" {
		return true
	}
	h := strings.ToLower(host)
	p := strings.ToLower(pattern)
	if h == p {
		return true
	}
	if strings.HasSuffix(h, "."+p) {
		return true
	}
	if strings.Contains(h, p) {
		return true
	}
	return false
}

func (p *SunnyNetProxy) HandleHTTPRequest(Conn SunnyNet.ConnHTTP) {
	switch Conn.Type() {
	case public.HttpSendRequest:
		ctx := &SunnyNetContext{
			Mock: func(status_code int, headers map[string]string, body []byte) {
				hh := http.Header{}
				for k, v := range headers {
					hh.Set(k, v)
				}
				Conn.StopRequest(status_code, body, hh)
			},
		}
		ctx.Req = func() *SunnyNetContextReq {
			u := Conn.URL()
			parsed_url, _ := url.Parse(u)
			// hostname := parsed_url.Hostname()
			// pathname := parsed_url.Pathname()
			body := Conn.GetRequestBody()
			req := SunnyNetContextReq{
				URL: SunnyNetContextURL{
					Hostname: func() string { return parsed_url.Hostname() },
					Pathname: parsed_url.Path,
					RawQuery: parsed_url.RawQuery,
				},
				Body: body,
			}
			return &req
		}
		u := Conn.URL()
		parsed, _ := url.Parse(u)
		host := ""
		if parsed != nil {
			host = parsed.Hostname()
		}
		for _, plugin := range p.plugins {
			switch pl := plugin.(type) {
			case *SunnyNetPlugin:
				if hostMatches(host, pl.Match) {
					if pl.Target != nil {
						targetProto := strings.ToLower(pl.Target.Protocol)
						if targetProto == "" {
							targetProto = "http"
						}
						if targetProto == "ws" {
							targetProto = "http"
						}
						if targetProto == "wss" {
							targetProto = "https"
						}
						targetHost := pl.Target.Host
						targetPort := pl.Target.Port
						if targetPort <= 0 {
							if targetProto == "https" {
								targetPort = 443
							} else {
								targetPort = 80
							}
						}
						u := Conn.URL()
						parsedURL, _ := url.Parse(u)
						path := ""
						rawQuery := ""
						if parsedURL != nil {
							path = parsedURL.Path
							rawQuery = parsedURL.RawQuery
						}
						targetURL := fmt.Sprintf("%s://%s:%d%s", targetProto, targetHost, targetPort, path)
						if rawQuery != "" {
							targetURL = targetURL + "?" + rawQuery
						}
						Conn.UpdateURL(targetURL)
						return
					}
					pl.OnRequest(ctx)
				}
			}
		}
		return
	case public.HttpResponseOK: // 请求完成
		ctx := &SunnyNetContext{
			GetResponseHeader: func(key string) string {
				return Conn.GetResponseHeader().Get(key)
			},
			GetResponseBody: func() ([]byte, error) {
				b := Conn.GetResponseBody()
				enc := strings.ToLower(Conn.GetResponseHeader().Get("Content-Encoding"))
				if strings.Contains(enc, "br") {
					r := brotli.NewReader(bytes.NewReader(b))
					if rb, err := io.ReadAll(r); err == nil {
						b = rb
					}
				} else if strings.Contains(enc, "gzip") {
					r, err := gzip.NewReader(bytes.NewReader(b))
					if err == nil {
						if rb, err2 := io.ReadAll(r); err2 == nil {
							b = rb
						}
						r.Close()
					}
				} else if strings.Contains(enc, "deflate") {
					r, err := zlib.NewReader(bytes.NewReader(b))
					if err == nil {
						if rb, err2 := io.ReadAll(r); err2 == nil {
							b = rb
						}
						r.Close()
					}
				}
				ct := strings.ToLower(Conn.GetResponseHeader().Get("Content-Type"))
				cs := ""
				if idx := strings.Index(ct, "charset="); idx != -1 {
					cs = strings.Trim(strings.TrimSpace(ct[idx+8:]), "\"'")
				}
				if cs != "" && cs != "utf-8" && cs != "utf8" {
					switch {
					case strings.Contains(cs, "gbk"):
						if rb, err := decodeGBK(b); err == nil {
							b = rb
						}
					case strings.Contains(cs, "gb2312"):
						if rb, err := decodeGB2312(b); err == nil {
							b = rb
						}
					case strings.Contains(cs, "gb18030"):
						if rb, err := decodeGB18030(b); err == nil {
							b = rb
						}
					}
				} else {
					if !utf8.Valid(b) {
						if rb, err := decodeGB18030(b); err == nil {
							b = rb
						}
					}
				}
				return b, nil
			},
			// 为响应阶段也提供请求信息，方便插件获取 Hostname/Path 等
			Req: func() *SunnyNetContextReq {
				u := Conn.URL()
				parsed_url, _ := url.Parse(u)
				req := SunnyNetContextReq{
					URL: SunnyNetContextURL{
						Hostname: func() string { return parsed_url.Hostname() },
						Pathname: parsed_url.Path,
						RawQuery: parsed_url.RawQuery,
					},
					Body: nil,
				}
				return &req
			},
			Res: func() *SunnyNetContextRes {
				res := SunnyNetContextRes{
					Header: Conn.GetResponseHeader(),
				}
				return &res
			},
			SetResponseBody: func(content string) {
				hdr := Conn.GetResponseHeader()
				hdr.Del("Content-Encoding")
				hdr.Del("Content-Length")
				Conn.SetResponseBodyIO(io.NopCloser(bytes.NewBuffer([]byte(content))))
			},
		}
		u := Conn.URL()
		parsed, _ := url.Parse(u)
		host := ""
		if parsed != nil {
			host = parsed.Hostname()
		}
		for _, plugin := range p.plugins {
			switch pl := plugin.(type) {
			case *SunnyNetPlugin:
				if hostMatches(host, pl.Match) {
					pl.OnResponse(ctx)
				}
			}
		}
		return
	}
}

func (p *SunnyNetProxy) ServeHTTP(w h.ResponseWriter, r *h.Request) {
}

func decodeGBK(b []byte) ([]byte, error) {
	return simplifiedchinese.GBK.NewDecoder().Bytes(b)
}

func decodeGB2312(b []byte) ([]byte, error) {
	return simplifiedchinese.HZGB2312.NewDecoder().Bytes(b)
}

func decodeGB18030(b []byte) ([]byte, error) {
	return simplifiedchinese.GB18030.NewDecoder().Bytes(b)
}
