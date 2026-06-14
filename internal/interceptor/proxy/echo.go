//go:build !sunnynet
// +build !sunnynet

package proxy

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ltaoo/echo"
	"github.com/ltaoo/echo/tun"
)

type EchoProxy struct {
	echo           *echo.Echo
	proxyHostname  string
	tcpRelayConfig *TCPRelayConfig
}

func NewProxy(cert []byte, private_key []byte, upstreamProxy string, tunEnabled bool, proxyHostname string, proxyPort int, defaultInterface string, tcpRelayConfig *TCPRelayConfig) (InnerProxy, error) {
	opts := &echo.Options{
		EnableBuiltinBypass:  false,
		InterceptOnlyMatched: true,
		UpstreamProxy:        upstreamProxy,
	}
	if tunEnabled {
		defaultInterface = strings.TrimSpace(defaultInterface)
		opts.Tun = true
		opts.TunConfig = tun.DefaultConfig()
		opts.TunConfig.Inbound.AutoRoute = true
		opts.TunConfig.Inbound.StrictRoute = true
		// Set the proxy outbound to point to our own proxy port
		for i := range opts.TunConfig.Outbounds {
			if opts.TunConfig.Outbounds[i].Tag == "proxy" {
				opts.TunConfig.Outbounds[i].Port = uint16(proxyPort)
			}
		}
		// Configure routing rules (evaluated in order)
		opts.TunConfig.Route = tun.RouteConfig{
			Rules: []tun.RuleConfig{
				// Highest priority: self-process direct to avoid loopback
				{
					ProcessName: selfProcessNames(),
					Outbound:    "direct",
				},
				// WeChat processes through proxy
				{
					ProcessName: []string{"WeChat", "WeChatAppEx", "WeChatAppEx.exe", "Weixin.exe", "WeChatAppEx Helper"},
					Outbound:    "proxy",
				},
				// qq.com domains through proxy
				{
					DomainSuffix: []string{"qq.com"},
					Outbound:     "proxy",
				},
			},
			Final:            "direct",
			DefaultInterface: defaultInterface,
		}
	}
	e, err := echo.NewEchoWithOptions(cert, private_key, opts)
	if err != nil {
		return nil, err
	}
	return &EchoProxy{
		echo:           e,
		proxyHostname:  proxyHostname,
		tcpRelayConfig: tcpRelayConfig,
	}, nil
}

func selfProcessNames() []string {
	names := []string{"wx_video_download", "wx_video_download.exe", "wx_channel", "wx_channel.exe", "go", "go.exe", "main", "main.exe"}
	addName := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		for _, existing := range names {
			if existing == name {
				return
			}
		}
		names = append(names, name)
	}
	if exe, err := os.Executable(); err == nil {
		addName(filepath.Base(exe))
	}
	addName(filepath.Base(os.Args[0]))
	return names
}

func (p *EchoProxy) Start(port int) error {
	if p.tcpRelayConfig == nil || !p.tcpRelayConfig.Enabled {
		return nil
	}
	relayHost := strings.TrimSpace(p.tcpRelayConfig.Hostname)
	if relayHost == "" {
		relayHost = "127.0.0.1"
	}
	relayPort := p.tcpRelayConfig.Port
	if relayPort <= 0 {
		return fmt.Errorf("tcp relay port must be greater than 0")
	}
	echoHost := normalizeDialHost(p.proxyHostname)
	relayAddr := net.JoinHostPort(relayHost, strconv.Itoa(relayPort))
	echoAddr := net.JoinHostPort(echoHost, strconv.Itoa(port))
	if relayAddr == echoAddr {
		return fmt.Errorf("tcp relay address must be different from proxy address: %s", relayAddr)
	}
	return p.echo.ListenTCP(relayAddr, echoAddr)
}

func normalizeDialHost(host string) string {
	host = strings.TrimSpace(host)
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		return "127.0.0.1"
	default:
		return host
	}
}

func (p *EchoProxy) Close() error {
	return p.echo.Close()
}

func (p *EchoProxy) AddPlugin(plugin interface{}) {
	switch pl := plugin.(type) {
	case *echo.Plugin:
		p.echo.AddPlugin(pl)
	case *Plugin:
		p.echo.AddPlugin(toEchoPlugin(pl))
	}
}

func (p *EchoProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.echo.ServeHTTP(w, r)
}

type EchoContext struct {
	impl *echo.Context
}

func toEchoPlugin(p *Plugin) *echo.Plugin {
	return &echo.Plugin{
		Match:  p.Match,
		Target: toEchoTarget(p.Target),
		OnRequest: func(c *echo.Context) {
			ctx := &EchoContext{impl: c}
			if p.OnRequest != nil {
				p.OnRequest(ctx)
			}
		},
		OnResponse: func(c *echo.Context) {
			ctx := &EchoContext{impl: c}
			if p.OnResponse != nil {
				p.OnResponse(ctx)
			}
		},
	}
}

func toEchoTarget(t *TargetConfig) *echo.TargetConfig {
	if t == nil {
		return nil
	}
	return &echo.TargetConfig{
		Protocol: t.Protocol,
		Host:     t.Host,
		Port:     t.Port,
	}
}

func (ctx *EchoContext) Req() *ContextReq {
	c := ctx.impl
	return &ContextReq{
		URL: &ContextURL{
			Path:     c.Req.URL.Path,
			Hostname: func() string { return c.Req.URL.Hostname() },
			RawQuery: c.Req.URL.RawQuery,
		},
		Body:   c.Req.Body,
		Header: c.Req.Header,
	}
}

func (ctx *EchoContext) Res() *ContextRes {
	return &ContextRes{
		Body:       ctx.impl.Res.Body,
		Header:     ctx.impl.Res.Header,
		StatusCode: ctx.impl.Res.StatusCode,
	}
}

func (ctx *EchoContext) Mock(status int, headers map[string]string, body string) {
	ctx.impl.Mock(status, headers, body)
}

func (ctx *EchoContext) GetResponseHeader(key string) string {
	return ctx.impl.GetResponseHeader(key)
}

func (ctx *EchoContext) SetResponseHeader(key, val string) {
	ctx.impl.SetResponseHeader(key, val)
}

func (ctx *EchoContext) SetResponseBody(body string) {
	ctx.impl.SetResponseBody(body)
}

func (ctx *EchoContext) GetResponseBody() ([]byte, error) {
	body, err := ctx.impl.GetResponseBody()
	return []byte(body), err
}

func (ctx *EchoContext) SetStatusCode(code int) {
	ctx.impl.Res.StatusCode = code
}
