package wxmp

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	scraper "wx_channel/pkg/scraper/wxmp"
)

type testRouteRegistrar struct {
	get  map[string]gin.HandlerFunc
	post map[string]gin.HandlerFunc
}

func (r *testRouteRegistrar) RegisterGET(path string, handler gin.HandlerFunc) {
	r.get[path] = handler
}

func (r *testRouteRegistrar) RegisterPOST(path string, handler gin.HandlerFunc) {
	r.post[path] = handler
}

func TestRoutesRegisterRoutes(t *testing.T) {
	logger := zerolog.Nop()
	client := scraper.NewOfficialAccountClient(&scraper.OfficialAccountConfig{}, &logger)
	routes := &Routes{client: client}
	defer routes.Stop()
	registrar := &testRouteRegistrar{get: map[string]gin.HandlerFunc{}, post: map[string]gin.HandlerFunc{}}

	routes.RegisterRoutes(registrar)

	for _, path := range []string{WebsocketPath, ManageWebsocketPath, "/api/mp/list", "/api/mp/msg/list", "/rss/mp"} {
		if registrar.get[path] == nil {
			t.Errorf("GET route %q was not registered", path)
		}
	}
	for _, path := range []string{"/api/mp/refresh_with_frontend", "/api/mp/delete", "/api/mp/refresh"} {
		if registrar.post[path] == nil {
			t.Errorf("POST route %q was not registered", path)
		}
	}
}
