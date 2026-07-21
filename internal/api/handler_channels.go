package api

import (
	"github.com/gin-gonic/gin"

	result "wx_channel/internal/util"
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
	// next_marker := ctx.Query("next_marker")
	// resp, err := c.channels.FetchChannelsFollowList(next_marker)
	// if err != nil {
	// 	result.Err(ctx, 400, err.Error())
	// 	return
	// }
	// result.Ok(ctx, resp)
	result.Ok(ctx, nil)
}

func (c *APIClient) handleFetchFeedCommentList(ctx *gin.Context) {
	// oid := ctx.Query("oid")
	// nid := ctx.Query("nid")
	// commentID := ctx.Query("comment_id")
	// nextMarker := ctx.Query("next_marker")
	// resp, err := c.channels.FetchChannelsFeedCommentList(oid, nid, commentID, nextMarker)
	// if err != nil {
	// 	result.Err(ctx, 400, err.Error())
	// 	return
	// }
	// result.Ok(ctx, resp)
}

func (c *APIClient) handleFetchFeedShareUrl(ctx *gin.Context) {
	oid := ctx.Query("oid")
	if oid == "" {
		result.Err(ctx, 400, "missing oid")
		return
	}
	result.Err(ctx, 400, "need to process")
	return
	// resp, err := c.channels.FetchChannelsFeedShareUrl(oid)
	// if err != nil {
	// 	result.Err(ctx, 400, err.Error())
	// 	return
	// }
	// result.Ok(ctx, resp)
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
	result.Err(ctx, 400, "need to process")
	// resp, err := c.channels.FetchChannelsSharedFeedProfile(_url)
	// if err != nil {
	// 	result.Err(ctx, 400, err.Error())
	// 	return
	// }
	// result.Ok(ctx, resp)
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
func (c *APIClient) handleCompatChannelsSearchAuthor(ctx *gin.Context) {
	c.handleSearchChannelsContact(ctx)
}

func (c *APIClient) handleCompatChannelsAuthorVideos(ctx *gin.Context) {
	c.handleFetchFeedListOfContact(ctx)
}

func (c *APIClient) handleCompatChannelsMediaProfile(ctx *gin.Context) {
	c.handleFetchFeedProfile(ctx)
}
