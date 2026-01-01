package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GopeedLab/gopeed/pkg/base"
	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"wx_channel/internal/api/types"
	"wx_channel/internal/assets"
	"wx_channel/internal/interceptor"
	"wx_channel/pkg/cache"
	"wx_channel/pkg/decrypt"
	"wx_channel/pkg/system"
	"wx_channel/pkg/util"
)

var ws_upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	hub  *APIClient
	conn *websocket.Conn
	send chan []byte
}

func (c *Client) writePump() {
	ticker := time.NewTicker(5 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

type APIClient struct {
	decryptor   *ChannelsVideoDecryptor
	downloader  *downloadpkg.Downloader
	Interceptor *interceptor.Interceptor
	formatter   *util.FilenameProcessor
	cfg         *APIConfig
	ws_clients  map[*Client]bool
	ws_mu       sync.RWMutex
	engine      *gin.Engine
	requests    map[string]chan ChannelsWSResponse
	requests_mu sync.RWMutex
	cache       *cache.Cache
	req_seq     uint64
}

func NewAPIClient(cfg *APIConfig) *APIClient {
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
		ws_clients: make(map[*Client]bool),
		requests:   make(map[string]chan ChannelsWSResponse),
		engine:     gin.Default(),
		cache:      cache.New(),
		req_seq:    uint64(time.Now().UnixNano()),
	}
	client.setupRoutes()
	return client
}

type APIClientWSMessage struct {
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
		c.broadcast(APIClientWSMessage{
			Type: "event",
			Data: evt,
		})
		if evt.Key == downloadpkg.EventKeyDone {
			go assets.PlayDoneAudio()
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
	for client := range c.ws_clients {
		close(client.send)
		delete(c.ws_clients, client)
	}
	c.ws_mu.Unlock()
	return nil
}

func (c *APIClient) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.engine.ServeHTTP(w, r)
}

func (c *APIClient) handleWS(ctx *gin.Context) {
	conn, err := ws_upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	c.ws_mu.Lock()
	client := &Client{hub: c, conn: conn, send: make(chan []byte, 256)}
	c.ws_clients[client] = true
	c.ws_mu.Unlock()

	go client.writePump()

	// Initial tasks
	tasks := c.downloader.GetTasks()
	if data, err := json.Marshal(APIClientWSMessage{Type: "tasks", Data: tasks}); err == nil {
		client.send <- data
	}

	defer func() {
		c.ws_mu.Lock()
		if _, ok := c.ws_clients[client]; ok {
			delete(c.ws_clients, client)
			close(client.send)
		}
		c.ws_mu.Unlock()
		conn.Close()
	}()
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return
		}
		// 前端「响应」给 ws api 请求的响应值
		var resp ChannelsWSResponse
		if err := json.Unmarshal(message, &resp); err == nil && resp.Id != "" {
			c.requests_mu.RLock()
			ch, ok := c.requests[resp.Id]
			c.requests_mu.RUnlock()
			if ok {
				ch <- resp
			}
		}
	}
}

func (wc *APIClient) Validate() error {
	// wc.clientsMu.Lock()
	// defer wc.clientsMu.Unlock()
	if len(wc.ws_clients) == 0 {
		return errors.New("请先初始化客户端 socket 连接")
	}
	return nil
}

type ChannelsWSRequestBody struct {
	ID   string      `json:"id"`
	Key  string      `json:"key"`
	Body interface{} `json:"data"`
}
type ChannelsWSResponse struct {
	Id string `json:"id"`
	// 调用 wx api 原始响应
	Data json.RawMessage `json:"data"`
}

func (c *APIClient) RequestAPI(endpoint string, body interface{}, timeout time.Duration) (*ChannelsWSResponse, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	id := strconv.FormatUint(atomic.AddUint64(&c.req_seq, 1), 10)
	req := ChannelsWSRequestBody{
		ID:   id,
		Key:  endpoint,
		Body: body,
	}
	msg := APIClientWSMessage{
		Type: "api_call",
		Data: req,
	}
	resp_chan := make(chan ChannelsWSResponse, 1)
	c.requests_mu.Lock()
	c.requests[id] = resp_chan
	c.requests_mu.Unlock()
	defer func() {
		c.requests_mu.Lock()
		delete(c.requests, id)
		c.requests_mu.Unlock()
	}()
	c.ws_mu.Lock()
	var client *Client
	for c := range c.ws_clients {
		client = c
		break
	}
	if client == nil {
		c.ws_mu.Unlock()
		return nil, errors.New("没有可用的客户端")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		c.ws_mu.Unlock()
		return nil, err
	}

	select {
	case client.send <- data:
	default:
		c.ws_mu.Unlock()
		return nil, errors.New("发送缓冲区已满")
	}
	c.ws_mu.Unlock()
	select {
	case resp := <-resp_chan:
		return &resp, nil
	case <-time.After(timeout):
		return nil, errors.New("请求超时")
	}
}

func (c *APIClient) SearchChannelsContact(keyword string) (*types.ChannelsContactSearchResp, error) {
	cache_key := "search:" + keyword
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(*types.ChannelsContactSearchResp); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestAPI("/api/contact/search", types.ChannelsAccountSearchBody{Keyword: keyword}, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var r types.ChannelsContactSearchResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, &r, 5*time.Minute)
	return &r, nil
}

func (c *APIClient) FetchChannelsFeedListOfContact(username string) (*types.ChannelsFeedListOfAccountResp, error) {
	// fmt.Println("[API]fetch feed list of contact", username)
	// cache_key := "feed:" + username
	// if val, found := c.cache.Get(cache_key); found {
	// 	if resp, ok := val.(*types.ChannelsFeedListOfAccountResp); ok {
	// 		return resp, nil
	// 	}
	// }
	resp, err := c.RequestAPI("/api/contact/feed/list", types.ChannelsFeedListBody{Username: username}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var r types.ChannelsFeedListOfAccountResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	// c.cache.Set(cache_key, &r, 5*time.Minute)
	return &r, nil
}

func (c *APIClient) FetchChannelsFeedProfile(oid, uid, url string) (*types.ChannelsFeedProfileResp, error) {
	// fmt.Println("[API]fetch feed profile", oid, uid)
	// cache_key := "feed:" + username
	// if val, found := c.cache.Get(cache_key); found {
	// 	if resp, ok := val.(*types.ChannelsFeedProfileResp); ok {
	// 		return resp, nil
	// 	}
	// }
	resp, err := c.RequestAPI("/api/feed/profile", types.ChannelsFeedProfileBody{ObjectId: oid, NonceId: uid, URL: url}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var r types.ChannelsFeedProfileResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	// c.cache.Set(cache_key, &r, 5*time.Minute)
	return &r, nil
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

func (c *APIClient) broadcast(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	c.ws_mu.Lock()
	defer c.ws_mu.Unlock()
	for client := range c.ws_clients {
		select {
		case client.send <- data:
		default:
			close(client.send)
			delete(c.ws_clients, client)
		}
	}
}
