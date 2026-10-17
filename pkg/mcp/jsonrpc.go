package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type rpc_request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpc_response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpc_error      `json:"error,omitempty"`
}

type rpc_error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type call_tool_params struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type cancel_params struct {
	RequestID json.RawMessage `json:"requestId"`
}

type initialize_params struct {
	ProtocolVersion string `json:"protocolVersion"`
}

type request_metadata struct {
	Meta map[string]json.RawMessage `json:"_meta"`
}

func decode_params(raw json.RawMessage, destination any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage("{}")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("请求参数无效: %w", err)
	}
	return nil
}

func invalid_params_error(err error) *rpc_error {
	return &rpc_error{Code: -32602, Message: err.Error()}
}
