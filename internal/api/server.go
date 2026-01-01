package api

import (
	"wx_channel/internal/manager"
)

type APIServer struct {
	*manager.HTTPServer
	APIClient *APIClient
}

func NewAPIServer(cfg *APIConfig) *APIServer {
	srv := manager.NewHTTPServer("API服务", "api", cfg.Addr)
	client := NewAPIClient(cfg)
	srv.SetHandler(withCORS(client))
	return &APIServer{
		APIClient:  client,
		HTTPServer: srv,
	}
}

func (s *APIServer) Start() error {
	if err := s.APIClient.Start(); err != nil {
		return err
	}
	return s.HTTPServer.Start()
}

func (s *APIServer) Stop() error {
	s.APIClient.downloader.Pause(nil)
	if err := s.APIClient.Stop(); err != nil {
		return err
	}
	return s.HTTPServer.Stop()
}
