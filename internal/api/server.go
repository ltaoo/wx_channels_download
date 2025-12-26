package api

import (
	"wx_channel/internal/manager"
)

type APIServer struct {
	*manager.HTTPServer
	client *APIClient
}

func NewAPIServer(cfg *APISettings) *APIServer {
	srv := manager.NewHTTPServer("API服务", "api", cfg.Addr)
	client := NewAPIClient(cfg)
	srv.SetHandler(withCORS(client))
	return &APIServer{
		client:     client,
		HTTPServer: srv,
	}
}

func (s *APIServer) Start() error {
	if err := s.client.Start(); err != nil {
		return err
	}
	return s.HTTPServer.Start()
}

func (s *APIServer) Stop() error {
	s.client.downloader.Pause(nil)
	if err := s.client.Stop(); err != nil {
		return err
	}
	return s.HTTPServer.Stop()
}
