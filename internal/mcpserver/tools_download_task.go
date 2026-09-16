package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

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

func download_source(job *ScraperJob, output scraper_output) map[string]any {
	return map[string]any{
		"job_id":   job.ID,
		"platform": output.Platform,
		"url":      output.URL,
	}
}

func (s *ToolSet) get_download_tasks(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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
	raw_response, err := s.api_client.download_tasks(ctx, query)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(raw_json_value(raw_response))
}

func (s *ToolSet) get_download_task_detail(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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
	raw_response, err := s.api_client.download_task_detail(ctx, arguments.ID)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(raw_json_value(raw_response))
}

func (s *ToolSet) delete_download_tasks(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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

func (s *ToolSet) create_download_task_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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

func (s *ToolSet) create_download_task(ctx context.Context, request DownloadTaskCreateRequest, fallback_message string) (*DownloadTaskCreateResult, error) {
	if s.download_task_creator != nil {
		return s.download_task_creator.CreateDownloadTask(ctx, request)
	}
	if s.api_client == nil {
		return nil, fmt.Errorf("下载任务创建服务未初始化")
	}
	create_response, err := s.api_client.create_download_task(ctx, request)
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

// tools_download_task declares the download-task tools whose handlers live in
// this file. The row order here is filesystem-local only; the published order
// is fixed by the concatenation in tool_declarations (registry.go).
var tools_download_task = []tool{
	{
		name:         "download_content",
		title:        `下载内容`,
		description:  `根据 fetch_content 返回的 job_id 创建并启动下载任务；也可直接传 url，此时会先解析链接。默认在任务启动后返回，设置 wait_for_completion 可等待文件下载完成。此工具会写入下载目录。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"anyOf":[{"required":["job_id"]},{"required":["fetch_id"]},{"required":["url"]}],"properties":{"download_dir":{"description":"下载目录；留空时使用应用配置。","type":"string"},"existing_action":{"default":"error","description":"遇到相同任务时的处理方式。overwrite 会覆盖已有任务及文件。","enum":["error","skip","overwrite","duplicate"],"type":"string"},"fetch_id":{"deprecated":true,"description":"job_id 的旧名称，仅用于兼容已有调用。","type":"string"},"filename":{"description":"可选的自定义文件名。","type":"string"},"force_refresh":{"default":false,"description":"直接传 url 时忽略抓取缓存。","type":"boolean"},"job_id":{"description":"fetch_content 返回的 job_id；优先使用它以复用解析结果。","type":"string"},"timeout_seconds":{"default":300,"description":"解析链接以及可选等待下载完成的最长秒数。","maximum":86400,"minimum":1,"type":"integer"},"url":{"description":"未提供 job_id 时，要解析并下载的平台内容链接。","format":"uri","type":"string"},"video_variant_key":{"description":"可选的视频规格 variant_key。","type":"string"},"video_variant_spec":{"description":"可选的视频规格名称；同时兼容适配器的 spec 配置。","type":"string"},"wait_for_completion":{"default":false,"description":"是否等待实际下载结束后再返回。","type":"boolean"}},"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":true,"idempotentHint":false,"openWorldHint":true,"readOnlyHint":false}`),
		supports:     supports_download_content,
		handle:       (*ToolSet).download_content,
	},
	{
		name:         "create_download_task",
		title:        `创建下载任务`,
		description:  `直接创建并启动一个下载任务。platform 指定内容平台，content 是平台归一化后的内容对象，config 可携带 existing_action、video_variant_key 等下载配置。相比 download_content 或 download_wxchannels_video，此工具不做链接解析，只调用下载任务服务。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"auto_start":{"default":true,"description":"创建后是否立即启动，默认 true。","type":"boolean"},"build_from_fetch":{"default":false,"description":"是否按抓取结果构建下载任务。","type":"boolean"},"config":{"additionalProperties":true,"description":"下载配置，例如 existing_action、video_variant_key。","type":"object"},"content":{"description":"平台归一化后的内容对象，必须包含平台下载所需的字段。","type":"object"},"download_dir":{"description":"下载目录；留空时使用应用配置。","type":"string"},"filename":{"description":"可选的自定义文件名。","type":"string"},"parent_task_id":{"description":"可选的父任务 ID，用于建立任务关联。","minimum":1,"type":"integer"},"platform":{"description":"内容平台 ID，例如 wxchannels。","minLength":1,"type":"string"},"relation_type":{"description":"与父任务的关系类型。","type":"string"},"resource_indexes":{"description":"要下载的资源下标列表。","items":{"minimum":0,"type":"integer"},"type":"array","uniqueItems":true}},"required":["platform","content"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":true,"idempotentHint":false,"openWorldHint":true,"readOnlyHint":false}`),
		supports:     supports_task_create,
		handle:       (*ToolSet).create_download_task_tool,
	},
	{
		name:         "get_download_tasks",
		title:        `获取下载任务`,
		description:  `分页查询下载任务及状态统计，可按任务状态、父任务或根任务筛选。状态值：0 等待、1 准备、2 下载中、3 暂停、4 合并、5 完成、6 失败、7 取消。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"content_id":{"description":"只返回关联该内容 ID 的下载任务。","type":"string"},"page":{"default":1,"description":"页码，从 1 开始。","maximum":1000000,"minimum":1,"type":"integer"},"page_size":{"default":20,"description":"每页任务数。","maximum":100,"minimum":1,"type":"integer"},"parent_task_id":{"description":"只返回该父任务的直接子任务。","minimum":1,"type":"integer"},"root_task_id":{"description":"只返回属于该根任务的任务。","minimum":1,"type":"integer"},"statuses":{"description":"可选的任务状态列表。","items":{"maximum":7,"minimum":0,"type":"integer"},"type":"array","uniqueItems":true}},"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":true}`),
		supports:     supports_data,
		handle:       (*ToolSet).get_download_tasks,
	},
	{
		name:         "get_download_task_detail",
		title:        `获取下载任务详情`,
		description:  `按下载任务 ID 获取任务、文件、进度、关联内容和账号详情。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"id":{"description":"下载任务 ID。","minimum":1,"type":"integer"}},"required":["id"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":true}`),
		supports:     supports_data,
		handle:       (*ToolSet).get_download_task_detail,
	},
	{
		name:         "delete_download_tasks",
		title:        `删除下载任务`,
		description:  `用户明确确认后，停止并软删除指定下载任务。delete_files 默认为 false，仅删除任务记录；设为 true 时同时安全删除关联的最终文件、临时文件和直播录制目录。每个任务独立返回删除结果。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"delete_files":{"default":false,"description":"是否同时删除任务关联的本地文件。","type":"boolean"},"task_ids":{"description":"要删除的下载任务 ID。","items":{"description":"下载任务 ID。","minimum":1,"type":"integer"},"minItems":1,"type":"array","uniqueItems":true}},"required":["task_ids"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":true,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":false}`),
		supports:     supports_task_delete,
		handle:       (*ToolSet).delete_download_tasks,
	},
}
