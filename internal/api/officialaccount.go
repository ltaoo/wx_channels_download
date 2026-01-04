package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
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
	// resp, err := c.RequestAPI("/api/official_account/fetch_account_home", body, 15*time.Second)
	// if err != nil {
	// 	c.jsonError(ctx, 500, err.Error())
	// 	return
	// }
	// var data interface{}
	// if err := json.Unmarshal(resp.Data, &data); err != nil {
	// 	c.jsonError(ctx, 500, "Invalid response data")
	// 	return
	// }
	data, err := c.fake_curl(body.Biz)
	if err != nil {
		c.jsonError(ctx, 500, err.Error())
		return
	}
	c.jsonSuccess(ctx, string(data))
}

func (c *APIClient) fake_curl(biz string) ([]byte, error) {
	var wxuin string
	var cookieParts []string
	for _, cookie := range c.Cookies {
		if cookie.Name == "wxuin" {
			wxuin = cookie.Value
		}
		cookieParts = append(cookieParts, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}
	encodedUin := ""
	if wxuin != "" {
		encodedUin = base64.StdEncoding.EncodeToString([]byte(wxuin))
	}

	targetURL := fmt.Sprintf("https://mp.weixin.qq.com/mp/profile_ext?action=home&__biz=%s&scene=124&uin=%s", biz, url.QueryEscape(encodedUin))
	cookieHeader := strings.Join(cookieParts, "; ")

	args := []string{
		"-X", "GET", targetURL,
		"-H", "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/wxpic,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"-H", "Accept-Language: en-US,en;q=0.9",
		"-H", fmt.Sprintf("Cookie: %s", cookieHeader),
		"-H", "Priority: u=0, i",
		"-H", fmt.Sprintf("Referer: %s", targetURL),
		"-H", "Sec-Fetch-Dest: document",
		"-H", "Sec-Fetch-Mode: navigate",
		"-H", "Sec-Fetch-Site: same-origin",
		"-H", "Upgrade-Insecure-Requests: 1",
		"-H", "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 NetType/WIFI MicroMessenger/7.0.20.1781(0x6700143B) MacWechat/3.8.7(0x13080712) UnifiedPCMacWechat(0xf2640619) XWEB/14304 Flue",
		"--compressed",
	}

	cmd := exec.Command("curl", args...)
	return cmd.Output()
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
