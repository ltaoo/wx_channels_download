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
)

type APIServer struct {
	addr            string
	handler         http.Handler
	server          *http.Server
	APIClient       *APIClient
	event_publisher events.Publisher
	logger          *zerolog.Logger
}

func NewAPIServer(
	cfg *APIConfig,
	logger *zerolog.Logger,
	db *gorm.DB,
	static_assets *webassets.Registry,
	download_task_broadcaster *DownloadTaskBroadcaster,
	event_publisher events.Publisher,
	runtime_status_service *services.RuntimeStatusService,
	account_service *services.AccountService,
	content_service *services.ContentService,
	browse_history_service *services.BrowseService,
	download_task_service *services.DownloadTaskService,
	fs_service *services.FSService,
	scraper_job_service *services.ScraperJobService,
	hub_service *services.HubService,
	certificate_service *services.CertificateService,
	mcp_service *services.MCPService,
	application_update_service *services.ApplicationUpdateService,
	restart_service *services.ApplicationRestartService,
) *APIServer {
	client := NewAPIClient(
		cfg,
		logger,
		db,
		static_assets,
		download_task_broadcaster,
		event_publisher,
		runtime_status_service,
		account_service,
		content_service,
		browse_history_service,
		download_task_service,
		fs_service,
		scraper_job_service,
		hub_service,
		certificate_service,
		mcp_service,
		application_update_service,
		restart_service,
	)
	logger.Info().
		Str("listen_addr", cfg.Hostname+":"+strconv.Itoa(cfg.Port)).
		Msg("api server configured")
	return &APIServer{
		addr:            cfg.Hostname + ":" + strconv.Itoa(cfg.Port),
		handler:         client.HTTPHandler(),
		APIClient:       client,
		event_publisher: event_publisher,
		logger:          logger,
	}
}

func (s *APIServer) Start() error {
	s.logger.Info().Str("addr", s.addr).Msg("api server listen starting")
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		s.logger.Error().Err(err).Str("addr", s.addr).Msg("api server listen failed")
		return fmt.Errorf("启动API服务失败，端口被占用: %v", err)
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
	s.publish_status("running")
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
	s.publish_status("stopped")
	return nil
}

func (s *APIServer) publish_status(status string) {
	if s.event_publisher == nil {
		return
	}
	s.event_publisher.Publish(events.ServiceStatusChanged{
		Name:   "api",
		Title:  "API服务",
		Addr:   s.Addr(),
		Status: status,
	})
}
