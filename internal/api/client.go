package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	"github.com/gorilla/websocket"
)

type APIClient struct {
	decryptor   *ChannelsVideoDecryptor
	downloader  *downloadpkg.Downloader
	cfg         *APISettings
	ws_upgrader websocket.Upgrader
	ws_clients  map[*websocket.Conn]struct{}
	ws_mu       sync.RWMutex
}

func NewAPIClient(cfg *APISettings) *APIClient {
	data_dir := cfg.RootDir
	downloader := downloadpkg.NewDownloader(&downloadpkg.DownloaderConfig{
		RefreshInterval: 360,
		Storage:         downloadpkg.NewMemStorage(),
		StorageDir:      data_dir,
	})
	return &APIClient{
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
	}
}

type DownloaderWSMessage struct {
	Type  string      `json:"type"`
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}

func (c *APIClient) Start() error {
	if err := c.downloader.Setup(); err != nil {
		return err
	}
	_ = c.downloader.PutConfig(&base.DownloaderStoreConfig{
		DownloadDir:    c.cfg.DownloadDir,
		MaxRunning:     c.cfg.MaxRunning,
		ProtocolConfig: map[string]any{},
		Extra:          map[string]any{},
		Proxy:          &base.DownloaderProxyConfig{},
		Webhook:        &base.WebhookConfig{},
	})
	c.downloader.Listener(func(evt *downloadpkg.Event) {
		if evt == nil || evt.Task == nil || evt.Task.ID == "" {
			return
		}
		c.broadcast(DownloaderWSMessage{
			Type: string(evt.Key),
			Data: evt.Task,
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
	if r.URL.Path == "/ws" {
		c.handleWS(w, r)
		return
	}
	if r.URL.Path == "/api/task/create" {
		c.handleCreateTask(w, r)
		return
	}
	c.decryptor.ServeHTTP(w, r)
}

func (c *APIClient) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := c.ws_upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "upgrade failed", http.StatusBadRequest)
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
	Meta     string `json:"meta"`
}

func (c *APIClient) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body CreateTaskReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	id, err := c.downloader.CreateDirect(&base.Request{
		URL: body.URL,
	}, &base.Options{
		Name: body.Filename,
		Extra: map[string]any{
			"key": body.Key,
		},
	})
	if err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": id,
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
