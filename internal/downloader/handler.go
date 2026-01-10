package downloader

// "fmt"
// "net/url"
// "path/filepath"
// "strconv"
// "strings"
// "time"

// "github.com/GopeedLab/gopeed/pkg/base"
// "github.com/gin-gonic/gin"

// result "wx_channel/internal/util"
// "wx_channel/pkg/system"
// "wx_channel/pkg/util"

type FeedDownloadTaskBody struct {
	Id       string `json:"id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
	Key      int    `json:"key"`
	Spec     string `json:"spec"`
	Suffix   string `json:"suffix"`
}

// func (c *DownloadClient) handleCreateTask(ctx *gin.Context) {
// 	var body FeedDownloadTaskBody
// 	if err := ctx.ShouldBindJSON(&body); err != nil {
// 		result.Err(ctx, 400, "不合法的参数")
// 		return
// 	}
// 	if body.Id == "" {
// 		result.Err(ctx, 400, "缺少 feed id 参数")
// 		return
// 	}
// 	if body.Suffix == ".mp3" {
// 		has_ffmpeg := system.ExistingCommand("ffmpeg")
// 		if !has_ffmpeg {
// 			result.Err(ctx, 400, "下载 mp3 需要支持 ffmpeg 命令")
// 			return
// 		}
// 	}
// 	tasks := c.downloader.GetTasks()
// 	existing := c.check_existing_feed(tasks, &body)
// 	if existing {
// 		result.Err(ctx, 409, "已存在该下载内容")
// 		// ctx.JSON(http.StatusOK, Response{Code: 409, Msg: , Data: body})
// 		return
// 	}
// 	filename, dir, err := c.formatter.ProcessFilename(body.Filename)
// 	if err != nil {
// 		result.Err(ctx, 409, "不合法的文件名")
// 		return
// 	}
// 	connections := c.resolveConnections(body.URL)
// 	id, err := c.downloader.CreateDirect(
// 		&base.Request{
// 			URL: body.URL,
// 			Labels: map[string]string{
// 				"id":     body.Id,
// 				"title":  body.Title,
// 				"key":    strconv.Itoa(body.Key),
// 				"spec":   body.Spec,
// 				"suffix": body.Suffix,
// 			},
// 		},
// 		&base.Options{
// 			Name: filename + body.Suffix,
// 			Path: filepath.Join(c.cfg.DownloadDir, dir),
// 			Extra: &gopeedhttp.OptsExtra{
// 				Connections: connections,
// 			},
// 		},
// 	)
// 	if err != nil {
// 		result.Err(ctx, 500, "下载失败")
// 		return
// 	}
// 	c.channels.Broadcast(APIClientWSMessage{
// 		Type: "tasks",
// 		Data: c.downloader.GetTasks(),
// 	})
// 	result.Ok(ctx, gin.H{"id": id})
// }

// type LiveDownloadTaskBody struct {
// 	Url       string            `json:"url"`
// 	Name      string            `json:"name"`
// 	UserAgent string            `json:"userAgent"`
// 	Headers   map[string]string `json:"headers"`
// }

// func (c *DownloadClient) handleCreateLiveTask(ctx *gin.Context) {
// 	var body LiveDownloadTaskBody
// 	if err := ctx.ShouldBindJSON(&body); err != nil {
// 		result.Err(ctx, 400, "不合法的参数")
// 		return
// 	}
// 	if body.Url == "" {
// 		result.Err(ctx, 400, "缺少 url 参数")
// 		return
// 	}

// 	name := body.Name
// 	if name == "" {
// 		// Try to parse from URL or use timestamp
// 		u, _ := url.Parse(body.Url)
// 		if u != nil {
// 			name = filepath.Base(u.Path)
// 		}
// 		if name == "" || name == "." || name == "/" {
// 			name = fmt.Sprintf("live_%d.mp4", time.Now().Unix())
// 		}
// 	}
// 	if !strings.HasSuffix(name, ".mp4") && !strings.HasSuffix(name, ".ts") && !strings.HasSuffix(name, ".flv") && !strings.HasSuffix(name, ".mkv") {
// 		name += ".mp4"
// 	}

// 	reqExtra := &gopeedstream.ReqExtra{
// 		Header: make(map[string]string),
// 	}
// 	if body.UserAgent != "" {
// 		reqExtra.Header["User-Agent"] = body.UserAgent
// 	}
// 	for k, v := range body.Headers {
// 		reqExtra.Header[k] = v
// 	}

// 	id, err := c.downloader.CreateDirect(
// 		&base.Request{
// 			URL:   body.Url,
// 			Extra: reqExtra,
// 			Labels: map[string]string{
// 				"type": "live",
// 			},
// 		},
// 		&base.Options{
// 			Name: name,
// 			Path: c.cfg.DownloadDir,
// 		},
// 	)
// 	if err != nil {
// 		result.Err(ctx, 500, "创建任务失败: "+err.Error())
// 		return
// 	}
// 	c.channels.Broadcast(APIClientWSMessage{
// 		Type: "tasks",
// 		Data: c.downloader.GetTasks(),
// 	})
// 	result.Ok(ctx, gin.H{"id": id})
// }

// func (c *DownloadClient) handleBatchCreateTask(ctx *gin.Context) {
// 	var body struct {
// 		Feeds []FeedDownloadTaskBody `json:"feeds"`
// 	}
// 	if err := ctx.ShouldBindJSON(&body); err != nil {
// 		result.Err(ctx, 400, "不合法的参数")
// 		return
// 	}
// 	tasks := c.downloader.GetTasks()
// 	var items []map[string]string
// 	for _, req := range body.Feeds {
// 		if c.check_existing_feed(tasks, &req) {
// 			continue
// 		}
// 		items = append(items, map[string]string{
// 			"name":   req.Filename,
// 			"id":     req.Id,
// 			"url":    req.URL,
// 			"title":  req.Title,
// 			"key":    strconv.Itoa(req.Key),
// 			"suffix": req.Suffix,
// 		})
// 	}
// 	if len(items) == 0 {
// 		result.Ok(ctx, gin.H{"ids": []string{}})
// 		return
// 	}
// 	processed_reqs, err := util.ProcessFilenames(items, c.cfg.DownloadDir)
// 	if err != nil {
// 		result.Err(ctx, 500, "文件名处理失败: "+err.Error())
// 		return
// 	}
// 	task := base.CreateTaskBatch{}
// 	for _, item := range processed_reqs {
// 		url := item["url"]
// 		full_path := item["full_path"]
// 		// 从 full_path 中提取目录
// 		rel_dir := filepath.Dir(full_path)

// 		connections := c.resolveConnections(url)

// 		task.Reqs = append(task.Reqs, &base.CreateTaskBatchItem{
// 			Req: &base.Request{
// 				URL: url,
// 				Labels: map[string]string{
// 					"id":     item["id"],
// 					"title":  item["title"],
// 					"key":    item["key"],
// 					"suffix": item["suffix"],
// 				},
// 			},
// 			Opts: &base.Options{
// 				Name: item["name"] + item["suffix"],
// 				Path: filepath.Join(c.cfg.DownloadDir, rel_dir),
// 				Extra: &gopeedhttp.OptsExtra{
// 					Connections: connections,
// 				},
// 			},
// 		})
// 	}
// 	ids, err := c.downloader.CreateDirectBatch(&task)
// 	if err != nil {
// 		result.Err(ctx, 500, "创建失败")
// 		return
// 	}
// 	c.channels.Broadcast(APIClientWSMessage{
// 		Type: "tasks",
// 		Data: c.downloader.GetTasks(),
// 	})
// 	result.Ok(ctx, gin.H{"ids": ids})
// }

// func (c *DownloadClient) handleStartTask(ctx *gin.Context) {
// 	var body struct {
// 		Id string `json:"id"`
// 	}
// 	if err := ctx.ShouldBindJSON(&body); err != nil {
// 		result.Err(ctx, 400, "不合法的参数")
// 		return
// 	}
// 	if body.Id == "" {
// 		result.Err(ctx, 400, "缺少 feed id 参数")
// 		return
// 	}
// 	c.downloader.Continue(&downloadpkg.TaskFilter{
// 		IDs: []string{body.Id},
// 	})
// 	result.Ok(ctx, gin.H{"id": body.Id})
// }

// func (c *DownloadClient) handlePauseTask(ctx *gin.Context) {
// 	var body struct {
// 		Id string `json:"id"`
// 	}
// 	if err := ctx.ShouldBindJSON(&body); err != nil {
// 		result.Err(ctx, 400, "不合法的参数")
// 		return
// 	}
// 	if body.Id == "" {
// 		result.Err(ctx, 400, "缺少 feed id 参数")
// 		return
// 	}
// 	c.downloader.Pause(&downloadpkg.TaskFilter{
// 		IDs: []string{body.Id},
// 	})
// 	result.Ok(ctx, gin.H{"id": body.Id})
// }

// func (c *DownloadClient) handleResumeTask(ctx *gin.Context) {
// 	var body struct {
// 		Id string `json:"id"`
// 	}
// 	if err := ctx.ShouldBindJSON(&body); err != nil {
// 		result.Err(ctx, 400, "不合法的参数")
// 		return
// 	}
// 	if body.Id == "" {
// 		result.Err(ctx, 400, "缺少 feed id 参数")
// 		return
// 	}
// 	c.downloader.Continue(&downloadpkg.TaskFilter{
// 		IDs: []string{body.Id},
// 	})
// 	result.Ok(ctx, gin.H{"id": body.Id})
// }

// func (c *DownloadClient) handleDeleteTask(ctx *gin.Context) {
// 	var body struct {
// 		Id string `json:"id"`
// 	}
// 	if err := ctx.ShouldBindJSON(&body); err != nil {
// 		result.Err(ctx, 400, "不合法的参数")
// 		return
// 	}
// 	if body.Id == "" {
// 		result.Err(ctx, 400, "缺少 feed id 参数")
// 		return
// 	}
// 	task := c.downloader.GetTask(body.Id)
// 	c.downloader.Delete(&downloadpkg.TaskFilter{
// 		IDs: []string{body.Id},
// 	}, true)
// 	c.channels.Broadcast(APIClientWSMessage{
// 		Type: "event",
// 		Data: map[string]any{
// 			"Type": "delete",
// 			"Task": task,
// 		},
// 	})
// 	result.Ok(ctx, gin.H{"id": body.Id})
// }

// func (c *DownloadClient) handleClearTasks(ctx *gin.Context) {
// 	c.downloader.Delete(nil, true)
// 	c.channels.Broadcast(APIClientWSMessage{
// 		Type: "clear",
// 		Data: c.downloader.GetTasks(),
// 	})
// 	result.Ok(ctx, nil)
// }
