package api

import (
	"strings"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
	"wx_channel/internal/database/model"
	"wx_channel/internal/services"
)

func (c *APIClient) tag_service_or_error(ctx *gin.Context) (*services.TagService, bool) {
	if c == nil || c.tag_service == nil {
		result.Err(ctx, api_code_invalid_params, "标签服务未初始化")
		return nil, false
	}
	return c.tag_service, true
}

func (c *APIClient) handle_tag_list(ctx *gin.Context) {
	service, ok := c.tag_service_or_error(ctx)
	if !ok {
		return
	}
	keyword := strings.TrimSpace(ctx.Query("keyword"))
	tags, err := service.ListTags(keyword)
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	if tags == nil {
		tags = []model.Tag{}
	}
	result.Ok(ctx, tags)
}

type tag_create_body struct {
	Name string `json:"name"`
}

func (c *APIClient) handle_tag_create(ctx *gin.Context) {
	service, ok := c.tag_service_or_error(ctx)
	if !ok {
		return
	}
	var body tag_create_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	tag, err := service.CreateTag(body.Name)
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, tag)
}

type tag_delete_body struct {
	ID int `json:"id"`
}

func (c *APIClient) handle_tag_delete(ctx *gin.Context) {
	service, ok := c.tag_service_or_error(ctx)
	if !ok {
		return
	}
	var body tag_delete_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	if err := service.DeleteTag(body.ID); err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"id": body.ID, "deleted": true})
}

type tag_rename_body struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (c *APIClient) handle_tag_rename(ctx *gin.Context) {
	service, ok := c.tag_service_or_error(ctx)
	if !ok {
		return
	}
	var body tag_rename_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	if err := service.RenameTag(body.ID, body.Name); err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"id": body.ID, "name": body.Name})
}

type tag_content_set_body struct {
	ContentID string `json:"content_id"`
	TagIDs    []int  `json:"tag_ids"`
}

func (c *APIClient) handle_tag_content_set(ctx *gin.Context) {
	service, ok := c.tag_service_or_error(ctx)
	if !ok {
		return
	}
	var body tag_content_set_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	if err := service.SetContentTags(body.ContentID, body.TagIDs); err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"content_id": body.ContentID, "tag_ids": body.TagIDs})
}

type tag_account_set_body struct {
	AccountID string `json:"account_id"`
	TagIDs    []int  `json:"tag_ids"`
}

func (c *APIClient) handle_tag_account_set(ctx *gin.Context) {
	service, ok := c.tag_service_or_error(ctx)
	if !ok {
		return
	}
	var body tag_account_set_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	if err := service.SetAccountTags(body.AccountID, body.TagIDs); err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"account_id": body.AccountID, "tag_ids": body.TagIDs})
}
