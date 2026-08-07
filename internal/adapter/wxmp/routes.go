package wxmpadapter

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/util"
	"wx_channel/pkg/configapi"
	"wx_channel/pkg/scraper/wxmp"
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
	client *wxmp.OfficialAccountClient
}

func NewRoutes(provider configapi.Provider, runtime configapi.Runtime, logger *zerolog.Logger, db *gorm.DB) (*Routes, error) {
	if provider == nil || logger == nil {
		return &Routes{}, nil
	}
	cfg, err := wxmp.NewOfficialAccountConfig(provider, runtime)
	if err != nil {
		return nil, err
	}
	client := wxmp.NewOfficialAccountClient(cfg, logger)
	client.SetDB(db)
	return &Routes{client: client}, nil
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
	registrar.RegisterGET("/api/mp/postprocess/flows", r.HandleFetchPostprocessFlows)
	registrar.RegisterGET("/rss/mp", r.client.HandleOfficialAccountRSS)
	registrar.RegisterGET("/mp/proxy", r.client.HandleOfficialAccountProxy)
	registrar.RegisterGET("/mp/home", r.client.HandleOfficialAccountManagerHome)
	registrar.RegisterPOST("/api/mp/refresh_with_frontend", r.client.HandleRefreshOfficialAccountWithFrontend)
	registrar.RegisterPOST("/api/mp/delete", r.client.HandleDelete)
	registrar.RegisterPOST("/api/mp/refresh", r.client.HandleRefreshEvent)
}

// HandleFetchPostprocessFlows returns wxmp postprocess flow configs for read-only visualization.
func (r *Routes) HandleFetchPostprocessFlows(ctx *gin.Context) {
	flowID := ctx.Query("flow_id")
	payload, err := GetWXMPPostprocessFlowVisualization(flowID)
	if err != nil {
		util.Err(ctx, 400, err.Error())
		return
	}
	util.Ok(ctx, payload)
}

func (r *Routes) Stop() {
	if r != nil && r.client != nil {
		r.client.Stop()
	}
}
