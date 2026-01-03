package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var official_ws_upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (c *APIClient) handleFetchOfficialAccountMsgList(ctx *gin.Context) {
	var body struct {
		Biz string `json:"biz"`
		Uin string `json:"uin"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, err.Error())
		return
	}
	resp, err := c.RequestAPI("/api/official_account/fetch_msg_list", body, 15*time.Second)
	if err != nil {
		c.jsonError(ctx, 500, err.Error())
		return
	}
	var data interface{}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		c.jsonError(ctx, 500, "Invalid response data")
		return
	}
	c.jsonSuccess(ctx, data)
}

func (c *APIClient) handleFetchOfficialAccountHome(ctx *gin.Context) {
	var body struct {
		Biz string `json:"biz"`
		Uin string `json:"uin"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, err.Error())
		return
	}
	resp, err := c.RequestAPI("/api/official_account/fetch_account_home", body, 15*time.Second)
	if err != nil {
		c.jsonError(ctx, 500, err.Error())
		return
	}
	var data interface{}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		c.jsonError(ctx, 500, "Invalid response data")
		return
	}
	c.jsonSuccess(ctx, data)
}

func (c *APIClient) handleOfficialAccountWebsocket(ctx *gin.Context) {
	conn, err := official_ws_upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	c.ws_mu.Lock()
	client := &Client{hub: c, conn: conn, send: make(chan []byte, 256)}
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
