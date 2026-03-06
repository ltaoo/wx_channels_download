package filehelper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/net/publicsuffix"
)

// Config 文件传输助手配置
type Config struct {
	CallbackURL string // 消息回调地址
}

// Client 文件传输助手客户端
type Client struct {
	cfg        *Config
	logger     *zerolog.Logger
	httpClient *http.Client

	// 登录状态
	mu         sync.RWMutex
	uuid       string
	uuidTs     time.Time
	loggedIn   bool
	cookies    []*http.Cookie
	cookiesRaw string
	skey       string
	sid        string
	uin        string
	passTicket string
	userName   string
	deviceID   string
	entryHost  string
	loginHost  string
	fileHost   string

	// synckey
	synckey map[string]interface{}

	// 消息缓存
	msgCache   []map[string]interface{}
	msgCacheMu sync.RWMutex

	// 登录状态
	lastLoginCode    int
	lastLoginMessage string

	// 停止信号
	stopChan chan struct{}
}

// SyncKeyItem 同步键值对
type SyncKeyItem struct {
	Key int `json:"Key"`
	Val int `json:"Val"`
}

// InitResponse 初始化响应
type InitResponse struct {
	BaseResponse struct {
		Ret     int `json:"Ret"`
		ErrCode int `json:"ErrCode"`
	} `json:"BaseResponse"`
	Count       int           `json:"Count"`
	ContactList []interface{} `json:"ContactList"`
	SyncKey     struct {
		Count int           `json:"Count"`
		List  []SyncKeyItem `json:"List"`
	} `json:"SyncKey"`
	User struct {
		Uin        int    `json:"Uin"`
		UserName   string `json:"UserName"`
		NickName   string `json:"NickName"`
		HeadImgUrl string `json:"HeadImgUrl"`
	} `json:"User"`
}

// SyncResponse webwxsync 完整响应
type SyncResponse struct {
	BaseResponse struct {
		Ret    int    `json:"Ret"`
		ErrMsg string `json:"ErrMsg"`
	} `json:"BaseResponse"`
	AddMsgCount        int                     `json:"AddMsgCount"`
	AddMsgList         []map[string]interface{} `json:"AddMsgList"`
	ModContactCount    int                     `json:"ModContactCount"`
	ModContactList     []interface{}           `json:"ModContactList"`
	DelContactCount    int                     `json:"DelContactCount"`
	DelContactList     []interface{}           `json:"DelContactList"`
	ModChatRoomMemberCount int                 `json:"ModChatRoomMemberCount"`
	ModChatRoomMemberList  []interface{}       `json:"ModChatRoomMemberList"`
	Profile            map[string]interface{}  `json:"Profile"`
	ContinueFlag       int                     `json:"ContinueFlag"`
	SyncKey            struct {
		Count int `json:"Count"`
		List  []struct {
			Key int `json:"Key"`
			Val int `json:"Val"`
		} `json:"List"`
	} `json:"SyncKey"`
	SKey           string `json:"SKey"`
	SyncCheckKey   struct {
		Count int `json:"Count"`
		List  []struct {
			Key int `json:"Key"`
			Val int `json:"Val"`
		} `json:"List"`
	} `json:"SyncCheckKey"`
}

// LoginStatusDetail 登录状态详情
type LoginStatusDetail struct {
	LoggedIn       bool   `json:"logged_in"`
	Code           int    `json:"code"`
	Status         string `json:"status"`
	HasUUID        bool   `json:"has_uuid"`
	UUID           string `json:"uuid"`
	UUIDAgeSeconds int    `json:"uuid_age_seconds,omitempty"`
}

// NewClient 创建客户端
func NewClient(cfg *Config, logger *zerolog.Logger) *Client {
	if logger == nil {
		nopLogger := zerolog.Nop()
		logger = &nopLogger
	}
	// 创建带 cookie jar 的 http client
	jar, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		jar = nil // 如果创建失败，不使用 cookie jar
	}
	return &Client{
		cfg:        cfg,
		logger:     logger,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
			Jar:     jar,
		},
		deviceID:   generateDeviceID(),
		entryHost:  "szfilehelper.weixin.qq.com",
		synckey:    map[string]interface{}{"Count": 0, "List": []interface{}{}},
		msgCache:   make([]map[string]interface{}, 0),
		stopChan:   make(chan struct{}),
	}
}

// GetQRCode 获取登录二维码
func (c *Client) GetQRCode() (string, error) {
	c.resolveHosts()

	// UUID 不存在或过期 (>240s) 时重新获取
	c.mu.RLock()
	uuid := c.uuid
	uuidTs := c.uuidTs
	c.mu.RUnlock()

	if uuid == "" || time.Since(uuidTs) > 240*time.Second {
		if err := c.jsloginGetUUID(); err != nil {
			return "", err
		}
		c.mu.RLock()
		uuid = c.uuid
		c.mu.RUnlock()
	}

	c.mu.Lock()
	c.lastLoginMessage = "qr_ready"
	c.mu.Unlock()

	qrcodeURL := fmt.Sprintf("https://login.weixin.qq.com/qrcode/%s", uuid)
	return qrcodeURL, nil
}

// jsloginGetUUID 获取 UUID
func (c *Client) jsloginGetUUID() error {
	timestamp := time.Now().UnixMilli()
	redirectURI := url.QueryEscape(fmt.Sprintf("https://%s/cgi-bin/mmwebwx-bin/webwxnewloginpage", c.entryHost))
	apiURL := fmt.Sprintf(
		"https://%s/jslogin?appid=wx_webfilehelper&redirect_uri=%s&fun=new&lang=zh_CN&_=%d",
		c.loginHost,
		redirectURI,
		timestamp,
	)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	c.setRequestHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	bodyStr := string(body)
	uuidRegex := regexp.MustCompile(`window\.QRLogin\.uuid\s*=\s*"([^"]+)"`)
	uuidMatch := uuidRegex.FindStringSubmatch(bodyStr)

	if len(uuidMatch) < 2 {
		return fmt.Errorf("解析响应失败: %s", bodyStr)
	}

	uuid := uuidMatch[1]
	c.mu.Lock()
	c.uuid = uuid
	c.uuidTs = time.Now()
	c.mu.Unlock()

	c.logger.Info().Str("uuid", uuid).Msg("获取 UUID 成功")
	return nil
}

// WaitForLogin 等待登录（阻塞调用）
// 返回: 200=登录成功, 201=已扫码等待确认, 400=二维码过期, 408=等待扫码
func (c *Client) WaitForLogin() (int, string, error) {
	// 检查是否已经登录
	c.mu.RLock()
	uuid := c.uuid
	loggedIn := c.loggedIn
	c.mu.RUnlock()

	if loggedIn {
		return 200, "", nil
	}

	if uuid == "" {
		return 500, "", fmt.Errorf("请先获取二维码")
	}

	timestamp := time.Now().UnixMilli()
	rValue := ^int(time.Now().Unix())
	apiURL := fmt.Sprintf(
		"https://%s/cgi-bin/mmwebwx-bin/login?loginicon=true&uuid=%s&tip=1&r=%d&_=%d&appid=wx_webfilehelper",
		c.loginHost,
		url.QueryEscape(uuid),
		rValue,
		timestamp,
	)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return 500, "", fmt.Errorf("创建请求失败: %w", err)
	}

	c.setRequestHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 500, "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 500, "", fmt.Errorf("读取响应失败: %w", err)
	}

	bodyStr := string(body)

	// 解析响应
	codeRegex := regexp.MustCompile(`window\.code\s*=\s*(\d+)`)
	avatarRegex := regexp.MustCompile(`window\.userAvatar\s*=\s*'([^']+)'`)
	redirectRegex := regexp.MustCompile(`window\.redirect_uri\s*=\s*"([^"]+)"`)

	codeMatch := codeRegex.FindStringSubmatch(bodyStr)
	if len(codeMatch) < 2 {
		return 500, "", fmt.Errorf("解析响应失败: %s", bodyStr)
	}

	code, _ := strconv.Atoi(codeMatch[1])
	c.mu.Lock()
	c.lastLoginCode = code
	c.mu.Unlock()

	switch code {
	case 200:
		// 检查是否已经完成登录
		c.mu.RLock()
		loggedIn := c.loggedIn
		c.mu.RUnlock()
		if loggedIn {
			return 200, "", nil
		}

		redirectMatch := redirectRegex.FindStringSubmatch(bodyStr)
		if len(redirectMatch) < 2 {
			return 500, "", fmt.Errorf("未找到 redirect_uri")
		}
		redirectURI := redirectMatch[1]
		c.mu.Lock()
		c.lastLoginMessage = "authorized"
		c.mu.Unlock()

		// 完成登录
		if err := c.completeLogin(redirectURI); err != nil {
			return 500, "", err
		}
		return 200, "", nil

	case 201:
		avatarMatch := avatarRegex.FindStringSubmatch(bodyStr)
		avatar := ""
		if len(avatarMatch) > 1 {
			avatar = avatarMatch[1]
		}
		c.mu.Lock()
		c.lastLoginMessage = "scanned_wait_confirm"
		c.mu.Unlock()
		return 201, avatar, nil

	case 408:
		c.mu.Lock()
		c.lastLoginMessage = "qr_wait_scan"
		c.mu.Unlock()
		return 408, "", nil

	case 400, 500:
		c.mu.Lock()
		c.uuid = ""
		c.lastLoginMessage = "qr_expired"
		c.mu.Unlock()
		return code, "", nil

	default:
		return code, "", nil
	}
}

// completeLogin 完成登录
func (c *Client) completeLogin(redirectURI string) error {
	c.logger.Info().Str("redirect_uri", redirectURI).Msg("开始完成登录")

	// 解析 redirect_uri 获取参数
	parsedURL, err := url.Parse(redirectURI)
	if err != nil {
		return fmt.Errorf("解析 redirect_uri 失败: %w", err)
	}

	query := parsedURL.Query()
	domain := parsedURL.Host
	if domain == "" {
		domain = c.entryHost
	}

	// 更新 entry host
	c.mu.Lock()
	c.entryHost = domain
	c.resolveHostsLocked()
	c.mu.Unlock()

	// 获取参数，使用 fallback 值
	ticket := query.Get("ticket")
	uuid := query.Get("uuid")
	if uuid == "" {
		c.mu.RLock()
		uuid = c.uuid
		c.mu.RUnlock()
	}
	lang := query.Get("lang")
	if lang == "" {
		lang = "zh_CN"
	}
	scan := query.Get("scan")

	c.logger.Info().
		Str("domain", domain).
		Str("ticket", ticket).
		Str("uuid", uuid).
		Str("lang", lang).
		Str("scan", scan).
		Msg("webwxnewloginpage 请求参数")

	// 构造请求 URL 和参数（与 Python 保持一致）
	apiURL := fmt.Sprintf("https://%s/cgi-bin/mmwebwx-bin/webwxnewloginpage", domain)
	params := url.Values{}
	params.Set("fun", "new")
	params.Set("version", "v2")
	params.Set("ticket", ticket)
	params.Set("uuid", uuid)
	params.Set("lang", lang)
	params.Set("scan", scan)

	fullURL := apiURL + "?" + params.Encode()
	c.logger.Info().Str("url", fullURL).Msg("请求 webwxnewloginpage")

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 只设置必要的请求头，与 Python 保持一致
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36")
	req.Header.Set("mmweb_appid", "wx_webfilehelper")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 保存响应中的 cookies
	c.SetCookies(resp.Cookies(), extractCookiesFromResponse(resp))

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	xmlBody := string(body)
	c.logger.Info().Str("response", xmlBody).Msg("webwxnewloginpage 响应")

	// 检查错误响应：微信的响应使用 <error> 作为根元素，<ret>0</ret> 表示成功
	retValue := extractXMLTag(xmlBody, "ret")
	if retValue != "" && retValue != "0" {
		errMsg := extractXMLTag(xmlBody, "message")
		return fmt.Errorf("webwxnewloginpage 返回错误: ret=%s, message=%s, redirect_uri: %s",
			retValue, errMsg, redirectURI)
	}

	c.mu.Lock()
	c.skey = extractXMLTag(xmlBody, "skey")
	c.sid = extractXMLTag(xmlBody, "wxsid")
	c.uin = extractXMLTag(xmlBody, "wxuin")
	c.passTicket = extractXMLTag(xmlBody, "pass_ticket")
	c.mu.Unlock()

	if c.skey == "" || c.sid == "" || c.uin == "" || c.passTicket == "" {
		return fmt.Errorf("webwxnewloginpage 缺少认证字段，响应: %s", xmlBody[:min(500, len(xmlBody))])
	}

	// 初始化
	if err := c.webwxinit(); err != nil {
		return err
	}

	c.mu.Lock()
	c.loggedIn = true
	c.lastLoginCode = 200
	c.lastLoginMessage = "logged_in"
	c.mu.Unlock()

	c.logger.Info().Str("uin", c.uin).Msg("登录成功")
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// webwxinit 初始化会话
func (c *Client) webwxinit() error {
	c.mu.RLock()
	passTicket := c.passTicket
	entryHost := c.entryHost
	c.mu.RUnlock()

	apiURL := fmt.Sprintf(
		"https://%s/cgi-bin/mmwebwx-bin/webwxinit?r=%d&lang=zh_CN&pass_ticket=%s",
		entryHost,
		^int(time.Now().UnixMilli()),
		url.QueryEscape(passTicket),
	)

	reqBody := map[string]interface{}{
		"BaseRequest": c.buildBaseRequest(),
	}

	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	c.setRequestHeaders(req)
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("mmweb_appid", "wx_webfilehelper")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	var initResp InitResponse
	if err := json.Unmarshal(body, &initResp); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if initResp.BaseResponse.Ret != 0 {
		return fmt.Errorf("初始化失败: %d", initResp.BaseResponse.Ret)
	}

	c.mu.Lock()
	c.userName = initResp.User.UserName
	if initResp.User.Uin != 0 {
		c.uin = strconv.Itoa(initResp.User.Uin)
	}
	c.synckey = map[string]interface{}{
		"Count": initResp.SyncKey.Count,
		"List":  initResp.SyncKey.List,
	}
	c.mu.Unlock()

	c.logger.Info().Msg("初始化成功")
	return nil
}

// GetLoginStatusDetail 获取登录状态详情
func (c *Client) GetLoginStatusDetail() *LoginStatusDetail {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var uuidAge int
	if !c.uuidTs.IsZero() {
		uuidAge = int(time.Since(c.uuidTs).Seconds())
	}

	return &LoginStatusDetail{
		LoggedIn:       c.loggedIn,
		Code:           c.lastLoginCode,
		Status:         c.lastLoginMessage,
		HasUUID:        c.uuid != "",
		UUID:           c.uuid,
		UUIDAgeSeconds: uuidAge,
	}
}

// StartSyncCheck 启动同步检查（后台运行）
func (c *Client) StartSyncCheck() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.mu.RLock()
			loggedIn := c.loggedIn
			c.mu.RUnlock()

			if !loggedIn {
				continue
			}

			status, err := c.synccheck()
			if err != nil {
				c.logger.Error().Err(err).Msg("同步检查失败")
				continue
			}

			if status == "hasMsg" {
				c.logger.Info().Msg("有新消息")
				syncResp, err := c.webwxsync()
				if err != nil {
					c.logger.Error().Err(err).Msg("获取消息失败")
					continue
				}
				c.handleSyncResponse(syncResp)
			} else if status == "logout" {
				c.logger.Warn().Msg("已登出")
				c.mu.Lock()
				c.loggedIn = false
				c.mu.Unlock()
			}
		}
	}
}

// synccheck 同步检查
func (c *Client) synccheck() (string, error) {
	c.mu.RLock()
	skey := c.skey
	sid := c.sid
	uin := c.uin
	deviceID := c.deviceID
	entryHost := c.entryHost
	synckey := c.synckey
	c.mu.RUnlock()

	synckeyStr := formatSyncKey(synckey)

	timestamp := time.Now().UnixMilli()
	apiURL := fmt.Sprintf(
		"https://%s/cgi-bin/mmwebwx-bin/synccheck?r=%d&skey=%s&sid=%s&uin=%s&deviceid=%s&synckey=%s&mmweb_appid=wx_webfilehelper",
		entryHost,
		timestamp,
		url.QueryEscape(skey),
		url.QueryEscape(sid),
		url.QueryEscape(uin),
		url.QueryEscape(deviceID),
		url.QueryEscape(synckeyStr),
	)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	c.setRequestHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	retcodeRegex := regexp.MustCompile(`retcode\s*:\s*"?(\d+)"?`)
	selectorRegex := regexp.MustCompile(`selector\s*:\s*"?(\d+)"?`)

	retcodeMatch := retcodeRegex.FindStringSubmatch(bodyStr)
	selectorMatch := selectorRegex.FindStringSubmatch(bodyStr)

	retcode := ""
	selector := ""
	if len(retcodeMatch) > 1 {
		retcode = retcodeMatch[1]
	}
	if len(selectorMatch) > 1 {
		selector = selectorMatch[1]
	}

	if retcode != "0" {
		return "logout", nil
	}
	if selector != "" && selector != "0" {
		return "hasMsg", nil
	}
	return "wait", nil
}

// WaitSyncCheck 阻塞等待同步检查，返回状态
func (c *Client) WaitSyncCheck() (string, error) {
	return c.synccheck()
}

// webwxsync 获取新消息，返回完整响应
func (c *Client) webwxsync() (*SyncResponse, error) {
	c.mu.RLock()
	skey := c.skey
	sid := c.sid
	passTicket := c.passTicket
	entryHost := c.entryHost
	synckey := c.synckey
	c.mu.RUnlock()

	apiURL := fmt.Sprintf(
		"https://%s/cgi-bin/mmwebwx-bin/webwxsync?sid=%s&skey=%s&pass_ticket=%s",
		entryHost,
		url.QueryEscape(sid),
		url.QueryEscape(skey),
		url.QueryEscape(passTicket),
	)

	reqBody := map[string]interface{}{
		"BaseRequest": c.buildBaseRequest(),
		"SyncKey":     synckey,
		"rr":          ^int(time.Now().UnixMilli()),
	}

	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	c.setRequestHeaders(req)
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("mmweb_appid", "wx_webfilehelper")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var data SyncResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if data.BaseResponse.Ret != 0 {
		return nil, fmt.Errorf("同步失败: %d", data.BaseResponse.Ret)
	}

	// 更新 synckey
	if data.SyncKey.Count > 0 {
		c.mu.Lock()
		c.synckey = map[string]interface{}{
			"Count": data.SyncKey.Count,
			"List":  data.SyncKey.List,
		}
		c.mu.Unlock()
	}

	return &data, nil
}

// GetLatestMessages 获取最新消息
func (c *Client) GetLatestMessages(limit int) []map[string]interface{} {
	c.msgCacheMu.RLock()
	defer c.msgCacheMu.RUnlock()

	if limit <= 0 || limit > len(c.msgCache) {
		limit = len(c.msgCache)
	}

	start := len(c.msgCache) - limit
	if start < 0 {
		start = 0
	}

	result := make([]map[string]interface{}, limit)
	copy(result, c.msgCache[start:])
	return result
}

// SyncMessages 同步消息，返回完整响应
func (c *Client) SyncMessages() (*SyncResponse, error) {
	return c.webwxsync()
}

// SendText 发送文本消息
func (c *Client) SendText(text string) error {
	c.mu.RLock()
	entryHost := c.entryHost
	passTicket := c.passTicket
	userName := c.userName
	c.mu.RUnlock()

	apiURL := fmt.Sprintf(
		"https://%s/cgi-bin/mmwebwx-bin/webwxsendmsg?lang=zh_CN&pass_ticket=%s",
		entryHost,
		url.QueryEscape(passTicket),
	)

	msgID := generateMsgID()
	reqBody := map[string]interface{}{
		"BaseRequest": c.buildBaseRequest(),
		"Msg": map[string]interface{}{
			"Type":         1,
			"Content":      text,
			"FromUserName": userName,
			"ToUserName":   "filehelper",
			"LocalID":      msgID,
			"ClientMsgId":  msgID,
		},
		"Scene": 0,
	}

	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	c.setRequestHeaders(req)
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("mmweb_appid", "wx_webfilehelper")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var data struct {
		BaseResponse struct {
			Ret int `json:"Ret"`
		} `json:"BaseResponse"`
		MsgID string `json:"MsgID"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if data.BaseResponse.Ret != 0 {
		return fmt.Errorf("发送失败: %d", data.BaseResponse.Ret)
	}

	c.logger.Info().Str("msgId", data.MsgID).Msg("发送文本成功")
	return nil
}

// handleSyncResponse 处理同步响应
func (c *Client) handleSyncResponse(resp *SyncResponse) {
	if resp == nil || len(resp.AddMsgList) == 0 {
		return
	}

	// 缓存消息
	c.msgCacheMu.Lock()
	for _, msg := range resp.AddMsgList {
		c.msgCache = append(c.msgCache, msg)
	}
	if len(c.msgCache) > 200 {
		c.msgCache = c.msgCache[len(c.msgCache)-200:]
	}
	c.msgCacheMu.Unlock()

	// 回调通知
	if c.cfg.CallbackURL == "" {
		c.logger.Warn().Msg("未配置回调地址，跳过消息通知")
		return
	}

	for _, msg := range resp.AddMsgList {
		go func(m map[string]interface{}) {
			bodyBytes, _ := json.Marshal(m)
			req, err := http.NewRequest("POST", c.cfg.CallbackURL, strings.NewReader(string(bodyBytes)))
			if err != nil {
				c.logger.Error().Err(err).Msg("创建回调请求失败")
				return
			}
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				c.logger.Error().Err(err).Msg("发送回调失败")
				return
			}
			resp.Body.Close()

			msgId, _ := m["MsgId"].(string)
			c.logger.Info().Str("msgId", msgId).Msg("消息回调成功")
		}(msg)
	}
}

// IsLoggedIn 检查是否已登录
func (c *Client) IsLoggedIn() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loggedIn
}

// GetUUID 获取当前 UUID
func (c *Client) GetUUID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.uuid
}

// Logout 登出
func (c *Client) Logout() error {
	c.mu.RLock()
	skey := c.skey
	sid := c.sid
	uin := c.uin
	loggedIn := c.loggedIn
	entryHost := c.entryHost
	c.mu.RUnlock()

	if !loggedIn {
		return nil
	}

	// 停止同步检查
	close(c.stopChan)
	c.stopChan = make(chan struct{})

	apiURL := fmt.Sprintf(
		"https://%s/cgi-bin/mmwebwx-bin/webwxlogout?redirect=1&type=0&skey=%s",
		entryHost,
		url.QueryEscape(skey),
	)

	reqBody := map[string]interface{}{
		"sid": sid,
		"uin": uin,
	}

	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	c.setRequestHeaders(req)
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("mmweb_appid", "wx_webfilehelper")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	resp.Body.Close()

	c.logger.Info().Msg("登出成功")

	c.mu.Lock()
	c.loggedIn = false
	c.uuid = ""
	c.cookies = nil
	c.cookiesRaw = ""
	c.skey = ""
	c.sid = ""
	c.uin = ""
	c.passTicket = ""
	c.userName = ""
	c.synckey = map[string]interface{}{"Count": 0, "List": []interface{}{}}
	c.mu.Unlock()

	return nil
}

// SetCookies 设置 cookies
func (c *Client) SetCookies(cookies []*http.Cookie, raw string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cookies = cookies
	c.cookiesRaw = raw
}

// resolveHosts 解析主机
func (c *Client) resolveHosts() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resolveHostsLocked()
}

func (c *Client) resolveHostsLocked() {
	host := c.entryHost
	if strings.Contains(host, "cmfilehelper.weixin") {
		c.loginHost = "login.wx8.qq.com"
		c.fileHost = "file.wx8.qq.com"
	} else if strings.Contains(host, "szfilehelper.weixin.qq.com") {
		c.loginHost = "login.wx2.qq.com"
		c.fileHost = "file.wx2.qq.com"
	} else {
		c.loginHost = "login.wx.qq.com"
		c.fileHost = "file.wx.qq.com"
	}
}

// setRequestHeaders 设置请求头
func (c *Client) setRequestHeaders(req *http.Request) {
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36")
	req.Header.Set("sec-ch-ua", `"Not:A-Brand";v="99", "Google Chrome";v="145", "Chromium";v="145"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)

	c.mu.RLock()
	if c.cookiesRaw != "" {
		req.Header.Set("Cookie", c.cookiesRaw)
	}
	c.mu.RUnlock()
}

// 辅助函数

// buildBaseRequest 构造 BaseRequest，与 Python 版本保持一致
func (c *Client) buildBaseRequest() map[string]interface{} {
	c.mu.RLock()
	uin := c.uin
	sid := c.sid
	skey := c.skey
	deviceID := c.deviceID
	c.mu.RUnlock()

	// 尝试将 uin 转换为 int，与 Python 版本保持一致
	var uinValue interface{}
	if uinInt, err := strconv.Atoi(uin); err == nil {
		uinValue = uinInt
	} else {
		uinValue = uin
	}

	return map[string]interface{}{
		"Uin":      uinValue,
		"Sid":      sid,
		"Skey":     skey,
		"DeviceID": deviceID,
	}
}

func generateDeviceID() string {
	return fmt.Sprintf("e%d", time.Now().UnixNano()/1000000)
}

func generateMsgID() string {
	return fmt.Sprintf("%d%d", time.Now().UnixMilli(), time.Now().Nanosecond()/1000000%1000)
}

func formatSyncKey(synckey map[string]interface{}) string {
	list := synckey["List"]
	if list == nil {
		return ""
	}

	var parts []string

	// 处理 []SyncKeyItem 类型（从 webwxinit 获取）
	if items, ok := list.([]SyncKeyItem); ok {
		for _, item := range items {
			parts = append(parts, fmt.Sprintf("%d_%d", item.Key, item.Val))
		}
		return strings.Join(parts, "|")
	}

	// 处理结构体切片类型（从 SyncResponse 获取）
	if items, ok := list.([]struct {
		Key int `json:"Key"`
		Val int `json:"Val"`
	}); ok {
		for _, item := range items {
			parts = append(parts, fmt.Sprintf("%d_%d", item.Key, item.Val))
		}
		return strings.Join(parts, "|")
	}

	// 处理 []interface{} 类型（从 JSON 反序列化获取）
	if items, ok := list.([]interface{}); ok {
		for _, item := range items {
			if m, ok := item.(map[string]interface{}); ok {
				key, _ := m["Key"].(float64)
				val, _ := m["Val"].(float64)
				parts = append(parts, fmt.Sprintf("%d_%d", int(key), int(val)))
			}
		}
		return strings.Join(parts, "|")
	}

	return ""
}

func extractCookiesFromResponse(resp *http.Response) string {
	var cookies []string
	for _, c := range resp.Cookies() {
		cookies = append(cookies, fmt.Sprintf("%s=%s", c.Name, c.Value))
	}
	return strings.Join(cookies, "; ")
}

func extractXMLTag(xmlText, tag string) string {
	re := regexp.MustCompile(fmt.Sprintf(`<%s>(.*?)</%s>`, tag, tag))
	match := re.FindStringSubmatch(xmlText)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}
