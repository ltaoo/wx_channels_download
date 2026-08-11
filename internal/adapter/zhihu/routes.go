package zhihuadapter

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"wx_channel/internal/util"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/scraper/zhihu"
)

// RouteRegistrar is the narrow HTTP capability required by this adapter.
type RouteRegistrar interface {
	RegisterGET(path string, handler gin.HandlerFunc)
	RegisterPOST(path string, handler gin.HandlerFunc)
}

// Routes owns the zhihu HTTP endpoints.
type Routes struct {
	cookie_reader *cookies.Reader
	logger        *zerolog.Logger
}

// NewRoutes creates routes that share the runtime cookie reader and logger.
func NewRoutes(cookie_reader *cookies.Reader, logger *zerolog.Logger) *Routes {
	return &Routes{cookie_reader: cookie_reader, logger: logger}
}

// RegisterRoutes installs routes owned by this adapter.
func (r *Routes) RegisterRoutes(registrar RouteRegistrar) {
	if r == nil || registrar == nil {
		return
	}
	registrar.RegisterGET("/api/zhihu/fetch", r.HandleFetch)
}

// HandleFetch fetches zhihu page content by URL and returns the built HTML.
// Query parameters:
//
//	url - the zhihu answer/question/article URL to fetch
func (r *Routes) HandleFetch(ctx *gin.Context) {
	rawURL := ctx.Query("url")
	if rawURL == "" {
		util.Err(ctx, 400, "url parameter is required")
		return
	}

	log.Printf("[zhihu] HandleFetch called with url=%s", rawURL)

	client := zhihu.NewClientWithCookieReader(r.cookie_reader, r.logger)
	html, err := client.BuildHTMLFromURL(rawURL)
	if err != nil {
		log.Printf("[zhihu] BuildHTMLFromURL failed: %v", err)
		util.Err(ctx, 400, err.Error())
		return
	}

	log.Printf("[zhihu] BuildHTMLFromURL succeeded, html length=%d", len(html))
	util.Ok(ctx, gin.H{
		"url":  rawURL,
		"html": html,
	})
}

// Stop is a no-op for routes that don't own long-lived resources.
func (r *Routes) Stop() {}
