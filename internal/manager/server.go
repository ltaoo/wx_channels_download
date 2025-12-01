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
	name     string
	port     int
	status   ServerStatus
	mux      *http.ServeMux
	server   *http.Server
	mu       sync.RWMutex
	stopChan chan struct{}
}

func NewHTTPServer(name string, port int) *HTTPServer {
	mux := http.NewServeMux()

	// 添加测试路由
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	return &HTTPServer{
		name:     name,
		port:     port,
		status:   StatusStopped,
		mux:      mux,
		stopChan: make(chan struct{}),
	}
}

func (s *HTTPServer) Name() string {
	return s.name
}

func (s *HTTPServer) Port() int {
	return s.port
}

func (s *HTTPServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == StatusRunning || s.status == StatusStarting {
		return fmt.Errorf("server is already %s", s.status)
	}

	s.status = StatusStarting
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.mux,
	}

	go func() {
		s.mu.Lock()
		s.status = StatusRunning
		s.mu.Unlock()

		fmt.Printf("Server %s starting on port %d\n", s.name, s.port)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.mu.Lock()
			s.status = StatusError
			s.mu.Unlock()
			fmt.Printf("Server %s error: %v\n", s.name, err)
			return
		}

		s.mu.Lock()
		s.status = StatusStopped
		s.mu.Unlock()
		fmt.Printf("Server %s stopped\n", s.name)
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (s *HTTPServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status != StatusRunning {
		return fmt.Errorf("server is not running")
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

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", s.port))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %s", resp.Status)
	}

	return nil
}
