package services

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"

	"wx_channel/internal/mcpserver"
)

const mcp_transport = "streamable_http"

// MCPServiceConfig contains the dependencies used by the MCP protocol server.
type MCPServiceConfig struct {
	APIBaseURL          string
	Version             string
	DataReader          mcpserver.DataReader
	ScraperJobs         mcpserver.ScraperJobBackend
	DownloadTaskCreator mcpserver.DownloadTaskCreator
	DownloadTaskDeleter mcpserver.DownloadTaskDeleter
}

// MCPServiceStatus describes the process-local MCP service state.
type MCPServiceStatus struct {
	Enabled   bool
	Status    string
	Transport string
	Tools     []string
}

// MCPService owns the MCP protocol handler and its availability state.
type MCPService struct {
	handler_mu      sync.RWMutex
	handler         http.Handler
	handler_factory mcp_handler_factory
	enabled         atomic.Bool
}

type mcp_handler_factory func() (http.Handler, error)

// NewMCPService constructs an enabled MCP service.
func NewMCPService(config MCPServiceConfig) (*MCPService, error) {
	handler, err := build_mcp_handler(config)
	if err != nil {
		return nil, err
	}
	return new_mcp_service(handler), nil
}

// NewLazyMCPService constructs a disabled MCP service whose protocol handler
// is initialized only when Enable is called for the first time.
func NewLazyMCPService(config MCPServiceConfig) *MCPService {
	return new_lazy_mcp_service(func() (http.Handler, error) {
		return build_mcp_handler(config)
	})
}

func build_mcp_handler(config MCPServiceConfig) (http.Handler, error) {
	server, err := mcpserver.NewServer(mcpserver.Config{
		APIBaseURL:          config.APIBaseURL,
		Version:             config.Version,
		DataReader:          config.DataReader,
		ScraperJobs:         config.ScraperJobs,
		DownloadTaskCreator: config.DownloadTaskCreator,
		DownloadTaskDeleter: config.DownloadTaskDeleter,
	})
	if err != nil {
		return nil, err
	}
	return mcpserver.NewHTTPHandler(server), nil
}

func new_mcp_service(handler http.Handler) *MCPService {
	service := &MCPService{handler: handler}
	service.enabled.Store(handler != nil)
	return service
}

func new_lazy_mcp_service(handler_factory mcp_handler_factory) *MCPService {
	return &MCPService{handler_factory: handler_factory}
}

// Enable allows requests to reach the MCP protocol handler.
func (s *MCPService) Enable() error {
	if s == nil {
		return errors.New("MCP 服务未初始化")
	}
	s.handler_mu.Lock()
	defer s.handler_mu.Unlock()
	if s.handler == nil {
		if s.handler_factory == nil {
			return errors.New("MCP 服务未初始化")
		}
		handler, err := s.handler_factory()
		if err != nil {
			return err
		}
		if handler == nil {
			return errors.New("MCP 服务未初始化")
		}
		s.handler = handler
		s.handler_factory = nil
	}
	s.enabled.Store(true)
	return nil
}

// Disable rejects new MCP protocol requests without destroying the handler.
func (s *MCPService) Disable() error {
	if s == nil {
		return errors.New("MCP 服务未初始化")
	}
	s.enabled.Store(false)
	return nil
}

// Enabled reports whether MCP protocol requests are currently accepted.
func (s *MCPService) Enabled() bool {
	return s != nil && s.enabled.Load()
}

// Status returns the current MCP service state and capabilities.
func (s *MCPService) Status() MCPServiceStatus {
	enabled := s.Enabled()
	status := "stopped"
	if enabled {
		status = "running"
	}
	return MCPServiceStatus{
		Enabled:   enabled,
		Status:    status,
		Transport: mcp_transport,
		Tools:     mcpserver.ToolNames(),
	}
}

// ServeHTTP applies the service availability gate and delegates MCP protocol handling.
func (s *MCPService) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if s == nil {
		write_mcp_service_http_error(writer, http.StatusInternalServerError, "MCP 服务未初始化")
		return
	}
	if !s.enabled.Load() {
		write_mcp_service_http_error(writer, http.StatusServiceUnavailable, "MCP 服务未启用")
		return
	}
	s.handler_mu.RLock()
	handler := s.handler
	s.handler_mu.RUnlock()
	if handler == nil {
		write_mcp_service_http_error(writer, http.StatusInternalServerError, "MCP 服务未初始化")
		return
	}
	handler.ServeHTTP(writer, request)
}

func write_mcp_service_http_error(writer http.ResponseWriter, status_code int, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status_code)
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      nil,
		"error": map[string]any{
			"code":    -32000,
			"message": message,
		},
	})
}
