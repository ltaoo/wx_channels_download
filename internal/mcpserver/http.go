package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
)

const max_http_request_size = 4 * 1024 * 1024

type http_handler struct {
	server *Server
}

// NewHTTPHandler exposes the MCP server over the stateless Streamable HTTP
// transport. Tool calls return their JSON-RPC response in the POST response.
func NewHTTPHandler(server *Server) http.Handler {
	return &http_handler{server: server}
}

func (h *http_handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if h == nil || h.server == nil {
		write_http_rpc_error(writer, http.StatusInternalServerError, json.RawMessage("null"), -32603, "MCP 服务未初始化")
		return
	}
	if !valid_http_origin(request) {
		write_http_rpc_error(writer, http.StatusForbidden, json.RawMessage("null"), -32000, "Origin 不受信任")
		return
	}
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		write_http_rpc_error(writer, http.StatusMethodNotAllowed, json.RawMessage("null"), -32600, "仅支持 POST")
		return
	}
	if !is_json_content_type(request.Header.Get("Content-Type")) {
		write_http_rpc_error(writer, http.StatusUnsupportedMediaType, json.RawMessage("null"), -32600, "Content-Type 必须是 application/json")
		return
	}

	rpc_request, rpc_error := decode_http_rpc_request(writer, request)
	if rpc_error != nil {
		write_http_rpc_error(writer, http.StatusBadRequest, json.RawMessage("null"), rpc_error.Code, rpc_error.Message)
		return
	}
	if len(rpc_request.ID) == 0 || bytes.Equal(bytes.TrimSpace(rpc_request.ID), []byte("null")) {
		h.server.handle_notification(rpc_request)
		writer.WriteHeader(http.StatusAccepted)
		return
	}

	request_server := h.request_server(request)
	response := request_server.handle_request(request.Context(), rpc_request)
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(writer).Encode(response)
}

func (h *http_handler) request_server(request *http.Request) *Server {
	request_server := &Server{
		api_client:   h.server.api_client,
		data_reader:  h.server.data_reader,
		error_output: h.server.error_output,
		version:      h.server.version,
		pending:      make(map[string]context.CancelFunc),
	}
	protocol_version := strings.TrimSpace(request.Header.Get("MCP-Protocol-Version"))
	if protocol_version != "" {
		request_server.set_protocol_version(protocol_version)
	}
	return request_server
}

func decode_http_rpc_request(writer http.ResponseWriter, request *http.Request) (rpc_request, *rpc_error) {
	var rpc_request rpc_request
	body := http.MaxBytesReader(writer, request.Body, max_http_request_size)
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&rpc_request); err != nil {
		return rpc_request, &rpc_error{Code: -32700, Message: "Parse error"}
	}
	var trailing_value any
	if err := decoder.Decode(&trailing_value); !errors.Is(err, io.EOF) {
		return rpc_request, &rpc_error{Code: -32700, Message: "Parse error"}
	}
	if rpc_request.JSONRPC != "2.0" || strings.TrimSpace(rpc_request.Method) == "" {
		return rpc_request, &rpc_error{Code: -32600, Message: "Invalid Request"}
	}
	return rpc_request, nil
}

func is_json_content_type(content_type string) bool {
	media_type, _, err := mime.ParseMediaType(content_type)
	return err == nil && strings.EqualFold(media_type, "application/json")
}

func valid_http_origin(request *http.Request) bool {
	raw_origin := strings.TrimSpace(request.Header.Get("Origin"))
	if raw_origin == "" {
		return true
	}
	origin, err := url.Parse(raw_origin)
	if err != nil || origin.Host == "" {
		return false
	}
	return strings.EqualFold(origin.Host, request.Host)
}

func write_http_rpc_error(writer http.ResponseWriter, status_code int, request_id json.RawMessage, code int, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status_code)
	_ = json.NewEncoder(writer).Encode(rpc_response{
		JSONRPC: "2.0",
		ID:      request_id,
		Error:   &rpc_error{Code: code, Message: message},
	})
}
