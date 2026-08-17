package wxchannelsadapter

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/adapter"
	result "wx_channel/internal/apiresult"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/scraper/wxchannels"
)

const (
	ChannelsWebsocketPath = "/ws/channels"
	ChannelsStatusPath    = "/api/channels/status"
	PlayPath              = "/play"
)

// WebsocketRoutes owns the HTTP routes backed by the Channels scraper client.
type WebsocketRoutes struct {
	client *wxchannels.Client
}

func NewWebsocketRoutes(refresh_interval int, cookie_reader *cookies.Reader, sph_cookie string) *WebsocketRoutes {
	options := wxchannels.ClientOptions{
		RefreshInterval: refresh_interval,
		CookieReader:    cookie_reader,
		SphCookie:       sph_cookie,
	}
	client := wxchannels.NewClient(options)
	return &WebsocketRoutes{client: client}
}

// RegisterRoutes installs routes owned by this adapter.
func (r *WebsocketRoutes) RegisterRoutes(registrar adapter.RouteRegistrar) {
	if r == nil || r.client == nil || registrar == nil {
		return
	}
	registrar.RegisterGET(ChannelsWebsocketPath, r.handle_websocket)
	registrar.RegisterGET(ChannelsStatusPath, r.HandleStatus)
	registrar.RegisterGET(PlayPath, r.HandlePlay)
	registrar.RegisterGET("/api/channels/parse_sph", r.HandleParseSph)
	registrar.RegisterPOST("/api/channels/decrypt", r.HandleDecryptVideo)
	registrar.RegisterGET("/api/channels/contact/search", r.HandleSearchChannelsContact)
	registrar.RegisterGET("/api/channels/contact/feed/list", r.HandleFetchFeedListOfContact)
	registrar.RegisterGET("/api/channels/feed/profile", r.HandleFetchFeedProfile)
	registrar.RegisterGET("/api/channels/live/replay/list", r.HandleFetchLiveReplayList)
	registrar.RegisterGET("/api/channels/interactioned/list", r.HandleFetchInteractionedFeedList)
	registrar.RegisterGET("/api/channels/follow/list", r.HandleFetchFollowList)
	registrar.RegisterGET("/api/channels/play/history", r.HandleFetchPlayHistory)
	registrar.RegisterGET("/api/channels/postprocess/flows", r.HandleFetchPostprocessFlows)
	registrar.RegisterGET("/api/channels/feed/share_url", r.HandleFetchFeedShareUrl)
	registrar.RegisterGET("/api/channels/shared_feed/profile", r.HandleFetchSharedFeedProfile)
	registrar.RegisterGET("/api/channels/feed/comment/list", r.HandleFetchFeedCommentList)
	registrar.RegisterGET("/rss/channels", r.HandleFetchFeedListOfContactRSS)
}

func (r *WebsocketRoutes) handle_websocket(ctx *gin.Context) {
	r.client.ServeWebsocket(ctx.Writer, ctx.Request)
}

// HandleStatus reports whether a Channels page is connected and can receive
// frontend API requests through channels.ws.js.
func (r *WebsocketRoutes) HandleStatus(ctx *gin.Context) {
	available := r != nil && r.client != nil && r.client.Available()
	result.Ok(ctx, gin.H{"available": available})
}

// HandlePlay proxies or decrypts a remote Channels video stream.
func (r *WebsocketRoutes) HandlePlay(ctx *gin.Context) {
	target_url := ctx.Query("url")
	if target_url == "" {
		result.Err(ctx, 400, "missing targetURL")
		return
	}
	if !strings.HasPrefix(target_url, "http") {
		target_url = "https://" + target_url
	}
	if _, err := url.Parse(target_url); err != nil {
		result.Err(ctx, 400, "Invalid URL")
		return
	}

	decrypt_key_str := ctx.Query("key")
	decryptor := wxchannels.NewChannelsVideoDecryptor()
	if decrypt_key_str != "" {
		decrypt_key, err := strconv.ParseUint(decrypt_key_str, 0, 64)
		if err != nil {
			result.Err(ctx, 400, "invalid decryptKey")
			return
		}
		decryptor.DecryptOnlyInline(ctx.Writer, ctx.Request, target_url, decrypt_key, 131072)
		return
	}
	decryptor.SimpleProxy(target_url, ctx.Writer, ctx.Request)
}

// HandleFetchPostprocessFlows returns wxchannels postprocess flow configs for read-only visualization.
func (r *WebsocketRoutes) HandleFetchPostprocessFlows(ctx *gin.Context) {
	flow_id := ctx.Query("flow_id")
	payload, err := GetWXChannelsPostprocessFlowVisualization(flow_id)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}

	result.Ok(ctx, payload)
}

// HandleParseSph parses an SPH share link to retrieve video information.
func (r *WebsocketRoutes) HandleParseSph(ctx *gin.Context) {
	share_url := ctx.Query("url")
	if share_url == "" {
		result.Err(ctx, 400, "url parameter is required")
		return
	}

	raw_result, err := r.client.Fetch(wxchannels.FetchParams{URL: share_url})
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	raw_resp, ok := raw_result.(json.RawMessage)
	if !ok {
		result.Err(ctx, 500, "unexpected shared profile response")
		return
	}

	// Parse feedInfo, add originVideoUrl, pass through other fields as-is
	var data map[string]interface{}
	if err := json.Unmarshal(raw_resp, &data); err == nil {
		if data_wrap, ok := data["data"].(map[string]interface{}); ok {
			if feed_info, ok := data_wrap["feedInfo"].(map[string]interface{}); ok {
				if video_url, ok := feed_info["videoUrl"].(string); ok && video_url != "" {
					feed_info["originVideoUrl"] = wxchannels.CleanVideoURL(video_url)
				}
				// Pre-store a copy of videoUrl for later use
				if _, ok := feed_info["originVideoUrl"]; !ok {
					feed_info["originVideoUrl"] = ""
				}
			}
		}
		result.Ok(ctx, data)
		return
	}

	// If parsing fails, pass through the raw response directly
	result.Ok(ctx, json.RawMessage(raw_resp))
}

// HandleSearchChannelsContact searches for Channels authors.
func (r *WebsocketRoutes) HandleSearchChannelsContact(ctx *gin.Context) {
	keyword := ctx.Query("keyword")
	next_marker := ctx.Query("next_marker")

	resp, err := r.client.SearchChannelsContact(keyword, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

// HandleFetchFeedListOfContact fetches the video list for a given user.
func (r *WebsocketRoutes) HandleFetchFeedListOfContact(ctx *gin.Context) {
	username := ctx.Query("username")
	next_marker := ctx.Query("next_marker")

	resp, err := r.client.FetchChannelsFeedListOfContact(username, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

// HandleFetchLiveReplayList fetches the live replay list for a given user.
func (r *WebsocketRoutes) HandleFetchLiveReplayList(ctx *gin.Context) {
	username := ctx.Query("username")
	next_marker := ctx.Query("next_marker")

	resp, err := r.client.FetchChannelsLiveReplayList(username, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

// HandleFetchInteractionedFeedList fetches the user's favorited or liked video list.
func (r *WebsocketRoutes) HandleFetchInteractionedFeedList(ctx *gin.Context) {
	flag := ctx.Query("flag")
	next_marker := ctx.Query("next_marker")

	resp, err := r.client.FetchChannelsInteractionedFeedList(flag, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

// HandleFetchFollowList fetches the user's following list.
func (r *WebsocketRoutes) HandleFetchFollowList(ctx *gin.Context) {
	result.Ok(ctx, nil)
}

// HandleFetchPlayHistory fetches the user's watch history.
func (r *WebsocketRoutes) HandleFetchPlayHistory(ctx *gin.Context) {
	next_marker := ctx.Query("next_marker")

	resp, err := r.client.FetchChannelsPlayHistory(next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

// HandleFetchFeedCommentList fetches the video comment list.
func (r *WebsocketRoutes) HandleFetchFeedCommentList(ctx *gin.Context) {
}

// HandleFetchFeedShareUrl fetches the video share link.
func (r *WebsocketRoutes) HandleFetchFeedShareUrl(ctx *gin.Context) {
	oid := ctx.Query("oid")
	if oid == "" {
		result.Err(ctx, 400, "missing oid")
		return
	}
	result.Err(ctx, 400, "need to process")
}

// HandleFetchFeedProfile fetches details for a given video.
func (r *WebsocketRoutes) HandleFetchFeedProfile(ctx *gin.Context) {
	oid := ctx.Query("oid")
	nid := ctx.Query("nid")
	req_url := ctx.Query("url")
	eid := ctx.Query("eid")

	if eid == "" && req_url != "" {
		if parsed_url, err := url.Parse(req_url); err == nil {
			if _eid := parsed_url.Query().Get("eid"); _eid != "" {
				eid = _eid
				req_url = ""
			}
		}
	}
	// When oid/nid are provided directly, clear reqUrl to avoid browser-side timeout from relative URL parsing
	if oid != "" && nid != "" {
		req_url = ""
	}

	resp, err := r.client.FetchChannelsFeedProfile(oid, nid, req_url, eid)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

// HandleFetchSharedFeedProfile fetches shared video details.
func (r *WebsocketRoutes) HandleFetchSharedFeedProfile(ctx *gin.Context) {
	req_url := ctx.Query("url")
	if req_url == "" {
		result.Err(ctx, 400, "missing url")
		return
	}
	result.Err(ctx, 400, "need to process")
}

// RSS types
type atom_author struct {
	Name string `xml:"name"`
}

type atom_link struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

type atom_content struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

type atom_entry struct {
	Title     string       `xml:"title"`
	ID        string       `xml:"id"`
	Updated   string       `xml:"updated"`
	Published string       `xml:"published"`
	Link      []atom_link  `xml:"link"`
	Content   atom_content `xml:"content"`
	Author    atom_author  `xml:"author"`
}

type atom_feed struct {
	XMLName xml.Name     `xml:"http://www.w3.org/2005/Atom feed"`
	Title   string       `xml:"title"`
	ID      string       `xml:"id"`
	Updated string       `xml:"updated"`
	Link    []atom_link  `xml:"link"`
	Author  atom_author  `xml:"author"`
	Entry   []atom_entry `xml:"entry"`
}

// HandleFetchFeedListOfContactRSS returns an RSS feed for Channels videos.
func (r *WebsocketRoutes) HandleFetchFeedListOfContactRSS(ctx *gin.Context) {
	username := ctx.Query("username")
	next_marker := ctx.Query("next_marker")

	_, err := r.client.FetchChannelsFeedListOfContact(username, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}

	atom := atom_feed{
		Title:   "WeChat Channels",
		ID:      username,
		Updated: time.Now().Format(time.RFC3339),
		Link: []atom_link{
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
		result.Err(ctx, 400, "filepath parameter is required")
		return
	}
	key, err := strconv.ParseUint(ctx.Query("key"), 10, 64)
	if err != nil || key == 0 {
		result.Err(ctx, 400, "key parameter is required and must be a non-zero integer")
		return
	}

	data, err := os.ReadFile(filepath)
	if err != nil {
		result.Err(ctx, 400, "failed to read file: "+err.Error())
		return
	}

	wxchannels.DecryptData(data, 131072, key)

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		result.Err(ctx, 400, "failed to write file: "+err.Error())
		return
	}

	result.Ok(ctx, gin.H{"filepath": filepath})
}

func (r *WebsocketRoutes) Stop() {
	if r != nil && r.client != nil {
		r.client.Stop()
	}
}
