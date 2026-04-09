package api

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/database/model"
	result "wx_channel/internal/util"
)

func (c *APIClient) handleCompatVideoList(ctx *gin.Context) {
	if c.db == nil || c.db.DB() == nil {
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
		model.Video
		Accounts     []model.Account     `json:"accounts"`
		DownloadTask *model.DownloadTask `json:"download_task"`
	}

	if body.AccountId != nil && *body.AccountId > 0 {
		countDb := c.db.DB().Table("video_account").
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
		var videos []model.Video
		if len(videoIDs) > 0 {
			_ = c.db.DB().Where("id IN ?", videoIDs).Find(&videos).Error
		}

		items := make([]videoWithAccount, 0, len(videos))
		for _, v := range videos {
			var accounts []model.Account
			_ = c.db.DB().Table("account").
				Joins("INNER JOIN video_account ON video_account.account_id = account.id").
				Where("video_account.video_id = ?", v.Id).
				Find(&accounts).Error

			var downloadTask *model.DownloadTask
			if v.DownloadTaskId != nil && *v.DownloadTaskId > 0 {
				var task model.DownloadTask
				if err := c.db.DB().First(&task, *v.DownloadTaskId).Error; err == nil {
					downloadTask = &task
				}
			}
			items = append(items, videoWithAccount{
				Video:        v,
				Accounts:     accounts,
				DownloadTask: downloadTask,
			})
		}
		result.Ok(ctx, gin.H{"list": items, "page": page, "page_size": size, "total": total})
		return
	}

	countDb := c.db.DB().Model(&model.Video{})
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
	var videos []model.Video
	if err := countDb.Order("publish_time DESC").Limit(size).Offset(offset).Find(&videos).Error; err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	items := make([]videoWithAccount, 0, len(videos))
	for _, v := range videos {
		var accounts []model.Account
		_ = c.db.DB().Table("account").
			Joins("INNER JOIN video_account ON video_account.account_id = account.id").
			Where("video_account.video_id = ?", v.Id).
			Find(&accounts).Error

		var downloadTask *model.DownloadTask
		if v.DownloadTaskId != nil && *v.DownloadTaskId > 0 {
			var task model.DownloadTask
			if err := c.db.DB().First(&task, *v.DownloadTaskId).Error; err == nil {
				downloadTask = &task
			}
		}
		items = append(items, videoWithAccount{
			Video:        v,
			Accounts:     accounts,
			DownloadTask: downloadTask,
		})
	}
	result.Ok(ctx, gin.H{"list": items, "page": page, "page_size": size, "total": total})
}
