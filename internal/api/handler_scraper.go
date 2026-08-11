package api

import (
	"strings"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/adapter"
	"wx_channel/internal/services"
	result "wx_channel/internal/util"
)

type scraper_cache_clear_body struct {
	URL string `json:"url"`
}

func (c *APIClient) handle_scraper_cache_clear(ctx *gin.Context) {
	var body scraper_cache_clear_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, result.CodeInvalidParams, "请求参数无效")
		return
	}
	raw_url := strings.TrimSpace(body.URL)
	if raw_url == "" {
		result.Err(ctx, result.CodeMissingUrl, "缺少参数：url")
		return
	}
	platform_id, err := services.DetectScraperPlatform(raw_url)
	if err != nil {
		result.Err(ctx, result.CodeInvalidParams, err.Error())
		return
	}
	handler := adapter.Get(platform_id)
	cache_handler, supports_cache := handler.(adapter.FetchCacheAdapter)
	if !supports_cache {
		result.Ok(ctx, gin.H{
			"platform":  platform_id,
			"url":       raw_url,
			"supported": false,
			"removed":   false,
		})
		return
	}
	removed, err := cache_handler.ClearFetchCache(raw_url)
	if err != nil {
		result.Err(ctx, result.CodeFetchMsgFailed, err.Error())
		return
	}
	result.Ok(ctx, gin.H{
		"platform":  platform_id,
		"url":       raw_url,
		"supported": true,
		"removed":   removed,
	})
}
