package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/fs"
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

	"wx_channel/internal/api/services"
	"wx_channel/internal/database/model"
	result "wx_channel/internal/util"
	channels "wx_channel/pkg/scraper/wxchannels"
	utilpkg "wx_channel/pkg/util"
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

func downloadTaskOutputPath(downloadDir string, task model.DownloadTask) string {
	outputPath := strings.TrimSpace(task.OutputPath)
	if outputPath == "" {
		outputPath = strings.TrimSpace(task.Filepath)
	}
	return downloadTaskFullPath(downloadDir, outputPath)
}

func serveDownloadTaskDirectory(ctx *gin.Context, root string, title string) {
	type entry struct {
		RelPath string
		AbsPath string
		Size    int64
	}
	var list []entry
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == root || d.IsDir() {
			return nil
		}
		relPath, relErr := filepath.Rel(root, path)
		if relErr != nil {
			relPath = filepath.Base(path)
		}
		info, statErr := d.Info()
		size := int64(0)
		if statErr == nil {
			size = info.Size()
		}
		list = append(list, entry{RelPath: filepath.ToSlash(relPath), AbsPath: path, Size: size})
		return nil
	})
	if strings.TrimSpace(title) == "" {
		title = filepath.Base(root)
	}
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</title><style>body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,sans-serif;margin:24px;line-height:1.5}a{color:#2563eb;text-decoration:none}a:hover{text-decoration:underline}.meta{color:#71717a;font-size:12px}</style></head><body><h1>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</h1><ol>")
	for _, item := range list {
		b.WriteString("<li><a target=\"_blank\" href=\"/file?path=")
		b.WriteString(html.EscapeString(url.QueryEscape(item.AbsPath)))
		b.WriteString("\">")
		b.WriteString(html.EscapeString(item.RelPath))
		b.WriteString("</a> <span class=\"meta\">")
		b.WriteString(strconv.FormatInt(item.Size, 10))
		b.WriteString(" bytes</span></li>")
	}
	b.WriteString("</ol></body></html>")
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(200, b.String())
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
	c.backfillCompatDownloadTaskParentLinks()
	page := 1
	size := 20
	if v, err := strconv.Atoi(ctx.Query("page")); err == nil && v > 0 {
		page = v
	}
	if v, err := strconv.Atoi(ctx.Query("page_size")); err == nil && v > 0 {
		size = v
	}
	statusParam := ctx.Query("status")
	offset := (page - 1) * size

	db := c.db.Model(&model.DownloadTask{})
	db = db.Where("parent_id IS NULL OR parent_id = 0")
	if statuses, ok := parseCompatDownloadTaskStatuses(statusParam); ok {
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

	ids := make([]string, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.Id)
	}
	c.ensureDownloadTaskBaselineEvents(tasks)
	var events []model.DownloadTaskEvent
	if len(ids) > 0 {
		_ = c.db.Where("task_id IN ?", ids).Order("id ASC").Find(&events).Error
	}
	evMap := map[string][]model.DownloadTaskEvent{}
	for _, e := range events {
		evMap[e.TaskId] = append(evMap[e.TaskId], e)
	}

	type DownloadTaskEventResp struct {
		Id        string `json:"id"`
		TaskId    string `json:"task_id"`
		Type      string `json:"type"`
		Message   string `json:"message"`
		Data      string `json:"data"`
		CreatedAt string `json:"created_at"`
	}
	type DownloadTaskResp struct {
		Id           string             `json:"id"`
		TaskUID      string             `json:"task_uid"`
		TaskId       string             `json:"task_id"`
		ParentId     string             `json:"parent_id"`
		RootId       string             `json:"root_id"`
		NodeType     string             `json:"node_type"`
		Engine       string             `json:"engine"`
		EngineTaskID string             `json:"engine_task_id"`
		Type         int                `json:"type"`
		Status       int                `json:"status"`
		ExternalId   string             `json:"external_id"`
		Protocol     string             `json:"protocol"`
		URL          string             `json:"url"`
		SourceURI    string             `json:"source_uri"`
		Method       string             `json:"method"`
		Title        string             `json:"title"`
		Filename     string             `json:"filename"`
		CoverURL     string             `json:"cover_url"`
		MimeType     string             `json:"mime_type"`
		Size         int64              `json:"size"`
		Downloaded   int64              `json:"downloaded"`
		Speed        int64              `json:"speed"`
		Progress     string             `json:"progress"`
		Filepath     string             `json:"filepath"`
		OutputPath   string             `json:"output_path,omitempty"`
		Error        string             `json:"error"`
		Reason       string             `json:"reason"`
		Metadata1    string             `json:"metadata1"`
		Metadata2    string             `json:"metadata2"`
		Metadata     string             `json:"metadata"`
		Idx          int                `json:"idx"`
		CreatedAt    string             `json:"created_at"`
		UpdatedAt    string             `json:"updated_at"`
		SubtaskCount int                `json:"subtask_count"`
		Subtasks     []DownloadTaskResp `json:"subtasks,omitempty"`
	}
	type TaskWithEvents struct {
		DownloadTaskResp
		Events []DownloadTaskEventResp `json:"events"`
	}
	taskResp := func(t model.DownloadTask) DownloadTaskResp {
		outputPath := downloadTaskOutputPath(c.cfg.DownloadDir, t)
		filepathOut := downloadTaskFullPath(c.cfg.DownloadDir, t.Filepath)
		if outputPath == filepathOut {
			outputPath = ""
		}
		metadata := firstNonEmpty(t.Metadata, t.Metadata2)
		return DownloadTaskResp{
			Id:           t.Id,
			TaskUID:      firstNonEmpty(t.TaskUID, t.TaskId),
			TaskId:       t.TaskId,
			ParentId:     t.ParentId,
			RootId:       t.RootId,
			NodeType:     t.NodeType,
			Engine:       t.Engine,
			EngineTaskID: t.EngineTaskID,
			Type:         t.Type,
			Status:       t.Status,
			ExternalId:   t.ExternalId,
			Protocol:     t.Protocol,
			URL:          t.URL,
			SourceURI:    t.SourceURI,
			Method:       t.Method,
			Title:        t.Title,
			Filename:     t.Filename,
			CoverURL:     t.CoverURL,
			MimeType:     t.MimeType,
			Size:         t.Size,
			Downloaded:   t.Downloaded,
			Speed:        t.Speed,
			Progress:     t.Progress,
			Filepath:     filepathOut,
			OutputPath:   outputPath,
			Error:        t.Error,
			Reason:       t.Reason,
			Metadata1:    t.Metadata1,
			Metadata2:    t.Metadata2,
			Metadata:     metadata,
			Idx:          t.Idx,
			CreatedAt:    strconv.FormatInt(t.CreatedAt, 10),
			UpdatedAt:    strconv.FormatInt(t.UpdatedAt, 10),
		}
	}
	resp := make([]TaskWithEvents, 0, len(tasks))
	for _, t := range tasks {
		taskOut := taskResp(t)
		group := c.compatDownloadTaskGroup(t)
		if len(group) > 1 {
			taskOut.SubtaskCount = len(group) - 1
		}
		out := TaskWithEvents{
			DownloadTaskResp: taskOut,
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
	if err := c.db.Model(&model.DownloadTask{}).
		Where("parent_id IS NULL OR parent_id = 0").
		Select("status, count(*) as total").
		Group("status").
		Scan(&rows).Error; err != nil {
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
		TaskId         string               `json:"task_id"`
		DownloadTaskId compatDownloadTaskID `json:"download_task_id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	taskID := strings.TrimSpace(body.TaskId)
	if body.DownloadTaskId.Empty() && taskID == "" {
		result.Err(ctx, 400, "缺少 task_id")
		return
	}
	var task model.DownloadTask
	var err error
	if !body.DownloadTaskId.Empty() {
		err = body.DownloadTaskId.Find(c.db, &task)
	} else {
		err = c.db.Where("task_id = ?", taskID).First(&task).Error
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err(ctx, 404, "未找到下载任务")
			return
		}
		result.Err(ctx, 2000, err.Error())
		return
	}
	type DownloadTaskResp struct {
		Id           string             `json:"id"`
		TaskUID      string             `json:"task_uid"`
		TaskId       string             `json:"task_id"`
		ParentId     string             `json:"parent_id"`
		RootId       string             `json:"root_id"`
		NodeType     string             `json:"node_type"`
		Engine       string             `json:"engine"`
		EngineTaskID string             `json:"engine_task_id"`
		Type         int                `json:"type"`
		Status       int                `json:"status"`
		ExternalId   string             `json:"external_id"`
		Protocol     string             `json:"protocol"`
		URL          string             `json:"url"`
		SourceURI    string             `json:"source_uri"`
		Method       string             `json:"method"`
		Title        string             `json:"title"`
		Filename     string             `json:"filename"`
		CoverURL     string             `json:"cover_url"`
		MimeType     string             `json:"mime_type"`
		Size         int64              `json:"size"`
		Downloaded   int64              `json:"downloaded"`
		Speed        int64              `json:"speed"`
		Progress     string             `json:"progress"`
		Filepath     string             `json:"filepath"`
		OutputPath   string             `json:"output_path,omitempty"`
		Error        string             `json:"error"`
		Reason       string             `json:"reason"`
		Metadata1    string             `json:"metadata1"`
		Metadata2    string             `json:"metadata2"`
		Metadata     string             `json:"metadata"`
		Idx          int                `json:"idx"`
		CreatedAt    string             `json:"created_at"`
		UpdatedAt    string             `json:"updated_at"`
		SubtaskCount int                `json:"subtask_count"`
		Subtasks     []DownloadTaskResp `json:"subtasks,omitempty"`
	}
	taskResp := func(t model.DownloadTask) DownloadTaskResp {
		outputPath := downloadTaskOutputPath(c.cfg.DownloadDir, t)
		filepathOut := downloadTaskFullPath(c.cfg.DownloadDir, t.Filepath)
		if outputPath == filepathOut {
			outputPath = ""
		}
		metadata := firstNonEmpty(t.Metadata, t.Metadata2)
		return DownloadTaskResp{
			Id:           t.Id,
			TaskUID:      firstNonEmpty(t.TaskUID, t.TaskId),
			TaskId:       t.TaskId,
			ParentId:     t.ParentId,
			RootId:       t.RootId,
			NodeType:     t.NodeType,
			Engine:       t.Engine,
			EngineTaskID: t.EngineTaskID,
			Type:         t.Type,
			Status:       t.Status,
			ExternalId:   t.ExternalId,
			Protocol:     t.Protocol,
			URL:          t.URL,
			SourceURI:    t.SourceURI,
			Method:       t.Method,
			Title:        t.Title,
			Filename:     t.Filename,
			CoverURL:     t.CoverURL,
			MimeType:     t.MimeType,
			Size:         t.Size,
			Downloaded:   t.Downloaded,
			Speed:        t.Speed,
			Progress:     t.Progress,
			Filepath:     filepathOut,
			OutputPath:   outputPath,
			Error:        t.Error,
			Reason:       t.Reason,
			Metadata1:    t.Metadata1,
			Metadata2:    t.Metadata2,
			Metadata:     metadata,
			Idx:          t.Idx,
			CreatedAt:    strconv.FormatInt(t.CreatedAt, 10),
			UpdatedAt:    strconv.FormatInt(t.UpdatedAt, 10),
		}
	}
	out := taskResp(task)
	group := c.compatDownloadTaskGroup(task)
	if len(group) > 1 {
		out.SubtaskCount = len(group) - 1
		out.Subtasks = make([]DownloadTaskResp, 0, len(group)-1)
		for _, child := range group[1:] {
			out.Subtasks = append(out.Subtasks, taskResp(child))
		}
	}
	result.Ok(ctx, out)
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

	if err := c.resumeCompatDownloadTaskRecord(rec); err != nil {
		result.Err(ctx, 500, "恢复下载任务失败: "+err.Error())
		return
	}
	result.Ok(ctx, gin.H{"message": "开始下载"})
}

func (c *APIClient) createCompatDirectTaskFromRecord(rec model.DownloadTask) (string, error) {
	if c.downloader == nil {
		return "", errors.New("下载器未初始化")
	}
	name := strings.TrimSpace(rec.Title)
	if name == "" {
		name = "download_" + strconv.FormatInt(time.Now().Unix(), 10) + ".mp4"
	}
	path := c.cfg.DownloadDir
	if strings.TrimSpace(rec.Filepath) != "" {
		fullPath := downloadTaskFullPath(c.cfg.DownloadDir, rec.Filepath)
		if filepath.Ext(fullPath) != "" {
			name = filepath.Base(fullPath)
			path = filepath.Dir(fullPath)
		} else {
			path = fullPath
		}
	}
	outputPath := filepath.Join(path, name)
	id, err := c.downloader.CreateDirect(
		&base.Request{URL: rec.URL},
		&base.Options{
			Name: name,
			Path: path,
			Extra: &gopeedhttp.OptsExtra{
				Connections: c.resolve_connections(rec.URL),
			},
		},
	)
	if err != nil {
		return "", err
	}
	_ = c.db.Model(&model.DownloadTask{}).Where("id = ?", rec.Id).Updates(map[string]any{
		"task_id":     id,
		"status":      1,
		"output_path": outputPath,
	}).Error
	return id, nil
}

func (c *APIClient) refreshCompatDownloadTaskURL(rec model.DownloadTask) string {
	newURL := strings.TrimSpace(rec.URL)
	if strings.TrimSpace(rec.Metadata2) == "" || c.channels == nil {
		return newURL
	}
	var meta struct {
		ExternalId string `json:"external_id"`
		NonceId    string `json:"nonce_id"`
		EID        string `json:"eid"`
	}
	if err := json.Unmarshal([]byte(rec.Metadata2), &meta); err != nil || meta.ExternalId == "" {
		return newURL
	}
	profile, err := c.channels.FetchChannelsFeedProfile(meta.ExternalId, meta.NonceId, "", meta.EID)
	if err != nil || profile == nil || profile.ErrCode != 0 || len(profile.Data.Object.ObjectDesc.Media) == 0 {
		return newURL
	}
	media := profile.Data.Object.ObjectDesc.Media[0]
	if url := strings.TrimSpace(media.URL + media.URLToken); url != "" {
		return url
	}
	return newURL
}

func (c *APIClient) retryCompatDownloadTaskRecord(rec model.DownloadTask) (string, error) {
	if compatDownloadTaskIsPlatformFileSubtask(rec) {
		return c.retryCompatPlatformFileSubtask(rec, compatDownloadTaskRetryFilePaths(rec))
	}
	if compatDownloadTaskUsesPlatformRetry(rec) {
		return c.restartPlatformDownloadTaskRecord(context.Background(), rec)
	}
	if c.downloader == nil {
		return "", errors.New("下载器未初始化")
	}
	newURL := c.refreshCompatDownloadTaskURL(rec)
	if rec.TaskId != "" {
		_ = c.downloader.Delete(&downloadpkg.TaskFilter{IDs: []string{rec.TaskId}}, true)
	}
	rec.URL = newURL
	newTaskID, err := c.createCompatDirectTaskFromRecord(rec)
	if err != nil {
		return "", err
	}
	now := utilpkg.NowMillis()
	if c.db != nil && rec.Id != "" {		outputPath := downloadTaskOutputPath(c.cfg.DownloadDir, rec)
		if task := c.downloader.GetTask(newTaskID); task != nil && task.Meta != nil {
			outputPath = task.Meta.SingleFilepath()
		}
		if err := c.db.Model(&model.DownloadTask{}).Where("id = ?", rec.Id).Updates(map[string]any{
			"url":         newURL,
			"task_id":     newTaskID,
			"status":      1,
			"error":       "",
			"output_path": outputPath,
			"updated_at":  now,
		}).Error; err != nil {
			return "", err
		}
		_ = c.db.Create(&model.DownloadTaskEvent{
			TaskId:    rec.Id,
			Type:      "retry",
			Message:   "重试下载任务",
			CreatedAt: now,
		}).Error
	}
	return newTaskID, nil
}

func (c *APIClient) retryCompatDownloadTaskRecordWithFilePaths(rec model.DownloadTask, retryFilePaths []string) (string, error) {
	if compatDownloadTaskIsPlatformFileSubtask(rec) {
		return c.retryCompatPlatformFileSubtask(rec, retryFilePaths)
	}
	if compatDownloadTaskUsesPlatformRetry(rec) {
		return c.restartPlatformDownloadTaskRecordWithFilePaths(context.Background(), rec, retryFilePaths)
	}
	return c.retryCompatDownloadTaskRecord(rec)
}

func compatDownloadTaskIsPlatformFileSubtask(rec model.DownloadTask) bool {
	if rec.ParentId == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(rec.NodeType), "file") && strings.TrimSpace(rec.Engine) != "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(rec.Protocol), "file_node") &&
		strings.EqualFold(strings.TrimSpace(rec.Reason), "platform_file")
}

func (c *APIClient) retryCompatPlatformFileSubtask(rec model.DownloadTask, retryFilePaths []string) (string, error) {
	if c.db == nil {
		return "", errors.New("数据库未初始化")
	}
	if len(retryFilePaths) == 0 {
		retryFilePaths = compatDownloadTaskRetryFilePaths(rec)
	}
	if len(retryFilePaths) == 0 {
		return "", errors.New("平台子任务缺少 tree_path，无法只重试该子任务")
	}
	var parent model.DownloadTask
	if err := c.db.First(&parent, "id = ?", rec.ParentId).Error; err != nil {
		return "", err
	}
	now := utilpkg.NowMillis()
	_ = c.db.Model(&model.DownloadTask{}).Where("id = ?", rec.Id).Updates(map[string]any{
		"status":     1,
		"error":      "",
		"updated_at": now,
	}).Error
	return c.restartPlatformDownloadTaskRecordWithFilePaths(context.Background(), parent, retryFilePaths)
}

func compatDownloadTaskUsesPlatformRetry(rec model.DownloadTask) bool {
	if strings.EqualFold(strings.TrimSpace(rec.NodeType), "container") {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(rec.Engine)) {
	case "clawreq", "cdp", "sandbox_cdp", "browser_pool_cdp":
		return true
	}
	if strings.EqualFold(strings.TrimSpace(rec.Reason), "platform") {
		return true
	}
	if compatDownloadTaskHasPlatformMetadata(rec) {
		return true
	}
	protocol := strings.ToLower(strings.TrimSpace(rec.Protocol))
	if protocol != "" && !compatDirectDownloadProtocol(protocol) {
		return true
	}
	scheme := compatDownloadTaskURLScheme(rec.URL)
	if scheme == "" {
		return false
	}
	return !compatDirectDownloadProtocol(scheme)
}

func compatDownloadTaskHasPlatformMetadata(rec model.DownloadTask) bool {
	for _, raw := range []string{rec.Metadata, rec.Metadata2, rec.Metadata1} {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var meta map[string]any
		if err := json.Unmarshal([]byte(raw), &meta); err != nil {
			continue
		}
		if strings.TrimSpace(toCompatString(compatNestedMetadataValue(meta, "source", "platform"))) != "" {
			return true
		}
		if strings.TrimSpace(toCompatString(compatNestedMetadataValue(meta, "metadata", "workflow_run_id"))) != "" {
			return true
		}
		if strings.TrimSpace(toCompatString(compatNestedMetadataValue(meta, "labels", "platform"))) != "" {
			return true
		}
	}
	return false
}

func compatNestedMetadataValue(meta map[string]any, path ...string) any {
	var current any = meta
	for _, key := range path {
		next, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = next[key]
	}
	return current
}

func compatDirectDownloadProtocol(protocol string) bool {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "", "http", "https", "stream", "rtmp", "m3u8", "officialaccount", "zhihu", "zip":
		return true
	default:
		return false
	}
}

func compatDownloadTaskURLScheme(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err == nil && parsed.Scheme != "" {
		return strings.ToLower(parsed.Scheme)
	}
	if i := strings.Index(rawURL, ":"); i > 0 {
		candidate := rawURL[:i]
		if candidate != "" && strings.Trim(candidate, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+-.") == "" {
			return strings.ToLower(candidate)
		}
	}
	return ""
}

func (c *APIClient) resumeCompatDownloadTaskRecord(rec model.DownloadTask) error {
	if rec.Status == 4 {
		return nil
	}
	if strings.TrimSpace(rec.TaskId) != "" && c.resumeActivePlatformDownload(rec.TaskId) {
		return nil
	}
	if compatDownloadTaskUsesPlatformRetry(rec) {
		_, err := c.restartPlatformDownloadTaskRecord(context.Background(), rec)
		return err
	}
	if c.downloader != nil && strings.TrimSpace(rec.TaskId) != "" && c.downloader.GetTask(rec.TaskId) != nil {
		err := c.downloader.Continue(&downloadpkg.TaskFilter{IDs: []string{rec.TaskId}})
		if err != nil && !errors.Is(err, downloadpkg.ErrTaskNotFound) {
			return err
		}
		return nil
	}
	_, err := c.createCompatDirectTaskFromRecord(rec)
	return err
}

func (c *APIClient) pauseCompatDownloadTaskRecord(rec model.DownloadTask) error {
	if rec.Status == 4 || rec.Status == 5 {
		return nil
	}
	if strings.TrimSpace(rec.TaskId) != "" && c.pauseActivePlatformDownload(rec.TaskId) {
		return nil
	}
	if c.downloader != nil && strings.TrimSpace(rec.TaskId) != "" && c.downloader.GetTask(rec.TaskId) != nil {
		err := c.downloader.Pause(&downloadpkg.TaskFilter{IDs: []string{rec.TaskId}})
		if err != nil && !errors.Is(err, downloadpkg.ErrTaskNotFound) {
			return err
		}
		return nil
	}
	return c.updateCompatDownloadTaskManualStatus(rec, 2, "pause", "暂停下载任务")
}

func (c *APIClient) updateCompatDownloadTaskManualStatus(rec model.DownloadTask, status int, eventType string, message string) error {
	if c.db == nil || rec.Id == "" {		return nil
	}
	now := utilpkg.NowMillis()
	if err := c.db.Model(&model.DownloadTask{}).Where("id = ?", rec.Id).Updates(map[string]any{
		"status":     status,
		"error":      "",
		"updated_at": now,
	}).Error; err != nil {
		return err
	}
	if eventType != "" {
		_ = c.db.Create(&model.DownloadTaskEvent{
			TaskId:    rec.Id,
			Type:      eventType,
			Message:   message,
			CreatedAt: now,
		}).Error
	}
	return nil
}

func (c *APIClient) compatDownloadTaskGroup(root model.DownloadTask) []model.DownloadTask {
	records := []model.DownloadTask{root}
	if c.db == nil {
		return records
	}
	seenIDs := map[string]bool{}
	seenTaskIDs := map[string]bool{}
	addChild := func(child model.DownloadTask) {
		if child.Id != "" && seenIDs[child.Id] {
			return
		}
		if child.TaskId != "" && seenTaskIDs[child.TaskId] {
			return
		}
		records = append(records, child)
		if child.Id != "" {
			seenIDs[child.Id] = true
		}
		if child.TaskId != "" {
			seenTaskIDs[child.TaskId] = true
		}
	}
	for _, rec := range records {
		if rec.Id != "" {
			seenIDs[rec.Id] = true
		}
		if rec.TaskId != "" {
			seenTaskIDs[rec.TaskId] = true
		}
	}

	var nativeChildren []model.DownloadTask
	if root.Id != "" {
		if err := c.db.Where("parent_id = ?", root.Id).Order("idx ASC, id ASC").Find(&nativeChildren).Error; err == nil {
			for _, child := range nativeChildren {
				addChild(child)
			}
		}
	}

	taskIDs, dbIDs := compatDownloadTaskChildRefs(root.Metadata1, root.Metadata2)
	if len(taskIDs) == 0 && len(dbIDs) == 0 {
		return records
	}
	db := c.db.Model(&model.DownloadTask{})
	hasWhere := false
	if len(taskIDs) > 0 {
		db = db.Where("task_id IN ?", taskIDs)
		hasWhere = true
	}
	if len(dbIDs) > 0 {
		if hasWhere {
			db = db.Or("id IN ?", dbIDs)
		} else {
			db = db.Where("id IN ?", dbIDs)
		}
	}

	var children []model.DownloadTask
	if err := db.Find(&children).Error; err != nil {
		return records
	}
	for _, child := range children {
		if root.Id != "" && child.Id != root.Id && child.ParentId == "" {
			rootID := root.RootId
			if rootID == "" {
				rootID = root.Id
			}
			_ = c.db.Model(&model.DownloadTask{}).Where("id = ?", child.Id).Updates(map[string]any{
				"parent_id": root.Id,
				"root_id":   rootID,
			}).Error
			child.ParentId = root.Id
			child.RootId = rootID
		}
		addChild(child)
	}
	return records
}

func (c *APIClient) backfillCompatDownloadTaskParentLinks() {
	if c.db == nil {
		return
	}
	var roots []model.DownloadTask
	if err := c.db.
		Where("(metadata1 IS NOT NULL AND metadata1 <> '') OR (metadata2 IS NOT NULL AND metadata2 <> '')").
		Find(&roots).Error; err != nil {
		return
	}
	for _, root := range roots {
		if root.Id == "" {
			continue
		}
		taskIDs, dbIDs := compatDownloadTaskChildRefs(root.Metadata1, root.Metadata2)
		if len(taskIDs) == 0 && len(dbIDs) == 0 {
			continue
		}
		db := c.db.Model(&model.DownloadTask{})
		hasWhere := false
		if len(taskIDs) > 0 {
			db = db.Where("task_id IN ?", taskIDs)
			hasWhere = true
		}
		if len(dbIDs) > 0 {
			if hasWhere {
				db = db.Or("id IN ?", dbIDs)
			} else {
				db = db.Where("id IN ?", dbIDs)
			}
		}
		rootID := root.RootId
		if rootID == "" {
			rootID = root.Id
		}
		_ = db.
			Where("id <> ?", root.Id).
			Where("parent_id IS NULL OR parent_id = 0").
			Updates(map[string]any{"parent_id": root.Id, "root_id": rootID}).Error
	}
}

func compatDownloadTaskChildRefs(values ...string) ([]string, []string) {
	taskIDs := make([]string, 0)
	dbIDs := make([]string, 0)
	seenTaskIDs := map[string]bool{}
	seenDBIDs := map[string]bool{}

	addTaskID := func(value any) {}
	addDBID := func(value any) {}
	addTaskID = func(value any) {
		switch v := value.(type) {
		case string:
			for _, part := range strings.Split(v, ",") {
				part = strings.TrimSpace(part)
				if part == "" || seenTaskIDs[part] {
					continue
				}
				seenTaskIDs[part] = true
				taskIDs = append(taskIDs, part)
			}
		case []any:
			for _, item := range v {
				addTaskID(item)
			}
		case []string:
			for _, item := range v {
				addTaskID(item)
			}
		}
	}
	addDBID = func(value any) {
		switch v := value.(type) {
		case float64:
			id := fmt.Sprintf("%.0f", v)
			if id != "" && id != "0" && !seenDBIDs[id] {
				seenDBIDs[id] = true
				dbIDs = append(dbIDs, id)
			}
		case int:
			id := strconv.Itoa(v)
			if id != "" && id != "0" && !seenDBIDs[id] {
				seenDBIDs[id] = true
				dbIDs = append(dbIDs, id)
			}
		case string:
			for _, part := range strings.Split(v, ",") {
				part = strings.TrimSpace(part)
				if part == "" || seenDBIDs[part] {
					continue
				}
				seenDBIDs[part] = true
				dbIDs = append(dbIDs, part)
			}
		case []any:
			for _, item := range v {
				addDBID(item)
			}
		case []int:
			for _, item := range v {
				addDBID(item)
			}
		}
	}

	var walk func(any)
	walk = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			for key, item := range v {
				normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
				switch normalized {
				case "taskid", "taskids", "childtaskid", "childtaskids", "subtaskid", "subtaskids":
					addTaskID(item)
				case "downloadtaskid", "downloadtaskids", "childdownloadtaskid", "childdownloadtaskids", "subdownloadtaskid", "subdownloadtaskids":
					addDBID(item)
				case "children", "childtasks", "subtasks":
					walk(item)
				default:
					walk(item)
				}
			}
		case []any:
			for _, item := range v {
				walk(item)
			}
		}
	}

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		var raw any
		if err := json.Unmarshal([]byte(value), &raw); err == nil {
			walk(raw)
		}
	}
	return taskIDs, dbIDs
}

func compatDownloadTaskProblemFileCount(values ...string) int {
	count := 0
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		var raw any
		if err := json.Unmarshal([]byte(value), &raw); err != nil {
			continue
		}
		count += compatProblemFileCountInMetadata(raw)
	}
	return count
}

func compatProblemFileCountInMetadata(value any) int {
	switch v := value.(type) {
	case map[string]any:
		total := 0
		for key, item := range v {
			normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
			if normalized == "files" {
				total += compatProblemFileNodeCount(item)
				continue
			}
			total += compatProblemFileCountInMetadata(item)
		}
		return total
	case []any:
		total := 0
		for _, item := range v {
			total += compatProblemFileCountInMetadata(item)
		}
		return total
	default:
		return 0
	}
}

func compatProblemFileNodeCount(value any) int {
	switch v := value.(type) {
	case []any:
		total := 0
		for _, item := range v {
			total += compatProblemFileNodeCount(item)
		}
		return total
	case map[string]any:
		children := firstCompatMapValue(v, "children", "Children")
		childCount := compatProblemFileNodeCount(children)
		if childCount > 0 {
			return childCount
		}
		if compatFileNodeHasProblem(v) {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func firstCompatMapValue(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func compatFileNodeHasProblem(values map[string]any) bool {
	errorValue := firstCompatMapValue(values, "error", "Error", "err", "Err")
	if strings.TrimSpace(toCompatString(errorValue)) != "" {
		return true
	}
	status := strings.ToLower(strings.TrimSpace(toCompatString(firstCompatMapValue(values, "status", "Status"))))
	switch status {
	case "error", "failed", "fail", "pause", "paused":
		return true
	default:
		return false
	}
}

func toCompatString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func (c *APIClient) handleCompatDownloadTaskPause(ctx *gin.Context) {
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
	records := c.compatDownloadTaskGroup(rec)
	for _, item := range records {
		if err := c.pauseCompatDownloadTaskRecord(item); err != nil {
			result.Err(ctx, 500, "暂停下载任务失败: "+err.Error())
			return
		}
	}
	result.Ok(ctx, gin.H{"message": "已暂停下载任务", "count": len(records)})
}

func (c *APIClient) handleCompatDownloadTaskResume(ctx *gin.Context) {
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
	records := c.compatDownloadTaskGroup(rec)
	for _, item := range records {
		if err := c.resumeCompatDownloadTaskRecord(item); err != nil {
			result.Err(ctx, 500, "恢复下载任务失败: "+err.Error())
			return
		}
	}
	result.Ok(ctx, gin.H{"message": "已恢复下载任务", "count": len(records)})
}

func (c *APIClient) handleCompatDownloadTaskRetry(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if body.Id == "" {
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

	newTaskID, err := c.retryCompatDownloadTaskRecord(rec)
	if err != nil {
		result.Err(ctx, 500, "提交新下载任务失败: "+err.Error())
		return
	}
	result.Ok(ctx, gin.H{"message": "已重试下载", "new_task_id": newTaskID})
}

func (c *APIClient) handleCompatDownloadTaskRetryChildren(ctx *gin.Context) {
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

	records := c.compatDownloadTaskGroup(rec)
	go c.retryCompatDownloadTaskChildren(rec, records)

	result.Ok(ctx, gin.H{
		"message": "重试成功",
	})
}

func (c *APIClient) retryCompatDownloadTaskChildren(rec model.DownloadTask, records []model.DownloadTask) {
	hasChildren := len(records) > 1
	retried := 0
	resumed := 0
	skipped := 0
	total := 0
	platformRetryPaths := compatDownloadTaskPlatformRetryPathsByParent(rec, records)
	restartedPlatformParents := map[string]bool{}
	process := func(item model.DownloadTask) error {
		total++
		if compatDownloadTaskShouldRestartPlatformOnRetryChildren(item) {
			parentKey := compatDownloadTaskPlatformRestartKey(item)
			if parentKey != "" && restartedPlatformParents[parentKey] {
				skipped++
				return nil
			}
			if _, err := c.retryCompatDownloadTaskRecordWithFilePaths(item, platformRetryPaths[parentKey]); err != nil {
				return err
			}
			if parentKey != "" {
				restartedPlatformParents[parentKey] = true
			}
			retried++
			return nil
		}
		switch item.Status {
		case 2:
			if err := c.resumeCompatDownloadTaskRecord(item); err != nil {
				return err
			}
			resumed++
		case 5:
			if _, err := c.retryCompatDownloadTaskRecord(item); err != nil {
				return err
			}
			retried++
		case 4:
			if compatDownloadTaskProblemFileCount(item.Metadata1, item.Metadata2) == 0 {
				skipped++
				return nil
			}
			if _, err := c.retryCompatDownloadTaskRecord(item); err != nil {
				return err
			}
			retried++
		default:
			skipped++
		}
		return nil
	}

	for _, item := range records {
		if hasChildren && item.Id == rec.Id {
			continue
		}
		if err := process(item); err != nil {
			if c.logger != nil {
				c.logger.Error().Err(err).Str("download_task_id", rec.Id).Str("child_task_id", item.Id).Msg("重新开始子任务失败")
			}
			return
		}
	}
	if retried == 0 && resumed == 0 && hasChildren && compatDownloadTaskShouldRetryParentAfterChildrenSkipped(rec) {
		if err := process(rec); err != nil {
			if c.logger != nil {
				c.logger.Error().Err(err).Str("download_task_id", rec.Id).Msg("重新开始子任务失败")
			}
			return
		}
	}

	if c.logger != nil {
		c.logger.Info().
			Str("download_task_id", rec.Id).
			Int("total", total).
			Int("retried", retried).
			Int("resumed", resumed).
			Int("skipped", skipped).
			Msg("重新开始异常子任务完成")
	}
}

func compatDownloadTaskShouldRestartPlatformOnRetryChildren(item model.DownloadTask) bool {
	if item.Status == 4 {
		return false
	}
	return compatDownloadTaskIsPlatformFileSubtask(item) || compatDownloadTaskUsesPlatformRetry(item)
}

func compatDownloadTaskShouldRetryParentAfterChildrenSkipped(rec model.DownloadTask) bool {
	if compatDownloadTaskProblemFileCount(rec.Metadata1, rec.Metadata2) > 0 {
		return true
	}
	if rec.Status == 2 || rec.Status == 5 {
		return true
	}
	return compatDownloadTaskShouldRestartPlatformOnRetryChildren(rec)
}

func compatDownloadTaskPlatformRetryPathsByParent(root model.DownloadTask, records []model.DownloadTask) map[string][]string {
	out := map[string][]string{}
	seen := map[string]map[string]bool{}
	for _, item := range records {
		if item.Id == root.Id && len(records) > 1 {
			continue
		}
		if !compatDownloadTaskShouldRestartPlatformOnRetryChildren(item) {
			continue
		}
		parentKey := compatDownloadTaskPlatformRestartKey(item)
		if parentKey == "" {
			continue
		}
		paths := compatDownloadTaskRetryFilePaths(item)
		if len(paths) == 0 {
			continue
		}
		if seen[parentKey] == nil {
			seen[parentKey] = map[string]bool{}
		}
		for _, path := range paths {
			if seen[parentKey][path] {
				continue
			}
			seen[parentKey][path] = true
			out[parentKey] = append(out[parentKey], path)
		}
	}
	return out
}

func compatDownloadTaskPlatformRestartKey(item model.DownloadTask) string {
	if item.ParentId != "" {
		return item.ParentId
	}
	return item.Id
}

func compatDownloadTaskRetryFilePaths(rec model.DownloadTask) []string {
	if path := compatDownloadTaskMetadataString(rec, "tree_path"); path != "" {
		return []string{path}
	}
	if path := strings.TrimSpace(rec.SourceURI); strings.HasPrefix(filepath.ToSlash(path), "chapters/") || strings.HasPrefix(filepath.ToSlash(path), "source/") {
		return []string{filepath.ToSlash(path)}
	}
	if path := strings.TrimSpace(rec.URL); strings.HasPrefix(filepath.ToSlash(path), "chapters/") || strings.HasPrefix(filepath.ToSlash(path), "source/") {
		return []string{filepath.ToSlash(path)}
	}
	return nil
}

func compatDownloadTaskMetadataString(rec model.DownloadTask, key string) string {
	for _, raw := range []string{rec.Metadata, rec.Metadata2, rec.Metadata1} {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var meta map[string]any
		if err := json.Unmarshal([]byte(raw), &meta); err != nil {
			continue
		}
		if value := strings.TrimSpace(toCompatString(meta[key])); value != "" {
			return value
		}
	}
	return ""
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
	records := c.compatDownloadTaskGroup(task)
	ids := make([]string, 0, len(records))
	for _, rec := range records {
		if rec.TaskId != "" && rec.TaskId != task.TaskId {
			_ = c.downloader.Delete(&downloadpkg.TaskFilter{IDs: []string{rec.TaskId}}, body.DeleteFile)
		}
		if rec.Id != "" {			ids = append(ids, rec.Id)
		}
	}
	_ = c.db.Delete(&model.Content{}, "download_task_id IN ?", ids).Error
	_ = c.db.Delete(&model.DownloadTask{}, "id IN ?", ids).Error
	result.Ok(ctx, gin.H{"message": "删除下载任务成功"})
}

func (c *APIClient) handleCompatDownloadTaskPauseAll(ctx *gin.Context) {
	platformCount := 0
	platformActiveDownloads.Range(func(_, value any) bool {
		active, ok := value.(*platformActiveDownload)
		if ok && active != nil && c.pauseActivePlatformDownload(active.taskID) {
			platformCount++
		}
		return true
	})
	err := c.downloader.Pause(&downloadpkg.TaskFilter{
		Statuses: []base.Status{
			base.DownloadStatusRunning,
			base.DownloadStatusWait,
			base.DownloadStatusReady,
		},
	})
	if err != nil && (!errors.Is(err, downloadpkg.ErrTaskNotFound) || platformCount == 0) {
		result.Err(ctx, 500, "暂停全部任务失败: "+err.Error())
		return
	}
	result.Ok(ctx, gin.H{"message": "已暂停全部任务"})
}

func (c *APIClient) handleCompatDownloadTaskStartAll(ctx *gin.Context) {
	platformCount := 0
	platformActiveDownloads.Range(func(_, value any) bool {
		active, ok := value.(*platformActiveDownload)
		if ok && active != nil && c.resumeActivePlatformDownload(active.taskID) {
			platformCount++
		}
		return true
	})
	err := c.downloader.Continue(&downloadpkg.TaskFilter{
		Statuses: []base.Status{
			base.DownloadStatusPause,
		},
	})
	if err != nil && (!errors.Is(err, downloadpkg.ErrTaskNotFound) || platformCount == 0) {
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
	filePath := strings.TrimSpace(task.OutputPath)
	if filePath == "" {
		filePath = strings.TrimSpace(task.Filepath)
	}
	if filePath == "" {
		filePath = strings.TrimSpace(task.Title)
	}
	if filePath == "" {
		result.Err(ctx, 404, "文件不存在")
		return
	}
	fullPath := downloadTaskFullPath(c.cfg.DownloadDir, filePath)
	if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
		serveDownloadTaskDirectory(ctx, fullPath, task.Title)
		return
	} else if os.IsNotExist(err) {
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
	} else if err != nil {
		result.Err(ctx, 404, "文件不存在: "+fullPath)
		return
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
	accId := ""
	{
		var acc model.Account
		err := c.db.Where("platform_id = ? AND external_id = ?", "wx_channels", username).First(&acc).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err(ctx, 500, err.Error())
			return
		}
		nickname := resp.Data.Contact.Nickname
		avatar := resp.Data.Contact.HeadUrl
		if acc.Id == "" {
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
		if existing.Id == "" {
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
		if accId != "" && existing.Id != "" {
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
