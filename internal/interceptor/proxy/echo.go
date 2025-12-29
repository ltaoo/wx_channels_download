//go:build !sunnynet
// +build !sunnynet

package proxy

import (
	"net/http"

	"github.com/ltaoo/echo"
)

type EchoProxy struct {
	echo *echo.Echo
}

func NewProxy(cert []byte, private_key []byte) (InnerProxy, error) {
	e, err := echo.NewEcho(cert, private_key)
	if err != nil {
		return nil, err
	}
	return &EchoProxy{echo: e}, nil
}

func (p *EchoProxy) Start(port int) error {
	return nil
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
		Header: nil,
	}
}

func (ctx *EchoContext) Res() *ContextRes {
	return &ContextRes{
		Body:       nil,
		Header:     nil,
		StatusCode: 0,
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
