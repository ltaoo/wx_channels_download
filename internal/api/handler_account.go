package api

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"wx_channel/internal/api/services"
	"wx_channel/internal/database/model"
	result "wx_channel/internal/util"
	"wx_channel/pkg/scraper"
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
	var body struct {
		HasContent    *bool  `form:"has_content"`
		ContentFilter string `form:"content_filter"`
	}
	if err := ctx.ShouldBindQuery(&body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}

	contentFilter := strings.ToLower(strings.TrimSpace(body.ContentFilter))
	if body.HasContent != nil {
		if *body.HasContent {
			contentFilter = "with"
		} else {
			contentFilter = "without"
		}
	}
	if contentFilter == "" {
		contentFilter = "with"
	}

	var accounts []model.Account
	query := c.db.Model(&model.Account{})
	switch contentFilter {
	case "with", "has", "true":
		query = query.Where(
			"EXISTS (SELECT 1 FROM content_account WHERE content_account.account_id = account.id)",
		)
	case "without", "none", "false":
		query = query.Where(
			"NOT EXISTS (SELECT 1 FROM content_account WHERE content_account.account_id = account.id)",
		)
	case "all":
	default:
		result.Err(ctx, 400, "invalid content_filter")
		return
	}
	if err := query.Order("created_at DESC, id DESC").Find(&accounts).Error; err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}

	list := make([]gin.H, 0, len(accounts))
	for _, acc := range accounts {
		type caRow struct {
			ContentId int    `json:"content_id"`
			AccountId int    `json:"account_id"`
			Role      string `json:"role"`
		}
		var contentRows []caRow
		_ = c.db.Table("content_account").
			Select("content_account.content_id, content_account.account_id, content_account.role").
			Joins("JOIN content ON content.id = content_account.content_id").
			Where("content_account.account_id = ?", acc.Id).
			Order("COALESCE(content.publish_time, content.updated_at, content.created_at) DESC").
			Limit(24).
			Scan(&contentRows).Error

		contentIDs := make([]int, 0, len(contentRows))
		for _, r := range contentRows {
			contentIDs = append(contentIDs, r.ContentId)
		}
		contentByID := map[int]gin.H{}
		if len(contentIDs) > 0 {
			var contents []model.Content
			_ = c.db.Where("id IN ?", contentIDs).Find(&contents).Error
			for _, content := range contents {
				contentByID[content.Id] = accountContentPayload(content)
			}
		}

		var contentAccountCount int64
		_ = c.db.Table("content_account").Where("account_id = ?", acc.Id).Count(&contentAccountCount).Error

		list = append(list, gin.H{
			"id":       acc.Id,
			"nickname": acc.Nickname,
			"avatar_url":    acc.AvatarURL,
			"external_id":   acc.ExternalId,
			"created_at":    acc.CreatedAt,
			"updated_at":    acc.UpdatedAt,
			"content_count": contentAccountCount,
			"has_content":   contentAccountCount > 0,
			"content_accounts": func() any {
				out := make([]gin.H, 0, len(contentRows))
				for _, r := range contentRows {
					out = append(out, gin.H{
						"content_id": r.ContentId,
						"account_id": r.AccountId,
						"role":       r.Role,
						"content":    contentByID[r.ContentId],
					})
				}
				return out
			}(),
		})
	}
	result.Ok(ctx, gin.H{"list": list})
}

func accountContentPayload(content model.Content) gin.H {
	metadata := platformJSONMap(content.Metadata)
	outputFormat := firstNonEmpty(
		toCompatString(metadata["output_format"]),
		platformOutputFormatFromPath(content.DownloadPath),
		platformOutputFormatFromPath(content.URL),
		platformOutputFormatFromPath(content.ContentURL),
		platformOutputFormatFromPath(content.SourceURL),
		platformOutputFormatFromMimeType(toCompatString(metadata["mime_type"])),
	)
	mimeType := firstNonEmpty(
		toCompatString(metadata["mime_type"]),
		platformMimeTypeFromOutputFormat(outputFormat),
	)
	sourceContentType := firstNonEmpty(
		toCompatString(metadata["source_content_type"]),
		content.ContentType,
	)
	mediaType := platformContentTypeFromOutput(outputFormat, mimeType, content.ContentType)
	displayType := accountContentDisplayType(sourceContentType, outputFormat, mimeType, content.ContentType)
	publishTime := int64(0)
	if content.PublishTime != nil {
		publishTime = *content.PublishTime
	}
	title := firstNonEmpty(content.Title, content.Description, content.ExternalId)
	return gin.H{
		"id":                   content.Id,
		"content_type":         mediaType,
		"media_type":           mediaType,
		"source_content_type":  sourceContentType,
		"output_format":        outputFormat,
		"mime_type":            mimeType,
		"type_label":           displayType,
		"display_type":         displayType,
		"external_id":          content.ExternalId,
		"external_id1":         content.ExternalId,
		"external_id2":         content.ExternalId2,
		"external_id3":         content.ExternalId3,
		"title":                title,
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
		"download_path":        content.DownloadPath,
		"error_msg":            content.ErrorMsg,
	}
}

func accountContentDisplayType(sourceContentType string, outputFormat string, mimeType string, fallback string) string {
	normalize := func(value string) string {
		return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(value, ".")))
	}
	sourceContentType = normalize(sourceContentType)
	outputFormat = normalize(firstNonEmpty(outputFormat, platformOutputFormatFromMimeType(mimeType)))
	fallback = normalize(fallback)
	if outputFormat == "" {
		return firstNonEmpty(sourceContentType, fallback, "file")
	}
	switch sourceContentType {
	case "", "file", "download":
		return outputFormat
	case "video", "audio", "image", "article", "text":
		return outputFormat
	default:
		if sourceContentType == outputFormat {
			return outputFormat
		}
		return sourceContentType + " " + outputFormat
	}
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
	return scraper.DisplayName(platformID)
}
