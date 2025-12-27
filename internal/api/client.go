package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	gopeedhttp "github.com/GopeedLab/gopeed/pkg/protocol/http"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"wx_channel/internal/assets"
	"wx_channel/internal/interceptor"
	"wx_channel/pkg/decrypt"
	"wx_channel/pkg/system"
	"wx_channel/pkg/util"
)

type APIClient struct {
	decryptor   *ChannelsVideoDecryptor
	downloader  *downloadpkg.Downloader
	Interceptor *interceptor.Interceptor
	formatter   *util.FilenameProcessor
	cfg         *APISettings
	ws_upgrader websocket.Upgrader
	ws_clients  map[*websocket.Conn]struct{}
	ws_mu       sync.RWMutex
	engine      *gin.Engine
}

func NewAPIClient(cfg *APISettings) *APIClient {
	data_dir := cfg.RootDir
	downloader := downloadpkg.NewDownloader(&downloadpkg.DownloaderConfig{
		RefreshInterval: 360,
		Storage:         downloadpkg.NewBoltStorage(data_dir),
		StorageDir:      data_dir,
	})
	client := &APIClient{
		decryptor:  NewChannelsVideoDecryptor(),
		downloader: downloader,
		formatter:  util.NewFilenameProcessor(cfg.DownloadDir),
		cfg:        cfg,
		ws_upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		ws_clients: make(map[*websocket.Conn]struct{}),
		engine:     gin.Default(),
	}
	client.setupRoutes()
	return client
}

func (c *APIClient) setupRoutes() {
	c.engine.Use(func(ctx *gin.Context) {
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		ctx.Header("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Access-Control-Request-Private-Network")
		ctx.Header("Access-Control-Allow-Private-Network", "true")

		if ctx.Request.Method == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusOK)
			return
		}
		ctx.Next()
	})

	c.engine.GET("/ws", c.handleWS)
	c.engine.POST("/api/task/create", c.handleCreateTask)
	c.engine.POST("/api/task/create_batch", c.handleBatchCreateTask)
	c.engine.POST("/api/task/start", c.handleStartTask)
	c.engine.POST("/api/task/pause", c.handlePauseTask)
	c.engine.POST("/api/task/resume", c.handleResumeTask)
	c.engine.POST("/api/task/delete", c.handleDeleteTask)
	c.engine.POST("/api/show_file", c.handleHighlightFileInFolder)
	c.engine.POST("/api/task/clear", c.handleClearTasks)
	c.engine.POST("/api/open_download_dir", c.handleOpenDownloadDir)

	c.engine.NoRoute(func(ctx *gin.Context) {
		c.handleIndex(ctx)
		// c.decryptor.ServeHTTP(ctx.Writer, ctx.Request)
	})
}

type DownloaderWSMessage struct {
	Type  string      `json:"type"`
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}

type ClientWSMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

func (c *APIClient) Start() error {
	if err := c.downloader.Setup(); err != nil {
		return err
	}
	_ = c.downloader.PutConfig(&base.DownloaderStoreConfig{
		DownloadDir: c.cfg.DownloadDir,
		MaxRunning:  c.cfg.MaxRunning,
		ProtocolConfig: map[string]any{
			"http": map[string]any{
				"connections": 4,
			},
		},
		Extra: map[string]any{},
		Proxy: &base.DownloaderProxyConfig{},
	})

	c.downloader.Listener(func(evt *downloadpkg.Event) {
		if evt == nil || evt.Task == nil || evt.Task.ID == "" {
			return
		}
		c.broadcast(DownloaderWSMessage{
			Type: "event",
			Data: evt,
		})
		if evt.Key == downloadpkg.EventKeyDone {
			assets.PlayDoneAudio()
			task := c.downloader.GetTask(evt.Task.ID)
			file_path := task.Meta.SingleFilepath()
			go func() {
				k := task.Meta.Req.Labels["key"]
				if k != "" {
					key, err := strconv.Atoi(k)
					if err == nil {
						if key != 0 {
							data, err := os.ReadFile(file_path)
							if err == nil {
								length := uint32(131072)
								_key := uint64(key)
								decrypt.DecryptData(data, length, _key)
								_ = os.WriteFile(file_path, data, 0644)
							}
						}
					}
				}
				suffix := task.Meta.Req.Labels["suffix"]
				if suffix == ".mp3" {
					temp_path := file_path + ".temp"
					if err := os.Rename(file_path, temp_path); err == nil {
						if err := system.RunCommand("ffmpeg", "-i", temp_path, "-vn", "-acodec", "libmp3lame", "-ab", "192k", "-f", "mp3", file_path); err == nil {
							_ = os.Remove(temp_path)
						} else {
							_ = os.Rename(temp_path, file_path)
						}
					}
				}
			}()
		}
	})
	return nil
}

func (c *APIClient) Stop() error {
	c.ws_mu.Lock()
	for conn := range c.ws_clients {
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(1*time.Second))
		_ = conn.Close()
		delete(c.ws_clients, conn)
	}
	c.ws_mu.Unlock()
	return nil
}

func (c *APIClient) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.engine.ServeHTTP(w, r)
}

func (c *APIClient) handleWS(ctx *gin.Context) {
	conn, err := c.ws_upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	c.ws_mu.Lock()
	c.ws_clients[conn] = struct{}{}
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_ = conn.WriteJSON(DownloaderWSMessage{Type: "tasks", Data: c.downloader.GetTasks()})
	c.ws_mu.Unlock()

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.ws_mu.Lock()
				if _, ok := c.ws_clients[conn]; !ok {
					c.ws_mu.Unlock()
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					c.ws_mu.Unlock()
					return
				}
				c.ws_mu.Unlock()
			}
		}
	}()

	go func() {
		defer func() {
			c.ws_mu.Lock()
			delete(c.ws_clients, conn)
			c.ws_mu.Unlock()
			_ = conn.Close()
		}()
		conn.SetReadLimit(1024)
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()
}

func (c *APIClient) handleIndex(ctx *gin.Context) {
	readAsset := func(path string, defaultData []byte) string {
		fullPath := filepath.Join("internal", "interceptor", path)
		data, err := os.ReadFile(fullPath)
		if err == nil {
			return string(data)
		}
		return string(defaultData)
	}
	html := readAsset("inject/index.html", interceptor.Assets.IndexHTML)
	files := interceptor.Assets
	css := readAsset("inject/lib/weui.min.css", files.CSSWeui)
	html = strings.Replace(html, "/* INJECT_CSS */", css, 1)
	var inserted_scripts string
	cfg_byte, _ := json.Marshal(c.cfg)
	inserted_scripts += fmt.Sprintf(`<script>var __wx_channels_config__ = %s; var __wx_channels_version__ = "local";</script>`, string(cfg_byte))

	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/lib/mitt.umd.js", files.JSMitt))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/src/eventbus.js", files.JSEventBus))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/src/utils.js", files.JSUtils))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/lib/floating-ui.core.1.7.4.min.js", files.JSFloatingUICore))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/lib/floating-ui.dom.1.7.4.min.js", files.JSFloatingUIDOM))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/lib/weui.min.js", files.JSWeui))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/lib/wui.umd.js", files.JSWui))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/src/components.js", files.JSComponents))
	inserted_scripts += fmt.Sprintf(`<script>%s</script>`, readAsset("inject/src/downloader.js", files.JSDownloader))

	html = strings.Replace(html, "<!-- INJECT_JS -->", inserted_scripts, 1)

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, html)
}
func (c *APIClient) check_existing_feed(tasks []*downloadpkg.Task, body *FeedDownloadTaskBody) bool {
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil || t.Meta.Req.Labels == nil {
			continue
		}
		same_id := t.Meta.Req.Labels["id"] == body.Id
		same_spec := t.Meta.Req.Labels["spec"] == body.Spec
		same_suffix := t.Meta.Req.Labels["suffix"] == body.Suffix
		if same_id && same_spec && same_suffix {
			return true
		}
	}
	return false
}

type FeedDownloadTaskBody struct {
	Id       string `json:"id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
	Key      int    `json:"key"`
	Spec     string `json:"spec"`
	Suffix   string `json:"suffix"`
}

func (c *APIClient) handleCreateTask(ctx *gin.Context) {
	var body FeedDownloadTaskBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		c.jsonError(ctx, 400, "缺少 feed id 参数")
		return
	}
	if body.Suffix == ".mp3" {
		has_ffmpeg := system.ExistingCommand("ffmpeg")
		if !has_ffmpeg {
			c.jsonError(ctx, 400, "下载 mp3 需要支持 ffmpeg 命令")
			return
		}
	}
	tasks := c.downloader.GetTasks()
	existing := c.check_existing_feed(tasks, &body)
	if existing {
		ctx.JSON(http.StatusOK, Response{Code: 409, Msg: "已存在该下载内容", Data: body})
		return
	}
	fmt.Print("before ProcessFilename", body.Filename)
	filename, dir, err := c.formatter.ProcessFilename(body.Filename)
	fmt.Print("after ProcessFilename", filename, dir)
	if err != nil {
		c.jsonError(ctx, 409, "不合法的文件名")
		return
	}
	connections := c.resolveConnections(body.URL)
	id, err := c.downloader.CreateDirect(
		&base.Request{
			URL: body.URL,
			Labels: map[string]string{
				"id":     body.Id,
				"title":  body.Title,
				"key":    strconv.Itoa(body.Key),
				"spec":   body.Spec,
				"suffix": body.Suffix,
			},
		},
		&base.Options{
			Name: filename + body.Suffix,
			Path: filepath.Join(c.cfg.DownloadDir, dir),
			Extra: &gopeedhttp.OptsExtra{
				Connections: connections,
			},
		},
	)
	if err != nil {
		c.jsonError(ctx, 500, "下载失败")
		return
	}
	c.broadcast(DownloaderWSMessage{
		Type: "tasks",
		Data: c.downloader.GetTasks(),
	})
	c.jsonSuccess(ctx, gin.H{"id": id})
}

func (c *APIClient) handleBatchCreateTask(ctx *gin.Context) {
	var body struct {
		Feeds []FeedDownloadTaskBody `json:"feeds"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, "不合法的参数")
		return
	}
	tasks := c.downloader.GetTasks()
	var items []map[string]string
	for _, req := range body.Feeds {
		if c.check_existing_feed(tasks, &req) {
			continue
		}
		items = append(items, map[string]string{
			"name":   req.Filename,
			"id":     req.Id,
			"url":    req.URL,
			"title":  req.Title,
			"key":    strconv.Itoa(req.Key),
			"suffix": req.Suffix,
		})
	}
	if len(items) == 0 {
		c.jsonSuccess(ctx, gin.H{"ids": []string{}})
		return
	}
	processed_reqs, err := util.ProcessFilenames(items, c.cfg.DownloadDir)
	if err != nil {
		c.jsonError(ctx, 500, "文件名处理失败: "+err.Error())
		return
	}
	task := base.CreateTaskBatch{}
	for _, item := range processed_reqs {
		url := item["url"]
		fullPath := item["full_path"]
		// 从 full_path 中提取目录
		relDir := filepath.Dir(fullPath)

		connections := c.resolveConnections(url)

		task.Reqs = append(task.Reqs, &base.CreateTaskBatchItem{
			Req: &base.Request{
				URL: url,
				Labels: map[string]string{
					"id":     item["id"],
					"title":  item["title"],
					"key":    item["key"],
					"suffix": item["suffix"],
				},
			},
			Opts: &base.Options{
				Name: item["name"] + item["suffix"],
				Path: filepath.Join(c.cfg.DownloadDir, relDir),
				Extra: &gopeedhttp.OptsExtra{
					Connections: connections,
				},
			},
		})
	}

	ids, err := c.downloader.CreateDirectBatch(&task)
	if err != nil {
		c.jsonError(ctx, 500, "创建失败")
		return
	}
	c.broadcast(DownloaderWSMessage{
		Type: "tasks",
		Data: c.downloader.GetTasks(),
	})
	c.jsonSuccess(ctx, gin.H{"ids": ids})
}

func (c *APIClient) handleStartTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		c.jsonError(ctx, 400, "缺少 feed id 参数")
		return
	}
	c.downloader.Continue(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	})
	c.jsonSuccess(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handlePauseTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		c.jsonError(ctx, 400, "缺少 feed id 参数")
		return
	}
	c.downloader.Pause(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	})
	c.jsonSuccess(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleResumeTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		c.jsonError(ctx, 400, "缺少 feed id 参数")
		return
	}
	c.downloader.Continue(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	})
	c.jsonSuccess(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleDeleteTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, "不合法的参数")
		return
	}
	if body.Id == "" {
		c.jsonError(ctx, 400, "缺少 feed id 参数")
		return
	}
	task := c.downloader.GetTask(body.Id)
	c.downloader.Delete(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	}, true)
	c.broadcast(DownloaderWSMessage{
		Type: "event",
		Data: map[string]any{
			"Type": "delete",
			"Task": task,
		},
	})
	c.jsonSuccess(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleClearTasks(ctx *gin.Context) {
	c.downloader.Delete(nil, true)
	c.broadcast(DownloaderWSMessage{
		Type: "clear",
		Data: c.downloader.GetTasks(),
	})
	c.jsonSuccess(ctx, nil)
}

func (c *APIClient) resolveConnections(url string) int {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Head(url)
	if err != nil {
		return 1
	}
	defer resp.Body.Close()

	if resp.ContentLength > 0 && resp.ContentLength < 1024*1024 {
		return 1
	}
	return 4
}

func (c *APIClient) handleOpenDownloadDir(ctx *gin.Context) {
	dir := c.cfg.DownloadDir
	if err := system.Open(dir); err != nil {
		c.jsonError(ctx, 500, err.Error())
		return
	}
	c.jsonSuccess(ctx, nil)
}

func (c *APIClient) broadcast(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	c.ws_mu.Lock()
	defer c.ws_mu.Unlock()
	for conn := range c.ws_clients {
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			_ = conn.Close()
			delete(c.ws_clients, conn)
		}
	}
}
