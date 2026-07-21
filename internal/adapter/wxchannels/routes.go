package wxchannels

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	scraper "wx_channel/pkg/scraper/wxchannels"
)

const ChannelsWebsocketPath = "/ws/channels"

// RouteRegistrar is the narrow HTTP capability required by this adapter. It
// keeps the adapter independent from the API package and its APIClient type.
type RouteRegistrar interface {
	RegisterGET(path string, handler gin.HandlerFunc)
}

// WebsocketRoutes owns the video-channel browser websocket endpoint and its
// scraper client lifecycle.
type WebsocketRoutes struct {
	client *scraper.ChannelsClient
}

func NewWebsocketRoutes(refreshInterval int, db *gorm.DB) *WebsocketRoutes {
	client := scraper.NewChannelsClient(refreshInterval)
	client.SetDB(db)
	return &WebsocketRoutes{client: client}
}

// RegisterRoutes installs routes owned by this adapter.
func (r *WebsocketRoutes) RegisterRoutes(registrar RouteRegistrar) {
	if r == nil || r.client == nil || registrar == nil {
		return
	}
	registrar.RegisterGET(ChannelsWebsocketPath, r.client.HandleChannelsWebsocket)
}

func (r *WebsocketRoutes) Stop() {
	if r != nil && r.client != nil {
		r.client.Stop()
	}
}
