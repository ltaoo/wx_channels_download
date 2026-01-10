package proxy

import (
	"io"
	"net/http"
)

type InnerProxy interface {
	Start(port int) error
	AddPlugin(plugin interface{})
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

type Plugin struct {
	Match      string
	Target     *TargetConfig
	OnRequest  func(ctx Context)
	OnResponse func(ctx Context)
}

type TargetConfig struct {
	Protocol string
	Host     string
	Port     int
}

type Context interface {
	Req() *ContextReq
	Res() *ContextRes
	Mock(status int, headers map[string]string, body string)
	GetResponseHeader(key string) string
	SetResponseHeader(key, val string)
	SetResponseBody(body string)
	GetResponseBody() ([]byte, error)
	SetStatusCode(code int)
}

type contextImpl struct {
	Req  *http.Request
	Res  *http.Response
	impl any
}
type ContextReq struct {
	URL     *ContextURL
	Body    io.Reader
	Header  http.Header
	Cookies []*http.Cookie
}
type ContextURL struct {
	Path     string
	Hostname func() string
	RawQuery string
}
type ContextRes struct {
	Body       io.ReadCloser
	Header     http.Header
	StatusCode int
}
