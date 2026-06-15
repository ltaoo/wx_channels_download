package api

import (
	"encoding/json"
	"errors"
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

	"wx_channel/internal/api/services"
	"wx_channel/internal/database/model"
	result "wx_channel/internal/util"
	utilpkg "wx_channel/pkg/util"
	channels "wx_channel/pkg/wxchannels"
)

type compatDownloadTaskID struct {
	ID     int
	TaskID string
}

func (id *compatDownloadTaskID) UnmarshalJSON(data []byte) error {
	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		id.ID = n
		id.TaskID = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		id.ID = 0
		id.TaskID = ""
		return nil
	}
	if parsed, err := strconv.Atoi(s); err == nil {
		id.ID = parsed
		id.TaskID = ""
		return nil
	}
	id.ID = 0
	id.TaskID = s
	return nil
}

func (id compatDownloadTaskID) Empty() bool {
	return id.ID == 0 && strings.TrimSpace(id.TaskID) == ""
}

func (id compatDownloadTaskID) Find(db *gorm.DB, out *model.DownloadTask) error {
	if id.ID > 0 {
		return db.First(out, "id = ?", id.ID).Error
	}
	return db.First(out, "task_id = ?", id.TaskID).Error
}

func downloadTaskFullPath(downloadDir, filePath string) string {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return ""
	}
	if filepath.IsAbs(filePath) {
		return filepath.Clean(filePath)
	}
	return filepath.Join(downloadDir, filePath)
}

func (c *APIClient) handleCompatDownloadTaskCreate(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	if c.downloader == nil {
		result.Err(ctx, 500, "downloader 未初始化")
		return
	}

	var body channels.ChannelsObject
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}

	feed, err := channels.ChannelsObjectToChannelsFeedProfile(&body)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if strings.TrimSpace(feed.ObjectId) == "" || strings.TrimSpace(feed.URL) == "" {
		result.Err(ctx, 400, "不合法的参数：缺少 objectId 或 url")
		return
	}

	spec := "original"
	if len(feed.Spec) > 0 {
		spec = feed.Spec[0].FileFormat
	}

	suffix := ".mp4"

	if strings.TrimSpace(body.Type) == "picture" || body.ObjectDesc.MediaType == 2 {
		if len(body.Files) > 0 {
			files := make([]map[string]string, len(body.Files))
			for i, f := range body.Files {
				files[i] = map[string]string{
					"url":      f.URL,
					"filename": strconv.Itoa(i+1) + ".jpg",
				}
			}
			filesJSON, _ := json.Marshal(files)
			feed.URL = "zip://weixin.qq.com?files=" + string(filesJSON)
			suffix = ".zip"
		}
	}

	downloadURL := strings.TrimSpace(feed.URL)
	if suffix != ".zip" && !strings.Contains(downloadURL, "zip://") && spec != "original" {
		downloadURL = downloadURL + "&X-snsvideoflag=" + spec
	}

	// feed.Title 已在 ChannelsObjectToChannelsFeedProfile 中保证不为空
	filenameBase := strings.TrimSpace(feed.Title)
	filename, dir, err := c.formatter.ProcessFilename(filenameBase)
	if err != nil || strings.TrimSpace(filename) == "" {
		filename = filenameBase
		dir = ""
	}
	if strings.HasSuffix(strings.ToLower(filename), ".mp4") {
		suffix = ""
	}

	sourceURL := strings.TrimSpace(feed.SourceURL)
	if sourceURL == "" {
		sourceURL = channels.BuildJumpUrl(feed)
	}

	key := 0
	if strings.TrimSpace(feed.DecryptKey) != "" {
		if v, err := strconv.Atoi(feed.DecryptKey); err == nil {
			key = v
		}
	}

	tasks := c.downloader.GetTasks()
	existing := c.check_existing_feed(tasks, &services.FeedDownloadTaskBody{
		Id:     feed.ObjectId,
		Spec:   spec,
		Suffix: suffix,
	})
	if existing {
		result.Err(ctx, 409, "已存在该下载内容")
		return
	}

	taskId, err := c.downloader.CreateDirect(
		&base.Request{
			URL: downloadURL,
			Labels: map[string]string{
				"id":         feed.ObjectId,
				"nonce_id":   feed.NonceId,
				"title":      feed.Title,
				"key":        strconv.Itoa(key),
				"spec":       spec,
				"suffix":     suffix,
				"source_url": sourceURL,
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

	task := c.downloader.GetTask(taskId)
	if task != nil {
		c.downloader_ws.Broadcast(APIClientWSMessage{
			Type: "event",
			Data: map[string]interface{}{
				"task": task,
			},
		})
	}

	result.Ok(ctx, gin.H{
		"message":          "创建下载任务成功",
		"task_id":          taskId,
		"download_task_id": 0,
		"content_id":       0,
	})
}

func (c *APIClient) handleCompatDownloadTaskBatchCreate(ctx *gin.Context) {
	var body []channels.ChannelsObject
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if len(body) == 0 {
		result.Err(ctx, 400, "缺少 feeds")
		return
	}

	batch := base.CreateTaskBatch{}
	for _, raw := range body {
		feed, err := channels.ChannelsObjectToChannelsFeedProfile(&raw)
		if err != nil {
			continue
		}
		if strings.TrimSpace(feed.ObjectId) == "" || strings.TrimSpace(feed.URL) == "" {
			continue
		}

		spec := "original"
		if len(feed.Spec) > 0 {
			spec = feed.Spec[0].FileFormat
		}

		suffix := ".mp4"

		if strings.TrimSpace(raw.Type) == "picture" || raw.ObjectDesc.MediaType == 2 {
			if len(raw.Files) > 0 {
				files := make([]map[string]string, len(raw.Files))
				for i, f := range raw.Files {
					files[i] = map[string]string{
						"url":      f.URL,
						"filename": strconv.Itoa(i+1) + ".jpg",
					}
				}
				filesJSON, _ := json.Marshal(files)
				feed.URL = "zip://weixin.qq.com?files=" + string(filesJSON)
				suffix = ".zip"
			}
		}

		downloadURL := strings.TrimSpace(feed.URL)
		if suffix != ".zip" && !strings.Contains(downloadURL, "zip://") {
			downloadURL = downloadURL + "&X-snsvideoflag=" + spec
		}

		// feed.Title 已在 ChannelsObjectToChannelsFeedProfile 中保证不为空
		filenameBase := strings.TrimSpace(feed.Title)
		name, dir, err := c.formatter.ProcessFilename(filenameBase)
		if err != nil || strings.TrimSpace(name) == "" {
			name = filenameBase
			dir = ""
		}
		if strings.HasSuffix(strings.ToLower(name), ".mp4") {
			suffix = ""
		}

		sourceURL := strings.TrimSpace(feed.SourceURL)
		if sourceURL == "" {
			sourceURL = channels.BuildJumpUrl(feed)
		}

		key := 0
		if strings.TrimSpace(feed.DecryptKey) != "" {
			if v, err := strconv.Atoi(feed.DecryptKey); err == nil {
				key = v
			}
		}

		batch.Reqs = append(batch.Reqs, &base.CreateTaskBatchItem{
			Req: &base.Request{
				URL: downloadURL,
				Labels: map[string]string{
					"id":         feed.ObjectId,
					"nonce_id":   feed.NonceId,
					"title":      feed.Title,
					"key":        strconv.Itoa(key),
					"spec":       spec,
					"suffix":     suffix,
					"source_url": sourceURL,
				},
			},
			Opts: &base.Options{
				Path: filepath.Join(c.cfg.DownloadDir, dir),
				Name: name + suffix,
				Extra: &gopeedhttp.OptsExtra{
					Connections: c.resolve_connections(downloadURL),
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

func (c *APIClient) handleCompatDownloadTaskList(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var body struct {
		Page     *int `json:"page"`
		PageSize *int `json:"pageSize"`
		Status   any  `json:"status"`
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

	db := c.db.Model(&model.DownloadTask{})
	if statuses, ok := parseCompatDownloadTaskStatuses(body.Status); ok {
		if len(statuses) == 1 {
			db = db.Where("status = ?", statuses[0])
		} else {
			db = db.Where("status IN ?", statuses)
		}
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
	counts, allTotal := c.compatDownloadTaskStatusCounts()

	ids := make([]int, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.Id)
	}
	c.ensureDownloadTaskBaselineEvents(tasks)
	var events []model.DownloadTaskEvent
	if len(ids) > 0 {
		_ = c.db.Where("task_id IN ?", ids).Order("id ASC").Find(&events).Error
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
				Filepath:   downloadTaskFullPath(c.cfg.DownloadDir, t.Filepath),
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
		"all_total": allTotal,
		"counts":    counts,
	})
}

func parseCompatDownloadTaskStatuses(raw any) ([]int, bool) {
	if raw == nil {
		return nil, false
	}
	appendStatus := func(out []int, value int) []int {
		for _, existing := range out {
			if existing == value {
				return out
			}
		}
		return append(out, value)
	}
	var statuses []int
	var parseOne func(any)
	parseOne = func(value any) {
		switch v := value.(type) {
		case float64:
			statuses = appendStatus(statuses, int(v))
		case int:
			statuses = appendStatus(statuses, v)
		case string:
			for _, part := range strings.Split(v, ",") {
				part = strings.TrimSpace(part)
				if part == "" || part == "all" {
					continue
				}
				if n, err := strconv.Atoi(part); err == nil {
					statuses = appendStatus(statuses, n)
				}
			}
		case []any:
			for _, item := range v {
				parseOne(item)
			}
		case []int:
			for _, item := range v {
				statuses = appendStatus(statuses, item)
			}
		}
	}
	parseOne(raw)
	return statuses, len(statuses) > 0
}

func (c *APIClient) compatDownloadTaskStatusCounts() (gin.H, int64) {
	counts := gin.H{
		"running": 0,
		"queued":  0,
		"done":    0,
		"error":   0,
		"paused":  0,
	}
	if c.db == nil {
		return counts, 0
	}
	type row struct {
		Status int
		Total  int64
	}
	var rows []row
	if err := c.db.Model(&model.DownloadTask{}).Select("status, count(*) as total").Group("status").Scan(&rows).Error; err != nil {
		return counts, 0
	}
	var allTotal int64
	for _, item := range rows {
		allTotal += item.Total
		switch item.Status {
		case 0, 3:
			counts["queued"] = counts["queued"].(int) + int(item.Total)
		case 1:
			counts["running"] = int(item.Total)
		case 2:
			counts["paused"] = int(item.Total)
		case 4:
			counts["done"] = int(item.Total)
		case 5:
			counts["error"] = int(item.Total)
		}
	}
	counts["total"] = int(allTotal)
	return counts, allTotal
}

func (c *APIClient) handleCompatDownloadTaskProfile(ctx *gin.Context) {
	if c.db == nil {
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
	if err := c.db.Where("task_id = ?", body.TaskId).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err(ctx, 404, "未找到下载任务")
			return
		}
		result.Err(ctx, 2000, err.Error())
		return
	}
	result.Ok(ctx, task)
}

func (c *APIClient) handleCompatDownloadTaskStart(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var body struct {
		TaskId compatDownloadTaskID `json:"download_task_id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if body.TaskId.Empty() {
		result.Err(ctx, 400, "缺少 download_task_id")
		return
	}
	var rec model.DownloadTask
	if err := body.TaskId.Find(c.db, &rec); err != nil {
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
	_ = c.db.Model(&model.DownloadTask{}).Where("id = ?", rec.Id).Updates(map[string]any{
		"task_id": id,
		"status":  2,
	}).Error
	result.Ok(ctx, gin.H{"message": "开始下载"})
}

func (c *APIClient) handleCompatDownloadTaskRetry(ctx *gin.Context) {
	if c.db == nil {
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
	if err := c.db.First(&rec, "id = ?", body.Id).Error; err != nil {
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

	_ = c.db.Model(&model.DownloadTask{}).Where("id = ?", rec.Id).Updates(map[string]any{
		"url":     newURL,
		"task_id": newTaskId,
		"status":  2,
	}).Error
	result.Ok(ctx, gin.H{"message": "已重试下载", "new_task_id": newTaskId})
}

func (c *APIClient) handleCompatDownloadTaskDelete(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var body struct {
		TaskId     compatDownloadTaskID `json:"download_task_id"`
		DeleteFile bool                 `json:"delete_file"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if body.TaskId.Empty() {
		result.Err(ctx, 400, "缺少 download_task_id")
		return
	}
	var task model.DownloadTask
	if err := body.TaskId.Find(c.db, &task); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err(ctx, 404, "未找到下载任务")
			return
		}
		result.Err(ctx, 2000, err.Error())
		return
	}
	if task.TaskId != "" {
		_ = c.downloader.Delete(&downloadpkg.TaskFilter{IDs: []string{task.TaskId}}, body.DeleteFile)
	}
	_ = c.db.Delete(&model.Content{}, "platform_id = ? AND download_task_id = ?", "wx_channels", task.Id).Error
	_ = c.db.Delete(&model.DownloadTask{}, "id = ?", task.Id).Error
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
	fullPath := downloadTaskFullPath(c.cfg.DownloadDir, body.FilePath)
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
	if c.db == nil {
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
	if err := c.db.First(&task, "id = ?", id).Error; err != nil {
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
	if c.db == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	taskId := ctx.Query("task_id")
	if strings.TrimSpace(taskId) == "" {
		result.Err(ctx, 1000, "缺少 task_id")
		return
	}
	var task model.DownloadTask
	if err := c.db.First(&task, "task_id = ?", taskId).Error; err != nil {
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
	if c.db == nil {
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
		if err := c.db.First(&acc, *body.AccountId).Error; err != nil {
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
		err := c.db.Where("platform_id = ? AND external_id = ?", "wx_channels", username).First(&acc).Error
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
			if err := c.db.Create(&acc).Error; err != nil {
				result.Err(ctx, 500, err.Error())
				return
			}
		} else {
			_ = c.db.Model(&model.Account{}).Where("id = ?", acc.Id).Updates(map[string]any{
				"nickname":   nickname,
				"avatar_url": avatar,
				"updated_at": now,
			}).Error
			_ = c.db.First(&acc, acc.Id).Error
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
		pub := int64(obj.CreateTime)
		content := model.Content{
			PlatformId:  "wx_channels",
			ContentType: "video",
			Title:       obj.ObjectDesc.Description,
			Description: obj.ObjectDesc.Description,
			ExternalId:  obj.ID,
			ExternalId2: obj.ObjectNonceId,
			ExternalId3: decodeKey,
			ContentURL:  mediaURL,
			URL:         mediaURL,
			SourceURL:   obj.SourceURL,
			CoverURL:    coverURL,
			Size:        size,
			Duration:    duration,
			PublishTime: &pub,
			Timestamps:  model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}
		var existing model.Content
		err := c.db.Where("platform_id = ? AND external_id = ?", "wx_channels", obj.ID).First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if existing.Id == 0 {
			if err := c.db.Create(&content).Error; err == nil {
				added++
				existing = content
			}
		} else {
			updates := map[string]any{
				"content_type": "video",
				"title":        content.Title,
				"description":  content.Description,
				"external_id2": content.ExternalId2,
				"external_id3": content.ExternalId3,
				"content_url":  content.ContentURL,
				"url":          content.URL,
				"source_url":   content.SourceURL,
				"cover_url":    content.CoverURL,
				"size":         content.Size,
				"duration":     content.Duration,
				"publish_time": content.PublishTime,
				"updated_at":   now,
			}
			if err := c.db.Model(&model.Content{}).Where("id = ?", existing.Id).Updates(updates).Error; err == nil {
				updated++
			}
		}
		if accId > 0 && existing.Id > 0 {
			link := model.ContentAccount{ContentId: existing.Id, AccountId: accId, Role: "owner", CreatedAt: now}
			_ = c.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error
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
