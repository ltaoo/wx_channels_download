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
	ContentID    string
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
	Page     int
	PageSize int
	MaxBytes int
	Keyword  string
	Source   string
	Levels   []string
}

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

// DownloadTaskCreateRequest is the transport-neutral input for one task.
type DownloadTaskCreateRequest struct {
	Platform        string          `json:"platform"`
	Content         json.RawMessage `json:"content"`
	BuildFromFetch  bool            `json:"build_from_fetch"`
	ResourceIndexes []int           `json:"resource_indexes,omitempty"`
	DownloadDir     string          `json:"download_dir"`
	Filename        string          `json:"filename"`
	Config          map[string]any  `json:"config"`
	AutoStart       *bool           `json:"auto_start"`
	ParentTaskID    *int            `json:"parent_task_id,omitempty"`
	RelationType    string          `json:"relation_type,omitempty"`
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

type download_task_list_arguments struct {
	Page         int    `json:"page"`
	PageSize     int    `json:"page_size"`
	Statuses     []int  `json:"statuses"`
	ParentTaskID int    `json:"parent_task_id"`
	RootTaskID   int    `json:"root_task_id"`
	ContentID    string `json:"content_id"`
}

type download_task_detail_arguments struct {
	ID int `json:"id"`
}

type delete_download_tasks_arguments struct {
	TaskIDs     []int `json:"task_ids"`
	DeleteFiles bool  `json:"delete_files"`
}

type create_download_task_arguments struct {
	Platform        string          `json:"platform"`
	Content         json.RawMessage `json:"content"`
	BuildFromFetch  bool            `json:"build_from_fetch"`
	ResourceIndexes []int           `json:"resource_indexes"`
	DownloadDir     string          `json:"download_dir"`
	Filename        string          `json:"filename"`
	Config          map[string]any  `json:"config"`
	AutoStart       *bool           `json:"auto_start"`
	ParentTaskID    *int            `json:"parent_task_id"`
	RelationType    string          `json:"relation_type"`
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
		ContentID:    strings.TrimSpace(arguments.ContentID),
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
	if query.ContentID != "" {
		values.Set("content_id", query.ContentID)
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

func (s *Server) delete_download_tasks(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments delete_download_tasks_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	if len(arguments.TaskIDs) == 0 {
		return nil, fmt.Errorf("task_ids 不能为空")
	}
	for _, task_id := range arguments.TaskIDs {
		if task_id <= 0 {
			return nil, fmt.Errorf("task_ids 中的任务 ID 必须是正整数")
		}
	}
	results, err := s.download_task_deleter.DeleteDownloadTasks(ctx, arguments.TaskIDs, arguments.DeleteFiles)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(map[string]any{"results": results})
}

func (s *Server) create_download_task_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments create_download_task_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	platform := strings.TrimSpace(arguments.Platform)
	if platform == "" {
		return nil, fmt.Errorf("platform 不能为空")
	}
	if !has_json_value(arguments.Content) {
		return nil, fmt.Errorf("content 不能为空")
	}
	create_result, err := s.create_download_task(ctx, DownloadTaskCreateRequest{
		Platform:        platform,
		Content:         arguments.Content,
		BuildFromFetch:  arguments.BuildFromFetch,
		ResourceIndexes: arguments.ResourceIndexes,
		DownloadDir:     strings.TrimSpace(arguments.DownloadDir),
		Filename:        strings.TrimSpace(arguments.Filename),
		Config:          arguments.Config,
		AutoStart:       arguments.AutoStart,
		ParentTaskID:    arguments.ParentTaskID,
		RelationType:    strings.TrimSpace(arguments.RelationType),
	}, "创建下载任务失败")
	if err != nil {
		return nil, err
	}
	if create_result.Skipped {
		return successful_tool_result(map[string]any{
			"created":       false,
			"started":       false,
			"skipped":       true,
			"existing_task": create_result.Task,
		})
	}
	return successful_tool_result(map[string]any{
		"created": true,
		"started": true,
		"skipped": false,
		"task":    create_result.Task,
		"ids":     create_result.IDs,
	})
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
	values := url.Values{
		"page":      []string{strconv.Itoa(query.Page)},
		"page_size": []string{strconv.Itoa(query.PageSize)},
		"max_bytes": []string{strconv.Itoa(query.MaxBytes)},
		"keyword":   []string{query.Keyword},
		"source":    []string{query.Source},
		"levels":    []string{strings.Join(query.Levels, ",")},
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
