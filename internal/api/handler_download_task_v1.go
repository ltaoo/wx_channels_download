package api

import (
	"encoding/json"
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
	"wx_channel/internal/download/registry"
	result "wx_channel/internal/util"
)

// CreateDownloadTaskV1Body 创建下载任务请求体
type CreateDownloadTaskV1Body struct {
	Platform string          `json:"platform"` // 内容平台
	Content  json.RawMessage `json:"content"`  // 平台内容原始 JSON
	Config   DownloadConfig  `json:"config"`   // 下载配置
}

// DownloadConfig 下载配置
type DownloadConfig struct {
	SavePath      string `json:"save_path"`
	Filename      string `json:"filename"`
	Spec          string `json:"spec"`
	DownloadCover bool   `json:"download_cover"`
	Overwrite     bool   `json:"overwrite"`
	SkipDuplicate bool   `json:"skip_duplicate"`
}

// taskV1IDBody 通用 task_id 请求体
type taskV1IDBody struct {
	TaskID int `json:"task_id"`
}

// CreateDownloadTaskByURLBody 通过资源地址创建下载任务请求体
type CreateDownloadTaskByURLBody struct {
	URL      string         `json:"url"`       // 资源下载地址，必填
	SavePath string         `json:"save_path"` // 保存路径
	Filename string         `json:"filename"`  // 文件名（可选，默认从URL提取）
	Config   DownloadConfig `json:"config"`    // 下载配置
}

// resolveDownloadSaveDir 统一解析下载任务保存目录。
// 请求未指定目录时使用应用配置；相对路径相对于工作目录展开。
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

// downloadTaskSavePath 根据资源类型生成任务最终保存路径。
// FILE 保存完整文件路径，COLLECTION/STREAM 保存输出根目录。
func downloadTaskSavePath(saveDir, resourceType, resourceName string) (string, error) {
	if resourceType != model.ResourceTypeFile {
		return saveDir, nil
	}

	filename := filepath.Base(strings.TrimSpace(resourceName))
	if filename == "" || filename == "." || filename == ".." || filename == string(filepath.Separator) {
		return "", fmt.Errorf("无法确定下载文件名")
	}
	return filepath.Join(saveDir, filename), nil
}

// startCreatedDownloadTask 补齐新建任务的连接记录并立即交给 Hermes 调度。
func (c *APIClient) startCreatedDownloadTask(task *model.DownloadTaskV1, endpoints []model.DownloadEndpoint) error {
	if c.downloader == nil {
		return fmt.Errorf("Hermes 下载器未初始化")
	}

	now := time.Now().UnixMilli()
	for _, endpoint := range endpoints {
		host := ""
		if parsedURL, err := url.Parse(endpoint.URL); err == nil {
			host = parsedURL.Host
		}
		connection := model.DownloadConnection{
			EndpointId: endpoint.Id,
			WorkerId:   "worker-" + strconv.Itoa(endpoint.Id),
			Host:       host,
			Status:     1,
			LastActive: now,
		}
		connection.CreatedAt = now
		connection.UpdatedAt = now
		if err := c.db.Create(&connection).Error; err != nil {
			return fmt.Errorf("创建下载连接失败: %w", err)
		}
	}

	if err := c.downloader.Start(task.Id); err != nil {
		return err
	}
	task.Status = model.TaskStatusPreparing
	c.broadcastDownloadTaskUpsert([]int{task.Id})
	return nil
}

// prepareDownloadTaskV1Single 预览单个平台下载任务（不写入数据库、不启动下载），返回将要创建的任务信息。
func (c *APIClient) prepareDownloadTaskV1Single(body CreateDownloadTaskV1Body) (gin.H, error) {
	if body.Platform == "" {
		return nil, fmt.Errorf("platform 不能为空")
	}

	h := registry.Get(body.Platform)
	if h == nil {
		return nil, fmt.Errorf("不支持的平台: " + body.Platform)
	}

	saveDir, err := c.resolveDownloadSaveDir(body.Config.SavePath)
	if err != nil {
		return nil, fmt.Errorf("准备保存目录失败: %w", err)
	}

	info, content, account, err := h.BuildDownloadTask(body.Content, registry.DownloadConfig{
		SavePath:      saveDir,
		Filename:      body.Config.Filename,
		Spec:          body.Config.Spec,
		DownloadCover: body.Config.DownloadCover,
		Overwrite:     body.Config.Overwrite,
		SkipDuplicate: body.Config.SkipDuplicate,
	})
	if err != nil {
		return nil, fmt.Errorf("构建下载任务失败: %w", err)
	}
	if info == nil {
		return nil, fmt.Errorf("构建下载任务失败: 平台未返回下载任务")
	}

	resourceInfos := info.Resources
	if len(resourceInfos) == 0 {
		resourceInfos = []registry.DownloadResourceInfo{{
			Resource:  info.Resource,
			Endpoints: []model.DownloadEndpoint{info.Endpoint},
		}}
	}
	if len(resourceInfos) > 1 {
		info.Task.ResourceType = model.ResourceTypeCollection
	}
	for _, resourceInfo := range resourceInfos {
		if len(resourceInfo.Endpoints) == 0 {
			return nil, fmt.Errorf("资源 " + resourceInfo.Resource.Name + " 没有下载端点")
		}
	}

	info.Task.SavePath, err = downloadTaskSavePath(saveDir, info.Task.ResourceType, resourceInfos[0].Resource.Name)
	if err != nil {
		return nil, fmt.Errorf("生成保存路径失败: %w", err)
	}

	// 构建预览数据（不写入数据库）
	resources := make([]gin.H, 0, len(resourceInfos))
	totalEndpoints := 0
	for i, ri := range resourceInfos {
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
			"name":      ri.Resource.Name,
			"kind":      ri.Resource.Kind,
			"task_name": ri.Resource.TaskName,
			"endpoints": eps,
		})
		totalEndpoints += len(ri.Endpoints)
	}

	return gin.H{
		"platform":        body.Platform,
		"task_name":       info.Task.Name,
		"resource_type":   info.Task.ResourceType,
		"save_path":       info.Task.SavePath,
		"resources":       resources,
		"resource_count":  len(resourceInfos),
		"endpoint_count":  totalEndpoints,
		"content":         content,
		"account":         account,
	}, nil
}

// handlePrepareDownloadTaskV1 批量预览平台下载任务
// POST /api/v1/download_task/prepare
func (c *APIClient) handlePrepareDownloadTaskV1(ctx *gin.Context) {
	var bodies []CreateDownloadTaskV1Body
	if err := ctx.ShouldBindJSON(&bodies); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(bodies) == 0 {
		result.Err(ctx, 400, "请求体不能为空数组")
		return
	}

	previews := make([]gin.H, 0, len(bodies))
	for _, body := range bodies {
		data, err := c.prepareDownloadTaskV1Single(body)
		if err != nil {
			previews = append(previews, gin.H{"success": false, "error": err.Error()})
		} else {
			previews = append(previews, gin.H{"success": true, "data": data})
		}
	}

	result.Ok(ctx, gin.H{"previews": previews})
}

// prepareDownloadTaskByURLV1Single 预览通过资源地址创建的下载任务（不写入数据库、不启动下载）。
func (c *APIClient) prepareDownloadTaskByURLV1Single(body CreateDownloadTaskByURLBody) (gin.H, error) {
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
		requestedSavePath = body.Config.SavePath
	}
	saveDir, err := c.resolveDownloadSaveDir(requestedSavePath)
	if err != nil {
		return nil, fmt.Errorf("准备保存目录失败: %w", err)
	}
	filename := body.Filename
	if filename == "" {
		filename = body.Config.Filename
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

	savePath, err := downloadTaskSavePath(saveDir, model.ResourceTypeFile, filename)
	if err != nil {
		return nil, fmt.Errorf("生成保存路径失败: %w", err)
	}

	return gin.H{
		"url":           body.URL,
		"protocol":      protocol,
		"task_name":     filename,
		"resource_type": model.ResourceTypeFile,
		"save_path":     savePath,
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

// handlePrepareDownloadTaskByURLV1 批量预览通过资源地址创建的下载任务
// POST /api/v1/download_task/prepare_by_url
func (c *APIClient) handlePrepareDownloadTaskByURLV1(ctx *gin.Context) {
	var bodies []CreateDownloadTaskByURLBody
	if err := ctx.ShouldBindJSON(&bodies); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(bodies) == 0 {
		result.Err(ctx, 400, "请求体不能为空数组")
		return
	}

	previews := make([]gin.H, 0, len(bodies))
	for _, body := range bodies {
		data, err := c.prepareDownloadTaskByURLV1Single(body)
		if err != nil {
			previews = append(previews, gin.H{"success": false, "error": err.Error()})
		} else {
			previews = append(previews, gin.H{"success": true, "data": data})
		}
	}

	result.Ok(ctx, gin.H{"previews": previews})
}

// createDownloadTaskV1Single 创建单个平台下载任务，返回结果数据或错误。
func (c *APIClient) createDownloadTaskV1Single(body CreateDownloadTaskV1Body) (gin.H, error) {
	if body.Platform == "" {
		return nil, fmt.Errorf("platform 不能为空")
	}

	// 根据平台获取对应的处理器
	h := registry.Get(body.Platform)
	if h == nil {
		return nil, fmt.Errorf("不支持的平台: " + body.Platform)
	}

	saveDir, err := c.resolveDownloadSaveDir(body.Config.SavePath)
	if err != nil {
		return nil, fmt.Errorf("准备保存目录失败: %w", err)
	}

	// 调用平台处理器构建下载模型
	info, content, account, err := h.BuildDownloadTask(body.Content, registry.DownloadConfig{
		SavePath:      saveDir,
		Filename:      body.Config.Filename,
		Spec:          body.Config.Spec,
		DownloadCover: body.Config.DownloadCover,
		Overwrite:     body.Config.Overwrite,
		SkipDuplicate: body.Config.SkipDuplicate,
	})
	if err != nil {
		return nil, fmt.Errorf("构建下载任务失败: %w", err)
	}
	if info == nil {
		return nil, fmt.Errorf("构建下载任务失败: 平台未返回下载任务")
	}

	resourceInfos := info.Resources
	if len(resourceInfos) == 0 {
		resourceInfos = []registry.DownloadResourceInfo{{
			Resource:  info.Resource,
			Endpoints: []model.DownloadEndpoint{info.Endpoint},
		}}
	}
	if len(resourceInfos) > 1 {
		info.Task.ResourceType = model.ResourceTypeCollection
	}
	for _, resourceInfo := range resourceInfos {
		if len(resourceInfo.Endpoints) == 0 {
			return nil, fmt.Errorf("资源 " + resourceInfo.Resource.Name + " 没有下载端点")
		}
	}

	// 保存路径由 API 统一生成，避免平台处理器使用各自的硬编码默认目录。
	info.Task.SavePath, err = downloadTaskSavePath(saveDir, info.Task.ResourceType, resourceInfos[0].Resource.Name)
	if err != nil {
		return nil, fmt.Errorf("生成保存路径失败: %w", err)
	}

	// 数据库未初始化
	if c.db == nil {
		return nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	// 写入数据库
	now := time.Now().UnixMilli()
	if info.Task.CreatedAt == 0 {
		info.Task.CreatedAt = now
	}
	info.Task.UpdatedAt = now
	if err := c.db.Create(&info.Task).Error; err != nil {
		return nil, fmt.Errorf("创建下载任务失败: %w", err)
	}

	resources := make([]model.DownloadResource, 0, len(resourceInfos))
	endpoints := make([]model.DownloadEndpoint, 0, len(resourceInfos))
	for i := range resourceInfos {
		resource := resourceInfos[i].Resource
		resource.TaskId = info.Task.Id
		if resource.CreatedAt == 0 {
			resource.CreatedAt = now
		}
		resource.UpdatedAt = now
		if err := c.db.Create(&resource).Error; err != nil {
			return nil, fmt.Errorf("创建资源失败: %w", err)
		}
		resources = append(resources, resource)
		for _, endpointInfo := range resourceInfos[i].Endpoints {
			endpoint := endpointInfo
			endpoint.ResourceId = resource.Id
			if endpoint.CreatedAt == 0 {
				endpoint.CreatedAt = now
			}
			endpoint.UpdatedAt = now
			if err := c.db.Create(&endpoint).Error; err != nil {
				return nil, fmt.Errorf("创建端点失败: %w", err)
			}
			endpoints = append(endpoints, endpoint)
		}
	}
	if len(resources) == 0 || len(endpoints) == 0 {
		return nil, fmt.Errorf("平台未返回可下载资源或端点")
	}
	info.Resource = resources[0]
	info.Endpoint = endpoints[0]

	if err := c.startCreatedDownloadTask(&info.Task, endpoints); err != nil {
		return nil, fmt.Errorf("启动下载任务失败: %w", err)
	}

	return gin.H{
		"task":      info.Task,
		"resource":  info.Resource,
		"endpoint":  info.Endpoint,
		"resources": resources,
		"endpoints": endpoints,
		"content":   content,
		"account":   account,
	}, nil
}

// handleCreateDownloadTaskV1 批量创建平台下载任务
// POST /api/v1/download_task/create
func (c *APIClient) handleCreateDownloadTaskV1(ctx *gin.Context) {
	var bodies []CreateDownloadTaskV1Body
	if err := ctx.ShouldBindJSON(&bodies); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(bodies) == 0 {
		result.Err(ctx, 400, "请求体不能为空数组")
		return
	}

	tasks := make([]gin.H, 0, len(bodies))
	for _, body := range bodies {
		data, err := c.createDownloadTaskV1Single(body)
		if err != nil {
			tasks = append(tasks, gin.H{"success": false, "error": err.Error()})
		} else {
			tasks = append(tasks, gin.H{"success": true, "data": data})
		}
	}

	result.Ok(ctx, gin.H{"tasks": tasks})
}

// createDownloadTaskByURLV1Single 通过资源地址创建单个下载任务。
func (c *APIClient) createDownloadTaskByURLV1Single(body CreateDownloadTaskByURLBody) (gin.H, error) {
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
		requestedSavePath = body.Config.SavePath
	}
	saveDir, err := c.resolveDownloadSaveDir(requestedSavePath)
	if err != nil {
		return nil, fmt.Errorf("准备保存目录失败: %w", err)
	}
	filename := body.Filename
	if filename == "" {
		filename = body.Config.Filename
	}
	if filename == "" {
		// 从 URL 路径提取文件名
		base := filepath.Base(parsedURL.Path)
		if base != "" && base != "." && base != "/" {
			if decoded, err := url.QueryUnescape(base); err == nil {
				filename = decoded
			} else {
				filename = base
			}
		}
	}
	// 如果仍然无法提取文件名，使用 URL 作为名称
	if filename == "" {
		filename = body.URL
	}
	filename = filepath.Base(filename)
	if filename == "" || filename == "." || filename == ".." || filename == string(filepath.Separator) {
		return nil, fmt.Errorf("无法确定下载文件名")
	}

	savePath, err := downloadTaskSavePath(saveDir, model.ResourceTypeFile, filename)
	if err != nil {
		return nil, fmt.Errorf("生成保存路径失败: %w", err)
	}

	taskName := filename

	// 存储原始下载地址到 config_json
	configJSON, _ := json.Marshal(map[string]string{
		"url": body.URL,
	})

	// 数据库未初始化
	if c.db == nil {
		return nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	now := time.Now().UnixMilli()

	// 创建任务
	task := model.DownloadTaskV1{
		Name:         taskName,
		ResourceType: model.ResourceTypeFile,
		Status:       model.TaskStatusWaiting,
		SavePath:     savePath,
		ConfigJSON:   string(configJSON),
	}
	task.CreatedAt = now
	task.UpdatedAt = now

	if err := c.db.Create(&task).Error; err != nil {
		return nil, fmt.Errorf("创建下载任务失败: %w", err)
	}

	// 创建资源
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

	// 创建端点
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

	// 交给调度器；任务先进入 PREPARING，获得并发槽位后再转为 DOWNLOADING。
	if err := c.startCreatedDownloadTask(&task, []model.DownloadEndpoint{endpoint}); err != nil {
		return nil, fmt.Errorf("启动下载任务失败: %w", err)
	}

	return gin.H{
		"task":     task,
		"resource": resource,
		"endpoint": endpoint,
	}, nil
}

// handleCreateDownloadTaskByURLV1 批量通过资源地址创建下载任务
// POST /api/v1/download_task/create_by_url
func (c *APIClient) handleCreateDownloadTaskByURLV1(ctx *gin.Context) {
	var bodies []CreateDownloadTaskByURLBody
	if err := ctx.ShouldBindJSON(&bodies); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(bodies) == 0 {
		result.Err(ctx, 400, "请求体不能为空数组")
		return
	}

	tasks := make([]gin.H, 0, len(bodies))
	for _, body := range bodies {
		data, err := c.createDownloadTaskByURLV1Single(body)
		if err != nil {
			tasks = append(tasks, gin.H{"success": false, "error": err.Error()})
		} else {
			tasks = append(tasks, gin.H{"success": true, "data": data})
		}
	}

	result.Ok(ctx, gin.H{"tasks": tasks})
}

// handleStartDownloadTaskV1 启动下载任务
// POST /api/v1/download_task/start
func (c *APIClient) handleStartDownloadTaskV1(ctx *gin.Context) {
	var body taskV1IDBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if body.TaskID <= 0 {
		result.Err(ctx, 400, "task_id 无效")
		return
	}
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	var task model.DownloadTaskV1
	if err := c.db.Where("id = ?", body.TaskID).First(&task).Error; err != nil {
		result.Err(ctx, 404, "下载任务不存在")
		return
	}

	// 只有 Waiting / Paused / Failed 状态可以启动
	if task.Status != model.TaskStatusWaiting &&
		task.Status != model.TaskStatusPaused &&
		task.Status != model.TaskStatusFailed {
		result.Err(ctx, 400, "当前状态不允许启动")
		return
	}

	// 获取端点 URL
	var ep model.DownloadEndpoint
	if err := c.db.Table("download_endpoint").
		Where("resource_id IN (SELECT id FROM download_resource WHERE task_id = ?)", task.Id).
		Order("priority ASC").First(&ep).Error; err != nil {
		result.Err(ctx, 500, "未找到下载端点")
		return
	}

	if err := c.downloader.Start(task.Id); err != nil {
		result.Err(ctx, 500, "启动下载任务失败: "+err.Error())
		return
	}

	now := time.Now().UnixMilli()
	task.Status = model.TaskStatusPreparing

	c.db.Create(&model.DownloadLog{
		TaskId:    task.Id,
		Level:     "info",
		Message:   "task started",
		CreatedAt: now,
	})

	c.broadcastDownloadTaskUpsert([]int{task.Id})

	result.Ok(ctx, gin.H{"task": task, "status_text": "preparing"})
}

// handlePauseDownloadTaskV1 暂停下载任务
// POST /api/v1/download_task/pause
func (c *APIClient) handlePauseDownloadTaskV1(ctx *gin.Context) {
	var body taskV1IDBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if body.TaskID <= 0 {
		result.Err(ctx, 400, "task_id 无效")
		return
	}
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	var task model.DownloadTaskV1
	if err := c.db.Where("id = ?", body.TaskID).First(&task).Error; err != nil {
		result.Err(ctx, 404, "下载任务不存在")
		return
	}

	if task.Status != model.TaskStatusPreparing && task.Status != model.TaskStatusDownloading {
		result.Err(ctx, 400, "当前状态不允许暂停")
		return
	}

	now := time.Now().UnixMilli()

	// 暂停 Hermes 下载引擎
	c.downloader.Pause(task.Id)

	// 暂停任务
	c.db.Model(&task).Updates(map[string]any{
		"status":     model.TaskStatusPaused,
		"updated_at": now,
	})
	task.Status = model.TaskStatusPaused

	// 暂停正在下载的 resources
	c.db.Model(&model.DownloadResource{}).Where("task_id = ? AND status = 1", task.Id).
		Updates(map[string]any{"status": 1, "updated_at": now})

	// 暂停激活的 segments
	c.db.Model(&model.DownloadSegment{}).Where("resource_id IN (SELECT id FROM download_resource WHERE task_id = ?) AND status = 1", task.Id).
		Updates(map[string]any{"status": 1, "updated_at": now})

	// 暂停 connections
	c.db.Model(&model.DownloadConnection{}).
		Where("endpoint_id IN (SELECT id FROM download_endpoint WHERE resource_id IN (SELECT id FROM download_resource WHERE task_id = ?))", task.Id).
		Updates(map[string]any{"status": 2, "speed": 0, "last_active": now, "updated_at": now})

	c.db.Create(&model.DownloadLog{
		TaskId:    task.Id,
		Level:     "info",
		Message:   "task paused",
		CreatedAt: now,
	})

	c.broadcastDownloadTaskUpsert([]int{task.Id})

	result.Ok(ctx, gin.H{"task": task, "status_text": "paused"})
}

// handleResumeDownloadTaskV1 恢复下载任务
// POST /api/v1/download_task/resume
func (c *APIClient) handleResumeDownloadTaskV1(ctx *gin.Context) {
	var body taskV1IDBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if body.TaskID <= 0 {
		result.Err(ctx, 400, "task_id 无效")
		return
	}
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	var task model.DownloadTaskV1
	if err := c.db.Where("id = ?", body.TaskID).First(&task).Error; err != nil {
		result.Err(ctx, 404, "下载任务不存在")
		return
	}

	if task.Status != model.TaskStatusPaused {
		result.Err(ctx, 400, "当前状态不允许恢复")
		return
	}

	now := time.Now().UnixMilli()

	if err := c.downloader.Start(task.Id); err != nil {
		result.Err(ctx, 500, "恢复下载任务失败: "+err.Error())
		return
	}
	task.Status = model.TaskStatusPreparing

	c.db.Create(&model.DownloadLog{
		TaskId:    task.Id,
		Level:     "info",
		Message:   "task resumed",
		CreatedAt: now,
	})

	c.broadcastDownloadTaskUpsert([]int{task.Id})

	result.Ok(ctx, gin.H{"task": task, "status_text": "preparing"})
}

// handleDeleteDownloadTaskV1 删除下载任务
// POST /api/v1/download_task/delete
func (c *APIClient) handleDeleteDownloadTaskV1(ctx *gin.Context) {
	var body taskV1IDBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if body.TaskID <= 0 {
		result.Err(ctx, 400, "task_id 无效")
		return
	}
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	var task model.DownloadTaskV1
	if err := c.db.Where("id = ?", body.TaskID).First(&task).Error; err != nil {
		result.Err(ctx, 404, "下载任务不存在")
		return
	}

	now := time.Now().UnixMilli()

	// 停止 Hermes 下载引擎任务
	c.downloader.Delete(task.Id)

	// 先标记 task 为取消状态，再软删除
	c.db.Model(&task).Updates(map[string]any{
		"status":     model.TaskStatusCancelled,
		"updated_at": now,
	})
	deletedRecord, _ := c.buildDownloadTaskRecord(task.Id)

	// 软删除 task
	c.db.Model(&task).Update("deleted_at", now)

	// 级联软删除关联数据
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

	c.db.Create(&model.DownloadLog{
		TaskId:    task.Id,
		Level:     "info",
		Message:   "task deleted",
		CreatedAt: now,
	})

	if deletedRecord != nil {
		c.broadcastDownloadTaskDelete([]DownloadTaskRecord{*deletedRecord})
	}

	result.Ok(ctx, gin.H{"task_id": task.Id, "status_text": "cancelled"})
}

// handleListDownloadTaskV1 查询下载任务列表
// GET /api/v1/download_task/list
func (c *APIClient) handleListDownloadTaskV1(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}
	if taskID, err := strconv.Atoi(ctx.Query("task_id")); err == nil && taskID > 0 {
		record, err := c.buildDownloadTaskRecord(taskID)
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

	var tasks []model.DownloadTaskV1
	var total int64

	query := c.db.Model(&model.DownloadTaskV1{}).Where("deleted_at IS NULL")
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
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks).Error; err != nil {
		result.Err(ctx, 500, "查询下载任务失败: "+err.Error())
		return
	}

	list, err := c.buildDownloadTaskRecords(tasks)
	if err != nil {
		result.Err(ctx, 500, "构建下载任务记录失败: "+err.Error())
		return
	}

	result.Ok(ctx, gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
