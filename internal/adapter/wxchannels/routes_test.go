package wxchannels

import (
	"testing"

	"github.com/gin-gonic/gin"
)

type testRouteRegistrar struct {
	path    string
	handler gin.HandlerFunc
}

func (r *testRouteRegistrar) RegisterGET(path string, handler gin.HandlerFunc) {
	r.path = path
	r.handler = handler
}

func TestWebsocketRoutesRegisterRoutes(t *testing.T) {
	routes := NewWebsocketRoutes(0, nil)
	registrar := &testRouteRegistrar{}
	routes.RegisterRoutes(registrar)

	if registrar.path != ChannelsWebsocketPath {
		t.Fatalf("path = %q, want %q", registrar.path, ChannelsWebsocketPath)
	}
	if registrar.handler == nil {
		t.Fatal("websocket handler was not registered")
	}
}
