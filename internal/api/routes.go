package api

import "github.com/gin-gonic/gin"

func (c *APIClient) setupRoutes() {
	c.engine.GET("/ws", c.handleWS)
	c.engine.POST("/api/task/create", c.handleCreateTask)
	c.engine.POST("/api/task/create_batch", c.handleBatchCreateTask)
	c.engine.POST("/api/task/start", c.handleStartTask)
	c.engine.POST("/api/task/pause", c.handlePauseTask)
	c.engine.POST("/api/task/resume", c.handleResumeTask)
	c.engine.POST("/api/task/delete", c.handleDeleteTask)
	c.engine.POST("/api/task/clear", c.handleClearTasks)
	c.engine.POST("/api/show_file", c.handleHighlightFileInFolder)
	c.engine.POST("/api/open_download_dir", c.handleOpenDownloadDir)
	c.engine.GET("/api/channels/contact/search", c.handleSearchChannelsContact)
	c.engine.GET("/api/channels/contact/feed/list", c.handleFetchFeedListOfContact)
	c.engine.GET("/api/channels/feed/profile", c.handleFetchFeedProfile)
	c.engine.GET("/api/channels/video/play", c.handlePlay)
	c.engine.GET("/api/test", c.handleTest)

	c.engine.NoRoute(func(ctx *gin.Context) {
		c.handleIndex(ctx)
		// c.decryptor.ServeHTTP(ctx.Writer, ctx.Request)
	})
}
