package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"

	"wx_channel/internal/mcpserver"
	servicetools "wx_channel/internal/services/tools"
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
	SphDeployer         mcpserver.SphDeployer
	ZhihuCollections    mcpserver.ZhihuCollectionReader
	ZhihuCredentials    mcpserver.ZhihuCredentialReader
	Automation          mcpserver.AutomationBackend
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
	handler_mu     sync.RWMutex
	handler        http.Handler
	server         *mcpserver.Server
	tool_service   *servicetools.Service
	server_factory mcp_server_factory
	enabled        atomic.Bool
}

type mcp_server_factory func() (*mcpserver.Server, error)

// NewMCPService constructs an enabled MCP service.
func NewMCPService(config MCPServiceConfig) (*MCPService, error) {
	server, err := build_mcp_server(config)
	if err != nil {
		return nil, err
	}
	return new_mcp_service(server), nil
}

// NewLazyMCPService constructs a disabled MCP service whose protocol server is
// initialized by the first HTTP enable or in-process tool execution.
func NewLazyMCPService(config MCPServiceConfig) *MCPService {
	return new_lazy_mcp_service(func() (*mcpserver.Server, error) {
		return build_mcp_server(config)
	})
}

func build_mcp_server(config MCPServiceConfig) (*mcpserver.Server, error) {
	return mcpserver.NewServer(mcpserver.Config{
		APIBaseURL:          config.APIBaseURL,
		Version:             config.Version,
		DataReader:          config.DataReader,
		ScraperJobs:         config.ScraperJobs,
		DownloadTaskCreator: config.DownloadTaskCreator,
		DownloadTaskDeleter: config.DownloadTaskDeleter,
		SphDeployer:         config.SphDeployer,
		ZhihuCollections:    config.ZhihuCollections,
		ZhihuCredentials:    config.ZhihuCredentials,
		Automation:          config.Automation,
	})
}

func new_mcp_service(server *mcpserver.Server) *MCPService {
	service := &MCPService{server: server}
	if server != nil {
		service.handler = mcpserver.NewHTTPHandler(server)
		service.tool_service = server.ToolService()
	}
	service.enabled.Store(server != nil)
	return service
}

func new_lazy_mcp_service(server_factory mcp_server_factory) *MCPService {
	return &MCPService{server_factory: server_factory}
}

func (s *MCPService) ensure_server_locked() error {
	if s.server != nil {
		if s.handler == nil {
			s.handler = mcpserver.NewHTTPHandler(s.server)
		}
		return nil
	}
	if s.server_factory == nil {
		return errors.New("MCP 服务未初始化")
	}
	server, err := s.server_factory()
	if err != nil {
		return err
	}
	if server == nil {
		return errors.New("MCP 服务未初始化")
	}
	s.server = server
	s.tool_service = server.ToolService()
	s.handler = mcpserver.NewHTTPHandler(server)
	s.server_factory = nil
	return nil
}

// Enable allows requests to reach the MCP protocol handler.
func (s *MCPService) Enable() error {
	if s == nil {
		return errors.New("MCP 服务未初始化")
	}
	s.handler_mu.Lock()
	defer s.handler_mu.Unlock()
	if err := s.ensure_server_locked(); err != nil {
		return err
	}
	s.enabled.Store(true)
	return nil
}

// ExecuteTool makes all service tools available to process-local callers. It
// initializes a lazy server without changing the HTTP enabled state.
func (s *MCPService) ExecuteTool(ctx context.Context, name string, arguments map[string]any) (any, error) {
	if s == nil {
		return nil, errors.New("MCP 服务未初始化")
	}
	s.handler_mu.Lock()
	if err := s.ensure_server_locked(); err != nil {
		s.handler_mu.Unlock()
		return nil, err
	}
	tool_service := s.tool_service
	s.handler_mu.Unlock()
	if tool_service == nil {
		return nil, errors.New("工具服务未初始化")
	}
	return tool_service.Execute(ctx, name, arguments)
}

// ToolCatalog returns the canonical service tool declarations used by MCP,
// workflow nodes, and other process-local callers.
func (s *MCPService) ToolCatalog() []servicetools.Definition {
	if s == nil {
		return []servicetools.Definition{}
	}
	s.handler_mu.RLock()
	tool_service := s.tool_service
	s.handler_mu.RUnlock()
	if tool_service == nil {
		return mcpserver.ToolCatalog()
	}
	return tool_service.Definitions()
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
	tools := mcpserver.ToolNames()
	if s != nil {
		s.handler_mu.RLock()
		if s.tool_service != nil {
			tools = s.tool_service.Names()
		}
		s.handler_mu.RUnlock()
	}
	return MCPServiceStatus{
		Enabled:   enabled,
		Status:    status,
		Transport: mcp_transport,
		Tools:     tools,
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
