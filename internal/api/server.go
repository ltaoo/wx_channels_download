package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/webassets"
	"wx_channel/pkg/events"
	"wx_channel/pkg/hermes"
)

type APIServer struct {
	APIClient *APIClient
	addr      string
	server    *http.Server
	running   bool
	mu        sync.Mutex
}

func NewAPIServer(
	cfg *APIConfig,
	config_store ConfigStore,
	logger *zerolog.Logger,
	db *gorm.DB,
	staticAssets *webassets.Registry,
	downloader *hermes.HermesEngine,
	hookManager *hermes.HookManager,
) *APIServer {
	client := NewAPIClient(cfg, config_store, logger, db, staticAssets, downloader, hookManager)
	return &APIServer{
		APIClient: client,
		addr:      cfg.Hostname + ":" + strconv.Itoa(cfg.Port),
	}
}

func (s *APIServer) SubscribeEvents(bus *events.Bus) {
	s.APIClient.SubscribeEvents(bus)
}

func (s *APIServer) Addr() string {
	return s.addr
}

func (s *APIServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return fmt.Errorf("API service is already running")
	}

	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("启动API服务失败，端口被占用: %v", err)
	}
	if err := s.APIClient.Start(); err != nil {
		_ = l.Close()
		return err
	}

	server := &http.Server{Addr: s.addr, Handler: s.APIClient.HTTPHandler()}
	s.server = server
	s.running = true
	go func() {
		if err := server.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("API服务 error: %v\n", err)
		}
		s.mu.Lock()
		if s.server == server {
			s.server = nil
			s.running = false
		}
		s.mu.Unlock()
	}()
	return nil
}

func (s *APIServer) Stop() error {
	if err := s.APIClient.Stop(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running || s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.server.Shutdown(ctx); err != nil {
		return err
	}
	s.server = nil
	s.running = false
	return nil
}
