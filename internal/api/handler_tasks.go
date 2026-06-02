package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	officialaccountdownload "github.com/GopeedLab/gopeed/pkg/officialaccount"
	gopeedhttp "github.com/GopeedLab/gopeed/pkg/protocol/http"
	gopeedstream "github.com/GopeedLab/gopeed/pkg/protocol/stream"
	"github.com/gin-gonic/gin"

	apitypes "wx_channel/internal/api/types"
	result "wx_channel/internal/util"
	"wx_channel/pkg/system"
	"wx_channel/pkg/util"
)

type ChannelsDownloadRequest struct {
	Object apitypes.ChannelsObject `json:"object"`
	Spec   string                  `json:"spec"`
	Suffix string                  `json:"suffix"`
}

func (c *APIClient) handleCreateFeedDownloadTask(ctx *gin.Context) {
	bodyBytes, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		result.Err(ctx, 400, "读取请求参数失败")
		return
	}
	ctx.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var dispatchBody struct {
		URL    string                  `json:"url"`
		Object apitypes.ChannelsObject `json:"object"`
	}
	if err := json.Unmarshal(bodyBytes, &dispatchBody); err == nil &&
		dispatchBody.URL != "" &&
		dispatchBody.Object.ID == "" {
		if !isChannelsDownloadURL(dispatchBody.URL) {
			result.Err(ctx, 400, "暂时不支持该下载链接")
			return
		}
		ctx.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		c.handleCreateChannelsTask(ctx)
		return
	}

	var body ChannelsDownloadRequest
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Object.ID == "" {
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

	id, err := c.startDownloadChannelsObject(&body)
	if err != nil {
		c.logger.Error().Interface("body", body).Err(err).Msg("创建任务失败")
		result.Err(ctx, 500, "创建任务失败："+err.Error())
		return
	}

	result.Ok(ctx, gin.H{"id": id})
}

func isChannelsDownloadURL(rawURL string) bool {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	hostname := parsedURL.Hostname()
	path := parsedURL.EscapedPath()
	if strings.EqualFold(hostname, "finder.video.qq.com") {
		return strings.Contains(path, "/stodownload")
	}
	if strings.EqualFold(hostname, "channels.weixin.qq.com") {
		return path == "/web/pages/feed"
	}
	return false
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
		Feeds []ChannelsDownloadRequest `json:"feeds"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}

	var ids []string
	for _, req := range body.Feeds {
		id, err := c.startDownloadChannelsObject(&req)
		if err != nil {
			c.logger.Warn().Err(err).Interface("req", req).Msg("批量创建任务跳过一项")
			continue
		}
		ids = append(ids, id)
	}

	result.Ok(ctx, gin.H{"ids": ids})
}

func (c *APIClient) startDownloadChannelsObject(body *ChannelsDownloadRequest) (string, error) {
	obj := body.Object

	// 1. Convert to profile (validates the object)
	profile, err := apitypes.ChannelsObjectToChannelsFeedProfile(&obj)
	if err != nil {
		return "", fmt.Errorf("转换失败: %w", err)
	}

	// 2. Live is not supported here
	if obj.LiveInfo != nil {
		return "", fmt.Errorf("直播类型请使用直播下载")
	}

	// 3. Upsert Account/Video/VideoAccount in DB (non-fatal)
	video, err := c.channelsUploadService.HandleChannelsFeed(profile)
	if err != nil {
		c.logger.Warn().Err(err).Msg("HandleChannelsFeed failed, continuing without DB records")
	}

	// 4. Resolve spec: request override > config default
	isPicture := obj.Type == "picture" || obj.ObjectDesc.MediaType == 2

	var objMedia *apitypes.ChannelsMediaItem
	if !isPicture && len(obj.ObjectDesc.Media) > 0 {
		objMedia = &obj.ObjectDesc.Media[0]
	}

	spec := body.Spec
	if spec == "" && !c.cfg.Original.GetBool("download.defaultHighest") {
		if objMedia != nil && len(objMedia.Spec) > 0 {
			spec = objMedia.Spec[0].FileFormat
		}
	}

	// 5. Build filename using the template
	filenameTemplate := c.cfg.Original.GetString("download.filenameTemplate")
	filename := util.BuildFilename(
		struct {
			Title     string
			ObjectId  string
			CreatedAt string
			Contact   struct {
				Nickname string
				Username string
			}
		}{
			Title:     profile.Title,
			ObjectId:  profile.ObjectId,
			CreatedAt: strconv.Itoa(profile.CreatedAt),
			Contact: struct {
				Nickname string
				Username string
			}{
				Nickname: profile.Contact.Nickname,
				Username: profile.Contact.Username,
			},
		},
		func() *struct{ FileFormat string } {
			if spec != "" {
				return &struct{ FileFormat string }{FileFormat: spec}
			}
			return nil
		}(),
		struct{ FilenameTemplate string }{FilenameTemplate: filenameTemplate},
	)

	// 6. Validate and split filename into dir/name
	dir, name, err := util.ValidateAndSplitFilename(filename)
	if err != nil {
		return "", fmt.Errorf("不合法的文件名: %w", err)
	}

	// 7. Determine URL and suffix
	var downloadURL string
	suffix := ".mp4"

	if isPicture {
		suffix = ".zip"
		var files []map[string]string
		for i, f := range obj.Files {
			files = append(files, map[string]string{
				"url":      f.URL + f.URLToken,
				"filename": fmt.Sprintf("%d.jpg", i+1),
			})
		}
		data, _ := json.Marshal(files)
		downloadURL = fmt.Sprintf("zip://weixin.qq.com?files=%s", url.QueryEscape(string(data)))
	} else {
		if objMedia == nil {
			return "", fmt.Errorf("缺少可下载的视频内容")
		}
		downloadURL = objMedia.URL + objMedia.URLToken

		// Apply spec to URL
		if spec != "" {
			downloadURL += "&X-snsvideoflag=" + spec
		} else {
			if u, err := url.Parse(downloadURL); err == nil {
				filekey := u.Query().Get("encfilekey")
				token := u.Query().Get("token")
				if filekey != "" && token != "" {
					newURL := u.Scheme + "://" + u.Host + u.Path
					newURL += "?encfilekey=" + filekey + "&token=" + token
					downloadURL = newURL
				}
			}
		}
	}

	// 8. Apply suffix override from request
	if body.Suffix != "" {
		suffix = body.Suffix
	}

	// 9. Dedup filename on disk
	finalName := name + suffix
	finalPath := filepath.Join(c.cfg.DownloadDir, dir)
	if err := os.MkdirAll(finalPath, 0o755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}
	counter := 1
	baseName := name
	for {
		if _, err := os.Stat(filepath.Join(finalPath, finalName)); err == nil {
			finalName = fmt.Sprintf("%s_%d%s", baseName, counter, suffix)
			counter++
		} else {
			break
		}
	}

	// 10. Dedup by external_id in active downloader tasks
	tasks := c.downloader.GetTasks()
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil || t.Meta.Req.Labels == nil {
			continue
		}
		sameID := t.Meta.Req.Labels["id"] == obj.ID
		sameSpec := t.Meta.Req.Labels["spec"] == spec
		sameSuffix := t.Meta.Req.Labels["suffix"] == suffix
		if sameID && sameSpec && sameSuffix {
			return "", fmt.Errorf("已存在该下载内容")
		}
	}

	// 11. Extract decrypt key
	key := 0
	if objMedia != nil && objMedia.DecodeKey != "" {
		if k, err := strconv.Atoi(objMedia.DecodeKey); err == nil {
			key = k
		}
	}

	// 12. Build labels (preserves listener decrypt+mp3)
	labels := map[string]string{
		"id":       obj.ID,
		"nonce_id": obj.ObjectNonceId,
		"title":    profile.Title,
		"key":      strconv.Itoa(key),
		"spec":     spec,
		"suffix":   suffix,
	}

	// 13. Create download task
	taskID, err := c.downloader.CreateDirect(
		&base.Request{
			URL:    downloadURL,
			Labels: labels,
		},
		&base.Options{
			Name: finalName,
			Path: finalPath,
			Extra: &gopeedhttp.OptsExtra{
				Connections: 4,
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("创建任务失败: %w", err)
	}

	// 14. Link DownloadTask to Video in DB
	task := c.downloader.GetTask(taskID)
	if task != nil && video != nil {
		if _, err := c.channelsUploadService.CreateDownloadTaskWithVideo(video, task, "frontend"); err != nil {
			c.logger.Warn().Err(err).Msg("CreateDownloadTaskWithVideo failed")
		}
	}

	// 15. WS broadcast
	if task != nil {
		c.downloader_ws.Broadcast(APIClientWSMessage{
			Type: "event",
			Data: map[string]interface{}{
				"task": task,
			},
		})
	}

	return taskID, nil
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
