package officialaccount

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	result "wx_channel/internal/util"
	"wx_channel/pkg/cache"
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

type OfficialAccountBody struct {
	Biz string `json:"biz"`
}
type OfficialAccount struct {
	Biz         string `json:"biz"`
	Nickname    string `json:"nickname"`
	AvatarURL   string `json:"avatar_url"`
	Uin         string `json:"uin"`
	Key         string `json:"key"`
	PassTicket  string `json:"pass_ticket"`
	AppmsgToken string `json:"appmsg_token"`
	RefreshUri  string `json:"refresh_uri"`
	IsEffective bool   `json:"is_effective"`
	CreatedAt   int64  `json:"created_at"`
	UpdateTime  int64  `json:"update_time"`
	Error       string `json:"error"`
}

func (acct *OfficialAccount) MergeFrom(source *OfficialAccount) {
	if source.Nickname != "" {
		acct.Nickname = source.Nickname
	}
	if source.AvatarURL != "" {
		acct.AvatarURL = source.AvatarURL
	}
	if source.Uin != "" {
		acct.Uin = source.Uin
	}
	if source.Key != "" {
		acct.Key = source.Key
	}
	if source.PassTicket != "" {
		acct.PassTicket = source.PassTicket
	}
	if source.AppmsgToken != "" {
		acct.AppmsgToken = source.AppmsgToken
	}
	if source.RefreshUri != "" {
		acct.RefreshUri = source.RefreshUri
	}
	if source.Error != "" {
		acct.Error = source.Error
	}
}

type OfficialAccountClient struct {
	logger                    *zerolog.Logger
	RemoteServerAddr          string
	RefreshToken              string
	RemoteMode                bool
	RemoteServerProtocol      string
	RemoteServerHostname      string
	RemoteServerPort          int
	RefreshSkipMinutes        int
	AccountIdsRefreshInterval []string
	MaxWebsocketClients       int
	Tokens                    []string
	Cookies                   []*http.Cookie
	ws_clients                map[*Client]bool
	ws_mu                     sync.RWMutex
	manage_ws_clients         map[*Client]bool
	manage_ws_mu              sync.RWMutex
	requests                  map[string]chan ClientWebsocketResponse
	requests_mu               sync.RWMutex
	cache                     *cache.Cache
	req_seq                   uint64
	wait_chan_map             map[string]chan *OfficialAccount
	wait_mu                   sync.Mutex
	refresh_mu                sync.Mutex
	is_refreshing             bool
}

func (c *OfficialAccountClient) next_trace_id(prefix string) string {
	n := atomic.AddUint64(&c.req_seq, 1)
	return fmt.Sprintf("%s-%d", prefix, n)
}

func NewOfficialAccountClient(cfg *OfficialAccountConfig, parent_logger *zerolog.Logger) *OfficialAccountClient {
	logger := parent_logger.With().Str("service", "OfficialAccountClient").Logger()
	c := &OfficialAccountClient{
		logger:                    &logger,
		RemoteMode:                cfg.RemoteMode,
		RemoteServerProtocol:      cfg.RemoteServerProtocol,
		RemoteServerHostname:      cfg.RemoteServerHostname,
		RemoteServerPort:          cfg.RemoteServerPort,
		RefreshSkipMinutes:        cfg.RefreshSkipMinutes,
		AccountIdsRefreshInterval: cfg.AccountIdsRefreshInterval,
		MaxWebsocketClients:       5,
		RefreshToken:              cfg.RefreshToken,
		Tokens:                    make([]string, 0),
		ws_clients:                make(map[*Client]bool),
		manage_ws_clients:         make(map[*Client]bool),
		requests:                  make(map[string]chan ClientWebsocketResponse),
		cache:                     cache.New(),
		req_seq:                   uint64(time.Now().UnixNano()),
		wait_chan_map:             make(map[string]chan *OfficialAccount),
	}
	if cfg.RootDir != "" {
		mp_json_filepath = filepath.Join(cfg.RootDir, "mp.json")
	}
	load_accounts()
	if cfg.RemoteServerHostname != "" {
		origin := cfg.RemoteServerProtocol + "://" + cfg.RemoteServerHostname
		if cfg.RemoteServerPort != 80 && cfg.RemoteServerPort > 0 {
			origin += ":" + strconv.Itoa(cfg.RemoteServerPort)
		}
		c.RemoteServerAddr = origin
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
	if !cfg.RemoteMode && len(c.AccountIdsRefreshInterval) > 0 {
		var valid_accounts []string
		acct_mu.RLock()
		for _, biz := range c.AccountIdsRefreshInterval {
			if _, ok := accounts[biz]; ok {
				valid_accounts = append(valid_accounts, biz)
			}
		}
		acct_mu.RUnlock()
		c.AccountIdsRefreshInterval = valid_accounts
		if len(c.AccountIdsRefreshInterval) > 0 {
			go func() {
				ticker := time.NewTicker(28 * time.Minute)
				defer ticker.Stop()
				for range ticker.C {
					c.RefreshSpecifiedOfficialAccountList(c.AccountIdsRefreshInterval)
				}
			}()
		}
	}
	return c
}

func (c *OfficialAccountClient) HandleWebsocket(ctx *gin.Context) {
	conn, err := official_ws_upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	c.ws_mu.Lock()
	if c.MaxWebsocketClients > 0 && len(c.ws_clients) >= c.MaxWebsocketClients {
		c.ws_mu.Unlock()
		c.logger.Warn().Msg("websocket client limit reached, closing connection")
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "server busy"))
		conn.Close()
		return
	}
	client := &Client{conn: conn, send: make(chan []byte, 256)}
	c.ws_clients[client] = true
	c.ws_mu.Unlock()

	go client.write_pump()

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
			continue
		}
		var msg ClientWSMessage
		if err := json.Unmarshal(message, &msg); err == nil && msg.Type != "" {
			switch msg.Type {
			case "ping":
				c.ws_mu.Lock()
				if _, ok := c.ws_clients[client]; ok {
					client.available = true
					client.last_ping = time.Now().Unix()
					if msg.Data != "" {
						client.title = msg.Data
					}
				}
				c.ws_mu.Unlock()
			}
		}
	}
}

func (c *OfficialAccountClient) HandleManageWebsocket(ctx *gin.Context) {
	conn, err := official_ws_upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	c.manage_ws_mu.Lock()
	client := &Client{conn: conn, send: make(chan []byte, 256)}
	c.manage_ws_clients[client] = true
	c.manage_ws_mu.Unlock()

	go client.write_pump()

	defer func() {
		c.manage_ws_mu.Lock()
		if _, ok := c.manage_ws_clients[client]; ok {
			delete(c.manage_ws_clients, client)
			close(client.send)
		}
		c.manage_ws_mu.Unlock()
		conn.Close()
	}()
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg ClientWSMessage
		if err := json.Unmarshal(message, &msg); err == nil && msg.Type != "" {
			switch msg.Type {
			case "ping":
				c.manage_ws_mu.Lock()
				if _, ok := c.manage_ws_clients[client]; ok {
					client.available = true
					client.last_ping = time.Now().Unix()
					if msg.Data != "" {
						client.title = msg.Data
					}
				}
				c.manage_ws_mu.Unlock()
			}
		}
	}
}

func (c *OfficialAccountClient) BroadcastProgress(total, current, success, failed int) {
	percent := 0
	if total > 0 {
		percent = int(float64(current) / float64(total) * 100)
	}

	data := map[string]int{
		"total":   total,
		"current": current,
		"success": success,
		"failed":  failed,
		"percent": percent,
	}

	msg := APIClientWSMessage{
		Type: "refresh_progress",
		Data: data,
	}

	bytes, err := json.Marshal(msg)
	if err != nil {
		return
	}

	c.manage_ws_mu.RLock()
	defer c.manage_ws_mu.RUnlock()
	for client := range c.manage_ws_clients {
		select {
		case client.send <- bytes:
		default:
		}
	}
}

// 获取公众号推送列表
func (c *OfficialAccountClient) HandleFetchMsgList(ctx *gin.Context) {
	biz := ctx.Query("biz")
	offset := ctx.Query("offset")
	token := ctx.Query("token")
	if valid := c.ValidateToken(token); !valid {
		result.ErrCode(ctx, result.CodeTokenInvalid)
		return
	}
	_offset, err := strconv.Atoi(offset)
	if err != nil {
		_offset = 0
	}
	trace_id := c.next_trace_id("fetch_msg_list")
	logger := c.logger.With().
		Str("trace_id", trace_id).
		Str("biz", biz).
		Int("offset", _offset).
		Logger()
	data, err := c.fetchMsgList(logger, biz, _offset)
	if err != nil {
		code := result.CodeFetchMsgFailed
		msg := result.GetMsg(code)
		loc := ""
		if c, m, l, ok := codedErrorOf(err); ok {
			code = c
			msg = m
			loc = l
		}
		logger.Error().
			Int("resp_code", code).
			Str("resp_msg", msg).
			Str("err", safeLogErr(err)).
			Str("location", loc).
			Msg("fetch msg list: failed")
		result.Err(ctx, code, fmt.Sprintf("%s (loc=%s, trace_id=%s)", msg, loc, trace_id))
		return
	}
	result.Ok(ctx, data)
}

// 获取已添加到公众号列表
func (c *OfficialAccountClient) HandleFetchList(ctx *gin.Context) {
	token := ctx.Query("token")
	if valid := c.ValidateToken(token); !valid {
		result.ErrCode(ctx, result.CodeTokenInvalid)
		return
	}
	page, err := strconv.Atoi(ctx.Query("page"))
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(ctx.Query("page_size"))
	if err != nil || pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 200 {
		pageSize = 200
	}
	keyword := strings.TrimSpace(ctx.Query("keyword"))
	keywordLower := strings.ToLower(keyword)
	is_effective_filter := ctx.Query("is_effective")

	type SafeOfficialAccount struct {
		Biz         string `json:"biz"`
		Nickname    string `json:"nickname"`
		AvatarURL   string `json:"avatar_url"`
		IsEffective bool   `json:"is_effective"`
		CreatedAt   int64  `json:"created_at"`
		UpdateTime  int64  `json:"update_time"`
		Error       string `json:"error"`
		RefreshUri  string `json:"refresh_uri,omitempty"`
	}
	var list []SafeOfficialAccount
	now := time.Now().Unix()
	changed := false
	acct_mu.Lock()
	for _, acct := range accounts {
		if acct != nil && acct.UpdateTime > 0 {
			if now-acct.UpdateTime > 30*60 {
				if acct.IsEffective {
					changed = true
				}
				acct.IsEffective = false
			}
		}
		summary := SafeOfficialAccount{
			Biz:         acct.Biz,
			Nickname:    acct.Nickname,
			AvatarURL:   acct.AvatarURL,
			IsEffective: acct.IsEffective,
			CreatedAt:   acct.CreatedAt,
			UpdateTime:  acct.UpdateTime,
			Error:       acct.Error,
		}
		if !c.RemoteMode {
			summary.RefreshUri = acct.RefreshUri
		}

		if is_effective_filter != "" {
			filterVal := is_effective_filter == "1" || is_effective_filter == "true"
			if summary.IsEffective != filterVal {
				continue
			}
		}

		if keywordLower == "" {
			list = append(list, summary)
		} else {
			bizLower := strings.ToLower(summary.Biz)
			nicknameLower := strings.ToLower(summary.Nickname)
			if strings.Contains(bizLower, keywordLower) || strings.Contains(nicknameLower, keywordLower) {
				list = append(list, summary)
			}
		}
	}
	acct_mu.Unlock()
	if changed {
		save_accounts()
	}
	sort.Slice(list, func(i, j int) bool {
		a := list[i]
		b := list[j]
		if a.CreatedAt != b.CreatedAt {
			return a.CreatedAt > b.CreatedAt
		}
		if a.UpdateTime != b.UpdateTime {
			return a.UpdateTime > b.UpdateTime
		}
		if a.Nickname != b.Nickname {
			return a.Nickname > b.Nickname
		}
		return a.Biz > b.Biz
	})
	total := len(list)
	if total == 0 {
		page = 1
	} else {
		totalPages := (total + pageSize - 1) / pageSize
		if page > totalPages {
			page = totalPages
		}
	}
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	paged := list[start:end]
	result.Ok(ctx, gin.H{
		"list":      paged,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"keyword":   keyword,
	})
}

func (c *OfficialAccountClient) HandleDelete(ctx *gin.Context) {
	token := ctx.Query("token")
	if token != c.RefreshToken {
		result.ErrCode(ctx, result.CodeTokenInvalid)
		return
	}
	var body OfficialAccountBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.ErrCode(ctx, result.CodeInvalidParams)
		return
	}
	if body.Biz == "" {
		result.ErrCode(ctx, result.CodeMissingBiz)
		return
	}

	acct_mu.Lock()
	delete(accounts, body.Biz)
	acct_mu.Unlock()

	save_accounts()
	result.Ok(ctx, nil)
}

// 接收 刷新账号凭证 事件（假定收到的凭证一定是最新的）
func (c *OfficialAccountClient) HandleRefreshEvent(ctx *gin.Context) {
	token := ctx.Query("token")
	if token != c.RefreshToken {
		result.ErrCode(ctx, result.CodeTokenInvalid)
		return
	}
	var body OfficialAccount
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.ErrCode(ctx, result.CodeInvalidParams)
		return
	}
	if body.Biz == "" {
		result.ErrCode(ctx, result.CodeMissingBiz)
		return
	}
	if body.Key == "" {
		result.ErrCode(ctx, result.CodeMissingBiz)
		return
	}
	trace_id := c.next_trace_id("refresh_event")
	logger := c.logger.With().
		Str("trace_id", trace_id).
		Str("biz", body.Biz).
		Str("nickname", body.Nickname).
		Logger()
	logger.Info().Msg("refresh official account event: received")
	now := time.Now().Unix()
	acct_mu.Lock()
	var target_acct *OfficialAccount
	if old, exists := accounts[body.Biz]; exists {
		// copy old account to avoid data race on reading fields
		new_acct := *old
		new_acct.MergeFrom(&body)
		new_acct.IsEffective = true
		if new_acct.CreatedAt == 0 {
			new_acct.CreatedAt = now
		}
		new_acct.UpdateTime = now
		new_acct.Error = ""
		target_acct = &new_acct
		accounts[body.Biz] = target_acct
	} else {
		if len(accounts) >= 20 {
			result.ErrCode(ctx, result.CodeTooManyAccounts)
			return
		}
		body.IsEffective = true
		if body.CreatedAt == 0 {
			body.CreatedAt = now
		}
		body.UpdateTime = now
		target_acct = &body
		accounts[body.Biz] = target_acct
	}
	acct_mu.Unlock()
	save_accounts()
	c.wait_mu.Lock()
	ch, ok := c.wait_chan_map[body.Biz]
	if ok {
		select {
		case ch <- target_acct:
		default:
		}
	}
	c.wait_mu.Unlock()
	logger.Info().
		Bool("has_waiter", ok).
		Bool("remote_mode", c.RemoteMode).
		Msg("refresh official account event: stored and notified")
	if !ok && !c.RemoteMode {
		// 这里是手动刷新页面时，主动向远端服务推送凭证。所以如果是远端服务，不能向自己推，就循环了
		go c.pushCredentialToRemoteServer(logger, target_acct)
	}
	result.Ok(ctx, nil)
}

func (c *OfficialAccountClient) HandleRefreshAllRemoteOfficialAccount(ctx *gin.Context) {
	if err := c.Validate(); err != nil {
		result.ErrCode(ctx, result.CodeClientNotReady)
		return
	}
	run_id := c.next_trace_id("refresh_all_remote")
	c.logger.Info().
		Str("run_id", run_id).
		Str("origin", c.RemoteServerAddr).
		Int("refresh_skip_minutes", c.RefreshSkipMinutes).
		Bool("remote_mode", c.RemoteMode).
		Msg("refresh all remote official accounts: start")
	err := c.refreshAllRemoteOfficialAccount(run_id)
	if err != nil {
		c.logger.Error().
			Str("run_id", run_id).
			Err(err).
			Msg("refresh all remote official accounts: failed")
		result.Err(ctx, 1001, "refresh failed")
		return
	}
	c.logger.Info().
		Str("run_id", run_id).
		Msg("refresh all remote official accounts: completed")
	result.Ok(ctx, nil)
}
func (c *OfficialAccountClient) HandleRefreshRemoteOfficialAccount(ctx *gin.Context) {
	if err := c.Validate(); err != nil {
		result.ErrCode(ctx, result.CodeClientNotReady)
		return
	}
	result.Ok(ctx, nil)
}

// 在本地前端，手动刷新指定公众号（支持批量）
func (c *OfficialAccountClient) HandleRefreshOfficialAccountWithFrontend(ctx *gin.Context) {
	if err := c.Validate(); err != nil {
		result.ErrCode(ctx, result.CodeClientNotReady)
		return
	}

	var req struct {
		BizList []string `json:"biz_list"`
	}
	// Try to bind JSON, ignore error if body is empty
	_ = ctx.ShouldBindJSON(&req)

	err := c.RefreshSpecifiedOfficialAccountList(req.BizList)
	if err != nil {
		if err.Error() == "client not ready" {
			result.ErrCode(ctx, result.CodeClientNotReady)
		} else {
			// For other errors, we might still return OK if some succeeded,
			// or change behavior. Original code didn't return error for partial failure.
			// If RefreshSpecifiedOfficialAccountList returns error, it means it couldn't start (e.g. no clients).
			result.Err(ctx, 1001, err.Error())
		}
		return
	}

	result.Ok(ctx, nil)
}

func (c *OfficialAccountClient) RefreshSpecifiedOfficialAccountList(biz_list []string) error {
	// Identify targets
	var targets []*OfficialAccount
	acct_mu.RLock()
	if len(biz_list) == 0 {
		// All accounts
		targets = make([]*OfficialAccount, 0, len(accounts))
		for _, acct := range accounts {
			if acct != nil {
				targets = append(targets, acct)
			}
		}
	} else {
		// Specific accounts
		targets = make([]*OfficialAccount, 0, len(biz_list))
		for _, biz := range biz_list {
			if acct, ok := accounts[biz]; ok && acct != nil {
				targets = append(targets, acct)
			}
		}
	}
	acct_mu.RUnlock()

	if len(targets) == 0 {
		return nil
	}

	// Filter targets that have RefreshUri
	var jobs []remoteOfficialAccountJob
	for _, t := range targets {
		if t.RefreshUri != "" {
			jobs = append(jobs, remoteOfficialAccountJob{Biz: t.Biz, Nickname: t.Nickname})
		}
	}

	if len(jobs) == 0 {
		return nil
	}

	// Check clients
	clients := c.ListClients()
	if len(clients) == 0 {
		return errors.New("client not ready")
	}

	// Prepare worker pool
	total := len(jobs)
	jobChan := make(chan remoteOfficialAccountJob, total)
	for _, j := range jobs {
		jobChan <- j
	}
	close(jobChan)

	var wg sync.WaitGroup
	processed := make([]int64, len(clients))
	var success int64
	var processed_total int64
	failures := make([]FailureDetail, 0)
	var failures_mu sync.Mutex

	c.BroadcastProgress(total, 0, 0, 0)

	logger := c.logger.With().Str("action", "refresh_specified_list").Logger()

	for i, ws := range clients {
		clientTitle := ""
		if ws != nil {
			clientTitle = ws.title
		}
		wg.Add(1)
		go func(idx int, ws *Client) {
			defer wg.Done()
			workerLogger := logger.With().
				Int("worker_idx", idx).
				Str("client_title", clientTitle).
				Logger()

			for job := range jobChan {
				biz := job.Biz
				jobLogger := workerLogger.With().
					Str("biz", biz).
					Str("nickname", job.Nickname).
					Logger()

				_, err := c.refresh_credential_from_frontend(jobLogger, &OfficialAccountBody{Biz: biz}, ws)

				if err != nil {
					// Fallback logic
					jobLogger.Warn().Err(err).Msg("refresh job: on client failed, fallback to any client")
					fallbackWS, pickErr := c.firstClient()
					if pickErr != nil {
						// Fallback failed
						err2 := pickErr
						failures_mu.Lock()
						failures = append(failures, FailureDetail{Biz: biz, Nickname: job.Nickname, Error: err2.Error()})
						failures_mu.Unlock()

						// Update account error status
						acct_mu.Lock()
						if existing, ok := accounts[biz]; ok {
							existing.Error = err2.Error()
							existing.UpdateTime = time.Now().Unix()
						}
						acct_mu.Unlock()
						save_accounts()
					} else {
						// Retry with fallback client
						fallbackLogger := jobLogger
						if fallbackWS != nil {
							fallbackLogger = fallbackLogger.With().Str("client_title", fallbackWS.title).Logger()
						}
						_, err2 := c.refresh_credential_from_frontend(fallbackLogger, &OfficialAccountBody{Biz: biz}, fallbackWS)
						if err2 != nil {
							failures_mu.Lock()
							failures = append(failures, FailureDetail{Biz: biz, Nickname: job.Nickname, Error: err2.Error()})
							failures_mu.Unlock()

							acct_mu.Lock()
							if existing, ok := accounts[biz]; ok {
								existing.Error = err2.Error()
								existing.UpdateTime = time.Now().Unix()
							}
							acct_mu.Unlock()
							save_accounts()
						} else {
							atomic.AddInt64(&success, 1)
							acct_mu.Lock()
							if acct, ok := accounts[biz]; ok {
								acct.Error = ""
							}
							acct_mu.Unlock()
						}
					}
				} else {
					atomic.AddInt64(&success, 1)
					acct_mu.Lock()
					if acct, ok := accounts[biz]; ok {
						acct.Error = ""
					}
					acct_mu.Unlock()
				}

				atomic.AddInt64(&processed[idx], 1)
				curr := atomic.AddInt64(&processed_total, 1)
				succ := atomic.LoadInt64(&success)
				failures_mu.Lock()
				fail := len(failures)
				failures_mu.Unlock()
				c.BroadcastProgress(total, int(curr), int(succ), fail)
			}
		}(i, ws)
	}

	wg.Wait()
	c.BroadcastProgress(total, total, int(success), len(failures))

	return nil
}

func (c *OfficialAccountClient) HandleOfficialAccountRSS(ctx *gin.Context) {
	biz := ctx.Query("biz")
	offset := ctx.Query("offset")
	need_content := ctx.Query("content")
	need_proxy := ctx.Query("proxy")
	only_proxy_cover := ctx.Query("proxy_cover")

	cache_key := fmt.Sprintf("rss:%s:%s:%s:%s", biz, need_proxy, need_content, only_proxy_cover)
	if val, found := c.cache.Get(cache_key); found {
		if atom, ok := val.(AtomFeed); ok {
			ctx.Header("Content-Type", "application/atom+xml; charset=utf-8")
			ctx.XML(http.StatusOK, atom)
			return
		}
	}
	token := ctx.Query("token")
	if valid := c.ValidateToken(token); !valid {
		result.ErrCode(ctx, result.CodeTokenInvalid)
		return
	}
	_offset, err := strconv.Atoi(offset)
	if err != nil {
		_offset = 0
	}
	trace_id := c.next_trace_id("fetch_msg_list")
	logger := c.logger.With().
		Str("trace_id", trace_id).
		Str("biz", biz).
		Int("offset", _offset).
		Logger()
	data, err := c.fetchMsgList(logger, biz, _offset)
	if err != nil {
		code := result.CodeFetchMsgFailed
		msg := result.GetMsg(code)
		loc := ""
		if c, m, l, ok := codedErrorOf(err); ok {
			code = c
			msg = m
			loc = l
		}
		logger.Error().
			Int("resp_code", code).
			Str("resp_msg", msg).
			Str("err", safeLogErr(err)).
			Str("location", loc).
			Msg("fetch msg list: failed")
		result.Err(ctx, code, fmt.Sprintf("%s (loc=%s, trace_id=%s)", msg, loc, trace_id))
		return
	}
	var list struct {
		List []OfficialAccountMsgListRespItem `json:"list"`
	}
	err = json.Unmarshal([]byte(data.MsgList), &list)
	if err != nil {
		result.ErrCode(ctx, result.CodeDataParseFailed)
		return
	}
	var acct *OfficialAccount
	acct_mu.RLock()
	if a, ok := accounts[biz]; ok {
		acct = a
	}
	acct_mu.RUnlock()
	if acct == nil {
		result.ErrCode(ctx, result.CodeAccountNotFound)
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
		if need_proxy == "1" && c.RemoteServerAddr != "" {
			u = fmt.Sprintf("%s/mp/proxy?url=%s", c.RemoteServerAddr, url.QueryEscape(u))
		}
		desc := digest
		var thumb *MediaThumbnail
		if cover != "" {
			// cover = html.UnescapeString(cover)
			if (need_proxy == "1" || only_proxy_cover == "1") && c.RemoteServerAddr != "" {
				cover = fmt.Sprintf("%s/mp/proxy?url=%s", c.RemoteServerAddr, url.QueryEscape(cover))
			}
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
	c.cache.Set(cache_key, atom, 30*time.Minute)
	ctx.Header("Content-Type", "application/atom+xml; charset=utf-8")
	ctx.XML(http.StatusOK, atom)
}

func (c *OfficialAccountClient) HandleOfficialAccountProxy(ctx *gin.Context) {
	targetURL := ctx.Query("url")
	token := ctx.Query("token")
	if valid := c.ValidateToken(token); !valid {
		result.ErrCode(ctx, result.CodeTokenInvalid)
		return
	}
	if targetURL == "" {
		result.ErrCode(ctx, result.CodeMissingUrl)
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
		result.ErrCode(ctx, result.CodeProxyRequestErr)
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
		result.ErrCode(ctx, result.CodeProxyDispatchErr)
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
			return fmt.Sprintf("%s/mp/proxy?url=%s", c.RemoteServerAddr, url.QueryEscape(u))
		})
		ctx.Writer.Write([]byte(bodyString))
	} else {
		_, _ = io.Copy(ctx.Writer, resp.Body)
	}
}

func (c *OfficialAccountClient) HandleOfficialAccountManagerHome(ctx *gin.Context) {
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	html := string(manager_html)
	remote := c.RemoteServerAddr
	var token string
	if len(c.Tokens) > 0 {
		token = c.Tokens[0]
	}
	mode := "0"
	if c.RemoteMode {
		mode = "1"
	}
	html = strings.ReplaceAll(html, "%%REMOTE_SERVER%%", remote)
	html = strings.ReplaceAll(html, "%%TOKEN%%", token)
	html = strings.ReplaceAll(html, "%%REMOTE_MODE%%", mode)
	ctx.String(http.StatusOK, html)
}

func (c *OfficialAccountClient) HandleFetchOfficialAccountClients(ctx *gin.Context) {
	var list []gin.H
	now := time.Now().Unix()
	c.ws_mu.RLock()
	for cl := range c.ws_clients {
		healthy := cl.available && (now-cl.last_ping) <= 65
		list = append(list, gin.H{
			"title":     cl.title,
			"available": healthy,
			"last_ping": cl.last_ping,
		})
	}
	c.ws_mu.RUnlock()
	result.Ok(ctx, gin.H{
		"list": list,
	})
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

func (c *OfficialAccountClient) Validate() error {
	if c.RemoteMode {
		return nil
	}
	c.ws_mu.RLock()
	empty := len(c.ws_clients) == 0
	c.ws_mu.RUnlock()
	if empty {
		return errors.New(result.GetMsg(result.CodeClientNotReady))
	}
	return nil
}
func (c *OfficialAccountClient) EnsureFrontendReady(timeout time.Duration) error {
	if c.RemoteMode {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for {
		c.ws_mu.RLock()
		ready := len(c.ws_clients) > 0
		c.ws_mu.RUnlock()
		if ready {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New(result.GetMsg(result.CodeClientNotReady))
		}
		time.Sleep(200 * time.Millisecond)
	}
}
func (c *OfficialAccountClient) RequestFrontend(endpoint string, body interface{}, timeout time.Duration) (*ClientWebsocketResponse, error) {
	if err := c.EnsureFrontendReady(3 * time.Second); err != nil {
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
	c.ws_mu.RLock()
	var client *Client
	for cl := range c.ws_clients {
		client = cl
		break
	}
	c.ws_mu.RUnlock()
	if client == nil {
		return nil, errors.New(result.GetMsg(result.CodeClientNotReady))
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	end := time.Now().Add(2 * time.Second)
	for {
		select {
		case client.send <- data:
			goto WAIT_RESP
		default:
			if time.Now().After(end) {
				return nil, errors.New(result.GetMsg(result.CodeClientBusy))
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
WAIT_RESP:
	select {
	case resp := <-resp_chan:
		return &resp, nil
	case <-time.After(timeout):
		return nil, errors.New(result.GetMsg(result.CodeTimeout))
	}
}
func (c *OfficialAccountClient) ListClients() []*Client {
	c.ws_mu.RLock()
	clients := make([]*Client, 0, len(c.ws_clients))
	for cl := range c.ws_clients {
		clients = append(clients, cl)
	}
	c.ws_mu.RUnlock()
	return clients
}

func (c *OfficialAccountClient) firstClient() (*Client, error) {
	c.ws_mu.RLock()
	defer c.ws_mu.RUnlock()
	for cl := range c.ws_clients {
		if cl != nil {
			return cl, nil
		}
	}
	return nil, errors.New(result.GetMsg(result.CodeClientNotReady))
}
func (c *OfficialAccountClient) RequestFrontendOn(ws *Client, endpoint string, body interface{}, timeout time.Duration) (*ClientWebsocketResponse, error) {
	if err := c.EnsureFrontendReady(3 * time.Second); err != nil {
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
	c.ws_mu.RLock()
	_, ok := c.ws_clients[ws]
	c.ws_mu.RUnlock()
	if !ok {
		return nil, errors.New("没有可用的客户端")
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	end := time.Now().Add(2 * time.Second)
	for {
		select {
		case ws.send <- data:
			goto WAIT_RESP_ON
		default:
			if time.Now().After(end) {
				return nil, errors.New("发送缓冲区已满")
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
WAIT_RESP_ON:
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

func (c *OfficialAccountClient) Fetch(target_url string, referer string) (*http.Response, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", target_url, nil)
	if err != nil {
		return nil, err
	}
	// req.Header.Set("accept", "*/*")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept-language", "en-US,en;q=0.9")
	req.Header.Set("priority", "u=1, i")
	req.Header.Set("referer", referer)
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

// 调用前端刷新指定公众号的凭证信息
func (c *OfficialAccountClient) RefreshAccountWithFrontend(body *OfficialAccountBody) (*OfficialAccount, error) {
	trace_id := c.next_trace_id("refresh_account_frontend")
	logger := c.logger.With().
		Str("trace_id", trace_id).
		Str("biz", body.Biz).
		Logger()
	start := time.Now()
	if body.Biz == "" {
		logger.Error().Msg("refresh official account via frontend: missing biz")
		return nil, errors.New(result.GetMsg(result.CodeMissingBiz))
	}
	if err := c.EnsureFrontendReady(5 * time.Second); err != nil {
		logger.Error().Err(err).Msg("refresh official account via frontend: frontend not ready")
		return nil, err
	}
	acct_mu.RLock()
	acct, ok := accounts[body.Biz]
	if !ok {
		acct_mu.RUnlock()
		return nil, errors.New(result.GetMsg(result.CodeAccountNotFound))
	}
	if strings.TrimSpace(acct.RefreshUri) == "" {
		acct_mu.RUnlock()
		return nil, errors.New(result.GetMsg(result.CodeMissingRefreshUri))
	}
	if acct.IsEffective && time.Now().Unix()-acct.UpdateTime < 20*60 {
		age := time.Now().Unix() - acct.UpdateTime
		logger.Info().
			Int64("acct_update_time", acct.UpdateTime).
			Int64("acct_update_age_sec", age).
			Msg("refresh official account via frontend: skip (recent update)")
		acct_mu.RUnlock()
		go c.pushCredentialToRemoteServer(logger, acct)
		logger.Info().Dur("cost", time.Since(start)).Msg("refresh official account via frontend: completed (skipped)")
		return acct, nil
	}
	acct_mu.RUnlock()
	ws, err := c.firstClient()
	if err != nil {
		logger.Error().Err(err).Msg("refresh official account via frontend: no available client")
		return nil, err
	}
	if ws != nil {
		logger = logger.With().Str("client_title", ws.title).Logger()
	}
	return c.refresh_credential_from_frontend(logger, body, ws)
}

func (c *OfficialAccountClient) refresh_credential_from_frontend(logger zerolog.Logger, body *OfficialAccountBody, ws *Client) (*OfficialAccount, error) {
	start := time.Now()
	logger.Info().Msg("refresh official account via frontend: start")
	if body.Biz == "" {
		logger.Error().Msg("refresh official account via frontend: missing biz")
		return nil, errors.New(result.GetMsg(result.CodeMissingBiz))
	}
	if err := c.EnsureFrontendReady(5 * time.Second); err != nil {
		logger.Error().Err(err).Msg("refresh official account via frontend: frontend not ready")
		return nil, err
	}
	if ws == nil {
		return nil, errors.New(result.GetMsg(result.CodeClientNotReady))
	}
	acct_mu.RLock()
	acct, ok := accounts[body.Biz]
	if !ok {
		acct_mu.RUnlock()
		return nil, errors.New(result.GetMsg(result.CodeAccountNotFound))
	}
	if strings.TrimSpace(acct.RefreshUri) == "" {
		acct_mu.RUnlock()
		return nil, errors.New("缺少 refresh_uri")
	}
	acct_mu.RUnlock()
	c.wait_mu.Lock()
	if ch, ok := c.wait_chan_map[acct.Biz]; ok {
		c.wait_mu.Unlock()
		logger.Debug().Msg("refresh official account via frontend: wait channel exists, waiting")
		select {
		case cur_acct := <-ch:
			logger.Info().Dur("cost", time.Since(start)).Msg("refresh official account via frontend: completed (shared result)")
			return cur_acct, nil
		case <-time.After(20 * time.Second):
			logger.Error().Dur("cost", time.Since(start)).Msg("refresh official account via frontend: timeout (shared wait)")
			return nil, errors.New(result.GetMsg(result.CodeTimeout))
		}
	}
	ch := make(chan *OfficialAccount, 1)
	c.wait_chan_map[acct.Biz] = ch
	c.wait_mu.Unlock()

	logger.Info().Msg("refresh official account via frontend: request frontend fetch_account_home")

	reqBody := struct {
		Biz        string `json:"biz"`
		RefreshUri string `json:"refresh_uri"`
	}{Biz: acct.Biz, RefreshUri: acct.RefreshUri}

	if _, err := c.RequestFrontendOn(ws, "key:fetch_account_home", reqBody, 15*time.Second); err != nil {
		c.wait_mu.Lock()
		delete(c.wait_chan_map, acct.Biz)
		c.wait_mu.Unlock()
		logger.Error().Err(err).Dur("cost", time.Since(start)).Msg("refresh official account via frontend: request failed")
		return nil, err
	}
	select {
	case cur_acct := <-ch:
		c.wait_mu.Lock()
		delete(c.wait_chan_map, acct.Biz)
		c.wait_mu.Unlock()
		cur_acct.IsEffective = true
		cur_acct.UpdateTime = time.Now().Unix()
		cur_acct.Error = ""
		acct_mu.Lock()
		accounts[cur_acct.Biz] = cur_acct
		acct_mu.Unlock()
		save_accounts()
		logger.Info().
			Str("nickname", cur_acct.Nickname).
			Int64("acct_update_time", cur_acct.UpdateTime).
			Msg("refresh official account via frontend: credential updated")
		go c.pushCredentialToRemoteServer(logger, cur_acct)
		logger.Info().Dur("cost", time.Since(start)).Msg("refresh official account via frontend: completed")
		return cur_acct, nil
	case <-time.After(20 * time.Second):
		c.wait_mu.Lock()
		delete(c.wait_chan_map, acct.Biz)
		c.wait_mu.Unlock()
		logger.Error().Dur("cost", time.Since(start)).Msg("refresh official account via frontend: timeout")
		return nil, errors.New(result.GetMsg(result.CodeTimeout))
	}
}
func (c *OfficialAccountClient) RefreshAllRemoteOfficialAccount() error {
	run_id := c.next_trace_id("refresh_all_remote")
	return c.refreshAllRemoteOfficialAccount(run_id)
}

func (c *OfficialAccountClient) refreshAllRemoteOfficialAccount(run_id string) error {
	c.refresh_mu.Lock()
	if c.is_refreshing {
		c.refresh_mu.Unlock()
		return errors.New("refreshing is already in progress")
	}
	c.is_refreshing = true
	c.refresh_mu.Unlock()
	defer func() {
		c.refresh_mu.Lock()
		c.is_refreshing = false
		c.refresh_mu.Unlock()
	}()

	if err := c.Validate(); err != nil {
		return err
	}
	logger := c.logger.With().
		Str("run_id", run_id).
		Str("origin", c.RemoteServerAddr).
		Logger()
	logger.Info().Msg("refresh all remote official accounts: start")
	report, err := c.refreshRemoteOfficialAccount(logger, c.RemoteServerAddr)
	if err != nil {
		return err
	}
	c.save_refresh_log(report)
	logger.Info().Msg("refresh all remote official accounts: completed")
	return nil
}
func (c *OfficialAccountClient) RefreshRemoteOfficialAccount(origin string) error {
	run_id := c.next_trace_id("refresh_remote")
	logger := c.logger.With().Str("run_id", run_id).Str("origin", origin).Logger()
	_, err := c.refreshRemoteOfficialAccount(logger, origin)
	return err
}

type remoteOfficialAccountJob struct {
	Biz      string
	Nickname string
}

func (c *OfficialAccountClient) refreshRemoteOfficialAccount(logger zerolog.Logger, origin string) (*RefreshReport, error) {
	start_time := time.Now()
	report := &RefreshReport{
		StartTime: start_time.Format("2006-01-02 15:04:05"),
	}
	logger.Info().Msg("refresh remote official accounts: start")
	client := &http.Client{Timeout: 30 * time.Second}
	// var token string
	// if len(c.Tokens) > 0 {
	// 	token = c.Tokens[0]
	// }
	page := 1
	page_size := 200
	var items []struct {
		Nickname string `json:"nickname"`
		Biz      string `json:"biz"`
	}
	remote_total := 0
	for {
		baseURL, err := url.Parse(origin + "/api/mp/list")
		if err != nil {
			logger.Error().Err(err).Msg("refresh remote official accounts: parse request url failed")
			return nil, err
		}
		q := baseURL.Query()
		if c.RefreshToken != "" {
			q.Set("token", c.RefreshToken)
		}
		q.Set("page", strconv.Itoa(page))
		q.Set("page_size", strconv.Itoa(page_size))
		baseURL.RawQuery = q.Encode()
		req, err := http.NewRequest("GET", baseURL.String(), nil)
		if err != nil {
			logger.Error().Err(err).Msg("refresh remote official accounts: build request failed")
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			logger.Error().Err(err).Msg("refresh remote official accounts: request failed")
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			logger.Error().Err(err).Msg("refresh remote official accounts: read response failed")
			return nil, err
		}
		var out struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				List []struct {
					Nickname string `json:"nickname"`
					Biz      string `json:"biz"`
				} `json:"list"`
				Total    int    `json:"total"`
				Page     int    `json:"page"`
				PageSize int    `json:"page_size"`
				Keyword  string `json:"keyword"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			logger.Error().Err(err).Msg("refresh remote official accounts: decode response failed")
			return nil, err
		}
		if out.Code != 0 {
			logger.Error().Int("code", out.Code).Str("msg", out.Msg).Msg("refresh remote official accounts: remote error")
			return nil, fmt.Errorf("remote error: %s (code: %d)", out.Msg, out.Code)
		}
		if page == 1 && out.Data.Total == 0 && out.Data.Page == 0 && out.Data.PageSize == 0 {
			items = out.Data.List
			break
		}

		if out.Data.Total > 0 {
			remote_total = out.Data.Total
		}
		items = append(items, out.Data.List...)
		if len(out.Data.List) == 0 {
			break
		}
		if remote_total > 0 && len(items) >= remote_total {
			break
		}
		if len(out.Data.List) < page_size {
			break
		}
		page++
		if page > 1000 {
			break
		}
	}
	clients := c.ListClients()
	if len(clients) == 0 {
		logger.Error().Msg("refresh remote official accounts: no frontend clients")
		return nil, errors.New(result.GetMsg(result.CodeClientNotReady))
	}
	skip_minutes := c.RefreshSkipMinutes
	skip_seconds := int64(skip_minutes) * 60
	now := time.Now().Unix()
	total := 0
	logger.Info().
		Int("remote_list_count", len(items)).
		Int("client_count", len(clients)).
		Int("skip_threshold_minutes", skip_minutes).
		Msg("refresh remote official accounts: fetched remote list")
	jobs := make(chan remoteOfficialAccountJob, len(items))
	for _, item := range items {
		if item.Biz == "" {
			logger.Warn().Str("nickname", item.Nickname).Msg("refresh remote official accounts: skip item (missing biz)")
			continue
		}
		should_skip := false
		var updateTime int64
		acct_mu.RLock()
		acct, ok := accounts[item.Biz]
		if !ok || acct == nil {
			should_skip = true
		} else {
			updateTime = acct.UpdateTime
			if acct.UpdateTime > 0 && now-acct.UpdateTime <= skip_seconds {
				should_skip = true
			}
			if acct.RefreshUri == "" {
				should_skip = true
			}
		}
		acct_mu.RUnlock()
		if should_skip {
			logger.Info().
				Str("biz", item.Biz).
				Str("nickname", item.Nickname).
				Int64("acct_update_time", updateTime).
				Int64("acct_update_age_sec", now-updateTime).
				Msg("refresh remote official accounts: skip (missing local refresh_uri or within refresh skip threshold)")
			continue
		}
		logger.Info().
			Str("biz", item.Biz).
			Str("nickname", item.Nickname).
			Int64("acct_update_time", updateTime).
			Int64("acct_update_age_sec", now-updateTime).
			Msg("refresh remote official accounts: enqueue")
		jobs <- remoteOfficialAccountJob{Biz: item.Biz, Nickname: item.Nickname}
		total++
	}
	close(jobs)
	if total == 0 {
		logger.Info().Msg("refresh remote official accounts: no jobs to process")
		endTime := time.Now()
		report.EndTime = endTime.Format("2006-01-02 15:04:05")
		report.Duration = endTime.Sub(start_time).String()
		return report, nil
	}
	var wg sync.WaitGroup
	processed := make([]int64, len(clients))
	var success int64
	var processed_total int64
	failures := make([]FailureDetail, 0)
	var failures_mu sync.Mutex

	c.BroadcastProgress(total, 0, 0, 0)

	for i, ws := range clients {
		clientTitle := ""
		if ws != nil {
			clientTitle = ws.title
		}
		wg.Add(1)
		go func(idx int, ws *Client) {
			defer wg.Done()
			workerLogger := logger.With().
				Int("worker_idx", idx).
				Str("client_title", clientTitle).
				Logger()
			workerLogger.Info().Msg("refresh worker: started")
			for job := range jobs {
				biz := job.Biz
				jobLogger := workerLogger.With().
					Str("biz", biz).
					Str("nickname", job.Nickname).
					Logger()
				jobLogger.Info().Msg("refresh job: start")
				_, err := c.refresh_credential_from_frontend(jobLogger, &OfficialAccountBody{Biz: biz}, ws)
				if err != nil {
					jobLogger.Warn().Err(err).Msg("refresh job: on client failed, fallback to any client")
					fallbackWS, pickErr := c.firstClient()
					if pickErr != nil {
						err2 := pickErr
						failures_mu.Lock()
						failures = append(failures, FailureDetail{Biz: biz, Nickname: job.Nickname, Error: err2.Error()})
						failures_mu.Unlock()
						jobLogger.Error().Err(err2).Msg("refresh job: failed")
						acct_mu.Lock()
						existing := accounts[biz]
						if existing == nil {
							existing = &OfficialAccount{Biz: biz, IsEffective: true}
						}
						existing.Error = err2.Error()
						existing.UpdateTime = time.Now().Unix()
						accounts[biz] = existing
						acct_mu.Unlock()
						save_accounts()
						atomic.AddInt64(&processed[idx], 1)

						curr := atomic.AddInt64(&processed_total, 1)
						succ := atomic.LoadInt64(&success)
						failures_mu.Lock()
						fail := len(failures)
						failures_mu.Unlock()
						c.BroadcastProgress(total, int(curr), int(succ), fail)
						continue
					}
					fallbackLogger := jobLogger
					if fallbackWS != nil {
						fallbackLogger = fallbackLogger.With().Str("client_title", fallbackWS.title).Logger()
					}
					_, err2 := c.refresh_credential_from_frontend(fallbackLogger, &OfficialAccountBody{Biz: biz}, fallbackWS)
					if err2 != nil {
						failures_mu.Lock()
						failures = append(failures, FailureDetail{Biz: biz, Nickname: job.Nickname, Error: err2.Error()})
						failures_mu.Unlock()
						jobLogger.Error().Err(err2).Msg("refresh job: failed")
						acct_mu.Lock()
						existing := accounts[biz]
						if existing == nil {
							existing = &OfficialAccount{Biz: biz, IsEffective: true}
						}
						existing.Error = err2.Error()
						existing.UpdateTime = time.Now().Unix()
						accounts[biz] = existing
						acct_mu.Unlock()
						save_accounts()
					} else {
						atomic.AddInt64(&success, 1)
						jobLogger.Info().Msg("refresh job: success (fallback)")
						acct_mu.Lock()
						if acct, ok := accounts[biz]; ok && acct != nil {
							acct.Error = ""
							accounts[biz] = acct
						}
						acct_mu.Unlock()
					}
				} else {
					atomic.AddInt64(&success, 1)
					jobLogger.Info().Msg("refresh job: success")
					acct_mu.Lock()
					if acct, ok := accounts[biz]; ok && acct != nil {
						acct.Error = ""
						accounts[biz] = acct
					}
					acct_mu.Unlock()
				}
				atomic.AddInt64(&processed[idx], 1)

				curr := atomic.AddInt64(&processed_total, 1)
				succ := atomic.LoadInt64(&success)
				failures_mu.Lock()
				fail := len(failures)
				failures_mu.Unlock()
				c.BroadcastProgress(total, int(curr), int(succ), fail)
			}
			workerLogger.Info().Int64("processed", processed[idx]).Msg("refresh worker: completed")
		}(i, ws)
	}
	wg.Wait()
	c.BroadcastProgress(total, total, int(success), len(failures))
	if int(success) == total {
		logger.Info().
			Int("total", total).
			Int64("success", success).
			Msg("refresh remote official accounts: completed")
	} else {
		logger.Warn().
			Int("total", total).
			Int64("success", success).
			Int("failed", len(failures)).
			Msg("refresh remote official accounts: completed with failures")
		for _, f := range failures {
			logger.Error().Str("biz", f.Biz).Str("nickname", f.Nickname).Str("error", f.Error).Msg("refresh remote official accounts: failure detail")
		}
	}
	endTime := time.Now()
	report.EndTime = endTime.Format("2006-01-02 15:04:05")
	report.Duration = endTime.Sub(start_time).String()
	report.Total = total
	report.Success = int(success)
	report.Failed = len(failures)
	report.Failures = failures
	return report, nil
}

func (c *OfficialAccountClient) PushCredentialToRemoteServer(credential *OfficialAccount) error {
	logger := c.logger.With().
		Str("biz", func() string {
			if credential == nil {
				return ""
			}
			return credential.Biz
		}()).
		Logger()
	return c.pushCredentialToRemoteServer(logger, credential)
}

func (c *OfficialAccountClient) pushCredentialToRemoteServer(logger zerolog.Logger, credential *OfficialAccount) error {
	server_addr := c.RemoteServerAddr
	if server_addr == "" || credential == nil {
		logger.Error().Msg("push credential to remote server: server or credential is empty")
		return errors.New("server or credential is empty")
	}
	logger.Info().
		Str("server", server_addr).
		Bool("has_token", c.RefreshToken != "").
		Str("nickname", credential.Nickname).
		Msg("push credential to remote server: start")
	u := server_addr + "/api/mp/refresh"
	if c.RefreshToken != "" {
		u = c.BuildURL(u, map[string]string{"token": c.RefreshToken})
	}
	b, err := json.Marshal(credential)
	if err != nil {
		logger.Error().Err(err).Msg("push credential to remote server: marshal failed")
		return err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", u, bytes.NewReader(b))
	if err != nil {
		logger.Error().Err(err).Msg("push credential to remote server: build request failed")
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	// Add User-Agent to avoid being blocked by Cloudflare
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		logger.Error().Err(err).Msg("push credential to remote server: request failed")
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error().Err(err).Msg("push credential to remote server: read response failed")
		return err
	}
	var out struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		logger.Error().Err(err).Msg("push credential to remote server: decode response failed")
		return err
	}
	if out.Code != 0 {
		logger.Error().Int("code", out.Code).Str("msg", out.Msg).Msg("push credential to remote server: remote error")
		return fmt.Errorf("remote error: %s (code: %d)", out.Msg, out.Code)
	}
	logger.Info().Msg("push credential to remote server: completed")
	return nil
}

func (c *OfficialAccountClient) BuildMsgListURL(acct *OfficialAccount, offset int) string {
	u := "https://mp.weixin.qq.com/mp/profile_ext"
	query := map[string]string{
		"action":      "getmsg",
		"__biz":       acct.Biz,
		"uin":         acct.Uin,
		"key":         acct.Key,
		"pass_ticket": acct.PassTicket,
		"wxtoken":     "",
		"x5":          "0",
		"count":       "10",
		"offset":      strconv.Itoa(offset),
		"f":           "json",
	}
	target_url := c.BuildURL(u, query)
	return target_url
}

// 获取指定公众号的推送列表
func (c *OfficialAccountClient) FetchMsgList(biz string, offset int) (*OfficialMsgListResp, error) {
	logger := c.logger.With().
		Str("biz", biz).
		Int("offset", offset).
		Logger()
	return c.fetchMsgList(logger, biz, offset)
}

type codedError struct {
	code     int
	msg      string
	err      error
	location string
}

func (e *codedError) Error() string {
	return e.msg
}

func (e *codedError) Unwrap() error {
	return e.err
}

func newCodedError(code int, msg string, err error) *codedError {
	pc, file, line, _ := runtime.Caller(1)
	funcName := runtime.FuncForPC(pc).Name()
	return &codedError{
		code:     code,
		msg:      msg,
		err:      err,
		location: fmt.Sprintf("%s:%d:%s", filepath.Base(file), line, filepath.Base(funcName)),
	}
}

func codedErrorOf(err error) (int, string, string, bool) {
	var ce *codedError
	if errors.As(err, &ce) {
		return ce.code, ce.msg, ce.location, true
	}
	return 0, "", "", false
}

func safeLogErr(err error) string {
	if err == nil {
		return ""
	}
	in := err.Error()
	re := regexp.MustCompile(`https?://[^\s]+`)
	return re.ReplaceAllStringFunc(in, func(m string) string {
		u, parseErr := url.Parse(m)
		if parseErr != nil {
			return m
		}
		q := u.Query()
		if len(q) == 0 {
			return m
		}
		for _, k := range []string{"uin", "key", "pass_ticket", "appmsg_token"} {
			if q.Has(k) {
				q.Set(k, "REDACTED")
			}
		}
		u.RawQuery = q.Encode()
		return u.String()
	})
}

func safeNetReason(err error) string {
	if err == nil {
		return ""
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return result.GetMsg(result.CodeTimeout)
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if strings.Contains(strings.ToLower(opErr.Err.Error()), "refused") {
			return "连接被拒绝"
		}
	}
	if strings.Contains(strings.ToLower(err.Error()), "no such host") {
		return "DNS 解析失败"
	}
	return "网络请求失败"
}

func (c *OfficialAccountClient) fetchMsgList(logger zerolog.Logger, biz string, offset int) (*OfficialMsgListResp, error) {
	logger.Info().Msg("fetch msg list: start")
	if biz == "" {
		return nil, newCodedError(result.CodeMissingBiz, result.GetMsg(result.CodeMissingBiz), nil)
	}
	var existing *OfficialAccount
	acct_mu.RLock()
	if _, ok := accounts[biz]; ok {
		data := accounts[biz]
		existing = data
	}
	acct_mu.RUnlock()
	if existing == nil {
		return nil, newCodedError(result.CodeAccountNotFound, result.GetMsg(result.CodeAccountNotFound), nil)
	}
	target_url := c.BuildMsgListURL(existing, offset)
	params := url.Values{}
	params.Add("action", "home")
	params.Add("__biz", existing.Biz)
	params.Add("scene", "124")
	params.Add("uin", existing.Uin)
	params.Add("key", existing.Key)
	params.Add("devicetype", "UnifiedPCWindows")
	params.Add("version", "f2541022")
	params.Add("lang", "zh_CN")
	params.Add("a8scene", "1")
	params.Add("acctmode", "0")
	params.Add("pass_ticket", existing.PassTicket)
	referer := `https://mp.weixin.qq.com/mp/profile_ext?${refererParams.toString()}`
	resp, err := c.Fetch(target_url, referer)
	if err != nil {
		fmt.Printf("c.Fetch msg list: error: %s\n", err.Error())
		code := result.CodeFetchMsgFailed
		msg := result.GetMsg(code)
		reason := safeNetReason(err)
		if reason == result.GetMsg(result.CodeTimeout) {
			code = result.CodeTimeout
			msg = reason
		} else if reason != "" {
			msg = fmt.Sprintf("%s: %s", msg, reason)
		}
		return nil, newCodedError(code, msg, err)
	}
	defer resp.Body.Close()
	resp_bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, newCodedError(result.CodeFetchMsgFailed, "读取响应失败", err)
	}
	var data OfficialMsgListResp
	err = json.Unmarshal(resp_bytes, &data)
	if err != nil {
		fmt.Printf("json.Unmarshal msg list: error: %s\n", err.Error())
		return nil, newCodedError(result.CodeDataParseFailed, result.GetMsg(result.CodeDataParseFailed), err)
	}
	if data.Ret != 0 {
		fmt.Printf("data.Ret != 0 msg list: error: %s\n", string(resp_bytes))
		if data.Ret == -3 {
			existing.IsEffective = false
			save_accounts()
			return nil, newCodedError(result.CodeAccountExpired, result.GetMsg(result.CodeAccountExpired), nil)
		}
		if data.Ret == -6 {
			existing.IsEffective = false
			save_accounts()
			return nil, newCodedError(result.CodeAccountBanned, result.GetMsg(result.CodeAccountBanned), nil)
		}
		msg := data.ErrMsg
		if strings.TrimSpace(msg) == "" {
			msg = result.GetMsg(result.CodeFetchMsgFailed)
		}
		return nil, newCodedError(result.CodeFetchMsgFailed, msg, nil)
	}
	logger.Info().Int("ret", data.Ret).Msg("fetch msg list: completed")
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

func friendlyErrMsg(err error) string {
	if err == nil {
		return ""
	}
	switch err.Error() {
	case "Please adding Credentials first":
		return "请先添加凭证"
	case "the Account is expired":
		return "账号凭证已过期"
	case "request timeout":
		return "请求超时"
	default:
		return err.Error()
	}
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

var mp_json_filepath = "mp.json"

func save_accounts() {
	acct_mu.RLock()
	defer acct_mu.RUnlock()

	data, err := json.MarshalIndent(accounts, "", "  ")
	if err != nil {
		fmt.Println("saveAccounts marshal err:", err)
		return
	}

	err = os.WriteFile(mp_json_filepath, data, 0644)
	if err != nil {
		fmt.Println("saveAccounts write err:", err)
	}
}
func load_accounts() {
	data, err := os.ReadFile(mp_json_filepath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Println("loadAccounts read err:", err)
		}
		return
	}

	acct_mu.Lock()
	defer acct_mu.Unlock()

	err = json.Unmarshal(data, &accounts)
	if err != nil {
		fmt.Println("loadAccounts unmarshal err:", err)
	}
}

var refresh_log_filepath = "refresh_log.json"

type FailureDetail struct {
	Biz      string `json:"biz"`
	Nickname string `json:"nickname"`
	Error    string `json:"error"`
}

type RefreshReport struct {
	StartTime string          `json:"start_time"`
	EndTime   string          `json:"end_time"`
	Duration  string          `json:"duration"`
	Total     int             `json:"total"`
	Success   int             `json:"success"`
	Failed    int             `json:"failed"`
	Failures  []FailureDetail `json:"failures"`
}

func (c *OfficialAccountClient) save_refresh_log(report *RefreshReport) {
	if report == nil {
		return
	}
	var logs []*RefreshReport
	if c.RefreshToken != "" {
		// Try to load existing logs
		// Assuming we save to refresh_log.json in the same dir as mp.json
		fp := refresh_log_filepath
		if filepath.IsAbs(mp_json_filepath) {
			fp = filepath.Join(filepath.Dir(mp_json_filepath), "refresh_log.json")
		}

		data, err := os.ReadFile(fp)
		if err == nil {
			_ = json.Unmarshal(data, &logs)
		}

		// Keep last 100 logs to avoid file growing too large
		if len(logs) > 100 {
			logs = logs[len(logs)-100:]
		}
		logs = append(logs, report)

		data, err = json.MarshalIndent(logs, "", "  ")
		if err != nil {
			c.logger.Error().Err(err).Msg("save refresh log: marshal failed")
			return
		}

		err = os.WriteFile(fp, data, 0644)
		if err != nil {
			c.logger.Error().Err(err).Msg("save refresh log: write failed")
		}
	}
}
