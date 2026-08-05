package api

import (
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
