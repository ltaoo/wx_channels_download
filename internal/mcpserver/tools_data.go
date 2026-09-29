package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	default_data_page_size = 20
	max_data_page_size     = 200
	default_log_page_size  = 300
	max_log_page_size      = 2000
	default_log_max_bytes  = 2 * 1024 * 1024
	max_log_max_bytes      = 10 * 1024 * 1024
)

// DataReader supplies read-only data tools.
type DataReader interface {
	ListDownloadTasks(ctx context.Context, query DownloadTaskListQuery) (any, error)
	GetDownloadTaskDetail(ctx context.Context, task_id int) (any, error)
	ListAccounts(ctx context.Context, query AccountListQuery) (any, error)
	ListBrowseHistory(ctx context.Context, query BrowseHistoryListQuery) (any, error)
	ListLogs(ctx context.Context, query LogListQuery) (any, error)
	GetCertificateStatus(ctx context.Context) (any, error)
}

// DeleteDownloadTaskResult is one item in a batch deletion result.
type DeleteDownloadTaskResult struct {
	TaskID     int    `json:"task_id"`
	Success    bool   `json:"success"`
	StatusText string `json:"status_text,omitempty"`
	Error      string `json:"error,omitempty"`
}

// DownloadTaskDeleter supplies download task deletion tools.
type DownloadTaskDeleter interface {
	DeleteDownloadTasks(ctx context.Context, task_ids []int, delete_files bool) ([]DeleteDownloadTaskResult, error)
}

// DownloadTaskCreateResult is the normalized result consumed by MCP tools.
type DownloadTaskCreateResult struct {
	Task    any
	IDs     []int
	Skipped bool
}

// DownloadTaskCreator supplies process-local download task creation.
type DownloadTaskCreator interface {
	CreateDownloadTask(ctx context.Context, request DownloadTaskCreateRequest) (*DownloadTaskCreateResult, error)
}

type account_list_arguments struct {
	Page      int    `json:"page"`
	PageSize  int    `json:"page_size"`
	Keyword   string `json:"keyword"`
	AccountID string `json:"account_id"`
}

type browse_history_list_arguments struct {
	Page        int      `json:"page"`
	PageSize    int      `json:"page_size"`
	Keyword     string   `json:"keyword"`
	Username    string   `json:"username"`
	PlatformIDs []string `json:"platform_ids"`
}

type log_list_arguments struct {
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
	MaxBytes int      `json:"max_bytes"`
	Keyword  string   `json:"keyword"`
	Source   string   `json:"source"`
	Levels   []string `json:"levels"`
}

func (s *ToolSet) get_accounts(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments account_list_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	page, page_size, err := normalize_data_page_with_default(arguments.Page, arguments.PageSize, 24, max_data_page_size)
	if err != nil {
		return nil, err
	}
	query := AccountListQuery{
		Page:      page,
		PageSize:  page_size,
		Keyword:   strings.TrimSpace(arguments.Keyword),
		AccountID: strings.TrimSpace(arguments.AccountID),
	}
	if s.data_reader != nil {
		value, read_err := s.data_reader.ListAccounts(ctx, query)
		if read_err != nil {
			return nil, read_err
		}
		return successful_tool_result(value)
	}
	raw_response, err := s.api_client.accounts(ctx, query)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(raw_json_value(raw_response))
}

func (s *ToolSet) get_browse_history(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments browse_history_list_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	page, page_size, err := normalize_data_page(arguments.Page, arguments.PageSize, max_data_page_size)
	if err != nil {
		return nil, err
	}
	platform_ids := normalize_string_list(arguments.PlatformIDs)
	query := BrowseHistoryListQuery{
		Page:        page,
		PageSize:    page_size,
		Keyword:     strings.TrimSpace(arguments.Keyword),
		Username:    strings.TrimSpace(arguments.Username),
		PlatformIDs: platform_ids,
	}
	if s.data_reader != nil {
		value, read_err := s.data_reader.ListBrowseHistory(ctx, query)
		if read_err != nil {
			return nil, read_err
		}
		return successful_tool_result(value)
	}
	raw_response, err := s.api_client.browse_history(ctx, query)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(raw_json_value(raw_response))
}

func (s *ToolSet) get_logs(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments log_list_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	page, page_size, err := normalize_data_page_with_default(arguments.Page, arguments.PageSize, default_log_page_size, max_log_page_size)
	if err != nil {
		return nil, err
	}
	max_bytes := arguments.MaxBytes
	if max_bytes == 0 {
		max_bytes = default_log_max_bytes
	}
	if max_bytes < 64*1024 || max_bytes > max_log_max_bytes {
		return nil, fmt.Errorf("max_bytes 必须在 %d 到 %d 之间", 64*1024, max_log_max_bytes)
	}
	query := LogListQuery{
		Page:     page,
		PageSize: page_size,
		MaxBytes: max_bytes,
		Keyword:  strings.TrimSpace(arguments.Keyword),
		Source:   strings.TrimSpace(arguments.Source),
		Levels:   normalize_string_list(arguments.Levels),
	}
	if s.data_reader != nil {
		value, read_err := s.data_reader.ListLogs(ctx, query)
		if read_err != nil {
			return nil, read_err
		}
		return successful_tool_result(value)
	}
	raw_response, err := s.api_client.logs(ctx, query)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(raw_json_value(raw_response))
}

func (s *ToolSet) get_certificate_status(ctx context.Context) (map[string]any, error) {
	if s.data_reader != nil {
		value, err := s.data_reader.GetCertificateStatus(ctx)
		if err != nil {
			return nil, err
		}
		return successful_tool_result(value)
	}
	raw_response, err := s.api_client.certificate_status(ctx)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(raw_json_value(raw_response))
}

func normalize_data_page(page int, page_size int, maximum_page_size int) (int, int, error) {
	return normalize_data_page_with_default(page, page_size, default_data_page_size, maximum_page_size)
}

func normalize_data_page_with_default(page int, page_size int, default_page_size int, maximum_page_size int) (int, int, error) {
	if page == 0 {
		page = 1
	}
	if page_size == 0 {
		page_size = default_page_size
	}
	if page < 1 {
		return 0, 0, fmt.Errorf("page 必须是正整数")
	}
	if page_size < 1 || page_size > maximum_page_size {
		return 0, 0, fmt.Errorf("page_size 必须在 1 到 %d 之间", maximum_page_size)
	}
	return page, page_size, nil
}

func normalize_string_list(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		normalized = append(normalized, value)
	}
	return normalized
}

// tools_data declares the read-only data tools whose handlers live in this
// file. The row order here is filesystem-local only; the published order is
// fixed by the concatenation in tool_declarations (registry.go).
var tools_data = []tool{
	{
		name:         "get_accounts",
		title:        `获取账号列表`,
		description:  `分页查询已保存的平台账号，可按账号 ID 精确筛选，或按 ID、平台外部 ID、别名和昵称模糊搜索。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"account_id":{"description":"可选的数据库账号 ID。","type":"string"},"keyword":{"description":"账号搜索关键词。","type":"string"},"page":{"default":1,"description":"页码，从 1 开始。","maximum":1000000,"minimum":1,"type":"integer"},"page_size":{"default":24,"description":"每页账号数。","maximum":200,"minimum":1,"type":"integer"}},"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":true}`),
		supports:     supports_data,
		handle:       (*ToolSet).get_accounts,
	},
	{
		name:         "get_browse_history",
		title:        `获取浏览记录`,
		description:  `分页查询已保存的浏览记录，可按平台、关联账号和关键词筛选。username 对应账号的数据库 ID。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"keyword":{"description":"匹配标题、内容 ID、链接或关联账号的关键词。","type":"string"},"page":{"default":1,"description":"页码，从 1 开始。","maximum":1000000,"minimum":1,"type":"integer"},"page_size":{"default":20,"description":"每页记录数。","maximum":200,"minimum":1,"type":"integer"},"platform_ids":{"description":"平台 ID 列表；留空时查询常用平台。","items":{"minLength":1,"type":"string"},"type":"array","uniqueItems":true},"username":{"description":"关联账号的数据库 ID，例如 wxchannels:xxx。","type":"string"}},"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":true}`),
		supports:     supports_data,
		handle:       (*ToolSet).get_browse_history,
	},
	{
		name:         "get_logs",
		title:        `获取应用日志`,
		description:  `分页读取应用日志，可按级别、来源和关键词过滤。默认最多从日志末尾读取 2 MB。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"keyword":{"description":"不区分大小写的日志关键词。","type":"string"},"levels":{"description":"日志级别列表，例如 debug、info、warn、error。","items":{"minLength":1,"type":"string"},"type":"array","uniqueItems":true},"max_bytes":{"default":2097152,"description":"从每个日志文件末尾读取的最大字节数。","maximum":10485760,"minimum":65536,"type":"integer"},"page":{"default":1,"description":"页码，从 1 开始。","maximum":1000000,"minimum":1,"type":"integer"},"page_size":{"default":300,"description":"每页日志条数。","maximum":2000,"minimum":1,"type":"integer"},"source":{"description":"日志来源、文件或组件筛选。","type":"string"}},"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":true}`),
		supports:     supports_data,
		handle:       (*ToolSet).get_logs,
	},
	{
		name:         "get_certificate_status",
		title:        `获取代理证书状态`,
		description:  `获取当前代理根证书的来源、安装和信任状态、证书详情及风险提示。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":true}`),
		supports:     supports_data,
		handle:       without_arguments((*ToolSet).get_certificate_status),
	},
}
