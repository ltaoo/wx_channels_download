package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/frontend"
	"wx_channel/internal/events"
	"wx_channel/internal/services"
	"wx_channel/internal/webassets"
	"wx_channel/pkg/clawreq"
)

type APIClient struct {
	download_task_broadcaster *DownloadTaskBroadcaster
	file_helper               *FileHelperHandler
	cfg                       *APIConfig
	engine                    *gin.Engine
	db                        *gorm.DB
	logger                    *zerolog.Logger
	http_handler              http.Handler
	static_assets             *webassets.Registry
	event_publisher           events.Publisher
	runtime_status_service    *services.RuntimeStatusService

	claw_client *clawreq.Client

	// Services
	account_service             *services.AccountService
	content_service             *services.ContentService
	browse_history_service      *services.BrowseService
	download_task_service       *services.DownloadTaskService
	fs_service                  *services.FSService
	scraper_job_service         *services.ScraperJobService
	application_update_service  *services.ApplicationUpdateService
	application_restart_service *services.ApplicationRestartService
	bridge_service              *services.BridgeService
	certificate_service         *services.CertificateService
	mcp_service                 *services.MCPService
}

func NewAPIClient(
	cfg *APIConfig,
	parent_logger *zerolog.Logger,
	db *gorm.DB,
	static_assets *webassets.Registry,
	download_task_broadcaster *DownloadTaskBroadcaster,
	event_publisher events.Publisher,
	runtime_status_service *services.RuntimeStatusService,
	account_service *services.AccountService,
	content_service *services.ContentService,
	browse_history_service *services.BrowseService,
	download_task_service *services.DownloadTaskService,
	fs_service *services.FSService,
	scraper_job_service *services.ScraperJobService,
	bridge_service *services.BridgeService,
	certificate_service *services.CertificateService,
	mcp_service *services.MCPService,
	application_update_service *services.ApplicationUpdateService,
	application_restart_service *services.ApplicationRestartService,
) *APIClient {
	logger := parent_logger.With().Str("component", "APIClient").Logger()
	engine := gin.New()
	engine.Use(
		gin.LoggerWithConfig(gin.LoggerConfig{
			SkipPaths: []string{"/report"},
		}),
		gin.Recovery(),
	)

	api_client := &APIClient{
		cfg:                         cfg,
		engine:                      engine,
		db:                          db,
		logger:                      &logger,
		static_assets:               static_assets,
		download_task_broadcaster:   download_task_broadcaster,
		event_publisher:             event_publisher,
		runtime_status_service:      runtime_status_service,
		account_service:             account_service,
		content_service:             content_service,
		browse_history_service:      browse_history_service,
		fs_service:                  fs_service,
		scraper_job_service:         scraper_job_service,
		application_update_service:  application_update_service,
		application_restart_service: application_restart_service,
		download_task_service:       download_task_service,
		bridge_service:              bridge_service,
		certificate_service:         certificate_service,
		mcp_service:                 mcp_service,
	}

	// // Set file transfer helper Channels auto-download callback
	// api_client.file_helper.SetFinderAutoDownloadCallback(api_client.auto_create_channels_task)
	// // Set file transfer helper SPH auto-download callback
	// api_client.file_helper.SetSphAutoDownloadCallback(api_client.auto_download_sph_video)

	api_client.SetupRoutes()
	// api_client.http_handler = api_client.build_http_handler()
	return api_client
}

type APIClientWSMessage struct {
	Type  string      `json:"type"`
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}

type ClientWSMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}
type ClientWebsocketRequestBody struct {
	ID   string      `json:"id"`
	Key  string      `json:"key"`
	Body interface{} `json:"data"`
}
type ClientWebsocketResponse struct {
	Id string `json:"id"`
	// Raw response from wx api call
	Data json.RawMessage `json:"data"`
}

func (c *APIClient) service_statuses_map() map[string]string {
	return c.runtime_status_service.ServiceStatuses()
}

func (c *APIClient) Stop() error {
	v1_task_bridge.close_all()
	return nil
}

func (c *APIClient) Engine() *gin.Engine {
	return c.engine
}

// RegisterGET exposes the narrow route-registration capability used by
// platform adapters. It deliberately keeps platform packages from importing
// APIClient or reaching into its Gin engine.
func (c *APIClient) RegisterGET(path string, handler gin.HandlerFunc) {
	c.engine.GET(path, handler)
}

// RegisterPOST exposes POST route registration for platform adapters.
func (c *APIClient) RegisterPOST(path string, handler gin.HandlerFunc) {
	c.engine.POST(path, handler)
}

func (c *APIClient) HTTPHandler() http.Handler {
	return with_cors(c)
}

// DownloadTaskService returns the download task service.
func (c *APIClient) DownloadTaskService() *services.DownloadTaskService {
	return c.download_task_service
}

func (c *APIClient) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if c.http_handler == nil {
		c.http_handler = c.build_http_handler()
	}
	c.http_handler.ServeHTTP(w, r)
}

func (c *APIClient) setup_static_asset_routes() {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		c.engine.Handle(method, "/__assets/public/*filepath", c.handle_public_asset)
		c.engine.Handle(method, "/__assets/src/*filepath", c.handle_src_asset)
		c.engine.Handle(method, "/__assets/inject/*filepath", c.handle_frontend_inject_asset)
		c.engine.Handle(method, "/__assets/platform/*filepath", c.handle_platform_static_asset)
		c.engine.Handle(method, "/__assets/user/*filepath", c.handle_user_static_asset)
	}
}

func (c *APIClient) handle_public_asset(ctx *gin.Context) {
	rel := ctx.Param("filepath")
	data, err := frontend.Assets().ReadPublic(rel)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}
	data = frontend.StaticAssetResponseData(rel, data)
	ctx.Header("Content-Type", frontend.StaticAssetContentType(rel))
	ctx.Header("Cache-Control", frontend.PublicAssetCacheControl)
	if ctx.Request.Method == http.MethodHead {
		ctx.Status(http.StatusOK)
		return
	}
	ctx.Data(http.StatusOK, frontend.StaticAssetContentType(rel), data)
}

func (c *APIClient) handle_frontend_inject_asset(ctx *gin.Context) {
	rel := ctx.Param("filepath")
	data, err := frontend.Assets().ReadInject(rel)
	if err != nil {
		c.static_assets.ServeHTTP(ctx.Writer, ctx.Request)
		return
	}
	etag := frontend.StaticAssetETag(data)
	ctx.Header("Content-Type", frontend.StaticAssetContentType(rel))
	ctx.Header("Cache-Control", frontend.SrcAssetCacheControl)
	ctx.Header("ETag", etag)
	if strings.Contains(ctx.GetHeader("If-None-Match"), etag) {
		ctx.Status(http.StatusNotModified)
		return
	}
	if ctx.Request.Method == http.MethodHead {
		ctx.Status(http.StatusOK)
		return
	}
	ctx.Data(http.StatusOK, frontend.StaticAssetContentType(rel), data)
}

func (c *APIClient) handle_platform_static_asset(ctx *gin.Context) {
	c.static_assets.ServeHTTP(ctx.Writer, ctx.Request)
}

func (c *APIClient) handle_user_static_asset(ctx *gin.Context) {
	if c.logger != nil {
		c.logger.Info().
			Str("method", ctx.Request.Method).
			Str("path", ctx.Request.URL.Path).
			Msg("serving user static asset through API")
	}
	c.static_assets.ServeHTTP(ctx.Writer, ctx.Request)
	if c.logger != nil {
		c.logger.Info().
			Str("method", ctx.Request.Method).
			Str("path", ctx.Request.URL.Path).
			Int("status", ctx.Writer.Status()).
			Msg("served user static asset through API")
	}
}

func (c *APIClient) handle_src_asset(ctx *gin.Context) {
	rel := ctx.Param("filepath")
	data, err := frontend.Assets().ReadSrc(rel)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}
	etag := frontend.StaticAssetETag(data)
	ctx.Header("Content-Type", frontend.StaticAssetContentType(rel))
	ctx.Header("Cache-Control", frontend.SrcAssetCacheControl)
	ctx.Header("ETag", etag)
	if strings.Contains(ctx.GetHeader("If-None-Match"), etag) {
		ctx.Status(http.StatusNotModified)
		return
	}
	if ctx.Request.Method == http.MethodHead {
		ctx.Status(http.StatusOK)
		return
	}
	ctx.Data(http.StatusOK, frontend.StaticAssetContentType(rel), data)
}
