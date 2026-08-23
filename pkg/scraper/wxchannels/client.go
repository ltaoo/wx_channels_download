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

	"github.com/gorilla/websocket"

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

// ClientOptions contains application-provided values needed by Client.
// Keeping this contract explicit prevents the reusable scraper from depending
// on the application's internal configuration package.
type ClientOptions struct {
	RefreshInterval int
	CookieReader    *cookies.Reader
	SphCookie       string
}

type Client struct {
	ws_clients       map[*WebsocketClient]bool
	ws_mu            sync.RWMutex
	requests         map[string]chan ClientWebsocketResponse
	requests_mu      sync.RWMutex
	cache            *cache.Cache
	req_seq          uint64
	refresh_interval int
	cookie_reader    *cookies.Reader
	sph_cookie       string
	OnConnected      func(client *WebsocketClient)
	OnDisconnected   func(client *WebsocketClient)
	OnMessage        func(client *WebsocketClient, message []byte)
}

func NewClient(options ClientOptions) *Client {
	return &Client{
		ws_clients:       make(map[*WebsocketClient]bool),
		requests:         make(map[string]chan ClientWebsocketResponse),
		cache:            cache.New(),
		req_seq:          uint64(time.Now().UnixNano()),
		refresh_interval: options.RefreshInterval,
		cookie_reader:    options.CookieReader,
		sph_cookie:       strings.TrimSpace(options.SphCookie),
	}
}

func (c *Client) Fetch(params FetchParams) (any, error) {
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

func (c *Client) fetch_profile_with_share_url(raw_url string) (any, error) {
	cookie, err := c.resolve_sph_cookie()
	if err != nil {
		return c.FetchChannelsFeedProfile("", "", raw_url, "")
	}
	return FetchVideoProfileWithShareUrl(raw_url, cookie)
}

func (c *Client) resolve_sph_cookie() (string, error) {
	fallback_cookie := strings.TrimSpace(c.sph_cookie)
	if c.cookie_reader == nil {
		if fallback_cookie != "" {
			return fallback_cookie, nil
		}
		return "", errors.New("persistent cookie reader is unavailable")
	}
	cookie, err := c.cookie_reader.HeaderForDomain(yuanbao_cookie_domain)
	if err == nil && strings.TrimSpace(cookie) != "" {
		return strings.TrimSpace(cookie), nil
	}
	if err == nil {
		err = cookies.ErrCookieNotFound
	}
	if fallback_cookie != "" {
		return fallback_cookie, nil
	}
	if errors.Is(err, cookies.ErrCookieNotFound) {
		return "", errors.New("no yuanbao.tencent.com cookie was found")
	}
	return "", fmt.Errorf("read yuanbao.tencent.com cookie: %w", err)
}

// CheckSphCookie verifies that a yuanbao.tencent.com cookie is available from
// persistent storage or the configured fallback for SPH share-link parsing.
func (c *Client) CheckSphCookie() error {
	_, err := c.resolve_sph_cookie()
	return err
}

func (c *Client) fetch_profile_with_channels_client(raw_url string) (any, error) {
	return c.FetchFeedPage(raw_url)
}

func (c *Client) ServeWebsocket(writer http.ResponseWriter, request *http.Request) {
	conn, err := channels_ws_upgrader.Upgrade(writer, request, nil)
	if err != nil {
		return
	}
	c.ws_mu.Lock()
	client := &WebsocketClient{Conn: conn, Send: make(chan []byte, 256)}
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
func (c *Client) Stop() {
	disconnected_clients := make([]*WebsocketClient, 0)
	c.ws_mu.Lock()
	for client := range c.ws_clients {
		close(client.Send)
		delete(c.ws_clients, client)
		disconnected_clients = append(disconnected_clients, client)
	}
	c.ws_mu.Unlock()
	c.notify_disconnected(disconnected_clients)
}
func (c *Client) notify_disconnected(clients []*WebsocketClient) {
	if c.OnDisconnected != nil {
		for _, client := range clients {
			c.OnDisconnected(client)
		}
	}
}
func (c *Client) Broadcast(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	disconnected_clients := make([]*WebsocketClient, 0)
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
func (wc *Client) Validate() error {
	if !wc.Available() {
		return errors.New("please initialize the client socket connection first")
	}
	return nil
}

func (wc *Client) Available() bool {
	wc.ws_mu.RLock()
	defer wc.ws_mu.RUnlock()
	return len(wc.ws_clients) > 0
}
func (c *Client) RequestFrontend(endpoint string, body interface{}, timeout time.Duration) (json.RawMessage, error) {
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
	var client *WebsocketClient
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
		return resp.Data, nil
	case <-time.After(timeout):
		return nil, errors.New("request timed out")
	}
}

// Search for users by keyword
func (c *Client) SearchChannelsContact(keyword string, next_marker string) (json.RawMessage, error) {
	if keyword == "" {
		return nil, errors.New("keyword cannot be empty")
	}
	clean_keyword := strings.TrimSpace(keyword)
	cache_key := "channels:contact_list:" + clean_keyword + ":" + next_marker
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(json.RawMessage); ok {
			return resp, nil
		}
	}
	fmt.Println("next_marker", next_marker)
	resp, err := c.RequestFrontend("key:channels:contact_list", ChannelsAccountSearchBody{Keyword: keyword, NextMarker: next_marker}, 20*time.Second)
	if err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, resp, 5*time.Minute)
	return resp, nil
}

// Fetch video feed list for a specific user
func (c *Client) FetchChannelsFeedListOfContact(username, next_marker string) (json.RawMessage, error) {
	clean_name := strings.TrimSpace(username)
	if !strings.HasSuffix(clean_name, "@finder") {
		clean_name += "@finder"
	}
	cache_key := "channels:feed_list:" + clean_name + ":" + next_marker
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(json.RawMessage); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:feed_list", ChannelsFeedListBody{Username: clean_name, NextMarker: next_marker}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, resp, 5*time.Minute)
	return resp, nil
}

// Fetch live replay list for a specific user
func (c *Client) FetchChannelsLiveReplayList(username, next_marker string) (json.RawMessage, error) {
	clean_name := strings.TrimSpace(username)
	if !strings.HasSuffix(clean_name, "@finder") {
		clean_name += "@finder"
	}
	cache_key := "channels:live_replay_list:" + clean_name + ":" + next_marker
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(json.RawMessage); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:live_replay_list", ChannelsLiveReplayListBody{Username: clean_name, NextMarker: next_marker}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, resp, 5*time.Minute)
	return resp, nil
}

// FetchLiveInfo fetches a live profile through the Channels frontend joinLive API.
func (c *Client) FetchLiveInfo(username, oid, nid, live_id string) (json.RawMessage, error) {
	username = strings.TrimSpace(username)
	oid = strings.TrimSpace(oid)
	nid = strings.TrimSpace(nid)
	live_id = strings.TrimSpace(live_id)
	if username == "" {
		return nil, errors.New("username cannot be empty")
	}
	if oid == "" {
		return nil, errors.New("oid cannot be empty")
	}
	if nid == "" {
		return nil, errors.New("nid cannot be empty")
	}
	if live_id == "" {
		return nil, errors.New("id cannot be empty")
	}
	return c.RequestFrontend("key:channels:live_info", ChannelsLiveInfoBody{
		Username:      username,
		ObjectId:      oid,
		ObjectNonceId: nid,
		LiveId:        live_id,
	}, 10*time.Second)
}

// Fetch favorited or liked feed list for the user
func (c *Client) FetchChannelsInteractionedFeedList(flag, next_marker string) (json.RawMessage, error) {
	cache_key := "channels:interactioned_list:" + flag + ":" + next_marker
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(json.RawMessage); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:interactioned_list", ChannelsInteractionedFeedListBody{Flag: flag, NextMarker: next_marker}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, resp, 5*time.Minute)
	return resp, nil
}

func (c *Client) FetchChannelsFollowList(next_marker string) (json.RawMessage, error) {
	cache_key := "channels:follow_list:" + next_marker
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(json.RawMessage); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:follow_list", ChannelsFollowListBody{NextMarker: next_marker}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, resp, 5*time.Minute)
	return resp, nil
}

func (c *Client) FetchChannelsPlayHistory(next_marker string) (json.RawMessage, error) {
	cache_key := "channels:play_history:" + next_marker
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(json.RawMessage); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:play_history", ChannelsPlayHistoryBody{NextMarker: next_marker}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, resp, 5*time.Minute)
	return resp, nil
}

func (c *Client) FetchChannelsFeedProfile(oid, uid, url, eid string) (json.RawMessage, error) {
	// fmt.Println("[API]fetch feed profile", oid, uid)
	kk := fmt.Sprintf("%s:%s:%s:%s", oid, uid, url, eid)
	cache_key := "channels:feed_profile:" + kk
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(json.RawMessage); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:feed_profile", ChannelsFeedProfileBody{ObjectId: oid, NonceId: uid, URL: url, EncryptedObjectId: eid}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, resp, 60*time.Minute)
	return resp, nil
}

func (c *Client) FetchFeedPage(raw_url string) (json.RawMessage, error) {
	parts, err := ParseFeedURL(raw_url)
	if err != nil {
		return nil, err
	}
	return c.FetchChannelsFeedProfile(parts.Oid, parts.Nid, raw_url, parts.Eid)
}

func (c *Client) FetchChannelsSharedFeedProfile(url string) (json.RawMessage, error) {
	// fmt.Println("[API]fetch feed profile", oid, uid)
	kk := fmt.Sprintf("%s", url)
	cache_key := "channels:shared_feed_profile:" + kk
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(json.RawMessage); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:shared_feed_profile", ChannelsSharedFeedProfileBody{URL: url}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, resp, 60*time.Minute)
	return resp, nil

}

func (c *Client) FetchChannelsFeedCommentList(oid, nid, comment_id, next_marker string) (json.RawMessage, error) {
	if oid == "" {
		return nil, errors.New("missing oid")
	}
	if nid == "" && comment_id == "" {
		return nil, errors.New("missing nid or comment_id")
	}
	kk := fmt.Sprintf("%s:%s:%s:%s", oid, nid, comment_id, next_marker)
	cache_key := "channels:feed_comment_list:" + kk
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(json.RawMessage); ok {
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
	c.cache.Set(cache_key, resp, 60*time.Minute)
	return resp, nil
}

func (c *Client) FetchChannelsFeedShareUrl(oid string) (json.RawMessage, error) {
	if oid == "" {
		return nil, errors.New("missing oid")
	}
	kk := fmt.Sprintf("%s", oid)
	cache_key := "channels:feed_share_url:" + kk
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(json.RawMessage); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:feed_share_url", ChannelsFeedShareUrlBody{
		ObjectId: oid,
	}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, resp, 60*time.Minute)
	return resp, nil
}

// Reload the channels page
func (c *Client) ReloadChannels() (json.RawMessage, error) {
	return c.RequestFrontend("key:channels:reload", nil, 5*time.Second)
}
