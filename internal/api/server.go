package api

import (
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/events"
	"wx_channel/internal/manager"
	"wx_channel/internal/webassets"
)

type APIServer struct {
	*manager.HTTPServer
	APIClient *APIClient
	bus       *events.Bus
}

func NewAPIServer(cfg *APIConfig, logger *zerolog.Logger, db *gorm.DB, staticAssets *webassets.Registry) *APIServer {
	srv := manager.NewHTTPServer("API服务", cfg.Hostname+":"+strconv.Itoa(cfg.Port))
	client := NewAPIClient(cfg, logger, db, staticAssets)
	srv.SetHandler(client.HTTPHandler())
	return &APIServer{
		HTTPServer: srv,
		APIClient:  client,
	}
}

func (s *APIServer) SubscribeEvents(bus *events.Bus) {
	s.bus = bus
	bus.Subscribe(events.TypeServiceCommand, func(e events.Event) {
		cmd, ok := e.(events.ServiceCommand)
		if !ok || cmd.Name != "api" {
			return
		}
		switch cmd.Action {
		case "start":
			_ = s.Start()
		case "stop":
			_ = s.Stop()
		}
	})
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
	if err := s.HTTPServer.Start(); err != nil {
		return err
	}
	s.publishStatus()
	return nil
}

func (s *APIServer) SetHandler(handler http.Handler) {
	s.HTTPServer.SetHandler(handler)
}

func (s *APIServer) Stop() error {
	if err := s.APIClient.Stop(); err != nil {
		return err
	}
	if err := s.HTTPServer.Stop(); err != nil {
		return err
	}
	s.publishStatus()
	return nil
}

func (s *APIServer) publishStatus() {
	if s.bus == nil {
		return
	}
	s.bus.Publish(events.ServiceStatusChanged{
		Name:   "api",
		Title:  "API服务",
		Addr:   s.Addr(),
		Status: string(s.Status()),
	})
}
