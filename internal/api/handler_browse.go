package api

import (
	"encoding/json"
	"errors"

	"github.com/gin-gonic/gin"

	apitypes "wx_channel/internal/api/types"
	"wx_channel/internal/database/model"
	result "wx_channel/internal/util"
	"wx_channel/pkg/util"
)

var ErrDBNotInitialized = errors.New("数据库未初始化")
var ErrInvalidInput = errors.New("invalid input")

func (c *APIClient) CreateBrowseHistory(browse *model.BrowseHistory) error {
	if c.db == nil || c.db.DB() == nil {
		return ErrDBNotInitialized
	}
	if browse == nil {
		return ErrInvalidInput
	}
	return browse.Upsert(c.db.DB())
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

	now := util.NowMillis()
	var mediaURL, mediaCoverURL, decodeKey, urlToken string
	if len(body.ObjectDesc.Media) > 0 {
		mediaURL = body.ObjectDesc.Media[0].URL
		mediaCoverURL = body.ObjectDesc.Media[0].CoverUrl
		decodeKey = body.ObjectDesc.Media[0].DecodeKey
		urlToken = body.ObjectDesc.Media[0].URLToken
	}
	extraDataBytes, _ := json.Marshal(map[string]interface{}{
		"nonce_id":   body.ObjectNonceId,
		"decodeKey":  decodeKey,
		"urlToken":   urlToken,
		"source_url": body.SourceURL,
	})

	// Use service
	err := c.contentService.DB().Create(&map[string]interface{}{
		"platform_id":         "wx_channels",
		"visited_times":       1,
		"account_external_id": body.Contact.Username,
		"account_username":    body.Contact.Username,
		"account_nickname":    body.Contact.Nickname,
		"account_avatar_url":  body.Contact.HeadUrl,
		"content_type":        "video",
		"content_external_id": body.ID,
		"content_title":       body.ObjectDesc.Description,
		"content_url":         mediaURL,
		"content_source_url":  body.SourceURL,
		"content_cover_url":   mediaCoverURL,
		"extra_data":          string(extraDataBytes),
		"created_at":          now,
		"updated_at":          now,
	}).Error

	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, nil)
}

func (c *APIClient) handleFetchBrowseHistoryList(ctx *gin.Context) {
	var body struct {
		Username *string `json:"username"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}

	query := c.contentService.DB().Where("platform_id = ?", "wx_channels")
	if body.Username != nil {
		query = query.Where("account_username = ?", *body.Username)
	}

	var browseHistories []interface{}
	if err := query.Order("updated_at DESC").Find(&browseHistories).Error; err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{
		"list": browseHistories,
	})
}
