package api

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (c *APIClient) SetupRoutes() {
	c.engine.GET("/api/config", c.handle_config_view)
	c.engine.GET("/api/config/schema", c.handle_config_schema)
	c.engine.PATCH("/api/config", c.handle_config_update)
	// favicon
	c.engine.GET("/favicon.ico", c.handle_favicon)
	c.setupStaticAssetRoutes()
	c.engine.GET("/", c.handle_index)
	c.engine.GET("/download", c.handle_download_page)
	c.engine.GET("/browsehistory", c.handle_browse_history_page)
	c.engine.GET("/content", c.handle_content_page)
	c.engine.GET("/preview", c.handlePreviewPage)
	c.engine.GET("/channels", c.handle_channels_page)
	// File transfer helper endpoints
	c.engine.GET("/filehelper", c.filehelper.HandlePage)
	c.engine.GET("/api/filehelper/qrcode", c.filehelper.HandleGetQRCode)
	c.engine.GET("/api/filehelper/login/wait", c.filehelper.HandleWaitLogin)
	c.engine.GET("/api/filehelper/status", c.filehelper.HandleGetStatus)
	c.engine.GET("/api/filehelper/synccheck", c.filehelper.HandleSyncCheck)
	c.engine.GET("/api/filehelper/sync", c.filehelper.HandleSyncMessages)
	c.engine.GET("/api/filehelper/messages", c.filehelper.HandleGetMessages)
	c.engine.POST("/api/filehelper/send", c.filehelper.HandleSendMessage)
	c.engine.POST("/api/filehelper/logout", c.filehelper.HandleLogout)
	c.engine.POST("/api/filehelper/parse_finder_feed", c.filehelper.HandleParseFinderFeed)
	// File operations
	c.engine.POST("/api/show_file", c.handleShowFile)
	c.engine.POST("/api/open_file", c.handleHighlightFileInFolder)
	c.engine.POST("/api/open_download_dir", c.handleOpenDownloadDir)
	c.engine.POST("/api/open", c.handleOpenURL)
	c.engine.GET("/api/file", c.handleFetchFile)
	c.engine.POST("/api/v1/fs/list", c.handleListFiles)
	c.engine.POST("/api/v1/fs/search", c.handleSearchFiles)
	c.engine.GET("/play", c.handlePlay)
	c.engine.GET("/file", c.handleStreamVideo)

	// c.engine.GET("/migration", c.handle_migration_page)
	// c.engine.POST("/api/v1/migration/load", c.handleMigrationLoad)
	// c.engine.POST("/api/v1/migration/table", c.handleMigrationTable)
	// c.engine.POST("/api/v1/migration/file/list", c.handleMigrationFileList)
	// c.engine.GET("/api/v1/migration/common_dirs", c.handleMigrationCommonDirs)
	// c.engine.POST("/api/task/pipeline/start", c.handleProbePlatformDownloadTask)
	// c.engine.POST("/api/task/probe", c.handleProbePlatformDownloadTask)
	// c.engine.GET("/api/task/pipeline/workflow", c.handleFetchPlatformDownloadWorkflow)
	// c.engine.POST("/api/task/pipeline/resume", c.handleResumePlatformDownloadPipeline)
	c.engine.POST("/api/browse_history/create", c.handleCreateBrowseHistory)
	c.engine.POST("/api/browse_history/list", c.handleFetchBrowseHistoryList)
	c.engine.POST("/api/v1/download_task/prepare", c.handlePrepareDownloadTask)
	c.engine.POST("/api/v1/download_task/prepare_by_url", c.handlePrepareDownloadTaskByURL)
	c.engine.POST("/api/v1/download_task/create", c.handleCreateDownloadTask)
	c.engine.POST("/api/v1/download_task/create_by_url", c.handleCreateDownloadTaskByURL)
	c.engine.POST("/api/v1/download_task/start", c.handleStartDownloadTask)
	c.engine.POST("/api/v1/download_task/pause", c.handlePauseDownloadTask)
	c.engine.POST("/api/v1/download_task/resume", c.handleResumeDownloadTask)
	c.engine.POST("/api/v1/download_task/retry", c.handleRetryDownloadTask)
	c.engine.POST("/api/v1/download_task/delete", c.handleDeleteDownloadTask)
	c.engine.POST("/api/v1/download_task/start_all", c.handleStartAllDownloadTask)
	c.engine.POST("/api/v1/download_task/pause_all", c.handlePauseAllDownloadTask)
	c.engine.POST("/api/v1/download_task/clear", c.handleClearDownloadTask)
	c.engine.POST("/api/v1/download_task/check_files", c.handleCheckDownloadTaskFiles)
	c.engine.GET("/api/v1/download_task/list", c.handleListDownloadTask)
	c.engine.GET("/api/v1/download_task/detail", c.handleDownloadTaskDetail)
	c.engine.GET("/ws/v1/download_task", c.handleDownloadTaskWS)

	c.engine.GET("/api/account/list", c.handleCompatAccountList)

	c.engine.GET("/api/content/list", c.handleCompatContentList)
	c.engine.GET("/api/content/detail", c.handleContentDetail)

	c.engine.GET("/imgproxy", c.handleImgProxy)
	// Official account endpoints (both remote and local)
	// c.engine.GET("/api/mp/list", c.official.HandleFetchList)
	// c.engine.GET("/api/mp/msg/list", c.official.HandleFetchMsgList)
	// c.engine.GET("/api/mp/article/list", c.official.HandleFetchArticleList)
	// c.engine.POST("/api/mp/delete", c.official.HandleDelete)
	// c.engine.POST("/api/mp/refresh", c.official.HandleRefreshEvent)
	// c.engine.POST("/api/mp/download_all", c.handleDownloadAllOfficialAccountMsgs)
	// c.engine.GET("/rss/mp", c.official.HandleOfficialAccountRSS)
	// Other endpoints
	c.engine.POST("/report", c.handle_frontend_report)
	// Admin endpoints
	c.engine.GET("/api/status", c.handle_status)
	c.engine.GET("/api/proxy/status", c.handleProxyStatus)
	c.engine.POST("/api/proxy/config", c.handleProxyConfigUpdate)
	c.engine.POST("/api/proxy/restart", c.handleProxyRestart)
	c.engine.POST("/api/proxy/system/enable", c.handleProxySystemEnable)
	c.engine.POST("/api/proxy/system/disable", c.handleProxySystemDisable)
	c.engine.GET("/api/proxy/certificate/status", c.handleProxyCertificateStatus)
	c.engine.GET("/api/proxy/certificate/pem", c.handleProxyCertificatePEM)
	c.engine.POST("/api/proxy/certificate/generate", c.handleProxyCertificateGenerate)
	c.engine.POST("/api/proxy/certificate/install", c.handleProxyCertificateInstall)
	c.engine.POST("/api/proxy/certificate/replace", c.handleProxyCertificateReplace)
	c.engine.POST("/api/proxy/certificate/uninstall", c.handleProxyCertificateUninstall)
	c.engine.POST("/api/proxy/certificate/uninstall_by_name", c.handleProxyCertificateUninstallByName)
	c.engine.GET("/api/cookie/extract", c.handleCookieExtract)
	c.engine.GET("/api/certificate/root/status", c.handleRootCertificateStatus)
	c.engine.POST("/api/certificate/root/install", c.handleRootCertificateInstall)
	c.engine.POST("/api/certificate/root/uninstall", c.handleRootCertificateUninstall)

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
	api_config := c.current_api_endpoint_config()
	proxy_config := c.current_proxy_config()
	api_addr := fmt.Sprintf("%s:%d", api_config.Hostname, api_config.Port)
	proxy_addr := fmt.Sprintf("%s:%d", proxy_config.Hostname, proxy_config.Port)
	api_status := "stopped"
	if check_port(api_addr) {
		api_status = "running"
	}
	proxy_service := c.proxyServiceStatusData()
	proxy_status, _ := proxy_service["status"].(string)
	if proxy_status == "" {
		proxy_status = "stopped"
	}
	statuses := gin.H{"api": api_status, "interceptor": proxy_status}
	data := gin.H{
		"version":         c.cfg.Version,
		"server_statuses": statuses,
		"api": gin.H{
			"addr":      api_addr,
			"listening": api_status == "running",
			"status":    api_status,
		},
		"proxy": gin.H{
			"addr":      proxy_addr,
			"listening": proxy_service["listening"],
			"status":    proxy_status,
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
