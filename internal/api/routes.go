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
	c.engine.GET("/favicon.ico", c.handleFavicon)
	c.setupStaticAssetRoutes()
	c.engine.GET("/", c.handleIndex)
	c.engine.GET("/download", c.handleDownloadPage)
	c.engine.GET("/channels", c.handleChannelsPage)
	c.engine.GET("/admin", c.handlePlatformWorkflowWebsocket)
	// 只在本地有的接口
	if !c.cfg.RemoteServerMode {
		// 视频号接口
		c.engine.GET("/api/channels/contact/search", c.handleSearchChannelsContact)
		c.engine.GET("/api/channels/contact/feed/list", c.handleFetchFeedListOfContact)
		c.engine.GET("/api/channels/feed/profile", c.handleFetchFeedProfile)
		c.engine.GET("/api/channels/live/replay/list", c.handleFetchLiveReplayList)
		c.engine.GET("/api/channels/interactioned/list", c.handleFetchInteractionedFeedList)
		c.engine.GET("/api/channels/follow/list", c.handleFetchFollowList)
		c.engine.GET("/api/channels/feed/share_url", c.handleFetchFeedShareUrl)
		c.engine.GET("/api/channels/shared_feed/profile", c.handleFetchSharedFeedProfile)
		c.engine.GET("/api/channels/feed/comment/list", c.handleFetchFeedCommentList)
		c.engine.GET("/api/channels/parse_sph", c.handleParseSph)
		c.engine.GET("/rss/channels", c.handleFetchFeedListOfContactRSS)
		// 公众号接口
		// c.engine.GET("/ws/mp", c.official.HandleWebsocket)
		// c.engine.GET("/ws/manage", c.official.HandleManageWebsocket)
		// c.engine.POST("/api/mp/refresh_with_frontend", c.official.HandleRefreshOfficialAccountWithFrontend)
		// c.engine.GET("/api/mp/ws_pool", c.official.HandleFetchOfficialAccountClients)
		// 文件传输助手接口
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
		// 文件操作
		c.engine.POST("/api/show_file", c.handleHighlightFileInFolder)
		c.engine.POST("/api/open_file", c.handleHighlightFileInFolder)
		c.engine.POST("/api/open_download_dir", c.handleOpenDownloadDir)
		c.engine.POST("/api/open", c.handleOpenURL)
	}
	// c.engine.GET("/ws/channels", c.channels.HandleChannelsWebsocket)
	// 下载任务接口
	c.engine.GET("/ws/downloader", c.downloader_ws.HandleDownloaderWebsocket)
	c.engine.GET("/ws/status", c.status_ws.HandleDownloaderWebsocket)
	c.engine.GET("/ws/admin", c.handlePlatformWorkflowWebsocket)
	c.engine.POST("/api/browse_history/create", c.handleCreateBrowseHistory)
	c.engine.POST("/api/browse_history/list", c.handleFetchBrowseHistoryList)
	c.engine.GET("/api/task/list", c.handleFetchTaskList)
	c.engine.GET("/api/task/profile", c.handleFetchTaskProfile)
	// c.engine.POST("/api/task/pipeline/start", c.handleProbePlatformDownloadTask)
	// c.engine.POST("/api/task/probe", c.handleProbePlatformDownloadTask)
	// c.engine.GET("/api/task/pipeline/workflow", c.handleFetchPlatformDownloadWorkflow)
	// c.engine.POST("/api/task/pipeline/resume", c.handleResumePlatformDownloadPipeline)
	c.engine.POST("/api/task/create", c.handleCreateFeedDownloadTask)
	// c.engine.POST("/api/task/create2", c.handleCreateDownloadTask)
	// c.engine.POST("/api/task/create_batch", c.handleBatchCreateTask)
	// c.engine.POST("/api/task/create_channels", c.handleCreateChannelsTask)
	// c.engine.POST("/api/task/create_live", c.handleCreateLiveTask)
	c.engine.POST("/api/task/start", c.handleStartTask)
	c.engine.POST("/api/task/pause", c.handlePauseTask)
	c.engine.POST("/api/task/resume", c.handleResumeTask)
	c.engine.POST("/api/task/delete", c.handleDeleteTask)
	c.engine.POST("/api/task/clear", c.handleClearTasks)
	c.engine.POST("/api/task/create3", c.handleBatchCreateDownloadTask)
	c.engine.POST("/api/task/start_all", c.handleStartAllTasks)
	c.engine.POST("/api/task/pause_all", c.handlePauseAllTasks)
	c.engine.POST("/api/remote/proxy", c.handleRemoteProxyRequest)
	c.engine.GET("/api/remote/task/list", c.handleFetchRemoteTaskList)
	c.engine.GET("/api/file", c.handleFetchFile)


	// c.engine.GET("/api/download_task/list", c.handleCompatDownloadTaskList)
	// c.engine.POST("/api/download_task/start", c.handleCompatDownloadTaskStart)
	// c.engine.POST("/api/download_task/profile", c.handleCompatDownloadTaskProfile)
	c.engine.POST("/api/v1/download_task/create", c.handleCreateDownloadTaskV1)
	c.engine.POST("/api/v1/download_task/create_by_url", c.handleCreateDownloadTaskByURLV1)
	c.engine.POST("/api/v1/download_task/start", c.handleStartDownloadTaskV1)
	c.engine.POST("/api/v1/download_task/pause", c.handlePauseDownloadTaskV1)
	c.engine.POST("/api/v1/download_task/resume", c.handleResumeDownloadTaskV1)
	c.engine.POST("/api/v1/download_task/delete", c.handleDeleteDownloadTaskV1)
	c.engine.GET("/api/v1/download_task/list", c.handleListDownloadTaskV1)
	c.engine.GET("/ws/v1/download_task", c.handleDownloadTaskV1WS)
	// c.engine.POST("/api/download_task/batch_create", c.handleCompatDownloadTaskBatchCreate)
	// c.engine.POST("/api/download_task/delete", c.handleCompatDownloadTaskDelete)
	// c.engine.POST("/api/download_task/retry", c.handleCompatDownloadTaskRetry)
	// c.engine.POST("/api/download_task/retry_children", c.handleCompatDownloadTaskRetryChildren)
	// c.engine.POST("/api/download_task/pause", c.handleCompatDownloadTaskPause)
	// c.engine.POST("/api/download_task/resume", c.handleCompatDownloadTaskResume)
	// c.engine.POST("/api/download_task/pause_all", c.handleCompatDownloadTaskPauseAll)
	// c.engine.POST("/api/download_task/start_all", c.handleCompatDownloadTaskStartAll)
	// c.engine.POST("/api/download_task/highlight_file", c.handleCompatDownloadTaskHighlightFile)
	// c.engine.GET("/api/download_task/play", c.handleCompatDownloadTaskPlay)

	// c.engine.POST("/browse_history/create", c.handleCreateBrowseHistory)
	// c.engine.POST("/browse_history/list", c.handleFetchBrowseHistoryList)

	// c.engine.GET("/api/influencers", c.handleCompatInfluencerList)
	// c.engine.GET("/api/influencers/:id", c.handleCompatInfluencerGet)
	// c.engine.POST("/api/influencers", c.handleCompatInfluencerCreate)
	// c.engine.PUT("/api/influencers/:id", c.handleCompatInfluencerUpdate)
	// c.engine.GET("/influencers", c.handleCompatInfluencerList)
	// c.engine.GET("/influencers/:id", c.handleCompatInfluencerGet)
	// c.engine.POST("/influencers", c.handleCompatInfluencerCreate)
	// c.engine.PUT("/influencers/:id", c.handleCompatInfluencerUpdate)

	c.engine.GET("/api/account/list", c.handleCompatAccountList)
	// c.engine.POST("/api/account/synchronize", c.handleCompatAccountSynchronize)
	// c.engine.POST("/account/list", c.handleCompatAccountList)
	// c.engine.POST("/account/synchronize", c.handleCompatAccountSynchronize)

	c.engine.GET("/api/content/list", c.handleCompatContentList)
	// c.engine.POST("/content/list", c.handleCompatContentList)
	// c.engine.POST("/api/video/list", c.handleCompatVideoList)
	// c.engine.POST("/video/list", c.handleCompatVideoList)

	// c.engine.GET("/api/channels/search/author", c.handleCompatChannelsSearchAuthor)
	// c.engine.GET("/api/channels/author/videos", c.handleCompatChannelsAuthorVideos)
	// c.engine.GET("/api/channels/media/profile", c.handleCompatChannelsMediaProfile)
	// c.engine.GET("/api/channels/task/status", c.handleCompatChannelsTaskStatus)
	// c.engine.GET("/api/channels/task/start", c.handleCompatChannelsTaskStart)
	// c.engine.GET("/channels/search/author", c.handleCompatChannelsSearchAuthor)
	// c.engine.GET("/channels/author/videos", c.handleCompatChannelsAuthorVideos)
	// c.engine.GET("/channels/media/profile", c.handleCompatChannelsMediaProfile)
	// c.engine.GET("/channels/task/status", c.handleCompatChannelsTaskStatus)
	// c.engine.GET("/channels/task/start", c.handleCompatChannelsTaskStart)
	// 文件操作
	c.engine.GET("/play", c.handlePlay)
	c.engine.GET("/file", c.handleStreamVideo)
	c.engine.GET("/preview", c.handlePreviewFile)
	// 公众号接口 远端和本地都有的接口
	// c.engine.GET("/api/mp/list", c.official.HandleFetchList)
	// c.engine.GET("/api/mp/msg/list", c.official.HandleFetchMsgList)
	// c.engine.GET("/api/mp/article/list", c.official.HandleFetchArticleList)
	// c.engine.POST("/api/mp/delete", c.official.HandleDelete)
	// c.engine.POST("/api/mp/refresh", c.official.HandleRefreshEvent)
	// c.engine.POST("/api/mp/download_all", c.handleDownloadAllOfficialAccountMsgs)
	// c.engine.GET("/rss/mp", c.official.HandleOfficialAccountRSS)
	// c.engine.GET("/mp/proxy", c.official.HandleOfficialAccountProxy)
	// c.engine.GET("/mp/home", c.official.HandleOfficialAccountManagerHome)
	// c.engine.GET("/xiaohongshu/proxy", xiaohongshu.HandleImageProxy)
	// c.engine.GET("/bilibili/proxy", contentbilibili.HandleImageProxy)
	// c.engine.GET("/douban/proxy", douban.HandleImageProxy)
	// c.engine.GET("/instagram/proxy", instagram.HandleImageProxy)
	// c.engine.GET("/qidian/proxy", qidian.HandleImageProxy)
	// c.engine.GET("/weibo/proxy", weibo.HandleImageProxy)
	// 其他
	c.engine.GET("/api/status", c.handleStatus)
	// c.engine.POST("/api/service/start", c.handleServiceStart)
	// c.engine.POST("/api/service/stop", c.handleServiceStop)
	// c.engine.POST("/api/service/config", c.handleServiceConfigUpdate)
	// c.engine.GET("/api/proxy/status", c.handleProxyStatus)
	// c.engine.POST("/api/proxy/config", c.handleProxyConfigUpdate)
	// c.engine.POST("/api/proxy/restart", c.handleProxyRestart)
	// c.engine.POST("/api/proxy/system/enable", c.handleProxySystemEnable)
	// c.engine.POST("/api/proxy/system/disable", c.handleProxySystemDisable)
	// c.engine.GET("/api/proxy/certificate/status", c.handleProxyCertificateStatus)
	// c.engine.GET("/api/proxy/certificate/pem", c.handleProxyCertificatePEM)
	// c.engine.POST("/api/proxy/certificate/generate", c.handleProxyCertificateGenerate)
	// c.engine.POST("/api/proxy/certificate/install", c.handleProxyCertificateInstall)
	// c.engine.POST("/api/proxy/certificate/uninstall", c.handleProxyCertificateUninstall)
	// c.engine.GET("/api/certificate/root/status", c.handleRootCertificateStatus)
	// c.engine.POST("/api/certificate/root/install", c.handleRootCertificateInstall)
	// c.engine.POST("/api/certificate/root/uninstall", c.handleRootCertificateUninstall)
	// c.engine.GET("/api/test", c.handleTest)

	c.engine.NoRoute(func(ctx *gin.Context) {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(http.StatusNotFound, "<!doctype html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>404 Not Found</title><style>body{margin:0;font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Helvetica,Arial,sans-serif;background:#0b0c0f;color:#e6e6e6;display:flex;align-items:center;justify-content:center;height:100vh}.box{max-width:560px;padding:24px 28px;border-radius:12px;background:#14171f;box-shadow:0 8px 24px rgba(0,0,0,.3)}h1{margin:0 0 8px;font-size:24px}p{margin:0;color:#b0b0b0}a{color:#8ab4f8;text-decoration:none}a:hover{text-decoration:underline}</style></head><body><div class=\"box\"><h1>404 未找到页面</h1><p>请求的路径不存在。返回 <a href=\"/\">首页</a></p></div></body></html>")
	})
}

func (c *APIClient) handleFavicon(ctx *gin.Context) {
	ctx.Header("Content-Type", "image/png")
	ctx.Header("Cache-Control", "public, max-age=86400")
	ctx.File("build/winres/icon.png")
}

func (c *APIClient) handleStatus(ctx *gin.Context) {
	channels_data := c.channelsStatusData()["channels"]
	apiHost := c.cfg.Hostname
	apiPort := c.cfg.Port
	proxyAddr := "127.0.0.1:2023"
	if c.cfg.Original != nil {
		if host := c.cfg.Original.GetString("api.hostname"); host != "" {
			apiHost = host
		}
		if port := c.cfg.Original.GetInt("api.port"); port > 0 {
			apiPort = port
		}
		host := c.cfg.Original.GetString("proxy.hostname")
		port := c.cfg.Original.GetInt("proxy.port")
		if host == "" {
			host = "127.0.0.1"
		}
		if port <= 0 {
			port = 2023
		}
		proxyAddr = fmt.Sprintf("%s:%d", host, port)
	}
	apiAddr := fmt.Sprintf("%s:%d", apiHost, apiPort)
	statuses := gin.H{}
	if c.serviceMgr != nil {
		for name, status := range c.serviceMgr.GetAllStatus() {
			statuses[name] = string(status)
		}
	}
	data := gin.H{
		"version":         c.cfg.Version,
		"channels":        channels_data,
		"server_statuses": statuses,
		"api": gin.H{
			"addr":      apiAddr,
			"listening": checkPort(apiAddr),
			"status":    statuses["api"],
		},
		"proxy": gin.H{
			"addr":      proxyAddr,
			"listening": checkPort(proxyAddr),
			"status":    statuses["interceptor"],
		},
	}
	ctx.JSON(200, gin.H{
		"code": 0,
		"msg":  "ok",
		"data": data,
	})
}

func checkPort(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
