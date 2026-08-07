package api

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/services"
	result "wx_channel/internal/util"
)

func (c *APIClient) handleCompatVideoList(ctx *gin.Context) {
	c.handleCompatContentListWithType(ctx, "video")
}

func (c *APIClient) handleCompatContentList(ctx *gin.Context) {
	c.handleCompatContentListWithType(ctx, "")
}

func (c *APIClient) handleCompatContentListWithType(ctx *gin.Context, forceContentType string) {
	if c.content_service == nil {
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

	contentType := forceContentType
	if contentType == "" && body.ContentType != nil {
		contentType = strings.TrimSpace(*body.ContentType)
	}
	var accountID, keyword string
	if body.AccountId != nil {
		accountID = *body.AccountId
	}
	if body.Keyword != nil {
		keyword = *body.Keyword
	}

	pageResult, err := c.content_service.ListContents(services.ContentListOptions{
		AccountID:   accountID,
		Type: contentType,
		Keyword:     keyword,
		StartAt:     body.StartAt,
		EndAt:       body.EndAt,
		Page:        page,
		PageSize:    size,
		Offset:      &offset,
	})
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, pageResult)
}

func (c *APIClient) handleContentDetail(ctx *gin.Context) {
	if c.content_service == nil {
		result.Err(ctx, 500, "数据库未初始化")
		return
	}

	contentID := strings.TrimSpace(ctx.Query("id"))
	if contentID == "" {
		result.Err(ctx, 400, "id is required")
		return
	}

	item, err := c.content_service.GetContentDetail(contentID)
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}

	// Enrich resources with local file info.
	type resourceWithFile struct {
		services.ContentResourceRecord
		LocalPath string `json:"local_path"`
		FileType  string `json:"file_type"`
		FileURL   string `json:"file_url"`
		Exists    bool   `json:"exists"`
	}

	resources := make([]resourceWithFile, 0, len(item.Resources))
	for _, r := range item.Resources {
		localPath := filepath.Join(c.cfg.DownloadDir, r.Name)
		exists := false
		if _, err := os.Stat(localPath); err == nil {
			exists = true
		}
		fileType := fileTypeByExt(r.Name)
		resources = append(resources, resourceWithFile{
			ContentResourceRecord: r,
			LocalPath:             localPath,
			FileType:              fileType,
			FileURL:               "/file?path=" + localPath,
			Exists:                exists,
		})
	}

	result.Ok(ctx, gin.H{
		"id":              item.ID,
		"platform_id":     item.PlatformID,
		"type":            item.Type,
		"external_id":     item.ExternalID,
		"external_id2":    item.ExternalID2,
		"external_id3":    item.ExternalID3,
		"title":           item.Title,
		"description":     item.Description,
		"url":             item.URL,
		"source_url":      item.SourceURL,
		"cover_url":       item.CoverURL,
		"cover_width":     item.CoverWidth,
		"cover_height":    item.CoverHeight,
		"publish_time":    item.PublishTime,
		"accounts":        item.Accounts,
		"download_tasks":  item.DownloadTasks,
		"resources":       resources,
	})
}

func fileTypeByExt(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return "image"
	case ".mp4", ".mkv", ".avi", ".mov", ".webm":
		return "video"
	case ".mp3", ".aac", ".ogg", ".wav", ".flac":
		return "audio"
	case ".html", ".htm":
		return "html"
	case ".zip":
		return "zip"
	case ".pdf":
		return "pdf"
	default:
		return "other"
	}
}
