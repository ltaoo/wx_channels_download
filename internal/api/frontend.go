package api

import (
	"log"
	"net/http"
	"strings"
	"encoding/json"

	"github.com/gin-gonic/gin"

	"wx_channel/frontend"
)

// func (c *APIClient) handleIndex(ctx *gin.Context) {
// 	ctx.Header("Content-Type", "text/html; charset=utf-8")
// 	ctx.String(http.StatusOK, string(files.HTMLHome))
// }

func (c *APIClient) handleIndex(ctx *gin.Context) {
	c.handleDownloadPage(ctx)
}

func (c *APIClient) handleDownloadPage(ctx *gin.Context) {
	c.renderInjectedRootHTML(ctx, "index.html")
}

func (c *APIClient) handleChannelsPage(ctx *gin.Context) {
	log.Println("[ROUTE] handleChannelsPage called, rendering channels.html")
	c.renderInjectedRootHTML(ctx, "channels.html")
}

func (c *APIClient) renderInjectedRootHTML(ctx *gin.Context, name string) {
	data, err := frontend.Assets.ReadRoot(name)
	if err != nil {
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}
	cfgByte, _ := json.Marshal(c.cfg)
	html := string(data)
	html = strings.ReplaceAll(html, "__WX_DOWNLOAD_CONFIG_JSON__", string(cfgByte))
	html = strings.ReplaceAll(html, "__WX_DOWNLOAD_VERSION__", "local")

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, html)
}

func (c *APIClient) buildHTTPHandler() http.Handler {
	frontendHandler := frontend.NewServer(c.cfg.Mode)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldServeByAPI(r.URL.Path) {
			c.engine.ServeHTTP(w, r)
			return
		}
		frontendHandler.ServeHTTP(w, r)
	})
}

func shouldServeByAPI(path string) bool {
	if path == "/favicon.ico" ||
		path == "/filehelper" ||
		path == "/play" ||
		path == "/file" ||
		path == "/preview" ||
		path == "/channels" ||
		path == "/admin" ||
		path == "/influencers" {
		return true
	}

	apiPrefixes := []string{
		"/api/",
		"/ws/",
		"/rss/",
		"/mp/",
		"/browse_history/",
		"/influencers/",
		"/account/",
		"/video/",
		"/channels/",
		"/xiaohongshu/",
		"/bilibili/",
		"/douban/",
		"/instagram/",
		"/weibo/",
		"/__wx_channels_assets/",
	}
	for _, prefix := range apiPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
