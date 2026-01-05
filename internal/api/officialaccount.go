package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"
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
var accounts = make(map[string]*OfficialAccount)
var acct_mu sync.RWMutex
var officialTimerOnce sync.Once

type OfficialAccount struct {
	Nickname   string `json:"nickname"`
	AvatarURL  string `json:"avatar_url"`
	Biz        string `json:"biz"`
	Uin        string `json:"uin"`
	Key        string `json:"key"`
	PassTicket string `json:"pass_ticket"`
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

// 添加公众号
func (c *APIClient) handleMonitoringOfficialAccount(ctx *gin.Context) {
	var body OfficialAccount
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, err.Error())
		return
	}
	officialTimerOnce.Do(func() { c.start_official_account_timer() })
	acct, err := c.refresh_account(&body)
	if err != nil {
		c.jsonError(ctx, 500, err.Error())
		return
	}
	acct_mu.Lock()
	accounts[body.Biz] = acct
	acct_mu.Unlock()
	c.jsonSuccess(ctx, acct)
}

// 获取公众号推送列表
func (c *APIClient) handleFetchOfficialAccountMsgList(ctx *gin.Context) {
	var body struct {
		Biz string `json:"biz"`
		Uin string `json:"uin"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, err.Error())
		return
	}
	var existing *OfficialAccount
	acct_mu.RLock()
	if _, ok := accounts[body.Biz]; ok {
		data := accounts[body.Biz]
		existing = data
	}
	acct_mu.RUnlock()
	if existing == nil {
		c.jsonError(ctx, 404, "Please Monitor the Official Account first")
		return
	}
	u, _ := url.Parse("https://mp.weixin.qq.com/mp/profile_ext")
	q := u.Query()
	q.Set("action", "getmsg")
	q.Set("__biz", existing.Biz)
	q.Set("uin", existing.Uin)
	q.Set("key", existing.Key)
	q.Set("pass_ticket", existing.PassTicket)
	q.Set("wxtoken", "")
	q.Set("x5", "0")
	q.Set("count", "10")
	q.Set("offset", "0")
	q.Set("f", "json")
	u.RawQuery = q.Encode()
	target_url := u.String()
	fmt.Println("[OfficialAccountAPI]fetchMsgList - url", target_url)
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", target_url, nil)
	if err != nil {
		c.jsonError(ctx, 500, err.Error())
		return
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/wxpic,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Priority", "u=0, i")
	req.Header.Set("Referer", target_url)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 NetType/WIFI MicroMessenger/7.0.20.1781(0x6700143B) MacWechat/3.8.7(0x13080712) UnifiedPCMacWechat(0xf2640619) XWEB/14304 Flue")
	resp, err := client.Do(req)
	if err != nil {
		c.jsonError(ctx, 500, err.Error())
		return
	}
	defer resp.Body.Close()
	var data interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		c.jsonError(ctx, 500, "Invalid response data")
		return
	}
	c.jsonSuccess(ctx, data)
}

// 获取已添加到公众号列表
func (c *APIClient) handleFetchOfficialAccountList(ctx *gin.Context) {
	var list []*OfficialAccount
	acct_mu.RLock()
	for _, acct := range accounts {
		list = append(list, acct)
	}
	acct_mu.RUnlock()
	c.jsonSuccess(ctx, gin.H{
		"list": list,
	})
}

// 尝试获取公众号详情页html
func (c *APIClient) handleFetchOfficialAccountHome(ctx *gin.Context) {
	var body struct {
		Biz string `json:"biz"`
		Uin string `json:"uin"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.jsonError(ctx, 400, err.Error())
		return
	}
	var existing *OfficialAccount
	acct_mu.RLock()
	if _, ok := accounts[body.Biz]; ok {
		data := accounts[body.Biz]
		existing = data
	}
	acct_mu.RUnlock()
	if existing == nil {
		c.jsonError(ctx, 400, "Please Monitor the Official Account first")
		return
	}
	u, _ := url.Parse("https://mp.weixin.qq.com/mp/profile_ext")
	q := u.Query()
	q.Set("action", "home")
	q.Set("__biz", existing.Biz)
	q.Set("scene", "124")
	q.Set("uin", existing.Uin)
	q.Set("key", existing.Key)
	q.Set("devicetype", "UnifiedPCWindows")
	q.Set("version", "f2541022")
	q.Set("lang", "zh_CN")
	q.Set("a8scene", "1")
	q.Set("acctmode", "0")
	q.Set("pass_ticket", existing.PassTicket)
	u.RawQuery = q.Encode()
	target_url := u.String()
	client := &http.Client{Timeout: 15 * time.Second}
	fmt.Println("[OfficialAccountAPI]fetchHome - url", target_url)
	req, err := http.NewRequest("GET", target_url, nil)
	if err != nil {
		c.jsonError(ctx, 500, err.Error())
		return
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/wxpic,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Priority", "u=0, i")
	req.Header.Set("Referer", target_url)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 NetType/WIFI MicroMessenger/7.0.20.1781(0x6700143B) MacWechat/3.8.7(0x13080712) UnifiedPCMacWechat(0xf2640619) XWEB/14304 Flue")
	resp, err := client.Do(req)
	if err != nil {
		c.jsonError(ctx, 500, err.Error())
		return
	}
	defer resp.Body.Close()
	resp_bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		c.jsonError(ctx, 500, err.Error())
		return
	}
	c.jsonSuccess(ctx, string(resp_bytes))
}

func (c *APIClient) start_official_account_timer() {
	ticker := time.NewTicker(10 * time.Minute)
	go func() {
		for range ticker.C {
			c.refresh_all_official_account()
		}
	}()
}

// 刷新指定公众号的凭证信息
func (c *APIClient) refresh_account(body *OfficialAccount) (*OfficialAccount, error) {
	// https://mp.weixin.qq.com/mp/profile_ext?action=home&__biz=MzA5MDM1MTcyNQ==&scene=124&uin=Mjk5Mzg5NDE4Mg==&key=daf9bdc5abc4e8d00f609f167aabb6af06ea4dbc232b89ea3625ff4c8356d231d2253abff286be5d3657e0ae2bbed4fc6a12dd1454c7b92ffd89cf0de5ff414c59393f87751427d77cba09a8a42330e4514dd8830773e8c23a3998093ea6daedde2bb1c297f46752095f66be9ed5a5cec88599695f1ad7cffcedb1f68433e70b&devicetype=UnifiedPCWindows&version=f2541022&lang=zh_CN&a8scene=1&acctmode=0&pass_ticket=5wosi2ITX0uw6quyzee0f9Fj+sGMluU7CwaH1V+dReYpDmcYMjjw7983I839W8FG
	client := &http.Client{Timeout: 15 * time.Second}
	u, _ := url.Parse("https://mp.weixin.qq.com/mp/profile_ext")
	q := u.Query()
	q.Set("action", "home")
	q.Set("__biz", body.Biz)
	q.Set("uin", body.Uin)
	q.Set("key", body.Key)
	q.Set("pass_ticket", body.PassTicket)
	q.Set("scene", "124")
	q.Set("devicetype", "UnifiedPCWindows")
	q.Set("version", "f2541022")
	q.Set("lang", "zh_CN")
	q.Set("a8scene", "1")
	q.Set("acctmode", "0")
	u.RawQuery = q.Encode()
	target_url := u.String()
	fmt.Println("[OfficialAccountAPI]refresh account - url", target_url)
	req, _ := http.NewRequest("GET", target_url, nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/wxpic,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Priority", "u=0, i")
	req.Header.Set("Referer", target_url)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 NetType/WIFI MicroMessenger/7.0.20.1781(0x6700143B) MacWechat/3.8.7(0x13080712) UnifiedPCMacWechat(0xf2640619) XWEB/14304 Flue")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("[OfficialAccountAPI]refresh account failed,", err)
		return nil, err
	}
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		fmt.Println("[OfficialAccountAPI]refresh account failed, the cookies is empty")
		return nil, errors.New("request failed, maybe the Credentials is expired")
	}
	fmt.Println("[OfficialAccountAPI]refresh account success")
	new_one := &OfficialAccount{
		Nickname:  body.Nickname,
		AvatarURL: body.AvatarURL,
		Biz:       body.Biz,
		Uin:       body.Uin,
		Key:       body.Key,
	}
	for _, ck := range cookies {
		switch ck.Name {
		case "pass_ticket":
			new_one.PassTicket = ck.Value
		case "key":
			// 貌似拿不到新的 key
			new_one.Key = ck.Value
		case "wxuin", "uin":
			if new_one.Uin == "" {
				new_one.Uin = base64.StdEncoding.EncodeToString([]byte(ck.Value))
			}
		}
	}
	resp.Body.Close()
	return new_one, nil
}
func (c *APIClient) refresh_all_official_account() {
	acct_mu.RLock()
	snapshot := make(map[string]*OfficialAccount, len(accounts))
	for k, v := range accounts {
		snapshot[k] = v
	}
	acct_mu.RUnlock()
	for _, acct := range snapshot {
		go func() {
			refreshed_acct, err := c.refresh_account(acct)
			if err != nil {
				return
			}
			acct_mu.Lock()
			accounts[acct.Biz] = refreshed_acct
			acct_mu.Unlock()
		}()
	}
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
