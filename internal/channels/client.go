package channels

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"wx_channel/internal/api/types"
	"wx_channel/pkg/cache"
)

var channels_ws_upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ChannelsClient struct {
	// decryptor   *ChannelsVideoDecryptor
	// downloader  *downloadpkg.Downloader
	// Interceptor *interceptor.Interceptor
	// official    *officialaccount.OfficialAccountBrowser
	// formatter   *util.FilenameProcessor
	// Cookies     []*http.Cookie
	// cfg         *APIConfig
	ws_clients  map[*Client]bool
	ws_mu       sync.RWMutex
	engine      *gin.Engine
	requests    map[string]chan ClientWebsocketResponse
	requests_mu sync.RWMutex
	cache       *cache.Cache
	req_seq     uint64
}

func NewChannelsClient(addr string) *ChannelsClient {
	return &ChannelsClient{
		// ServerAddr: addr,
		ws_clients: make(map[*Client]bool),
		requests:   make(map[string]chan ClientWebsocketResponse),
		// engine:     gin.Default(),
		cache:   cache.New(),
		req_seq: uint64(time.Now().UnixNano()),
	}
}

func (c *ChannelsClient) HandleChannelsWebsocket(ctx *gin.Context) {
	conn, err := channels_ws_upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	c.ws_mu.Lock()
	client := &Client{conn: conn, send: make(chan []byte, 256)}
	c.ws_clients[client] = true
	c.ws_mu.Unlock()

	go client.writePump()

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
		var resp ClientWebsocketResponse
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
func (c *ChannelsClient) Stop() {
	c.ws_mu.Lock()
	for client := range c.ws_clients {
		close(client.send)
		delete(c.ws_clients, client)
	}
	c.ws_mu.Unlock()
}
func (c *ChannelsClient) Broadcast(v interface{}) {
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
func (wc *ChannelsClient) Validate() error {
	// wc.clientsMu.Lock()
	// defer wc.clientsMu.Unlock()
	if len(wc.ws_clients) == 0 {
		return errors.New("请先初始化客户端 socket 连接")
	}
	return nil
}
func (c *ChannelsClient) RequestFrontend(endpoint string, body interface{}, timeout time.Duration) (*ClientWebsocketResponse, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	id := strconv.FormatUint(atomic.AddUint64(&c.req_seq, 1), 10)
	req := ClientWebsocketRequestBody{
		ID:   id,
		Key:  endpoint,
		Body: body,
	}
	msg := APIClientWSMessage{
		Type: "api_call",
		Data: req,
	}
	resp_chan := make(chan ClientWebsocketResponse, 1)
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

func (c *ChannelsClient) SearchChannelsContact(keyword string) (*types.ChannelsContactSearchResp, error) {
	cache_key := "search:" + keyword
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(*types.ChannelsContactSearchResp); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:contact_list", types.ChannelsAccountSearchBody{Keyword: keyword}, 20*time.Second)
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

func (c *ChannelsClient) FetchChannelsFeedListOfContact(username, next_marker string) (*types.ChannelsFeedListOfAccountResp, error) {
	// fmt.Println("[API]fetch feed list of contact", username)
	// cache_key := "feed:" + username
	// if val, found := c.cache.Get(cache_key); found {
	// 	if resp, ok := val.(*types.ChannelsFeedListOfAccountResp); ok {
	// 		return resp, nil
	// 	}
	// }
	resp, err := c.RequestFrontend("key:channels:feed_list", types.ChannelsFeedListBody{Username: username, NextMarker: next_marker}, 10*time.Second)
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

func (c *ChannelsClient) FetchChannelsFeedProfile(oid, uid, url string) (*types.ChannelsFeedProfileResp, error) {
	// fmt.Println("[API]fetch feed profile", oid, uid)
	// cache_key := "feed:" + username
	// if val, found := c.cache.Get(cache_key); found {
	// 	if resp, ok := val.(*types.ChannelsFeedProfileResp); ok {
	// 		return resp, nil
	// 	}
	// }
	resp, err := c.RequestFrontend("key:channels:feed_profile", types.ChannelsFeedProfileBody{ObjectId: oid, NonceId: uid, URL: url}, 10*time.Second)
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
