package wxchannels

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"wx_channel/pkg/cache"
	"wx_channel/pkg/configapi"
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
	channels_feed_url_reg  = regexp.MustCompile(`^https://channels\.weixin\.qq\.com/web/pages/feed\?oid=[A-Za-z0-9_-]+&nid=[A-Za-z0-9_-]+$`)
)

type FetchParams struct {
	URL string `json:"url"`
}

type ChannelsClientConfig struct {
	RefreshInterval int    `json:"refreshInterval"`
	YuanbaoCookie   string `json:"sphCookie"`
	revision        uint64
}

// ClientConfigDeclaration explicitly lists the runtime configuration consumed
// by ChannelsClient.
var ClientConfigDeclaration = configapi.Declare("channels", "cloudflare")

type ChannelsClient struct {
	ws_clients       map[*Client]bool
	ws_mu            sync.RWMutex
	engine           *gin.Engine
	requests         map[string]chan ClientWebsocketResponse
	requests_mu      sync.RWMutex
	cache            *cache.Cache
	req_seq          uint64
	config_provider  configapi.Provider
	runtime_config   atomic.Pointer[ChannelsClientConfig]
	config_apply_mu  sync.Mutex
	config_change_mu sync.Mutex
	config_changed   chan struct{}
	unsubscribes     []func()
	stop_once        sync.Once
	OnConnected      func(client *Client)
	OnDisconnected   func(client *Client)
	OnMessage        func(client *Client, message []byte)
}

func NewChannelsClient(config_provider configapi.Provider) *ChannelsClient {
	c := &ChannelsClient{
		ws_clients:      make(map[*Client]bool),
		requests:        make(map[string]chan ClientWebsocketResponse),
		cache:           cache.New(),
		req_seq:         uint64(time.Now().UnixNano()),
		config_provider: config_provider,
		config_changed:  make(chan struct{}),
	}
	c.runtime_config.Store(&ChannelsClientConfig{})
	if config_provider != nil {
		unsubscribe, err := ClientConfigDeclaration.Subscribe(config_provider, func(uint64) {
			if err := c.reload_runtime_config(); err != nil {
				log.Printf("wxchannels: reload runtime config: %v", err)
			}
		})
		if err != nil {
			log.Printf("wxchannels: subscribe runtime config: %v", err)
		} else {
			c.unsubscribes = append(c.unsubscribes, unsubscribe)
		}
		if err := c.reload_runtime_config(); err != nil {
			log.Printf("wxchannels: load runtime config: %v", err)
		}
	}
	return c
}

func (c *ChannelsClient) Fetch(params FetchParams) (any, error) {
	raw_url := strings.TrimSpace(params.URL)
	switch {
	case channels_share_url_reg.MatchString(raw_url):
		return c.fetch_profile_with_share_url(raw_url)
	case channels_feed_url_reg.MatchString(raw_url):
		return c.fetch_profile_with_channels_client(raw_url)
	default:
		return nil, ErrUnsupportedURL
	}
}

func (c *ChannelsClient) fetch_profile_with_share_url(raw_url string) (any, error) {
	cfg := c.current_config()
	cookie := strings.TrimSpace(cfg.YuanbaoCookie)
	if cookie == "" {
		return nil, errors.New("cloudflare.sphCookie not configured")
	}
	return FetchVideoProfileWithShareUrl(raw_url, cookie)
}

func (c *ChannelsClient) fetch_profile_with_channels_client(raw_url string) (any, error) {
	return nil, nil
}

func (c *ChannelsClient) reload_runtime_config() error {
	if c.config_provider == nil {
		return errors.New("config provider is not initialized")
	}

	var channels_config struct {
		RefreshInterval int `json:"refreshInterval"`
	}
	channels_snapshot, err := ClientConfigDeclaration.Snapshot(c.config_provider, "channels")
	if err != nil {
		return fmt.Errorf("load channels config: %w", err)
	}
	if err := channels_snapshot.Decode(&channels_config); err != nil {
		return fmt.Errorf("decode channels config: %w", err)
	}
	if channels_config.RefreshInterval < 0 {
		return errors.New("channels.refreshInterval cannot be negative")
	}

	var cloudflare_config struct {
		YuanbaoCookie string `json:"sphCookie"`
	}
	cloudflare_snapshot, err := ClientConfigDeclaration.Snapshot(c.config_provider, "cloudflare")
	if err != nil {
		return fmt.Errorf("load cloudflare config: %w", err)
	}
	if err := cloudflare_snapshot.Decode(&cloudflare_config); err != nil {
		return fmt.Errorf("decode cloudflare config: %w", err)
	}

	revision := channels_snapshot.Revision()
	if cloudflare_snapshot.Revision() > revision {
		revision = cloudflare_snapshot.Revision()
	}
	return c.apply_runtime_config(ChannelsClientConfig{
		RefreshInterval: channels_config.RefreshInterval,
		YuanbaoCookie:   cloudflare_config.YuanbaoCookie,
		revision:        revision,
	})
}

func (c *ChannelsClient) apply_runtime_config(next ChannelsClientConfig) error {
	if next.RefreshInterval < 0 {
		return errors.New("channels.refreshInterval cannot be negative")
	}
	c.config_apply_mu.Lock()
	defer c.config_apply_mu.Unlock()
	current := c.current_config()
	if next.revision != 0 && next.revision < current.revision {
		return nil
	}

	next_copy := next
	c.config_change_mu.Lock()
	c.runtime_config.Store(&next_copy)
	close(c.config_changed)
	c.config_changed = make(chan struct{})
	c.config_change_mu.Unlock()
	return nil
}

func (c *ChannelsClient) current_config() ChannelsClientConfig {
	cfg := c.runtime_config.Load()
	if cfg == nil {
		return ChannelsClientConfig{}
	}
	return *cfg
}

func (c *ChannelsClient) runtime_config_state() (ChannelsClientConfig, <-chan struct{}) {
	c.config_change_mu.Lock()
	defer c.config_change_mu.Unlock()
	return c.current_config(), c.config_changed
}

func (c *ChannelsClient) refresh_channels(client *Client, done <-chan struct{}) {
	for {
		cfg, config_changed := c.runtime_config_state()
		if cfg.RefreshInterval <= 0 {
			select {
			case <-config_changed:
				continue
			case <-done:
				return
			}
		}

		timer := time.NewTimer(time.Duration(cfg.RefreshInterval) * time.Second)
		select {
		case <-timer.C:
			c.ws_mu.RLock()
			_, available := c.ws_clients[client]
			c.ws_mu.RUnlock()
			if !available {
				return
			}
			_ = c.ReloadChannels()
		case <-config_changed:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			continue
		case <-done:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}
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

	client_done := make(chan struct{})
	go c.refresh_channels(client, client_done)

	defer func() {
		close(client_done)
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
	c.stop_once.Do(func() {
		for _, unsubscribe := range c.unsubscribes {
			unsubscribe()
		}
		c.unsubscribes = nil
	})
	c.ws_mu.Lock()
	for client := range c.ws_clients {
		close(client.Send)
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
		case client.Send <- data:
		default:
			close(client.Send)
			delete(c.ws_clients, client)
		}
	}
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

func (c *ChannelsClient) FetchFeedPage(raw_url string) (*FeedPage, error) {
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
	obj := resp.Data.Object
	return &FeedPage{
		URL:    *parts,
		Resp:   resp,
		Object: obj,
	}, nil
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
