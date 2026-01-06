package api

import "github.com/gin-gonic/gin"

func (c *APIClient) setupRoutes() {
	// 下载任务接口
	c.engine.POST("/api/task/create", c.handleCreateTask)
	c.engine.POST("/api/task/create_batch", c.handleBatchCreateTask)
	c.engine.POST("/api/task/start", c.handleStartTask)
	c.engine.POST("/api/task/create_live", c.handleCreateLiveTask)
	c.engine.POST("/api/task/pause", c.handlePauseTask)
	c.engine.POST("/api/task/resume", c.handleResumeTask)
	c.engine.POST("/api/task/delete", c.handleDeleteTask)
	c.engine.POST("/api/task/clear", c.handleClearTasks)
	c.engine.POST("/api/show_file", c.handleHighlightFileInFolder)
	c.engine.POST("/api/open_download_dir", c.handleOpenDownloadDir)
	// 视频号接口
	c.engine.GET("/ws/channels", c.handleChannelsWebsocket)
	c.engine.GET("/api/channels/contact/search", c.handleSearchChannelsContact)
	c.engine.GET("/api/channels/contact/feed/list", c.handleFetchFeedListOfContact)
	c.engine.GET("/api/channels/feed/profile", c.handleFetchFeedProfile)
	c.engine.GET("/rss/channels", c.handleFetchFeedListOfContactRSS)
	c.engine.GET("/play", c.handlePlay)
	// 公众号接口
	c.engine.GET("/ws/mp", c.official.HandleWebsocket)
	c.engine.GET("/api/official_account/list", c.official.HandleFetchOfficialAccountList)
	c.engine.GET("/api/official_account/msg/list", c.official.HandleFetchOfficialAccountMsgList)
	c.engine.POST("/api/official_account/refresh", c.official.HandleRefreshOfficialAccount)
	c.engine.GET("/rss/mp", c.official.HandleFetchMsgListOfOfficialAccountRSS)
	c.engine.GET("/official_account/proxy", c.official.HandleOfficialAccountProxy)
	// 其他
	c.engine.GET("/api/test", c.handleTest)

	c.engine.NoRoute(func(ctx *gin.Context) {
		c.handleIndex(ctx)
		// c.decryptor.ServeHTTP(ctx.Writer, ctx.Request)
	})
}
