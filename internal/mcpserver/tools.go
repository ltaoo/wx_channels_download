package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	servicetools "wx_channel/internal/services/tools"
	mcp "wx_channel/pkg/mcp"
)

type ToolDefinition = servicetools.Definition
type ToolFormField = servicetools.FormField
type ToolFormOption = servicetools.FormOption

// tool_execution_error is a true alias so errors.As in mcp.ErrorResult keeps
// matching errors returned by this package's tool handlers.
type tool_execution_error = mcp.ToolError

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

type decrypt_wxchannels_video_arguments struct {
	FilePath string `json:"file_path"`
	Key      string `json:"key"`
}

type update_config_arguments struct {
	Values map[string]any `json:"values"`
}

type get_restart_status_arguments struct {
	RestartToken string `json:"restart_token"`
}

type wxchannels_download_preview struct {
	Resources []wxchannels_download_resource_info `json:"Resources"`
}

type wxchannels_download_resource_info struct {
	Resource  map[string]any                 `json:"Resource"`
	Endpoints []wxchannels_download_endpoint `json:"Endpoints"`
}

type wxchannels_download_endpoint struct {
	Protocol string `json:"protocol"`
	URL      string `json:"url"`
	Priority int    `json:"priority"`
	Enabled  int    `json:"enabled"`
	Headers  string `json:"headers,omitempty"`
	Cookies  string `json:"cookies,omitempty"`
}

func new_tool_execution_error(message string, data any) error {
	return mcp.NewToolError(message, data)
}

// ToolNames returns the MCP tool names exposed by this server.
func ToolNames() []string {
	catalog := ToolCatalog()
	names := make([]string, 0, len(catalog))
	for _, definition := range catalog {
		names = append(names, definition.Name)
	}
	return names
}

// ToolCatalog returns every tool registered by tools.go and its extension
// definition files. It is used by the automation editor to configure service
// nodes without maintaining a second tool list.
func ToolCatalog() []ToolDefinition {
	return servicetools.BuiltinCatalog()
}

// ToolCatalog returns the tools enabled by this server's configured service
// backends. CLI and other process-local callers use the same catalog as MCP.
func (s *ToolSet) ToolCatalog() []ToolDefinition {
	if s == nil || s.tool_service == nil {
		return []ToolDefinition{}
	}
	return s.tool_service.Definitions()
}

// ToolNames returns the tools enabled by this server's tool service.
func (s *ToolSet) ToolNames() []string {
	if s == nil || s.tool_service == nil {
		return []string{}
	}
	return s.tool_service.Names()
}

// ToolService exposes the transport-neutral registry used internally by MCP.
// Process-local adapters such as the CLI can invoke it without speaking MCP.
func (s *ToolSet) ToolService() *servicetools.Service {
	if s == nil {
		return nil
	}
	return s.tool_service
}

// ExecuteTool invokes an MCP tool directly and returns its structured result.
// This shares the exact same validation and dispatch path as tools/call.
func (s *ToolSet) ExecuteTool(ctx context.Context, name string, arguments map[string]any) (any, error) {
	if s == nil || s.tool_service == nil {
		return nil, errors.New("工具服务未初始化")
	}
	return s.tool_service.Execute(ctx, name, arguments)
}

func (s *ToolSet) get_config(ctx context.Context) (map[string]any, error) {
	raw_config, err := s.api_client.get_config(ctx)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(raw_json_value(raw_config))
}

func (s *ToolSet) update_config(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments update_config_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	if len(arguments.Values) == 0 {
		return nil, fmt.Errorf("values 至少需要包含一个配置项")
	}
	raw_result, err := s.api_client.update_config(ctx, arguments.Values)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(raw_json_value(raw_result))
}

func (s *ToolSet) get_restart_status(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments get_restart_status_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	arguments.RestartToken = strings.TrimSpace(arguments.RestartToken)
	if arguments.RestartToken == "" {
		return nil, fmt.Errorf("restart_token 不能为空")
	}
	raw_result, err := s.api_client.get_restart_status(ctx, arguments.RestartToken)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(raw_json_value(raw_result))
}

func (s *ToolSet) get_platform_status(ctx context.Context) (map[string]any, error) {
	raw_status, err := s.api_client.get_platform_status(ctx)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(raw_json_value(raw_status))
}

func (s *ToolSet) fetch_content(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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
	job, err := s.create_scraper_job(fetch_context, arguments.URL, arguments.ForceRefresh)
	if err != nil {
		return nil, err
	}
	job, err = s.wait_scraper_job(fetch_context, job)
	if err != nil {
		return nil, err
	}
	output, err := fetch_content_output(job.Output)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(output)
}

func fetch_content_output(raw_output json.RawMessage) (any, error) {
	var output map[string]any
	if err := json.Unmarshal(raw_output, &output); err != nil {
		return nil, fmt.Errorf("解析抓取结果失败: %w", err)
	}
	platform, _ := output["platform"].(string)
	if strings.TrimSpace(strings.ToLower(platform)) != "wxchannels" {
		return output, nil
	}
	download_info, exists := output["download_info"]
	if !exists || download_info == nil {
		return output, nil
	}
	raw_download_info, err := json.Marshal(download_info)
	if err != nil {
		return nil, fmt.Errorf("编码微信视频号下载资源失败: %w", err)
	}
	resources, err := normalize_wxchannels_download_resources(raw_download_info)
	if err != nil {
		return nil, err
	}
	output["download_resources"] = resources
	return output, nil
}

func normalize_wxchannels_download_resources(raw_download_info json.RawMessage) ([]map[string]any, error) {
	var preview wxchannels_download_preview
	if err := json.Unmarshal(raw_download_info, &preview); err != nil {
		return nil, fmt.Errorf("解析微信视频号下载资源失败: %w", err)
	}
	resources := make([]map[string]any, 0, len(preview.Resources))
	for _, info := range preview.Resources {
		download_url := preferred_download_url(info.Endpoints)
		decode_key := resource_decode_key(info.Resource)
		resources = append(resources, map[string]any{
			"resource":            info.Resource,
			"endpoints":           info.Endpoints,
			"download_url":        download_url,
			"decode_key":          decode_key,
			"requires_decryption": decode_key != "",
		})
	}
	return resources, nil
}

func preferred_download_url(endpoints []wxchannels_download_endpoint) string {
	for _, endpoint := range endpoints {
		if endpoint.Enabled != 0 && strings.TrimSpace(endpoint.URL) != "" {
			return endpoint.URL
		}
	}
	for _, endpoint := range endpoints {
		if strings.TrimSpace(endpoint.URL) != "" {
			return endpoint.URL
		}
	}
	return ""
}

func resource_decode_key(resource map[string]any) string {
	raw_extra, ok := resource["extra"]
	if !ok {
		raw_extra = resource["Extra"]
	}
	var extra map[string]any
	switch value := raw_extra.(type) {
	case string:
		if json.Unmarshal([]byte(value), &extra) != nil {
			return ""
		}
	case map[string]any:
		extra = value
	default:
		return ""
	}
	decode_key, _ := extra["decode_key"].(string)
	return strings.TrimSpace(decode_key)
}

func (s *ToolSet) decrypt_wxchannels_video(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments decrypt_wxchannels_video_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	file_path := strings.TrimSpace(arguments.FilePath)
	if file_path == "" {
		return nil, fmt.Errorf("file_path 不能为空")
	}
	if !filepath.IsAbs(file_path) {
		return nil, fmt.Errorf("file_path 必须是运行下载器服务所在机器上的绝对路径")
	}
	decode_key := strings.TrimSpace(arguments.Key)
	key, err := strconv.ParseUint(decode_key, 10, 64)
	if err != nil || key == 0 {
		return nil, fmt.Errorf("key 必须是非零十进制整数")
	}
	if _, err := s.api_client.decrypt_wxchannels_video(ctx, file_path, decode_key); err != nil {
		return nil, err
	}
	return successful_tool_result(map[string]any{
		"decrypted": true,
		"file_path": file_path,
	})
}

func (s *ToolSet) download_content(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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
	auto_start := true
	create_result, err := s.create_download_task(download_context, DownloadTaskCreateRequest{
		Platform:       output.Platform,
		Content:        output.Result,
		BuildFromFetch: has_json_value(output.DownloadInfo),
		DownloadDir:    strings.TrimSpace(arguments.DownloadDir),
		Filename:       strings.TrimSpace(arguments.Filename),
		Config:         config,
		AutoStart:      &auto_start,
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
			"source":        download_source(job, output),
		})
	}

	result := map[string]any{
		"created": true,
		"started": true,
		"skipped": false,
		"task":    create_result.Task,
		"ids":     create_result.IDs,
		"source":  download_source(job, output),
	}
	if arguments.WaitForCompletion {
		task_id := first_download_task_id(create_result)
		if task_id <= 0 {
			return nil, fmt.Errorf("下载任务响应缺少 id，无法等待完成")
		}
		completed_task, err := s.wait_download_task(download_context, task_id)
		if err != nil {
			return nil, err
		}
		result["completed"] = true
		result["task"] = completed_task
	}
	return successful_tool_result(result)
}

func (s *ToolSet) resolve_download_job(ctx context.Context, arguments download_content_arguments) (*ScraperJob, error) {
	if arguments.JobID != "" {
		job, err := s.get_scraper_job(ctx, arguments.JobID)
		if err != nil {
			return nil, fmt.Errorf("读取 job_id %s 失败: %w；可改为传入 url 重新解析", arguments.JobID, err)
		}
		return s.wait_scraper_job(ctx, job)
	}
	job, err := s.create_scraper_job(ctx, arguments.URL, arguments.ForceRefresh)
	if err != nil {
		return nil, err
	}
	return s.wait_scraper_job(ctx, job)
}

func (s *ToolSet) create_scraper_job(ctx context.Context, raw_url string, force_refresh bool) (*ScraperJob, error) {
	if s.scraper_jobs != nil {
		return s.scraper_jobs.CreateScraperJob(ctx, raw_url, force_refresh)
	}
	if s.api_client == nil {
		return nil, fmt.Errorf("抓取任务服务未初始化")
	}
	return s.api_client.create_scraper_job(ctx, raw_url, force_refresh)
}

func (s *ToolSet) get_scraper_job(ctx context.Context, job_id string) (*ScraperJob, error) {
	if s.scraper_jobs != nil {
		return s.scraper_jobs.GetScraperJob(ctx, job_id)
	}
	if s.api_client == nil {
		return nil, fmt.Errorf("抓取任务服务未初始化")
	}
	return s.api_client.get_scraper_job(ctx, job_id)
}

func (s *ToolSet) wait_scraper_job(ctx context.Context, job *ScraperJob) (*ScraperJob, error) {
	if job == nil || strings.TrimSpace(job.ID) == "" {
		return nil, fmt.Errorf("抓取任务响应缺少 id")
	}
	poll_interval := default_poll_interval
	if s.api_client != nil && s.api_client.poll_interval > 0 {
		poll_interval = s.api_client.poll_interval
	}
	poll_timer := time.NewTimer(poll_interval)
	defer poll_timer.Stop()
	current_job := job
	for {
		switch current_job.Status {
		case "completed":
			if !has_json_value(current_job.Output) {
				return nil, fmt.Errorf("抓取任务已完成，但响应缺少 output")
			}
			return current_job, nil
		case "failed":
			return nil, new_tool_execution_error(value_or_default(current_job.Error, "抓取内容失败"), raw_json_value(current_job.Progress))
		case "interrupted":
			return nil, new_tool_execution_error(value_or_default(current_job.Error, "抓取任务已中断"), raw_json_value(current_job.Progress))
		}

		select {
		case <-ctx.Done():
			if s.scraper_jobs != nil {
				s.scraper_jobs.InterruptScraperJob(current_job.ID)
			}
			return nil, new_tool_execution_error("等待抓取任务超时或已取消: "+ctx.Err().Error(), raw_json_value(current_job.Progress))
		case <-poll_timer.C:
		}
		next_job, err := s.get_scraper_job(ctx, current_job.ID)
		if err != nil {
			return nil, err
		}
		current_job = next_job
		poll_timer.Reset(poll_interval)
	}
}

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

func download_source(job *ScraperJob, output scraper_output) map[string]any {
	return map[string]any{
		"job_id":   job.ID,
		"platform": output.Platform,
		"url":      output.URL,
	}
}

func (s *ToolSet) create_download_task(ctx context.Context, request DownloadTaskCreateRequest, fallback_message string) (*DownloadTaskCreateResult, error) {
	if s.download_task_creator != nil {
		return s.download_task_creator.CreateDownloadTask(ctx, request)
	}
	if s.api_client == nil {
		return nil, fmt.Errorf("下载任务创建服务未初始化")
	}
	create_response, err := s.api_client.create_download_task(ctx, map[string]any{
		"objects": []DownloadTaskCreateRequest{request},
	})
	if err != nil {
		return nil, err
	}
	item := create_response.Tasks[0]
	if item.Code != 0 {
		return nil, new_tool_execution_error(value_or_default(item.Msg, fallback_message), raw_json_value(item.Data))
	}
	return &DownloadTaskCreateResult{
		Task:    raw_json_value(item.Data),
		IDs:     create_response.IDs,
		Skipped: download_item_was_skipped(item.Data),
	}, nil
}

func (s *ToolSet) wait_download_task(ctx context.Context, task_id int) (any, error) {
	if s.data_reader == nil {
		if s.api_client == nil {
			return nil, fmt.Errorf("下载任务查询服务未初始化")
		}
		return s.api_client.wait_download_task(ctx, task_id)
	}
	poll_interval := default_poll_interval
	if s.api_client != nil && s.api_client.poll_interval > 0 {
		poll_interval = s.api_client.poll_interval
	}
	poll_ticker := time.NewTicker(poll_interval)
	defer poll_ticker.Stop()
	for {
		task, err := s.data_reader.GetDownloadTaskDetail(ctx, task_id)
		if err != nil {
			return nil, err
		}
		raw_task, err := json.Marshal(task)
		if err != nil {
			return nil, fmt.Errorf("解析下载进度响应失败: %w", err)
		}
		if !has_json_value(raw_task) {
			return nil, fmt.Errorf("下载任务不存在: %d", task_id)
		}
		var status struct {
			Status int    `json:"status"`
			Error  string `json:"error"`
		}
		if err := json.Unmarshal(raw_task, &status); err != nil {
			return nil, fmt.Errorf("解析下载进度响应失败: %w", err)
		}
		switch status.Status {
		case 5:
			return task, nil
		case 6, 7:
			return nil, new_tool_execution_error(value_or_default(status.Error, fmt.Sprintf("下载任务以状态 %d 结束", status.Status)), task)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("等待下载任务超时或已取消: %w", ctx.Err())
		case <-poll_ticker.C:
		}
	}
}

func first_download_task_id(result *DownloadTaskCreateResult) int {
	if result == nil {
		return 0
	}
	if len(result.IDs) > 0 {
		return result.IDs[0]
	}
	raw_task, _ := json.Marshal(result.Task)
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
