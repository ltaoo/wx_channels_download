package interceptor

import (
	"fmt"
	"strconv"
	"wx_channel/internal/manager"
	"wx_channel/pkg/certificate"
)

type InterceptorServer struct {
	*manager.HTTPServer
	Interceptor *Interceptor
}

func NewInterceptorServer(settings *InterceptorSettings, cert *certificate.CertFileAndKeyFile) *InterceptorServer {
	interceptor := NewInterceptor(settings, cert)
	addr := settings.ProxyServerHostname + ":" + strconv.Itoa(settings.ProxyServerPort)
	srv := manager.NewHTTPServer("代理服务", "interceptor", addr)
	srv.SetHandler(interceptor)

	return &InterceptorServer{
		HTTPServer:  srv,
		Interceptor: interceptor,
	}
}

func (s *InterceptorServer) Start() error {
	if err := s.Interceptor.Start(); err != nil {
		return fmt.Errorf("failed to start interceptor: %v", err)
	}
	return s.HTTPServer.Start()
}

func (s *InterceptorServer) Stop() error {
	// 先关闭代理设置，防止新流量进入
	if err := s.Interceptor.Stop(); err != nil {
		return fmt.Errorf("failed to stop interceptor: %v", err)
	}
	return s.HTTPServer.Stop()
}
