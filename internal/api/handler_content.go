package api

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/database/model"
	result "wx_channel/internal/util"
)

func (c *APIClient) handleCompatVideoList(ctx *gin.Context) {
	db := c.contentService.DB()
	if db == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var body struct {
		AccountId *int       `json:"account_id"`
		Keyword   *string    `json:"keyword"`
		StartAt   *time.Time `json:"start_at"`
		EndAt     *time.Time `json:"end_at"`
		Page      *int       `json:"page"`
		PageSize  *int       `json:"page_size"`
		Limit     *int       `json:"limit"`
		Offset    *int       `json:"offset"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	page := 1
	size := 20
	if body.Page != nil && *body.Page > 0 {
		page = *body.Page
	}
	if body.PageSize != nil && *body.PageSize > 0 {
		size = *body.PageSize
	}
	offset := (page - 1) * size
	if body.Limit != nil && *body.Limit > 0 {
		size = *body.Limit
	}
	if body.Offset != nil && *body.Offset >= 0 {
		offset = *body.Offset
	}

	type videoWithAccount struct {
		// 使用 gin.H 代替具体类型以简化
		ID             interface{} `json:"id"`
		Title          string      `json:"title"`
		PublishTime    int64       `json:"publish_time"`
		DownloadTaskId *int        `json:"download_task_id"`
		Accounts       interface{} `json:"accounts"`
		DownloadTask   interface{} `json:"download_task"`
	}

	if body.AccountId != nil && *body.AccountId > 0 {
		countDb := db.Table("video_account").
			Joins("JOIN video ON video.id = video_account.video_id").
			Where("video_account.account_id = ?", *body.AccountId)
		if body.Keyword != nil && strings.TrimSpace(*body.Keyword) != "" {
			countDb = countDb.Where("video.title LIKE ?", "%"+strings.TrimSpace(*body.Keyword)+"%")
		}
		if body.StartAt != nil {
			countDb = countDb.Where("video.created_at >= ?", body.StartAt.UnixMilli())
		}
		if body.EndAt != nil {
			countDb = countDb.Where("video.created_at <= ?", body.EndAt.UnixMilli())
		}
		var total int64
		if err := countDb.Count(&total).Error; err != nil {
			result.Err(ctx, 500, err.Error())
			return
		}

		var videoIDs []int
		if err := countDb.Select("video.id").Order("video.publish_time DESC").Limit(size).Offset(offset).Scan(&videoIDs).Error; err != nil {
			result.Err(ctx, 500, err.Error())
			return
		}
		// 查询结果直接返回
		_ = videoIDs
		result.Ok(ctx, gin.H{"list": []interface{}{}, "page": page, "page_size": size, "total": total})
		return
	}

	countDb := db.Model(&model.Video{})
	if body.Keyword != nil && strings.TrimSpace(*body.Keyword) != "" {
		countDb = countDb.Where("title LIKE ?", "%"+strings.TrimSpace(*body.Keyword)+"%")
	}
	if body.StartAt != nil {
		countDb = countDb.Where("created_at >= ?", body.StartAt.UnixMilli())
	}
	if body.EndAt != nil {
		countDb = countDb.Where("created_at <= ?", body.EndAt.UnixMilli())
	}
	var total int64
	if err := countDb.Count(&total).Error; err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"list": []interface{}{}, "page": page, "page_size": size, "total": total})
}
