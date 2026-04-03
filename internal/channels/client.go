package channels

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"wx_channel/internal/api/types"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/cache"
	"wx_channel/pkg/util"
)

var channels_ws_upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ChannelsClient struct {
	// decryptor   *ChannelsVideoDecryptor
	// downloader  *downloadpkg.Downloader
	// Interceptor *interceptor.Interceptor
	// official    *officialaccount.OfficialAccountBrowser
	// formatter   *util.FilenameProcessor
	// Cookies     []*http.Cookie
	// cfg         *APIConfig
	ws_clients      map[*Client]bool
	ws_mu           sync.RWMutex
	engine          *gin.Engine
	requests        map[string]chan ClientWebsocketResponse
	requests_mu     sync.RWMutex
	cache           *cache.Cache
	req_seq         uint64
	refreshInterval int
	db              *gorm.DB
	OnConnected     func(client *Client)
	OnMessage       func(client *Client, message []byte)
}

func NewChannelsClient(refreshInterval int) *ChannelsClient {
	return &ChannelsClient{
		ws_clients:      make(map[*Client]bool),
		requests:        make(map[string]chan ClientWebsocketResponse),
		cache:           cache.New(),
		req_seq:         uint64(time.Now().UnixNano()),
		refreshInterval: refreshInterval,
	}
}

func (c *ChannelsClient) SetDB(db *gorm.DB) {
	c.db = db
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

	go client.writePump()

	if c.OnConnected != nil {
		c.OnConnected(client)
	}

	// 定时刷新逻辑
	refreshInterval := c.refreshInterval
	if c.refreshInterval > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(refreshInterval) * time.Second)
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
		c.ws_mu.Lock()
		if _, ok := c.ws_clients[client]; ok {
			delete(c.ws_clients, client)
			close(client.Send)
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
				continue
			}
		}
	}
}
func (c *ChannelsClient) Stop() {
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
	// wc.clientsMu.Lock()
	// defer wc.clientsMu.Unlock()
	if len(wc.ws_clients) == 0 {
		return errors.New("请先初始化客户端 socket 连接")
	}
	return nil
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
		return nil, errors.New("没有可用的客户端")
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

// 根据关键字搜索用户
func (c *ChannelsClient) SearchChannelsContact(keyword string, next_marker string) (*types.ChannelsContactSearchResp, error) {
	if keyword == "" {
		return nil, errors.New("keyword 不能为空")
	}
	clean_keyword := strings.TrimSpace(keyword)
	cache_key := "channels:contact_list:" + clean_keyword + ":" + next_marker
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(*types.ChannelsContactSearchResp); ok {
			return resp, nil
		}
	}
	fmt.Println("next_marker", next_marker)
	resp, err := c.RequestFrontend("key:channels:contact_list", types.ChannelsAccountSearchBody{Keyword: keyword, NextMarker: next_marker}, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var r types.ChannelsContactSearchResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, &r, 5*time.Minute)
	return &r, nil
}

// 获取指定用户的视频列表
func (c *ChannelsClient) FetchChannelsFeedListOfContact(username, next_marker string) (*types.ChannelsFeedListOfAccountResp, error) {
	clean_name := strings.TrimSpace(username)
	if !strings.HasSuffix(clean_name, "@finder") {
		clean_name += "@finder"
	}
	cache_key := "channels:feed_list:" + clean_name + ":" + next_marker
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(*types.ChannelsFeedListOfAccountResp); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:feed_list", types.ChannelsFeedListBody{Username: clean_name, NextMarker: next_marker}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var r types.ChannelsFeedListOfAccountResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, &r, 5*time.Minute)
	return &r, nil
}

// 获取指定用户的直播回放列表
func (c *ChannelsClient) FetchChannelsLiveReplayList(username, next_marker string) (*types.ChannelsFeedListOfAccountResp, error) {
	clean_name := strings.TrimSpace(username)
	if !strings.HasSuffix(clean_name, "@finder") {
		clean_name += "@finder"
	}
	cache_key := "channels:live_replay_list:" + clean_name + ":" + next_marker
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(*types.ChannelsFeedListOfAccountResp); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:live_replay_list", types.ChannelsLiveReplayListBody{Username: clean_name, NextMarker: next_marker}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var r types.ChannelsFeedListOfAccountResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, &r, 5*time.Minute)
	return &r, nil
}

// 获取用户 收藏或点赞 的视频列表
func (c *ChannelsClient) FetchChannelsInteractionedFeedList(flag, next_marker string) (*types.ChannelsFeedListOfAccountResp, error) {
	cache_key := "channels:interactioned_list:" + flag + ":" + next_marker
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(*types.ChannelsFeedListOfAccountResp); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:interactioned_list", types.ChannelsInteractionedFeedListBody{Flag: flag, NextMarker: next_marker}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var r types.ChannelsFeedListOfAccountResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, &r, 5*time.Minute)
	return &r, nil
}

// 获取指定视频详情
func (c *ChannelsClient) FetchChannelsFeedProfile(oid, uid, url, eid string) (*types.ChannelsFeedProfileResp, error) {
	// fmt.Println("[API]fetch feed profile", oid, uid)
	kk := fmt.Sprintf("%s:%s:%s:%s", oid, uid, url, eid)
	cache_key := "channels:feed_profile:" + kk
	if val, found := c.cache.Get(cache_key); found {
		if resp, ok := val.(*types.ChannelsFeedProfileResp); ok {
			return resp, nil
		}
	}
	resp, err := c.RequestFrontend("key:channels:feed_profile", types.ChannelsFeedProfileBody{ObjectId: oid, NonceId: uid, URL: url, EncryptedObjectId: eid}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	var r types.ChannelsFeedProfileResp
	if err := json.Unmarshal(resp.Data, &r); err != nil {
		return nil, err
	}
	c.cache.Set(cache_key, &r, 60*time.Minute)
	return &r, nil
}

// 刷新视频号页面
func (c *ChannelsClient) ReloadChannels() error {
	_, err := c.RequestFrontend("key:channels:reload", nil, 5*time.Second)
	return err
}

// 保存 channels feed profile 到数据库，返回 model.Content 实例
func (c *ChannelsClient) UpsertChannelsFeed(feed *types.ChannelsFeedProfile) (*model.Content, error) {
	if c.db == nil {
		return nil, errors.New("db is nil")
	}

	if feed == nil {
		return nil, errors.New("feed is nil")
	}
	if strings.TrimSpace(feed.ObjectId) == "" {
		return nil, errors.New("missing object_id")
	}
	if strings.TrimSpace(feed.URL) == "" {
		return nil, errors.New("missing url")
	}

	platformID := "wx_channels"
	now := util.NowMillis()

	acc := &model.Account{
		PlatformId: platformID,
		ExternalId: feed.Contact.Username,
		Username:   feed.Contact.Username,
		Nickname:   feed.Contact.Nickname,
		AvatarURL:  feed.Contact.AvatarURL,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	existingAccount := &model.Account{}
	if err := c.db.Where("platform_id = ? AND external_id = ?", platformID, acc.ExternalId).First(existingAccount).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := c.db.Create(acc).Error; err != nil {
				return nil, err
			}
			existingAccount = acc
		} else {
			return nil, err
		}
	}

	media := feed
	var existing model.Content
	if err := c.db.Where("platform_id = ? AND external_id = ?", platformID, media.ObjectId).First(&existing).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	pub := int64(media.CreatedAt)
	content := model.Content{
		PlatformId:  platformID,
		ContentType: "video",
		Title:       media.Title,
		ExternalId:  media.ObjectId,
		ExternalId2: media.NonceId,
		ExternalId3: media.DecryptKey,
		SourceURL:   media.SourceURL,
		ContentURL:  media.URL,
		URL:         media.SourceURL,
		CoverURL:    media.CoverURL,
		CoverWidth:  strconv.Itoa(media.CoverWidth),
		CoverHeight: strconv.Itoa(media.CoverHeight),
		Duration:    int64(media.Duration),
		Size:        int64(media.FileSize),
		PublishTime: &pub,
		Metadata:    fmt.Sprintf(`{"key":"%s"}`, media.DecryptKey),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if existing.Id == 0 {
		if err := c.db.Create(&content).Error; err != nil {
			return nil, err
		}
		ac := model.ContentAccount{
			AccountId: existingAccount.Id,
			ContentId: content.Id,
			Role:      "owner",
			CreatedAt: now,
		}
		if err := c.db.Create(&ac).Error; err != nil {
			return nil, err
		}
	} else {
		content.Id = existing.Id
		if err := c.db.Model(&model.Content{}).Where("id = ?", existing.Id).Updates(map[string]any{
			"title":        content.Title,
			"content_url":  content.ContentURL,
			"url":          content.URL,
			"cover_url":    content.CoverURL,
			"cover_width":  content.CoverWidth,
			"cover_height": content.CoverHeight,
			"duration":     content.Duration,
			"size":         content.Size,
			"update_time":  content.UpdateTime,
			"updated_at":   now,
		}).Error; err != nil {
			return nil, err
		}
	}
	return &content, nil
}
