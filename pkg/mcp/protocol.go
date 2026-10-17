package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	modern_protocol_version = "2026-07-28"
	legacy_protocol_version = "2025-11-25"
)

var supported_legacy_protocol_versions = map[string]struct{}{
	"2025-11-25": {},
	"2025-06-18": {},
	"2025-03-26": {},
	"2024-11-05": {},
}

func (s *Server) handle_request(ctx context.Context, request rpc_request) rpc_response {
	response := rpc_response{JSONRPC: "2.0", ID: request.ID}
	modern := s.is_modern_request(request)
	switch request.Method {
	case "server/discover":
		s.set_protocol_version(modern_protocol_version)
		response.Result = s.discover_result()
	case "initialize":
		var params initialize_params
		if err := decode_params(request.Params, &params); err != nil {
			response.Error = invalid_params_error(err)
			break
		}
		protocol_version := legacy_protocol_version
		if _, ok := supported_legacy_protocol_versions[params.ProtocolVersion]; ok {
			protocol_version = params.ProtocolVersion
		}
		s.set_protocol_version(protocol_version)
		response.Result = map[string]any{
			"protocolVersion": protocol_version,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      s.server_info(),
			"instructions":    s.instructions,
		}
	case "ping":
		response.Result = s.decorate_result(map[string]any{}, modern)
	case "tools/list":
		response.Result = s.decorate_result(map[string]any{
			"tools":      s.tool_definitions(),
			"ttlMs":      300000,
			"cacheScope": "public",
		}, modern)
	case "tools/call":
		var params call_tool_params
		if err := decode_params(request.Params, &params); err != nil {
			response.Error = invalid_params_error(err)
			break
		}
		if strings.TrimSpace(params.Name) == "" {
			response.Error = &rpc_error{Code: -32602, Message: "工具名称不能为空"}
			break
		}
		tool_result, err := s.call_tool(ctx, params)
		if err != nil {
			if errors.Is(err, ErrToolNotFound) {
				response.Error = &rpc_error{Code: -32602, Message: err.Error()}
				break
			}
			response.Result = s.decorate_result(ErrorResult(err), modern)
			break
		}
		response.Result = s.decorate_result(tool_result, modern)
	default:
		response.Error = &rpc_error{Code: -32601, Message: "Method not found: " + request.Method}
	}
	return response
}

func (s *Server) handle_notification(request rpc_request) {
	if request.Method != "notifications/cancelled" {
		return
	}
	var params cancel_params
	if err := decode_params(request.Params, &params); err != nil || len(params.RequestID) == 0 {
		return
	}
	request_key := string(params.RequestID)
	s.pending_mu.Lock()
	cancel_request := s.pending[request_key]
	s.pending_mu.Unlock()
	if cancel_request != nil {
		cancel_request()
	}
}

func (s *Server) discover_result() map[string]any {
	return map[string]any{
		"resultType":        "complete",
		"supportedVersions": []string{modern_protocol_version, legacy_protocol_version, "2025-06-18", "2025-03-26", "2024-11-05"},
		"capabilities":      map[string]any{"tools": map[string]any{}},
		"_meta":             map[string]any{"io.modelcontextprotocol/serverInfo": s.server_info()},
		"instructions":      s.instructions,
		"ttlMs":             300000,
		"cacheScope":        "public",
	}
}

func (s *Server) decorate_result(result map[string]any, modern bool) map[string]any {
	if !modern {
		return result
	}
	result["resultType"] = "complete"
	result["_meta"] = map[string]any{"io.modelcontextprotocol/serverInfo": s.server_info()}
	return result
}

func (s *Server) server_info() map[string]any {
	return map[string]any{"name": s.name, "version": s.version}
}

func (s *Server) is_modern_request(request rpc_request) bool {
	if request.Method == "server/discover" {
		return true
	}
	var metadata request_metadata
	if len(request.Params) > 0 && json.Unmarshal(request.Params, &metadata) == nil {
		if raw_version := metadata.Meta["io.modelcontextprotocol/protocolVersion"]; len(raw_version) > 0 {
			var protocol_version string
			if json.Unmarshal(raw_version, &protocol_version) == nil && protocol_version == modern_protocol_version {
				return true
			}
		}
	}
	s.protocol_mu.RLock()
	defer s.protocol_mu.RUnlock()
	return s.protocol_version == modern_protocol_version
}

func (s *Server) set_protocol_version(protocol_version string) {
	s.protocol_mu.Lock()
	s.protocol_version = protocol_version
	s.protocol_mu.Unlock()
}

func (s *Server) write_response(response rpc_response) {
	data, err := json.Marshal(response)
	if err != nil {
		fmt.Fprintf(s.error_output, "编码 MCP 响应失败: %v\n", err)
		return
	}
	s.write_mu.Lock()
	defer s.write_mu.Unlock()
	if _, err := s.output.Write(append(data, '\n')); err != nil {
		fmt.Fprintf(s.error_output, "写入 MCP 响应失败: %v\n", err)
	}
}

func (s *Server) remove_pending(request_key string) {
	s.pending_mu.Lock()
	delete(s.pending, request_key)
	s.pending_mu.Unlock()
}

func (s *Server) cancel_all_pending() {
	s.pending_mu.Lock()
	defer s.pending_mu.Unlock()
	for _, cancel_request := range s.pending {
		cancel_request()
	}
}
