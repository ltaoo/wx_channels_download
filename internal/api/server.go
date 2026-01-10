package api

import (
	"fmt"
	"net"

	"github.com/rs/zerolog"

	"wx_channel/internal/manager"
)

type APIServer struct {
	*manager.HTTPServer
	APIClient *APIClient
}

func NewAPIServer(cfg *APIConfig, logger *zerolog.Logger) *APIServer {
	srv := manager.NewHTTPServer("API服务", "api", cfg.Addr)
	client := NewAPIClient(cfg, logger)
	srv.SetHandler(withCORS(client))
	return &APIServer{
		HTTPServer: srv,
		APIClient:  client,
	}
}

func (s *APIServer) Start() error {
	l, err := net.Listen("tcp", s.HTTPServer.Addr())
	if err != nil {
		return fmt.Errorf("启动API服务失败，端口被占用: %v", err)
	}
	l.Close()
	if err := s.APIClient.Start(); err != nil {
		return err
	}
	return s.HTTPServer.Start()
}

func (s *APIServer) Stop() error {
	if err := s.APIClient.Stop(); err != nil {
		return err
	}
	return s.HTTPServer.Stop()
}
