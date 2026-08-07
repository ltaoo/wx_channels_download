package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/util"
	"wx_channel/pkg/configapi"
)

func (c *APIClient) handle_config_view(ctx *gin.Context) {
	if c.config_store == nil {
		result.Err(ctx, http.StatusServiceUnavailable, "配置服务未初始化")
		return
	}
	result.Ok(ctx, c.config_store.View(true))
}

func (c *APIClient) handle_config_schema(ctx *gin.Context) {
	if c.config_store == nil {
		result.Err(ctx, http.StatusServiceUnavailable, "配置服务未初始化")
		return
	}
	view := c.config_store.View(true)
	result.Ok(ctx, gin.H{
		"revision": view.Revision,
		"items":    view.Items,
		"sources":  view.Sources,
	})
}

func (c *APIClient) handle_config_update(ctx *gin.Context) {
	if c.config_store == nil {
		result.Err(ctx, http.StatusServiceUnavailable, "配置服务未初始化")
		return
	}
	var request configapi.UpdateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		result.Err(ctx, http.StatusBadRequest, "不合法的配置修改请求")
		return
	}
	if len(request.Values) == 0 && len(request.Delete) == 0 {
		result.Err(ctx, http.StatusBadRequest, "缺少配置修改项")
		return
	}
	update_result, err := c.config_store.Apply(ctx.Request.Context(), request)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, configapi.ErrRevisionConflict) {
			status = http.StatusConflict
		}
		result.Err(ctx, status, err.Error())
		return
	}
	result.Ok(ctx, gin.H{
		"result": update_result,
		"config": c.config_store.View(true),
	})
}
