package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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

// DownloadTaskListQuery describes a read-only download task query.
type DownloadTaskListQuery struct {
	Page         int
	PageSize     int
	Statuses     []int
	ParentTaskID int
	RootTaskID   int
}

// AccountListQuery describes a read-only account query.
type AccountListQuery struct {
	Page      int
	PageSize  int
	Keyword   string
	AccountID string
}

// BrowseHistoryListQuery describes a read-only browse history query.
type BrowseHistoryListQuery struct {
	Page        int
	PageSize    int
	Keyword     string
	Username    string
	PlatformIDs []string
}

// LogListQuery describes a read-only application log query.
type LogListQuery struct {
	Page       int
	PageSize   int
	MaxBytes   int
	Keyword    string
	Source     string
	Levels     []string
	FormatJSON bool
}

// DataReader supplies process-local read access for MCP data tools. The
// embedded MCP server uses the application's DB and runtime services; stdio
// servers may omit it and fall back to the downloader HTTP API.
type DataReader interface {
	ListDownloadTasks(ctx context.Context, query DownloadTaskListQuery) (any, error)
	GetDownloadTaskDetail(ctx context.Context, task_id int) (any, error)
	ListAccounts(ctx context.Context, query AccountListQuery) (any, error)
	ListBrowseHistory(ctx context.Context, query BrowseHistoryListQuery) (any, error)
	ListLogs(ctx context.Context, query LogListQuery) (any, error)
	GetCertificateStatus(ctx context.Context) (any, error)
}

type download_task_list_arguments struct {
	Page         int   `json:"page"`
	PageSize     int   `json:"page_size"`
	Statuses     []int `json:"statuses"`
	ParentTaskID int   `json:"parent_task_id"`
	RootTaskID   int   `json:"root_task_id"`
}

type download_task_detail_arguments struct {
	ID int `json:"id"`
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
	Page       int      `json:"page"`
	PageSize   int      `json:"page_size"`
	MaxBytes   int      `json:"max_bytes"`
	Keyword    string   `json:"keyword"`
	Source     string   `json:"source"`
	Levels     []string `json:"levels"`
	FormatJSON bool     `json:"format_json"`
}

func data_tool_definitions() []any {
	return []any{
		data_tool_definition(
			"get_download_tasks",
			"获取下载任务",
			"分页查询下载任务及状态统计，可按任务状态、父任务或根任务筛选。状态值：0 等待、1 准备、2 下载中、3 暂停、4 合并、5 完成、6 失败、7 取消。",
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"page":      data_page_schema(1, 1000000, 1, "页码，从 1 开始。"),
					"page_size": data_page_schema(1, 100, 20, "每页任务数。"),
					"statuses": map[string]any{
						"type":        "array",
						"description": "可选的任务状态列表。",
						"uniqueItems": true,
						"items": map[string]any{
							"type":    "integer",
							"minimum": 0,
							"maximum": 7,
						},
					},
					"parent_task_id": data_positive_id_schema("只返回该父任务的直接子任务。"),
					"root_task_id":   data_positive_id_schema("只返回属于该根任务的任务。"),
				},
			},
		),
		data_tool_definition(
			"get_download_task_detail",
			"获取下载任务详情",
			"按下载任务 ID 获取任务、文件、进度、关联内容和账号详情。",
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"id": data_positive_id_schema("下载任务 ID。"),
				},
				"required": []string{"id"},
			},
		),
		data_tool_definition(
			"get_accounts",
			"获取账号列表",
			"分页查询已保存的平台账号，可按账号 ID 精确筛选，或按 ID、平台外部 ID、别名和昵称模糊搜索。",
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"page":      data_page_schema(1, 1000000, 1, "页码，从 1 开始。"),
					"page_size": data_page_schema(1, 200, 24, "每页账号数。"),
					"keyword": map[string]any{
						"type":        "string",
						"description": "账号搜索关键词。",
					},
					"account_id": map[string]any{
						"type":        "string",
						"description": "可选的数据库账号 ID。",
					},
				},
			},
		),
		data_tool_definition(
			"get_browse_history",
			"获取浏览记录",
			"分页查询已保存的浏览记录，可按平台、关联账号和关键词筛选。username 对应账号的数据库 ID。",
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"page":      data_page_schema(1, 1000000, 1, "页码，从 1 开始。"),
					"page_size": data_page_schema(1, 200, 20, "每页记录数。"),
					"keyword": map[string]any{
						"type":        "string",
						"description": "匹配标题、内容 ID、链接或关联账号的关键词。",
					},
					"username": map[string]any{
						"type":        "string",
						"description": "关联账号的数据库 ID，例如 wxchannels:xxx。",
					},
					"platform_ids": map[string]any{
						"type":        "array",
						"description": "平台 ID 列表；留空时查询常用平台。",
						"uniqueItems": true,
						"items":       map[string]any{"type": "string", "minLength": 1},
					},
				},
			},
		),
		data_tool_definition(
			"get_logs",
			"获取应用日志",
			"分页读取应用日志，可按级别、来源和关键词过滤。默认最多从日志末尾读取 2 MB。",
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"page":      data_page_schema(1, 1000000, 1, "页码，从 1 开始。"),
					"page_size": data_page_schema(1, max_log_page_size, default_log_page_size, "每页日志条数。"),
					"max_bytes": data_page_schema(64*1024, max_log_max_bytes, default_log_max_bytes, "从每个日志文件末尾读取的最大字节数。"),
					"keyword": map[string]any{
						"type":        "string",
						"description": "不区分大小写的日志关键词。",
					},
					"source": map[string]any{
						"type":        "string",
						"description": "日志来源、文件或组件筛选。",
					},
					"levels": map[string]any{
						"type":        "array",
						"description": "日志级别列表，例如 debug、info、warn、error。",
						"uniqueItems": true,
						"items":       map[string]any{"type": "string", "minLength": 1},
					},
					"format_json": map[string]any{
						"type":        "boolean",
						"default":     false,
						"description": "是否为 JSON 日志附加格式化文本。",
					},
				},
			},
		),
		data_tool_definition(
			"get_certificate_status",
			"获取代理证书状态",
			"获取当前代理根证书的来源、安装和信任状态、证书详情及风险提示。",
			map[string]any{
				"type":                 "object",
				"additionalProperties": false,
			},
		),
	}
}

func data_tool_definition(name string, title string, description string, input_schema map[string]any) map[string]any {
	return map[string]any{
		"name":        name,
		"title":       title,
		"description": description,
		"inputSchema": input_schema,
		"annotations": map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
			"openWorldHint":   false,
		},
	}
}

func data_page_schema(minimum int, maximum int, default_value int, description string) map[string]any {
	return map[string]any{
		"type":        "integer",
		"minimum":     minimum,
		"maximum":     maximum,
		"default":     default_value,
		"description": description,
	}
}

func data_positive_id_schema(description string) map[string]any {
	return map[string]any{
		"type":        "integer",
		"minimum":     1,
		"description": description,
	}
}

func (s *Server) get_download_tasks(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments download_task_list_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	page, page_size, err := normalize_data_page(arguments.Page, arguments.PageSize, 100)
	if err != nil {
		return nil, err
	}
	for _, status := range arguments.Statuses {
		if status < 0 || status > 7 {
			return nil, fmt.Errorf("statuses 中的状态值必须在 0 到 7 之间")
		}
	}
	if arguments.ParentTaskID < 0 || arguments.RootTaskID < 0 {
		return nil, fmt.Errorf("parent_task_id 和 root_task_id 不能为负数")
	}
	query := DownloadTaskListQuery{
		Page:         page,
		PageSize:     page_size,
		Statuses:     arguments.Statuses,
		ParentTaskID: arguments.ParentTaskID,
		RootTaskID:   arguments.RootTaskID,
	}
	if s.data_reader != nil {
		value, read_err := s.data_reader.ListDownloadTasks(ctx, query)
		if read_err != nil {
			return nil, read_err
		}
		return successful_tool_result(value)
	}
	values := url.Values{
		"page":      []string{strconv.Itoa(query.Page)},
		"page_size": []string{strconv.Itoa(query.PageSize)},
	}
	if query.ParentTaskID > 0 {
		values.Set("parent_task_id", strconv.Itoa(query.ParentTaskID))
	}
	if query.RootTaskID > 0 {
		values.Set("root_task_id", strconv.Itoa(query.RootTaskID))
	}
	if len(query.Statuses) > 0 {
		values.Set("status", join_ints(query.Statuses))
	}
	return s.call_read_api(ctx, http.MethodGet, "/api/v1/download_task/list?"+values.Encode(), nil)
}

func (s *Server) get_download_task_detail(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments download_task_detail_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	if arguments.ID <= 0 {
		return nil, fmt.Errorf("id 必须是正整数")
	}
	if s.data_reader != nil {
		value, read_err := s.data_reader.GetDownloadTaskDetail(ctx, arguments.ID)
		if read_err != nil {
			return nil, read_err
		}
		if value == nil {
			return nil, fmt.Errorf("下载任务不存在: %d", arguments.ID)
		}
		return successful_tool_result(value)
	}
	values := url.Values{"id": []string{strconv.Itoa(arguments.ID)}}
	return s.call_read_api(ctx, http.MethodGet, "/api/v1/download_task/detail?"+values.Encode(), nil)
}

func (s *Server) get_accounts(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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
	values := url.Values{
		"page":       []string{strconv.Itoa(query.Page)},
		"page_size":  []string{strconv.Itoa(query.PageSize)},
		"keyword":    []string{query.Keyword},
		"account_id": []string{query.AccountID},
	}
	return s.call_read_api(ctx, http.MethodGet, "/api/account/list?"+values.Encode(), nil)
}

func (s *Server) get_browse_history(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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
	body := map[string]any{
		"page":         query.Page,
		"page_size":    query.PageSize,
		"keyword":      query.Keyword,
		"platform_ids": query.PlatformIDs,
	}
	if query.Username != "" {
		body["username"] = query.Username
	}
	return s.call_read_api(ctx, http.MethodPost, "/api/browse_history/list", body)
}

func (s *Server) get_logs(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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
		Page:       page,
		PageSize:   page_size,
		MaxBytes:   max_bytes,
		Keyword:    strings.TrimSpace(arguments.Keyword),
		Source:     strings.TrimSpace(arguments.Source),
		Levels:     normalize_string_list(arguments.Levels),
		FormatJSON: arguments.FormatJSON,
	}
	if s.data_reader != nil {
		value, read_err := s.data_reader.ListLogs(ctx, query)
		if read_err != nil {
			return nil, read_err
		}
		return successful_tool_result(value)
	}
	values := url.Values{
		"page":        []string{strconv.Itoa(query.Page)},
		"page_size":   []string{strconv.Itoa(query.PageSize)},
		"max_bytes":   []string{strconv.Itoa(query.MaxBytes)},
		"keyword":     []string{query.Keyword},
		"source":      []string{query.Source},
		"levels":      []string{strings.Join(query.Levels, ",")},
		"format_json": []string{strconv.FormatBool(query.FormatJSON)},
	}
	return s.call_read_api(ctx, http.MethodGet, "/api/logs?"+values.Encode(), nil)
}

func (s *Server) get_certificate_status(ctx context.Context) (map[string]any, error) {
	if s.data_reader != nil {
		value, err := s.data_reader.GetCertificateStatus(ctx)
		if err != nil {
			return nil, err
		}
		return successful_tool_result(value)
	}
	return s.call_read_api(ctx, http.MethodGet, "/api/proxy/certificate/status", nil)
}

func (s *Server) call_read_api(ctx context.Context, method string, path string, body any) (map[string]any, error) {
	raw_response, err := s.api_client.do_json(ctx, method, path, body)
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

func join_ints(values []int) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = strconv.Itoa(value)
	}
	return strings.Join(parts, ",")
}
