package wxmp

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/config"
	scraper "wx_channel/pkg/scraper/wxmp"
)

const (
	WebsocketPath       = "/ws/mp"
	ManageWebsocketPath = "/ws/manage"
)

// RouteRegistrar is the narrow HTTP capability required by this adapter.
type RouteRegistrar interface {
	RegisterGET(path string, handler gin.HandlerFunc)
	RegisterPOST(path string, handler gin.HandlerFunc)
}

// Routes owns the official-account client lifecycle and endpoints.
type Routes struct {
	client *scraper.OfficialAccountClient
}

func NewRoutes(cfg *config.Config, remoteMode bool, logger *zerolog.Logger, db *gorm.DB) *Routes {
	if cfg == nil || logger == nil {
		return &Routes{}
	}
	client := scraper.NewOfficialAccountClient(scraper.NewOfficialAccountConfig(cfg, remoteMode), logger)
	client.SetDB(db)
	return &Routes{client: client}
}

// RegisterRoutes installs the previously local-only official-account routes.
func (r *Routes) RegisterRoutes(registrar RouteRegistrar) {
	if r == nil || r.client == nil || registrar == nil {
		return
	}
	registrar.RegisterGET(WebsocketPath, r.client.HandleWebsocket)
	registrar.RegisterGET(ManageWebsocketPath, r.client.HandleManageWebsocket)
	registrar.RegisterGET("/api/mp/ws_pool", r.client.HandleFetchOfficialAccountClients)
	registrar.RegisterGET("/api/mp/list", r.client.HandleFetchList)
	registrar.RegisterGET("/api/mp/msg/list", r.client.HandleFetchMsgList)
	registrar.RegisterGET("/api/mp/article/list", r.client.HandleFetchArticleList)
	registrar.RegisterGET("/rss/mp", r.client.HandleOfficialAccountRSS)
	registrar.RegisterGET("/mp/proxy", r.client.HandleOfficialAccountProxy)
	registrar.RegisterGET("/mp/home", r.client.HandleOfficialAccountManagerHome)
	registrar.RegisterPOST("/api/mp/refresh_with_frontend", r.client.HandleRefreshOfficialAccountWithFrontend)
	registrar.RegisterPOST("/api/mp/delete", r.client.HandleDelete)
	registrar.RegisterPOST("/api/mp/refresh", r.client.HandleRefreshEvent)
}

func (r *Routes) Stop() {
	if r != nil && r.client != nil {
		r.client.Stop()
	}
}
