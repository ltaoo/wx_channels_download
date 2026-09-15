package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

// ServerOptions configures an MCP protocol server.
type ServerOptions struct {
	Name         string    // serverInfo.name; defaults to "mcp"
	Version      string    // defaults to "dev"
	Instructions string    // emitted by initialize and server/discover
	Input        io.Reader // defaults to an empty reader
	Output       io.Writer // defaults to io.Discard
	ErrorOutput  io.Writer // defaults to io.Discard
}

// Server implements the MCP JSON-RPC protocol over the stdio and Streamable
// HTTP transports. It owns only protocol state; tools are plugged in with
// AddTool.
type Server struct {
	name             string
	version          string
	instructions     string
	input            io.Reader
	output           io.Writer
	error_output     io.Writer
	tools_mu         sync.RWMutex
	tools            []Tool
	handlers         map[string]ToolHandler
	write_mu         sync.Mutex
	pending_mu       sync.Mutex
	pending          map[string]context.CancelFunc
	protocol_mu      sync.RWMutex
	protocol_version string
}

// NewServer constructs an MCP protocol server.
func NewServer(options ServerOptions) *Server {
	if options.Input == nil {
		options.Input = strings.NewReader("")
	}
	if options.Output == nil {
		options.Output = io.Discard
	}
	if options.ErrorOutput == nil {
		options.ErrorOutput = io.Discard
	}
	if strings.TrimSpace(options.Name) == "" {
		options.Name = "mcp"
	}
	if strings.TrimSpace(options.Version) == "" {
		options.Version = "dev"
	}
	return &Server{
		name:         options.Name,
		version:      options.Version,
		instructions: options.Instructions,
		input:        options.Input,
		output:       options.Output,
		error_output: options.ErrorOutput,
		handlers:     make(map[string]ToolHandler),
		pending:      make(map[string]context.CancelFunc),
	}
}

// AddTool registers a tool and its handler. A later registration with the same
// name replaces the earlier entry in place.
func (s *Server) AddTool(tool Tool, handler ToolHandler) {
	if s == nil {
		return
	}
	s.tools_mu.Lock()
	defer s.tools_mu.Unlock()
	for index := range s.tools {
		if s.tools[index].Name == tool.Name {
			s.tools[index] = tool
			s.handlers[tool.Name] = handler
			return
		}
	}
	s.tools = append(s.tools, tool)
	s.handlers[tool.Name] = handler
}

// RemoveTool unregisters a tool by name.
func (s *Server) RemoveTool(name string) {
	if s == nil {
		return
	}
	s.tools_mu.Lock()
	defer s.tools_mu.Unlock()
	delete(s.handlers, name)
	for index := range s.tools {
		if s.tools[index].Name == name {
			s.tools = append(s.tools[:index], s.tools[index+1:]...)
			return
		}
	}
}

// Tools returns a copy of the registered tool declarations.
func (s *Server) Tools() []Tool {
	if s == nil {
		return []Tool{}
	}
	s.tools_mu.RLock()
	defer s.tools_mu.RUnlock()
	result := make([]Tool, len(s.tools))
	copy(result, s.tools)
	return result
}

// ToolNames returns registered tool names in registration order.
func (s *Server) ToolNames() []string {
	if s == nil {
		return []string{}
	}
	s.tools_mu.RLock()
	defer s.tools_mu.RUnlock()
	names := make([]string, 0, len(s.tools))
	for _, tool := range s.tools {
		names = append(names, tool.Name)
	}
	return names
}

// CallTool invokes a registered tool directly and returns its structured
// result, mirroring the process-local service entry point.
func (s *Server) CallTool(ctx context.Context, name string, arguments json.RawMessage) (any, error) {
	result, err := s.call_tool(ctx, call_tool_params{Name: strings.TrimSpace(name), Arguments: arguments})
	if err != nil {
		return nil, err
	}
	return result["structuredContent"], nil
}

// Serve reads newline-delimited JSON-RPC messages until input closes.
func (s *Server) Serve(ctx context.Context) error {
	serve_context, cancel_serve := context.WithCancel(ctx)
	defer cancel_serve()

	scanner := bufio.NewScanner(s.input)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var requests sync.WaitGroup
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var request rpc_request
		if err := json.Unmarshal(line, &request); err != nil {
			s.write_response(rpc_response{
				JSONRPC: "2.0",
				ID:      json.RawMessage("null"),
				Error:   &rpc_error{Code: -32700, Message: "Parse error"},
			})
			continue
		}
		if len(request.ID) == 0 || bytes.Equal(bytes.TrimSpace(request.ID), []byte("null")) {
			s.handle_notification(request)
			continue
		}

		request_context, cancel_request := context.WithCancel(serve_context)
		request_key := string(request.ID)
		s.pending_mu.Lock()
		s.pending[request_key] = cancel_request
		s.pending_mu.Unlock()
		requests.Add(1)
		go func(current_context context.Context, current_cancel context.CancelFunc, current_request rpc_request, current_key string) {
			defer requests.Done()
			defer current_cancel()
			defer s.remove_pending(current_key)
			s.write_response(s.handle_request(current_context, current_request))
		}(request_context, cancel_request, request, request_key)
	}

	cancel_serve()
	s.cancel_all_pending()
	requests.Wait()
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取 MCP stdio 请求失败: %w", err)
	}
	return nil
}

func (s *Server) call_tool(ctx context.Context, params call_tool_params) (map[string]any, error) {
	handler := s.tool_handler(params.Name)
	if handler == nil {
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, params.Name)
	}
	value, err := handler(ctx, &CallToolRequest{Name: params.Name, Arguments: params.Arguments})
	if err != nil {
		return nil, err
	}
	return SuccessfulResult(value)
}

func (s *Server) tool_handler(name string) ToolHandler {
	if s == nil {
		return nil
	}
	s.tools_mu.RLock()
	defer s.tools_mu.RUnlock()
	return s.handlers[name]
}

func (s *Server) tool_definitions() []any {
	if s == nil {
		return []any{}
	}
	s.tools_mu.RLock()
	defer s.tools_mu.RUnlock()
	definitions := make([]any, 0, len(s.tools))
	for _, tool := range s.tools {
		definitions = append(definitions, tool.definition())
	}
	return definitions
}
