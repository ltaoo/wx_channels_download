package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	mcp "wx_channel/pkg/mcp"
)

type ToolDefinition = Definition

// tool_execution_error is a true alias so errors.As in mcp.ErrorResult keeps
// matching errors returned by this package's tool handlers.
type tool_execution_error = mcp.ToolError

func new_tool_execution_error(message string, data any) error {
	return mcp.NewToolError(message, data)
}

// ToolNames returns every declared tool name, independent of which backends
// happen to be enabled in one process.
func ToolNames() []string {
	catalog := ToolCatalog()
	names := make([]string, 0, len(catalog))
	for _, definition := range catalog {
		names = append(names, definition.Name)
	}
	return names
}

// ToolCatalog returns every declared tool, independent of which backends happen
// to be enabled in one process. It is used by the automation editor to
// configure service nodes without maintaining a second tool list.
//
// The slice is a fresh copy but its elements are the shared init-time
// definitions: callers must treat the returned metadata as read-only.
func ToolCatalog() []ToolDefinition {
	definitions := make([]ToolDefinition, len(tool_definitions))
	copy(definitions, tool_definitions)
	return definitions
}

// ToolCatalog returns the tools enabled by this server's configured service
// backends. CLI and other process-local callers use the same catalog as MCP.
func (s *ToolSet) ToolCatalog() []ToolDefinition {
	if s == nil {
		return []ToolDefinition{}
	}
	definitions := make([]ToolDefinition, 0, len(tool_declarations))
	for index, declaration := range tool_declarations {
		if declaration.supports(s) {
			definitions = append(definitions, tool_definitions[index])
		}
	}
	return definitions
}

// ToolNames returns the tools enabled by this server.
func (s *ToolSet) ToolNames() []string {
	catalog := s.ToolCatalog()
	names := make([]string, 0, len(catalog))
	for _, definition := range catalog {
		names = append(names, definition.Name)
	}
	return names
}

// ExecuteTool invokes an MCP tool directly and returns its structured result.
// This shares the exact same validation and dispatch path as tools/call.
func (s *ToolSet) ExecuteTool(ctx context.Context, name string, arguments map[string]any) (any, error) {
	if s == nil {
		return nil, errors.New("工具服务未初始化")
	}
	if arguments == nil {
		arguments = map[string]any{}
	}
	raw_arguments, err := json.Marshal(arguments)
	if err != nil {
		return nil, fmt.Errorf("编码工具参数失败: %w", err)
	}
	result, err := s.call(ctx, name, raw_arguments)
	if err != nil {
		return nil, err
	}
	if structured, ok := result["structuredContent"]; ok {
		return structured, nil
	}
	return result, nil
}

// Shared helpers used by the per-domain tool handlers.

func successful_tool_result(value any) (map[string]any, error) {
	return mcp.SuccessfulResult(value)
}

func decode_tool_arguments(raw json.RawMessage, destination any) error {
	return mcp.DecodeArguments(raw, destination)
}

func validate_source_url(raw_url string) error {
	if raw_url == "" {
		return fmt.Errorf("url 不能为空")
	}
	parsed_url, err := url.Parse(raw_url)
	if err != nil || parsed_url.Host == "" || (parsed_url.Scheme != "http" && parsed_url.Scheme != "https") {
		return fmt.Errorf("url 必须是有效的 http 或 https 链接")
	}
	return nil
}

func timeout_duration(value int, fallback int, maximum int) (time.Duration, error) {
	if value == 0 {
		value = fallback
	}
	if value < 1 || value > maximum {
		return 0, fmt.Errorf("timeout_seconds 必须在 1 到 %d 之间", maximum)
	}
	return time.Duration(value) * time.Second, nil
}

func is_existing_action(action string) bool {
	switch action {
	case "error", "skip", "overwrite", "duplicate":
		return true
	default:
		return false
	}
}
