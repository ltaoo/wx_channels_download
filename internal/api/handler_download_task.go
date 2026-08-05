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

	"github.com/adrg/xdg"
	"github.com/gin-gonic/gin"

	"wx_channel/internal/database/model"
	"wx_channel/internal/adapter"
	"wx_channel/internal/download/tasklineage"
	"wx_channel/internal/events"
	"wx_channel/internal/services"
	result "wx_channel/internal/util"
)

// CreateDownloadTaskRequest is the request body for creating download tasks.
type CreateDownloadTaskRequest struct {
	Objects []services.CreateDownloadTaskBody `json:"objects"`
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

// CreateDownloadTaskByURLRequest is the request for creating download tasks by URL.
type CreateDownloadTaskByURLRequest struct {
	Objects []CreateDownloadTaskByURLBody `json:"objects"`
}

// CreateDownloadTaskByURLBody is a single item in the create-by-URL request.
type CreateDownloadTaskByURLBody struct {
	URL          string         `json:"url"`       // resource download URL, required
	SavePath     string         `json:"save_path"` // save directory
	Filename     string         `json:"filename"`  // filename (optional, extracted from URL by default)
	Config       map[string]any `json:"config"`    // custom download config
	ParentTaskID *int           `json:"parent_task_id"`
	RelationType string         `json:"relation_type"`
}

// resolveDownloadSaveDir resolves the download save directory for a task.
// Uses app config default when none is specified; relative paths are expanded
// relative to the working directory.
func (c *APIClient) resolveDownloadSaveDir(requested string) (string, error) {
	savePath := strings.TrimSpace(requested)
	if savePath == "" && c.cfg != nil {
		savePath = strings.TrimSpace(c.cfg.DownloadDir)
	}
	if savePath == "" {
		return "", fmt.Errorf("保存目录不能为空")
	}

	workDir := ""
	if c.cfg != nil {
		workDir = c.cfg.WorkDir
	}
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("获取工作目录失败: %w", err)
		}
	}

	savePath = strings.ReplaceAll(savePath, "%UserDownloads%", xdg.UserDirs.Download)
	savePath = strings.ReplaceAll(savePath, "%CWD%", workDir)
	savePath = filepath.Clean(savePath)
	if !filepath.IsAbs(savePath) {
		savePath = filepath.Join(workDir, savePath)
	}

	if err := os.MkdirAll(savePath, 0755); err != nil {
		return "", fmt.Errorf("创建保存目录 %q 失败: %w", savePath, err)
	}

	return savePath, nil
}

// downloadTaskSavePath returns the task save root directory.
func downloadTaskSavePath(saveDir string) string {
	return saveDir
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
	if err := c.downloader.StartTask(taskID); err != nil {
		c.logger.Error().Int("task_id", taskID).Err(err).Msg("Hermes scheduler failed to start download task")
		return err
	}
	c.logger.Info().Int("task_id", taskID).Msg("Download task submitted to Hermes scheduling queue")
	return nil
}

// prepareDownloadTaskSingle previews a single platform download task (no DB write, no download start).
func (c *APIClient) prepareDownloadTaskSingle(body services.CreateDownloadTaskBody) (gin.H, error) {
	if body.Platform == "" {
		return nil, fmt.Errorf("platform 不能为空")
	}

	h := adapter.Get(body.Platform)
	if h == nil {
		return nil, fmt.Errorf("不支持的平台: %s", body.Platform)
	}

	saveDir, err := c.resolveDownloadSaveDir(body.SavePath)
	if err != nil {
		return nil, fmt.Errorf("准备保存目录失败: %w", err)
	}

	config := make(map[string]any, len(body.Config)+2)
	for key, value := range body.Config {
		config[key] = value
	}
	config["save_path"] = saveDir
	if body.Filename != "" {
		config["filename"] = body.Filename
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("构建下载配置失败: %w", err)
	}

	info, err := h.BuildDownloadTask(body.Content, json.RawMessage(configJSON))
	if err != nil {
		return nil, fmt.Errorf("构建下载任务失败: %w", err)
	}
	if info == nil {
		return nil, fmt.Errorf("构建下载任务失败: 平台未返回下载任务")
	}

	savePath := downloadTaskSavePath(saveDir)

	for _, ri := range info.Resources {
		if len(ri.Endpoints) == 0 {
			return nil, fmt.Errorf("资源 %s 没有下载端点", ri.Name)
		}
	}

	// Build preview data (no DB write)
	resources := make([]gin.H, 0, len(info.Resources))
	totalEndpoints := 0
	for i, ri := range info.Resources {
		eps := make([]gin.H, 0, len(ri.Endpoints))
		for _, ep := range ri.Endpoints {
			eps = append(eps, gin.H{
				"protocol": ep.Protocol,
				"url":      ep.URL,
				"priority": ep.Priority,
			})
		}
		resources = append(resources, gin.H{
			"index":     i,
			"name":      ri.Name,
			"kind":      ri.Kind,
			"endpoints": eps,
		})
		totalEndpoints += len(ri.Endpoints)
	}
	tree := buildResourceTree(resources)

	return gin.H{
		"platform":       body.Platform,
		"task_name":      info.Task.Name,
		"save_path":      savePath,
		"resources":      resources,
		"tree":           tree,
		"resource_count": len(info.Resources),
		"endpoint_count": totalEndpoints,
		"content":        info.Content,
		"account":        info.Account,
	}, nil
}

// handlePrepareDownloadTask batch-previews platform download tasks.
// POST /api/v1/download_task/prepare
func (c *APIClient) handlePrepareDownloadTask(ctx *gin.Context) {
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

	requestedSavePath := body.SavePath
	if requestedSavePath == "" {
		requestedSavePath, _ = body.Config["save_path"].(string)
	}
	saveDir, err := c.resolveDownloadSaveDir(requestedSavePath)
	if err != nil {
		return nil, fmt.Errorf("准备保存目录失败: %w", err)
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

	savePath := downloadTaskSavePath(saveDir)

	return gin.H{
		"url":       body.URL,
		"protocol":  protocol,
		"task_name": filename,
		"save_path": savePath,
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

// handlePrepareDownloadTaskByURL batch-previews download tasks created by resource URL.
// POST /api/v1/download_task/prepare_by_url
func (c *APIClient) handlePrepareDownloadTaskByURL(ctx *gin.Context) {
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

// createDownloadTaskSingle creates a single platform download task and returns the result data or error.
func (c *APIClient) createDownloadTaskSingle(body services.CreateDownloadTaskBody) (gin.H, error) {
	if c.download_task_service == nil {
		return nil, fmt.Errorf("下载任务服务未初始化")
	}
	result, err := c.download_task_service.CreateTask(body)
	if err != nil {
		return nil, err
	}
	return gin.H{
		"task":      result.Task,
		"resource":  result.Resource,
		"endpoint":  result.Endpoint,
		"resources": result.Resources,
		"endpoints": result.Endpoints,
		"content":   result.Content,
		"account":   result.Account,
	}, nil
}

// handleCreateDownloadTask batch-creates platform download tasks.
// POST /api/v1/download_task/create
func (c *APIClient) handleCreateDownloadTask(ctx *gin.Context) {
	var req CreateDownloadTaskRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.logger.Warn().Str("api", "POST /api/v1/download_task/create").Err(err).Msg("Failed to parse request body")
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(req.Objects) == 0 {
		result.Err(ctx, 400, "请求体不能为空数组")
		return
	}

	c.logger.Info().Str("api", "POST /api/v1/download_task/create").Int("object_count", len(req.Objects)).Msg("Received batch create download task request")

	var duplicateErr *services.DuplicateTaskError
	tasks := make([]gin.H, 0, len(req.Objects))
	ids := make([]int, 0, len(req.Objects))
	successCount := 0
	failCount := 0
	for _, body := range req.Objects {
		data, err := c.createDownloadTaskSingle(body)
		if err != nil {
			if errors.As(err, &duplicateErr) {
				// Single task conflict: return business code 409 (HTTP 200), frontend prompts user
				if len(req.Objects) == 1 {
					c.logger.Warn().
						Str("api", "POST /api/v1/download_task/create").
						Int("existing_task_id", duplicateErr.ExistingTaskID).
						Str("incoming_task_unique_id", duplicateErr.IncomingUniqueID).
						Msg("Task conflict, already exists")
					resp := gin.H{"existing_task_id": duplicateErr.ExistingTaskID}
					result.ErrWithData(ctx, result.CodeDuplicateTask, duplicateErr.Error(), resp)
					return
				}
				c.logger.Warn().
					Str("api", "POST /api/v1/download_task/create").
					Int("existing_task_id", duplicateErr.ExistingTaskID).
					Str("incoming_task_unique_id", duplicateErr.IncomingUniqueID).
					Msg("Task conflict in batch, skipping")
				tasks = append(tasks, gin.H{"success": false, "error": err.Error(), "duplicate": true, "existing_task_id": duplicateErr.ExistingTaskID})
				failCount++
				continue
			}
			c.logger.Warn().Str("platform", body.Platform).Err(err).Msg("Failed to create download task")
			tasks = append(tasks, gin.H{"success": false, "error": err.Error()})
			failCount++
		} else {
			tasks = append(tasks, gin.H{"success": true, "data": data})
			if task, ok := data["task"].(model.DownloadTask); ok {
				ids = append(ids, task.Id)
			}
			successCount++
		}
	}

	c.logger.Info().
		Str("api", "POST /api/v1/download_task/create").
		Int("total", len(tasks)).
		Int("success", successCount).
		Int("failed", failCount).
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

	// Store original download URL in config_json
	configJSON, _ := json.Marshal(map[string]string{
		"url": body.URL,
	})

	// Database not initialized
	if c.db == nil {
		return nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	now := time.Now().UnixMilli()

	// Create task
	task := model.DownloadTask{
		Name:       taskName,
		Status:     model.TaskStatusWaiting,
		ConfigJSON: string(configJSON),
	}
	task.CreatedAt = now
	task.UpdatedAt = now

	if err := tasklineage.Apply(c.db, &task, body.ParentTaskID, body.RelationType); err != nil {
		return nil, err
	}
	if err := c.db.Create(&task).Error; err != nil {
		c.logger.Error().Str("url", body.URL).Err(err).Msg("URL download task failed to write to database")
		return nil, fmt.Errorf("创建下载任务失败: %w", err)
	}
	if err := tasklineage.FinalizeRoot(c.db, &task); err != nil {
		return nil, err
	}

	c.logger.Info().Int("task_id", task.Id).Str("url", body.URL).Msg("URL download task written to database")
	// Create resource
	resource := model.DownloadResource{
		TaskId:     task.Id,
		Name:       filename,
		Kind:       "file",
		Status:     0,
		MergeOrder: 0,
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

	// Hand off to scheduler; task enters PREPARING first, transitions to DOWNLOADING after acquiring a concurrency slot.
	if err := c.startCreatedDownloadTask(task.Id); err != nil {
		return nil, fmt.Errorf("启动下载任务失败: %w", err)
	}
	task.Status = model.TaskStatusPreparing // Hermes has written to DB; here we only update the in-memory variable for the response

	return gin.H{
		"task":     task,
		"resource": resource,
		"endpoint": endpoint,
	}, nil
}

// handleCreateDownloadTaskByURL batch-creates download tasks by resource URL.
// POST /api/v1/download_task/create_by_url
func (c *APIClient) handleCreateDownloadTaskByURL(ctx *gin.Context) {
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
	successCount := 0
	failCount := 0
	for _, body := range req.Objects {
		data, err := c.createDownloadTaskByURLSingle(body)
		if err != nil {
			c.logger.Warn().Str("url", body.URL).Err(err).Msg("Failed to create URL download task")
			tasks = append(tasks, gin.H{"success": false, "error": err.Error()})
			failCount++
		} else {
			tasks = append(tasks, gin.H{"success": true, "data": data})
			successCount++
		}
	}

	c.logger.Info().
		Str("api", "POST /api/v1/download_task/create_by_url").
		Int("total", len(tasks)).
		Int("success", successCount).
		Int("failed", failCount).
		Msg("Batch create URL download tasks completed")

	result.Ok(ctx, gin.H{"tasks": tasks})
}

// handleStartDownloadTask batch-starts download tasks.
// POST /api/v1/download_task/start
func (c *APIClient) handleStartDownloadTask(ctx *gin.Context) {
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

// handlePauseDownloadTask batch-pauses download tasks.
// POST /api/v1/download_task/pause
func (c *APIClient) handlePauseDownloadTask(ctx *gin.Context) {
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

		c.downloader.PauseTask(task.Id)

		if c.hasStreamResources(task.Id) {
			now := time.Now().UnixMilli()
			c.db.Model(&task).Updates(map[string]any{"status": model.TaskStatusFinished, "updated_at": now})
			task.Status = model.TaskStatusFinished
			if c.bus != nil {
				go c.bus.Publish(events.DownloadTaskFinished{TaskID: task.Id})
			}
			results = append(results, gin.H{"task_id": taskID, "success": true, "task": task, "status_text": "finished"})
			continue
		}

		task.Status = model.TaskStatusPaused
		results = append(results, gin.H{"task_id": taskID, "success": true, "task": task, "status_text": "paused"})
	}

	result.Ok(ctx, gin.H{"results": results})
}

// handleResumeDownloadTask batch-resumes download tasks.
// POST /api/v1/download_task/resume
func (c *APIClient) handleResumeDownloadTask(ctx *gin.Context) {
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

		if err := c.downloader.StartTask(task.Id); err != nil {
			results = append(results, gin.H{"task_id": taskID, "success": false, "error": "恢复下载任务失败: " + err.Error()})
			continue
		}
		task.Status = model.TaskStatusPreparing
		results = append(results, gin.H{"task_id": taskID, "success": true, "task": task, "status_text": "preparing"})
	}

	result.Ok(ctx, gin.H{"results": results})
}

// handleRetryDownloadTask batch-retries failed/cancelled download tasks.
// POST /api/v1/download_task/retry
func (c *APIClient) handleRetryDownloadTask(ctx *gin.Context) {
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

// handleDeleteDownloadTask batch-deletes download tasks.
// POST /api/v1/download_task/delete
func (c *APIClient) handleDeleteDownloadTask(ctx *gin.Context) {
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
	for _, taskID := range body.TaskIDs {
		r := c.deleteSingleDownloadTask(taskID, body.DeleteFiles)
		results = append(results, r)
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
	deletedRecord, recordErr := c.download_task_service.BuildTaskRecord(task.Id)
	if recordErr != nil {
		c.logger.Warn().Int("task_id", task.Id).Err(recordErr).Msg("Download task deletion failed to build pre-delete broadcast record")
	}

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

	if deletedRecord != nil {
		c.broadcastDownloadTaskDelete([]services.DownloadTaskRecord{*deletedRecord})
		c.logger.Info().Int("task_id", task.Id).Msg("Download task deletion broadcast emitted")
	} else {
		c.logger.Warn().Int("task_id", task.Id).Msg("Download task deletion broadcast skipped because no pre-delete record was available")
	}
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

func (c *APIClient) downloadTaskLocalFileRoots(task model.DownloadTask) map[string]string {
	roots := make(map[string]string)
	addRoot := func(root, source string) {
		root = strings.TrimSpace(root)
		if root == "" {
			return
		}
		if !filepath.IsAbs(root) && c.cfg != nil && strings.TrimSpace(c.cfg.WorkDir) != "" {
			root = filepath.Join(c.cfg.WorkDir, root)
		}
		absoluteRoot, err := filepath.Abs(root)
		if err != nil {
			c.logger.Warn().Int("task_id", task.Id).Str("path_source", source).Str("root", root).Err(err).Msg("Unable to resolve local file candidate root")
			return
		}
		roots[filepath.Clean(absoluteRoot)] = source
	}
	if c.cfg != nil {
		addRoot(c.cfg.DownloadDir, "download_root")
	}
	var taskConfig map[string]any
	if strings.TrimSpace(task.ConfigJSON) != "" {
		if err := json.Unmarshal([]byte(task.ConfigJSON), &taskConfig); err != nil {
			c.logger.Warn().Int("task_id", task.Id).Err(err).Msg("Unable to parse task config while resolving local file roots")
		} else {
			savePath, _ := taskConfig["save_path"].(string)
			addRoot(savePath, "task_config_save_path")
		}
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

func (c *APIClient) downloadTaskLocalFileCandidates(task model.DownloadTask, resource model.DownloadResource) []downloadTaskLocalFileCandidate {
	name := strings.TrimSpace(resource.Name)
	if name == "" {
		return nil
	}
	roots := c.downloadTaskLocalFileRoots(task)
	seen := make(map[string]struct{})
	candidates := make([]downloadTaskLocalFileCandidate, 0, len(roots)*2)
	for root, source := range roots {
		path := name
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		path = filepath.Clean(path)
		if !pathWithinDownloadRoot(root, path) {
			c.logger.Error().Int("task_id", task.Id).Int("resource_id", resource.Id).Str("resource_name", resource.Name).Str("download_root", root).Str("path", path).Msg("Rejected local file candidate outside download root")
			continue
		}
		for _, candidate := range []downloadTaskLocalFileCandidate{
			{Path: path, PathSource: source, CandidateType: "final"},
			{Path: path + ".part", PathSource: source, CandidateType: "partial"},
		} {
			if _, exists := seen[candidate.Path]; exists {
				continue
			}
			seen[candidate.Path] = struct{}{}
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

func (c *APIClient) deleteDownloadTaskLocalFiles(task model.DownloadTask, resources []model.DownloadResource) error {
	var deletionErrors []string
	for _, resource := range resources {
		candidates := c.downloadTaskLocalFileCandidates(task, resource)
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
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				err := fmt.Errorf("拒绝删除非普通文件 %q (mode=%s)", candidate.Path, info.Mode())
				deletionErrors = append(deletionErrors, err.Error())
				c.logger.Error().Int("task_id", task.Id).Int("resource_id", resource.Id).Str("path", candidate.Path).Str("mode", info.Mode().String()).Msg("Rejected unsafe associated local file removal")
				continue
			}
			if err := os.Remove(candidate.Path); err != nil {
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
// may have been created with a per-task save_path that differs from DownloadDir.
func (c *APIClient) logDownloadTaskLocalFiles(task model.DownloadTask, resources []model.DownloadResource, phase string) {
	roots := c.downloadTaskLocalFileRoots(task)
	c.logger.Info().Int("task_id", task.Id).Str("phase", phase).Int("resource_count", len(resources)).Int("candidate_root_count", len(roots)).Msg("Inspecting associated local file candidates")
	for _, resource := range resources {
		if strings.TrimSpace(resource.Name) == "" {
			c.logger.Warn().Int("task_id", task.Id).Int("resource_id", resource.Id).Str("phase", phase).Msg("Cannot resolve local file candidate because resource name is empty")
			continue
		}
		for _, candidate := range c.downloadTaskLocalFileCandidates(task, resource) {
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

// handleListDownloadTask lists download tasks.
// GET /api/v1/download_task/list
func (c *APIClient) handleListDownloadTask(ctx *gin.Context) {
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

// handleStartAllDownloadTask batch-starts download tasks.
// POST /api/v1/download_task/start_all
func (c *APIClient) handleStartAllDownloadTask(ctx *gin.Context) {
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
	for _, task := range tasks {
		if err := c.downloader.StartTask(task.Id); err != nil {
			continue
		}
		started++
	}

	result.Ok(ctx, gin.H{"started": started, "total": len(tasks)})
}

// handlePauseAllDownloadTask batch-pauses download tasks.
// POST /api/v1/download_task/pause_all
func (c *APIClient) handlePauseAllDownloadTask(ctx *gin.Context) {
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
	for _, task := range tasks {
		c.downloader.PauseTask(task.Id)
		// Stream pause should be marked as finished
		if c.hasStreamResources(task.Id) {
			now := time.Now().UnixMilli()
			c.db.Model(&model.DownloadTask{}).Where("id = ?", task.Id).
				Updates(map[string]any{"status": model.TaskStatusFinished, "updated_at": now})
			if c.bus != nil {
				go c.bus.Publish(events.DownloadTaskFinished{TaskID: task.Id})
			}
		}
		paused++
	}

	result.Ok(ctx, gin.H{"paused": paused, "total": len(tasks)})
}

// handleClearDownloadTask clears completed/failed/cancelled download tasks.
// POST /api/v1/download_task/clear
func (c *APIClient) handleClearDownloadTask(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	var body struct {
		DeleteFiles bool `json:"delete_files"`
	}
	ctx.ShouldBindJSON(&body)

	var tasks []model.DownloadTask
	if err := c.db.Where("deleted_at IS NULL").
		Where("status IN (?, ?, ?)",
			model.TaskStatusFinished, model.TaskStatusFailed, model.TaskStatusCancelled).
		Find(&tasks).Error; err != nil {
		result.Err(ctx, 500, "查询下载任务失败: "+err.Error())
		return
	}

	now := time.Now().UnixMilli()
	var cleared int

	for _, task := range tasks {
		c.downloader.DeleteTask(task.Id)

		// Soft-delete task
		c.db.Model(&task).Update("deleted_at", now)

		// Cascade soft-delete related data
		c.db.Model(&model.DownloadResource{}).Where("task_id = ?", task.Id).Update("deleted_at", now)

		var resourceIDs []int
		c.db.Model(&model.DownloadResource{}).Where("task_id = ?", task.Id).Pluck("id", &resourceIDs)
		if len(resourceIDs) > 0 {
			c.db.Model(&model.DownloadEndpoint{}).Where("resource_id IN ?", resourceIDs).Update("deleted_at", now)
			c.db.Model(&model.DownloadSegment{}).Where("resource_id IN ?", resourceIDs).Update("deleted_at", now)

			var endpointIDs []int
			c.db.Model(&model.DownloadEndpoint{}).Where("resource_id IN ?", resourceIDs).Pluck("id", &endpointIDs)
			if len(endpointIDs) > 0 {
				c.db.Model(&model.DownloadConnection{}).Where("endpoint_id IN ?", endpointIDs).Update("deleted_at", now)
			}
		}

		cleared++
	}

	result.Ok(ctx, gin.H{"cleared": cleared})
}

// hasStreamResources checks whether the task contains STREAM type resources (live streams).
func (c *APIClient) hasStreamResources(taskID int) bool {
	if c.db == nil {
		return false
	}
	var count int64
	c.db.Model(&model.DownloadResource{}).
		Where("task_id = ? AND type = ?", taskID, model.ResourceTypeStream).
		Count(&count)
	return count > 0
}
