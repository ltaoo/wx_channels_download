package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/adapter"
	result "wx_channel/internal/apiresult"
	"wx_channel/internal/config"
	"wx_channel/internal/database"
	"wx_channel/internal/database/model"
	"wx_channel/internal/services"
)

// CreateDownloadTaskRequest is the request body for creating download tasks.
type CreateDownloadTaskRequest struct {
	Objects []services.CreateDownloadTaskBody `json:"objects"`
}

// DownloadTaskItem is the compact task shape returned after creating a task.
type DownloadTaskItem struct {
	ID           int                    `json:"id"`
	ContentID    *string                `json:"content_id,omitempty"`
	ParentTaskID *int                   `json:"parent_task_id,omitempty"`
	RootTaskID   int                    `json:"root_task_id"`
	RelationType string                 `json:"relation_type,omitempty"`
	Name         string                 `json:"name"`
	UniqueID     string                 `json:"unique_id"`
	PlatformID   string                 `json:"platform_id"`
	Status       int                    `json:"status"`
	SourceURL    string                 `json:"source_url,omitempty"`
	CoverURL     string                 `json:"cover_url,omitempty"`
	CoverWidth   string                 `json:"cover_width,omitempty"`
	CoverHeight  string                 `json:"cover_height,omitempty"`
	ConfigJSON   string                 `json:"config_json,omitempty"`
	MetadataJSON string                 `json:"metadata_json,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Resources    []DownloadResourceItem `json:"resources"`
	CreatedAt    int64                  `json:"created_at"`
	UpdatedAt    int64                  `json:"updated_at"`
}

// DownloadResourceItem is the compact resource shape embedded in DownloadTaskItem.
type DownloadResourceItem struct {
	ID          int     `json:"id"`
	TaskID      *int    `json:"task_id,omitempty"`
	ContentID   *string `json:"content_id,omitempty"`
	DownloadDir string  `json:"download_dir"`
	Name        string  `json:"name"`
	Kind        string  `json:"kind"`
	Type        string  `json:"type"`
	UniqueID    string  `json:"unique_id"`
	Size        int64   `json:"size"`
	Downloaded  int64   `json:"downloaded"`
	Speed       int64   `json:"speed"`
	Status      int     `json:"status"`
	OutputPath  string  `json:"output_path,omitempty"`
	FilePath    string  `json:"file_path,omitempty"`
}

// DownloadTaskCreateItem is the per-object result shape in the batch create response.
type DownloadTaskCreateItem struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// taskV1IDsBody is a request body with task_ids for batch operations (start/pause/resume).
type taskV1IDsBody struct {
	TaskIDs []int `json:"task_ids"`
}

// deleteDownloadTasksBody includes task_ids and the caller's local-file cleanup intent.
type deleteDownloadTasksBody struct {
	TaskIDs     []int `json:"task_ids"`
	DeleteFiles bool  `json:"delete_files"`
}

type clear_download_tasks_body struct {
	TaskIDs     []int `json:"task_ids"`
	DeleteFiles bool  `json:"delete_files"`
}

type clear_all_download_tasks_body struct {
	DeleteFiles bool `json:"delete_files"`
}

// checkDownloadTaskFileItem identifies one file returned by the task list API.
// Name and OutputPath are accepted as part of that record, but the server uses
// the persisted task/resource relationship to resolve a safe local path.
type checkDownloadTaskFileItem struct {
	TaskID     int    `json:"task_id"`
	ID         int    `json:"id"`
	Name       string `json:"name"`
	OutputPath string `json:"output_path"`
}

type checkDownloadTaskFilesBody struct {
	Files []checkDownloadTaskFileItem `json:"files"`
}

type checkDownloadTaskFileResult struct {
	TaskID  int    `json:"task_id"`
	ID      int    `json:"id"`
	Checked bool   `json:"checked"`
	Exists  bool   `json:"exists"`
	Error   string `json:"error,omitempty"`
}

// CreateDownloadTaskByURLRequest is the request for creating download tasks by URL.
type CreateDownloadTaskByURLRequest struct {
	Objects []CreateDownloadTaskByURLBody `json:"objects"`
}

// CreateDownloadTaskByURLBody is a single item in the create-by-URL request.
type CreateDownloadTaskByURLBody struct {
	URL          string         `json:"url"`          // resource download URL, required
	DownloadDir  string         `json:"download_dir"` // download directory
	Filename     string         `json:"filename"`     // filename (optional, extracted from URL by default)
	Config       map[string]any `json:"config"`       // custom download config
	AutoStart    *bool          `json:"auto_start"`
	ParentTaskID *int           `json:"parent_task_id"`
	RelationType string         `json:"relation_type"`
}

const (
	download_existing_action_skip      = "skip"
	download_existing_action_duplicate = "duplicate"
	download_existing_action_overwrite = "overwrite"
)

// resolve_download_dir resolves the download directory for a task.
// Uses app config default when none is specified; relative paths are expanded
// relative to the working directory.
func (c *APIClient) resolve_download_dir(requested string) (string, error) {
	download_dir := strings.TrimSpace(requested)
	if download_dir == "" && c.cfg != nil {
		download_dir = strings.TrimSpace(c.cfg.DownloadDir)
	}
	if download_dir == "" {
		return "", fmt.Errorf("下载目录不能为空")
	}

	work_dir := ""
	if c.cfg != nil {
		work_dir = c.cfg.WorkDir
	}
	if work_dir == "" {
		var err error
		work_dir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("获取工作目录失败: %w", err)
		}
	}

	download_dir = config.ResolveWorkDirPath(download_dir, work_dir)

	if err := os.MkdirAll(download_dir, 0755); err != nil {
		return "", fmt.Errorf("创建下载目录 %q 失败: %w", download_dir, err)
	}

	return download_dir, nil
}

// download_task_download_dir returns the task download root directory.
func download_task_download_dir(download_dir string) string {
	return download_dir
}

// startCreatedDownloadTask hands the newly created task off to Hermes for scheduling.
// Hermes manages all internal state (connections, status changes, logging) via the
// Store interface and triggers broadcasts through the EventHandler callback.
func (c *APIClient) startCreatedDownloadTask(taskID int) error {
	if c.downloader == nil {
		c.logger.Error().Int("task_id", taskID).Msg("Hermes download engine not initialized, cannot start download task")
		return fmt.Errorf("Hermes 下载器未初始化")
	}
	c.logger.Info().Int("task_id", taskID).Msg("Submitting download task to Hermes scheduler")
	if err := c.downloader.StartCreatedTask(taskID); err != nil {
		c.logger.Error().Int("task_id", taskID).Err(err).Msg("Hermes scheduler failed to start download task")
		return err
	}
	c.logger.Info().Int("task_id", taskID).Msg("Download task submitted to Hermes scheduling queue")
	return nil
}

// prepareDownloadTaskSingle previews a single platform download task (no DB write, no download start).
func (c *APIClient) prepareDownloadTaskSingle(body services.CreateDownloadTaskBody) (*adapter.DownloadTaskResult, error) {
	if body.Platform == "" {
		return nil, fmt.Errorf("platform 不能为空")
	}

	h := adapter.Get(body.Platform)
	if h == nil {
		return nil, fmt.Errorf("不支持的平台: %s", body.Platform)
	}

	saveDir, err := c.resolve_download_dir(body.DownloadDir)
	if err != nil {
		return nil, fmt.Errorf("准备下载目录失败: %w", err)
	}

	config := make(map[string]any, len(body.Config)+2)
	for key, value := range body.Config {
		config[key] = value
	}
	config["download_dir"] = saveDir
	if body.Filename != "" {
		config["filename"] = body.Filename
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("构建下载配置失败: %w", err)
	}

	var info *adapter.DownloadTaskResult
	if body.BuildFromFetch {
		fetch_builder, ok := h.(adapter.FetchDownloadTaskBuilder)
		if !ok {
			return nil, fmt.Errorf("平台 %s 不支持从抓取结果构建下载任务", body.Platform)
		}
		info, err = fetch_builder.BuildDownloadTaskFromFetch(body.Content, json.RawMessage(configJSON))
	} else {
		info, err = h.BuildDownloadTask(body.Content, json.RawMessage(configJSON))
	}
	if err != nil {
		return nil, fmt.Errorf("构建下载任务失败: %w", err)
	}
	if info == nil {
		return nil, fmt.Errorf("构建下载任务失败: 平台未返回下载任务")
	}

	for _, ri := range info.Resources {
		if len(ri.Endpoints) == 0 {
			return nil, fmt.Errorf("资源 %s 没有下载端点", ri.Resource.Name)
		}
	}

	// Keep preview.data identical to scraper job's download_info payload.
	return info, nil
}

// handle_prepare_download_task batch-previews platform download tasks.
// POST /api/v1/download_task/prepare
func (c *APIClient) handle_prepare_download_task(ctx *gin.Context) {
	var req CreateDownloadTaskRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(req.Objects) == 0 {
		result.Err(ctx, 400, "请求体不能为空数组")
		return
	}

	previews := make([]gin.H, 0, len(req.Objects))
	for _, body := range req.Objects {
		data, err := c.prepareDownloadTaskSingle(body)
		if err != nil {
			previews = append(previews, gin.H{"success": false, "error": err.Error()})
		} else {
			previews = append(previews, gin.H{"success": true, "data": data})
		}
	}

	result.Ok(ctx, gin.H{"previews": previews})
}

// prepareDownloadTaskByURLSingle previews a download task created by resource URL (no DB write, no download start).
func (c *APIClient) prepareDownloadTaskByURLSingle(body CreateDownloadTaskByURLBody) (gin.H, error) {
	if body.URL == "" {
		return nil, fmt.Errorf("url 不能为空")
	}

	parsedURL, err := url.Parse(body.URL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("无效的下载地址")
	}

	protocol := strings.ToUpper(parsedURL.Scheme)

	requested_download_dir := body.DownloadDir
	if requested_download_dir == "" {
		requested_download_dir, _ = body.Config["download_dir"].(string)
	}
	saveDir, err := c.resolve_download_dir(requested_download_dir)
	if err != nil {
		return nil, fmt.Errorf("准备下载目录失败: %w", err)
	}
	filename := body.Filename
	if filename == "" {
		filename, _ = body.Config["filename"].(string)
	}
	if filename == "" {
		base := filepath.Base(parsedURL.Path)
		if base != "" && base != "." && base != "/" {
			if decoded, err := url.QueryUnescape(base); err == nil {
				filename = decoded
			} else {
				filename = base
			}
		}
	}
	if filename == "" {
		filename = body.URL
	}
	filename = filepath.Base(filename)
	if filename == "" || filename == "." || filename == ".." || filename == string(filepath.Separator) {
		return nil, fmt.Errorf("无法确定下载文件名")
	}

	download_dir := download_task_download_dir(saveDir)

	return gin.H{
		"url":          body.URL,
		"protocol":     protocol,
		"task_name":    filename,
		"download_dir": download_dir,
		"resources": []gin.H{{
			"index": 0,
			"name":  filename,
			"kind":  "file",
			"endpoints": []gin.H{{
				"protocol": protocol,
				"url":      body.URL,
				"priority": 0,
			}},
		}},
		"resource_count": 1,
		"endpoint_count": 1,
	}, nil
}

// handle_prepare_download_task_by_url batch-previews download tasks created by resource URL.
// POST /api/v1/download_task/prepare_by_url
func (c *APIClient) handle_prepare_download_task_by_url(ctx *gin.Context) {
	var req CreateDownloadTaskByURLRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(req.Objects) == 0 {
		result.Err(ctx, 400, "请求体不能为空数组")
		return
	}

	previews := make([]gin.H, 0, len(req.Objects))
	for _, body := range req.Objects {
		data, err := c.prepareDownloadTaskByURLSingle(body)
		if err != nil {
			previews = append(previews, gin.H{"success": false, "error": err.Error()})
		} else {
			previews = append(previews, gin.H{"success": true, "data": data})
		}
	}

	result.Ok(ctx, gin.H{"previews": previews})
}

// create_download_task_single creates a single platform download task and returns the compact task item or error.
func (c *APIClient) create_download_task_single(body services.CreateDownloadTaskBody) (DownloadTaskItem, error) {
	if c.download_task_service == nil {
		return DownloadTaskItem{}, fmt.Errorf("下载任务服务未初始化")
	}
	create_result, err := c.download_task_service.CreateTask(body)
	if err != nil {
		return DownloadTaskItem{}, err
	}
	return build_download_task_item(create_result), nil
}

func build_download_task_item(create_result *services.CreateTaskResult) DownloadTaskItem {
	if create_result == nil {
		return DownloadTaskItem{}
	}
	task := create_result.Task
	resources := make([]DownloadResourceItem, 0, len(create_result.Resources))
	for _, resource := range create_result.Resources {
		resources = append(resources, build_download_resource_item(resource))
	}
	return DownloadTaskItem{
		ID:           task.Id,
		ContentID:    task.ContentId,
		ParentTaskID: task.ParentTaskID,
		RootTaskID:   task.RootTaskID,
		RelationType: task.RelationType,
		Name:         task.Name,
		UniqueID:     task.UniqueID,
		PlatformID:   task.PlatformId,
		Status:       task.Status,
		SourceURL:    task.SourceURL,
		CoverURL:     task.CoverURL,
		CoverWidth:   task.CoverWidth,
		CoverHeight:  task.CoverHeight,
		ConfigJSON:   task.ConfigJSON,
		MetadataJSON: task.MetadataJSON,
		ErrorMessage: task.ErrorMessage,
		Resources:    resources,
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}
}

func build_download_resource_item(resource model.DownloadResource) DownloadResourceItem {
	file_path := ""
	if strings.TrimSpace(resource.DownloadDir) != "" {
		file_path = filepath.Join(resource.DownloadDir, resource.Name)
	}
	return DownloadResourceItem{
		ID:          resource.Id,
		TaskID:      resource.TaskId,
		ContentID:   resource.ContentId,
		DownloadDir: resource.DownloadDir,
		Name:        resource.Name,
		Kind:        resource.Kind,
		Type:        resource.Type,
		UniqueID:    resource.UniqueID,
		Size:        resource.Size,
		Downloaded:  resource.Downloaded,
		Speed:       resource.Speed,
		Status:      resource.Status,
		OutputPath:  resource.Name,
		FilePath:    file_path,
	}
}

func download_task_create_success_item(data DownloadTaskItem) DownloadTaskCreateItem {
	return DownloadTaskCreateItem{
		Code: api_code_success,
		Msg:  api_success_message,
		Data: data,
	}
}

func download_task_create_failed_item(code int, msg string, data any) DownloadTaskCreateItem {
	if data == nil {
		data = gin.H{}
	}
	return DownloadTaskCreateItem{
		Code: code,
		Msg:  msg,
		Data: data,
	}
}

func download_task_create_skipped_item(err *services.DuplicateTaskError) DownloadTaskCreateItem {
	data := duplicate_download_task_data(err)
	data["skipped"] = true
	data["action"] = download_existing_action_skip
	return DownloadTaskCreateItem{
		Code: api_code_success,
		Msg:  "Skipped existing download task",
		Data: data,
	}
}

func duplicate_download_task_data(err *services.DuplicateTaskError) gin.H {
	data := gin.H{}
	if err == nil {
		return data
	}
	if err.ExistingTaskID > 0 {
		data["id"] = err.ExistingTaskID
	}
	if err.ExistingTaskName != "" {
		data["name"] = err.ExistingTaskName
	}
	return data
}

func normalize_download_existing_action(action string) string {
	switch strings.TrimSpace(action) {
	case "skip":
		return download_existing_action_skip
	case "duplicate":
		return download_existing_action_duplicate
	case "overwrite":
		return download_existing_action_overwrite
	default:
		return ""
	}
}

func requested_download_existing_action(config map[string]any) (string, bool) {
	if config == nil {
		return "", false
	}
	raw_action, exists := config["existing_action"]
	if !exists {
		return "", false
	}
	action, ok := raw_action.(string)
	if !ok {
		return "", true
	}
	action = strings.TrimSpace(action)
	if action == "error" {
		return "", true
	}
	return normalize_download_existing_action(action), true
}

func (c *APIClient) default_action_when_existing() string {
	raw := ""
	if c != nil && c.cfg != nil {
		raw = strings.TrimSpace(c.cfg.DefaultActionWhenExisting)
		if c.cfg.Original != nil {
			raw = strings.TrimSpace(c.cfg.Original.GetString("download.defaultActionWhenExisting"))
		}
	}
	return normalize_download_existing_action(raw)
}

func clone_download_task_config(config map[string]any) map[string]any {
	cloned := make(map[string]any, len(config)+2)
	for key, value := range config {
		cloned[key] = value
	}
	return cloned
}

func (c *APIClient) handle_duplicate_download_task_action(body services.CreateDownloadTaskBody, duplicate_err *services.DuplicateTaskError, action string) (DownloadTaskCreateItem, int, bool) {
	switch action {
	case download_existing_action_skip:
		c.logger.Info().
			Str("api", "POST /api/v1/download_task/create").
			Int("existing_task_id", duplicate_err.ExistingTaskID).
			Str("incoming_task_unique_id", duplicate_err.IncomingUniqueID).
			Str("existing_action", action).
			Msg("Task conflict handled by existing-task action")
		return download_task_create_skipped_item(duplicate_err), 0, true
	case download_existing_action_duplicate, download_existing_action_overwrite:
		retry_body := body
		retry_body.Config = clone_download_task_config(body.Config)
		retry_body.Config["duplicate"] = action == download_existing_action_duplicate
		retry_body.Config["overwrite"] = action == download_existing_action_overwrite

		c.logger.Info().
			Str("api", "POST /api/v1/download_task/create").
			Int("existing_task_id", duplicate_err.ExistingTaskID).
			Str("incoming_task_unique_id", duplicate_err.IncomingUniqueID).
			Str("existing_action", action).
			Msg("Retrying conflicted task creation with existing-task action")

		data, err := c.create_download_task_single(retry_body)
		if err != nil {
			var retry_duplicate_err *services.DuplicateTaskError
			if errors.As(err, &retry_duplicate_err) {
				c.logger.Warn().
					Str("api", "POST /api/v1/download_task/create").
					Int("existing_task_id", retry_duplicate_err.ExistingTaskID).
					Str("incoming_task_unique_id", retry_duplicate_err.IncomingUniqueID).
					Str("existing_action", action).
					Err(err).
					Msg("Existing-task action still resulted in a conflict")
				return download_task_create_failed_item(api_code_invalid_params, err.Error(), duplicate_download_task_data(retry_duplicate_err)), 0, true
			}
			c.logger.Warn().
				Str("api", "POST /api/v1/download_task/create").
				Str("platform", body.Platform).
				Str("existing_action", action).
				Err(err).
				Msg("Existing-task action failed to create download task")
			return download_task_create_failed_item(api_code_invalid_params, err.Error(), gin.H{}), 0, true
		}
		return download_task_create_success_item(data), data.ID, true
	default:
		return DownloadTaskCreateItem{}, 0, false
	}
}

// handle_create_download_task batch-creates platform download tasks.
// POST /api/v1/download_task/create
func (c *APIClient) handle_create_download_task(ctx *gin.Context) {
	var req CreateDownloadTaskRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.logger.Warn().Str("api", "POST /api/v1/download_task/create").Err(err).Msg("Failed to parse request body")
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(req.Objects) == 0 {
		result.Err(ctx, 400, "没有传入要下载的内容")
		return
	}

	c.logger.Info().Str("api", "POST /api/v1/download_task/create").Int("object_count", len(req.Objects)).Msg("Received batch create download task request")

	default_existing_action := c.default_action_when_existing()
	var duplicate_err *services.DuplicateTaskError
	tasks := make([]DownloadTaskCreateItem, 0, len(req.Objects))
	ids := make([]int, 0, len(req.Objects))
	success_count := 0
	fail_count := 0
	skip_count := 0
	manual_start_count := 0
	for _, body := range req.Objects {
		data, err := c.create_download_task_single(body)
		if err != nil {
			if errors.As(err, &duplicate_err) {
				existing_action := default_existing_action
				if requested_action, specified := requested_download_existing_action(body.Config); specified {
					existing_action = requested_action
				}
				if item, id, handled := c.handle_duplicate_download_task_action(body, duplicate_err, existing_action); handled {
					tasks = append(tasks, item)
					if id > 0 {
						ids = append(ids, id)
						if body.AutoStart != nil && !*body.AutoStart {
							c.broadcast_download_task_create(id)
							manual_start_count++
						}
					}
					if item.Code == api_code_success {
						success_count++
						if existing_action == download_existing_action_skip {
							skip_count++
						}
					} else {
						fail_count++
					}
					continue
				}
				c.logger.Warn().
					Str("api", "POST /api/v1/download_task/create").
					Int("existing_task_id", duplicate_err.ExistingTaskID).
					Str("incoming_task_unique_id", duplicate_err.IncomingUniqueID).
					Msg("Task conflict in batch, skipping")
				tasks = append(tasks, download_task_create_failed_item(api_code_duplicate_download_task, err.Error(), duplicate_download_task_data(duplicate_err)))
				fail_count++
				continue
			}
			c.logger.Warn().Str("platform", body.Platform).Err(err).Msg("Failed to create download task")
			tasks = append(tasks, download_task_create_failed_item(api_code_invalid_params, err.Error(), gin.H{}))
			fail_count++
		} else {
			tasks = append(tasks, download_task_create_success_item(data))
			ids = append(ids, data.ID)
			success_count++
			if body.AutoStart != nil && !*body.AutoStart {
				c.broadcast_download_task_create(data.ID)
				manual_start_count++
			}
		}
	}
	if manual_start_count > 0 {
		c.broadcast_download_task_stats()
	}

	c.logger.Info().
		Str("api", "POST /api/v1/download_task/create").
		Int("total", len(tasks)).
		Int("success", success_count).
		Int("skipped", skip_count).
		Int("failed", fail_count).
		Msg("Batch create download tasks completed")

	result.Ok(ctx, gin.H{"tasks": tasks, "ids": ids})
}

// createDownloadTaskByURLSingle creates a single download task by resource URL.
func (c *APIClient) createDownloadTaskByURLSingle(body CreateDownloadTaskByURLBody) (gin.H, error) {
	if body.URL == "" {
		return nil, fmt.Errorf("url 不能为空")
	}

	parsedURL, err := url.Parse(body.URL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("无效的下载地址")
	}

	protocol := strings.ToUpper(parsedURL.Scheme)
	requested_download_dir := body.DownloadDir
	if strings.TrimSpace(requested_download_dir) == "" {
		requested_download_dir, _ = body.Config["download_dir"].(string)
	}
	save_dir, err := c.resolve_download_dir(requested_download_dir)
	if err != nil {
		return nil, fmt.Errorf("准备下载目录失败: %w", err)
	}

	filename := body.Filename
	if filename == "" {
		filename, _ = body.Config["filename"].(string)
	}
	if filename == "" {
		// Extract filename from URL path
		base := filepath.Base(parsedURL.Path)
		if base != "" && base != "." && base != "/" {
			if decoded, err := url.QueryUnescape(base); err == nil {
				filename = decoded
			} else {
				filename = base
			}
		}
	}
	// If filename still cannot be extracted, use the URL as the name
	if filename == "" {
		filename = body.URL
	}
	filename = filepath.Base(filename)
	if filename == "" || filename == "." || filename == ".." || filename == string(filepath.Separator) {
		return nil, fmt.Errorf("无法确定下载文件名")
	}

	taskName := filename

	// Store the task-specific output container together with the source URL.
	task_config := make(map[string]any, len(body.Config)+3)
	for key, value := range body.Config {
		task_config[key] = value
	}
	task_config["url"] = body.URL
	task_config["download_dir"] = save_dir
	if filename != "" {
		task_config["filename"] = filename
	}
	config_json, err := json.Marshal(task_config)
	if err != nil {
		return nil, fmt.Errorf("构建下载配置失败: %w", err)
	}

	// Database not initialized
	if c.db == nil {
		return nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	now := time.Now().UnixMilli()

	// Create task
	task := model.DownloadTask{
		Name:       taskName,
		Status:     model.TaskStatusWaiting,
		ConfigJSON: string(config_json),
	}
	task.CreatedAt = now
	task.UpdatedAt = now

	if err := database.ApplyTaskLineage(c.db, &task, body.ParentTaskID, body.RelationType); err != nil {
		return nil, err
	}
	if err := c.db.Create(&task).Error; err != nil {
		c.logger.Error().Str("url", body.URL).Err(err).Msg("URL download task failed to write to database")
		return nil, fmt.Errorf("创建下载任务失败: %w", err)
	}
	if err := database.FinalizeTaskRoot(c.db, &task); err != nil {
		return nil, err
	}

	c.logger.Info().Int("task_id", task.Id).Str("url", body.URL).Str("download_dir", save_dir).Msg("URL download task written to database")
	// Create resource
	task_id := task.Id
	resource := model.DownloadResource{
		TaskId:      &task_id,
		DownloadDir: save_dir,
		Name:        filename,
		Kind:        "file",
		Status:      0,
		MergeOrder:  0,
	}
	resource.CreatedAt = now
	resource.UpdatedAt = now

	if err := c.db.Create(&resource).Error; err != nil {
		return nil, fmt.Errorf("创建资源失败: %w", err)
	}

	// Create endpoint
	endpoint := model.DownloadEndpoint{
		ResourceId: resource.Id,
		Protocol:   protocol,
		URL:        body.URL,
		Priority:   0,
		Enabled:    1,
		Status:     0,
	}
	endpoint.CreatedAt = now
	endpoint.UpdatedAt = now

	if err := c.db.Create(&endpoint).Error; err != nil {
		return nil, fmt.Errorf("创建端点失败: %w", err)
	}

	// Hand off to scheduler when requested. Otherwise the persisted task remains waiting.
	if body.AutoStart == nil || *body.AutoStart {
		if err := c.startCreatedDownloadTask(task.Id); err != nil {
			return nil, fmt.Errorf("启动下载任务失败: %w", err)
		}
		task.Status = model.TaskStatusPreparing // Hermes has written to DB; here we only update the in-memory variable for the response
	}

	return gin.H{
		"task":     task,
		"resource": resource,
		"endpoint": endpoint,
	}, nil
}

// handle_create_download_task_by_url batch-creates download tasks by resource URL.
// POST /api/v1/download_task/create_by_url
func (c *APIClient) handle_create_download_task_by_url(ctx *gin.Context) {
	var req CreateDownloadTaskByURLRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.logger.Warn().Str("api", "POST /api/v1/download_task/create_by_url").Err(err).Msg("Failed to parse request body")
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(req.Objects) == 0 {
		result.Err(ctx, 400, "请求体不能为空数组")
		return
	}

	c.logger.Info().Str("api", "POST /api/v1/download_task/create_by_url").Int("object_count", len(req.Objects)).Msg("Received batch create URL download task request")

	tasks := make([]gin.H, 0, len(req.Objects))
	success_count := 0
	fail_count := 0
	manual_start_count := 0
	for _, body := range req.Objects {
		data, err := c.createDownloadTaskByURLSingle(body)
		if err != nil {
			c.logger.Warn().Str("url", body.URL).Err(err).Msg("Failed to create URL download task")
			tasks = append(tasks, gin.H{"success": false, "error": err.Error()})
			fail_count++
		} else {
			tasks = append(tasks, gin.H{"success": true, "data": data})
			success_count++
			if body.AutoStart != nil && !*body.AutoStart {
				if task, ok := data["task"].(model.DownloadTask); ok {
					c.broadcast_download_task_create(task.Id)
					manual_start_count++
				}
			}
		}
	}
	if manual_start_count > 0 {
		c.broadcast_download_task_stats()
	}

	c.logger.Info().
		Str("api", "POST /api/v1/download_task/create_by_url").
		Int("total", len(tasks)).
		Int("success", success_count).
		Int("failed", fail_count).
		Msg("Batch create URL download tasks completed")

	result.Ok(ctx, gin.H{"tasks": tasks})
}

// handle_start_download_task batch-starts download tasks.
// POST /api/v1/download_task/start
func (c *APIClient) handle_start_download_task(ctx *gin.Context) {
	var body taskV1IDsBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.logger.Warn().Str("api", "POST /api/v1/download_task/start").Err(err).Msg("Failed to parse request body")
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(body.TaskIDs) == 0 {
		result.Err(ctx, 400, "task_ids 不能为空")
		return
	}
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	c.logger.Info().Str("api", "POST /api/v1/download_task/start").Int("task_count", len(body.TaskIDs)).Msg("Received batch start download task request")

	results := make([]gin.H, 0, len(body.TaskIDs))
	for _, taskID := range body.TaskIDs {
		if taskID <= 0 {
			results = append(results, gin.H{"task_id": taskID, "success": false, "error": "task_id 无效"})
			continue
		}

		var task model.DownloadTask
		if err := c.db.Where("id = ?", taskID).First(&task).Error; err != nil {
			c.logger.Warn().Str("api", "POST /api/v1/download_task/start").Int("task_id", taskID).Msg("Task not found")
			results = append(results, gin.H{"task_id": taskID, "success": false, "error": "下载任务不存在"})
			continue
		}

		if task.Status != model.TaskStatusWaiting &&
			task.Status != model.TaskStatusPaused &&
			task.Status != model.TaskStatusFailed {
			c.logger.Warn().Str("api", "POST /api/v1/download_task/start").Int("task_id", taskID).Int("current_status", task.Status).Msg("Current status does not allow start")
			results = append(results, gin.H{"task_id": taskID, "success": false, "error": "当前状态不允许启动"})
			continue
		}

		c.logger.Info().Str("api", "POST /api/v1/download_task/start").Int("task_id", taskID).Str("task_name", task.Name).Int("previous_status", task.Status).Msg("Starting download task")

		if err := c.downloader.StartTask(task.Id); err != nil {
			c.logger.Error().Int("task_id", taskID).Err(err).Msg("Failed to start download task")
			results = append(results, gin.H{"task_id": taskID, "success": false, "error": "启动下载任务失败: " + err.Error()})
			continue
		}
		c.logger.Info().Int("task_id", taskID).Str("status", "preparing").Msg("Download task started")

		task.Status = model.TaskStatusPreparing
		results = append(results, gin.H{"task_id": taskID, "success": true, "task": task, "status_text": "preparing"})
	}

	result.Ok(ctx, gin.H{"results": results})
}

// handle_pause_download_task batch-pauses download tasks.
// POST /api/v1/download_task/pause
func (c *APIClient) handle_pause_download_task(ctx *gin.Context) {
	var body taskV1IDsBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(body.TaskIDs) == 0 {
		result.Err(ctx, 400, "task_ids 不能为空")
		return
	}
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	results := make([]gin.H, 0, len(body.TaskIDs))
	for _, taskID := range body.TaskIDs {
		if taskID <= 0 {
			results = append(results, gin.H{"task_id": taskID, "success": false, "error": "task_id 无效"})
			continue
		}

		var task model.DownloadTask
		if err := c.db.Where("id = ?", taskID).First(&task).Error; err != nil {
			results = append(results, gin.H{"task_id": taskID, "success": false, "error": "下载任务不存在"})
			continue
		}

		if task.Status != model.TaskStatusPreparing && task.Status != model.TaskStatusDownloading {
			results = append(results, gin.H{"task_id": taskID, "success": false, "error": "当前状态不允许暂停"})
			continue
		}

		if c.hasStreamResources(task.Id) {
			if err := c.downloader.StopTask(task.Id); err != nil {
				results = append(results, gin.H{"task_id": taskID, "success": false, "error": "停止直播录制失败: " + err.Error()})
				continue
			}
			if err := c.db.Where("id = ?", taskID).First(&task).Error; err != nil {
				results = append(results, gin.H{"task_id": taskID, "success": false, "error": "读取直播录制最终状态失败: " + err.Error()})
				continue
			}
			if task.Status != model.TaskStatusFinished {
				message := strings.TrimSpace(task.ErrorMessage)
				if message == "" {
					message = fmt.Sprintf("收尾后的任务状态异常: %d", task.Status)
				}
				results = append(results, gin.H{"task_id": taskID, "success": false, "task": task, "error": "直播录制收尾失败: " + message})
				continue
			}
			results = append(results, gin.H{"task_id": taskID, "success": true, "task": task, "status_text": "finished"})
			continue
		}

		c.downloader.PauseTask(task.Id)

		task.Status = model.TaskStatusPaused
		results = append(results, gin.H{"task_id": taskID, "success": true, "task": task, "status_text": "paused"})
	}

	result.Ok(ctx, gin.H{"results": results})
}

// handle_resume_download_task batch-resumes download tasks.
// POST /api/v1/download_task/resume
func (c *APIClient) handle_resume_download_task(ctx *gin.Context) {
	var body taskV1IDsBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(body.TaskIDs) == 0 {
		result.Err(ctx, 400, "task_ids 不能为空")
		return
	}
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	results := make([]gin.H, 0, len(body.TaskIDs))
	for _, taskID := range body.TaskIDs {
		if taskID <= 0 {
			results = append(results, gin.H{"task_id": taskID, "success": false, "error": "task_id 无效"})
			continue
		}

		var task model.DownloadTask
		if err := c.db.Where("id = ?", taskID).First(&task).Error; err != nil {
			results = append(results, gin.H{"task_id": taskID, "success": false, "error": "下载任务不存在"})
			continue
		}

		if task.Status != model.TaskStatusPaused {
			results = append(results, gin.H{"task_id": taskID, "success": false, "error": "当前状态不允许恢复"})
			continue
		}

		if !c.downloader.HasAvailableSlot() {
			results = append(results, gin.H{"task_id": taskID, "success": false, "error": fmt.Sprintf("exceeds maximum concurrent download tasks (%d)", c.downloader.MaxConcurrent())})
			continue
		}

		if err := c.downloader.StartTask(task.Id); err != nil {
			results = append(results, gin.H{"task_id": taskID, "success": false, "error": "恢复下载任务失败: " + err.Error()})
			continue
		}
		task.Status = model.TaskStatusPreparing
		results = append(results, gin.H{"task_id": taskID, "success": true, "task": task, "status_text": "preparing"})
	}

	result.Ok(ctx, gin.H{"results": results})
}

// handle_retry_download_task batch-retries failed/cancelled download tasks.
// POST /api/v1/download_task/retry
func (c *APIClient) handle_retry_download_task(ctx *gin.Context) {
	var body taskV1IDsBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.logger.Warn().Str("api", "POST /api/v1/download_task/retry").Err(err).Msg("Failed to parse request body")
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(body.TaskIDs) == 0 {
		result.Err(ctx, 400, "task_ids 不能为空")
		return
	}
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	c.logger.Info().Str("api", "POST /api/v1/download_task/retry").Int("task_count", len(body.TaskIDs)).Msg("Received batch retry download task request")

	results := make([]gin.H, 0, len(body.TaskIDs))
	for _, taskID := range body.TaskIDs {
		if taskID <= 0 {
			results = append(results, gin.H{"task_id": taskID, "success": false, "error": "task_id 无效"})
			continue
		}

		var task model.DownloadTask
		if err := c.db.Where("id = ?", taskID).First(&task).Error; err != nil {
			c.logger.Warn().Str("api", "POST /api/v1/download_task/retry").Int("task_id", taskID).Msg("Task not found")
			results = append(results, gin.H{"task_id": taskID, "success": false, "error": "下载任务不存在"})
			continue
		}

		if task.Status != model.TaskStatusFailed && task.Status != model.TaskStatusCancelled {
			c.logger.Warn().Str("api", "POST /api/v1/download_task/retry").Int("task_id", taskID).Int("current_status", task.Status).Msg("Current status does not allow retry")
			results = append(results, gin.H{"task_id": taskID, "success": false, "error": "当前状态不允许重试"})
			continue
		}

		c.logger.Info().Str("api", "POST /api/v1/download_task/retry").Int("task_id", taskID).Str("task_name", task.Name).Int("previous_status", task.Status).Msg("Retrying download task")

		// Clear error state before retrying
		now := time.Now().UnixMilli()
		c.db.Model(&task).Updates(map[string]any{"error_message": "", "status": model.TaskStatusWaiting, "updated_at": now})

		if err := c.downloader.StartTask(task.Id); err != nil {
			c.logger.Error().Int("task_id", taskID).Err(err).Msg("Failed to retry download task")
			results = append(results, gin.H{"task_id": taskID, "success": false, "error": "重试下载任务失败: " + err.Error()})
			continue
		}
		c.logger.Info().Int("task_id", taskID).Str("status", "preparing").Msg("Download task retried")

		task.Status = model.TaskStatusPreparing
		task.ErrorMessage = ""
		results = append(results, gin.H{"task_id": taskID, "success": true, "task": task, "status_text": "preparing"})
	}

	result.Ok(ctx, gin.H{"results": results})
}

// handle_delete_download_task batch-deletes download tasks.
// POST /api/v1/download_task/delete
func (c *APIClient) handle_delete_download_task(ctx *gin.Context) {
	var body deleteDownloadTasksBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.logger.Warn().Str("api", "POST /api/v1/download_task/delete").Err(err).Msg("Failed to parse request body")
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(body.TaskIDs) == 0 {
		result.Err(ctx, 400, "task_ids 不能为空")
		return
	}
	if c.db == nil {
		c.logger.Error().Str("api", "POST /api/v1/download_task/delete").Bool("delete_files", body.DeleteFiles).Msg("Failed because database is unavailable")
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	c.logger.Info().Str("api", "POST /api/v1/download_task/delete").Int("task_count", len(body.TaskIDs)).Bool("delete_files", body.DeleteFiles).Msg("Received batch delete download task request")

	results := make([]gin.H, 0, len(body.TaskIDs))
	successful_deletions := 0
	for _, taskID := range body.TaskIDs {
		r := c.deleteSingleDownloadTask(taskID, body.DeleteFiles)
		results = append(results, r)
		if success, _ := r["success"].(bool); success {
			successful_deletions++
		}
	}
	if successful_deletions > 0 {
		c.broadcast_download_task_stats()
	}

	result.Ok(ctx, gin.H{"results": results})
}

// deleteSingleDownloadTask performs full deletion for a single download task
// and returns a result entry for the batch response.
func (c *APIClient) deleteSingleDownloadTask(taskID int, deleteFiles bool) gin.H {
	startedAt := time.Now()

	if taskID <= 0 {
		c.logger.Warn().Str("api", "POST /api/v1/download_task/delete").Int("task_id", taskID).Bool("delete_files", deleteFiles).Msg("Rejected invalid task ID")
		return gin.H{"task_id": taskID, "success": false, "error": "task_id 无效"}
	}

	requestLog := c.logger.Info().
		Str("api", "POST /api/v1/download_task/delete").
		Int("task_id", taskID).
		Bool("delete_files", deleteFiles)
	if c.cfg != nil {
		requestLog.Str("download_root", c.cfg.DownloadDir)
	}
	requestLog.Msg("Processing delete download task")

	var task model.DownloadTask
	taskQuery := c.db
	if deleteFiles {
		taskQuery = taskQuery.Unscoped()
	}
	if err := taskQuery.Where("id = ?", taskID).First(&task).Error; err != nil {
		c.logger.Warn().Int("task_id", taskID).Bool("delete_files", deleteFiles).Err(err).Msg("Download task deletion failed to load task")
		return gin.H{"task_id": taskID, "success": false, "error": "下载任务不存在"}
	}
	alreadySoftDeleted := task.DeletedAt != nil

	var resources []model.DownloadResource
	resourceQuery := c.db
	if alreadySoftDeleted {
		resourceQuery = resourceQuery.Unscoped()
	}
	if err := resourceQuery.Where("task_id = ?", task.Id).Order("merge_order ASC, id ASC").Find(&resources).Error; err != nil {
		c.logger.Error().Int("task_id", task.Id).Err(err).Msg("Download task deletion failed to load associated resources")
		return gin.H{"task_id": taskID, "success": false, "error": "查询下载任务资源失败: " + err.Error()}
	}
	resourceIDs := make([]int, 0, len(resources))
	for _, resource := range resources {
		resourceIDs = append(resourceIDs, resource.Id)
	}

	c.logger.Info().
		Int("task_id", task.Id).
		Str("task_name", task.Name).
		Int("task_status", task.Status).
		Int("resource_count", len(resources)).
		Ints("resource_ids", resourceIDs).
		Bool("delete_files", deleteFiles).
		Bool("already_soft_deleted", alreadySoftDeleted).
		Msg("Download task deletion loaded task and resource snapshot")
	c.logDownloadTaskLocalFiles(task, resources, "before_delete")

	now := time.Now().UnixMilli()

	if c.downloader == nil {
		c.logger.Error().Int("task_id", task.Id).Msg("Download task deletion failed because Hermes engine is unavailable")
		return gin.H{"task_id": taskID, "success": false, "error": "下载器未初始化"}
	}
	c.logger.Info().Int("task_id", task.Id).Msg("Stopping Hermes download job before soft deletion")
	c.downloader.DeleteTask(task.Id)
	c.logger.Info().Int("task_id", task.Id).Msg("Hermes delete call completed")
	if deleteFiles {
		c.logger.Info().Int("task_id", task.Id).Bool("local_file_cleanup_attempted", true).Msg("Starting associated local file cleanup")
		if err := c.deleteDownloadTaskLocalFiles(task, resources); err != nil {
			c.logger.Error().Int("task_id", task.Id).Bool("local_file_cleanup_attempted", true).Err(err).Msg("Associated local file cleanup failed; database soft deletion was skipped")
			return gin.H{"task_id": taskID, "success": false, "error": "删除任务关联的本地文件失败: " + err.Error()}
		}
		c.logger.Info().Int("task_id", task.Id).Bool("local_file_cleanup_attempted", true).Msg("Associated local file cleanup completed")
	} else {
		c.logger.Info().Int("task_id", task.Id).Bool("local_file_cleanup_attempted", false).Msg("Local file deletion was not requested; associated files will be left on disk")
	}
	if alreadySoftDeleted {
		c.logDownloadTaskLocalFiles(task, resources, "after_delete")
		c.logger.Info().
			Int("task_id", task.Id).
			Bool("delete_files", deleteFiles).
			Bool("local_file_cleanup_attempted", deleteFiles).
			Bool("already_soft_deleted", true).
			Dur("elapsed", time.Since(startedAt)).
			Msg("Recovered local file cleanup for previously soft-deleted download task")
		return gin.H{"task_id": taskID, "success": true, "status_text": "cancelled"}
	}
	c.logger.Info().Int("task_id", task.Id).Msg("Starting database soft deletion")
	taskDelete := c.db.Model(&task).Update("deleted_at", now)
	c.logDownloadTaskSoftDeleteResult(task.Id, "task", taskDelete.Error, taskDelete.RowsAffected)

	resourceDelete := c.db.Model(&model.DownloadResource{}).Where("task_id = ?", task.Id).Update("deleted_at", now)
	c.logDownloadTaskSoftDeleteResult(task.Id, "resources", resourceDelete.Error, resourceDelete.RowsAffected)

	if len(resourceIDs) > 0 {
		var endpointIDs []int
		endpointIDQuery := c.db.Model(&model.DownloadEndpoint{}).Where("resource_id IN ?", resourceIDs).Pluck("id", &endpointIDs)
		if endpointIDQuery.Error != nil {
			c.logger.Error().Int("task_id", task.Id).Ints("resource_ids", resourceIDs).Err(endpointIDQuery.Error).Msg("Download task deletion failed to query associated endpoint IDs")
		} else {
			c.logger.Info().Int("task_id", task.Id).Ints("endpoint_ids", endpointIDs).Msg("Download task deletion loaded associated endpoint IDs")
		}

		endpointDelete := c.db.Model(&model.DownloadEndpoint{}).Where("resource_id IN ?", resourceIDs).Update("deleted_at", now)
		c.logDownloadTaskSoftDeleteResult(task.Id, "endpoints", endpointDelete.Error, endpointDelete.RowsAffected)
		segmentDelete := c.db.Model(&model.DownloadSegment{}).Where("resource_id IN ?", resourceIDs).Update("deleted_at", now)
		c.logDownloadTaskSoftDeleteResult(task.Id, "segments", segmentDelete.Error, segmentDelete.RowsAffected)

		if len(endpointIDs) > 0 {
			connectionDelete := c.db.Model(&model.DownloadConnection{}).Where("endpoint_id IN ?", endpointIDs).Update("deleted_at", now)
			c.logDownloadTaskSoftDeleteResult(task.Id, "connections", connectionDelete.Error, connectionDelete.RowsAffected)
		}
	} else {
		c.logger.Warn().Int("task_id", task.Id).Msg("Download task deletion found no associated resources to cascade")
	}

	c.broadcast_download_task_delete([]int{task.Id})
	c.logger.Info().Int("task_id", task.Id).Msg("Download task deletion broadcast emitted")
	c.logDownloadTaskLocalFiles(task, resources, "after_delete")
	c.logger.Info().
		Int("task_id", task.Id).
		Bool("delete_files", deleteFiles).
		Bool("local_file_cleanup_attempted", deleteFiles).
		Dur("elapsed", time.Since(startedAt)).
		Msg("Download task deletion request completed")

	return gin.H{"task_id": taskID, "success": true, "status_text": "cancelled"}
}

func (c *APIClient) logDownloadTaskSoftDeleteResult(taskID int, entity string, err error, rowsAffected int64) {
	if err != nil {
		c.logger.Error().Int("task_id", taskID).Str("entity", entity).Int64("rows_affected", rowsAffected).Err(err).Msg("Download task cascade soft-delete failed")
		return
	}
	c.logger.Info().Int("task_id", taskID).Str("entity", entity).Int64("rows_affected", rowsAffected).Msg("Download task cascade soft-delete completed")
}

type downloadTaskLocalFileCandidate struct {
	Path          string
	PathSource    string
	CandidateType string
}

func (c *APIClient) download_task_local_file_roots(task model.DownloadTask, resource *model.DownloadResource) map[string]string {
	roots := make(map[string]string)
	add_root := func(root, source string) {
		root = strings.TrimSpace(root)
		if root == "" {
			return
		}
		if !filepath.IsAbs(root) && c.cfg != nil && strings.TrimSpace(c.cfg.WorkDir) != "" {
			root = filepath.Join(c.cfg.WorkDir, root)
		}
		absolute_root, err := filepath.Abs(root)
		if err != nil {
			c.logger.Warn().Int("task_id", task.Id).Str("path_source", source).Str("root", root).Err(err).Msg("Unable to resolve local file candidate root")
			return
		}
		roots[filepath.Clean(absolute_root)] = source
	}
	if c.cfg != nil {
		add_root(c.cfg.DownloadDir, "download_root")
	}
	var task_config map[string]any
	if strings.TrimSpace(task.ConfigJSON) != "" {
		if err := json.Unmarshal([]byte(task.ConfigJSON), &task_config); err != nil {
			c.logger.Warn().Int("task_id", task.Id).Err(err).Msg("Unable to parse task config while resolving local file roots")
		} else {
			download_dir, _ := task_config["download_dir"].(string)
			add_root(download_dir, "task_config_download_dir")
		}
	}
	if resource != nil {
		resource_download_dir := strings.TrimSpace(resource.DownloadDir)
		if resource_download_dir != "" && !filepath.IsAbs(resource_download_dir) && c.cfg != nil {
			resource_download_dir = filepath.Join(c.cfg.DownloadDir, resource_download_dir)
		}
		add_root(resource_download_dir, "resource_download_dir")
	}
	return roots
}

func pathWithinDownloadRoot(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == "." || relative == ".." {
		return false
	}
	return !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func (c *APIClient) download_task_local_file_candidates(task model.DownloadTask, resource model.DownloadResource) []downloadTaskLocalFileCandidate {
	names := []struct {
		value  string
		source string
	}{
		{value: strings.TrimSpace(resource.Name), source: "resource_name"},
		{value: strings.TrimSpace(resource.UniqueID), source: "resource_unique_id"},
	}
	if names[0].value == "" && names[1].value == "" {
		return nil
	}
	roots := c.download_task_local_file_roots(task, &resource)
	seen := make(map[string]struct{})
	candidates := make([]downloadTaskLocalFileCandidate, 0, len(roots)*6)
	for root, source := range roots {
		for _, name := range names {
			if name.value == "" {
				continue
			}
			path := name.value
			if !filepath.IsAbs(path) {
				path = filepath.Join(root, path)
			}
			path = filepath.Clean(path)
			if !pathWithinDownloadRoot(root, path) {
				c.logger.Error().Int("task_id", task.Id).Int("resource_id", resource.Id).Str("resource_name", resource.Name).Str("download_root", root).Str("path", path).Msg("Rejected local file candidate outside download root")
				continue
			}
			pathSource := source + ":" + name.source
			resourceCandidates := []downloadTaskLocalFileCandidate{
				{Path: path, PathSource: pathSource, CandidateType: "final"},
				{Path: path + ".part", PathSource: pathSource, CandidateType: "partial"},
			}
			if strings.EqualFold(resource.Type, model.ResourceTypeStream) {
				resourceCandidates = append(resourceCandidates, downloadTaskLocalFileCandidate{
					Path: path + ".recording", PathSource: pathSource, CandidateType: "recording",
				})
			}
			for _, candidate := range resourceCandidates {
				if _, exists := seen[candidate.Path]; exists {
					continue
				}
				seen[candidate.Path] = struct{}{}
				candidates = append(candidates, candidate)
			}
		}
	}
	return candidates
}

func (c *APIClient) deleteDownloadTaskLocalFiles(task model.DownloadTask, resources []model.DownloadResource) error {
	var deletionErrors []string
	for _, resource := range resources {
		candidates := c.download_task_local_file_candidates(task, resource)
		if len(candidates) == 0 {
			deletionErrors = append(deletionErrors, fmt.Sprintf("资源 %d (%q) 没有可安全删除的本地文件路径", resource.Id, resource.Name))
			c.logger.Warn().Int("task_id", task.Id).Int("resource_id", resource.Id).Str("resource_name", resource.Name).Msg("No safe local file candidates were resolved for resource")
			continue
		}
		for _, candidate := range candidates {
			info, err := os.Lstat(candidate.Path)
			if os.IsNotExist(err) {
				c.logger.Info().Int("task_id", task.Id).Int("resource_id", resource.Id).Str("path_source", candidate.PathSource).Str("candidate_type", candidate.CandidateType).Str("path", candidate.Path).Bool("exists", false).Msg("Associated local file did not exist; cleanup skipped")
				continue
			}
			if err != nil {
				deletionErrors = append(deletionErrors, fmt.Sprintf("检查 %q 失败: %v", candidate.Path, err))
				c.logger.Error().Int("task_id", task.Id).Int("resource_id", resource.Id).Str("path", candidate.Path).Err(err).Msg("Failed to inspect associated local file before removal")
				continue
			}
			isRecordingDir := candidate.CandidateType == "recording" && info.IsDir() && strings.HasSuffix(candidate.Path, ".recording")
			if info.Mode()&os.ModeSymlink != 0 || (!info.Mode().IsRegular() && !isRecordingDir) {
				err := fmt.Errorf("拒绝删除非普通文件 %q (mode=%s)", candidate.Path, info.Mode())
				deletionErrors = append(deletionErrors, err.Error())
				c.logger.Error().Int("task_id", task.Id).Int("resource_id", resource.Id).Str("path", candidate.Path).Str("mode", info.Mode().String()).Msg("Rejected unsafe associated local file removal")
				continue
			}
			remove := os.Remove
			if isRecordingDir {
				remove = os.RemoveAll
			}
			if err := remove(candidate.Path); err != nil {
				deletionErrors = append(deletionErrors, fmt.Sprintf("删除 %q 失败: %v", candidate.Path, err))
				c.logger.Error().Int("task_id", task.Id).Int("resource_id", resource.Id).Str("path_source", candidate.PathSource).Str("candidate_type", candidate.CandidateType).Str("path", candidate.Path).Int64("size", info.Size()).Err(err).Msg("Failed to remove associated local file")
				continue
			}
			c.logger.Info().Int("task_id", task.Id).Int("resource_id", resource.Id).Str("path_source", candidate.PathSource).Str("candidate_type", candidate.CandidateType).Str("path", candidate.Path).Int64("size", info.Size()).Msg("Associated local file removed")
		}
	}
	if len(deletionErrors) > 0 {
		return errors.New(strings.Join(deletionErrors, "; "))
	}
	return nil
}

// logDownloadTaskLocalFiles records all plausible final and partial paths without
// mutating the filesystem. The task config path is included because older tasks
// may have been created with a per-task download_dir that differs from DownloadDir.
func (c *APIClient) logDownloadTaskLocalFiles(task model.DownloadTask, resources []model.DownloadResource, phase string) {
	roots := c.download_task_local_file_roots(task, nil)
	c.logger.Info().Int("task_id", task.Id).Str("phase", phase).Int("resource_count", len(resources)).Int("candidate_root_count", len(roots)).Msg("Inspecting associated local file candidates")
	for _, resource := range resources {
		if strings.TrimSpace(resource.Name) == "" {
			c.logger.Warn().Int("task_id", task.Id).Int("resource_id", resource.Id).Str("phase", phase).Msg("Cannot resolve local file candidate because resource name is empty")
			continue
		}
		for _, candidate := range c.download_task_local_file_candidates(task, resource) {
			c.logDownloadTaskLocalFile(task.Id, resource, phase, candidate.PathSource, candidate.CandidateType, candidate.Path)
		}
	}
}

func (c *APIClient) logDownloadTaskLocalFile(taskID int, resource model.DownloadResource, phase, source, candidateType, path string) {
	info, err := os.Stat(path)
	event := c.logger.Info().
		Int("task_id", taskID).
		Int("resource_id", resource.Id).
		Str("resource_name", resource.Name).
		Str("resource_type", resource.Type).
		Str("phase", phase).
		Str("path_source", source).
		Str("candidate_type", candidateType).
		Str("path", path)
	if err == nil {
		event.Bool("exists", true).Bool("is_dir", info.IsDir()).Int64("size", info.Size()).Msg("Associated local file candidate inspected")
		return
	}
	if os.IsNotExist(err) {
		event.Bool("exists", false).Msg("Associated local file candidate inspected")
		return
	}
	event.Bool("exists", false).Err(err).Msg("Associated local file candidate inspection failed")
}

// handle_check_download_task_files checks a page of task files without delaying the
// task-list response. Paths supplied by the browser are never accessed directly:
// resource IDs are matched to their persisted task and resolved under the task's
// configured download roots.
// POST /api/v1/download_task/check_files
func (c *APIClient) handle_check_download_task_files(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	var body checkDownloadTaskFilesBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "请求参数错误: "+err.Error())
		return
	}
	if len(body.Files) == 0 {
		result.Ok(ctx, gin.H{"files": []checkDownloadTaskFileResult{}})
		return
	}
	if len(body.Files) > 5000 {
		result.Err(ctx, 400, "单次最多校验 5000 个文件")
		return
	}

	taskIDs := make([]int, 0, len(body.Files))
	resourceIDs := make([]int, 0, len(body.Files))
	seenTaskIDs := make(map[int]struct{})
	seenResourceIDs := make(map[int]struct{})
	for _, file := range body.Files {
		if file.TaskID > 0 {
			if _, exists := seenTaskIDs[file.TaskID]; !exists {
				seenTaskIDs[file.TaskID] = struct{}{}
				taskIDs = append(taskIDs, file.TaskID)
			}
		}
		if file.ID > 0 {
			if _, exists := seenResourceIDs[file.ID]; !exists {
				seenResourceIDs[file.ID] = struct{}{}
				resourceIDs = append(resourceIDs, file.ID)
			}
		}
	}

	var tasks []model.DownloadTask
	if len(taskIDs) > 0 {
		if err := c.db.Where("id IN ? AND deleted_at IS NULL", taskIDs).Find(&tasks).Error; err != nil {
			result.Err(ctx, 500, "查询下载任务失败: "+err.Error())
			return
		}
	}
	var resources []model.DownloadResource
	if len(resourceIDs) > 0 {
		if err := c.db.Where("id IN ? AND deleted_at IS NULL", resourceIDs).Find(&resources).Error; err != nil {
			result.Err(ctx, 500, "查询下载文件失败: "+err.Error())
			return
		}
	}

	taskByID := make(map[int]model.DownloadTask, len(tasks))
	for _, task := range tasks {
		taskByID[task.Id] = task
	}
	resourceByID := make(map[int]model.DownloadResource, len(resources))
	for _, resource := range resources {
		resourceByID[resource.Id] = resource
	}

	checkedFiles := make([]checkDownloadTaskFileResult, 0, len(body.Files))
	for _, file := range body.Files {
		checked := checkDownloadTaskFileResult{TaskID: file.TaskID, ID: file.ID}
		task, taskExists := taskByID[file.TaskID]
		resource, resourceExists := resourceByID[file.ID]
		if !taskExists || !resourceExists || resource.TaskId == nil || *resource.TaskId != task.Id {
			checked.Error = "下载文件记录不存在"
			checkedFiles = append(checkedFiles, checked)
			continue
		}

		candidates := c.download_task_local_file_candidates(task, resource)
		if len(candidates) == 0 {
			checked.Error = "无法解析本地文件路径"
			checkedFiles = append(checkedFiles, checked)
			continue
		}

		checked.Checked = true
		for _, candidate := range candidates {
			// A finished resource only exists when its final output exists. A
			// partial/recording artifact is valid evidence for an active resource.
			if (resource.Status == 2 || task.Status == model.TaskStatusFinished) && candidate.CandidateType != "final" {
				continue
			}
			info, err := os.Stat(candidate.Path)
			if err == nil {
				if !info.IsDir() || candidate.CandidateType == "recording" {
					checked.Exists = true
					break
				}
				continue
			}
			if !os.IsNotExist(err) {
				checked.Checked = false
				checked.Error = "检查本地文件失败"
				break
			}
		}
		checkedFiles = append(checkedFiles, checked)
	}

	result.Ok(ctx, gin.H{"files": checkedFiles})
}

// handle_list_download_task lists download tasks.
// GET /api/v1/download_task/list
func (c *APIClient) handle_list_download_task(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}
	if taskID, err := strconv.Atoi(ctx.Query("task_id")); err == nil && taskID > 0 {
		record, err := c.download_task_service.BuildTaskRecord(taskID)
		if err != nil {
			result.Err(ctx, 500, "查询下载任务失败: "+err.Error())
			return
		}
		if record == nil {
			result.Err(ctx, 404, "下载任务不存在")
			return
		}
		result.Ok(ctx, record)
		return
	}

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))
	statusFilter := ctx.Query("status")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var tasks []model.DownloadTask
	var total int64

	query := c.db.Model(&model.DownloadTask{}).Where("deleted_at IS NULL")
	if parentTaskID, err := strconv.Atoi(ctx.Query("parent_task_id")); err == nil && parentTaskID > 0 {
		query = query.Where("parent_task_id = ?", parentTaskID)
	}
	if rootTaskID, err := strconv.Atoi(ctx.Query("root_task_id")); err == nil && rootTaskID > 0 {
		query = query.Where("root_task_id = ?", rootTaskID)
	}
	if statusFilter != "" {
		parts := strings.Split(statusFilter, ",")
		ints := make([]int, 0, len(parts))
		for _, p := range parts {
			if v, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
				ints = append(ints, v)
			}
		}
		if len(ints) == 1 {
			query = query.Where("status = ?", ints[0])
		} else if len(ints) > 1 {
			query = query.Where("status IN ?", ints)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		result.Err(ctx, 500, "查询下载任务总数失败: "+err.Error())
		return
	}

	// stats: count of tasks grouped by status (same base filters minus status filter)
	stats, err := c.queryTaskStats(ctx)
	if err != nil {
		result.Err(ctx, 500, "查询下载任务统计失败: "+err.Error())
		return
	}

	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks).Error; err != nil {
		result.Err(ctx, 500, "查询下载任务失败: "+err.Error())
		return
	}

	list, err := c.download_task_service.BuildTaskRecords(tasks)
	if err != nil {
		result.Err(ctx, 500, "构建下载任务记录失败: "+err.Error())
		return
	}

	result.Ok(ctx, gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"stats":     stats,
	})
}

// queryTaskStats returns a map of status -> count for download tasks, respecting
// parent_task_id and root_task_id filters (but not status filter).
func (c *APIClient) queryTaskStats(ctx *gin.Context) (map[int]int64, error) {
	query := c.db.Model(&model.DownloadTask{}).Where("deleted_at IS NULL")
	if parentTaskID, err := strconv.Atoi(ctx.Query("parent_task_id")); err == nil && parentTaskID > 0 {
		query = query.Where("parent_task_id = ?", parentTaskID)
	}
	if rootTaskID, err := strconv.Atoi(ctx.Query("root_task_id")); err == nil && rootTaskID > 0 {
		query = query.Where("root_task_id = ?", rootTaskID)
	}

	type statusCount struct {
		Status int   `gorm:"column:status"`
		Count  int64 `gorm:"column:count"`
	}
	var rows []statusCount
	if err := query.Select("status, COUNT(*) as count").Group("status").Find(&rows).Error; err != nil {
		return nil, err
	}

	stats := make(map[int]int64, len(rows))
	for _, r := range rows {
		stats[r.Status] = r.Count
	}
	return stats, nil
}

// ResourceTreeNode is a resource tree node for the frontend to render file directory structures.
type ResourceTreeNode struct {
	Name      string              `json:"name"`
	Type      string              `json:"type"` // "file" | "directory"
	Kind      string              `json:"kind,omitempty"`
	Endpoints []gin.H             `json:"endpoints,omitempty"`
	Children  []*ResourceTreeNode `json:"children,omitempty"`
}

// buildResourceTree splits a flat resource list into a directory tree by path.
// A resource name such as "chapters/0001.html" is placed under the "chapters" directory.
func buildResourceTree(resources []gin.H) *ResourceTreeNode {
	root := &ResourceTreeNode{Name: "", Type: "directory", Children: []*ResourceTreeNode{}}
	for _, r := range resources {
		name, _ := r["name"].(string)
		kind, _ := r["kind"].(string)
		eps, _ := r["endpoints"].([]gin.H)
		parts := strings.Split(name, "/")

		node := root
		for i, part := range parts {
			if i == len(parts)-1 {
				// File node
				node.Children = append(node.Children, &ResourceTreeNode{
					Name:      part,
					Type:      "file",
					Kind:      kind,
					Endpoints: eps,
				})
			} else {
				// Directory node, find or create
				var dir *ResourceTreeNode
				for _, child := range node.Children {
					if child.Type == "directory" && child.Name == part {
						dir = child
						break
					}
				}
				if dir == nil {
					dir = &ResourceTreeNode{
						Name:     part,
						Type:     "directory",
						Children: []*ResourceTreeNode{},
					}
					node.Children = append(node.Children, dir)
				}
				node = dir
			}
		}
	}
	return root
}

// handle_start_all_download_task batch-starts download tasks.
// POST /api/v1/download_task/start_all
func (c *APIClient) handle_start_all_download_task(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	var body struct {
		Status string `json:"status"`
	}
	ctx.ShouldBindJSON(&body)

	query := c.db.Where("deleted_at IS NULL")
	switch body.Status {
	case "waiting":
		query = query.Where("status = ?", model.TaskStatusWaiting)
	case "paused":
		query = query.Where("status = ?", model.TaskStatusPaused)
	case "failed":
		query = query.Where("status = ?", model.TaskStatusFailed)
	default:
		// Start all startable tasks
		query = query.Where("status IN (?, ?, ?)",
			model.TaskStatusWaiting, model.TaskStatusPaused, model.TaskStatusFailed)
	}

	var tasks []model.DownloadTask
	if err := query.Find(&tasks).Error; err != nil {
		result.Err(ctx, 500, "查询下载任务失败: "+err.Error())
		return
	}

	var started int
	available := c.downloader.MaxConcurrent() - c.downloader.RunningTaskCount()
	for _, task := range tasks {
		if available <= 0 {
			break
		}
		if err := c.downloader.StartTask(task.Id); err != nil {
			continue
		}
		started++
		available--
	}

	result.Ok(ctx, gin.H{"started": started, "total": len(tasks)})
}

// handle_pause_all_download_task batch-pauses download tasks.
// POST /api/v1/download_task/pause_all
func (c *APIClient) handle_pause_all_download_task(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	var body struct {
		Status string `json:"status"`
	}
	ctx.ShouldBindJSON(&body)

	query := c.db.Where("deleted_at IS NULL")
	switch body.Status {
	case "preparing":
		query = query.Where("status = ?", model.TaskStatusPreparing)
	case "downloading":
		query = query.Where("status = ?", model.TaskStatusDownloading)
	case "running":
		query = query.Where("status IN (?, ?)",
			model.TaskStatusPreparing, model.TaskStatusDownloading)
	default:
		query = query.Where("status IN (?, ?)",
			model.TaskStatusPreparing, model.TaskStatusDownloading)
	}

	var tasks []model.DownloadTask
	if err := query.Find(&tasks).Error; err != nil {
		result.Err(ctx, 500, "查询下载任务失败: "+err.Error())
		return
	}

	var paused int
	var failures []gin.H
	for _, task := range tasks {
		if c.hasStreamResources(task.Id) {
			if err := c.downloader.StopTask(task.Id); err != nil {
				failures = append(failures, gin.H{"task_id": task.Id, "error": err.Error()})
				continue
			}
			var stoppedTask model.DownloadTask
			if err := c.db.Where("id = ?", task.Id).First(&stoppedTask).Error; err != nil {
				failures = append(failures, gin.H{"task_id": task.Id, "error": err.Error()})
				continue
			}
			if stoppedTask.Status != model.TaskStatusFinished {
				message := strings.TrimSpace(stoppedTask.ErrorMessage)
				if message == "" {
					message = fmt.Sprintf("收尾后的任务状态异常: %d", stoppedTask.Status)
				}
				failures = append(failures, gin.H{"task_id": task.Id, "error": message})
				continue
			}
		} else {
			c.downloader.PauseTask(task.Id)
		}
		paused++
	}

	result.Ok(ctx, gin.H{"paused": paused, "total": len(tasks), "failures": failures})
}

// handle_clear_download_task clears the requested download task records.
// POST /api/v1/download_task/clear
func (c *APIClient) handle_clear_download_task(ctx *gin.Context) {
	var body clear_download_tasks_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(body.TaskIDs) == 0 {
		result.Err(ctx, 400, "task_ids 不能为空")
		return
	}
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	cleared, results := c.clear_download_task_records(body.TaskIDs, body.DeleteFiles)
	result.Ok(ctx, gin.H{
		"cleared": cleared,
		"total":   len(body.TaskIDs),
		"results": results,
	})
}

// handle_clear_all_download_task clears all download task records.
// POST /api/v1/download_task/clear_all
func (c *APIClient) handle_clear_all_download_task(ctx *gin.Context) {
	var body clear_all_download_tasks_body
	if ctx.Request.Body != nil && ctx.Request.ContentLength != 0 {
		if err := ctx.ShouldBindJSON(&body); err != nil {
			result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
			return
		}
	}
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	var task_ids []int
	if err := c.db.Model(&model.DownloadTask{}).
		Where("deleted_at IS NULL").
		Order("id ASC").
		Pluck("id", &task_ids).Error; err != nil {
		result.Err(ctx, 500, "查询下载任务失败: "+err.Error())
		return
	}

	cleared, results := c.clear_download_task_records(task_ids, body.DeleteFiles)
	result.Ok(ctx, gin.H{
		"cleared": cleared,
		"total":   len(task_ids),
		"results": results,
	})
}

func (c *APIClient) clear_download_task_records(task_ids []int, delete_files bool) (int, []gin.H) {
	results := make([]gin.H, 0, len(task_ids))
	cleared := 0
	for _, task_id := range task_ids {
		clear_result := c.deleteSingleDownloadTask(task_id, delete_files)
		results = append(results, clear_result)
		if success, _ := clear_result["success"].(bool); success {
			cleared++
		}
	}
	if cleared > 0 {
		c.broadcast_download_task_stats()
	}
	return cleared, results
}

// hasStreamResources checks whether the task contains STREAM type resources (live streams).
func (c *APIClient) hasStreamResources(taskID int) bool {
	if c.db == nil {
		return false
	}
	var count int64
	c.db.Model(&model.DownloadResource{}).
		Where("task_id = ? AND UPPER(type) = ?", taskID, model.ResourceTypeStream).
		Count(&count)
	return count > 0
}

// handle_download_task_detail returns download task detail with enriched file info.
// GET /api/v1/download_task/detail
func (c *APIClient) handle_download_task_detail(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	taskID, err := strconv.Atoi(ctx.Query("id"))
	if err != nil || taskID <= 0 {
		result.Err(ctx, 400, "id is required and must be a positive integer")
		return
	}

	record, err := c.download_task_service.BuildTaskRecord(taskID)
	if err != nil {
		result.Err(ctx, 500, "查询下载任务失败: "+err.Error())
		return
	}
	if record == nil {
		result.Err(ctx, 404, "下载任务不存在")
		return
	}

	// Look up associated content if present.
	var contentData gin.H
	if record.ContentID != nil && *record.ContentID != "" {
		var content model.Content
		if err := c.db.Where("id = ?", *record.ContentID).First(&content).Error; err == nil {
			publishTime := int64(0)
			if content.PublishTime != nil {
				publishTime = *content.PublishTime
			}
			contentData = gin.H{
				"id":           content.Id,
				"platform_id":  content.PlatformId,
				"type":         content.Type,
				"title":        content.Title,
				"description":  content.Description,
				"cover_url":    content.CoverURL,
				"url":          content.URL,
				"source_url":   content.SourceURL,
				"publish_time": publishTime,
			}

			// Load accounts linked to this content.
			var accounts []gin.H
			type accountRow struct {
				AccountID  string `gorm:"column:account_id"`
				Nickname   string `gorm:"column:nickname"`
				AvatarURL  string `gorm:"column:avatar_url"`
				ExternalID string `gorm:"column:external_id"`
			}
			var rows []accountRow
			if err := c.db.Table("content_account").
				Select("content_account.account_id, account.nickname, account.avatar_url, account.external_id").
				Joins("JOIN account ON account.id = content_account.account_id").
				Where("content_account.content_id = ? AND account.deleted_at IS NULL", *record.ContentID).
				Scan(&rows).Error; err == nil && len(rows) > 0 {
				for _, r := range rows {
					accounts = append(accounts, gin.H{
						"id":          r.AccountID,
						"nickname":    r.Nickname,
						"avatar_url":  r.AvatarURL,
						"external_id": r.ExternalID,
					})
				}
			}
			contentData["accounts"] = accounts
		}
	}

	// Enrich files with local file info.
	type fileWithPath struct {
		services.DownloadTaskFileRecord
		LocalPath string `json:"local_path"`
		FileType  string `json:"file_type"`
		FileURL   string `json:"file_url"`
		Exists    bool   `json:"exists"`
	}

	files := make([]fileWithPath, 0, len(record.Files))
	for _, f := range record.Files {
		local_path := filepath.Join(f.DownloadDir, f.Name)
		exists := false
		if _, stat_err := os.Stat(local_path); stat_err == nil {
			exists = true
		}
		files = append(files, fileWithPath{
			DownloadTaskFileRecord: f,
			LocalPath:              local_path,
			FileType:               file_type_by_ext(f.Name),
			FileURL:                api_file_url(local_path),
			Exists:                 exists,
		})
	}

	record.Files = nil // replaced by enriched files below
	result.Ok(ctx, gin.H{
		"id":             record.ID,
		"content":        contentData,
		"content_id":     record.ContentID,
		"content_type":   record.ContentType,
		"parent_task_id": record.ParentTaskID,
		"root_task_id":   record.RootTaskID,
		"relation_type":  record.RelationType,
		"child_count":    record.ChildCount,
		"name":           record.Name,
		"platform_id":    record.PlatformID,
		"status":         record.Status,
		"source_url":     record.SourceURL,
		"cover_url":      record.CoverURL,
		"cover_width":    record.CoverWidth,
		"cover_height":   record.CoverHeight,
		"config_json":    record.ConfigJSON,
		"metadata_json":  record.MetadataJSON,
		"url":            record.URL,
		"size":           record.Size,
		"downloaded":     record.Downloaded,
		"speed":          record.Speed,
		"progress":       record.Progress,
		"error":          record.Error,
		"files":          files,
		"file_count":     len(files),
		"created_at":     record.CreatedAt,
		"updated_at":     record.UpdatedAt,
	})
}
