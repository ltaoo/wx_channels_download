package api

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	officialaccountdownload "github.com/GopeedLab/gopeed/pkg/officialaccount"
	gopeedhttp "github.com/GopeedLab/gopeed/pkg/protocol/http"
	gopeedstream "github.com/GopeedLab/gopeed/pkg/protocol/stream"
	"github.com/gin-gonic/gin"

	"wx_channel/internal/api/services"
	result "wx_channel/internal/util"
	"wx_channel/pkg/system"
)

func (c *APIClient) handleCreateFeedDownloadTask(ctx *gin.Context) {
	var body services.FeedDownloadTaskBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		result.Err(ctx, 400, "缺少 feed id 参数")
		return
	}
	if body.Suffix == ".mp3" {
		hasFFmpeg := system.ExistingCommand("ffmpeg")
		if !hasFFmpeg {
			result.Err(ctx, 3001, "下载 mp3 需要支持 ffmpeg 命令")
			return
		}
	}

	// Use service
	existing := c.downloadService.CheckExisting(body.Id, body.Spec, body.Suffix)
	if existing {
		result.Err(ctx, 409, "已存在该下载内容")
		return
	}

	opts, err := c.downloadService.BuildTaskOpts(&body)
	if err != nil {
		result.Err(ctx, 409, "不合法的文件名，"+err.Error())
		return
	}

	labels := c.downloadService.BuildTaskLabels(&body)
	id, err := c.downloadService.CreateTask(&base.Request{
		URL:    body.URL,
		Labels: labels,
	}, opts)
	if err != nil {
		c.logger.Error().Interface("body", body).Err(err).Msg("创建任务失败")
		result.Err(ctx, 500, "创建任务失败："+err.Error())
		return
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

	pageNum := 1
	pageSizeNum := 20
	if pageStr != "" {
		if v, err := strconv.Atoi(pageStr); err == nil && v > 0 {
			pageNum = v
		}
	}
	if pageSizeStr != "" {
		if v, err := strconv.Atoi(pageSizeStr); err == nil && v > 0 {
			pageSizeNum = v
		}
	}

	// Use service
	pageResult := c.downloadService.ListTasks(pageNum, pageSizeNum, status)
	result.Ok(ctx, pageResult)
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
		Feeds []services.FeedDownloadTaskBody `json:"feeds"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}

	// Use service
	ids, err := c.downloadService.CreateBatchDownloadTask(body.Feeds)
	if err != nil {
		c.logger.Error().Interface("body", body).Err(err).Msg("创建任务失败")
		result.Err(ctx, 500, "创建任务失败: "+err.Error())
		return
	}

	result.Ok(ctx, gin.H{"ids": ids})
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
	// Use service
	c.downloadService.StartTask(body.Id)
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
	// Use service
	c.downloadService.PauseTask(body.Id)
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
	// Use service
	c.downloadService.ResumeTask(body.Id)
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
	// Use service
	c.downloadService.DeleteTask(body.Id)
	result.Ok(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleClearTasks(ctx *gin.Context) {
	// Use service
	c.downloadService.Clear()
	result.Ok(ctx, nil)
}

func (c *APIClient) handleFetchTaskProfile(ctx *gin.Context) {
	id := ctx.Query("id")
	if id == "" {
		result.Err(ctx, 400, "missing task id")
		return
	}
	// Use service
	profile, err := c.downloadService.GetTaskProfile(id)
	if err != nil {
		result.Err(ctx, 404, "task not found")
		return
	}
	result.Ok(ctx, profile)
}
