package api

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/interceptor"
)

func (c *APIClient) SetupRoutes() {
	// 首页
	c.engine.GET("/", c.handleHome)
	// UI 静态资源 - index.js, store/, pages/
	if uiFS, err := UIFS(); err == nil {
		c.engine.GET("/index.js", c.serveUIFile(uiFS))
		c.engine.GET("/store/*filepath", c.serveUIFile(uiFS))
		c.engine.GET("/pages/*filepath", c.serveUIFile(uiFS))
	}
	// favicon
	c.engine.GET("/favicon.ico", c.handleFavicon)
	// 只在本地有的接口
	if !c.cfg.RemoteServerMode {
		// 视频号接口
		c.engine.GET("/ws/channels", c.channels.HandleChannelsWebsocket)
		c.engine.GET("/api/channels/contact/search", c.handleSearchChannelsContact)
		c.engine.GET("/api/channels/contact/feed/list", c.handleFetchFeedListOfContact)
		c.engine.GET("/api/channels/feed/profile", c.handleFetchFeedProfile)
		c.engine.GET("/api/channels/live/replay/list", c.handleFetchLiveReplayList)
		c.engine.GET("/api/channels/interactioned/list", c.handleFetchInteractionedFeedList)
		c.engine.GET("/rss/channels", c.handleFetchFeedListOfContactRSS)
		// 公众号接口
		c.engine.GET("/ws/mp", c.official.HandleWebsocket)
		c.engine.GET("/ws/manage", c.official.HandleManageWebsocket)
		c.engine.POST("/api/mp/refresh_with_frontend", c.official.HandleRefreshOfficialAccountWithFrontend)
		c.engine.GET("/api/mp/ws_pool", c.official.HandleFetchOfficialAccountClients)
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
		c.engine.POST("/api/open_download_dir", c.handleOpenDownloadDir)
	}
	// 下载任务接口
	c.engine.GET("/ws/downloader", c.downloader_ws.HandleDownloaderWebsocket)
	c.engine.GET("/api/task/list", c.handleFetchTaskList)
	c.engine.GET("/api/task/profile", c.handleFetchTaskProfile)
	c.engine.POST("/api/task/create", c.handleCreateFeedDownloadTask)
	c.engine.POST("/api/task/create_batch", c.handleBatchCreateTask)
	c.engine.POST("/api/task/create_channels", c.handleCreateChannelsTask)
	// c.engine.POST("/api/task/create_live", c.handleCreateLiveTask)
	c.engine.POST("/api/task/create2", c.handleCreateDownloadTask)
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
	c.engine.GET("/api/status", c.handleStatus)

	// 静态资源 - lib JS/CSS 文件
	if libFS, err := interceptor.LibFS(); err == nil {
		c.engine.StaticFS("/__wx_channels_assets/lib", http.FS(libFS))
	}
	// 静态资源 - src JS 文件
	if srcFS, err := interceptor.SrcFS(); err == nil {
		c.engine.StaticFS("/__wx_channels_assets/src", http.FS(srcFS))
	}

	// SPA fallback - 非 API/静态资源的 GET 请求返回 index.html，由前端路由处理
	c.engine.NoRoute(func(ctx *gin.Context) {
		// 只对 GET 请求做 SPA fallback
		if ctx.Request.Method != http.MethodGet {
			ctx.Status(http.StatusNotFound)
			return
		}
		p := ctx.Request.URL.Path
		// API、WebSocket、RSS、静态资源等路径不做 fallback
		if strings.HasPrefix(p, "/api/") ||
			strings.HasPrefix(p, "/ws/") ||
			strings.HasPrefix(p, "/rss/") ||
			strings.HasPrefix(p, "/__wx_channels_assets/") {
			ctx.Header("Content-Type", "application/json; charset=utf-8")
			ctx.String(http.StatusNotFound, `{"code":404,"msg":"not found"}`)
			return
		}
		// 带扩展名的请求视为静态资源，返回 404
		if ext := path.Ext(p); ext != "" {
			ctx.Status(http.StatusNotFound)
			return
		}
		// 其余 GET 请求 fallback 到 index.html
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(http.StatusOK, string(files.HTMLHome))
	})
}

func (c *APIClient) handleHome(ctx *gin.Context) {
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, string(files.HTMLHome))
}

func (c *APIClient) serveUIFile(fsys fs.FS) gin.HandlerFunc {
	fileServer := http.FileServer(http.FS(fsys))
	return func(ctx *gin.Context) {
		ctx.Header("Content-Type", "application/javascript; charset=utf-8")
		ctx.Header("Cache-Control", "no-cache")
		fileServer.ServeHTTP(ctx.Writer, ctx.Request)
	}
}

func (c *APIClient) handleFavicon(ctx *gin.Context) {
	ctx.Header("Content-Type", "image/png")
	ctx.Header("Cache-Control", "public, max-age=86400")
	ctx.File("winres/icon.png")
}

func (c *APIClient) handleStatus(ctx *gin.Context) {
	err := c.channels.Validate()
	channels_data := gin.H{
		"available": false,
	}
	data := gin.H{
		"version":  c.cfg.Version,
		"channels": channels_data,
	}
	if err != nil {
		channels_data["available"] = false
	}
	ctx.JSON(200, gin.H{
		"code": 0,
		"msg":  "ok",
		"data": data,
	})
}
