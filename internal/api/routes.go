package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (c *APIClient) SetupRoutes() {
	// 只在本地有的接口
	if !c.cfg.RemoteServerMode {
		c.engine.POST("/api/show_file", c.handleHighlightFileInFolder)
		c.engine.POST("/api/open_download_dir", c.handleOpenDownloadDir)
		// 视频号接口
		c.engine.GET("/api/channels/contact/search", c.handleSearchChannelsContact)
		c.engine.GET("/api/channels/contact/feed/list", c.handleFetchFeedListOfContact)
		c.engine.GET("/api/channels/feed/profile", c.handleFetchFeedProfile)
		c.engine.GET("/rss/channels", c.handleFetchFeedListOfContactRSS)
		// 公众号接口 本地服务
		c.engine.GET("/ws/mp", c.official.HandleWebsocket)
		c.engine.GET("/ws/manage", c.official.HandleManageWebsocket)
		c.engine.POST("/api/mp/refresh_with_frontend", c.official.HandleRefreshOfficialAccountWithFrontend)
		c.engine.GET("/api/mp/ws_pool", c.official.HandleFetchOfficialAccountClients)
	}
	// 下载任务接口
	c.engine.GET("/ws/channels", c.channels.HandleChannelsWebsocket)
	c.engine.GET("/api/task/list", c.handleFetchTaskList)
	c.engine.GET("/api/task/profile", c.handleFetchTaskProfile)
	c.engine.POST("/api/task/create", c.handleCreateTask)
	c.engine.POST("/api/task/create_batch", c.handleBatchCreateTask)
	c.engine.POST("/api/task/create_channels", c.handleCreateChannelsTask)
	// c.engine.POST("/api/task/create_live", c.handleCreateLiveTask)
	c.engine.POST("/api/task/start", c.handleStartTask)
	c.engine.POST("/api/task/pause", c.handlePauseTask)
	c.engine.POST("/api/task/resume", c.handleResumeTask)
	c.engine.POST("/api/task/delete", c.handleDeleteTask)
	c.engine.POST("/api/task/clear", c.handleClearTasks)
	c.engine.GET("/api/file", c.handleFetchFile)
	// 文件操作
	c.engine.GET("/play", c.handlePlay)
	c.engine.GET("/file", c.handleStreamVideo)
	c.engine.GET("/preview", c.handlePreviewFile)
	// 公众号接口 远端和本地都有的接口
	c.engine.GET("/api/mp/list", c.official.HandleFetchList)
	c.engine.GET("/api/mp/msg/list", c.official.HandleFetchMsgList)
	c.engine.GET("/api/mp/article/list", c.official.HandleFetchArticleList)
	c.engine.POST("/api/mp/delete", c.official.HandleDelete)
	c.engine.POST("/api/mp/refresh", c.official.HandleRefreshEvent)
	c.engine.GET("/rss/mp", c.official.HandleOfficialAccountRSS)
	c.engine.GET("/mp/proxy", c.official.HandleOfficialAccountProxy)
	c.engine.GET("/mp/home", c.official.HandleOfficialAccountManagerHome)
	// 其他
	// c.engine.GET("/api/test", c.handleTest)

	c.engine.NoRoute(func(ctx *gin.Context) {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(http.StatusNotFound, "<!doctype html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>404 Not Found</title><style>body{margin:0;font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Helvetica,Arial,sans-serif;background:#0b0c0f;color:#e6e6e6;display:flex;align-items:center;justify-content:center;height:100vh}.box{max-width:560px;padding:24px 28px;border-radius:12px;background:#14171f;box-shadow:0 8px 24px rgba(0,0,0,.3)}h1{margin:0 0 8px;font-size:24px}p{margin:0;color:#b0b0b0}a{color:#8ab4f8;text-decoration:none}a:hover{text-decoration:underline}</style></head><body><div class=\"box\"><h1>404 未找到页面</h1><p>请求的路径不存在。返回 <a href=\"/\">首页</a></p></div></body></html>")
	})
}
