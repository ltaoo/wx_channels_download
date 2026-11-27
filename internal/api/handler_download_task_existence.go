package api

import (
	"github.com/gin-gonic/gin"
	result "wx_channel/internal/apiresult"
)

// POST /api/v1/download_task/check_contents
func (c *APIClient) handle_check_download_task_contents(ctx *gin.Context) {
	var body struct {
		ContentIDs []string `json:"content_ids" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "content_ids 必须是字符串数组")
		return
	}
	if c.service_download_task == nil {
		result.Err(ctx, 500, "下载任务服务未初始化")
		return
	}
	checks, err := c.service_download_task.CheckContentIDs(ctx.Request.Context(), body.ContentIDs)
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, checks)
}
