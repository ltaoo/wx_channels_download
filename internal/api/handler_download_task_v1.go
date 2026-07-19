package api

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

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

	now := time.Now().UnixMilli()

	// 更新任务状态到 Downloading
	if err := c.db.Model(&task).Updates(map[string]any{
		"status":     model.TaskStatusDownloading,
		"updated_at": now,
	}).Error; err != nil {
		result.Err(ctx, 500, "更新任务状态失败: "+err.Error())
		return
	}
	task.Status = model.TaskStatusDownloading

	// 激活 resources
	c.db.Model(&model.DownloadResource{}).Where("task_id = ? AND status = 0", task.Id).
		Updates(map[string]any{"status": 1, "updated_at": now})

	// 为每个 resource 创建或激活 segment
	var resources []model.DownloadResource
	c.db.Where("task_id = ?", task.Id).Find(&resources)
	for _, r := range resources {
		var count int64
		c.db.Model(&model.DownloadSegment{}).Where("resource_id = ?", r.Id).Count(&count)
		if count == 0 && r.Size > 0 {
			// 初始化分片
			c.db.Create(&model.DownloadSegment{
				ResourceId:  r.Id,
				Index:       0,
				URL:         "",
				OffsetStart: 0,
				OffsetEnd:   r.Size - 1,
				Size:        r.Size,
				Downloaded:  0,
				Status:      1,
			})
		} else {
			c.db.Model(&model.DownloadSegment{}).Where("resource_id = ? AND status IN (0,1)", r.Id).
				Updates(map[string]any{"status": 1, "updated_at": now})
		}
	}

	// 激活 endpoints 和 connections
	var endpoints []model.DownloadEndpoint
	c.db.Where("resource_id IN (SELECT id FROM download_resource WHERE task_id = ?)", task.Id).Find(&endpoints)
	for _, ep := range endpoints {
		c.db.Model(&model.DownloadEndpoint{}).Where("id = ?", ep.Id).
			Updates(map[string]any{"status": 1, "updated_at": now})

		var connCount int64
		c.db.Model(&model.DownloadConnection{}).Where("endpoint_id = ?", ep.Id).Count(&connCount)
		if connCount == 0 {
			c.db.Create(&model.DownloadConnection{
				EndpointId: ep.Id,
				WorkerId:   "worker-" + strconv.Itoa(ep.Id),
				Status:     1,
			})
		} else {
			c.db.Model(&model.DownloadConnection{}).Where("endpoint_id = ?", ep.Id).
				Updates(map[string]any{"status": 1, "last_active": now, "updated_at": now})
		}
	}

	c.db.Create(&model.DownloadLog{
		TaskId:    task.Id,
		Level:     "info",
		Message:   "task started",
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

	// 恢复下载
	c.db.Model(&task).Updates(map[string]any{
		"status":     model.TaskStatusDownloading,
		"updated_at": now,
	})
	task.Status = model.TaskStatusDownloading

	// 恢复 resources
	c.db.Model(&model.DownloadResource{}).Where("task_id = ? AND status = 1", task.Id).
		Updates(map[string]any{"status": 1, "updated_at": now})

	// 恢复 segments
	c.db.Model(&model.DownloadSegment{}).Where("resource_id IN (SELECT id FROM download_resource WHERE task_id = ?) AND status = 1", task.Id).
		Updates(map[string]any{"status": 1, "updated_at": now})

	// 恢复 connections
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
		if len(parts) == 1 {
			query = query.Where("status = ?", strings.TrimSpace(parts[0]))
		} else {
			trimmed := make([]string, 0, len(parts))
			for _, p := range parts {
				trimmed = append(trimmed, strings.TrimSpace(p))
			}
			query = query.Where("status IN ?", trimmed)
		}
	}
	query.Count(&total)
	query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks)

	result.Ok(ctx, gin.H{
		"list":      tasks,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// broadcastTaskProgress 推送任务进度到 WebSocket 客户端
func (c *APIClient) broadcastTaskProgress(taskID int) {
	snapshot := c.buildTaskProgressSnapshot(taskID)
	if snapshot != nil {
		v1TaskHub.BroadcastProgress(taskID, snapshot)
	}
}
