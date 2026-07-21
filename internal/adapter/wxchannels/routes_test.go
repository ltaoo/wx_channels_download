package wxchannels

import (
	"testing"

	"github.com/gin-gonic/gin"
)

type testRouteRegistrar struct {
	routes map[string]gin.HandlerFunc
}

func newTestRouteRegistrar() *testRouteRegistrar {
	return &testRouteRegistrar{routes: make(map[string]gin.HandlerFunc)}
}

func (r *testRouteRegistrar) RegisterGET(path string, handler gin.HandlerFunc) {
	r.routes[path] = handler
}

func TestWebsocketRoutesRegisterRoutes(t *testing.T) {
	routes := NewWebsocketRoutes(0, nil, "", false)
	registrar := newTestRouteRegistrar()
	routes.RegisterRoutes(registrar)

	if _, ok := registrar.routes[ChannelsWebsocketPath]; !ok {
		t.Fatalf("websocket path %q was not registered", ChannelsWebsocketPath)
	}
	if registrar.routes[ChannelsWebsocketPath] == nil {
		t.Fatal("websocket handler was not registered")
	}
	if _, ok := registrar.routes["/api/channels/parse_sph"]; !ok {
		t.Fatal("parse_sph path was not registered")
	}
	if registrar.routes["/api/channels/parse_sph"] == nil {
		t.Fatal("parse_sph handler was not registered")
	}
}
