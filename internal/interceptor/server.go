package interceptor

import (
	"fmt"
	"strconv"
	"wx_channel/internal/manager"
)

type InterceptorServer struct {
	*manager.HTTPServer
	interceptor *Interceptor
}

func NewInterceptorServer(config InterceptorConfig) (*InterceptorServer, error) {
	interceptor, err := NewInterceptor(config)
	if err != nil {
		return nil, err
	}
	addr := config.Hostname + ":" + strconv.Itoa(config.Port)
	srv := manager.NewHTTPServer("代理服务", "interceptor", addr)
	srv.SetHandler(interceptor)

	return &InterceptorServer{
		HTTPServer:  srv,
		interceptor: interceptor,
	}, nil
}

func (s *InterceptorServer) Start() error {
	if err := s.interceptor.Start(); err != nil {
		return fmt.Errorf("failed to start interceptor: %v", err)
	}
	return s.HTTPServer.Start()
}

func (s *InterceptorServer) Stop() error {
	// 先关闭代理设置，防止新流量进入
	if err := s.interceptor.Stop(); err != nil {
		return fmt.Errorf("failed to stop interceptor: %v", err)
	}
	return s.HTTPServer.Stop()
}
