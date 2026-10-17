package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ScraperJobBackend provides process-local scraper job access. Implementations
// may call the application service directly without an HTTP API listener.
type ScraperJobBackend interface {
	CreateScraperJob(ctx context.Context, raw_url string, force_refresh bool) (*ScraperJob, error)
	GetScraperJob(ctx context.Context, job_id string) (*ScraperJob, error)
	InterruptScraperJob(job_id string)
}

type fetch_content_arguments struct {
	URL            string `json:"url"`
	ForceRefresh   bool   `json:"force_refresh"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type create_scraper_job_arguments struct {
	URL          string `json:"url"`
	ForceRefresh bool   `json:"force_refresh"`
}

type get_scraper_job_arguments struct {
	ID string `json:"id"`
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

func (s *ToolSet) create_scraper_job_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments create_scraper_job_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	arguments.URL = strings.TrimSpace(arguments.URL)
	if err := validate_source_url(arguments.URL); err != nil {
		return nil, err
	}
	job, err := s.create_scraper_job(ctx, arguments.URL, arguments.ForceRefresh)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(job)
}

func (s *ToolSet) get_scraper_job_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments get_scraper_job_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	arguments.ID = strings.TrimSpace(arguments.ID)
	if arguments.ID == "" {
		return nil, fmt.Errorf("id 不能为空")
	}
	job, err := s.get_scraper_job(ctx, arguments.ID)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(job)
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

// tools_scraper declares the tools whose handlers live in this file. The row
// order here is filesystem-local only; the published order is fixed
// by the concatenation in tool_declarations (registry.go).
var tools_scraper = []tool{
	{
		name:         "get_platform_status",
		title:        `获取平台状态`,
		description:  `获取下载器当前支持的抓取平台及各平台可用状态。解析或下载链接前可先调用此工具。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":true}`),
		supports:     supports_api_client,
		handle:       without_arguments((*ToolSet).get_platform_status),
	},
	{
		name:         "fetch_content",
		title:        `获取链接内容`,
		description:  `解析受支持的平台链接并返回规范化内容、账号、内容详情、缓存条目和下载预览。微信视频号结果会额外提供 download_resources，其中包含可交给 aria2 等下载器的 download_url 和可选 decode_key。返回的 job_id 可传给 download_content，避免再次解析链接。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"force_refresh":{"default":false,"description":"忽略抓取缓存并重新获取。","type":"boolean"},"timeout_seconds":{"default":300,"description":"等待链接解析完成的最长秒数。","maximum":3600,"minimum":1,"type":"integer"},"url":{"description":"要解析的平台内容链接。","format":"uri","type":"string"}},"required":["url"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":true,"readOnlyHint":true}`),
		supports:     supports_scraper,
		handle:       (*ToolSet).fetch_content,
	},
	{
		name:         "create_scraper_job",
		title:        `创建页面抓取任务`,
		description:  `创建异步页面抓取任务并立即返回任务快照。使用 get_scraper_job 查询状态和结果；需要直接等待结果时可使用 fetch_content。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"force_refresh":{"default":false,"description":"忽略抓取缓存并重新获取。","type":"boolean"},"url":{"description":"要抓取的平台或普通页面链接。","format":"uri","type":"string"}},"required":["url"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":false,"openWorldHint":true,"readOnlyHint":true}`),
		supports:     supports_scraper,
		handle:       (*ToolSet).create_scraper_job_tool,
	},
	{
		name:         "get_scraper_job",
		title:        `获取页面抓取任务`,
		description:  `按任务 ID 查询抓取状态、进度和完成后的输出。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"id":{"description":"create_scraper_job 返回的任务 ID。","type":"string"}},"required":["id"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":true}`),
		supports:     supports_scraper,
		handle:       (*ToolSet).get_scraper_job_tool,
	},
}
