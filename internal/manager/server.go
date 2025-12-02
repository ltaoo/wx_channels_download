package manager

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// HTTPServer 实现
type HTTPServer struct {
	title    string
	key      string
	addr     string
	status   ServerStatus
	mux      http.Handler
	server   *http.Server
	mu       sync.RWMutex
	stopChan chan struct{}
}

func NewHTTPServer(title string, key string, addr string) *HTTPServer {
	return &HTTPServer{
		title:    title,
		key:      key,
		addr:     addr,
		status:   StatusStopped,
		stopChan: make(chan struct{}),
	}
}

func (s *HTTPServer) Name() string {
	return s.key
}

func (s *HTTPServer) Addr() string {
	return s.addr
}

func (s *HTTPServer) SetHandler(handler http.Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mux = handler
}

func (s *HTTPServer) Mux() http.Handler {
	return s.mux
}

func (s *HTTPServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == StatusRunning || s.status == StatusStarting {
		return fmt.Errorf("server is already %s", s.status)
	}

	s.status = StatusStarting
	s.server = &http.Server{
		Addr:    s.addr,
		Handler: s.mux,
	}

	go func() {
		s.mu.Lock()
		s.status = StatusRunning
		s.mu.Unlock()

		// fmt.Printf("Server %s starting on addr %s\n", s.name, s.addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.mu.Lock()
			s.status = StatusError
			s.mu.Unlock()
			fmt.Printf("%s error: %v\n", s.title, err)
			return
		}

		s.mu.Lock()
		s.status = StatusStopped
		s.mu.Unlock()
		fmt.Printf("%s 已关闭\n", s.title)
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (s *HTTPServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status != StatusRunning {
		return nil
	}

	s.status = StatusStopping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		s.status = StatusError
		return err
	}

	return nil
}

func (s *HTTPServer) Status() ServerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *HTTPServer) HealthCheck() error {
	if s.Status() != StatusRunning {
		return fmt.Errorf("server not running")
	}

	resp, err := http.Get(fmt.Sprintf("http://%s/health", s.addr))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %s", resp.Status)
	}

	return nil
}
