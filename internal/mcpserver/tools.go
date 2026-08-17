package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

var err_unknown_tool = errors.New("未知工具")

type tool_execution_error struct {
	message string
	data    any
}

func (e *tool_execution_error) Error() string {
	return e.message
}

type fetch_content_arguments struct {
	URL            string `json:"url"`
	ForceRefresh   bool   `json:"force_refresh"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type download_content_arguments struct {
	JobID             string `json:"job_id"`
	FetchID           string `json:"fetch_id"`
	URL               string `json:"url"`
	DownloadDir       string `json:"download_dir"`
	Filename          string `json:"filename"`
	ForceRefresh      bool   `json:"force_refresh"`
	ExistingAction    string `json:"existing_action"`
	VideoVariantKey   string `json:"video_variant_key"`
	VideoVariantSpec  string `json:"video_variant_spec"`
	WaitForCompletion bool   `json:"wait_for_completion"`
	TimeoutSeconds    int    `json:"timeout_seconds"`
}

func new_tool_execution_error(message string, data any) error {
	return &tool_execution_error{message: message, data: data}
}

func tool_error_result(err error) map[string]any {
	message := err.Error()
	structured := map[string]any{"error": message}
	var execution_error *tool_execution_error
	if errors.As(err, &execution_error) && execution_error.data != nil {
		structured["details"] = execution_error.data
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": message}},
		"structuredContent": structured,
		"isError":           true,
	}
}

func tool_definitions() []any {
	return []any{
		map[string]any{
			"name":        "get_platform_status",
			"title":       "获取平台状态",
			"description": "获取下载器当前支持的抓取平台及各平台可用状态。解析或下载链接前可先调用此工具。",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
			},
			"annotations": map[string]any{
				"readOnlyHint":    true,
				"destructiveHint": false,
				"idempotentHint":  true,
				"openWorldHint":   false,
			},
		},
		map[string]any{
			"name":        "fetch_content",
			"title":       "获取链接内容",
			"description": "解析受支持的平台链接并返回规范化内容、账号、内容详情、缓存条目和下载预览。返回的 job_id 可传给 download_content，避免再次解析链接。",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"url": map[string]any{
						"type":        "string",
						"format":      "uri",
						"description": "要解析的平台内容链接。",
					},
					"force_refresh": map[string]any{
						"type":        "boolean",
						"default":     false,
						"description": "忽略抓取缓存并重新获取。",
					},
					"timeout_seconds": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"maximum":     3600,
						"default":     300,
						"description": "等待链接解析完成的最长秒数。",
					},
				},
				"required": []string{"url"},
			},
			"annotations": map[string]any{
				"readOnlyHint":    true,
				"destructiveHint": false,
				"idempotentHint":  true,
				"openWorldHint":   true,
			},
		},
		map[string]any{
			"name":        "download_content",
			"title":       "下载内容",
			"description": "根据 fetch_content 返回的 job_id 创建并启动下载任务；也可直接传 url，此时会先解析链接。默认在任务启动后返回，设置 wait_for_completion 可等待文件下载完成。此工具会写入下载目录。",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"job_id": map[string]any{
						"type":        "string",
						"description": "fetch_content 返回的 job_id；优先使用它以复用解析结果。",
					},
					"fetch_id": map[string]any{
						"type":        "string",
						"description": "job_id 的旧名称，仅用于兼容已有调用。",
						"deprecated":  true,
					},
					"url": map[string]any{
						"type":        "string",
						"format":      "uri",
						"description": "未提供 job_id 时，要解析并下载的平台内容链接。",
					},
					"download_dir": map[string]any{
						"type":        "string",
						"description": "下载目录；留空时使用应用配置。",
					},
					"filename": map[string]any{
						"type":        "string",
						"description": "可选的自定义文件名。",
					},
					"force_refresh": map[string]any{
						"type":        "boolean",
						"default":     false,
						"description": "直接传 url 时忽略抓取缓存。",
					},
					"existing_action": map[string]any{
						"type":        "string",
						"enum":        []string{"error", "skip", "overwrite", "duplicate"},
						"default":     "error",
						"description": "遇到相同任务时的处理方式。overwrite 会覆盖已有任务及文件。",
					},
					"video_variant_key": map[string]any{
						"type":        "string",
						"description": "可选的视频规格 variant_key。",
					},
					"video_variant_spec": map[string]any{
						"type":        "string",
						"description": "可选的视频规格名称；同时兼容适配器的 spec 配置。",
					},
					"wait_for_completion": map[string]any{
						"type":        "boolean",
						"default":     false,
						"description": "是否等待实际下载结束后再返回。",
					},
					"timeout_seconds": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"maximum":     86400,
						"default":     300,
						"description": "解析链接以及可选等待下载完成的最长秒数。",
					},
				},
				"anyOf": []any{
					map[string]any{"required": []string{"job_id"}},
					map[string]any{"required": []string{"fetch_id"}},
					map[string]any{"required": []string{"url"}},
				},
			},
			"annotations": map[string]any{
				"readOnlyHint":    false,
				"destructiveHint": true,
				"idempotentHint":  false,
				"openWorldHint":   true,
			},
		},
	}
}

func (s *Server) call_tool(ctx context.Context, params call_tool_params) (map[string]any, error) {
	switch params.Name {
	case "get_platform_status":
		return s.get_platform_status(ctx)
	case "fetch_content":
		return s.fetch_content(ctx, params.Arguments)
	case "download_content":
		return s.download_content(ctx, params.Arguments)
	default:
		return nil, fmt.Errorf("%w: %s", err_unknown_tool, params.Name)
	}
}

func (s *Server) get_platform_status(ctx context.Context) (map[string]any, error) {
	raw_status, err := s.api_client.get_platform_status(ctx)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(raw_json_value(raw_status))
}

func (s *Server) fetch_content(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments fetch_content_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	arguments.URL = strings.TrimSpace(arguments.URL)
	if err := validate_source_url(arguments.URL); err != nil {
		return nil, err
	}
	timeout, err := timeout_duration(arguments.TimeoutSeconds, 300, 3600)
	if err != nil {
		return nil, err
	}
	fetch_context, cancel_fetch := context.WithTimeout(ctx, timeout)
	defer cancel_fetch()
	job, err := s.api_client.create_scraper_job(fetch_context, arguments.URL, arguments.ForceRefresh)
	if err != nil {
		return nil, err
	}
	job, err = s.api_client.wait_scraper_job(fetch_context, job)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(raw_json_value(job.Output))
}

func (s *Server) download_content(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments download_content_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	arguments.JobID = strings.TrimSpace(arguments.JobID)
	arguments.FetchID = strings.TrimSpace(arguments.FetchID)
	arguments.URL = strings.TrimSpace(arguments.URL)
	if arguments.JobID != "" && arguments.FetchID != "" && arguments.JobID != arguments.FetchID {
		return nil, fmt.Errorf("job_id 与 fetch_id 不能指向不同任务")
	}
	if arguments.JobID == "" {
		arguments.JobID = arguments.FetchID
	}
	if arguments.JobID == "" && arguments.URL == "" {
		return nil, fmt.Errorf("job_id 和 url 至少需要提供一个")
	}
	if arguments.URL != "" {
		if err := validate_source_url(arguments.URL); err != nil {
			return nil, err
		}
	}
	existing_action := strings.TrimSpace(arguments.ExistingAction)
	if existing_action == "" {
		existing_action = "error"
	}
	if !is_existing_action(existing_action) {
		return nil, fmt.Errorf("existing_action 必须是 error、skip、overwrite 或 duplicate")
	}
	timeout, err := timeout_duration(arguments.TimeoutSeconds, 300, 86400)
	if err != nil {
		return nil, err
	}
	download_context, cancel_download := context.WithTimeout(ctx, timeout)
	defer cancel_download()

	job, err := s.resolve_download_job(download_context, arguments)
	if err != nil {
		return nil, err
	}
	var output scraper_output
	if err := json.Unmarshal(job.Output, &output); err != nil {
		return nil, fmt.Errorf("解析抓取结果失败: %w", err)
	}
	if strings.TrimSpace(output.Platform) == "" || !has_json_value(output.Result) {
		return nil, fmt.Errorf("抓取结果缺少 platform 或 result")
	}

	config := map[string]any{
		"platform":        output.Platform,
		"existing_action": existing_action,
	}
	if value := strings.TrimSpace(arguments.VideoVariantKey); value != "" {
		config["video_variant_key"] = value
	}
	if value := strings.TrimSpace(arguments.VideoVariantSpec); value != "" {
		config["video_variant_spec"] = value
		config["spec"] = value
	}
	if existing_action == "overwrite" {
		config["overwrite"] = true
	}
	if existing_action == "duplicate" {
		config["duplicate"] = true
	}
	request_body := map[string]any{
		"objects": []any{map[string]any{
			"platform":         output.Platform,
			"content":          output.Result,
			"build_from_fetch": has_json_value(output.DownloadInfo),
			"download_dir":     strings.TrimSpace(arguments.DownloadDir),
			"filename":         strings.TrimSpace(arguments.Filename),
			"config":           config,
			"auto_start":       true,
		}},
	}
	create_response, err := s.api_client.create_download_task(download_context, request_body)
	if err != nil {
		return nil, err
	}
	item := create_response.Tasks[0]
	if item.Code != 0 {
		return nil, new_tool_execution_error(value_or_default(item.Msg, "创建下载任务失败"), raw_json_value(item.Data))
	}
	if existing_action == "skip" && download_item_was_skipped(item.Data) {
		return successful_tool_result(map[string]any{
			"created":       false,
			"started":       false,
			"skipped":       true,
			"existing_task": raw_json_value(item.Data),
			"source":        download_source(job, output),
		})
	}

	task_value := raw_json_value(item.Data)
	result := map[string]any{
		"created": true,
		"started": true,
		"skipped": false,
		"task":    task_value,
		"ids":     create_response.IDs,
		"source":  download_source(job, output),
	}
	if arguments.WaitForCompletion {
		task_id := first_download_task_id(create_response, item.Data)
		if task_id <= 0 {
			return nil, fmt.Errorf("下载任务响应缺少 id，无法等待完成")
		}
		completed_task, err := s.api_client.wait_download_task(download_context, task_id)
		if err != nil {
			return nil, err
		}
		result["completed"] = true
		result["task"] = completed_task
	}
	return successful_tool_result(result)
}

func (s *Server) resolve_download_job(ctx context.Context, arguments download_content_arguments) (*scraper_job, error) {
	if arguments.JobID != "" {
		job, err := s.api_client.get_scraper_job(ctx, arguments.JobID)
		if err != nil {
			return nil, fmt.Errorf("读取 job_id %s 失败: %w；可改为传入 url 重新解析", arguments.JobID, err)
		}
		return s.api_client.wait_scraper_job(ctx, job)
	}
	job, err := s.api_client.create_scraper_job(ctx, arguments.URL, arguments.ForceRefresh)
	if err != nil {
		return nil, err
	}
	return s.api_client.wait_scraper_job(ctx, job)
}

func successful_tool_result(value any) (map[string]any, error) {
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

func decode_tool_arguments(raw json.RawMessage, destination any) error {
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

func download_source(job *scraper_job, output scraper_output) map[string]any {
	return map[string]any{
		"job_id":   job.ID,
		"platform": output.Platform,
		"url":      output.URL,
	}
}

func first_download_task_id(response *download_create_response, raw_task json.RawMessage) int {
	if response != nil && len(response.IDs) > 0 {
		return response.IDs[0]
	}
	var task struct {
		ID int `json:"id"`
	}
	_ = json.Unmarshal(raw_task, &task)
	return task.ID
}

func download_item_was_skipped(raw_task json.RawMessage) bool {
	var task struct {
		Skipped bool `json:"skipped"`
	}
	return json.Unmarshal(raw_task, &task) == nil && task.Skipped
}
