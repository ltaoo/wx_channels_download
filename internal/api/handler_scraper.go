package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/adapter"
	result "wx_channel/internal/apiresult"
	"wx_channel/internal/services"
)

type scraper_cache_clear_body struct {
	URL string `json:"url"`
}

const scraper_cache_content_max_size int64 = 8 * 1024 * 1024

func (c *APIClient) handle_scraper_platform_status(ctx *gin.Context) {
	statuses := c.scraper_platform_status_snapshots()
	available_count := 0
	for _, status := range statuses {
		if status.Available {
			available_count++
		}
	}
	result.Ok(ctx, gin.H{
		"statuses":        statuses,
		"available_count": available_count,
		"total_count":     len(statuses),
	})
}

func (c *APIClient) handle_scraper_cache_content(ctx *gin.Context) {
	job_id := strings.TrimSpace(ctx.Query("id"))
	cache_key := strings.TrimSpace(ctx.Query("key"))
	if job_id == "" || cache_key == "" {
		result.Err(ctx, api_code_invalid_params, "缺少参数：id 或 key")
		return
	}
	cache_entry := c.scraper_job_service.GetCacheEntry(job_id, cache_key)
	if cache_entry == nil {
		result.Err(ctx, api_code_invalid_params, "缓存条目不存在")
		return
	}
	content, err := read_scraper_cache_content(cache_entry.Path)
	if err != nil {
		result.Err(ctx, api_code_scraper_operation_failed, err.Error())
		return
	}
	content_type := strings.TrimSpace(http.DetectContentType(content))
	if extension_type := content_type_for_cache_extension(cache_entry.Path); extension_type != "" {
		content_type = extension_type
	}
	result.Ok(ctx, gin.H{
		"key":          cache_entry.Key,
		"name":         cache_entry.Name,
		"path":         cache_entry.Path,
		"size":         len(content),
		"content_type": content_type,
		"content":      strings.ToValidUTF8(string(content), "�"),
	})
}

func read_scraper_cache_content(cache_path string) ([]byte, error) {
	cache_path = strings.TrimSpace(cache_path)
	if cache_path == "" {
		return nil, fmt.Errorf("缓存文件路径为空")
	}
	cache_file, err := os.Open(cache_path)
	if err != nil {
		return nil, fmt.Errorf("读取缓存文件失败: %w", err)
	}
	defer cache_file.Close()
	file_info, err := cache_file.Stat()
	if err != nil {
		return nil, fmt.Errorf("读取缓存文件信息失败: %w", err)
	}
	if !file_info.Mode().IsRegular() {
		return nil, fmt.Errorf("缓存路径不是文件")
	}
	if file_info.Size() > scraper_cache_content_max_size {
		return nil, fmt.Errorf("缓存文件超过 %d MB，无法预览", scraper_cache_content_max_size/(1024*1024))
	}
	content, err := io.ReadAll(io.LimitReader(cache_file, scraper_cache_content_max_size+1))
	if err != nil {
		return nil, fmt.Errorf("读取缓存内容失败: %w", err)
	}
	if int64(len(content)) > scraper_cache_content_max_size {
		return nil, fmt.Errorf("缓存文件超过 %d MB，无法预览", scraper_cache_content_max_size/(1024*1024))
	}
	return content, nil
}

func content_type_for_cache_extension(cache_path string) string {
	switch strings.ToLower(filepath.Ext(strings.TrimSpace(cache_path))) {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".txt", ".md":
		return "text/plain; charset=utf-8"
	default:
		return ""
	}
}

func (c *APIClient) handle_scraper_cache_clear(ctx *gin.Context) {
	var body scraper_cache_clear_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	raw_url := strings.TrimSpace(body.URL)
	if raw_url == "" {
		result.Err(ctx, api_code_missing_url, "缺少参数：url")
		return
	}
	platform_id, err := services.DetectScraperPlatform(raw_url)
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
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
		result.Err(ctx, api_code_scraper_operation_failed, err.Error())
		return
	}
	result.Ok(ctx, gin.H{
		"platform":  platform_id,
		"url":       raw_url,
		"supported": true,
		"removed":   removed,
	})
}
