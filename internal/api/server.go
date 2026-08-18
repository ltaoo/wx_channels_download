package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/events"
	"wx_channel/internal/services"
	"wx_channel/internal/webassets"
	"wx_channel/pkg/hermes"
)

type APIServer struct {
	addr      string
	handler   http.Handler
	server    *http.Server
	APIClient *APIClient
	bus       *events.Bus
	logger    *zerolog.Logger
}

func NewAPIServer(
	cfg *APIConfig,
	logger *zerolog.Logger,
	db *gorm.DB,
	static_assets *webassets.Registry,
	downloader *hermes.HermesEngine,
	hook_manager *hermes.HookManager,
	update_service *services.UpdateService,
	restart_service *services.ApplicationRestartService,
) *APIServer {
	client := NewAPIClient(cfg, logger, db, static_assets, downloader, hook_manager, update_service, restart_service)
	logger.Info().
		Str("listen_addr", cfg.Hostname+":"+strconv.Itoa(cfg.Port)).
		Msg("api server configured")
	return &APIServer{
		addr:      cfg.Hostname + ":" + strconv.Itoa(cfg.Port),
		handler:   client.HTTPHandler(),
		APIClient: client,
		logger:    logger,
	}
}

func (s *APIServer) SubscribeEvents(bus *events.Bus) {
	s.bus = bus
	s.APIClient.SubscribeEvents(bus)
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
	s.logger.Info().Str("addr", s.addr).Msg("api server listen starting")
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		s.logger.Error().Err(err).Str("addr", s.addr).Msg("api server listen failed")
		return fmt.Errorf("启动API服务失败，端口被占用: %v", err)
	}
	if err := s.APIClient.Start(); err != nil {
		_ = listener.Close()
		s.logger.Error().Err(err).Str("addr", s.addr).Msg("api client start failed")
		return err
	}
	server := &http.Server{
		Addr:    s.addr,
		Handler: s.handler,
	}
	s.server = server
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("API服务 error: %v\n", err)
			return
		}
		fmt.Println("API服务 stopped")
	}()

	s.logger.Info().
		Str("addr", s.addr).
		Str("listener_addr", listener.Addr().String()).
		Msg("api server listen started")
	s.publishStatus("running")
	return nil
}

func (s *APIServer) Addr() string {
	return s.addr
}

func (s *APIServer) Stop() error {
	s.logger.Info().Str("addr", s.addr).Msg("api server stopping")
	if err := s.APIClient.Stop(); err != nil {
		s.logger.Error().Err(err).Str("addr", s.addr).Msg("api client stop failed")
		return err
	}
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); err != nil {
			s.logger.Error().Err(err).Str("addr", s.addr).Msg("api server shutdown failed")
			return err
		}
		s.server = nil
	}
	s.logger.Info().Str("addr", s.addr).Msg("api server stopped")
	s.publishStatus("stopped")
	return nil
}

func (s *APIServer) publishStatus(status string) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(events.ServiceStatusChanged{
		Name:   "api",
		Title:  "API服务",
		Addr:   s.Addr(),
		Status: status,
	})
}
