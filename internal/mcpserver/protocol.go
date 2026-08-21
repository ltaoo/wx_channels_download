package mcpserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	modern_protocol_version = "2026-07-28"
	legacy_protocol_version = "2025-11-25"
	server_name             = "dm"
)

var supported_legacy_protocol_versions = map[string]struct{}{
	"2025-11-25": {},
	"2025-06-18": {},
	"2025-03-26": {},
	"2024-11-05": {},
}

// Config configures a stdio MCP server backed by a downloader API, process-local
// services, or both.
type Config struct {
	APIBaseURL   string
	Version      string
	Input        io.Reader
	Output       io.Writer
	ErrorOutput  io.Writer
	HTTPClient   *http.Client
	PollInterval time.Duration
	DataReader   DataReader
	ScraperJobs  ScraperJobBackend
}

// Server implements the MCP stdio transport and exposes the tools supported by
// its configured backends.
type Server struct {
	api_client       *api_client
	data_reader      DataReader
	scraper_jobs     ScraperJobBackend
	input            io.Reader
	output           io.Writer
	error_output     io.Writer
	version          string
	write_mu         sync.Mutex
	pending_mu       sync.Mutex
	pending          map[string]context.CancelFunc
	protocol_mu      sync.RWMutex
	protocol_version string
}

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

// NewServer constructs a stdio MCP server.
func NewServer(config Config) (*Server, error) {
	if config.Input == nil {
		config.Input = strings.NewReader("")
	}
	if config.Output == nil {
		config.Output = io.Discard
	}
	if config.ErrorOutput == nil {
		config.ErrorOutput = io.Discard
	}
	if strings.TrimSpace(config.Version) == "" {
		config.Version = "dev"
	}
	var client *api_client
	if strings.TrimSpace(config.APIBaseURL) != "" {
		var err error
		client, err = new_api_client(config.APIBaseURL, config.HTTPClient, config.PollInterval)
		if err != nil {
			return nil, err
		}
	}
	if client == nil && config.DataReader == nil && config.ScraperJobs == nil {
		return nil, fmt.Errorf("至少需要配置 API 地址、数据查询服务或抓取任务服务")
	}
	return &Server{
		api_client:   client,
		data_reader:  config.DataReader,
		scraper_jobs: config.ScraperJobs,
		input:        config.Input,
		output:       config.Output,
		error_output: config.ErrorOutput,
		version:      config.Version,
		pending:      make(map[string]context.CancelFunc),
	}, nil
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
			"instructions":    server_instructions(),
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
			if errors.Is(err, err_unknown_tool) {
				response.Error = &rpc_error{Code: -32602, Message: err.Error()}
				break
			}
			response.Result = s.decorate_result(tool_error_result(err), modern)
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
		"instructions":      server_instructions(),
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
	return map[string]any{"name": server_name, "version": s.version}
}

func server_instructions() string {
	return "查询下载器平台状态、解析受支持平台的内容链接，并创建和启动内容下载任务。调用 download_content 前应先获得用户对下载操作的确认。凡涉及下载文件的文件系统操作，包括重命名、移动、删除以及修改文件名或路径，都必须在同一业务流程中同步更新数据库中的 DownloadResource 表记录，禁止仅操作本地文件。若当前工具无法保证文件系统与 DownloadResource 记录一致，必须停止操作并明确告知用户暂不支持，不得使用其他本地文件工具绕过该约束。只读数据工具可查询下载任务及详情、账号、浏览记录、应用日志和代理证书状态；列表结果支持分页，应优先使用筛选参数限制返回量。微信视频号工具可搜索账号、查询账号视频与直播回放、赞或收藏的视频、关注账号、播放记录、视频详情、评论及分享链接；这些工具依赖已连接的视频号页面，调用前可先使用 get_wxchannels_status。分页时把上一次响应的 lastBuffer 原样传给 next_marker。微信视频号 fetch_content 结果包含可供 aria2 等第三方下载器使用的 download_resources；第三方下载完成后，仅在 requires_decryption 为 true 时使用 decode_key 调用 decrypt_wxchannels_video。解密会原地覆盖文件。"
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
