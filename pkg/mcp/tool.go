package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Tool declares one callable MCP tool.
type Tool struct {
	Name        string
	Title       string
	Description string
	InputSchema map[string]any
	Annotations map[string]any
}

// CallToolRequest carries one tools/call invocation.
type CallToolRequest struct {
	Name      string
	Arguments json.RawMessage
}

// ToolHandler executes one tool and returns its bare domain value. The engine
// wraps the value in a successful MCP result envelope; returned errors become
// isError envelopes.
type ToolHandler func(ctx context.Context, request *CallToolRequest) (any, error)

// ErrToolNotFound is returned by tool dispatch when the requested name is not
// registered with the server.
var ErrToolNotFound = errors.New("未知工具")

// ToolError carries an optional structured payload alongside its message so an
// error envelope can surface tool-specific details.
type ToolError struct {
	message string
	data    any
}

func (e *ToolError) Error() string {
	return e.message
}

// NewToolError constructs a ToolError.
func NewToolError(message string, data any) error {
	return &ToolError{message: message, data: data}
}

// SuccessfulResult wraps a domain value in a successful MCP tool result.
func SuccessfulResult(value any) (map[string]any, error) {
	text_content, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("编码工具结果失败: %w", err)
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": string(text_content)}},
		"structuredContent": value,
		"isError":           false,
	}, nil
}

// ErrorResult renders an error as an MCP tool error envelope.
func ErrorResult(err error) map[string]any {
	message := err.Error()
	structured := map[string]any{"error": message}
	var tool_error *ToolError
	if errors.As(err, &tool_error) && tool_error.data != nil {
		structured["details"] = tool_error.data
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": message}},
		"structuredContent": structured,
		"isError":           true,
	}
}

// DecodeArguments unmarshals raw tool arguments, rejecting unknown fields.
func DecodeArguments(raw json.RawMessage, destination any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage("{}")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("工具参数无效: %w", err)
	}
	return nil
}

// definition renders the tool in MCP tools/list format.
func (t Tool) definition() map[string]any {
	definition := map[string]any{
		"name":        t.Name,
		"title":       t.Title,
		"description": t.Description,
		"inputSchema": t.InputSchema,
	}
	if len(t.Annotations) > 0 {
		definition["annotations"] = t.Annotations
	}
	return definition
}
