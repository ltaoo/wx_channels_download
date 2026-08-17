package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/frontend"
	"wx_channel/internal/events"
	"wx_channel/internal/services"
	"wx_channel/internal/webassets"
	"wx_channel/pkg/clawreq"
	"wx_channel/pkg/hermes"
)

type APIClient struct {
	downloader    *hermes.HermesEngine
	broadcaster   *task_broadcaster
	file_helper   *FileHelperHandler
	cfg           *APIConfig
	engine        *gin.Engine
	db            *gorm.DB
	logger        *zerolog.Logger
	http_handler  http.Handler
	static_assets *webassets.Registry
	mcp_handler   http.Handler
	mcp_enabled   atomic.Bool

	bus                 *events.Bus
	proxy_status_mu     sync.RWMutex
	cached_proxy_status string
	cached_proxy_addr   string
	svc_status_mu       sync.RWMutex
	svc_statuses        map[string]events.ServiceStatusChanged
	platform_status_mu  sync.RWMutex
	platform_statuses   map[string]events.PlatformStatusChanged

	claw_client *clawreq.Client

	// Services
	account_service        *services.AccountService
	content_service        *services.ContentService
	browse_history_service *services.BrowseService
	download_task_service  *services.DownloadTaskService
	fs_service             *services.FSService
	scraper_job_service    *services.ScraperJobService
}

func NewAPIClient(
	cfg *APIConfig,
	parent_logger *zerolog.Logger,
	db *gorm.DB,
	static_assets *webassets.Registry,
	downloader *hermes.HermesEngine,
	hook_manager *hermes.HookManager,
) *APIClient {
	logger := parent_logger.With().Logger()
	engine := gin.New()
	engine.Use(
		gin.LoggerWithConfig(gin.LoggerConfig{
			SkipPaths: []string{"/report"},
		}),
		gin.Recovery(),
	)

	// Initialize services
	account_service := services.NewAccountService(db)
	content_service := services.NewContentService(db)
	browse_history_service := services.NewBrowseService(db, logger)
	fs_service := services.NewFSService()
	if static_assets == nil {
		static_assets = webassets.NewRegistry()
	}

	api_client := &APIClient{
		cfg:                    cfg,
		engine:                 engine,
		db:                     db,
		logger:                 &logger,
		static_assets:          static_assets,
		account_service:        account_service,
		content_service:        content_service,
		browse_history_service: browse_history_service,
		fs_service:             fs_service,
		downloader:             downloader,
	}
	api_client.mcp_handler = api_client.new_mcp_handler()
	api_client.scraper_job_service = services.NewScraperJobService(
		nil,
		scraper_ws_hub.broadcast_job_event,
		&logger,
	)

	api_client.download_task_service = services.NewDownloadTaskService(
		db, &logger, downloader, hook_manager,
		cfg.WorkDir, cfg.DownloadDir,
	)

	api_client.broadcaster = new_task_broadcaster()
	api_client.downloader.OnEvent(func(event hermes.EventType, data hermes.EventData) {
		task_id, progress, finished_resources, ok := download_task_event_data(event, data)
		if !ok {
			return
		}
		logger.Info().Int("task_id", task_id).Str("event", string(event)).Msg("Hermes task event")
		api_client.broadcaster.notify(api_client, task_id, event, progress, finished_resources)
		if event == hermes.EventFinished && api_client.bus != nil {
			go func() {
				api_client.bus.Publish(events.DownloadTaskFinished{TaskID: task_id})
			}()
		}
	})

	// // Set file transfer helper Channels auto-download callback
	// api_client.file_helper.SetFinderAutoDownloadCallback(api_client.auto_create_channels_task)
	// // Set file transfer helper SPH auto-download callback
	// api_client.file_helper.SetSphAutoDownloadCallback(api_client.auto_download_sph_video)

	api_client.SetupRoutes()
	// api_client.http_handler = api_client.build_http_handler()
	return api_client
}

func download_task_event_data(event hermes.EventType, data hermes.EventData) (int, *hermes.TaskProgress, []hermes.TaskFinishedResource, bool) {
	switch event {
	// Task creation.
	case hermes.EventCreated:
		event_data, ok := data.(hermes.TaskCreatedEventData)
		return event_data.TaskID, nil, nil, ok

	// Task completion, including unsuccessful terminal states.
	case hermes.EventFinished:
		event_data, ok := data.(hermes.TaskFinishedEventData)
		return event_data.TaskID, nil, event_data.Resources, ok
	case hermes.EventFailed:
		event_data, ok := data.(hermes.TaskFailedEventData)
		return event_data.TaskID, nil, nil, ok
	case hermes.EventDeleted:
		event_data, ok := data.(hermes.TaskDeletedEventData)
		return event_data.TaskID, nil, nil, ok

	// Task progress includes start/resume, pause, and byte progress updates.
	case hermes.EventStarted:
		event_data, ok := data.(hermes.TaskStartedEventData)
		return event_data.TaskID, nil, nil, ok
	case hermes.EventPreparing:
		event_data, ok := data.(hermes.TaskPreparingEventData)
		return event_data.TaskID, nil, nil, ok
	case hermes.EventPaused:
		event_data, ok := data.(hermes.TaskPausedEventData)
		return event_data.TaskID, nil, nil, ok
	case hermes.EventProgress:
		event_data, ok := data.(hermes.TaskProgressEventData)
		return event_data.TaskID, event_data.Progress, nil, ok && event_data.Progress != nil
	default:
		return 0, nil, nil, false
	}
}

func (c *APIClient) SubscribeEvents(bus *events.Bus) {
	c.bus = bus
	bus.Subscribe(events.TypeProxyStatusChanged, func(e events.Event) {
		ev, ok := e.(events.ProxyStatusChanged)
		if !ok {
			return
		}
		c.proxy_status_mu.Lock()
		c.cached_proxy_status = ev.Status
		c.cached_proxy_addr = ev.Addr
		c.proxy_status_mu.Unlock()
	})
	bus.Subscribe(events.TypeServiceStatusChanged, func(e events.Event) {
		ev, ok := e.(events.ServiceStatusChanged)
		if !ok {
			return
		}
		c.svc_status_mu.Lock()
		if c.svc_statuses == nil {
			c.svc_statuses = make(map[string]events.ServiceStatusChanged)
		}
		c.svc_statuses[ev.Name] = ev
		c.svc_status_mu.Unlock()
	})
	bus.Subscribe(events.TypeScraperFetchProgress, func(e events.Event) {
		progress, ok := e.(events.ScraperFetchProgress)
		if !ok {
			return
		}
		c.scraper_job_service.UpdateProgress(progress)
	})
	bus.Subscribe(events.TypePlatformStatusChanged, func(e events.Event) {
		status, ok := e.(events.PlatformStatusChanged)
		if !ok {
			return
		}
		var valid bool
		status, valid = normalize_platform_status(status)
		if !valid {
			return
		}

		c.platform_status_mu.Lock()
		if c.platform_statuses == nil {
			c.platform_statuses = make(map[string]events.PlatformStatusChanged)
		}
		previous_status, exists := c.platform_statuses[status.Key]
		if exists &&
			previous_status.Available == status.Available &&
			previous_status.Status == status.Status &&
			previous_status.Name == status.Name &&
			previous_status.Reason == status.Reason {
			c.platform_status_mu.Unlock()
			return
		}
		c.platform_statuses[status.Key] = status
		c.platform_status_mu.Unlock()
		scraper_ws_hub.broadcast_platform_status(&status)
	})
}

func normalize_platform_status(status events.PlatformStatusChanged) (events.PlatformStatusChanged, bool) {
	status.Platform = strings.TrimSpace(status.Platform)
	status.Key = strings.TrimSpace(status.Key)
	status.Name = strings.TrimSpace(status.Name)
	status.Status = strings.TrimSpace(status.Status)
	status.Reason = strings.TrimSpace(status.Reason)
	if status.Platform == "" {
		return status, false
	}
	if status.Key == "" {
		status.Key = status.Platform
	}
	if status.Status == "" {
		if status.Available {
			status.Status = "available"
		} else {
			status.Status = "unavailable"
		}
	}
	switch status.Status {
	case "available":
		status.Available = true
	case "checking", "unavailable":
		status.Available = false
	}
	return status, true
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
	c.svc_status_mu.RLock()
	defer c.svc_status_mu.RUnlock()
	result := make(map[string]string, len(c.svc_statuses))
	for name, s := range c.svc_statuses {
		result[name] = s.Status
	}
	return result
}

func (c *APIClient) Start() error {
	return nil
}

func (c *APIClient) Stop() error {
	if c.scraper_job_service != nil {
		c.scraper_job_service.InterruptAll()
	}
	v1_task_hub.close_all()
	// Match the previous shutdown behavior: request all tasks to pause, but do
	// not hold up the service shutdown while task goroutines finish.
	if c.downloader != nil {
		c.downloader.RequestPauseAllTask()
	}
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
