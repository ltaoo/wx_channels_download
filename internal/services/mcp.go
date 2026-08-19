package services

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"

	"wx_channel/internal/mcpserver"
)

const mcp_transport = "streamable_http"

// MCPServiceConfig contains the dependencies used by the MCP protocol server.
type MCPServiceConfig struct {
	APIBaseURL  string
	Version     string
	DataReader  mcpserver.DataReader
	ScraperJobs mcpserver.ScraperJobBackend
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
	handler http.Handler
	enabled atomic.Bool
}

// NewMCPService constructs an enabled MCP service.
func NewMCPService(config MCPServiceConfig) (*MCPService, error) {
	server, err := mcpserver.NewServer(mcpserver.Config{
		APIBaseURL:  config.APIBaseURL,
		Version:     config.Version,
		DataReader:  config.DataReader,
		ScraperJobs: config.ScraperJobs,
	})
	if err != nil {
		return nil, err
	}
	return new_mcp_service(mcpserver.NewHTTPHandler(server)), nil
}

func new_mcp_service(handler http.Handler) *MCPService {
	service := &MCPService{handler: handler}
	service.enabled.Store(handler != nil)
	return service
}

// Enable allows requests to reach the MCP protocol handler.
func (s *MCPService) Enable() error {
	if s == nil || s.handler == nil {
		return errors.New("MCP 服务未初始化")
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
	if s == nil || s.handler == nil {
		write_mcp_service_http_error(writer, http.StatusInternalServerError, "MCP 服务未初始化")
		return
	}
	if !s.enabled.Load() {
		write_mcp_service_http_error(writer, http.StatusServiceUnavailable, "MCP 服务未启用")
		return
	}
	s.handler.ServeHTTP(writer, request)
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
