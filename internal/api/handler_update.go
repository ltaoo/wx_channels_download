package api

import (
	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
)

func (c *APIClient) handle_update_check(ctx *gin.Context) {
	service := c.application_update_service
	if service == nil {
		result.Err(ctx, 503, "更新服务未初始化")
		return
	}
	status, err := service.Check(ctx.Request.Context())
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, status)
}

func (c *APIClient) handle_update_status(ctx *gin.Context) {
	service := c.application_update_service
	if service == nil {
		result.Err(ctx, 503, "更新服务未初始化")
		return
	}
	result.Ok(ctx, service.Status())
}

func (c *APIClient) handle_update_download(ctx *gin.Context) {
	service := c.application_update_service
	if service == nil {
		result.Err(ctx, 503, "更新服务未初始化")
		return
	}
	status, err := service.Start()
	if err != nil {
		result.Err(ctx, 409, err.Error())
		return
	}
	result.Ok(ctx, status)
}

func (c *APIClient) handle_update_restart(ctx *gin.Context) {
	service := c.application_update_service
	if service == nil {
		result.Err(ctx, 503, "更新服务未初始化")
		return
	}
	status, err := service.Restart()
	if err != nil {
		result.Err(ctx, 409, err.Error())
		return
	}
	result.Ok(ctx, status)
}
