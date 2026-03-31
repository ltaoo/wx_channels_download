package api

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	result "wx_channel/internal/util"
	utilpkg "wx_channel/pkg/util"
)

type influencerCreateBody struct {
	Name        string `json:"name"`
	AvatarURL   string `json:"avatar_url"`
	Description string `json:"description"`
}

type influencerUpdateBody struct {
	Name        string `json:"name"`
	AvatarURL   string `json:"avatar_url"`
	Description string `json:"description"`
}

func (c *APIClient) handleCompatInfluencerList(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	pageStr := ctx.Query("page")
	sizeStr := ctx.Query("page_size")
	page := 1
	size := 20
	if pageStr != "" {
		if v, err := strconv.Atoi(pageStr); err == nil && v > 0 {
			page = v
		}
	}
	if sizeStr != "" {
		if v, err := strconv.Atoi(sizeStr); err == nil && v > 0 {
			size = v
		}
	}
	var total int64
	_ = c.db.DB().Model(&model.Influencer{}).Count(&total).Error
	var list []model.Influencer
	_ = c.db.DB().Order("id DESC").Limit(size).Offset((page - 1) * size).Find(&list).Error

	type influencerResp struct {
		Id          int    `json:"id"`
		Name        string `json:"name"`
		AvatarURL   string `json:"avatar_url"`
		Sex         int    `json:"sex"`
		Description string `json:"description"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
		DeletedAt   *int64 `json:"deleted_at"`
	}
	out := make([]influencerResp, 0, len(list))
	for _, m := range list {
		out = append(out, influencerResp{
			Id:          m.Id,
			Name:        m.Name,
			AvatarURL:   m.AvatarURL,
			Sex:         m.Sex,
			Description: m.Description,
			CreatedAt:   strconv.FormatInt(m.CreatedAt, 10),
			UpdatedAt:   strconv.FormatInt(m.UpdatedAt, 10),
			DeletedAt:   m.DeletedAt,
		})
	}
	result.Ok(ctx, gin.H{
		"list":      out,
		"page":      page,
		"page_size": size,
		"total":     total,
	})
}

func (c *APIClient) handleCompatInfluencerGet(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		result.Err(ctx, 400, "invalid id")
		return
	}
	var m model.Influencer
	if err := c.db.DB().First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err(ctx, 404, err.Error())
			return
		}
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{
		"id":          m.Id,
		"name":        m.Name,
		"avatar_url":  m.AvatarURL,
		"sex":         m.Sex,
		"description": m.Description,
		"created_at":  strconv.FormatInt(m.CreatedAt, 10),
		"updated_at":  strconv.FormatInt(m.UpdatedAt, 10),
		"deleted_at":  m.DeletedAt,
	})
}

func (c *APIClient) handleCompatInfluencerCreate(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var body influencerCreateBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		result.Err(ctx, 400, "name is required")
		return
	}
	now := utilpkg.NowMillis()
	m := model.Influencer{
		Name:        body.Name,
		AvatarURL:   body.AvatarURL,
		Description: body.Description,
		Timestamps: model.Timestamps{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := c.db.DB().Create(&m).Error; err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{
		"id":          m.Id,
		"name":        m.Name,
		"avatar_url":  m.AvatarURL,
		"sex":         m.Sex,
		"description": m.Description,
		"created_at":  strconv.FormatInt(m.CreatedAt, 10),
		"updated_at":  strconv.FormatInt(m.UpdatedAt, 10),
		"deleted_at":  m.DeletedAt,
	})
}

func (c *APIClient) handleCompatInfluencerUpdate(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		result.Err(ctx, 400, "invalid id")
		return
	}
	var body influencerUpdateBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	var m model.Influencer
	if err := c.db.DB().First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err(ctx, 404, err.Error())
			return
		}
		result.Err(ctx, 500, err.Error())
		return
	}
	updates := map[string]any{
		"updated_at": utilpkg.NowMillis(),
	}
	if strings.TrimSpace(body.Name) != "" {
		updates["name"] = body.Name
	}
	if strings.TrimSpace(body.AvatarURL) != "" {
		updates["avatar_url"] = body.AvatarURL
	}
	if strings.TrimSpace(body.Description) != "" {
		updates["description"] = body.Description
	}
	if len(updates) > 1 {
		if err := c.db.DB().Model(&model.Influencer{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			result.Err(ctx, 500, err.Error())
			return
		}
	}
	_ = c.db.DB().First(&m, id).Error
	result.Ok(ctx, gin.H{
		"id":          m.Id,
		"name":        m.Name,
		"avatar_url":  m.AvatarURL,
		"sex":         m.Sex,
		"description": m.Description,
		"created_at":  strconv.FormatInt(m.CreatedAt, 10),
		"updated_at":  strconv.FormatInt(m.UpdatedAt, 10),
		"deleted_at":  m.DeletedAt,
	})
}

func (c *APIClient) handleCompatAccountList(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var accounts []model.Account
	if err := c.db.DB().Model(&model.Account{}).Find(&accounts).Error; err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}

	list := make([]gin.H, 0, len(accounts))
	for _, acc := range accounts {
		type vaRow struct {
			VideoId   int    `json:"video_id"`
			AccountId int    `json:"account_id"`
			Role      string `json:"role"`
		}
		var rows []vaRow
		_ = c.db.DB().Table("video_account").
			Select("video_account.video_id, video_account.account_id, video_account.role").
			Joins("JOIN video ON video.id = video_account.video_id").
			Where("video_account.account_id = ?", acc.Id).
			Order("video.publish_time DESC").
			Limit(10).
			Scan(&rows).Error

		videoIDs := make([]int, 0, len(rows))
		for _, r := range rows {
			videoIDs = append(videoIDs, r.VideoId)
		}
		videoByID := map[int]model.Video{}
		if len(videoIDs) > 0 {
			var videos []model.Video
			_ = c.db.DB().Where("id IN ?", videoIDs).Find(&videos).Error
			for _, v := range videos {
				videoByID[v.Id] = v
			}
		}

		list = append(list, gin.H{
			"id":          acc.Id,
			"nickname":    acc.Nickname,
			"avatar_url":  acc.AvatarURL,
			"external_id": acc.ExternalId,
			"video_accounts": func() any {
				out := make([]gin.H, 0, len(rows))
				for _, r := range rows {
					out = append(out, gin.H{
						"video_id":   r.VideoId,
						"account_id": r.AccountId,
						"role":       r.Role,
						"video":      videoByID[r.VideoId],
					})
				}
				return out
			}(),
		})
	}
	result.Ok(ctx, gin.H{"list": list})
}
