package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/GopeedLab/gopeed/pkg/base"
	"github.com/gin-gonic/gin"

	result "wx_channel/internal/util"
	"wx_channel/pkg/system"
)

// 搜索视频号作者
func (c *APIClient) handleSearchChannelsContact(ctx *gin.Context) {
	keyword := ctx.Query("keyword")
	next_marker := ctx.Query("next_marker")

	// Use service
	resp, err := c.channelsService.SearchContact(keyword, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

// 获取指定用户的视频列表
func (c *APIClient) handleFetchFeedListOfContact(ctx *gin.Context) {
	username := ctx.Query("username")
	next_marker := ctx.Query("next_marker")

	// Use service
	resp, err := c.channelsService.FetchFeedList(username, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

// 获取指定用户的直播回放列表
func (c *APIClient) handleFetchLiveReplayList(ctx *gin.Context) {
	username := ctx.Query("username")
	next_marker := ctx.Query("next_marker")

	// Use service
	resp, err := c.channelsService.FetchLiveReplayList(username, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

// 获取用户 收藏或点赞 的视频列表
func (c *APIClient) handleFetchInteractionedFeedList(ctx *gin.Context) {
	flag := ctx.Query("flag")
	next_marker := ctx.Query("next_marker")

	// Use service
	resp, err := c.channelsService.FetchInteractionedFeedList(flag, next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

func (c *APIClient) handleFetchFollowList(ctx *gin.Context) {
	next_marker := ctx.Query("next_marker")
	resp, err := c.channels.FetchChannelsFollowList(next_marker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

func (c *APIClient) handleFetchFeedCommentList(ctx *gin.Context) {
	oid := ctx.Query("oid")
	nid := ctx.Query("nid")
	commentID := ctx.Query("comment_id")
	nextMarker := ctx.Query("next_marker")
	resp, err := c.channels.FetchChannelsFeedCommentList(oid, nid, commentID, nextMarker)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

func (c *APIClient) handleFetchFeedShareUrl(ctx *gin.Context) {
	oid := ctx.Query("oid")
	if oid == "" {
		result.Err(ctx, 400, "missing oid")
		return
	}
	resp, err := c.channels.FetchChannelsFeedShareUrl(oid)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

// 获取指定视频详情
func (c *APIClient) handleFetchFeedProfile(ctx *gin.Context) {
	oid := ctx.Query("oid")
	uid := ctx.Query("nid")
	_url := ctx.Query("url")
	eid := ctx.Query("eid")

	// Use service
	resp, err := c.channelsService.FetchFeedProfile(oid, uid, _url, eid)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, resp)
}

// 获取分享视频详情
func (c *APIClient) handleFetchSharedFeedProfile(ctx *gin.Context) {
	_url := ctx.Query("url")
	if _url == "" {
		result.Err(ctx, 400, "missing url")
		return
	}
	resp, err := c.channels.FetchChannelsSharedFeedProfile(_url)
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
	Spec  string `json:"spec"`
	MP3   bool   `json:"mp3"`
	Cover bool   `json:"cover"`
}

// 创建视频号视频下载任务
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
				body.Eid = eid
			}
		}
	}

	payload, content, account, err := c.createFeedTaskBody(body.Oid, body.Nid, body.URL, body.Eid, body.MP3, body.Cover, body.Spec)
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

	if c.cfg.RemoteServerEnabled {
		protocol := c.cfg.RemoteServerProtocol
		if protocol == "" {
			protocol = "http"
		}
		targetURL := fmt.Sprintf("%s://%s:%d/api/task/create", protocol, c.cfg.RemoteServerHostname, c.cfg.RemoteServerPort)

		jsonData, err := json.Marshal(payload)
		if err != nil {
			result.Err(ctx, 500, "序列化请求参数失败")
			return
		}

		resp, err := http.Post(targetURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			result.Err(ctx, 500, "请求远程服务器失败: "+err.Error())
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			result.Err(ctx, 500, fmt.Sprintf("远程服务器创建任务失败, status: %d, body: %s", resp.StatusCode, string(bodyBytes)))
			return
		}

		var respBody struct {
			Id string `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
			result.Err(ctx, 500, "解析远程服务器响应失败")
			return
		}

		result.Ok(ctx, gin.H{"id": respBody.Id})
		return
	}

	// Use download service
	existing := c.downloadService.CheckExisting(payload.Id, payload.Spec, payload.Suffix)
	if existing {
		result.Err(ctx, 409, "已存在该下载内容")
		return
	}

	opts, err := c.downloadService.BuildTaskOpts(payload)
	if err != nil {
		result.Err(ctx, 409, "不合法的文件名，"+err.Error())
		return
	}

	labels := c.downloadService.BuildTaskLabels(payload)
	id, err := c.downloadService.CreateTask(&base.Request{
		URL:    payload.URL,
		Labels: labels,
	}, opts)
	if err != nil {
		result.Err(ctx, 500, "下载失败")
		return
	}
	task := c.downloader.GetTask(id)
	if task != nil && content != nil && account != nil && c.channelsUploadService != nil {
		ct, err := c.channelsUploadService.HandleChannelsFeed(content, account)
		if err != nil {
			c.logger.Warn().Err(err).Msg("HandleChannelsFeed failed, continuing without DB records")
		} else if ct != nil {
			if _, err := c.CreateContentDownloadTask(ct, task, "admin"); err != nil {
				c.logger.Warn().Err(err).Msg("CreateContentDownloadTask failed")
			}
		}
	}
	result.Ok(ctx, gin.H{"id": id})
}

func (c *APIClient) handleCompatChannelsSearchAuthor(ctx *gin.Context) {
	c.handleSearchChannelsContact(ctx)
}

func (c *APIClient) handleCompatChannelsAuthorVideos(ctx *gin.Context) {
	c.handleFetchFeedListOfContact(ctx)
}

func (c *APIClient) handleCompatChannelsMediaProfile(ctx *gin.Context) {
	c.handleFetchFeedProfile(ctx)
}
