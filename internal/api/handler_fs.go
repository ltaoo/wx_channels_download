package api

import (
	"github.com/gin-gonic/gin"

	"wx_channel/internal/services"
	result "wx_channel/internal/util"
)

func (c *APIClient) handleListFiles(ctx *gin.Context) {
	if c.fs_service == nil {
		result.Err(ctx, 500, "文件服务未初始化")
		return
	}
	var options services.ListFilesOptions
	if err := ctx.ShouldBindJSON(&options); err != nil {
		result.Err(ctx, 400, "请求参数无效: "+err.Error())
		return
	}
	files, err := c.fs_service.ListFiles(options)
	if err != nil {
		result.Err(ctx, 500, "读取文件列表失败: "+err.Error())
		return
	}
	result.Ok(ctx, files)
}

func (c *APIClient) handleSearchFiles(ctx *gin.Context) {
	if c.fs_service == nil {
		result.Err(ctx, 500, "文件服务未初始化")
		return
	}
	var options services.SearchFilesOptions
	if err := ctx.ShouldBindJSON(&options); err != nil {
		result.Err(ctx, 400, "请求参数无效: "+err.Error())
		return
	}
	files, err := c.fs_service.SearchFiles(options)
	if err != nil {
		result.Err(ctx, 500, "搜索文件失败: "+err.Error())
		return
	}
	result.Ok(ctx, files)
}
