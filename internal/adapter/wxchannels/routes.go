package wxchannels

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"wx_channel/internal/util"
	scraper "wx_channel/pkg/scraper/wxchannels"
)

// ChannelsTaskCreator creates a download task from channels feed object JSON.
// feedObject is the JSON-serialized ChannelsObject from the feed profile.
type ChannelsTaskCreator func(feedObject json.RawMessage, savePath, filename, spec string, downloadCover, overwrite, duplicate, convertMP3 bool) (any, error)

const ChannelsWebsocketPath = "/ws/channels"

// RouteRegistrar is the narrow HTTP capability required by this adapter. It
// keeps the adapter independent from the API package and its APIClient type.
type RouteRegistrar interface {
	RegisterGET(path string, handler gin.HandlerFunc)
	RegisterPOST(path string, handler gin.HandlerFunc)
}

// WebsocketRoutes owns the video-channel browser websocket endpoint and its
// scraper client lifecycle.
type WebsocketRoutes struct {
	client             *scraper.ChannelsClient
	sphCookie          string
	remoteServerMode   bool
	createDownloadTask ChannelsTaskCreator
}

func NewWebsocketRoutes(refreshInterval int, db *gorm.DB, sphCookie string, remoteServerMode bool, createDownloadTask ChannelsTaskCreator) *WebsocketRoutes {
	client := scraper.NewChannelsClient(refreshInterval)
	client.SetDB(db)
	return &WebsocketRoutes{client: client, sphCookie: sphCookie, remoteServerMode: remoteServerMode, createDownloadTask: createDownloadTask}
}

// RegisterRoutes installs routes owned by this adapter.
func (r *WebsocketRoutes) RegisterRoutes(registrar RouteRegistrar) {
	if r == nil || r.client == nil || registrar == nil {
		return
	}
	registrar.RegisterGET(ChannelsWebsocketPath, r.client.HandleChannelsWebsocket)
	registrar.RegisterGET("/api/channels/parse_sph", r.HandleParseSph)
	registrar.RegisterPOST("/api/channels/decrypt", r.HandleDecryptVideo)
	registrar.RegisterGET("/api/channels/contact/search", r.HandleSearchChannelsContact)
	registrar.RegisterGET("/api/channels/contact/feed/list", r.HandleFetchFeedListOfContact)
	registrar.RegisterGET("/api/channels/feed/profile", r.HandleFetchFeedProfile)
	registrar.RegisterGET("/api/channels/live/replay/list", r.HandleFetchLiveReplayList)
	registrar.RegisterGET("/api/channels/interactioned/list", r.HandleFetchInteractionedFeedList)
	registrar.RegisterGET("/api/channels/follow/list", r.HandleFetchFollowList)
	registrar.RegisterGET("/api/channels/play/history", r.HandleFetchPlayHistory)
	registrar.RegisterGET("/api/channels/feed/share_url", r.HandleFetchFeedShareUrl)
	registrar.RegisterGET("/api/channels/shared_feed/profile", r.HandleFetchSharedFeedProfile)
	registrar.RegisterGET("/api/channels/feed/comment/list", r.HandleFetchFeedCommentList)
	registrar.RegisterGET("/rss/channels", r.HandleFetchFeedListOfContactRSS)
	registrar.RegisterGET("/api/channels/download_task/create", r.HandleChannelsCreateDownloadTask)
}

// HandleParseSph parses an SPH share link to retrieve video information.
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

	rawResp, err := scraper.FetchVideoProfileWithShareUrl(shareUrl, cookie)
	if err != nil {
		util.Err(ctx, 400, err.Error())
		return
	}

	// Parse feedInfo, add originVideoUrl, pass through other fields as-is
	var data map[string]interface{}
	if err := json.Unmarshal(rawResp, &data); err == nil {
		if dataWrap, ok := data["data"].(map[string]interface{}); ok {
			if feedInfo, ok := dataWrap["feedInfo"].(map[string]interface{}); ok {
				if videoUrl, ok := feedInfo["videoUrl"].(string); ok && videoUrl != "" {
					feedInfo["originVideoUrl"] = scraper.CleanVideoURL(videoUrl)
				}
				// Pre-store a copy of videoUrl for later use
				if _, ok := feedInfo["originVideoUrl"]; !ok {
					feedInfo["originVideoUrl"] = ""
				}
			}
		}
		util.Ok(ctx, data)
		return
	}

	// If parsing fails, pass through the raw response directly
	util.Ok(ctx, json.RawMessage(rawResp))
}

// HandleSearchChannelsContact searches for Channels authors.
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

// HandleFetchFeedListOfContact fetches the video list for a given user.
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

// HandleFetchLiveReplayList fetches the live replay list for a given user.
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

// HandleFetchInteractionedFeedList fetches the user's favorited or liked video list.
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

// HandleFetchFollowList fetches the user's following list.
func (r *WebsocketRoutes) HandleFetchFollowList(ctx *gin.Context) {
	util.Ok(ctx, nil)
}

// HandleFetchPlayHistory fetches the user's watch history.
func (r *WebsocketRoutes) HandleFetchPlayHistory(ctx *gin.Context) {
	nextMarker := ctx.Query("next_marker")

	resp, err := r.client.FetchChannelsPlayHistory(nextMarker)
	if err != nil {
		util.Err(ctx, 400, err.Error())
		return
	}
	util.Ok(ctx, resp)
}

// HandleFetchFeedCommentList fetches the video comment list.
func (r *WebsocketRoutes) HandleFetchFeedCommentList(ctx *gin.Context) {
}

// HandleFetchFeedShareUrl fetches the video share link.
func (r *WebsocketRoutes) HandleFetchFeedShareUrl(ctx *gin.Context) {
	oid := ctx.Query("oid")
	if oid == "" {
		util.Err(ctx, 400, "missing oid")
		return
	}
	util.Err(ctx, 400, "need to process")
}

// HandleFetchFeedProfile fetches details for a given video.
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
	// When oid/nid are provided directly, clear reqUrl to avoid browser-side timeout from relative URL parsing
	if oid != "" && nid != "" {
		reqUrl = ""
	}

	resp, err := r.client.FetchChannelsFeedProfile(oid, nid, reqUrl, eid)
	if err != nil {
		util.Err(ctx, 400, err.Error())
		return
	}
	util.Ok(ctx, resp)
}

// HandleChannelsCreateDownloadTask fetches Channels feed details and creates a download task.
// GET /api/channels/download_task/create
func (r *WebsocketRoutes) HandleChannelsCreateDownloadTask(ctx *gin.Context) {
	if r.createDownloadTask == nil {
		util.Err(ctx, 500, "下载任务服务未初始化")
		return
	}

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
	if oid != "" && nid != "" {
		reqUrl = ""
	}

	resp, err := r.client.FetchChannelsFeedProfile(oid, nid, reqUrl, eid)
	if err != nil {
		util.Err(ctx, 400, err.Error())
		return
	}

	contentJSON, err := json.Marshal(resp.Data.Object)
	if err != nil {
		util.Err(ctx, 500, "序列化 feed 数据失败: "+err.Error())
		return
	}

	savePath := ctx.Query("save_path")
	filename := ctx.Query("filename")
	spec := ctx.Query("spec")
	downloadCover := ctx.Query("download_cover") == "true"
	overwrite := ctx.Query("overwrite") == "true"
	duplicate := ctx.Query("duplicate") == "true"
	convertMP3 := ctx.Query("convert_mp3") == "true"

	result, err := r.createDownloadTask(contentJSON, savePath, filename, spec, downloadCover, overwrite, duplicate, convertMP3)
	if err != nil {
		code := 400
		var dupErr duplicateTaskError
		if errors.As(err, &dupErr) {
			code = dupErr.StatusCode()
		}
		util.Err(ctx, code, err.Error())
		return
	}

	util.Ok(ctx, result)
}

// duplicateTaskError is used to identify duplicate task errors.
type duplicateTaskError interface {
	StatusCode() int
}

// HandleFetchSharedFeedProfile fetches shared video details.
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
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title   string      `xml:"title"`
	ID      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Link    []atomLink  `xml:"link"`
	Author  atomAuthor  `xml:"author"`
	Entry   []atomEntry `xml:"entry"`
}

// HandleFetchFeedListOfContactRSS returns an RSS feed for Channels videos.
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

// HandleDecryptVideo decrypts a local encrypted video in place.
func (r *WebsocketRoutes) HandleDecryptVideo(ctx *gin.Context) {
	filepath := ctx.Query("filepath")
	if filepath == "" {
		util.Err(ctx, 400, "filepath parameter is required")
		return
	}
	key, err := strconv.Atoi(ctx.Query("key"))
	if err != nil || key == 0 {
		util.Err(ctx, 400, "key parameter is required and must be a non-zero integer")
		return
	}

	data, err := os.ReadFile(filepath)
	if err != nil {
		util.Err(ctx, 400, "failed to read file: "+err.Error())
		return
	}

	scraper.DecryptData(data, 131072, uint64(key))

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		util.Err(ctx, 400, "failed to write file: "+err.Error())
		return
	}

	util.Ok(ctx, gin.H{"filepath": filepath})
}

func (r *WebsocketRoutes) Stop() {
	if r != nil && r.client != nil {
		r.client.Stop()
	}
}
