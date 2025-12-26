package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"wx_channel/internal/interceptor"
	"wx_channel/pkg/decrypt"
	"wx_channel/pkg/util"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type APIClient struct {
	decryptor   *ChannelsVideoDecryptor
	downloader  *downloadpkg.Downloader
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
				"userAgent":   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			},
		},
		Extra:   map[string]any{},
		Proxy:   &base.DownloaderProxyConfig{},
		Webhook: &base.WebhookConfig{},
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
			task := c.downloader.GetTask(evt.Task.ID)
			k := task.Meta.Req.Labels["key"]
			key, err := strconv.Atoi(k)
			if err != nil {
				return
			}
			if key == 0 {
				return
			}
			file_path := task.Meta.SingleFilepath()
			go func() {
				data, err := os.ReadFile(file_path)
				if err != nil {
					return
				}
				length := uint32(131072)
				_key := uint64(key)
				decrypt.DecryptData(data, length, _key)
				_ = os.WriteFile(file_path, data, 0644)
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
	return c.downloader.Close()
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

	// Inject CSS
	css := readAsset("inject/lib/weui.min.css", files.CSSWeui)
	html = strings.Replace(html, "/* INJECT_CSS */", css, 1)

	// Inject JS
	var inserted_scripts string

	// Inject Config
	// mock config
	config := map[string]interface{}{
		"apiServerAddr": "127.0.0.1:2022",
	}
	cfg_byte, _ := json.Marshal(config)
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

type CreateTaskReq struct {
	Id       string `json:"id"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
	Key      int    `json:"key"`
}

func (c *APIClient) check_existing_feed(tasks []*downloadpkg.Task, id string) bool {
	for _, t := range tasks {
		if t == nil || t.Meta == nil || t.Meta.Req == nil || t.Meta.Req.Labels == nil {
			continue
		}
		if t.Meta.Req.Labels["id"] == id {
			fmt.Println("find matched feed", id)
			fmt.Println(t.Meta.Req.Labels)
			return true
		}
	}
	return false
}
func (c *APIClient) handleCreateTask(ctx *gin.Context) {
	var body CreateTaskReq
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, "invalid json")
		return
	}
	if body.Id == "" {
		c.jsonError(ctx, 400, "missing task id")
		return
	}
	tasks := c.downloader.GetTasks()
	fmt.Println(len(tasks))
	existing := c.check_existing_feed(tasks, body.Id)
	if existing {
		ctx.JSON(http.StatusOK, Response{Code: 409, Msg: "feed already exists", Data: body})
		return
	}
	filename, _, err := c.formatter.ProcessFilename(body.Filename)
	if err != nil {
		c.jsonError(ctx, 409, "invalid filename")
		return
	}
	id, err := c.downloader.CreateDirect(
		&base.Request{
			URL: body.URL,
			Labels: map[string]string{
				"id":    body.Id,
				"title": body.Title,
				"key":   strconv.Itoa(body.Key),
			},
		},
		&base.Options{
			Name: filename + ".mp4",
		},
	)
	if err != nil {
		c.jsonError(ctx, 500, "create failed")
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
		Feeds []CreateTaskReq `json:"feeds"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, "invalid json")
		return
	}
	tasks := c.downloader.GetTasks()
	task := base.CreateTaskBatch{}
	for _, req := range body.Feeds {
		existing := c.check_existing_feed(tasks, req.Id)
		if existing {
			continue
		}
		filename, _, err := c.formatter.ProcessFilename(req.Filename)
		if err != nil {
			continue
		}
		task.Reqs = append(task.Reqs, &base.CreateTaskBatchItem{
			Req: &base.Request{
				URL: req.URL,
				Labels: map[string]string{
					"id":    req.Id,
					"title": req.Title,
					"key":   strconv.Itoa(req.Key),
				},
			},
			Opts: &base.Options{
				Name: filename + ".mp4",
			},
		})
	}
	ids, err := c.downloader.CreateDirectBatch(&task)
	if err != nil {
		c.jsonError(ctx, 500, "create failed")
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
		c.jsonError(ctx, 400, "invalid json")
		return
	}
	if body.Id == "" {
		c.jsonError(ctx, 400, "missing task id")
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
		c.jsonError(ctx, 400, "invalid json")
		return
	}
	if body.Id == "" {
		c.jsonError(ctx, 400, "missing task id")
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
		c.jsonError(ctx, 400, "invalid json")
		return
	}
	if body.Id == "" {
		c.jsonError(ctx, 400, "missing task id")
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
		c.jsonError(ctx, 400, "invalid json")
		return
	}
	if body.Id == "" {
		c.jsonError(ctx, 400, "missing task id")
		return
	}
	c.downloader.Delete(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	}, true)
	c.jsonSuccess(ctx, gin.H{"id": body.Id})
}

func (c *APIClient) handleClearTasks(ctx *gin.Context) {
	c.downloader.Delete(nil, true)
	c.broadcast(DownloaderWSMessage{
		Type: "tasks",
		Data: c.downloader.GetTasks(),
	})
	c.jsonSuccess(ctx, nil)
}

func (c *APIClient) handleOpenDownloadDir(ctx *gin.Context) {
	dir := c.cfg.DownloadDir
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	case "linux":
		cmd = exec.Command("xdg-open", dir)
	default:
		c.jsonError(ctx, 500, "Unsupported operating system")
		return
	}
	if err := cmd.Start(); err != nil {
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
