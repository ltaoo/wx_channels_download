package api

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	gopeedhttp "github.com/GopeedLab/gopeed/pkg/protocol/http"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	apitypes "wx_channel/internal/api/types"
	"wx_channel/internal/database/model"
	result "wx_channel/internal/util"
	utilpkg "wx_channel/pkg/util"
)

func (c *APIClient) handleCompatDownloadTaskList(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var body struct {
		Page     *int `json:"page"`
		PageSize *int `json:"pageSize"`
		Status   *int `json:"status"`
	}
	_ = ctx.ShouldBindJSON(&body)
	page := 1
	size := 20
	if body.Page != nil && *body.Page > 0 {
		page = *body.Page
	}
	if body.PageSize != nil && *body.PageSize > 0 {
		size = *body.PageSize
	}
	offset := (page - 1) * size

	db := c.db.DB().Model(&model.DownloadTask{})
	if body.Status != nil {
		db = db.Where("status = ?", *body.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		result.Err(ctx, 2000, err.Error())
		return
	}
	var tasks []model.DownloadTask
	if err := db.Order("id DESC").Limit(size).Offset(offset).Find(&tasks).Error; err != nil {
		result.Err(ctx, 2000, err.Error())
		return
	}

	ids := make([]int, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.Id)
	}
	var events []model.DownloadTaskEvent
	if len(ids) > 0 {
		_ = c.db.DB().Where("task_id IN ?", ids).Order("id ASC").Find(&events).Error
	}
	evMap := map[int][]model.DownloadTaskEvent{}
	for _, e := range events {
		evMap[e.TaskId] = append(evMap[e.TaskId], e)
	}

	type DownloadTaskEventResp struct {
		Id        int    `json:"id"`
		TaskId    int    `json:"task_id"`
		Type      string `json:"type"`
		Message   string `json:"message"`
		Data      string `json:"data"`
		CreatedAt string `json:"created_at"`
	}
	type DownloadTaskResp struct {
		Id         int    `json:"id"`
		TaskId     string `json:"task_id"`
		Type       int    `json:"type"`
		Status     int    `json:"status"`
		ExternalId string `json:"external_id"`
		Protocol   string `json:"protocol"`
		URL        string `json:"url"`
		Title      string `json:"title"`
		CoverURL   string `json:"cover_url"`
		Size       int64  `json:"size"`
		Progress   string `json:"progress"`
		Filepath   string `json:"filepath"`
		Error      string `json:"error"`
		Reason     string `json:"reason"`
		Metadata1  string `json:"metadata1"`
		Metadata2  string `json:"metadata2"`
		Idx        int    `json:"idx"`
		CreatedAt  string `json:"created_at"`
		UpdatedAt  string `json:"updated_at"`
	}
	type TaskWithEvents struct {
		DownloadTaskResp
		Events []DownloadTaskEventResp `json:"events"`
	}
	resp := make([]TaskWithEvents, 0, len(tasks))
	for _, t := range tasks {
		out := TaskWithEvents{
			DownloadTaskResp: DownloadTaskResp{
				Id:         t.Id,
				TaskId:     t.TaskId,
				Type:       t.Type,
				Status:     t.Status,
				ExternalId: t.ExternalId,
				Protocol:   t.Protocol,
				URL:        t.URL,
				Title:      t.Title,
				CoverURL:   t.CoverURL,
				Size:       t.Size,
				Progress:   t.Progress,
				Filepath:   t.Filepath,
				Error:      t.Error,
				Reason:     t.Reason,
				Metadata1:  t.Metadata1,
				Metadata2:  t.Metadata2,
				Idx:        t.Idx,
				CreatedAt:  strconv.FormatInt(t.CreatedAt, 10),
				UpdatedAt:  strconv.FormatInt(t.UpdatedAt, 10),
			},
		}
		for _, e := range evMap[t.Id] {
			out.Events = append(out.Events, DownloadTaskEventResp{
				Id:        e.Id,
				TaskId:    e.TaskId,
				Type:      e.Type,
				Message:   e.Message,
				Data:      e.Data,
				CreatedAt: strconv.FormatInt(e.CreatedAt, 10),
			})
		}
		resp = append(resp, out)
	}
	result.Ok(ctx, gin.H{
		"list":      resp,
		"page":      page,
		"page_size": size,
		"total":     total,
	})
}

func (c *APIClient) handleCompatDownloadTaskProfile(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var body struct {
		TaskId string `json:"task_id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if strings.TrimSpace(body.TaskId) == "" {
		result.Err(ctx, 400, "缺少 task_id")
		return
	}
	var task model.DownloadTask
	if err := c.db.DB().Where("task_id = ?", body.TaskId).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err(ctx, 404, "未找到下载任务")
			return
		}
		result.Err(ctx, 2000, err.Error())
		return
	}
	result.Ok(ctx, task)
}

func (c *APIClient) handleCompatDownloadTaskCreate(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	if c.downloader == nil {
		result.Err(ctx, 500, "downloader 未初始化")
		return
	}

	var body apitypes.ChannelsObject
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}

	feed := apitypes.ChannelsObjectToChannelsFeedProfile(&body)
	if feed == nil || strings.TrimSpace(feed.ObjectId) == "" || strings.TrimSpace(feed.URL) == "" {
		result.Err(ctx, 400, "不合法的参数")
		return
	}

	if c.channels == nil {
		result.Err(ctx, 500, "channels client 未初始化")
		return
	}
	content, err := c.channels.UpsertChannelsFeed(feed)
	if err != nil || content == nil || content.Id <= 0 {
		if err != nil {
			result.Err(ctx, 2000, "保存内容失败："+err.Error())
			return
		}
		result.Err(ctx, 2000, "保存内容失败")
		return
	}

	// 按 id/spec/suffix 检查是否已存在相同下载内容（与 handleCreateFeedDownloadTask 一致）
	spec := "original"
	suffix := ".mp4"
	if c.check_existing_feed(c.downloader.GetTasks(), &FeedDownloadTaskBody{
		Id:     feed.ObjectId,
		Spec:   spec,
		Suffix: suffix,
	}) {
		result.Err(ctx, 409, "已存在该下载内容")
		return
	}

	var existingTask model.DownloadTask
	if err := c.db.DB().Where("external_id = ?", content.ExternalId).Order("id DESC").First(&existingTask).Error; err == nil && strings.TrimSpace(existingTask.TaskId) != "" {
		now := utilpkg.NowMillis()
		_ = c.db.DB().Model(&model.Content{}).Where("id = ?", content.Id).Updates(map[string]any{
			"download_task_id": existingTask.Id,
			"download_status":  existingTask.Status,
			"download_path":    existingTask.Filepath,
			"updated_at":       now,
		}).Error
		result.Ok(ctx, gin.H{
			"message":          "创建下载任务成功",
			"task_id":          existingTask.TaskId,
			"download_task_id": existingTask.Id,
			"content_id":       content.Id,
		})
		return
	}

	filenameBase := strings.TrimSpace(feed.Title)
	if filenameBase == "" {
		filenameBase = feed.ObjectId
	}
	filename, dir, err := c.formatter.ProcessFilename(filenameBase)
	if err != nil || strings.TrimSpace(filename) == "" {
		filename = filenameBase
		dir = ""
	}
	// 与旧逻辑对齐：suffix 单独控制拼接，避免双重扩展名
	if strings.HasSuffix(strings.ToLower(filename), ".mp4") {
		suffix = ""
	}

	downloadURL := strings.TrimSpace(feed.URL)
	id, err := c.downloader.CreateDirect(
		&base.Request{
			URL: downloadURL,
			Labels: map[string]string{
				"id":       feed.ObjectId,
				"nonce_id": feed.NonceId,
				"title":    feed.Title,
				"key": func() string {
					k := 0
					if strings.TrimSpace(feed.DecryptKey) != "" {
						if v, err := strconv.Atoi(feed.DecryptKey); err == nil {
							k = v
						}
					}
					return strconv.Itoa(k)
				}(),
				"spec":   spec,
				"suffix": ".mp4",
			},
		},
		&base.Options{
			Name: filename + suffix,
			Path: filepath.Join(c.cfg.DownloadDir, dir),
			Extra: &gopeedhttp.OptsExtra{
				Connections: c.resolve_connections(downloadURL),
			},
		},
	)
	if err != nil {
		result.Err(ctx, 2000, err.Error())
		return
	}

	task := c.downloader.GetTask(id)
	if task != nil {
		c.downloader_ws.Broadcast(APIClientWSMessage{
			Type: "event",
			Data: map[string]interface{}{
				"task": task,
			},
		})
	}

	taskRec, err := c.CreateContentDownloadTask(content, task, "api call")
	if err != nil {
		result.Err(ctx, 2000, err.Error())
		return
	}

	result.Ok(ctx, gin.H{
		"message":          "创建下载任务成功",
		"task_id":          id,
		"download_task_id": taskRec.Id,
		"content_id":       content.Id,
	})
}

func (c *APIClient) handleCompatDownloadTaskBatchCreate(ctx *gin.Context) {
	var body struct {
		URLs []string `json:"urls"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if len(body.URLs) == 0 {
		result.Err(ctx, 400, "缺少 urls")
		return
	}
	batch := base.CreateTaskBatch{}
	for _, raw := range body.URLs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		filename := strings.TrimSpace(u.Query().Get("filename"))
		if filename == "" {
			filename = filepath.Base(u.Path)
		}
		if filename == "" || filename == "." || filename == "/" {
			filename = strconv.FormatInt(time.Now().UnixNano(), 10)
		}
		name, dir, err := c.formatter.ProcessFilename(filename)
		if err != nil {
			name = filename
			dir = ""
		}
		batch.Reqs = append(batch.Reqs, &base.CreateTaskBatchItem{
			Req: &base.Request{URL: raw},
			Opts: &base.Options{
				Path: filepath.Join(c.cfg.DownloadDir, dir),
				Name: name,
				Extra: &gopeedhttp.OptsExtra{
					Connections: c.resolve_connections(raw),
				},
			},
		})
	}
	if len(batch.Reqs) == 0 {
		result.Ok(ctx, gin.H{"ids": []string{}})
		return
	}
	ids, err := c.downloader.CreateDirectBatch(&batch)
	if err != nil {
		result.Err(ctx, 2000, err.Error())
		return
	}
	var batchTasks []interface{}
	for _, id := range ids {
		task := c.downloader.GetTask(id)
		if task != nil {
			batchTasks = append(batchTasks, task)
		}
	}
	if len(batchTasks) > 0 {
		c.downloader_ws.Broadcast(APIClientWSMessage{
			Type: "batch_tasks",
			Data: batchTasks,
		})
	}
	result.Ok(ctx, gin.H{"ids": ids})
}

func (c *APIClient) handleCompatDownloadTaskStart(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var body struct {
		TaskId int `json:"download_task_id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if body.TaskId == 0 {
		result.Err(ctx, 400, "缺少 download_task_id")
		return
	}
	var rec model.DownloadTask
	if err := c.db.DB().First(&rec, "id = ?", body.TaskId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err(ctx, 404, "未找到下载任务")
			return
		}
		result.Err(ctx, 2000, err.Error())
		return
	}

	dTask := c.downloader.GetTask(rec.TaskId)
	if dTask != nil {
		if err := c.downloader.Continue(&downloadpkg.TaskFilter{IDs: []string{rec.TaskId}}); err != nil {
			result.Err(ctx, 500, "恢复下载任务失败: "+err.Error())
			return
		}
		result.Ok(ctx, gin.H{"message": "开始下载"})
		return
	}

	name := strings.TrimSpace(rec.Title)
	if name == "" {
		name = "download_" + strconv.FormatInt(time.Now().Unix(), 10) + ".mp4"
	}
	id, err := c.downloader.CreateDirect(
		&base.Request{URL: rec.URL},
		&base.Options{
			Name: name,
			Path: c.cfg.DownloadDir,
			Extra: &gopeedhttp.OptsExtra{
				Connections: c.resolve_connections(rec.URL),
			},
		},
	)
	if err != nil {
		result.Err(ctx, 500, "重新提交下载任务失败: "+err.Error())
		return
	}
	_ = c.db.DB().Model(&model.DownloadTask{}).Where("id = ?", rec.Id).Updates(map[string]any{
		"task_id": id,
		"status":  2,
	}).Error
	result.Ok(ctx, gin.H{"message": "开始下载"})
}

func (c *APIClient) handleCompatDownloadTaskRetry(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var body struct {
		Id int `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if body.Id == 0 {
		result.Err(ctx, 400, "缺少 id")
		return
	}
	var rec model.DownloadTask
	if err := c.db.DB().First(&rec, "id = ?", body.Id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err(ctx, 404, "未找到下载任务")
			return
		}
		result.Err(ctx, 2000, err.Error())
		return
	}

	newURL := strings.TrimSpace(rec.URL)
	if strings.TrimSpace(rec.Metadata2) != "" && c.channels != nil {
		var meta struct {
			ExternalId string `json:"external_id"`
			NonceId    string `json:"nonce_id"`
			EID        string `json:"eid"`
		}
		if err := json.Unmarshal([]byte(rec.Metadata2), &meta); err == nil && meta.ExternalId != "" {
			profile, err := c.channels.FetchChannelsFeedProfile(meta.ExternalId, meta.NonceId, "", meta.EID)
			if err == nil && profile != nil && profile.ErrCode == 0 && len(profile.Data.Object.ObjectDesc.Media) > 0 {
				m := profile.Data.Object.ObjectDesc.Media[0]
				u := strings.TrimSpace(m.URL + m.URLToken)
				if u != "" {
					newURL = u
				}
			}
		}
	}

	if rec.TaskId != "" {
		_ = c.downloader.Delete(&downloadpkg.TaskFilter{IDs: []string{rec.TaskId}}, true)
	}

	name := strings.TrimSpace(rec.Title)
	if name == "" {
		name = "download_" + strconv.FormatInt(time.Now().Unix(), 10) + ".mp4"
	}
	newTaskId, err := c.downloader.CreateDirect(
		&base.Request{URL: newURL},
		&base.Options{
			Name: name,
			Path: c.cfg.DownloadDir,
			Extra: &gopeedhttp.OptsExtra{
				Connections: c.resolve_connections(newURL),
			},
		},
	)
	if err != nil {
		result.Err(ctx, 500, "提交新下载任务失败: "+err.Error())
		return
	}

	_ = c.db.DB().Model(&model.DownloadTask{}).Where("id = ?", rec.Id).Updates(map[string]any{
		"url":     newURL,
		"task_id": newTaskId,
		"status":  2,
	}).Error
	result.Ok(ctx, gin.H{"message": "已重试下载", "new_task_id": newTaskId})
}

func (c *APIClient) handleCompatDownloadTaskDelete(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var body struct {
		TaskId int `json:"download_task_id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if body.TaskId == 0 {
		result.Err(ctx, 400, "缺少 download_task_id")
		return
	}
	var task model.DownloadTask
	if err := c.db.DB().First(&task, "id = ?", body.TaskId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err(ctx, 404, "未找到下载任务")
			return
		}
		result.Err(ctx, 2000, err.Error())
		return
	}
	if task.TaskId != "" {
		_ = c.downloader.Delete(&downloadpkg.TaskFilter{IDs: []string{task.TaskId}}, true)
	}
	_ = c.db.DB().Delete(&model.Video{}, "platform_id = ? AND download_task_id = ?", "wx_channels", body.TaskId).Error
	_ = c.db.DB().Delete(&model.DownloadTask{}, "id = ?", body.TaskId).Error
	result.Ok(ctx, gin.H{"message": "删除下载任务成功"})
}

func (c *APIClient) handleCompatDownloadTaskPauseAll(ctx *gin.Context) {
	err := c.downloader.Pause(&downloadpkg.TaskFilter{
		Statuses: []base.Status{
			base.DownloadStatusRunning,
			base.DownloadStatusWait,
			base.DownloadStatusReady,
		},
	})
	if err != nil {
		result.Err(ctx, 500, "暂停全部任务失败: "+err.Error())
		return
	}
	result.Ok(ctx, gin.H{"message": "已暂停全部任务"})
}

func (c *APIClient) handleCompatDownloadTaskStartAll(ctx *gin.Context) {
	err := c.downloader.Continue(&downloadpkg.TaskFilter{
		Statuses: []base.Status{
			base.DownloadStatusPause,
		},
	})
	if err != nil {
		result.Err(ctx, 500, "开始全部任务失败: "+err.Error())
		return
	}
	result.Ok(ctx, gin.H{"message": "已开始全部任务"})
}

func (c *APIClient) handleCompatDownloadTaskHighlightFile(ctx *gin.Context) {
	var body struct {
		FilePath string `json:"file_path"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if strings.TrimSpace(body.FilePath) == "" {
		result.Err(ctx, 500, "Missing the `file_path`")
		return
	}
	fullPath := filepath.Join(c.cfg.DownloadDir, body.FilePath)
	if _, err := os.Stat(fullPath); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", "/select,", fullPath)
	case "darwin":
		cmd = exec.Command("open", "-R", fullPath)
	case "linux":
		cmd = exec.Command("xdg-open", fullPath)
	default:
		result.Err(ctx, 500, "Unsupported operating system")
		return
	}
	if err := cmd.Start(); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, "Success")
}

func (c *APIClient) handleCompatDownloadTaskPlay(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	idStr := ctx.Query("id")
	if strings.TrimSpace(idStr) == "" {
		result.Err(ctx, 400, "缺少 id")
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		result.Err(ctx, 400, "id 不合法")
		return
	}
	var task model.DownloadTask
	if err := c.db.DB().First(&task, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err(ctx, 404, "未找到下载任务")
			return
		}
		result.Err(ctx, 2000, err.Error())
		return
	}
	filePath := strings.TrimSpace(task.Filepath)
	if filePath == "" {
		filePath = strings.TrimSpace(task.Title)
	}
	if filePath == "" {
		result.Err(ctx, 404, "文件不存在")
		return
	}
	fullPath := filepath.Join(c.cfg.DownloadDir, filePath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		if filepath.Ext(fullPath) == "" {
			fullPath = fullPath + ".mp4"
			if _, err2 := os.Stat(fullPath); os.IsNotExist(err2) {
				result.Err(ctx, 404, "文件不存在: "+fullPath)
				return
			}
		} else {
			result.Err(ctx, 404, "文件不存在: "+fullPath)
			return
		}
	}
	ctx.File(fullPath)
}

func (c *APIClient) handleCompatChannelsTaskStatus(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	taskId := ctx.Query("task_id")
	if strings.TrimSpace(taskId) == "" {
		result.Err(ctx, 1000, "缺少 task_id")
		return
	}
	var task model.DownloadTask
	if err := c.db.DB().First(&task, "task_id = ?", taskId).Error; err != nil {
		result.Err(ctx, 2000, err.Error())
		return
	}
	result.Ok(ctx, gin.H{
		"task_id": task.TaskId,
		"status":  task.Status,
		"error":   task.Error,
		"title":   task.Title,
	})
}

func (c *APIClient) handleCompatChannelsTaskStart(ctx *gin.Context) {
	taskId := ctx.Query("task_id")
	if strings.TrimSpace(taskId) == "" {
		result.Ok(ctx, nil)
		return
	}
	_ = c.downloader.Continue(&downloadpkg.TaskFilter{IDs: []string{taskId}})
	result.Ok(ctx, gin.H{"task_id": taskId})
}

func (c *APIClient) handleCompatAccountSynchronize(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var body struct {
		AccountId *int    `json:"account_id"`
		Username  *string `json:"username"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	username := ""
	if body.Username != nil && strings.TrimSpace(*body.Username) != "" {
		username = strings.TrimSpace(*body.Username)
	}
	if username == "" && body.AccountId != nil && *body.AccountId > 0 {
		var acc model.Account
		if err := c.db.DB().First(&acc, *body.AccountId).Error; err != nil {
			result.Err(ctx, 404, err.Error())
			return
		}
		username = acc.ExternalId
	}
	if username == "" {
		result.Err(ctx, 400, "account_id or username is required")
		return
	}
	resp, err := c.channels.FetchChannelsFeedListOfContact(username, "")
	if err != nil {
		result.Err(ctx, 998, err.Error())
		return
	}
	if resp == nil || resp.ErrCode != 0 {
		result.Err(ctx, 998, "wechat channels internal request failed")
		return
	}

	now := utilpkg.NowMillis()
	accId := 0
	{
		var acc model.Account
		err := c.db.DB().Where("platform_id = ? AND external_id = ?", "wx_channels", username).First(&acc).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err(ctx, 500, err.Error())
			return
		}
		nickname := resp.Data.Contact.Nickname
		avatar := resp.Data.Contact.HeadUrl
		if acc.Id == 0 {
			acc = model.Account{
				PlatformId:   "wx_channels",
				ExternalId:   username,
				Username:     username,
				Nickname:     nickname,
				AvatarURL:    avatar,
				Timestamps:   model.Timestamps{CreatedAt: now, UpdatedAt: now},
				InfluencerId: nil,
			}
			if err := c.db.DB().Create(&acc).Error; err != nil {
				result.Err(ctx, 500, err.Error())
				return
			}
		} else {
			_ = c.db.DB().Model(&model.Account{}).Where("id = ?", acc.Id).Updates(map[string]any{
				"nickname":   nickname,
				"avatar_url": avatar,
				"updated_at": now,
			}).Error
			_ = c.db.DB().First(&acc, acc.Id).Error
		}
		accId = acc.Id
	}

	added := 0
	updated := 0
	for _, obj := range resp.Data.Object {
		mediaURL := ""
		coverURL := ""
		decodeKey := ""
		size := int64(0)
		duration := int64(0)
		if len(obj.ObjectDesc.Media) > 0 {
			m := obj.ObjectDesc.Media[0]
			mediaURL = strings.TrimSpace(m.URL + m.URLToken)
			coverURL = m.CoverUrl
			decodeKey = m.DecodeKey
			size = int64(m.FileSize)
			if len(m.Spec) > 0 {
				duration = int64(m.Spec[0].DurationMs / 1000)
			}
		}
		v := model.Video{
			PlatformId:  "wx_channels",
			Title:       obj.ObjectDesc.Description,
			Description: obj.ObjectDesc.Description,
			ExternalId1: obj.ID,
			ExternalId2: obj.ObjectNonceId,
			ExternalId3: decodeKey,
			URL:         mediaURL,
			SourceURL:   obj.SourceURL,
			CoverURL:    coverURL,
			Size:        size,
			Duration:    duration,
			PublishTime: int64(obj.CreateTime),
			Timestamps:  model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}
		var existing model.Video
		err := c.db.DB().Where("platform_id = ? AND external_id1 = ?", "wx_channels", obj.ID).First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if existing.Id == 0 {
			if err := c.db.DB().Create(&v).Error; err == nil {
				added++
				existing = v
			}
		} else {
			updates := map[string]any{
				"title":        v.Title,
				"description":  v.Description,
				"external_id2": v.ExternalId2,
				"external_id3": v.ExternalId3,
				"url":          v.URL,
				"source_url":   v.SourceURL,
				"cover_url":    v.CoverURL,
				"size":         v.Size,
				"duration":     v.Duration,
				"publish_time": v.PublishTime,
				"updated_at":   now,
			}
			if err := c.db.DB().Model(&model.Video{}).Where("id = ?", existing.Id).Updates(updates).Error; err == nil {
				updated++
			}
		}
		if accId > 0 && existing.Id > 0 {
			link := model.VideoAccount{VideoId: existing.Id, AccountId: accId, Role: "owner"}
			_ = c.db.DB().Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error
		}
	}
	result.Ok(ctx, gin.H{
		"account_id": accId,
		"diff": gin.H{
			"added":   added,
			"updated": updated,
		},
		"status": "synchronized",
	})
}

func (c *APIClient) upsertAccountAndVideoFromChannelsObject(obj *apitypes.ChannelsObject, downloadTaskId *int) {
	if obj == nil || c.db == nil || c.db.DB() == nil {
		return
	}
	now := utilpkg.NowMillis()
	accountExternal := strings.TrimSpace(obj.Contact.Username)
	accountId := 0
	if accountExternal != "" {
		var acc model.Account
		err := c.db.DB().Where("platform_id = ? AND external_id = ?", "wx_channels", accountExternal).First(&acc).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}
		if acc.Id == 0 {
			acc = model.Account{
				PlatformId: "wx_channels",
				ExternalId: accountExternal,
				Username:   obj.Contact.Username,
				Nickname:   obj.Contact.Nickname,
				AvatarURL:  obj.Contact.HeadUrl,
				Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
			}
			if err := c.db.DB().Create(&acc).Error; err != nil {
				return
			}
		} else {
			_ = c.db.DB().Model(&model.Account{}).Where("id = ?", acc.Id).Updates(map[string]any{
				"username":    obj.Contact.Username,
				"nickname":    obj.Contact.Nickname,
				"avatar_url":  obj.Contact.HeadUrl,
				"updated_at":  now,
				"platform_id": "wx_channels",
			}).Error
		}
		accountId = acc.Id
	}

	mediaURL := ""
	coverURL := ""
	decodeKey := ""
	size := int64(0)
	duration := int64(0)
	if len(obj.ObjectDesc.Media) > 0 {
		m := obj.ObjectDesc.Media[0]
		mediaURL = strings.TrimSpace(m.URL + m.URLToken)
		coverURL = m.CoverUrl
		decodeKey = m.DecodeKey
		size = int64(m.FileSize)
		if len(m.Spec) > 0 {
			duration = int64(m.Spec[0].DurationMs / 1000)
		}
	}

	if strings.TrimSpace(obj.ID) == "" {
		return
	}

	var v model.Video
	err := c.db.DB().Where("platform_id = ? AND external_id1 = ?", "wx_channels", obj.ID).First(&v).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	if v.Id == 0 {
		v = model.Video{
			PlatformId:     "wx_channels",
			DownloadTaskId: downloadTaskId,
			Title:          obj.ObjectDesc.Description,
			Description:    obj.ObjectDesc.Description,
			ExternalId1:    obj.ID,
			ExternalId2:    obj.ObjectNonceId,
			ExternalId3:    decodeKey,
			Metadata:       "",
			URL:            mediaURL,
			SourceURL:      obj.SourceURL,
			CoverURL:       coverURL,
			Size:           size,
			Duration:       duration,
			PublishTime:    int64(obj.CreateTime),
			Timestamps:     model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}
		if err := c.db.DB().Create(&v).Error; err != nil {
			return
		}
	} else {
		updates := map[string]any{
			"title":        obj.ObjectDesc.Description,
			"description":  obj.ObjectDesc.Description,
			"external_id2": obj.ObjectNonceId,
			"external_id3": decodeKey,
			"url":          mediaURL,
			"source_url":   obj.SourceURL,
			"cover_url":    coverURL,
			"size":         size,
			"duration":     duration,
			"publish_time": int64(obj.CreateTime),
			"updated_at":   now,
		}
		if downloadTaskId != nil {
			updates["download_task_id"] = downloadTaskId
		}
		_ = c.db.DB().Model(&model.Video{}).Where("id = ?", v.Id).Updates(updates).Error
	}

	if accountId > 0 && v.Id > 0 {
		link := model.VideoAccount{VideoId: v.Id, AccountId: accountId, Role: "owner"}
		_ = c.db.DB().Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error
	}
}
