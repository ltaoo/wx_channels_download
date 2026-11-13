package api

import (
	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
	"wx_channel/pkg/dm"
)

func (c *APIClient) handle_account_organize_files(ctx *gin.Context) {
	var request dm.OrganizeAccountFilesRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if err := request.Validate(); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	organized, err := c.service_download_task.OrganizeAccountFiles(ctx.Request.Context(), request)
	if err != nil {
		result.ErrWithData(ctx, 500, err.Error(), organized)
		return
	}
	result.Ok(ctx, organized)
}
