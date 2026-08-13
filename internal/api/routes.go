package api

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	// "wx_channel/pkg/scraper/douban"
	// "wx_channel/pkg/scraper/instagram"
	// "wx_channel/pkg/scraper/qidian"
	// "wx_channel/pkg/scraper/weibo"
	// "wx_channel/pkg/scraper/xiaohongshu"
)

func (c *APIClient) SetupRoutes() {
	// favicon
	c.engine.GET("/favicon.ico", c.handle_favicon)
	c.setup_static_asset_routes()
	c.engine.GET("/", c.handle_index)
	// c.engine.GET("/", func(ctx *gin.Context) {
	// 	c.renderFrontendFile(ctx, "index.html")
	// })
	c.engine.GET("/download", c.handle_download_page)
	c.engine.GET("/home", c.handle_home_page)
	c.engine.GET("/browsehistory", c.handle_browse_history_page)
	c.engine.GET("/account", c.handle_account_page)
	c.engine.GET("/content", c.handle_content_page)
	c.engine.GET("/content/detail", c.handle_content_detail_page)
	c.engine.GET("/logs", c.handle_logs_page)
	c.engine.GET("/preview", c.handle_preview_page)
	c.engine.GET("/channels", c.handle_channels_page)
	c.engine.GET("/filehelper", c.file_helper.HandlePage)
	// !!
	c.engine.POST("/api/scraper/fetch", c.handle_scraper_fetch)
	// GET remains available for callers migrating from the former synchronous API.
	c.engine.GET("/api/scraper/fetch", c.handle_scraper_fetch)
	c.engine.GET("/api/scraper/job", c.handle_scraper_job)
	c.engine.POST("/api/scraper/fetch/interrupt", c.handle_scraper_fetch_interrupt)
	c.engine.GET("/api/scraper/cache/content", c.handle_scraper_cache_content)
	c.engine.POST("/api/scraper/cache/clear", c.handle_scraper_cache_clear)
	// File transfer helper endpoints
	c.engine.GET("/api/filehelper/qrcode", c.file_helper.HandleGetQRCode)
	c.engine.GET("/api/filehelper/login/wait", c.file_helper.HandleWaitLogin)
	c.engine.GET("/api/filehelper/status", c.file_helper.HandleGetStatus)
	c.engine.GET("/api/filehelper/synccheck", c.file_helper.HandleSyncCheck)
	c.engine.GET("/api/filehelper/sync", c.file_helper.HandleSyncMessages)
	c.engine.GET("/api/filehelper/messages", c.file_helper.HandleGetMessages)
	c.engine.POST("/api/filehelper/send", c.file_helper.HandleSendMessage)
	c.engine.POST("/api/filehelper/logout", c.file_helper.HandleLogout)
	c.engine.POST("/api/filehelper/parse_finder_feed", c.file_helper.HandleParseFinderFeed)
	// Local file operations
	c.engine.POST("/api/show_file", c.handle_show_file)
	c.engine.GET("/api/file", c.handle_fetch_file)
	c.engine.POST("/api/v1/fs/list", c.handle_list_files)
	// Media proxies
	c.engine.GET("/imgproxy", c.handle_img_proxy)

	// c.engine.GET("/migration", c.handle_migration_page)
	// c.engine.POST("/api/v1/migration/load", c.handle_migration_load)
	// c.engine.POST("/api/v1/migration/table", c.handle_migration_table)
	// c.engine.POST("/api/v1/migration/file/list", c.handle_migration_file_list)
	// c.engine.GET("/api/v1/migration/common_dirs", c.handle_migration_common_dirs)
	// c.engine.POST("/api/task/pipeline/start", c.handle_probe_platform_download_task)
	// c.engine.POST("/api/task/probe", c.handle_probe_platform_download_task)
	// c.engine.GET("/api/task/pipeline/workflow", c.handle_fetch_platform_download_workflow)
	// c.engine.POST("/api/task/pipeline/resume", c.handle_resume_platform_download_pipeline)
	c.engine.GET("/ws/v1/download_task", c.handle_download_task_ws)
	c.engine.GET("/ws/scraper", c.handle_scraper_ws)
	c.engine.POST("/api/browse_history/create", c.handle_create_browse_history)
	c.engine.POST("/api/browse_history/list", c.handle_fetch_browse_history_list)
	c.engine.POST("/api/v1/download_task/prepare", c.handle_prepare_download_task)
	c.engine.POST("/api/v1/download_task/prepare_by_url", c.handle_prepare_download_task_by_url)
	c.engine.POST("/api/v1/download_task/create", c.handle_create_download_task)
	c.engine.POST("/api/v1/download_task/create_by_url", c.handle_create_download_task_by_url)
	c.engine.POST("/api/v1/download_task/start", c.handle_start_download_task)
	c.engine.POST("/api/v1/download_task/pause", c.handle_pause_download_task)
	c.engine.POST("/api/v1/download_task/resume", c.handle_resume_download_task)
	c.engine.POST("/api/v1/download_task/retry", c.handle_retry_download_task)
	c.engine.POST("/api/v1/download_task/delete", c.handle_delete_download_task)
	c.engine.POST("/api/v1/download_task/start_all", c.handle_start_all_download_task)
	c.engine.POST("/api/v1/download_task/pause_all", c.handle_pause_all_download_task)
	c.engine.POST("/api/v1/download_task/clear", c.handle_clear_download_task)
	c.engine.POST("/api/v1/download_task/clear_all", c.handle_clear_all_download_task)
	c.engine.POST("/api/v1/download_task/check_files", c.handle_check_download_task_files)
	c.engine.GET("/api/v1/download_task/list", c.handle_list_download_task)
	c.engine.GET("/api/v1/download_task/detail", c.handle_download_task_detail)
	// c.engine.GET("/api/influencers", c.handle_influencer_list)
	// c.engine.GET("/api/influencers/:id", c.handle_influencer_get)
	// c.engine.POST("/api/influencers", c.handle_influencer_create)
	// c.engine.PUT("/api/influencers/:id", c.handle_influencer_update)
	// c.engine.GET("/influencers", c.handle_influencer_list)
	// c.engine.GET("/influencers/:id", c.handle_influencer_get)
	// c.engine.POST("/influencers", c.handle_influencer_create)
	// c.engine.PUT("/influencers/:id", c.handle_influencer_update)
	c.engine.GET("/api/account/list", c.handle_account_list)
	// c.engine.POST("/api/account/synchronize", c.handle_account_synchronize)
	c.engine.GET("/api/content/list", c.handle_content_list)
	c.engine.GET("/api/content/detail", c.handle_content_detail)
	// Other endpoints
	c.engine.GET("/api/logs", c.handle_logs)
	c.engine.POST("/report", c.handle_frontend_report)
	c.engine.GET("/api/status", c.handle_status)
	c.engine.POST("/api/service/start", c.handle_service_start)
	c.engine.POST("/api/service/stop", c.handle_service_stop)
	c.engine.POST("/api/service/config", c.handle_service_config_update)
	c.engine.GET("/api/proxy/status", c.handle_proxy_status)
	c.engine.POST("/api/proxy/config", c.handle_proxy_config_update)
	c.engine.POST("/api/proxy/restart", c.handle_proxy_restart)
	c.engine.POST("/api/proxy/system/enable", c.handle_proxy_system_enable)
	c.engine.POST("/api/proxy/system/disable", c.handle_proxy_system_disable)
	c.engine.GET("/api/proxy/certificate/status", c.handle_proxy_certificate_status)
	c.engine.GET("/api/proxy/certificate/pem", c.handle_proxy_certificate_pem)
	c.engine.POST("/api/proxy/certificate/generate", c.handle_proxy_certificate_generate)
	c.engine.POST("/api/proxy/certificate/install", c.handle_proxy_certificate_install)
	c.engine.POST("/api/proxy/certificate/replace", c.handle_proxy_certificate_replace)
	c.engine.POST("/api/proxy/certificate/uninstall", c.handle_proxy_certificate_uninstall)
	c.engine.POST("/api/proxy/certificate/uninstall_by_name", c.handle_proxy_certificate_uninstall_by_name)
	c.engine.GET("/api/cookies/extract", c.handle_cookie_extract)
	c.engine.POST("/api/cookies/update", c.handle_cookie_update)
	c.engine.GET("/api/certificate/root/status", c.handle_root_certificate_status)
	c.engine.POST("/api/certificate/root/install", c.handle_root_certificate_install)
	c.engine.POST("/api/certificate/root/uninstall", c.handle_root_certificate_uninstall)

	c.engine.NoRoute(func(ctx *gin.Context) {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(http.StatusNotFound, "<!doctype html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>404 Not Found</title><style>body{margin:0;font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Helvetica,Arial,sans-serif;background:#0b0c0f;color:#e6e6e6;display:flex;align-items:center;justify-content:center;height:100vh}.box{max-width:560px;padding:24px 28px;border-radius:12px;background:#14171f;box-shadow:0 8px 24px rgba(0,0,0,.3)}h1{margin:0 0 8px;font-size:24px}p{margin:0;color:#b0b0b0}a{color:#8ab4f8;text-decoration:none}a:hover{text-decoration:underline}</style></head><body><div class=\"box\"><h1>404 未找到页面</h1><p>请求的路径不存在。返回 <a href=\"/\">首页</a></p></div></body></html>")
	})
}

func (c *APIClient) handle_favicon(ctx *gin.Context) {
	ctx.Header("Content-Type", "image/png")
	ctx.Header("Cache-Control", "public, max-age=86400")
	ctx.File("build/winres/icon.png")
}

func (c *APIClient) handle_status(ctx *gin.Context) {
	api_host := c.cfg.Hostname
	api_port := c.cfg.Port
	proxy_addr := "127.0.0.1:2023"
	if c.cfg.Original != nil {
		if host := c.cfg.Original.GetString("api.hostname"); host != "" {
			api_host = host
		}
		if port := c.cfg.Original.GetInt("api.port"); port > 0 {
			api_port = port
		}
		host := c.cfg.Original.GetString("proxy.hostname")
		port := c.cfg.Original.GetInt("proxy.port")
		if host == "" {
			host = "127.0.0.1"
		}
		if port <= 0 {
			port = 2023
		}
		proxy_addr = fmt.Sprintf("%s:%d", host, port)
	}
	api_addr := fmt.Sprintf("%s:%d", api_host, api_port)
	statuses := gin.H{}
	for name, status := range c.service_statuses_map() {
		statuses[name] = status
	}
	data := gin.H{
		"version":         c.cfg.Version,
		"server_statuses": statuses,
		"api": gin.H{
			"addr":      api_addr,
			"listening": check_port(api_addr),
			"status":    statuses["api"],
		},
		"proxy": gin.H{
			"addr":      proxy_addr,
			"listening": check_port(proxy_addr),
			"status":    statuses["interceptor"],
		},
	}
	ctx.JSON(200, gin.H{
		"code": 0,
		"msg":  "ok",
		"data": data,
	})
}

func check_port(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
