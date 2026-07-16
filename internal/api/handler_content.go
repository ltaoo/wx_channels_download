package api

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/database/model"
	result "wx_channel/internal/util"
	"wx_channel/pkg/scraper"
)

func (c *APIClient) handleCompatVideoList(ctx *gin.Context) {
	c.handleCompatContentListWithType(ctx, "video")
}

func (c *APIClient) handleCompatContentList(ctx *gin.Context) {
	c.handleCompatContentListWithType(ctx, "")
}

func (c *APIClient) handleCompatContentListWithType(ctx *gin.Context, forceContentType string) {
	db := c.contentService.DB()
	if db == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}
	var body struct {
		AccountId   *string    `form:"account_id"`
		ContentType *string    `form:"content_type"`
		Keyword     *string    `form:"keyword"`
		StartAt     *time.Time `form:"start_at" time_format:"2006-01-02"`
		EndAt       *time.Time `form:"end_at" time_format:"2006-01-02"`
		Page        *int       `form:"page"`
		PageSize    *int       `form:"page_size"`
		Limit       *int       `form:"limit"`
		Offset      *int       `form:"offset"`
	}
	if err := ctx.ShouldBindQuery(&body); err != nil {
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

	baseQuery := db.Model(&model.Content{})
	contentType := forceContentType
	if contentType == "" && body.ContentType != nil {
		contentType = strings.TrimSpace(*body.ContentType)
	}
	if contentType != "" {
		baseQuery = baseQuery.Where("content_type = ?", contentType)
	}
	if body.AccountId != nil && *body.AccountId != "" {
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

	contentIDs := make([]string, 0, len(contents))
	downloadTaskIDs := make([]int, 0, len(contents))
	for _, content := range contents {
		contentIDs = append(contentIDs, content.Id)
		if content.DownloadTaskId != nil && *content.DownloadTaskId > 0 {
			downloadTaskIDs = append(downloadTaskIDs, *content.DownloadTaskId)
		}
	}

	accountsByContentID := map[string][]gin.H{}
	if len(contentIDs) > 0 {
		type row struct {
			ContentId  string
			AccountId  string
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

	tasksByID := map[int]model.DownloadTask{}
	if len(downloadTaskIDs) > 0 {
		var tasks []model.DownloadTask
		_ = db.Where("id IN ?", downloadTaskIDs).Find(&tasks).Error
		for _, task := range tasks {
			tasksByID[task.Id] = task
		}
	}

	list := make([]gin.H, 0, len(contents))
	for _, content := range contents {
		publishTime := int64(0)
		if content.PublishTime != nil {
			publishTime = *content.PublishTime
		}
		metadata := platformJSONMap(content.Metadata)
		var task model.DownloadTask
		if content.DownloadTaskId != nil {
			task = tasksByID[*content.DownloadTaskId]
		}
		downloadPath := firstNonEmpty(content.DownloadPath, task.Filepath, task.OutputPath)
		mimeType := firstNonEmpty(
			toCompatString(metadata["mime_type"]),
			task.MimeType,
			platformMimeTypeFromOutputFormat(platformOutputFormatFromPath(downloadPath)),
		)
		outputFormat := firstNonEmpty(
			toCompatString(metadata["output_format"]),
			platformOutputFormatFromPath(downloadPath),
			platformOutputFormatFromPath(task.Filename),
			platformOutputFormatFromPath(task.Filepath),
			platformOutputFormatFromPath(task.OutputPath),
			platformOutputFormatFromMimeType(mimeType),
		)
		list = append(list, gin.H{
			"id":                   content.Id,
			"platform_id":          content.PlatformId,
			"platform_name":        platformNameOf(content.PlatformId),
			"platform_favicon_url": scraper.FaviconDataURL(content.PlatformId),
			"content_type":         content.ContentType,
			"source_content_type":  toCompatString(metadata["source_content_type"]),
			"output_format":        outputFormat,
			"mime_type":            mimeType,
			"external_id":          content.ExternalId,
			"external_id1":         content.ExternalId,
			"external_id2":         content.ExternalId2,
			"external_id3":         content.ExternalId3,
			"title":                content.Title,
			"description":          content.Description,
			"url":                  firstNonEmpty(content.URL, content.ContentURL),
			"content_url":          content.ContentURL,
			"source_url":           content.SourceURL,
			"cover_url":            content.CoverURL,
			"file_size":            content.FileSize,
			"size":                 content.Size,
			"duration":             content.Duration,
			"publish_time":         publishTime,
			"download_task_id":     content.DownloadTaskId,
			"download_status":      content.DownloadStatus,
			"download_path":        downloadPath,
			"error_msg":            content.ErrorMsg,
			"accounts":             accountsByContentID[content.Id],
		})
	}
	result.Ok(ctx, gin.H{"list": list, "page": page, "page_size": size, "total": total})
}
