package api

import (
	"errors"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/api/services"
	apitypes "wx_channel/internal/api/types"
	"wx_channel/internal/database/model"
	result "wx_channel/internal/util"
)

var ErrDBNotInitialized = errors.New("数据库未初始化")
var ErrInvalidInput = errors.New("invalid input")

func (c *APIClient) CreateBrowseHistory(browse *model.BrowseHistory) error {
	if c.browseService == nil {
		return ErrDBNotInitialized
	}
	if browse == nil {
		return ErrInvalidInput
	}
	return c.RecordBrowseHistory(browse.ContentExternalId, services.BrowseHistoryInfo{
		PlatformId:        browse.PlatformId,
		AccountExternalId: browse.AccountExternalId,
		AccountUsername:   browse.AccountUsername,
		AccountNickname:   browse.AccountNickname,
		AccountAvatarURL:  browse.AccountAvatarURL,
		ContentType:       browse.ContentType,
		ContentTitle:      browse.ContentTitle,
		ContentURL:        browse.ContentURL,
		ContentSourceURL:  browse.ContentSourceURL,
		ContentCoverURL:   browse.ContentCoverURL,
		ExtraDataJSON:     browse.ExtraData,
	})
}

func (c *APIClient) RecordBrowseHistory(uniqueMark string, info services.BrowseHistoryInfo) error {
	if c.browseService == nil {
		return ErrDBNotInitialized
	}
	return c.browseService.Record(uniqueMark, info)
}

func (c *APIClient) handleCreateBrowseHistory(ctx *gin.Context) {
	var body apitypes.ChannelsObject
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if body.ID == "" {
		result.Err(ctx, 400, "缺少内容 id")
		return
	}

	var mediaURL, mediaCoverURL, decodeKey, urlToken string
	if len(body.ObjectDesc.Media) > 0 {
		mediaURL = body.ObjectDesc.Media[0].URL
		mediaCoverURL = body.ObjectDesc.Media[0].CoverUrl
		decodeKey = body.ObjectDesc.Media[0].DecodeKey
		urlToken = body.ObjectDesc.Media[0].URLToken
	}

	if err := c.RecordBrowseHistory(body.ID, services.BrowseHistoryInfo{
		PlatformId:        "wx_channels",
		AccountExternalId: body.Contact.Username,
		AccountUsername:   body.Contact.Username,
		AccountNickname:   body.Contact.Nickname,
		AccountAvatarURL:  body.Contact.HeadUrl,
		ContentType:       "video",
		ContentTitle:      body.ObjectDesc.Description,
		ContentURL:        mediaURL,
		ContentSourceURL:  body.SourceURL,
		ContentCoverURL:   mediaCoverURL,
		ExtraData: map[string]any{
			"nonce_id":   body.ObjectNonceId,
			"decodeKey":  decodeKey,
			"urlToken":   urlToken,
			"source_url": body.SourceURL,
		},
	}); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, nil)
}

func (c *APIClient) handleFetchBrowseHistoryList(ctx *gin.Context) {
	var body struct {
		Username    *string  `json:"username"`
		PlatformId  string   `json:"platform_id"`
		PlatformIds []string `json:"platform_ids"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}

	platformIds := body.PlatformIds
	if body.PlatformId != "" {
		platformIds = []string{body.PlatformId}
	}
	if len(platformIds) == 0 {
		platformIds = []string{"wx_channels", "wxmp", "zhihu", "xiaohongshu", "bilibili", "youtube", "weibo"}
	}
	browseHistories, err := c.browseService.ListPlatforms(platformIds, body.Username)
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{
		"list": browseHistories,
	})
}
