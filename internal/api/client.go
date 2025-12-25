package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type APIClient struct {
	decryptor   *ChannelsVideoDecryptor
	downloader  *downloadpkg.Downloader
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
		Storage:         downloadpkg.NewMemStorage(),
		StorageDir:      data_dir,
	})
	client := &APIClient{
		decryptor:  NewChannelsVideoDecryptor(),
		downloader: downloader,
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
	c.engine.POST("/api/task/start", c.handleStartTask)

	c.engine.NoRoute(func(ctx *gin.Context) {
		c.decryptor.ServeHTTP(ctx.Writer, ctx.Request)
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
	c.ws_mu.Unlock()
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
		_ = conn.WriteJSON(DownloaderWSMessage{Type: "tasks", Data: c.downloader.GetTasks()})
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()
}

type CreateTaskReq struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
	Key      string `json:"key"`
	// Meta     string `json:"meta"`
}

func (c *APIClient) handleCreateTask(ctx *gin.Context) {
	var body CreateTaskReq
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	id, err := c.downloader.CreateDirect(
		&base.Request{URL: body.URL},
		&base.Options{
			Name: body.Filename + ".mp4",
			Path: c.cfg.DownloadDir,
			Extra: map[string]any{
				"key": body.Key,
			},
		},
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "create failed"})
		return
	}
	c.broadcast(DownloaderWSMessage{
		Type: "tasks",
		Data: c.downloader.GetTasks(),
	})
	ctx.JSON(http.StatusOK, gin.H{
		"id": id,
	})
}

func (c *APIClient) handleStartTask(ctx *gin.Context) {
	var body struct {
		Id string `json:"id"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if body.Id == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "missing task id"})
		return
	}
	c.downloader.Continue(&downloadpkg.TaskFilter{
		IDs: []string{body.Id},
	})
	ctx.JSON(http.StatusOK, gin.H{
		"id": body.Id,
	})
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
