package api

import (
	"net/url"
	"path/filepath"
	"strconv"

	"github.com/GopeedLab/gopeed/pkg/base"
	gopeedhttp "github.com/GopeedLab/gopeed/pkg/protocol/http"
	"github.com/gin-gonic/gin"

	result "wx_channel/internal/util"
	"wx_channel/pkg/system"
)

func (c *APIClient) handleSearchChannelsContact(ctx *gin.Context) {
	keyword := ctx.Query("keyword")
	next_marker := ctx.Query("next_marker")
	resp, err := c.channels.SearchChannelsContact(keyword, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

func (c *APIClient) handleFetchFeedListOfContact(ctx *gin.Context) {
	username := ctx.Query("username")
	next_marker := ctx.Query("next_marker")
	resp, err := c.channels.FetchChannelsFeedListOfContact(username, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

func (c *APIClient) handleFetchLiveReplayList(ctx *gin.Context) {
	username := ctx.Query("username")
	next_marker := ctx.Query("next_marker")
	resp, err := c.channels.FetchChannelsLiveReplayList(username, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

func (c *APIClient) handleFetchInteractionedFeedList(ctx *gin.Context) {
	flag := ctx.Query("flag")
	next_marker := ctx.Query("next_marker")
	resp, err := c.channels.FetchChannelsInteractionedFeedList(flag, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

func (c *APIClient) handleFetchFeedProfile(ctx *gin.Context) {
	oid := ctx.Query("oid")
	uid := ctx.Query("nid")
	_url := ctx.Query("url")
	eid := ctx.Query("eid")
	if eid == "" && _url != "" {
		if parsedURL, err := url.Parse(_url); err == nil {
			if _eid := parsedURL.Query().Get("eid"); _eid != "" {
				eid = _eid
				_url = ""
			}
		}
	}
	resp, err := c.channels.FetchChannelsFeedProfile(oid, uid, _url, eid)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

type ChannelsDownloadPayload struct {
	Oid   string `json:"oid"`
	Nid   string `json:"nid"`
	Eid   string `json:"eid"`
	URL   string `json:"url"`
	MP3   bool   `json:"mp3"`
	Cover bool   `json:"cover"`
}

func (c *APIClient) handleCreateChannelsTask(ctx *gin.Context) {
	var body ChannelsDownloadPayload
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.Oid == "" && body.Nid == "" && body.URL == "" && body.Eid == "" {
		result.Err(ctx, 400, "缺少参数")
		return
	}
	if body.Eid == "" && body.URL != "" {
		if parsedURL, err := url.Parse(body.URL); err == nil {
			if eid := parsedURL.Query().Get("eid"); eid != "" {
				body = ChannelsDownloadPayload{
					Eid: eid,
				}
			}
		}
	}
	payload, err := c.createFeedTaskBody(body.Oid, body.Nid, body.URL, body.Eid, body.MP3, body.Cover)
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}

	if payload.Id == "" {
		result.Err(ctx, 400, "缺少 feed id 参数")
		return
	}
	if payload.Suffix == ".mp3" {
		hasFFmpeg := system.ExistingCommand("ffmpeg")
		if !hasFFmpeg {
			result.Err(ctx, 3001, "下载 mp3 需要支持 ffmpeg 命令")
			return
		}
	}
	tasks := c.downloader.GetTasks()
	existing := c.check_existing_feed(tasks, payload)
	if existing {
		result.Err(ctx, 409, "已存在该下载内容")
		return
	}
	filename, dir, err := c.formatter.ProcessFilename(payload.Filename)
	if err != nil {
		result.Err(ctx, 409, "不合法的文件名，"+err.Error())
		return
	}
	connections := c.resolve_connections(payload.URL)
	id, err := c.downloader.CreateDirect(
		&base.Request{
			URL: payload.URL,
			Labels: map[string]string{
				"id":     payload.Id,
				"title":  payload.Title,
				"key":    strconv.Itoa(payload.Key),
				"spec":   payload.Spec,
				"suffix": payload.Suffix,
			},
		},
		&base.Options{
			Name: filename + payload.Suffix,
			Path: filepath.Join(c.cfg.DownloadDir, dir),
			Extra: &gopeedhttp.OptsExtra{
				Connections: connections,
			},
		},
	)
	if err != nil {
		result.Err(ctx, 500, "下载失败")
		return
	}
	task := c.downloader.GetTask(id)
	if task != nil {
		c.downloader_ws.Broadcast(APIClientWSMessage{
			Type: "event",
			Data: map[string]interface{}{
				"task": task,
			},
		})
	}
	result.Ok(ctx, gin.H{"id": id})
}

