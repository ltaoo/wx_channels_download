package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	downloadpkg "github.com/GopeedLab/gopeed/pkg/download"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"wx_channel/internal/api/types"
)

var channels_ws_upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (c *APIClient) handleChannelsWebsocket(ctx *gin.Context) {
	conn, err := channels_ws_upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
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

func (wc *APIClient) Validate() error {
	// wc.clientsMu.Lock()
	// defer wc.clientsMu.Unlock()
	if len(wc.ws_clients) == 0 {
		return errors.New("请先初始化客户端 socket 连接")
	}
	return nil
}
func (c *APIClient) RequestAPI(endpoint string, body interface{}, timeout time.Duration) (*ClientWebsocketResponse, error) {
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

func (c *APIClient) FetchChannelsFeedListOfContact(username, next_marker string) (*types.ChannelsFeedListOfAccountResp, error) {
	// fmt.Println("[API]fetch feed list of contact", username)
	// cache_key := "feed:" + username
	// if val, found := c.cache.Get(cache_key); found {
	// 	if resp, ok := val.(*types.ChannelsFeedListOfAccountResp); ok {
	// 		return resp, nil
	// 	}
	// }
	resp, err := c.RequestAPI("/api/contact/feed/list", types.ChannelsFeedListBody{Username: username, NextMarker: next_marker}, 10*time.Second)
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
