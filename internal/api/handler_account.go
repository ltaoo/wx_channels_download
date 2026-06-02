package api

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"wx_channel/internal/api/services"
	"wx_channel/internal/database/model"
	result "wx_channel/internal/util"
)

func (c *APIClient) handleCompatInfluencerList(ctx *gin.Context) {
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

	// Use service
	pageResult, err := c.accountService.ListInfluencers(page, size)
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, pageResult)
}

func (c *APIClient) handleCompatInfluencerGet(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		result.Err(ctx, 400, "invalid id")
		return
	}

	// Use service
	influencer, err := c.accountService.GetInfluencer(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err(ctx, 404, err.Error())
			return
		}
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, influencer)
}

type influencerCreateBody struct {
	Name        string `json:"name"`
	AvatarURL   string `json:"avatar_url"`
	Description string `json:"description"`
}

func (c *APIClient) handleCompatInfluencerCreate(ctx *gin.Context) {
	var body influencerCreateBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if body.Name == "" {
		result.Err(ctx, 400, "name is required")
		return
	}

	// Use service
	influencer, err := c.accountService.CreateInfluencer(&services.CreateInfluencerInput{
		Name:        body.Name,
		AvatarURL:   body.AvatarURL,
		Description: body.Description,
	})
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, influencer)
}

type influencerUpdateBody struct {
	Name        string `json:"name"`
	AvatarURL   string `json:"avatar_url"`
	Description string `json:"description"`
}

func (c *APIClient) handleCompatInfluencerUpdate(ctx *gin.Context) {
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

	// Use service
	influencer, err := c.accountService.UpdateInfluencer(id, &services.UpdateInfluencerInput{
		Name:        body.Name,
		AvatarURL:   body.AvatarURL,
		Description: body.Description,
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result.Err(ctx, 404, err.Error())
			return
		}
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, influencer)
}

func (c *APIClient) handleCompatAccountList(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var accounts []model.Account
	if err := c.db.Model(&model.Account{}).Find(&accounts).Error; err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}

	platformIDs := make([]string, 0, len(accounts))
	seenPlatformIDs := map[string]struct{}{}
	for _, acc := range accounts {
		if acc.PlatformId == "" {
			continue
		}
		if _, ok := seenPlatformIDs[acc.PlatformId]; ok {
			continue
		}
		seenPlatformIDs[acc.PlatformId] = struct{}{}
		platformIDs = append(platformIDs, acc.PlatformId)
	}

	platformByID := map[string]model.Platform{}
	if len(platformIDs) > 0 {
		var platforms []model.Platform
		_ = c.db.Where("id IN ?", platformIDs).Find(&platforms).Error
		for _, p := range platforms {
			platformByID[p.Id] = p
		}
	}

	list := make([]gin.H, 0, len(accounts))
	for _, acc := range accounts {
		type vaRow struct {
			VideoId   int    `json:"video_id"`
			AccountId int    `json:"account_id"`
			Role      string `json:"role"`
		}
		var rows []vaRow
		_ = c.db.Table("video_account").
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
			_ = c.db.Where("id IN ?", videoIDs).Find(&videos).Error
			for _, v := range videos {
				videoByID[v.Id] = v
			}
		}

		platform := platformByID[acc.PlatformId]
		platformName := platform.Name
		if platformName == "" {
			platformName = platformNameOf(acc.PlatformId)
		}

		list = append(list, gin.H{
			"id":          acc.Id,
			"platform_id": acc.PlatformId,
			"platform": gin.H{
				"id":        acc.PlatformId,
				"code":      firstNonEmpty(platform.Code, acc.PlatformId),
				"name":      platformName,
				"homepage":  platform.Homepage,
				"logo_url":  platform.LogoURL,
				"entry_url": platform.EntryURL,
			},
			"platform_name": platformName,
			"nickname":      acc.Nickname,
			"avatar_url":    acc.AvatarURL,
			"external_id":   acc.ExternalId,
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func platformNameOf(platformID string) string {
	switch platformID {
	case "wx_channels":
		return "视频号"
	case "douyin":
		return "抖音"
	case "bilibili":
		return "Bilibili"
	case "xiaohongshu", "xhs", "rednote":
		return "小红书"
	case "tiktok":
		return "TikTok"
	case "youtube":
		return "YouTube"
	case "zhihu":
		return "知乎"
	default:
		return platformID
	}
}
