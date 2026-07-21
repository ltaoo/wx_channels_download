package wxchannels

import (
	"encoding/xml"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"wx_channel/internal/util"
	scraper "wx_channel/pkg/scraper/wxchannels"
)

const ChannelsWebsocketPath = "/ws/channels"

// RouteRegistrar is the narrow HTTP capability required by this adapter. It
// keeps the adapter independent from the API package and its APIClient type.
type RouteRegistrar interface {
	RegisterGET(path string, handler gin.HandlerFunc)
}

// WebsocketRoutes owns the video-channel browser websocket endpoint and its
// scraper client lifecycle.
type WebsocketRoutes struct {
	client            *scraper.ChannelsClient
	sphCookie         string
	remoteServerMode  bool
}

func NewWebsocketRoutes(refreshInterval int, db *gorm.DB, sphCookie string, remoteServerMode bool) *WebsocketRoutes {
	client := scraper.NewChannelsClient(refreshInterval)
	client.SetDB(db)
	return &WebsocketRoutes{client: client, sphCookie: sphCookie, remoteServerMode: remoteServerMode}
}

// RegisterRoutes installs routes owned by this adapter.
func (r *WebsocketRoutes) RegisterRoutes(registrar RouteRegistrar) {
	if r == nil || r.client == nil || registrar == nil {
		return
	}
	registrar.RegisterGET(ChannelsWebsocketPath, r.client.HandleChannelsWebsocket)
	registrar.RegisterGET("/api/channels/parse_sph", r.HandleParseSph)

	if !r.remoteServerMode {
		// 视频号接口（仅本地模式）
		registrar.RegisterGET("/api/channels/contact/search", r.HandleSearchChannelsContact)
		registrar.RegisterGET("/api/channels/contact/feed/list", r.HandleFetchFeedListOfContact)
		registrar.RegisterGET("/api/channels/feed/profile", r.HandleFetchFeedProfile)
		registrar.RegisterGET("/api/channels/live/replay/list", r.HandleFetchLiveReplayList)
		registrar.RegisterGET("/api/channels/interactioned/list", r.HandleFetchInteractionedFeedList)
		registrar.RegisterGET("/api/channels/follow/list", r.HandleFetchFollowList)
		registrar.RegisterGET("/api/channels/feed/share_url", r.HandleFetchFeedShareUrl)
		registrar.RegisterGET("/api/channels/shared_feed/profile", r.HandleFetchSharedFeedProfile)
		registrar.RegisterGET("/api/channels/feed/comment/list", r.HandleFetchFeedCommentList)
		registrar.RegisterGET("/rss/channels", r.HandleFetchFeedListOfContactRSS)
	}
}

// HandleParseSph 解析 SPH 分享链接，获取视频信息
func (r *WebsocketRoutes) HandleParseSph(ctx *gin.Context) {
	shareUrl := ctx.Query("url")
	if shareUrl == "" {
		util.Err(ctx, 400, "url parameter is required")
		return
	}

	cookie := r.sphCookie
	if cookie == "" {
		util.Err(ctx, 400, "cloudflare.sphCookie not configured")
		return
	}

	feedResp, err := scraper.FetchVideoProfileWithShareUrl(shareUrl, cookie)
	if err != nil {
		util.Err(ctx, 400, err.Error())
		return
	}

	// 处理 video URL：仅保留 encfilekey 和 token 参数，存储为 originVideoUrl
	if feedResp != nil && feedResp.Data.Feedinfo.Videourl != "" {
		feedResp.Data.Feedinfo.OriginVideoUrl = scraper.CleanVideoURL(feedResp.Data.Feedinfo.Videourl)
	}

	util.Ok(ctx, feedResp)
}

// HandleSearchChannelsContact 搜索视频号作者
func (r *WebsocketRoutes) HandleSearchChannelsContact(ctx *gin.Context) {
	keyword := ctx.Query("keyword")
	nextMarker := ctx.Query("next_marker")

	resp, err := r.client.SearchChannelsContact(keyword, nextMarker)
	if err != nil {
		util.Err(ctx, 400, err.Error())
		return
	}
	util.Ok(ctx, resp)
}

// HandleFetchFeedListOfContact 获取指定用户的视频列表
func (r *WebsocketRoutes) HandleFetchFeedListOfContact(ctx *gin.Context) {
	username := ctx.Query("username")
	nextMarker := ctx.Query("next_marker")

	resp, err := r.client.FetchChannelsFeedListOfContact(username, nextMarker)
	if err != nil {
		util.Err(ctx, 400, err.Error())
		return
	}
	util.Ok(ctx, resp)
}

// HandleFetchLiveReplayList 获取指定用户的直播回放列表
func (r *WebsocketRoutes) HandleFetchLiveReplayList(ctx *gin.Context) {
	username := ctx.Query("username")
	nextMarker := ctx.Query("next_marker")

	resp, err := r.client.FetchChannelsLiveReplayList(username, nextMarker)
	if err != nil {
		util.Err(ctx, 400, err.Error())
		return
	}
	util.Ok(ctx, resp)
}

// HandleFetchInteractionedFeedList 获取用户收藏或点赞的视频列表
func (r *WebsocketRoutes) HandleFetchInteractionedFeedList(ctx *gin.Context) {
	flag := ctx.Query("flag")
	nextMarker := ctx.Query("next_marker")

	resp, err := r.client.FetchChannelsInteractionedFeedList(flag, nextMarker)
	if err != nil {
		util.Err(ctx, 400, err.Error())
		return
	}
	util.Ok(ctx, resp)
}

// HandleFetchFollowList 获取用户关注列表
func (r *WebsocketRoutes) HandleFetchFollowList(ctx *gin.Context) {
	util.Ok(ctx, nil)
}

// HandleFetchFeedCommentList 获取视频评论列表
func (r *WebsocketRoutes) HandleFetchFeedCommentList(ctx *gin.Context) {
}

// HandleFetchFeedShareUrl 获取视频分享链接
func (r *WebsocketRoutes) HandleFetchFeedShareUrl(ctx *gin.Context) {
	oid := ctx.Query("oid")
	if oid == "" {
		util.Err(ctx, 400, "missing oid")
		return
	}
	util.Err(ctx, 400, "need to process")
}

// HandleFetchFeedProfile 获取指定视频详情
func (r *WebsocketRoutes) HandleFetchFeedProfile(ctx *gin.Context) {
	oid := ctx.Query("oid")
	nid := ctx.Query("nid")
	reqUrl := ctx.Query("url")
	eid := ctx.Query("eid")

	if eid == "" && reqUrl != "" {
		if parsedURL, err := url.Parse(reqUrl); err == nil {
			if _eid := parsedURL.Query().Get("eid"); _eid != "" {
				eid = _eid
				reqUrl = ""
			}
		}
	}

	resp, err := r.client.FetchChannelsFeedProfile(oid, nid, reqUrl, eid)
	if err != nil {
		util.Err(ctx, 400, err.Error())
		return
	}
	util.Ok(ctx, resp)
}

// HandleFetchSharedFeedProfile 获取分享视频详情
func (r *WebsocketRoutes) HandleFetchSharedFeedProfile(ctx *gin.Context) {
	reqUrl := ctx.Query("url")
	if reqUrl == "" {
		util.Err(ctx, 400, "missing url")
		return
	}
	util.Err(ctx, 400, "need to process")
}

// RSS types
type atomAuthor struct {
	Name string `xml:"name"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

type atomContent struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

type atomEntry struct {
	Title     string      `xml:"title"`
	ID        string      `xml:"id"`
	Updated   string      `xml:"updated"`
	Published string      `xml:"published"`
	Link      []atomLink  `xml:"link"`
	Content   atomContent `xml:"content"`
	Author    atomAuthor  `xml:"author"`
}

type atomFeed struct {
	XMLName xml.Name   `xml:"http://www.w3.org/2005/Atom feed"`
	Title   string     `xml:"title"`
	ID      string     `xml:"id"`
	Updated string     `xml:"updated"`
	Link    []atomLink `xml:"link"`
	Author  atomAuthor `xml:"author"`
	Entry   []atomEntry `xml:"entry"`
}

// HandleFetchFeedListOfContactRSS 返回视频号视频的 RSS 订阅
func (r *WebsocketRoutes) HandleFetchFeedListOfContactRSS(ctx *gin.Context) {
	username := ctx.Query("username")
	nextMarker := ctx.Query("next_marker")

	_, err := r.client.FetchChannelsFeedListOfContact(username, nextMarker)
	if err != nil {
		util.Err(ctx, 400, err.Error())
		return
	}

	atom := atomFeed{
		Title:   "WeChat Channels",
		ID:      username,
		Updated: time.Now().Format(time.RFC3339),
		Link: []atomLink{
			{Rel: "self", Href: "http://" + ctx.Request.Host + ctx.Request.RequestURI},
			{Rel: "alternate", Href: "https://channels.weixin.qq.com"},
		},
	}
	ctx.Header("Content-Type", "application/atom+xml; charset=utf-8")
	ctx.XML(http.StatusOK, atom)
}

func (r *WebsocketRoutes) Stop() {
	if r != nil && r.client != nil {
		r.client.Stop()
	}
}
