package wxmp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"golang.org/x/net/html"

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
	Biz              string `json:"biz"`
	Nickname         string `json:"nickname"`
	AvatarURL        string `json:"avatar_url"`
	AuthorId         string `json:"author_id"`
	Uin              string `json:"uin"`
	Key              string `json:"key"`
	PassTicket       string `json:"pass_ticket"`
	AppmsgToken      string `json:"appmsg_token"`
	Cookie           string `json:"cookie"`
	CookieExpiration int64  `json:"cookie_expiration"`
	RefreshUri       string `json:"refresh_uri"`
	IsEffective      bool   `json:"is_effective"`
	CreatedAt        int64  `json:"created_at"`
	UpdateTime       int64  `json:"update_time"`
	Error            string `json:"error"`
}

func (acct *OfficialAccount) MergeFrom(source *OfficialAccount) {
	if source.Nickname != "" {
		acct.Nickname = source.Nickname
	}
	if source.AvatarURL != "" {
		acct.AvatarURL = source.AvatarURL
	}
	if source.AuthorId != "" {
		acct.AuthorId = source.AuthorId
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

type OfficialAccountServer struct {
	client                    *Client
	logger                    *zerolog.Logger
	RemoteServerAddr          string
	RefreshToken              string
	APIServerProtocol         string
	APIServerHostname         string
	APIServerPort             int
	RemoteServerProtocol      string
	RemoteServerHostname      string
	RemoteServerPort          int
	RefreshSkipMinutes        int
	AccountIdsRefreshInterval []string
	MaxWebsocketClients       int
	Tokens                    []string
	ws_clients                map[*WebsocketClient]bool
	ws_mu                     sync.RWMutex
	requests                  map[string]chan ClientWebsocketResponse
	requests_mu               sync.RWMutex
	cache                     *cache.Cache
	req_seq                   uint64
	wait_chan_map             map[string]chan *OfficialAccount
	wait_mu                   sync.Mutex
	refresh_mu                sync.Mutex
	is_refreshing             bool
}

func (c *OfficialAccountServer) next_trace_id(prefix string) string {
	n := atomic.AddUint64(&c.req_seq, 1)
	return fmt.Sprintf("%s-%d", prefix, n)
}

func NewOfficialAccountServer(cfg *OfficialAccountConfig, parent_logger *zerolog.Logger) *OfficialAccountServer {
	logger := parent_logger.With().Str("service", "OfficialAccountServer").Logger()
	c := &OfficialAccountServer{
		client:                    NewClient(cfg, parent_logger),
		logger:                    &logger,
		APIServerProtocol:         cfg.Protocol,
		APIServerHostname:         cfg.Hostname,
		APIServerPort:             cfg.Port,
		RemoteServerProtocol:      cfg.RemoteServerProtocol,
		RemoteServerHostname:      cfg.RemoteServerHostname,
		RemoteServerPort:          cfg.RemoteServerPort,
		RefreshSkipMinutes:        cfg.RefreshSkipMinutes,
		AccountIdsRefreshInterval: cfg.AccountIdsRefreshInterval,
		MaxWebsocketClients:       5,
		RefreshToken:              cfg.RefreshToken,
		Tokens:                    make([]string, 0),
		ws_clients:                make(map[*WebsocketClient]bool),
		requests:                  make(map[string]chan ClientWebsocketResponse),
		cache:                     cache.New(),
		req_seq:                   uint64(time.Now().UnixNano()),
		wait_chan_map:             make(map[string]chan *OfficialAccount),
	}
	if cfg.WorkDir != "" {
		mp_json_filepath = filepath.Join(cfg.WorkDir, "mp.json")
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
	// if !cfg.RemoteMode && len(c.AccountIdsRefreshInterval) > 0 {
	// 	var valid_accounts []string
	// 	acct_mu.RLock()
	// 	for _, biz := range c.AccountIdsRefreshInterval {
	// 		if _, ok := accounts[biz]; ok {
	// 			valid_accounts = append(valid_accounts, biz)
	// 		}
	// 	}
	// 	acct_mu.RUnlock()
	// 	c.AccountIdsRefreshInterval = valid_accounts
	// 	if len(c.AccountIdsRefreshInterval) > 0 {
	// 		go func() {
	// 			ticker := time.NewTicker(28 * time.Minute)
	// 			defer ticker.Stop()
	// 			for range ticker.C {
	// 				c.RefreshSpecifiedOfficialAccountList(c.AccountIdsRefreshInterval)
	// 			}
	// 		}()
	// 	}
	// }
	return c
}

// SetPersistentCache configures the HTML cache used by the server's scraper
// client.
func (c *OfficialAccountServer) SetPersistentCache(file_cache *cache.CacheProvider) {
	if c == nil || c.client == nil {
		return
	}
	c.client.SetPersistentCache(file_cache)
}

func (c *OfficialAccountServer) ServeWebsocket(writer http.ResponseWriter, request *http.Request) {
	conn, err := official_ws_upgrader.Upgrade(writer, request, nil)
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
	client := &WebsocketClient{conn: conn, send: make(chan []byte, 256)}
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
		// Response from frontend to ws api request
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

type FetchMessageListParams struct {
	Biz        string
	Offset     int
	Uin        string
	Key        string
	PassTicket string
}

func (c *OfficialAccountServer) FetchMessageList(params FetchMessageListParams) (*OfficialMsgListResp, error) {
	trace_id := c.next_trace_id("fetch_msg_list")
	logger := c.logger.With().
		Str("trace_id", trace_id).
		Str("biz", params.Biz).
		Int("offset", params.Offset).
		Logger()

	var data *OfficialMsgListResp
	var err error
	if params.Uin != "" && params.Key != "" {
		inline := &OfficialAccount{
			Biz:        params.Biz,
			Uin:        params.Uin,
			Key:        params.Key,
			PassTicket: params.PassTicket,
		}
		data, err = c.client.fetch_msg_list_inline(logger, inline, params.Offset)
	} else {
		if params.Biz == "" {
			return nil, new_scraper_error(ErrorKindMissingBiz, ErrorMessage(ErrorKindMissingBiz), nil)
		}
		acct := get_official_account(params.Biz)
		if acct == nil {
			return nil, new_scraper_error(ErrorKindAccountNotFound, ErrorMessage(ErrorKindAccountNotFound), nil)
		}
		data, err = c.fetch_msg_list(logger, acct, params.Offset)
	}
	if err != nil {
		kind := ErrorKindFetchMessage
		message := ErrorMessage(kind)
		loc := ""
		if error_kind, error_message, error_location, ok := scraper_error_of(err); ok {
			kind = error_kind
			message = error_message
			loc = error_location
		}
		logger.Error().
			Str("error_kind", string(kind)).
			Str("resp_msg", message).
			Str("err", safe_log_err(err)).
			Str("location", loc).
			Msg("fetch msg list: failed")
		return nil, err
	}
	return data, nil
}

func (c *OfficialAccountServer) FetchArticleList(biz string) (*ArticleListResponse, error) {
	acct := get_official_account(biz)
	if acct == nil {
		return nil, new_scraper_error(ErrorKindAccountNotFound, ErrorMessage(ErrorKindAccountNotFound), nil)
	}
	previous_cookie := acct.Cookie
	previous_expiration := acct.CookieExpiration
	defer func() {
		if acct.Cookie != previous_cookie || acct.CookieExpiration != previous_expiration {
			save_accounts()
		}
	}()
	return c.client.FetchArticleList(acct)
}

func (c *OfficialAccountServer) FetchBizMsgList(username, offset string) (json.RawMessage, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, new_scraper_error(
			ErrorKindInvalidArgument,
			ErrorMessage(ErrorKindInvalidArgument),
			errors.New("missing username"),
		)
	}
	response, err := c.RequestFrontend(
		"key:mp:bizmsglist",
		map[string]string{"username": username, "offset": offset},
		20*time.Second,
	)
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

func get_official_account(biz string) *OfficialAccount {
	acct_mu.RLock()
	defer acct_mu.RUnlock()
	return accounts[biz]
}

func (c *OfficialAccountServer) fetch_msg_list(logger zerolog.Logger, acct *OfficialAccount, offset int) (*OfficialMsgListResp, error) {
	data, err := c.client.fetch_msg_list(logger, acct, offset, true)
	if err == nil {
		return data, nil
	}
	error_kind, _, _, ok := scraper_error_of(err)
	if !ok || (error_kind != ErrorKindAccountExpired && error_kind != ErrorKindAccountBanned) {
		return nil, err
	}
	acct_mu.Lock()
	if stored_acct, found := accounts[acct.Biz]; found {
		stored_acct.IsEffective = false
	}
	acct_mu.Unlock()
	save_accounts()
	return nil, err
}

type OfficialAccountLink struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
}

type OfficialAccountSummary struct {
	Biz         string                `json:"biz"`
	Nickname    string                `json:"nickname"`
	AvatarURL   string                `json:"avatar_url"`
	IsEffective bool                  `json:"is_effective"`
	CreatedAt   int64                 `json:"created_at"`
	UpdateTime  int64                 `json:"update_time"`
	Error       string                `json:"error"`
	RefreshURI  string                `json:"refresh_uri,omitempty"`
	Links       []OfficialAccountLink `json:"links"`
}

type ListOfficialAccountsParams struct {
	Page      int
	PageSize  int
	Keyword   string
	Effective *bool
	Token     string
}

type OfficialAccountList struct {
	List     []OfficialAccountSummary `json:"list"`
	Total    int                      `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
	Keyword  string                   `json:"keyword"`
}

func (c *OfficialAccountServer) ListOfficialAccounts(params ListOfficialAccountsParams) OfficialAccountList {
	page := params.Page
	if page < 1 {
		page = 1
	}
	page_size := params.PageSize
	if page_size <= 0 {
		page_size = 10
	}
	if page_size > 200 {
		page_size = 200
	}
	keyword := strings.TrimSpace(params.Keyword)
	keyword_lower := strings.ToLower(keyword)

	var list []OfficialAccountSummary
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
		summary := OfficialAccountSummary{
			Biz:         acct.Biz,
			Nickname:    acct.Nickname,
			AvatarURL:   acct.AvatarURL,
			IsEffective: acct.IsEffective,
			CreatedAt:   acct.CreatedAt,
			UpdateTime:  acct.UpdateTime,
			Error:       acct.Error,
		}
		summary.RefreshURI = acct.RefreshUri

		// Build Links
		var links []OfficialAccountLink

		// 1. author_uri
		if acct.AuthorId != "" {
			u := url.URL{
				Scheme: "https",
				Host:   "mp.weixin.qq.com",
				Path:   "/mp/author",
			}
			q := u.Query()
			q.Set("action", "show")
			q.Set("__biz", acct.Biz)
			q.Set("idx", "1")
			q.Set("author_id", acct.AuthorId)
			q.Set("scene", "142")
			q.Set("rscene", "128")
			q.Set("uin", acct.Uin)
			q.Set("key", acct.Key)
			q.Set("pass_ticket", acct.PassTicket)
			q.Set("devicetype", "UnifiedPCMac")
			q.Set("version", "f2640619")
			q.Set("lang", "zh_CN")
			q.Set("ascene", "1")
			q.Set("acctmode", "0")
			q.Set("countrycode", "CN")
			u.RawQuery = q.Encode()
			links = append(links, OfficialAccountLink{Name: "author_uri", URI: u.String()})
		}

		// 2. home_uri
		{
			u := url.URL{
				Scheme: "https",
				Host:   "mp.weixin.qq.com",
				Path:   "/mp/profile_ext",
			}
			q := u.Query()
			q.Set("action", "home")
			q.Set("__biz", acct.Biz)
			q.Set("scene", "124")
			u.RawQuery = q.Encode()
			links = append(links, OfficialAccountLink{Name: "home_uri", URI: u.String() + "#wechat_redirect"})
		}

		origin := c.APIServerProtocol + "://" + c.APIServerHostname
		if c.APIServerPort > 0 && c.APIServerPort != 80 {
			origin += fmt.Sprintf(":%d", c.APIServerPort)
		}
		// 3. msg api
		{
			u := url.URL{
				Path: origin + "/api/mp/msg/list",
			}
			q := u.Query()
			q.Set("biz", acct.Biz)
			if params.Token != "" {
				q.Set("token", params.Token)
			}
			u.RawQuery = q.Encode()
			links = append(links, OfficialAccountLink{Name: "msg_api", URI: u.String()})
		}

		// 4. article api
		{
			u := url.URL{
				Path: origin + "/api/mp/article/list",
			}
			q := u.Query()
			q.Set("biz", acct.Biz)
			if params.Token != "" {
				q.Set("token", params.Token)
			}
			u.RawQuery = q.Encode()
			links = append(links, OfficialAccountLink{Name: "article_api", URI: u.String()})
		}

		summary.Links = links

		if params.Effective != nil {
			if summary.IsEffective != *params.Effective {
				continue
			}
		}

		if keyword_lower == "" {
			list = append(list, summary)
		} else {
			biz_lower := strings.ToLower(summary.Biz)
			nickname_lower := strings.ToLower(summary.Nickname)
			if strings.Contains(biz_lower, keyword_lower) || strings.Contains(nickname_lower, keyword_lower) {
				list = append(list, summary)
			}
		}
	}
	acct_mu.Unlock()
	if changed {
		go save_accounts()
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
		total_pages := (total + page_size - 1) / page_size
		if page > total_pages {
			page = total_pages
		}
	}
	start := (page - 1) * page_size
	if start > total {
		start = total
	}
	end := start + page_size
	if end > total {
		end = total
	}
	paged := list[start:end]
	return OfficialAccountList{
		List:     paged,
		Total:    total,
		Page:     page,
		PageSize: page_size,
		Keyword:  keyword,
	}
}

func (c *OfficialAccountServer) DeleteOfficialAccount(biz string) {
	acct_mu.Lock()
	delete(accounts, biz)
	acct_mu.Unlock()

	save_accounts()
}

func (c *OfficialAccountServer) RefreshOfficialAccount(body OfficialAccount) {
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
		if new_acct.AuthorId == "" && body.AuthorId != "" {
			new_acct.AuthorId = body.AuthorId
		}
		new_acct.IsEffective = true
		if new_acct.CreatedAt == 0 {
			new_acct.CreatedAt = now
		}
		new_acct.UpdateTime = now
		new_acct.Error = ""
		target_acct = &new_acct
		accounts[body.Biz] = target_acct
	} else {
		// if len(accounts) >= 20 {
		// 	// Account limits can be enforced by the application adapter.
		// 	return
		// }
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
		Msg("refresh official account event: stored and notified")
	is_manually_refresh := !ok
	if is_manually_refresh {
		go c.push_credential_to_remote_server(logger, target_acct)
	}
}

func (c *OfficialAccountServer) RefreshSpecifiedOfficialAccountList(biz_list []string) error {
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
	var jobs []remote_official_account_job
	for _, t := range targets {
		if t.RefreshUri != "" {
			jobs = append(jobs, remote_official_account_job{Biz: t.Biz, Nickname: t.Nickname})
		}
	}

	if len(jobs) == 0 {
		return nil
	}

	// Check clients
	clients := c.ListClients()
	if len(clients) == 0 {
		return new_scraper_error(ErrorKindClientNotReady, ErrorMessage(ErrorKindClientNotReady), nil)
	}

	// Prepare worker pool
	total := len(jobs)
	job_chan := make(chan remote_official_account_job, total)
	for _, j := range jobs {
		job_chan <- j
	}
	close(job_chan)

	var wg sync.WaitGroup

	logger := c.logger.With().Str("action", "refresh_specified_list").Logger()

	for i, ws := range clients {
		client_title := ""
		if ws != nil {
			client_title = ws.title
		}
		wg.Add(1)
		go func(idx int, ws *WebsocketClient) {
			defer wg.Done()
			worker_logger := logger.With().
				Int("worker_idx", idx).
				Str("client_title", client_title).
				Logger()

			for job := range job_chan {
				biz := job.Biz
				job_logger := worker_logger.With().
					Str("biz", biz).
					Str("nickname", job.Nickname).
					Logger()

				_, err := c.refresh_credential_from_frontend(job_logger, &OfficialAccountBody{Biz: biz}, ws)

				if err != nil {
					// Fallback logic
					job_logger.Warn().Err(err).Msg("refresh job: on client failed, fallback to any client")
					fallback_ws, pick_err := c.first_client()
					if pick_err != nil {
						// Fallback failed
						err2 := pick_err

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
						fallback_logger := job_logger
						if fallback_ws != nil {
							fallback_logger = fallback_logger.With().Str("client_title", fallback_ws.title).Logger()
						}
						_, err2 := c.refresh_credential_from_frontend(fallback_logger, &OfficialAccountBody{Biz: biz}, fallback_ws)
						if err2 != nil {
							acct_mu.Lock()
							if existing, ok := accounts[biz]; ok {
								existing.Error = err2.Error()
								existing.UpdateTime = time.Now().Unix()
							}
							acct_mu.Unlock()
							save_accounts()
						} else {
							acct_mu.Lock()
							if acct, ok := accounts[biz]; ok {
								acct.Error = ""
							}
							acct_mu.Unlock()
						}
					}
				} else {
					acct_mu.Lock()
					if acct, ok := accounts[biz]; ok {
						acct.Error = ""
					}
					acct_mu.Unlock()
				}
			}
		}(i, ws)
	}

	wg.Wait()

	return nil
}

type RSSParams struct {
	Biz            string
	Offset         int
	IncludeContent bool
	ProxyLinks     bool
	ProxyCover     bool
	SelfURL        string
}

func (c *OfficialAccountServer) BuildRSS(params RSSParams) (*AtomFeed, error) {
	if params.Biz == "" {
		return nil, new_scraper_error(ErrorKindMissingBiz, ErrorMessage(ErrorKindMissingBiz), nil)
	}
	cache_key := fmt.Sprintf("rss:%s:%t:%t:%t", params.Biz, params.ProxyLinks, params.IncludeContent, params.ProxyCover)
	if val, found := c.cache.Get(cache_key); found {
		if atom, ok := val.(AtomFeed); ok {
			return &atom, nil
		}
	}
	trace_id := c.next_trace_id("fetch_msg_list")
	logger := c.logger.With().
		Str("trace_id", trace_id).
		Str("biz", params.Biz).
		Int("offset", params.Offset).
		Logger()
	acct := get_official_account(params.Biz)
	if acct == nil {
		return nil, new_scraper_error(ErrorKindAccountNotFound, ErrorMessage(ErrorKindAccountNotFound), nil)
	}
	data, err := c.fetch_msg_list(logger, acct, params.Offset)
	if err != nil {
		kind := ErrorKindFetchMessage
		message := ErrorMessage(kind)
		loc := ""
		if error_kind, error_message, error_location, ok := scraper_error_of(err); ok {
			kind = error_kind
			message = error_message
			loc = error_location
		}
		logger.Error().
			Str("error_kind", string(kind)).
			Str("resp_msg", message).
			Str("err", safe_log_err(err)).
			Str("location", loc).
			Msg("fetch msg list: failed")
		return nil, err
	}
	var list struct {
		List []OfficialAccountMsgListRespItem `json:"list"`
	}
	err = json.Unmarshal([]byte(data.MsgList), &list)
	if err != nil {
		return nil, new_scraper_error(ErrorKindDataParse, ErrorMessage(ErrorKindDataParse), err)
	}
	feed_title := acct.Nickname
	if feed_title == "" {
		feed_title = params.Biz
	}
	feed_uri := fmt.Sprintf("https://mp.weixin.qq.com/mp/profile_ext?action=home&__biz=%s&scene=124", acct.Biz)
	build_url := func(u string) string {
		if u == "" {
			return ""
		}
		if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			return u
		}
		return "https://mp.weixin.qq.com" + u
	}

	build_entry := func(title, digest, content_url, cover, author string, file_id int, publish_date string, authors ...string) AtomEntry {
		u := build_url(html.UnescapeString(content_url))
		if params.ProxyLinks && c.RemoteServerAddr != "" {
			u = fmt.Sprintf("%s/mp/proxy?url=%s", c.RemoteServerAddr, url.QueryEscape(u))
		}
		desc := digest
		var thumb *MediaThumbnail
		if cover != "" {
			// cover = html.UnescapeString(cover)
			if (params.ProxyLinks || params.ProxyCover) && c.RemoteServerAddr != "" {
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
			id = fmt.Sprintf("%s#%d", params.Biz, file_id)
		}
		return AtomEntry{
			ID:        id,
			Title:     title,
			Updated:   publish_date,
			Published: publish_date,
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
		entries = append(entries, build_entry(
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
				entries = append(entries, build_entry(
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
	links = append(links, AtomLink{Rel: "self", Href: params.SelfURL})
	alt := "https://mp.weixin.qq.com/mp/profile_ext?action=home&__biz=" + params.Biz
	links = append(links, AtomLink{Rel: "alternate", Href: alt})
	if data.HasMore != 0 && data.NextOffset > 0 {
		if next_url, parse_err := url.Parse(params.SelfURL); parse_err == nil {
			query := next_url.Query()
			query.Set("offset", strconv.Itoa(data.NextOffset))
			next_url.RawQuery = query.Encode()
			links = append(links, AtomLink{Rel: "next", Href: next_url.String()})
		}
	}
	if params.IncludeContent {
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
				content := c.client.FetchFullContent(href)
				if content != "" {
					entries[idx].Content.Body = content
				}
			}(i, u)
		}
		wg.Wait()
	}
	atom := AtomFeed{
		ID:        params.Biz,
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
	return &atom, nil
}

func (c *OfficialAccountServer) FetchOfficialAccountProxy(target_url string) (*http.Response, error) {
	return c.client.FetchProxy(target_url)
}

type OfficialAccountClientStatus struct {
	Title     string `json:"title"`
	Available bool   `json:"available"`
	LastPing  int64  `json:"last_ping"`
}

func (c *OfficialAccountServer) ClientStatuses() []OfficialAccountClientStatus {
	var list []OfficialAccountClientStatus
	now := time.Now().Unix()
	c.ws_mu.RLock()
	for cl := range c.ws_clients {
		healthy := (now - cl.last_ping) <= 60
		list = append(list, OfficialAccountClientStatus{
			Title:     cl.title,
			Available: healthy,
			LastPing:  cl.last_ping,
		})
	}
	c.ws_mu.RUnlock()
	return list
}

func (c *OfficialAccountServer) RemoteServerAddress() string {
	return c.RemoteServerAddr
}

func (c *OfficialAccountServer) ValidateToken(t string) bool {
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

func (c *OfficialAccountServer) ValidateRefreshToken(token string) bool {
	return token == c.RefreshToken
}

func (c *OfficialAccountServer) Validate() error {
	c.ws_mu.RLock()
	empty := len(c.ws_clients) == 0
	c.ws_mu.RUnlock()
	if empty {
		return new_scraper_error(ErrorKindClientNotReady, ErrorMessage(ErrorKindClientNotReady), nil)
	}
	return nil
}
func (c *OfficialAccountServer) EnsureFrontendReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		c.ws_mu.RLock()
		ready := len(c.ws_clients) > 0
		c.ws_mu.RUnlock()
		if ready {
			return nil
		}
		if time.Now().After(deadline) {
			return new_scraper_error(ErrorKindClientNotReady, ErrorMessage(ErrorKindClientNotReady), nil)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
func (c *OfficialAccountServer) RequestFrontend(endpoint string, body interface{}, timeout time.Duration) (*ClientWebsocketResponse, error) {
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
	var client *WebsocketClient
	for cl := range c.ws_clients {
		client = cl
		break
	}
	c.ws_mu.RUnlock()
	if client == nil {
		return nil, new_scraper_error(ErrorKindClientNotReady, ErrorMessage(ErrorKindClientNotReady), nil)
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
				return nil, new_scraper_error(ErrorKindClientBusy, ErrorMessage(ErrorKindClientBusy), nil)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
WAIT_RESP:
	select {
	case resp := <-resp_chan:
		return &resp, nil
	case <-time.After(timeout):
		return nil, new_scraper_error(ErrorKindTimeout, ErrorMessage(ErrorKindTimeout), nil)
	}
}

type frontend_fetch_page_content_response struct {
	ErrCode int                      `json:"errCode"`
	ErrMsg  string                   `json:"errMsg"`
	Data    FetchPageContentResponse `json:"data"`
}

const (
	jsapi_default_timeout_ms = 15000
	jsapi_event_timeout_ms   = 30000
	jsapi_max_timeout_ms     = 120000
)

type frontend_jsapi_response struct {
	ErrCode int             `json:"errCode"`
	ErrMsg  string          `json:"errMsg"`
	Data    json.RawMessage `json:"data"`
}

// FetchPageContent asks a connected official-account page to fetch one page
// with its browser session and return the response HTML.
func (c *OfficialAccountServer) FetchPageContent(raw_url string) (*FetchPageContentResponse, error) {
	target_url := strings.TrimSpace(raw_url)
	if target_url == "" {
		return nil, new_scraper_error(ErrorKindMissingURL, ErrorMessage(ErrorKindMissingURL), nil)
	}
	parsed_url, err := url.Parse(target_url)
	if err != nil ||
		(parsed_url.Scheme != "http" && parsed_url.Scheme != "https") ||
		!strings.EqualFold(parsed_url.Hostname(), "mp.weixin.qq.com") {
		if err == nil {
			err = fmt.Errorf("unsupported wxmp page URL: %s", target_url)
		}
		return nil, new_scraper_error(ErrorKindInvalidArgument, ErrorMessage(ErrorKindInvalidArgument), err)
	}
	response, err := c.RequestFrontend(
		"key:fetch_page_content",
		FetchPageContentParams{URL: target_url},
		20*time.Second,
	)
	if err != nil {
		return nil, err
	}
	var frontend_response frontend_fetch_page_content_response
	if err := json.Unmarshal(response.Data, &frontend_response); err != nil {
		return nil, new_scraper_error(ErrorKindDataParse, ErrorMessage(ErrorKindDataParse), err)
	}
	if frontend_response.ErrCode != 0 {
		response_err := errors.New(strings.TrimSpace(frontend_response.ErrMsg))
		return nil, new_scraper_error(
			ErrorKindFetchPageContent,
			ErrorMessage(ErrorKindFetchPageContent),
			response_err,
		)
	}
	return &frontend_response.Data, nil
}

// CallJSAPI forwards an invoke, call, ready, capability query, or event-wait
// operation to a connected WeChat official-account page.
func (c *OfficialAccountServer) CallJSAPI(request JSAPIRequest) (json.RawMessage, error) {
	request.Operation = strings.ToLower(strings.TrimSpace(request.Operation))
	if request.Operation == "" {
		request.Operation = "invoke"
	}
	if request.Operation == "on" {
		request.Operation = "wait_event"
	}
	if request.Operation == "remove" {
		request.Operation = "remove_event"
	}
	request.Method = strings.TrimSpace(request.Method)
	request.Event = strings.TrimSpace(request.Event)

	switch request.Operation {
	case "capabilities", "ready":
	case "invoke", "call":
		if request.Method == "" {
			return nil, new_scraper_error(
				ErrorKindInvalidArgument,
				ErrorMessage(ErrorKindInvalidArgument),
				errors.New("missing JSAPI method"),
			)
		}
	case "wait_event", "remove_event":
		if request.Event == "" {
			return nil, new_scraper_error(
				ErrorKindInvalidArgument,
				ErrorMessage(ErrorKindInvalidArgument),
				errors.New("missing JSAPI event"),
			)
		}
	default:
		return nil, new_scraper_error(
			ErrorKindInvalidArgument,
			ErrorMessage(ErrorKindInvalidArgument),
			fmt.Errorf("unsupported JSAPI operation: %s", request.Operation),
		)
	}
	if len(request.Args) > 0 && !json.Valid(request.Args) {
		return nil, new_scraper_error(
			ErrorKindInvalidArgument,
			ErrorMessage(ErrorKindInvalidArgument),
			errors.New("invalid JSAPI args JSON"),
		)
	}

	default_timeout_ms := jsapi_default_timeout_ms
	if request.Operation == "wait_event" {
		default_timeout_ms = jsapi_event_timeout_ms
	}
	if request.TimeoutMS <= 0 {
		request.TimeoutMS = default_timeout_ms
	}
	if request.TimeoutMS > jsapi_max_timeout_ms {
		request.TimeoutMS = jsapi_max_timeout_ms
	}

	response_timeout := time.Duration(request.TimeoutMS+2000) * time.Millisecond
	response, err := c.RequestFrontend("key:jsapi", request, response_timeout)
	if err != nil {
		return nil, err
	}
	var frontend_response frontend_jsapi_response
	if err := json.Unmarshal(response.Data, &frontend_response); err != nil {
		return nil, new_scraper_error(ErrorKindDataParse, ErrorMessage(ErrorKindDataParse), err)
	}
	if frontend_response.ErrCode != 0 {
		frontend_error_message := strings.TrimSpace(frontend_response.ErrMsg)
		if frontend_error_message == "" {
			frontend_error_message = fmt.Sprintf("frontend JSAPI error: %d", frontend_response.ErrCode)
		}
		public_error_message := fmt.Sprintf(
			"%s [%d]: %s",
			ErrorMessage(ErrorKindJSAPI),
			frontend_response.ErrCode,
			frontend_error_message,
		)
		return nil, new_scraper_error(
			ErrorKindJSAPI,
			public_error_message,
			errors.New(frontend_error_message),
		)
	}
	if len(frontend_response.Data) == 0 {
		return json.RawMessage("null"), nil
	}
	return frontend_response.Data, nil
}

func (c *OfficialAccountServer) ListClients() []*WebsocketClient {
	c.ws_mu.RLock()
	clients := make([]*WebsocketClient, 0, len(c.ws_clients))
	for cl := range c.ws_clients {
		clients = append(clients, cl)
	}
	c.ws_mu.RUnlock()
	return clients
}

func (c *OfficialAccountServer) first_client() (*WebsocketClient, error) {
	c.ws_mu.RLock()
	defer c.ws_mu.RUnlock()
	for cl := range c.ws_clients {
		if cl != nil {
			return cl, nil
		}
	}
	return nil, new_scraper_error(ErrorKindClientNotReady, ErrorMessage(ErrorKindClientNotReady), nil)
}
func (c *OfficialAccountServer) RequestFrontendOn(ws *WebsocketClient, endpoint string, body interface{}, timeout time.Duration) (*ClientWebsocketResponse, error) {
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

// Call the frontend to refresh the credential info for a specified official account
func (c *OfficialAccountServer) RefreshAccountWithFrontend(body *OfficialAccountBody) (*OfficialAccount, error) {
	trace_id := c.next_trace_id("refresh_account_frontend")
	logger := c.logger.With().
		Str("trace_id", trace_id).
		Str("biz", body.Biz).
		Logger()
	start := time.Now()
	if body.Biz == "" {
		logger.Error().Msg("refresh official account via frontend: missing biz")
		return nil, new_scraper_error(ErrorKindMissingBiz, ErrorMessage(ErrorKindMissingBiz), nil)
	}
	if err := c.EnsureFrontendReady(5 * time.Second); err != nil {
		logger.Error().Err(err).Msg("refresh official account via frontend: frontend not ready")
		return nil, err
	}
	acct_mu.RLock()
	acct, ok := accounts[body.Biz]
	if !ok {
		acct_mu.RUnlock()
		return nil, new_scraper_error(ErrorKindAccountNotFound, ErrorMessage(ErrorKindAccountNotFound), nil)
	}
	if strings.TrimSpace(acct.RefreshUri) == "" {
		acct_mu.RUnlock()
		return nil, new_scraper_error(ErrorKindMissingRefreshURI, ErrorMessage(ErrorKindMissingRefreshURI), nil)
	}
	if acct.IsEffective && time.Now().Unix()-acct.UpdateTime < 20*60 {
		age := time.Now().Unix() - acct.UpdateTime
		logger.Info().
			Int64("acct_update_time", acct.UpdateTime).
			Int64("acct_update_age_sec", age).
			Msg("refresh official account via frontend: skip (recent update)")
		acct_mu.RUnlock()
		go c.push_credential_to_remote_server(logger, acct)
		logger.Info().Dur("cost", time.Since(start)).Msg("refresh official account via frontend: completed (skipped)")
		return acct, nil
	}
	acct_mu.RUnlock()
	ws, err := c.first_client()
	if err != nil {
		logger.Error().Err(err).Msg("refresh official account via frontend: no available client")
		return nil, err
	}
	if ws != nil {
		logger = logger.With().Str("client_title", ws.title).Logger()
	}
	return c.refresh_credential_from_frontend(logger, body, ws)
}

func (c *OfficialAccountServer) refresh_credential_from_frontend(logger zerolog.Logger, body *OfficialAccountBody, ws *WebsocketClient) (*OfficialAccount, error) {
	start := time.Now()
	logger.Info().Msg("refresh official account via frontend: start")
	if body.Biz == "" {
		logger.Error().Msg("refresh official account via frontend: missing biz")
		return nil, new_scraper_error(ErrorKindMissingBiz, ErrorMessage(ErrorKindMissingBiz), nil)
	}
	if err := c.EnsureFrontendReady(5 * time.Second); err != nil {
		logger.Error().Err(err).Msg("refresh official account via frontend: frontend not ready")
		return nil, err
	}
	if ws == nil {
		return nil, new_scraper_error(ErrorKindClientNotReady, ErrorMessage(ErrorKindClientNotReady), nil)
	}
	acct_mu.RLock()
	acct, ok := accounts[body.Biz]
	if !ok {
		acct_mu.RUnlock()
		return nil, new_scraper_error(ErrorKindAccountNotFound, ErrorMessage(ErrorKindAccountNotFound), nil)
	}
	if strings.TrimSpace(acct.RefreshUri) == "" {
		acct_mu.RUnlock()
		return nil, errors.New("缺少 refresh_uri")
	}
	acct_mu.RUnlock()
	c.wait_mu.Lock()
	if ch, ok := c.wait_chan_map[acct.Biz]; ok {
		c.wait_mu.Unlock()
		logger.Info().Msg("refresh official account via frontend: wait channel exists, waiting")
		select {
		case cur_acct := <-ch:
			logger.Info().Dur("cost", time.Since(start)).Msg("refresh official account via frontend: completed (shared result)")
			return cur_acct, nil
		case <-time.After(20 * time.Second):
			logger.Error().Dur("cost", time.Since(start)).Msg("refresh official account via frontend: timeout (shared wait)")
			return nil, new_scraper_error(ErrorKindTimeout, ErrorMessage(ErrorKindTimeout), nil)
		}
	}
	ch := make(chan *OfficialAccount, 1)
	c.wait_chan_map[acct.Biz] = ch
	c.wait_mu.Unlock()

	logger.Info().Msg("refresh official account via frontend: request frontend fetch_account_home")

	req_body := struct {
		Biz        string `json:"biz"`
		RefreshUri string `json:"refresh_uri"`
	}{Biz: acct.Biz, RefreshUri: acct.RefreshUri}

	if _, err := c.RequestFrontendOn(ws, "key:fetch_account_home", req_body, 15*time.Second); err != nil {
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
		go c.push_credential_to_remote_server(logger, cur_acct)
		logger.Info().Dur("cost", time.Since(start)).Msg("refresh official account via frontend: completed")
		return cur_acct, nil
	case <-time.After(20 * time.Second):
		c.wait_mu.Lock()
		delete(c.wait_chan_map, acct.Biz)
		c.wait_mu.Unlock()
		logger.Error().Dur("cost", time.Since(start)).Msg("refresh official account via frontend: timeout")
		return nil, new_scraper_error(ErrorKindTimeout, ErrorMessage(ErrorKindTimeout), nil)
	}
}
func (c *OfficialAccountServer) RefreshAllRemoteOfficialAccount() error {
	run_id := c.next_trace_id("refresh_all_remote")
	return c.refresh_all_remote_official_account(run_id)
}

func (c *OfficialAccountServer) refresh_all_remote_official_account(run_id string) error {
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
	report, err := c.refresh_remote_official_account(logger, c.RemoteServerAddr)
	if err != nil {
		return err
	}
	c.save_refresh_log(report)
	logger.Info().Msg("refresh all remote official accounts: completed")
	return nil
}
func (c *OfficialAccountServer) RefreshRemoteOfficialAccount(origin string) error {
	run_id := c.next_trace_id("refresh_remote")
	logger := c.logger.With().Str("run_id", run_id).Str("origin", origin).Logger()
	_, err := c.refresh_remote_official_account(logger, origin)
	return err
}

type remote_official_account_job struct {
	Biz      string
	Nickname string
}

func (c *OfficialAccountServer) refresh_remote_official_account(logger zerolog.Logger, origin string) (*RefreshReport, error) {
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
		base_url, err := url.Parse(origin + "/api/mp/list")
		if err != nil {
			logger.Error().Err(err).Msg("refresh remote official accounts: parse request url failed")
			return nil, err
		}
		q := base_url.Query()
		if c.RefreshToken != "" {
			q.Set("token", c.RefreshToken)
		}
		q.Set("page", strconv.Itoa(page))
		q.Set("page_size", strconv.Itoa(page_size))
		base_url.RawQuery = q.Encode()
		req, err := http.NewRequest("GET", base_url.String(), nil)
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
		return nil, new_scraper_error(ErrorKindClientNotReady, ErrorMessage(ErrorKindClientNotReady), nil)
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
	jobs := make(chan remote_official_account_job, len(items))
	for _, item := range items {
		if item.Biz == "" {
			logger.Warn().Str("nickname", item.Nickname).Msg("refresh remote official accounts: skip item (missing biz)")
			continue
		}
		should_skip := false
		var update_time int64
		acct_mu.RLock()
		acct, ok := accounts[item.Biz]
		if !ok || acct == nil {
			should_skip = true
		} else {
			update_time = acct.UpdateTime
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
				Int64("acct_update_time", update_time).
				Int64("acct_update_age_sec", now-update_time).
				Msg("refresh remote official accounts: skip (missing local refresh_uri or within refresh skip threshold)")
			continue
		}
		logger.Info().
			Str("biz", item.Biz).
			Str("nickname", item.Nickname).
			Int64("acct_update_time", update_time).
			Int64("acct_update_age_sec", now-update_time).
			Msg("refresh remote official accounts: enqueue")
		jobs <- remote_official_account_job{Biz: item.Biz, Nickname: item.Nickname}
		total++
	}
	close(jobs)
	if total == 0 {
		logger.Info().Msg("refresh remote official accounts: no jobs to process")
		end_time := time.Now()
		report.EndTime = end_time.Format("2006-01-02 15:04:05")
		report.Duration = end_time.Sub(start_time).String()
		return report, nil
	}
	var wg sync.WaitGroup
	processed := make([]int64, len(clients))
	var success int64
	failures := make([]FailureDetail, 0)
	var failures_mu sync.Mutex

	for i, ws := range clients {
		client_title := ""
		if ws != nil {
			client_title = ws.title
		}
		wg.Add(1)
		go func(idx int, ws *WebsocketClient) {
			defer wg.Done()
			worker_logger := logger.With().
				Int("worker_idx", idx).
				Str("client_title", client_title).
				Logger()
			worker_logger.Info().Msg("refresh worker: started")
			for job := range jobs {
				biz := job.Biz
				job_logger := worker_logger.With().
					Str("biz", biz).
					Str("nickname", job.Nickname).
					Logger()
				job_logger.Info().Msg("refresh job: start")
				_, err := c.refresh_credential_from_frontend(job_logger, &OfficialAccountBody{Biz: biz}, ws)
				if err != nil {
					job_logger.Warn().Err(err).Msg("refresh job: on client failed, fallback to any client")
					fallback_ws, pick_err := c.first_client()
					if pick_err != nil {
						err2 := pick_err
						failures_mu.Lock()
						failures = append(failures, FailureDetail{Biz: biz, Nickname: job.Nickname, Error: err2.Error()})
						failures_mu.Unlock()
						job_logger.Error().Err(err2).Msg("refresh job: failed")
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
						continue
					}
					fallback_logger := job_logger
					if fallback_ws != nil {
						fallback_logger = fallback_logger.With().Str("client_title", fallback_ws.title).Logger()
					}
					_, err2 := c.refresh_credential_from_frontend(fallback_logger, &OfficialAccountBody{Biz: biz}, fallback_ws)
					if err2 != nil {
						failures_mu.Lock()
						failures = append(failures, FailureDetail{Biz: biz, Nickname: job.Nickname, Error: err2.Error()})
						failures_mu.Unlock()
						job_logger.Error().Err(err2).Msg("refresh job: failed")
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
						job_logger.Info().Msg("refresh job: success (fallback)")
						acct_mu.Lock()
						if acct, ok := accounts[biz]; ok && acct != nil {
							acct.Error = ""
							accounts[biz] = acct
						}
						acct_mu.Unlock()
					}
				} else {
					atomic.AddInt64(&success, 1)
					job_logger.Info().Msg("refresh job: success")
					acct_mu.Lock()
					if acct, ok := accounts[biz]; ok && acct != nil {
						acct.Error = ""
						accounts[biz] = acct
					}
					acct_mu.Unlock()
				}
				atomic.AddInt64(&processed[idx], 1)
			}
			worker_logger.Info().Int64("processed", processed[idx]).Msg("refresh worker: completed")
		}(i, ws)
	}
	wg.Wait()
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
	end_time := time.Now()
	report.EndTime = end_time.Format("2006-01-02 15:04:05")
	report.Duration = end_time.Sub(start_time).String()
	report.Total = total
	report.Success = int(success)
	report.Failed = len(failures)
	report.Failures = failures
	return report, nil
}

func (c *OfficialAccountServer) PushCredentialToRemoteServer(credential *OfficialAccount) error {
	logger := c.logger.With().
		Str("biz", func() string {
			if credential == nil {
				return ""
			}
			return credential.Biz
		}()).
		Logger()
	return c.push_credential_to_remote_server(logger, credential)
}

func (c *OfficialAccountServer) push_credential_to_remote_server(logger zerolog.Logger, credential *OfficialAccount) error {
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
		u = c.client.BuildURL(u, map[string]string{"token": c.RefreshToken})
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

// Fetch the push message list for a specific official account
func (c *OfficialAccountServer) FetchMsgList(biz string, offset int) (*OfficialMsgListResp, error) {
	if biz == "" {
		return nil, new_scraper_error(ErrorKindMissingBiz, ErrorMessage(ErrorKindMissingBiz), nil)
	}
	acct := get_official_account(biz)
	if acct == nil {
		return nil, new_scraper_error(ErrorKindAccountNotFound, ErrorMessage(ErrorKindAccountNotFound), nil)
	}
	logger := c.logger.With().
		Str("biz", biz).
		Int("offset", offset).
		Logger()
	return c.fetch_msg_list(logger, acct, offset)
}

type scraper_error struct {
	kind     ErrorKind
	msg      string
	err      error
	location string
}

func (e *scraper_error) Error() string {
	return e.msg
}

func (e *scraper_error) Unwrap() error {
	return e.err
}

func new_scraper_error(kind ErrorKind, msg string, err error) *scraper_error {
	pc, file, line, _ := runtime.Caller(1)
	func_name := runtime.FuncForPC(pc).Name()
	return &scraper_error{
		kind:     kind,
		msg:      msg,
		err:      err,
		location: fmt.Sprintf("%s:%d:%s", filepath.Base(file), line, filepath.Base(func_name)),
	}
}

func scraper_error_of(err error) (ErrorKind, string, string, bool) {
	var scraper_err *scraper_error
	if errors.As(err, &scraper_err) {
		return scraper_err.kind, scraper_err.msg, scraper_err.location, true
	}
	return "", "", "", false
}

func safe_log_err(err error) string {
	if err == nil {
		return ""
	}
	in := err.Error()
	re := regexp.MustCompile(`https?://[^\s]+`)
	return re.ReplaceAllStringFunc(in, func(m string) string {
		u, parse_err := url.Parse(m)
		if parse_err != nil {
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

func safe_net_reason(err error) string {
	if err == nil {
		return ""
	}
	var net_err net.Error
	if errors.As(err, &net_err) && net_err.Timeout() {
		return ErrorMessage(ErrorKindTimeout)
	}
	var op_err *net.OpError
	if errors.As(err, &op_err) {
		if strings.Contains(strings.ToLower(op_err.Err.Error()), "refused") {
			return "连接被拒绝"
		}
	}
	if strings.Contains(strings.ToLower(err.Error()), "no such host") {
		return "DNS 解析失败"
	}
	return "网络请求失败"
}

func (c *OfficialAccountServer) Stop() {
	c.ws_mu.Lock()
	for client := range c.ws_clients {
		close(client.send)
		delete(c.ws_clients, client)
	}
	c.ws_mu.Unlock()
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

func (c *OfficialAccountServer) save_refresh_log(report *RefreshReport) {
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
