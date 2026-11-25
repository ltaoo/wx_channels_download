package api

import (
	"github.com/gin-gonic/gin"
	result "wx_channel/internal/apiresult"
	"wx_channel/internal/services"
)

func (c *APIClient) handle_update_download_task(ctx *gin.Context) {
	var body services.UpdateDownloadTaskBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if err := c.service_download_task.UpdateTask(body); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, gin.H{})
}
