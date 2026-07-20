package api

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"fmt"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/database/model"
	"wx_channel/internal/webcontent/registry"
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
	Overwrite     bool   `json:"overwrite"`
	SkipDuplicate bool   `json:"skip_duplicate"`
}

// taskV1IDBody 通用 task_id 请求体
type taskV1IDBody struct {
	TaskID int `json:"task_id"`
}

// CreateDownloadTaskByURLBody 通过资源地址创建下载任务请求体
type CreateDownloadTaskByURLBody struct {
	URL      string         `json:"url"`      // 资源下载地址，必填
	SavePath string         `json:"save_path"` // 保存路径
	Filename string         `json:"filename"`  // 文件名（可选，默认从URL提取）
	Config   DownloadConfig `json:"config"`    // 下载配置
}

func (c *APIClient) handleCreateDownloadTaskV1(ctx *gin.Context) {
	var body CreateDownloadTaskV1Body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}

	if body.Platform == "" {
		result.Err(ctx, 400, "platform 不能为空")
		return
	}

	// 根据平台获取对应的处理器
	h := registry.Get(body.Platform)
	if h == nil {
		result.Err(ctx, 400, "不支持的平台: "+body.Platform)
		return
	}

	// 调用平台处理器构建下载模型
	info, content, account, err := h.BuildDownloadTask(body.Content, registry.DownloadConfig{
		SavePath:      body.Config.SavePath,
		Filename:      body.Config.Filename,
		Spec:          body.Config.Spec,
		Overwrite:     body.Config.Overwrite,
		SkipDuplicate: body.Config.SkipDuplicate,
	})
	if err != nil {
		result.Err(ctx, 400, "构建下载任务失败: "+err.Error())
		return
	}

	// 数据库未初始化
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	// 写入数据库
	if err := c.db.Create(&info.Task).Error; err != nil {
		result.Err(ctx, 500, "创建下载任务失败: "+err.Error())
		return
	}

	info.Resource.TaskId = info.Task.Id
	if err := c.db.Create(&info.Resource).Error; err != nil {
		result.Err(ctx, 500, "创建资源失败: "+err.Error())
		return
	}

	info.Endpoint.ResourceId = info.Resource.Id
	if err := c.db.Create(&info.Endpoint).Error; err != nil {
		result.Err(ctx, 500, "创建端点失败: "+err.Error())
		return
	}

	result.Ok(ctx, gin.H{
		"task":     info.Task,
		"resource": info.Resource,
		"endpoint": info.Endpoint,
		"content":  content,
		"account":  account,
	})
}

// handleCreateDownloadTaskByURLV1 通过资源地址创建下载任务
// POST /api/v1/download_task/create_by_url
func (c *APIClient) handleCreateDownloadTaskByURLV1(ctx *gin.Context) {
	var body CreateDownloadTaskByURLBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}

	if body.URL == "" {
		result.Err(ctx, 400, "url 不能为空")
		return
	}

	parsedURL, err := url.Parse(body.URL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		result.Err(ctx, 400, "无效的下载地址")
		return
	}

	protocol := strings.ToUpper(parsedURL.Scheme)

	savePath := body.SavePath
	if savePath == "" {
		savePath = body.Config.SavePath
	}
	if savePath == "" && c.cfg != nil {
		savePath = c.cfg.DownloadDir
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

	taskName := filename

	// 存储原始下载地址到 config_json
	configJSON, _ := json.Marshal(map[string]string{
		"url": body.URL,
	})

	// 数据库未初始化
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
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
		result.Err(ctx, 500, "创建下载任务失败: "+err.Error())
		return
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
		result.Err(ctx, 500, "创建资源失败: "+err.Error())
		return
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
		result.Err(ctx, 500, "创建端点失败: "+err.Error())
		return
	}

	// 创建连接记录（供速度更新使用）
	conn := model.DownloadConnection{
		EndpointId: endpoint.Id,
		WorkerId:   "worker-" + strconv.Itoa(endpoint.Id),
		Host:       parsedURL.Host,
		Status:     1,
		LastActive: now,
	}
	conn.CreatedAt = now
	conn.UpdatedAt = now
	c.db.Create(&conn)

	// 启动原生下载（阻塞直到获取并发槽位，确保下载已开始）
	c.v1Nat.Start(task.Id)

	// 标记任务为下载中
	c.db.Model(&task).Updates(map[string]any{
		"status": model.TaskStatusDownloading, "updated_at": time.Now().UnixMilli(),
	})
	task.Status = model.TaskStatusDownloading

	c.broadcastTaskProgress(task.Id)

	result.Ok(ctx, gin.H{
		"task":     task,
		"resource": resource,
		"endpoint": endpoint,
	})
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

	// 启动原生下载（阻塞直到获取并发槽位，确保下载已开始）
	c.v1Nat.Start(task.Id)

	now := time.Now().UnixMilli()

	// 标记为下载中，前端立即看到状态变化；下载引擎内部会激活资源
	c.db.Model(&task).Updates(map[string]any{
		"status":     model.TaskStatusDownloading,
		"updated_at": now,
	})
	task.Status = model.TaskStatusDownloading

	// 激活连接
	c.db.Model(&model.DownloadConnection{}).
		Where("endpoint_id IN (SELECT id FROM download_endpoint WHERE resource_id IN (SELECT id FROM download_resource WHERE task_id = ?))", task.Id).
		Updates(map[string]any{"status": 1, "last_active": now, "updated_at": now})

	c.db.Create(&model.DownloadLog{
		TaskId:  task.Id,
		Level:   "info",
		Message: "task started",
		CreatedAt: now,
	})

	c.broadcastTaskProgress(task.Id)

	result.Ok(ctx, gin.H{"task": task, "status_text": "downloading"})
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

	if task.Status != model.TaskStatusDownloading {
		result.Err(ctx, 400, "当前状态不允许暂停")
		return
	}

	now := time.Now().UnixMilli()

	// 暂停原生下载引擎
	c.v1Nat.Pause(task.Id)

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

	c.broadcastTaskProgress(task.Id)

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

	// 恢复原生下载引擎（阻塞直到获取并发槽位，确保下载已开始）
	c.v1Nat.Start(task.Id)

	// 标记为下载中，前端立即看到状态变化
	c.db.Model(&task).Updates(map[string]any{
		"status":     model.TaskStatusDownloading,
		"updated_at": now,
	})
	task.Status = model.TaskStatusDownloading

	// 激活连接状态
	c.db.Model(&model.DownloadConnection{}).
		Where("endpoint_id IN (SELECT id FROM download_endpoint WHERE resource_id IN (SELECT id FROM download_resource WHERE task_id = ?))", task.Id).
		Updates(map[string]any{"status": 1, "last_active": now, "updated_at": now})

	c.db.Create(&model.DownloadLog{
		TaskId:    task.Id,
		Level:     "info",
		Message:   "task resumed",
		CreatedAt: now,
	})

	c.broadcastTaskProgress(task.Id)

	result.Ok(ctx, gin.H{"task": task, "status_text": "downloading"})
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

	// 停止原生下载引擎任务
	c.v1Nat.Delete(task.Id)

	// 先标记 task 为取消状态，再软删除
	c.db.Model(&task).Updates(map[string]any{
		"status":     model.TaskStatusCancelled,
		"updated_at": now,
	})

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

	c.broadcastTaskProgress(task.Id)

	result.Ok(ctx, gin.H{"task_id": task.Id, "status_text": "cancelled"})
}

// taskListItem 列表项，包含任务及其关联的资源和端点信息
type taskListItem struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	ResourceType string `json:"resource_type"`
	Status       int    `json:"status"`
	SavePath     string `json:"save_path"`
	ConfigJSON   string `json:"config_json"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	// 关联信息
	URL      string `json:"url"`
	Size     int64  `json:"size"`
	Speed    int64  `json:"speed"`
	Progress int    `json:"progress"`
}

// handleListDownloadTaskV1 查询下载任务列表
// GET /api/v1/download_task/list
func (c *APIClient) handleListDownloadTaskV1(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
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
	query.Count(&total)
	query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks)

	// 收集 task IDs
	taskIDs := make([]int, len(tasks))
	for i, t := range tasks {
		taskIDs[i] = t.Id
	}

	// 查询关联的 resources 和 endpoints
	type resInfo struct {
		TaskId int
		Size   int64
	}
	type epInfo struct {
		ResourceId int
		URL        string
	}
	var resources []resInfo
	var endpoints []epInfo

	if len(taskIDs) > 0 {
		c.db.Table("download_resource").Select("task_id, size").Where("task_id IN ? AND deleted_at IS NULL", taskIDs).Scan(&resources)
		var resIDs []int
		for _, r := range resources {
			resIDs = append(resIDs, r.TaskId)
		}
		if len(resIDs) > 0 {
			c.db.Table("download_endpoint").Select("resource_id, url").Where("resource_id IN (SELECT id FROM download_resource WHERE task_id IN ? AND deleted_at IS NULL)", taskIDs).Scan(&endpoints)
		}
	}

	// 按资源聚合 URL
	urlByTask := map[int]string{}
	sizeByTask := map[int]int64{}
	for _, res := range resources {
		if _, ok := sizeByTask[res.TaskId]; !ok {
			sizeByTask[res.TaskId] = res.Size
		}
	}
	// 按 task_id 聚合 endpoint URL（取第一个）
	for _, ep := range endpoints {
		if _, ok := urlByTask[ep.ResourceId]; !ok {
			urlByTask[ep.ResourceId] = ep.URL
		}
	}

	// 查询连接获取速度
	type connInfo struct {
		ResourceId int
		Speed      int64
	}
	var speeds []connInfo
	if len(taskIDs) > 0 {
		c.db.Table("download_connection").Select("endpoint_id, speed").
			Where("endpoint_id IN (SELECT id FROM download_endpoint WHERE resource_id IN (SELECT id FROM download_resource WHERE task_id IN ? AND deleted_at IS NULL))", taskIDs).
			Scan(&speeds)
	}

	// 构建响应列表
	list := make([]taskListItem, len(tasks))
	for i, t := range tasks {
		list[i] = taskListItem{
			ID:           t.Id,
			Name:         t.Name,
			ResourceType: t.ResourceType,
			Status:       t.Status,
			SavePath:     t.SavePath,
			ConfigJSON:   t.ConfigJSON,
			CreatedAt:    t.CreatedAt,
			UpdatedAt:    t.UpdatedAt,
			URL:          urlByTask[t.Id],
			Size:         sizeByTask[t.Id],
			Speed:        0,
			Progress:     0,
		}
	}

	result.Ok(ctx, gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// broadcastTaskProgress 推送任务进度到 WebSocket 客户端
func (c *APIClient) broadcastTaskProgress(taskID int) {
	snapshot := c.buildTaskProgressSnapshot(taskID)
	fmt.Println("after buildTaskProgressSnapshot", snapshot)
	if snapshot != nil {
		v1TaskHub.BroadcastProgress(taskID, snapshot)
	}
}
