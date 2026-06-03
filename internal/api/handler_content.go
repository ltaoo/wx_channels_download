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

	baseQuery := db.Model(&model.Content{}).Where("content_type = ?", "video")
	if body.AccountId != nil && *body.AccountId > 0 {
		baseQuery = baseQuery.Joins("JOIN content_account ON content_account.content_id = content.id").
			Where("content_account.account_id = ?", *body.AccountId)
	}
	if body.Keyword != nil && strings.TrimSpace(*body.Keyword) != "" {
		baseQuery = baseQuery.Where("content.title LIKE ? OR content.description LIKE ?", "%"+strings.TrimSpace(*body.Keyword)+"%", "%"+strings.TrimSpace(*body.Keyword)+"%")
	}
	if body.StartAt != nil {
		baseQuery = baseQuery.Where("content.created_at >= ?", body.StartAt.UnixMilli())
	}
	if body.EndAt != nil {
		baseQuery = baseQuery.Where("content.created_at <= ?", body.EndAt.UnixMilli())
	}
	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}

	var contents []model.Content
	if err := baseQuery.
		Order("COALESCE(content.publish_time, content.updated_at, content.created_at) DESC").
		Limit(size).
		Offset(offset).
		Find(&contents).Error; err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}

	contentIDs := make([]int, 0, len(contents))
	for _, content := range contents {
		contentIDs = append(contentIDs, content.Id)
	}

	accountsByContentID := map[int][]gin.H{}
	if len(contentIDs) > 0 {
		type row struct {
			ContentId  int
			AccountId  int
			Role       string
			PlatformId string
			ExternalId string
			Username   string
			Nickname   string
			AvatarURL  string
			ProfileURL string
		}
		var rows []row
		_ = db.Table("content_account").
			Select("content_account.content_id, content_account.account_id, content_account.role, account.platform_id, account.external_id, account.username, account.nickname, account.avatar_url, account.profile_url").
			Joins("JOIN account ON account.id = content_account.account_id").
			Where("content_account.content_id IN ?", contentIDs).
			Scan(&rows).Error
		for _, r := range rows {
			accountsByContentID[r.ContentId] = append(accountsByContentID[r.ContentId], gin.H{
				"id":          r.AccountId,
				"platform_id": r.PlatformId,
				"external_id": r.ExternalId,
				"username":    r.Username,
				"nickname":    r.Nickname,
				"avatar_url":  r.AvatarURL,
				"profile_url": r.ProfileURL,
				"role":        r.Role,
			})
		}
	}

	list := make([]gin.H, 0, len(contents))
	for _, content := range contents {
		publishTime := int64(0)
		if content.PublishTime != nil {
			publishTime = *content.PublishTime
		}
		list = append(list, gin.H{
			"id":               content.Id,
			"platform_id":      content.PlatformId,
			"content_type":     content.ContentType,
			"external_id":      content.ExternalId,
			"external_id1":     content.ExternalId,
			"external_id2":     content.ExternalId2,
			"external_id3":     content.ExternalId3,
			"title":            content.Title,
			"description":      content.Description,
			"url":              firstNonEmpty(content.URL, content.ContentURL),
			"content_url":      content.ContentURL,
			"source_url":       content.SourceURL,
			"cover_url":        content.CoverURL,
			"file_size":        content.FileSize,
			"size":             content.Size,
			"duration":         content.Duration,
			"publish_time":     publishTime,
			"download_task_id": content.DownloadTaskId,
			"accounts":         accountsByContentID[content.Id],
		})
	}
	result.Ok(ctx, gin.H{"list": list, "page": page, "page_size": size, "total": total})
}
