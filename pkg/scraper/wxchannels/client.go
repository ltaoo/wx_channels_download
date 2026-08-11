package wxchannels

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"wx_channel/internal/config"
	"wx_channel/pkg/cache"
	"wx_channel/pkg/cookies"
)

var channels_ws_upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var (
	channels_share_url_reg = regexp.MustCompile(`^https://weixin\.qq\.com/sph/[A-Za-z0-9_-]+/?$`)
)

const yuanbao_cookie_domain = ".tencent.com"

type FetchParams struct {
	URL string `json:"url"`
}

type ChannelsClient struct {
	ws_clients       map[*Client]bool
	ws_mu            sync.RWMutex
	engine           *gin.Engine
	requests         map[string]chan ClientWebsocketResponse
	requests_mu      sync.RWMutex
	cache            *cache.Cache
	req_seq          uint64
	refresh_interval int
	cfg              *config.Config
	cookie_reader    *cookies.Reader
	OnConnected      func(client *Client)
	OnDisconnected   func(client *Client)
	OnMessage        func(client *Client, message []byte)
}

func NewChannelsClient(refresh_interval int, cfg *config.Config) *ChannelsClient {
	work_dir := ""
	if cfg != nil {
		work_dir = cfg.WorkDir
	}
	return &ChannelsClient{
		ws_clients:       make(map[*Client]bool),
		requests:         make(map[string]chan ClientWebsocketResponse),
		cache:            cache.New(),
		req_seq:          uint64(time.Now().UnixNano()),
		refresh_interval: refresh_interval,
		cfg:              cfg,
		cookie_reader:    cookies.NewPersistentReader(work_dir),
	}
}

func (c *ChannelsClient) Fetch(params FetchParams) (any, error) {
	raw_url := strings.TrimSpace(params.URL)
	switch {
	case channels_share_url_reg.MatchString(raw_url):
		return c.fetch_profile_with_share_url(raw_url)
	case is_channels_feed_url(raw_url):
		return c.fetch_profile_with_channels_client(raw_url)
	default:
		return nil, ErrUnsupportedURL
	}
}

func is_channels_feed_url(raw_url string) bool {
	parts, err := ParseFeedURL(raw_url)
	return err == nil && parts.Oid != "" && parts.Nid != ""
}

func (c *ChannelsClient) fetch_profile_with_share_url(raw_url string) (any, error) {
	cookie, err := c.resolve_sph_cookie()
	if err != nil {
		return nil, err
	}
	return FetchVideoProfileWithShareUrl(raw_url, cookie)
}

func (c *ChannelsClient) resolve_sph_cookie() (string, error) {
	if c.cfg == nil {
		return "", errors.New("config is not initialized")
	}
	cookie := strings.TrimSpace(c.cfg.GetString("cloudflare.sphCookie"))
	if cookie != "" {
		return cookie, nil
	}
	if c.cookie_reader == nil {
		return "", errors.New("cloudflare.sphCookie not configured and persistent cookie reader is unavailable")
	}
	cookie, err := c.cookie_reader.HeaderForDomain(yuanbao_cookie_domain)
	if err == nil {
		return cookie, nil
	}
	if errors.Is(err, cookies.ErrCookieNotFound) {
		return "", errors.New("cloudflare.sphCookie not configured and no yuanbao.tencent.com cookie was found")
	}
	return "", fmt.Errorf("read yuanbao.tencent.com cookie: %w", err)
}

func (c *ChannelsClient) fetch_profile_with_channels_client(raw_url string) (any, error) {
	return c.FetchFeedPage(raw_url)
}

func (c *ChannelsClient) HandleChannelsWebsocket(ctx *gin.Context) {
	conn, err := channels_ws_upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	c.ws_mu.Lock()
	client := &Client{Conn: conn, Send: make(chan []byte, 256)}
	c.ws_clients[client] = true
	c.ws_mu.Unlock()

	go client.write_pump()

	if c.OnConnected != nil {
		c.OnConnected(client)
	}

	// Periodic refresh logic
	refresh_interval := c.refresh_interval
	if c.refresh_interval > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(refresh_interval) * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					c.ws_mu.RLock()
					if _, ok := c.ws_clients[client]; !ok {
						c.ws_mu.RUnlock()
						return
					}
					c.ws_mu.RUnlock()
					c.ReloadChannels()
				}
			}
		}()
	}

	defer func() {
		removed := false
		c.ws_mu.Lock()
		if _, ok := c.ws_clients[client]; ok {
			delete(c.ws_clients, client)
			close(client.Send)
			removed = true
		}
		c.ws_mu.Unlock()
		if removed && c.OnDisconnected != nil {
			c.OnDisconnected(client)
		}
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
				continue
			}
		}
	}
}
func (c *ChannelsClient) Stop() {
	disconnected_clients := make([]*Client, 0)
	c.ws_mu.Lock()
	for client := range c.ws_clients {
		close(client.Send)
		delete(c.ws_clients, client)
		disconnected_clients = append(disconnected_clients, client)
	}
	c.ws_mu.Unlock()
	c.notify_disconnected(disconnected_clients)
}
func (c *ChannelsClient) notify_disconnected(clients []*Client) {
	if c.OnDisconnected != nil {
		for _, client := range clients {
			c.OnDisconnected(client)
		}
	}
}
func (c *ChannelsClient) Broadcast(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	disconnected_clients := make([]*Client, 0)
	c.ws_mu.Lock()
	for client := range c.ws_clients {
		select {
		case client.Send <- data:
		default:
			close(client.Send)
			delete(c.ws_clients, client)
			disconnected_clients = append(disconnected_clients, client)
		}
	}
	c.ws_mu.Unlock()
	c.notify_disconnected(disconnected_clients)
}
func (wc *ChannelsClient) Validate() error {
	if !wc.Available() {
		return errors.New("please initialize the client socket connection first")
	}
	return nil
}

func (wc *ChannelsClient) Available() bool {
	wc.ws_mu.RLock()
	defer wc.ws_mu.RUnlock()
	return len(wc.ws_clients) > 0
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
		return nil, errors.New("no client is available")
	}
	data, err := json.Marshal(msg)
	if err != nil {
		c.ws_mu.Unlock()
		return nil, err
	}

	select {
	case client.Send <- data:
	default:
		c.ws_mu.Unlock()
		return nil, errors.New("send buffer is full")
	}
	c.ws_mu.Unlock()
	select {
	case resp := <-resp_chan:
		return &resp, nil
	case <-time.After(timeout):
		return nil, errors.New("request timed out")
	}
}

// Search for users by keyword
func (c *ChannelsClient) SearchChannelsContact(keyword string, next_marker string) (*ChannelsContactSearchResp, error) {
	if keyword == "" {
		return nil, errors.New("keyword cannot be empty")
	}
	clean_keyword := strings.TrimSpace(keyword)
	cache_key := "channels:contact_list:" + clean_keyword + ":" + next_marker
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(*ChannelsContactSearchResp); ok {
			return resp, nil
		}
	}
	fmt.Println("next_marker", next_marker)
	resp, err := c.RequestFrontend("key:channels:contact_list", ChannelsAccountSearchBody{Keyword: keyword, NextMarker: next_marker}, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var r ChannelsContactSearchResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, &r, 5*time.Minute)
	return &r, nil
}

// Fetch video feed list for a specific user
func (c *ChannelsClient) FetchChannelsFeedListOfContact(username, next_marker string) (*ChannelsFeedListOfAccountResp, error) {
	clean_name := strings.TrimSpace(username)
	if !strings.HasSuffix(clean_name, "@finder") {
		clean_name += "@finder"
	}
	cache_key := "channels:feed_list:" + clean_name + ":" + next_marker
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(*ChannelsFeedListOfAccountResp); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:feed_list", ChannelsFeedListBody{Username: clean_name, NextMarker: next_marker}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var r ChannelsFeedListOfAccountResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, &r, 5*time.Minute)
	return &r, nil
}

// Fetch live replay list for a specific user
func (c *ChannelsClient) FetchChannelsLiveReplayList(username, next_marker string) (*ChannelsFeedListOfAccountResp, error) {
	clean_name := strings.TrimSpace(username)
	if !strings.HasSuffix(clean_name, "@finder") {
		clean_name += "@finder"
	}
	cache_key := "channels:live_replay_list:" + clean_name + ":" + next_marker
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(*ChannelsFeedListOfAccountResp); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:live_replay_list", ChannelsLiveReplayListBody{Username: clean_name, NextMarker: next_marker}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var r ChannelsFeedListOfAccountResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, &r, 5*time.Minute)
	return &r, nil
}

// Fetch favorited or liked feed list for the user
func (c *ChannelsClient) FetchChannelsInteractionedFeedList(flag, next_marker string) (*ChannelsFeedListOfAccountResp, error) {
	cache_key := "channels:interactioned_list:" + flag + ":" + next_marker
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(*ChannelsFeedListOfAccountResp); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:interactioned_list", ChannelsInteractionedFeedListBody{Flag: flag, NextMarker: next_marker}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var r ChannelsFeedListOfAccountResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, &r, 5*time.Minute)
	return &r, nil
}

func (c *ChannelsClient) FetchChannelsFollowList(next_marker string) (*ChannelsFollowListResp, error) {
	cache_key := "channels:follow_list:" + next_marker
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(*ChannelsFollowListResp); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:follow_list", ChannelsFollowListBody{NextMarker: next_marker}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var r ChannelsFollowListResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, &r, 5*time.Minute)
	return &r, nil
}

func (c *ChannelsClient) FetchChannelsPlayHistory(next_marker string) (*ChannelsPlayHistoryResp, error) {
	cache_key := "channels:play_history:" + next_marker
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(*ChannelsPlayHistoryResp); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:play_history", ChannelsPlayHistoryBody{NextMarker: next_marker}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var r ChannelsPlayHistoryResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, &r, 5*time.Minute)
	return &r, nil
}

func (c *ChannelsClient) FetchChannelsFeedProfile(oid, uid, url, eid string) (*ChannelsFeedProfileResp, error) {
	// fmt.Println("[API]fetch feed profile", oid, uid)
	kk := fmt.Sprintf("%s:%s:%s:%s", oid, uid, url, eid)
	cache_key := "channels:feed_profile:" + kk
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(*ChannelsFeedProfileResp); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:feed_profile", ChannelsFeedProfileBody{ObjectId: oid, NonceId: uid, URL: url, EncryptedObjectId: eid}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var r ChannelsFeedProfileResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, &r, 60*time.Minute)
	return &r, nil
}

func (c *ChannelsClient) FetchFeedPage(raw_url string) (*ChannelsObject, error) {
	parts, err := ParseFeedURL(raw_url)
	if err != nil {
		return nil, err
	}
	resp, err := c.FetchChannelsFeedProfile(parts.Oid, parts.Nid, raw_url, parts.Eid)
	if err != nil {
		return nil, err
	}
	if resp.ErrCode != 0 {
		return nil, fmt.Errorf("fetch channels feed profile: %s", resp.ErrMsg)
	}
	return &resp.Data.Object, nil
}

func (c *ChannelsClient) FetchChannelsSharedFeedProfile(url string) (*ChannelsFeedProfileResp, error) {
	// fmt.Println("[API]fetch feed profile", oid, uid)
	kk := fmt.Sprintf("%s", url)
	cache_key := "channels:shared_feed_profile:" + kk
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(*ChannelsFeedProfileResp); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:shared_feed_profile", ChannelsSharedFeedProfileBody{URL: url}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var r ChannelsFeedProfileResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, &r, 60*time.Minute)
	return &r, nil

}

func (c *ChannelsClient) FetchChannelsFeedCommentList(oid, nid, comment_id, next_marker string) (*ChannelsFeedCommentListResp, error) {
	if oid == "" {
		return nil, errors.New("missing oid")
	}
	if nid == "" && comment_id == "" {
		return nil, errors.New("missing nid or comment_id")
	}
	kk := fmt.Sprintf("%s:%s:%s:%s", oid, nid, comment_id, next_marker)
	cache_key := "channels:feed_comment_list:" + kk
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(*ChannelsFeedCommentListResp); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:fetch_feed_comment_list", ChannelsFeedCommentListBody{
		ObjectId:      oid,
		ObjectNonceId: nid,
		CommentId:     comment_id,
		NextMarker:    next_marker,
	}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var r ChannelsFeedCommentListResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, &r, 60*time.Minute)
	return &r, nil
}

func (c *ChannelsClient) FetchChannelsFeedShareUrl(oid string) (*ChannelsFeedShareUrlResp, error) {
	if oid == "" {
		return nil, errors.New("missing oid")
	}
	kk := fmt.Sprintf("%s", oid)
	cache_key := "channels:feed_share_url:" + kk
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(*ChannelsFeedShareUrlResp); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:feed_share_url", ChannelsFeedShareUrlBody{
		ObjectId: oid,
	}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var r ChannelsFeedShareUrlResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, &r, 60*time.Minute)
	return &r, nil
}

// Reload the channels page
func (c *ChannelsClient) ReloadChannels() error {
	_, err := c.RequestFrontend("key:channels:reload", nil, 5*time.Second)
	return err
}
