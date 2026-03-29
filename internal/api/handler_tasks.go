package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	officialaccountdownload "github.com/GopeedLab/gopeed/pkg/officialaccount"
	gopeedhttp "github.com/GopeedLab/gopeed/pkg/protocol/http"
	gopeedstream "github.com/GopeedLab/gopeed/pkg/protocol/stream"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	apitypes "wx_channel/internal/api/types"
	"wx_channel/internal/database/model"
	result "wx_channel/internal/util"
	"wx_channel/pkg/system"
	utilpkg "wx_channel/pkg/util"
)

type FeedDownloadTaskBody struct {
	Id       string `json:"id"`
	NonceId  string `json:"nonce_id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
	Key      int    `json:"key"`
	Spec     string `json:"spec"`
	Suffix   string `json:"suffix"`
}

type FeedDownloadTaskAccountPayload struct {
	ExternalId string `json:"external_id"`
	Username   string `json:"username"`
	Nickname   string `json:"nickname"`
	AvatarURL  string `json:"avatar_url"`
}

type FeedDownloadTaskCreateReq struct {
	Id       string `json:"id"`
	NonceId  string `json:"nonce_id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
	Key      int    `json:"key"`
	Spec     string `json:"spec"`
	Suffix   string `json:"suffix"`

	SourceURL string `json:"source_url"`
	CoverURL  string `json:"cover_url"`
	FileSize  *int64 `json:"file_size"`
	Width     *int   `json:"width"`
	Height    *int   `json:"height"`

	Account        *FeedDownloadTaskAccountPayload `json:"account"`
	ChannelsObject *apitypes.ChannelsObject        `json:"channels_object"`
}

func (c *APIClient) validateAndDedupeFeedDownloadTask(body *FeedDownloadTaskBody) (int, string) {
	if body == nil {
		return 400, "不合法的参数"
	}
	if body.Id == "" {
		return 400, "缺少 feed id 参数"
	}
	if body.Suffix == ".mp3" {
		hasFFmpeg := system.ExistingCommand("ffmpeg")
		if !hasFFmpeg {
			return 3001, "下载 mp3 需要支持 ffmpeg 命令"
		}
	}
	if c.downloader == nil {
		return 500, "请先初始化 downloader"
	}
	tasks := c.downloader.GetTasks()
	existing := c.check_existing_feed(tasks, body)
	if existing {
		return 409, "已存在该下载内容"
	}
	return 0, ""
}

func (c *APIClient) createFeedDownloadTaskDirect(body *FeedDownloadTaskBody, filename, dir string) (string, int, string) {
	if body == nil {
		return "", 400, "不合法的参数"
	}
	connections := c.resolve_connections(body.URL)
	finalName := filename
	if strings.TrimSpace(body.Suffix) != "" {
		suffixLower := strings.ToLower(body.Suffix)
		if !strings.HasSuffix(strings.ToLower(finalName), suffixLower) {
			finalName += body.Suffix
		}
	}
	id, err := c.downloader.CreateDirect(
		&base.Request{
			URL: body.URL,
			Labels: map[string]string{
				"id":       body.Id,
				"nonce_id": body.NonceId,
				"title":    body.Title,
				"key":      strconv.Itoa(body.Key),
				"spec":     body.Spec,
				"suffix":   body.Suffix,
			},
		},
		&base.Options{
			Name: finalName,
			Path: filepath.Join(c.cfg.DownloadDir, dir),
			Extra: &gopeedhttp.OptsExtra{
				Connections: connections,
			},
		},
	)
	if err != nil {
		c.logger.Error().Interface("body", body).Err(err).Msg("创建任务失败")
		return "", 500, "创建任务失败：" + err.Error()
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
	return id, 0, ""
}

func (c *APIClient) ensureDownloadTaskRecord(taskId string, body *FeedDownloadTaskBody, coverURL string, metadata2 string) *int {
	if c.db == nil || c.db.DB() == nil || strings.TrimSpace(taskId) == "" || body == nil {
		return nil
	}
	now := utilpkg.NowMillis()
	var rec model.DownloadTask
	err := c.db.DB().Where("task_id = ?", taskId).First(&rec).Error
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		protocol := ""
		size := int64(0)
		task := c.downloader.GetTask(taskId)
		if task != nil {
			protocol = task.Protocol
			if task.Meta != nil && task.Meta.Res != nil {
				size = task.Meta.Res.Size
			}
		}
		rec = model.DownloadTask{
			TaskId:     taskId,
			Type:       1,
			Status:     0,
			ExternalId: body.Id,
			Protocol:   protocol,
			URL:        body.URL,
			Title:      strings.TrimSpace(body.Title),
			CoverURL:   coverURL,
			Size:       size,
			Metadata2:  metadata2,
			Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}
		if err := c.db.DB().Create(&rec).Error; err != nil {
			return nil
		}
	} else if err == nil && rec.Id > 0 {
		updates := map[string]any{
			"external_id": body.Id,
			"updated_at":  now,
		}
		if strings.TrimSpace(coverURL) != "" {
			updates["cover_url"] = coverURL
		}
		if strings.TrimSpace(metadata2) != "" {
			updates["metadata2"] = metadata2
		}
		_ = c.db.DB().Model(&model.DownloadTask{}).Where("id = ?", rec.Id).Updates(updates).Error
	} else {
		return nil
	}
	if rec.Id <= 0 {
		return nil
	}
	return &rec.Id
}

func (c *APIClient) upsertAccountAndContentFromFeedDownloadCreate(body *FeedDownloadTaskBody, req *FeedDownloadTaskCreateReq, downloadTaskId *int) {
	if body == nil || c.db == nil || c.db.DB() == nil {
		return
	}
	now := utilpkg.NowMillis()
	platformId := "wx_channels"
	contentType := "video"
	suffix := strings.ToLower(strings.TrimSpace(body.Suffix))
	if suffix == ".mp3" {
		contentType = "audio"
	} else if strings.Contains(suffix, ".jpg") || suffix == ".jpg" || strings.Contains(suffix, ".png") || suffix == ".png" {
		contentType = "image"
	}

	accountId := 0
	if req != nil && req.Account != nil {
		external := strings.TrimSpace(req.Account.ExternalId)
		if external == "" {
			external = strings.TrimSpace(req.Account.Username)
		}
		if external != "" {
			var acc model.Account
			err := c.db.DB().Where("platform_id = ? AND external_id = ?", platformId, external).First(&acc).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return
			}
			if acc.Id == 0 {
				acc = model.Account{
					PlatformId: platformId,
					ExternalId: external,
					Username:   req.Account.Username,
					Nickname:   req.Account.Nickname,
					AvatarURL:  req.Account.AvatarURL,
					Timestamps: model.Timestamps{CreatedAt: now, UpdatedAt: now},
				}
				if err := c.db.DB().Create(&acc).Error; err != nil {
					return
				}
			} else {
				_ = c.db.DB().Model(&model.Account{}).Where("id = ?", acc.Id).Updates(map[string]any{
					"username":    req.Account.Username,
					"nickname":    req.Account.Nickname,
					"avatar_url":  req.Account.AvatarURL,
					"updated_at":  now,
					"platform_id": platformId,
				}).Error
			}
			accountId = acc.Id
		}
	}

	externalId3 := ""
	if body.Key != 0 {
		externalId3 = strconv.Itoa(body.Key)
	}
	sourceURL := ""
	coverURL := ""
	fileSize := int64(0)
	width := 0
	height := 0
	if req != nil {
		sourceURL = strings.TrimSpace(req.SourceURL)
		coverURL = strings.TrimSpace(req.CoverURL)
		if req.FileSize != nil {
			fileSize = *req.FileSize
		}
		if req.Width != nil {
			width = *req.Width
		}
		if req.Height != nil {
			height = *req.Height
		}
	}

	var content model.Content
	err := c.db.DB().Where("platform_id = ? AND content_type = ? AND external_id = ?", platformId, contentType, body.Id).First(&content).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	publishTime := int64(0)
	if req != nil && strings.TrimSpace(req.NonceId) != "" {
		publishTime = now
	}
	if publishTime == 0 {
		publishTime = now
	}
	if content.Id == 0 {
		content = model.Content{
			PlatformId:     platformId,
			ContentType:    contentType,
			ExternalId:     body.Id,
			ExternalId2:    body.NonceId,
			ExternalId3:    externalId3,
			Title:          body.Title,
			Description:    body.Title,
			ContentURL:     "",
			URL:            body.URL,
			SourceURL:      sourceURL,
			CoverURL:       coverURL,
			DownloadTaskId: downloadTaskId,
			DownloadStatus: 0,
			Size:           fileSize,
			Duration:       0,
			PublishTime:    &publishTime,
			Timestamps:     model.Timestamps{CreatedAt: now, UpdatedAt: now},
		}
		if err := c.db.DB().Create(&content).Error; err != nil {
			return
		}
	} else {
		updates := map[string]any{
			"external_id2": body.NonceId,
			"external_id3": externalId3,
			"title":        body.Title,
			"description":  body.Title,
			"url":          body.URL,
			"source_url":   sourceURL,
			"cover_url":    coverURL,
			"size":         fileSize,
			"updated_at":   now,
		}
		if downloadTaskId != nil {
			updates["download_task_id"] = downloadTaskId
		}
		_ = c.db.DB().Model(&model.Content{}).Where("id = ?", content.Id).Updates(updates).Error
	}

	if content.Id > 0 && contentType == "video" {
		var cv model.ContentVideo
		err = c.db.DB().Where("content_id = ?", content.Id).First(&cv).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}
		decodeKey := externalId3
		if cv.ContentId == 0 {
			cv = model.ContentVideo{
				ContentId: content.Id,
				Duration:  0,
				Width:     width,
				Height:    height,
				NonceId:   body.NonceId,
				DecodeKey: decodeKey,
			}
			_ = c.db.DB().Create(&cv).Error
		} else {
			_ = c.db.DB().Model(&model.ContentVideo{}).Where("content_id = ?", content.Id).Updates(map[string]any{
				"width":      width,
				"height":     height,
				"nonce_id":   body.NonceId,
				"decode_key": decodeKey,
			}).Error
		}
	}

	if accountId > 0 && content.Id > 0 {
		link := model.ContentAccount{ContentId: content.Id, AccountId: accountId, Role: "owner", CreatedAt: now}
		_ = c.db.DB().Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error
	}
}

func (c *APIClient) handleCreateFeedDownloadTask(ctx *gin.Context) {
	var req FeedDownloadTaskCreateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	body := FeedDownloadTaskBody{
		Id:       req.Id,
		NonceId:  req.NonceId,
		URL:      req.URL,
		Title:    req.Title,
		Filename: req.Filename,
		Key:      req.Key,
		Spec:     req.Spec,
		Suffix:   req.Suffix,
	}
	if code, msg := c.validateAndDedupeFeedDownloadTask(&body); code != 0 {
		result.Err(ctx, code, msg)
		return
	}
	filename, dir, err := c.formatter.ProcessFilename(body.Filename)
	if err != nil {
		result.Err(ctx, 409, "不合法的文件名，"+err.Error())
		return
	}
	id, code, msg := c.createFeedDownloadTaskDirect(&body, filename, dir)
	if code != 0 {
		result.Err(ctx, code, msg)
		return
	}

	metadata2Bytes, _ := json.Marshal(gin.H{
		"platform":    "wx_channels",
		"id": body.Id,
		"nonce_id":    body.NonceId,
		"key":    body.Key,
	})
	downloadTaskId := c.ensureDownloadTaskRecord(id, &body, strings.TrimSpace(req.CoverURL), string(metadata2Bytes))
	if req.ChannelsObject != nil {
		c.upsertAccountAndVideoFromChannelsObject(req.ChannelsObject, downloadTaskId)
	} else {
		c.upsertAccountAndContentFromFeedDownloadCreate(&body, &req, downloadTaskId)
	}
	result.Ok(ctx, gin.H{"id": id})
}

type DownloadTaskPayload struct {
	URL      string
	Filename string
	Dir      string
	Extra    map[string]string
}

func (c *APIClient) handleCreateDownloadTask(ctx *gin.Context) {
	var body DownloadTaskPayload
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}

	articleID := officialaccountdownload.ExtractArticleID(body.URL)

	tasks := c.downloader.GetTasks()
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil {
			continue
		}
		if articleID != "" && t.Meta.Req.Labels != nil && t.Meta.Req.Labels["article_id"] == articleID {
			result.Err(ctx, 409, "已存在该下载内容")
			return
		}
		if articleID == "" && t.Meta.Req.URL == body.URL {
			result.Err(ctx, 409, "已存在该下载内容")
			return
		}
	}

	labels := body.Extra
	if labels == nil {
		labels = make(map[string]string)
	}
	if articleID != "" {
		labels["article_id"] = articleID
	}

	id, err := c.downloader.CreateDirect(
		&base.Request{
			URL:    body.URL,
			Labels: labels,
		},
		&base.Options{
			Name: body.Filename,
			Path: filepath.Join(c.cfg.DownloadDir, body.Dir),
			Extra: &gopeedhttp.OptsExtra{
				Connections: 1,
			},
		},
	)
	if err != nil {
		result.Err(ctx, 500, "创建任务失败："+err.Error())
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
	result.Ok(ctx, gin.H{"id": id})
}

func (c *APIClient) handleFetchTaskList(ctx *gin.Context) {
	status := ctx.Query("status")
	pageStr := ctx.Query("page")
	pageSizeStr := ctx.Query("page_size")

	filter := &downloadpkg.TaskFilter{}
	if status != "" && status != "all" {
		filter.Statuses = []base.Status{base.Status(status)}
	}
	list := c.downloader.GetTasksByFilter(filter)
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	total := len(list)
	pageNum, err := strconv.Atoi(pageStr)
	if err != nil {
		pageNum = 1
	}
	pageSizeNum, err := strconv.Atoi(pageSizeStr)
	if err != nil {
		pageSizeNum = 20
	}
	start := (pageNum - 1) * pageSizeNum
	if start > total {
		start = total
	}
	end := start + pageSizeNum
	if end > total {
		end = total
	}
	result.Ok(ctx, gin.H{
		"list":      list[start:end],
		"total":     total,
		"page":      pageNum,
		"page_size": pageSizeNum,
	})
}

type LiveDownloadTaskBody struct {
	Url       string            `json:"url"`
	Name      string            `json:"name"`
	UserAgent string            `json:"userAgent"`
	Headers   map[string]string `json:"headers"`
}

func (c *APIClient) handleCreateLiveTask(ctx *gin.Context) {
	var body LiveDownloadTaskBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Url == "" {
		result.Err(ctx, 400, "缺少 url 参数")
		return
	}

	name := body.Name
	if name == "" {
		u, _ := url.Parse(body.Url)
		if u != nil {
			name = filepath.Base(u.Path)
		}
		if name == "" || name == "." || name == "/" {
			name = fmt.Sprintf("live_%d.mp4", time.Now().Unix())
		}
	}
	if !strings.HasSuffix(name, ".mp4") && !strings.HasSuffix(name, ".ts") && !strings.HasSuffix(name, ".flv") && !strings.HasSuffix(name, ".mkv") {
		name += ".mp4"
	}

	reqExtra := &gopeedstream.ReqExtra{
		Header: make(map[string]string),
	}
	if body.UserAgent != "" {
		reqExtra.Header["User-Agent"] = body.UserAgent
	}
	for k, v := range body.Headers {
		reqExtra.Header[k] = v
	}

	id, err := c.downloader.CreateDirect(
		&base.Request{
			URL:   body.Url,
			Extra: reqExtra,
			Labels: map[string]string{
				"type": "live",
			},
		},
		&base.Options{
			Name: name,
			Path: c.cfg.DownloadDir,
		},
	)
	if err != nil {
		result.Err(ctx, 500, "创建任务失败: "+err.Error())
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
	result.Ok(ctx, gin.H{"id": id})
}

func (c *APIClient) handleBatchCreateTask(ctx *gin.Context) {
	var body struct {
		Feeds []FeedDownloadTaskBody `json:"feeds"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	tasks := c.downloader.GetTasks()
	existingTaskMap := make(map[string]int)
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil || t.Meta.Req.Labels == nil {
			continue
		}
		key := fmt.Sprintf("%s|%s|%s", t.Meta.Req.Labels["id"], t.Meta.Req.Labels["spec"], t.Meta.Req.Labels["suffix"])
		existingTaskMap[key] = 1
	}
	task, err := buildBatchCreateTask(c, existingTaskMap, body.Feeds, c.cfg.DownloadDir)
	if err != nil {
		result.Err(ctx, 500, "文件名处理失败: "+err.Error())
		return
	}
	if len(task.Reqs) == 0 {
		result.Ok(ctx, gin.H{"ids": []string{}})
		return
	}
	ids, err := c.downloader.CreateDirectBatch(task)
	if err != nil {
		c.logger.Error().Interface("body", body).Err(err).Msg("创建任务失败")
		result.Err(ctx, 500, "创建任务失败: "+err.Error())
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

func buildBatchCreateTask(c *APIClient, existingTaskMap map[string]int, feeds []FeedDownloadTaskBody, downloadDir string) (*base.CreateTaskBatch, error) {
	var items []map[string]string
	for _, req := range feeds {
		key := fmt.Sprintf("%s|%s|%s", req.Id, req.Spec, req.Suffix)
		if _, exists := existingTaskMap[key]; exists {
			continue
		}
		items = append(items, map[string]string{
			"id":       req.Id,
			"nonce_id": req.NonceId,
			"title":    req.Title,
			"key":      strconv.Itoa(req.Key),
			"spec":     req.Spec,
			"suffix":   req.Suffix,
			"url":      req.URL,
			"name":     req.Filename,
		})
	}
	if len(items) == 0 {
		return &base.CreateTaskBatch{}, nil
	}
	task := base.CreateTaskBatch{}
	for _, item := range items {
		filename, dir, err := c.formatter.ProcessFilename(item["name"] + item["suffix"])
		if err != nil {
			continue
		}
		task.Reqs = append(task.Reqs, &base.CreateTaskBatchItem{
			Req: &base.Request{
				URL: item["url"],
				Labels: map[string]string{
					"id":       item["id"],
					"nonce_id": item["nonce_id"],
					"title":    item["title"],
					"key":      item["key"],
					"spec":     item["spec"],
					"suffix":   item["suffix"],
				},
			},
			Opts: &base.Options{
				Name: filename,
				Path: filepath.Join(downloadDir, dir),
			},
		})
	}
	return &task, nil
}

func (c *APIClient) handleStartTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		result.Err(ctx, 400, "缺少 feed id 参数")
		return
	}
	c.downloader.Continue(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	})
	result.Ok(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handlePauseTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		result.Err(ctx, 400, "缺少 feed id 参数")
		return
	}
	c.downloader.Pause(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	})
	result.Ok(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleResumeTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		result.Err(ctx, 400, "缺少 feed id 参数")
		return
	}
	c.downloader.Continue(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	})
	result.Ok(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleDeleteTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		result.Err(ctx, 400, "缺少 feed id 参数")
		return
	}
	c.downloader.Delete(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	}, true)
	result.Ok(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleClearTasks(ctx *gin.Context) {
	c.downloader.Delete(nil, true)
	c.downloader_ws.Broadcast(APIClientWSMessage{
		Type: "clear",
		Data: c.downloader.GetTasks(),
	})
	result.Ok(ctx, nil)
}

func (c *APIClient) handleFetchTaskProfile(ctx *gin.Context) {
	id := ctx.Query("id")
	if id == "" {
		result.Err(ctx, 400, "missing task id")
		return
	}
	task := c.downloader.GetTask(id)
	if task == nil {
		result.Err(ctx, 404, "task not found")
		return
	}
	if task.Meta == nil || task.Meta.Req == nil {
		result.Err(ctx, 400, "invalid task meta")
		return
	}
	result.Ok(ctx, gin.H{
		"path": task.Meta.Opts.Path,
		"name": task.Meta.Opts.Name,
	})
}
