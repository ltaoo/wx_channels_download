package api

import (
	"net/http"
	"strings"

	"wx_channel/frontend"
)

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
	}
	for _, prefix := range apiPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
