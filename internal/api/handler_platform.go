package api

import (
	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
	"wx_channel/internal/services"
)

func (c *APIClient) handle_scraper_platform_list(ctx *gin.Context) {
	items := services.ScraperPlatforms()
	result.Ok(ctx, gin.H{"list": items, "total": len(items)})
}

func (c *APIClient) handle_scraper_platform_fetch(ctx *gin.Context) {
	var body services.ScraperPlatformFetchRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	data, err := services.FetchScraperPlatform(ctx.Request.Context(), body)
	if err != nil {
		result.Err(ctx, api_code_scraper_operation_failed, err.Error())
		return
	}
	result.Ok(ctx, data)
}
