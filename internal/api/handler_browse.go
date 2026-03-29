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

func (c *APIClient) CreateBrowseHistory(browse *model.BrowseHistory) error {
	if c.db == nil || c.db.DB() == nil {
		return errors.New("数据库未初始化")
	}
	if browse == nil {
		return errors.New("browse is nil")
	}
	return browse.Upsert(c.db.DB())
}

func (c *APIClient) handleCreateBrowseHistory(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
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
	extraDataBytes, _ := json.Marshal(gin.H{
		"nonce_id":  body.ObjectNonceId,
		"decodeKey": decodeKey,
		"urlToken":  urlToken,
	})

	browse := model.BrowseHistory{
		PlatformId:        "wx_channels",
		VisitedTimes:      1,
		AccountExternalId: body.Contact.Username,
		AccountUsername:   body.Contact.Username,
		AccountNickname:   body.Contact.Nickname,
		AccountAvatarURL:  body.Contact.HeadUrl,
		ContentType:       "video",
		ContentExternalId: body.ID,
		ContentTitle:      body.ObjectDesc.Description,
		ContentURL:        mediaURL,
		ContentSourceURL:  body.SourceURL,
		ContentCoverURL:   mediaCoverURL,
		ExtraData:         string(extraDataBytes),
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if err := c.CreateBrowseHistory(&browse); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, nil)
}

func (c *APIClient) handleFetchBrowseHistoryList(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var body struct {
		Username *string `json:"username"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	query := c.db.DB().Where("platform_id = ?", "wx_channels")
	if body.Username != nil {
		query = query.Where("account_username = ?", *body.Username)
	}
	var browseHistories []model.BrowseHistory
	if err := query.Order("updated_at DESC").Find(&browseHistories).Error; err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{
		"list": browseHistories,
	})
}
