package zhihuadapter

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	result "wx_channel/internal/apiresult"
	"wx_channel/pkg/cache"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/scraper/zhihu"
)

// Routes owns the zhihu HTTP endpoints.
type Routes struct {
	cookie_reader *cookies.Reader
	logger        *zerolog.Logger
	file_cache    *cache.CacheProvider
	browser_relay *zhihu.BrowserRelay
}

// NewRoutes creates routes that share the runtime cookie reader and logger.
func NewRoutes(cookie_reader *cookies.Reader, logger *zerolog.Logger) *Routes {
	return &Routes{
		cookie_reader: cookie_reader,
		logger:        logger,
		browser_relay: zhihu.NewBrowserRelay(logger),
	}
}

func (r *Routes) set_persistent_cache(file_cache *cache.CacheProvider) {
	if r == nil {
		return
	}
	r.file_cache = file_cache
}

// RegisterRoutes installs routes owned by this adapter.
func (r *Routes) RegisterRoutes(registrar adapter.RouteRegistrar) {
	if r == nil || registrar == nil {
		return
	}
	registrar.RegisterGET("/api/zhihu/fetch", r.HandleFetch)
	registrar.RegisterGET(zhihu.BrowserWebSocketPath, r.HandleBrowserWebSocket)
}

// HandleBrowserWebSocket attaches an injected real Zhihu browser tab.
func (r *Routes) HandleBrowserWebSocket(ctx *gin.Context) {
	if r == nil || r.browser_relay == nil {
		ctx.AbortWithStatus(503)
		return
	}
	r.browser_relay.HandleWebSocket(ctx.Writer, ctx.Request)
}

// HandleFetch fetches and parses zhihu page data by URL.
// Query parameters:
//
//	url - the zhihu answer/question/article URL to fetch
func (r *Routes) HandleFetch(ctx *gin.Context) {
	raw_url := ctx.Query("url")
	if raw_url == "" {
		result.Err(ctx, 400, "url parameter is required")
		return
	}

	log.Printf("[zhihu] HandleFetch called with url=%s", raw_url)

	client := zhihu.NewClient(r.cookie_reader, r.logger)
	client.SetPersistentCache(r.file_cache)
	client.SetBrowserFetcher(r.browser_relay)
	page, err := client.Fetch(raw_url)
	if err != nil {
		log.Printf("[zhihu] Fetch failed: %v", err)
		result.Err(ctx, 400, err.Error())
		return
	}

	log.Printf("[zhihu] Fetch succeeded, page type=%T", page)
	result.Ok(ctx, gin.H{
		"url":  raw_url,
		"page": page,
	})
}

// Stop closes browser connections owned by these routes.
func (r *Routes) Stop() {
	if r == nil || r.browser_relay == nil {
		return
	}
	r.browser_relay.Close()
	r.browser_relay = nil
}
