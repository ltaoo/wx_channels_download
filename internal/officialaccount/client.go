package officialaccount

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	result "wx_channel/internal/util"
)

var accounts = make(map[string]*OfficialAccount)
var acct_mu sync.RWMutex
var official_timer_once sync.Once
var official_ws_upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type OfficialAccountClient struct {
	ServerAddr   string
	RefreshToken string
	RemoteMode   bool
	Tokens       []string
	Cookies      []*http.Cookie
	ws_clients   map[*Client]bool
	ws_mu        sync.RWMutex
	engine       *gin.Engine
	requests     map[string]chan ClientWebsocketResponse
	requests_mu  sync.RWMutex
	// cache       *cache.Cache
	req_seq       uint64
	wait_chan_map map[string]chan *OfficialAccount
	wait_mu       sync.Mutex
}
type OfficialAccountBody struct {
	Biz string `json:"biz"`
}
type OfficialAccount struct {
	Nickname    string `json:"nickname"`
	AvatarURL   string `json:"avatar_url"`
	Biz         string `json:"biz"`
	Uin         string `json:"uin"`
	Key         string `json:"key"`
	PassTicket  string `json:"pass_ticket"`
	AppmsgToken string `json:"appmsg_token"`
}

func NewOfficialAccountClient(cfg *OfficialAccountConfig) *OfficialAccountClient {
	c := &OfficialAccountClient{
		ServerAddr:   cfg.Addr,
		RemoteMode:   cfg.RemoteMode,
		RefreshToken: cfg.RefreshToken,
		Tokens:       make([]string, 0),
		ws_clients:   make(map[*Client]bool),
		requests:     make(map[string]chan ClientWebsocketResponse),
		// engine:     gin.Default(),
		// cache:      cache.New(),
		req_seq:       uint64(time.Now().UnixNano()),
		wait_chan_map: make(map[string]chan *OfficialAccount),
	}
	if strings.TrimSpace(cfg.TokenFilepath) != "" {
		read_tokens := func() {
			f, err := os.Open(cfg.TokenFilepath)
			if err != nil {
				return
			}
			defer f.Close()
			var tokens []string
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				t := strings.TrimSpace(sc.Text())
				if t != "" {
					tokens = append(tokens, t)
				}
			}
			c.Tokens = tokens
		}
		read_tokens()
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				read_tokens()
			}
		}()
	}
	return c
}

func (c *OfficialAccountClient) HandleWebsocket(ctx *gin.Context) {
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

func (c *OfficialAccountClient) ValidateToken(t string) bool {
	if len(c.Tokens) == 0 {
		return true
	}
	if t == "" {
		return false
	}
	for _, v := range c.Tokens {
		if v == t {
			return true
		}
	}
	return false
}

// 获取公众号推送列表
func (c *OfficialAccountClient) HandleFetchOfficialAccountMsgList(ctx *gin.Context) {
	biz := ctx.Query("biz")
	offset := ctx.Query("offset")
	token := ctx.Query("token")
	if valid := c.ValidateToken(token); !valid {
		result.Err(ctx, 1002, "incorrect token")
		return
	}
	_offset, err := strconv.Atoi(offset)
	if err != nil {
		_offset = 0
	}
	data, err := c.FetchAccountMsgList(biz, _offset)
	if err != nil {
		result.Err(ctx, 1002, err.Error())
		return
	}
	result.Ok(ctx, data)
}

// 获取已添加到公众号列表
func (c *OfficialAccountClient) HandleFetchOfficialAccountList(ctx *gin.Context) {
	token := ctx.Query("token")
	if valid := c.ValidateToken(token); !valid {
		result.Err(ctx, 1002, "incorrect token")
		return
	}
	var list []*OfficialAccount
	acct_mu.RLock()
	for _, acct := range accounts {
		list = append(list, acct)
	}
	acct_mu.RUnlock()
	result.Ok(ctx, gin.H{
		"list": list,
	})
}

// 接收 刷新账号凭证 事件
func (c *OfficialAccountClient) HandleRefreshOfficialAccount(ctx *gin.Context) {
	token := ctx.Query("token")
	if token != c.RefreshToken {
		result.Err(ctx, 400, "incorrect refresh token")
		return
	}
	var body OfficialAccount
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if body.Biz == "" || body.Key == "" {
		result.Err(ctx, 400, "Missing the biz parameter")
		return
	}
	accounts[body.Biz] = &body
	c.wait_mu.Lock()
	if ch, ok := c.wait_chan_map[body.Biz]; ok {
		select {
		case ch <- &body:
		default:
		}
	}
	c.wait_mu.Unlock()
	result.Ok(ctx, nil)
}

func (c *OfficialAccountClient) HandleFetchMsgListOfOfficialAccountRSS(ctx *gin.Context) {
	biz := ctx.Query("biz")
	offset := ctx.Query("offset")
	need_content := ctx.Query("content")
	need_proxy := ctx.Query("proxy")
	token := ctx.Query("token")
	if valid := c.ValidateToken(token); !valid {
		result.Err(ctx, 401, "incorrect token")
		return
	}
	_offset, err := strconv.Atoi(offset)
	if err != nil {
		_offset = 0
	}
	data, err := c.FetchAccountMsgList(biz, _offset)
	if err != nil {
		result.Err(ctx, 1002, err.Error())
		return
	}
	var list struct {
		List []OfficialAccountMsgListRespItem `json:"list"`
	}
	err = json.Unmarshal([]byte(data.MsgList), &list)
	if err != nil {
		result.Err(ctx, 1002, err.Error())
		return
	}
	var acct *OfficialAccount
	acct_mu.RLock()
	if a, ok := accounts[biz]; ok {
		acct = a
	}
	acct_mu.RUnlock()
	if acct == nil {
		result.Err(ctx, 1002, "Can't find matched account")
		return
	}
	feed_title := acct.Nickname
	if feed_title == "" {
		feed_title = biz
	}
	feed_uri := fmt.Sprintf("https://mp.weixin.qq.com/mp/profile_ext?action=home&__biz=%s&scene=124", acct.Biz)
	buildURL := func(u string) string {
		if u == "" {
			return ""
		}
		if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			return u
		}
		return "https://mp.weixin.qq.com" + u
	}

	buildEntry := func(title, digest, contentURL, cover, author string, fileid int, pub_date string, authors ...string) AtomEntry {
		u := buildURL(html.UnescapeString(contentURL))
		if need_proxy == "1" && c.ServerAddr != "" {
			u = fmt.Sprintf("http://%s/official_account/proxy?url=%s", c.ServerAddr, url.QueryEscape(u))
		}
		desc := digest
		var thumb *MediaThumbnail
		if cover != "" {
			// cover = html.UnescapeString(cover)
			desc = fmt.Sprintf(`<img src="%s" /><br/>%s`, cover, digest)
			thumb = &MediaThumbnail{
				XMLNSMedia: "http://search.yahoo.com/mrss/",
				URL:        cover,
				Width:      1200,
				Height:     630,
			}
		}
		name := author
		if name == "" {
			for _, alt := range authors {
				if alt != "" {
					name = alt
					break
				}
			}
		}
		if name == "" {
			name = feed_title
		}
		id := u
		if id == "" {
			id = fmt.Sprintf("%s#%d", biz, fileid)
		}
		return AtomEntry{
			ID:        id,
			Title:     title,
			Updated:   pub_date,
			Published: pub_date,
			Author: AtomAuthor{
				Name: name,
			},
			Content: AtomContent{
				Type: "html",
				Body: desc,
			},
			Summary: AtomContent{
				Type: "html",
				Body: desc,
			},
			Link: []AtomLink{
				{Rel: "alternate", Href: u},
			},
			MediaThumbnail: thumb,
		}
	}
	var entries []AtomEntry
	for _, item := range list.List {
		msg := item.MsgExtInfo
		pub_date := time.Unix(int64(item.CommonMsgInfo.Datetime), 0).Format(time.RFC3339)
		entries = append(entries, buildEntry(
			msg.Title,
			msg.Digest,
			msg.ContentUrl,
			msg.Cover,
			msg.Author,
			msg.Fileid,
			pub_date,
		))
		if msg.IsMulti == 1 && len(msg.MultiAppMsgItemList) > 0 {
			for _, art := range msg.MultiAppMsgItemList {
				entries = append(entries, buildEntry(
					art.Title,
					art.Digest,
					art.ContentUrl,
					art.Cover,
					art.Author,
					art.Fileid,
					pub_date,
					msg.Author,
				))
			}
		}
	}
	var links []AtomLink
	self := "http://" + ctx.Request.Host + ctx.Request.RequestURI
	links = append(links, AtomLink{Rel: "self", Href: self})
	alt := "https://mp.weixin.qq.com/mp/profile_ext?action=home&__biz=" + biz
	links = append(links, AtomLink{Rel: "alternate", Href: alt})
	if data.HasMore != 0 && data.NextOffset > 0 {
		u := *ctx.Request.URL
		q := u.Query()
		q.Set("offset", strconv.Itoa(data.NextOffset))
		u.RawQuery = q.Encode()
		next_link := "http://" + ctx.Request.Host + u.String()
		links = append(links, AtomLink{Rel: "next", Href: next_link})
	}
	if need_content == "1" {
		var wg sync.WaitGroup
		for i := range entries {
			var u string
			for _, l := range entries[i].Link {
				if l.Rel == "alternate" {
					u = l.Href
					break
				}
			}
			if u == "" {
				continue
			}
			wg.Add(1)
			go func(idx int, href string) {
				defer wg.Done()
				content := fetch_full_content(href)
				if content != "" {
					entries[idx].Content.Body = content
				}
			}(i, u)
		}
		wg.Wait()
	}
	atom := AtomFeed{
		ID:        biz,
		Title:     feed_title,
		Updated:   time.Now().Format(time.RFC3339),
		Generator: "wx_channels_download",
		Icon:      acct.AvatarURL,
		Category:  []AtomCategory{{Term: "微信公众号"}},
		Link:      links,
		Author: AtomAuthor{
			Name: feed_title,
			URI:  feed_uri,
		},
		Entry: entries,
	}
	ctx.Header("Content-Type", "application/atom+xml; charset=utf-8")
	ctx.XML(http.StatusOK, atom)
}

func (c *OfficialAccountClient) HandleOfficialAccountProxy(ctx *gin.Context) {
	targetURL := ctx.Query("url")
	token := ctx.Query("token")
	if valid := c.ValidateToken(token); !valid {
		result.Err(ctx, 401, "incorrect token")
		return
	}
	if targetURL == "" {
		result.Err(ctx, 400, "Missing url parameter")
		return
	}
	// 尝试进行一次 URL 解码，防止传入的是双重编码的 URL
	// if decoded, err := url.QueryUnescape(targetURL); err == nil {
	// 	targetURL = decoded
	// }
	// 处理 HTML 实体编码，例如 &amp; 转为 &
	targetURL = strings.ReplaceAll(targetURL, "&amp;", "&")
	fmt.Println("[Proxy] Requesting:", targetURL)

	client := &http.Client{}
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		result.Err(ctx, 2000, err.Error())
		// ctx.String(http.StatusInternalServerError, err.Error())
		return
	}
	req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9")
	req.Header.Set("priority", "u=0, i")
	req.Header.Set("sec-ch-ua", `"Google Chrome";v="143", "Chromium";v="143", "Not A(Brand";v="24"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)
	req.Header.Set("sec-fetch-dest", "document")
	req.Header.Set("sec-fetch-mode", "navigate")
	req.Header.Set("sec-fetch-site", "none")
	req.Header.Set("sec-fetch-user", "?1")
	req.Header.Set("upgrade-insecure-requests", "1")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		result.Err(ctx, 2001, err.Error())
		// ctx.String(http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()
	for k, v := range resp.Header {
		if k == "Content-Length" {
			continue
		}
		for _, val := range v {
			ctx.Header(k, val)
		}
	}
	ctx.Status(resp.StatusCode)

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			// 如果读取 body 失败，至少尝试返回状态码（虽然可能已经晚了，但尽量不崩）
			return
		}
		bodyString := string(bodyBytes)
		// 使用正则替换 mmbiz_png/jpg/gif 等链接为代理链接
		// 匹配模式：https://mmbiz.qpic.cn/mmbiz_xxx/ 或 https://mmbiz.qpic.cn/sz_mmbiz_xxx/ 后接非引号和非空白字符
		// 兼容 http 和 https，以及不同的图片格式后缀和路径前缀
		re := regexp.MustCompile(`https?://mmbiz\.qpic\.cn/(?:[a-zA-Z0-9_]+/)*[a-zA-Z0-9_]+/[^\s"']+`)
		bodyString = re.ReplaceAllStringFunc(bodyString, func(match string) string {
			// 构造代理链接
			u := html.UnescapeString(match)
			return fmt.Sprintf("http://%s/official_account/proxy?url=%s", c.ServerAddr, url.QueryEscape(u))
		})
		ctx.Writer.Write([]byte(bodyString))
	} else {
		_, _ = io.Copy(ctx.Writer, resp.Body)
	}
}

func (c *OfficialAccountClient) Validate() error {
	// wc.clientsMu.Lock()
	// defer wc.clientsMu.Unlock()
	if c.RemoteMode {
		return nil
	}
	if len(c.ws_clients) == 0 {
		return errors.New("请先初始化客户端 socket 连接")
	}
	return nil
}
func (c *OfficialAccountClient) RequestAPI(endpoint string, body interface{}, timeout time.Duration) (*ClientWebsocketResponse, error) {
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

func (c *OfficialAccountClient) BuildURL(uu string, params map[string]string) string {
	u, _ := url.Parse(uu)
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	target_url := u.String()
	return target_url
}

func (c *OfficialAccountClient) Fetch(target_url string) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", target_url, nil)
	if err != nil {
		return nil, err
	}
	// req.Header.Set("accept", "*/*")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept-language", "en-US,en;q=0.9")
	req.Header.Set("priority", "u=1, i")
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-origin")
	req.Header.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36 NetType/WIFI MicroMessenger/7.0.20.1781(0x6700143B) WindowsWechat(0x63090a13) UnifiedPCWindowsWechat(0xf2541022) XWEB/16467 Flue")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	// defer resp.Body.Close()
	cookies := resp.Cookies()
	c.Cookies = cookies
	// resp_bytes, err := io.ReadAll(resp.Body)
	// if err != nil {
	// 	return nil, err
	// }
	return resp, nil
}

// 刷新指定公众号的凭证信息
func (c *OfficialAccountClient) RefreshAccount(body *OfficialAccountBody) (*OfficialAccount, error) {
	if body.Biz == "" {
		return nil, errors.New("Missing the biz parameter")
	}
	c.wait_mu.Lock()
	if ch, ok := c.wait_chan_map[body.Biz]; ok {
		c.wait_mu.Unlock()
		select {
		case acct := <-ch:
			return acct, nil
		case <-time.After(20 * time.Second):
			return nil, errors.New("request timeout")
		}
	}
	ch := make(chan *OfficialAccount, 1)
	c.wait_chan_map[body.Biz] = ch
	c.wait_mu.Unlock()
	_, _ = c.RequestAPI("/api/official_account/fetch_account_home", struct {
		Biz string `json:"biz"`
	}{Biz: body.Biz}, 15*time.Second)
	select {
	case acct := <-ch:
		c.wait_mu.Lock()
		delete(c.wait_chan_map, body.Biz)
		c.wait_mu.Unlock()
		accounts[acct.Biz] = acct
		return acct, nil
	case <-time.After(20 * time.Second):
		c.wait_mu.Lock()
		delete(c.wait_chan_map, body.Biz)
		c.wait_mu.Unlock()
		return nil, errors.New("request timeout")
	}
}

func (c *OfficialAccountClient) BuildMsgListURL(acct *OfficialAccount) string {
	u := "https://mp.weixin.qq.com/mp/profile_ext"
	query := map[string]string{
		"action":       "getmsg",
		"__biz":        acct.Biz,
		"uin":          acct.Uin,
		"key":          acct.Key,
		"pass_ticket":  acct.PassTicket,
		"wxtoken":      "",
		"appmsg_token": acct.AppmsgToken,
		"x5":           "0",
		"count":        "10",
		"offset":       "0",
		"f":            "json",
	}
	target_url := c.BuildURL(u, query)
	return target_url
}

// 获取指定公众号的推送列表
func (c *OfficialAccountClient) FetchAccountMsgList(biz string, offset int) (*OfficialMsgListResp, error) {
	err := c.Validate()
	if err != nil {
		return nil, err
	}
	var existing *OfficialAccount
	acct_mu.RLock()
	if _, ok := accounts[biz]; ok {
		data := accounts[biz]
		existing = data
	}
	acct_mu.RUnlock()
	if existing == nil && !c.RemoteMode {
		fmt.Println("there is no existing account, refresh the account with frontend")
		r, err := c.RefreshAccount(&OfficialAccountBody{
			Biz: biz,
		})
		if err != nil {
			return nil, err
		}
		existing = r
	}
	if existing == nil {
		return nil, errors.New("Please adding Credentials first")
	}
	target_url := c.BuildMsgListURL(existing)
	fmt.Println("[API]fetch account msg list1", target_url)
	resp, err := c.Fetch(target_url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	resp_bytes, err := io.ReadAll(resp.Body)
	var data OfficialMsgListResp
	err = json.Unmarshal(resp_bytes, &data)
	if err != nil {
		return nil, err
	}
	if data.Ret != 0 {
		if data.Ret == -3 {
			acct, err := c.RefreshAccount(&OfficialAccountBody{
				Biz: biz,
			})
			if err != nil {
				return nil, err
			}
			target_url := c.BuildMsgListURL(acct)
			fmt.Println("[API]fetch account msg list2", target_url)
			resp, err = c.Fetch(target_url)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			resp_bytes, err := io.ReadAll(resp.Body)
			err = json.Unmarshal(resp_bytes, &data)
			if err != nil {
				return nil, err
			}
			if data.Ret == 0 {
				return &data, nil
			}
		}
		return nil, errors.New(data.ErrMsg)
	}
	return &data, nil
}

func (c *OfficialAccountClient) CookiesToString() string {
	var cookie_parts []string
	for _, cookie := range c.Cookies {
		cookie_parts = append(cookie_parts, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}
	return strings.Join(cookie_parts, "; ")
}
func (c *OfficialAccountClient) Stop() {
	c.ws_mu.Lock()
	for client := range c.ws_clients {
		close(client.send)
		delete(c.ws_clients, client)
	}
	c.ws_mu.Unlock()
}

func fetch_full_content(u string) string {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9")
	req.Header.Set("upgrade-insecure-requests", "1")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	s := string(bodyBytes)
	re := regexp.MustCompile(`(?s)<div[^>]*id="js_content"[^>]*>(.*?)</div>`)
	m := re.FindStringSubmatch(s)
	if len(m) >= 2 {
		content := m[1]
		content = regexp.MustCompile(`\sdata-src="([^"]+)"`).ReplaceAllString(content, ` src="$1"`)
		content = strings.ReplaceAll(content, `src="//`, `src="https://`)
		return content
	}
	return s
}
