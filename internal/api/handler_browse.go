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
	normalizeMillis := func(ts int64) int64 {
		if ts <= 0 {
			return ts
		}
		if ts < 1_000_000_000_000 {
			return ts * 1000
		}
		return ts
	}
	var body struct {
		Username *string `json:"username"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	query := c.db.DB().Model(&model.BrowseHistory{}).Where("browse_history.platform_id = ?", "wx_channels")
	if body.Username != nil {
		query = query.Joins("JOIN account ON account.id = browse_history.account_id").
			Where("account.username = ?", *body.Username)
	}
	var browseHistories []model.BrowseHistory
	if err := query.Order("browse_history.updated_at DESC").Find(&browseHistories).Error; err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	type browseHistoryItem struct {
		Id                int    `json:"id"`
		PlatformId        string `json:"platform_id"`
		VisitedTimes      int64  `json:"visited_times"`
		AccountId         *int   `json:"account_id"`
		InfluencerId      *int   `json:"influencer_id"`
		ContentId         *int   `json:"content_id"`
		ContentType       string `json:"content_type"`
		ContentExternalId string `json:"content_external_id"`
		ContentTitle      string `json:"content_title"`
		ContentURL        string `json:"content_url"`
		ContentSourceURL  string `json:"content_source_url"`
		ContentCoverURL   string `json:"content_cover_url"`
		ExtraData         string `json:"extra_data"`
		CreatedAt         int64  `json:"created_at"`
		UpdatedAt         int64  `json:"updated_at"`
		AccountPlatform   string `json:"account_platform"`
	}
	list := make([]browseHistoryItem, 0, len(browseHistories))
	for _, b := range browseHistories {
		list = append(list, browseHistoryItem{
			Id:                b.Id,
			PlatformId:        b.PlatformId,
			VisitedTimes:      b.VisitedTimes,
			AccountId:         b.AccountId,
			InfluencerId:      b.InfluencerId,
			ContentId:         b.ContentId,
			ContentType:       b.ContentType,
			ContentExternalId: b.ContentExternalId,
			ContentTitle:      b.ContentTitle,
			ContentURL:        b.ContentURL,
			ContentSourceURL:  b.ContentSourceURL,
			ContentCoverURL:   b.ContentCoverURL,
			ExtraData:         b.ExtraData,
			CreatedAt:         normalizeMillis(b.CreatedAt),
			UpdatedAt:         normalizeMillis(b.UpdatedAt),
			AccountPlatform:   b.PlatformId,
		})
	}
	result.Ok(ctx, gin.H{"list": list})
}
