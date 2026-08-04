package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/frontend"
	"wx_channel/internal/database/model"
	"wx_channel/internal/download"
	"wx_channel/internal/services"
	"wx_channel/internal/events"
	"wx_channel/internal/manager"
	"wx_channel/internal/webassets"
	"wx_channel/pkg/hermes"
	"wx_channel/pkg/hermes/protocol"
)

type APIClient struct {
	downloader  *hermes.Engine
	hookManager *hermes.HookManager
	broadcaster *taskBroadcaster
	// official      *officialaccount.OfficialAccountClient
	// channels      *channels.ChannelsClient
	status_ws    *download.StatusHub
	filehelper   *FileHelperHandler
	cfg          *APIConfig
	engine       *gin.Engine
	db           *gorm.DB
	logger       *zerolog.Logger
	httpHandler  http.Handler
	staticAssets *webassets.Registry

	bus               *events.Bus
	proxyStatusMu     sync.RWMutex
	cachedProxyStatus string
	cachedProxyAddr   string
	svcStatusMu       sync.RWMutex
	svcStatuses       map[string]events.ServiceStatusChanged

	// Services
	accountService        *services.AccountService
	contentService        *services.ContentService
	browseService         *services.BrowseService
	downloadTaskService   *services.DownloadTaskService
}

func NewAPIClient(cfg *APIConfig, parent_logger *zerolog.Logger, db *gorm.DB, staticAssets *webassets.Registry) *APIClient {
	logger := parent_logger.With().Str("Client", "api_client").Logger()
	status_ws := download.NewStatusHub()

	// Initialize services
	accountService := services.NewAccountService(db)
	contentService := services.NewContentService(db)
	browseService := services.NewBrowseService(db)
	if staticAssets == nil {
		staticAssets = webassets.NewRegistry()
	}

	apiClient := &APIClient{
		status_ws: status_ws,
		cfg:          cfg,
		engine:       gin.Default(),
		db:           db,
		logger:       &logger,
		staticAssets: staticAssets,
		accountService:        accountService,
		contentService:        contentService,
		browseService:         browseService,
	}

	hookManager := hermes.NewHookManager()
	hookScript := cfg.HooksScript
	if hookScript == "" {
		conventionPath := filepath.Join(cfg.WorkDir, "hooks.js")
		if _, err := os.Stat(conventionPath); err == nil {
			hookScript = conventionPath
		}
	}
	if hookScript != "" {
		if err := hookManager.Load(hookScript); err != nil {
			logger.Warn().Err(err).Str("path", hookScript).Msg("Failed to load hook script")
		}
	}

	apiClient.downloader = hermes.New(hermes.NewOpt{
		Store: &dbTaskStore{db: db, logger: &logger},
		// SpeedLimit: 10 * 1024,
		Logger:           &logger,
		MaxConcurrent:    cfg.MaxRunning,
		FilenameTemplate: cfg.FilenameTemplate,
		BasePath:         cfg.DownloadDir,
	})
	apiClient.broadcaster = newTaskBroadcaster()
	apiClient.downloader.SetEventHandler(func(taskID int, event hermes.EventType, progress *hermes.TaskProgress) {
		logger.Debug().Int("task_id", taskID).Str("event", string(event)).Msg("Hermes task event")
		apiClient.broadcaster.notify(apiClient, taskID, event, progress)
		if event == hermes.EventFinished && apiClient.bus != nil {
			go func() {
				apiClient.bus.Publish(events.DownloadTaskFinished{TaskID: taskID})
			}()
		}
	})
	apiClient.hookManager = hookManager
	apiClient.downloader.SetHooks(hookManager)
	apiClient.downloader.RegisterProtocol(protocol.NewHTTPDriver())
	apiClient.downloader.RegisterProtocol(protocol.NewStreamDriver())
	apiClient.downloader.RegisterProtocol(protocol.NewInlineDriver())

	// Set up composite postprocessor for platform-specific post-download processing
	apiClient.downloader.SetPostprocessor(NewPlatformPostprocessor(db, logger, cfg.DownloadDir))

	apiClient.downloadTaskService = services.NewDownloadTaskService(db, &logger, apiClient.downloader, hookManager, cfg.WorkDir, cfg.DownloadDir)

	status_ws.OnConnected = func(wsClient *download.StatusWSClient) {
		data, err := json.Marshal(APIClientWSMessage{
			Type: "channels_status",
			Data: apiClient.channelsStatusData(),
		})
		if err != nil {
			return
		}
		select {
		case wsClient.Send <- data:
		default:
		}
	}
	// channels_client.OnConnected = func(_ *channels.Client) {
	// 	status_ws.Broadcast(APIClientWSMessage{
	// 		Type: "channels_status",
	// 		Data: apiClient.channelsStatusData(),
	// 	})
	// }
	// channels_client.OnDisconnected = func(_ *channels.Client) {
	// 	status_ws.Broadcast(APIClientWSMessage{
	// 		Type: "channels_status",
	// 		Data: apiClient.channelsStatusData(),
	// 	})
	// }

	// // Set file transfer helper Channels auto-download callback
	// apiClient.filehelper.SetFinderAutoDownloadCallback(apiClient.autoCreateChannelsTask)
	// // Set file transfer helper SPH auto-download callback
	// apiClient.filehelper.SetSphAutoDownloadCallback(apiClient.autoDownloadSphVideo)

	apiClient.SetupRoutes()
	// apiClient.httpHandler = apiClient.buildHTTPHandler()
	return apiClient
}

func (c *APIClient) SubscribeEvents(bus *events.Bus) {
	c.bus = bus
	bus.Subscribe(events.TypeProxyStatusChanged, func(e events.Event) {
		ev, ok := e.(events.ProxyStatusChanged)
		if !ok {
			return
		}
		c.proxyStatusMu.Lock()
		c.cachedProxyStatus = ev.Status
		c.cachedProxyAddr = ev.Addr
		c.proxyStatusMu.Unlock()
	})
	bus.Subscribe(events.TypeBrowseHistoryRecorded, func(e events.Event) {
		ev, ok := e.(events.BrowseHistoryRecorded)
		if !ok || ev.Browse == nil {
			return
		}
		b := ev.Browse
		if err := c.RecordBrowseHistory(b.ExternalId, services.BrowseHistoryInfo{
			PlatformId:        b.PlatformId,
			AccountExternalId: b.AccountExternalId,
			AccountNickname:   b.AccountNickname,
			AccountAvatarURL:  b.AccountAvatarURL,
			ContentType:       b.Type,
			ContentTitle:      b.Title,
			ContentURL:        b.URL,
			ContentSourceURL:  b.SourceURL,
			ContentCoverURL:   b.CoverURL,
			ExtraDataJSON:     b.ExtraData,
		}); err != nil {
			c.logger.Error().Err(err).Str("external_id", b.ExternalId).Msg("create browse history failed")
		}
	})
	bus.Subscribe(events.TypeServiceStatusChanged, func(e events.Event) {
		ev, ok := e.(events.ServiceStatusChanged)
		if !ok {
			return
		}
		c.svcStatusMu.Lock()
		if c.svcStatuses == nil {
			c.svcStatuses = make(map[string]events.ServiceStatusChanged)
		}
		c.svcStatuses[ev.Name] = ev
		c.svcStatusMu.Unlock()
	})
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

func (c *APIClient) serviceStatusesMap() map[string]manager.ServerStatus {
	c.svcStatusMu.RLock()
	defer c.svcStatusMu.RUnlock()
	result := make(map[string]manager.ServerStatus, len(c.svcStatuses))
	for name, s := range c.svcStatuses {
		result[name] = manager.ServerStatus(s.Status)
	}
	return result
}

func (c *APIClient) Start() error {
	return nil
}

func (c *APIClient) Stop() error {
	// Pause all Hermes download tasks
	if c.downloader != nil {
		c.downloader.PauseAll()
	}
	// Directly update V1 database: set in-progress download tasks to paused status
	if c.db != nil {
		now := time.Now().UnixMilli()
		c.db.Model(&model.DownloadTaskV1{}).
			Where("status = ? AND deleted_at IS NULL", model.TaskStatusDownloading).
			Updates(map[string]any{"status": model.TaskStatusPaused, "updated_at": now})
		c.db.Model(&model.DownloadConnection{}).
			Where("status = 1 AND deleted_at IS NULL").
			Updates(map[string]any{"status": 0, "speed": 0, "last_active": now, "updated_at": now})
	}
	// if c.channels != nil {
	// 	c.channels.Stop()
	// }
	if c.status_ws != nil {
		c.status_ws.Stop()
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
	return withCORS(c)
}

// DownloadTaskService returns the download task service.
func (c *APIClient) DownloadTaskService() *services.DownloadTaskService {
	return c.downloadTaskService
}

// CreateDownloadTask is the host capability exposed to registered adapters.
// Both request and config cross the adapter boundary as JSON so concrete
// adapter routes do not depend on API or service-layer request types.
func (c *APIClient) CreateDownloadTask(platform string, contentJSON, configJSON json.RawMessage) (any, error) {
	if c.downloadTaskService == nil {
		return nil, fmt.Errorf("下载任务服务未初始化")
	}
	var cfg services.DownloadConfig
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("解析下载配置失败: %w", err)
	}
	return c.downloadTaskService.CreateTask(services.CreateDownloadTaskV1Body{
		Platform: platform,
		Content:  contentJSON,
		Config:   cfg,
	})
}

func (c *APIClient) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if c.httpHandler == nil {
		c.httpHandler = c.buildHTTPHandler()
	}
	c.httpHandler.ServeHTTP(w, r)
}

func (c *APIClient) setupStaticAssetRoutes() {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		c.engine.Handle(method, "/__assets/lib/*filepath", c.handleChannelLibAsset)
		c.engine.Handle(method, "/__assets/public/*filepath", c.handleChannelPublicAsset)
		c.engine.Handle(method, "/__assets/src/*filepath", c.handleChannelSrcAsset)
		c.engine.Handle(method, "/__assets/inject/*filepath", c.handleFrontendInjectAsset)
		c.engine.Handle(method, "/__assets/platform/*filepath", c.handlePlatformStaticAsset)
	}
}

func (c *APIClient) handleChannelLibAsset(ctx *gin.Context) {
	rel := ctx.Param("filepath")
	data, err := frontend.Assets.ReadLib(rel)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}
	data = frontend.ChannelStaticAssetResponseData(rel, data)
	ctx.Header("Content-Type", frontend.ChannelStaticAssetContentType(rel))
	ctx.Header("Cache-Control", frontend.ChannelLibAssetCacheControl)
	if ctx.Request.Method == http.MethodHead {
		ctx.Status(http.StatusOK)
		return
	}
	ctx.Data(http.StatusOK, frontend.ChannelStaticAssetContentType(rel), data)
}

func (c *APIClient) handleChannelPublicAsset(ctx *gin.Context) {
	rel := ctx.Param("filepath")
	data, err := frontend.Assets.ReadPublic(rel)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}
	data = frontend.ChannelStaticAssetResponseData(rel, data)
	ctx.Header("Content-Type", frontend.ChannelStaticAssetContentType(rel))
	ctx.Header("Cache-Control", frontend.ChannelLibAssetCacheControl)
	if ctx.Request.Method == http.MethodHead {
		ctx.Status(http.StatusOK)
		return
	}
	ctx.Data(http.StatusOK, frontend.ChannelStaticAssetContentType(rel), data)
}

func (c *APIClient) handleFrontendInjectAsset(ctx *gin.Context) {
	rel := ctx.Param("filepath")
	data, err := frontend.Assets.ReadInject(rel)
	if err != nil {
		c.staticAssets.ServeHTTP(ctx.Writer, ctx.Request)
		return
	}
	etag := frontend.ChannelStaticAssetETag(data)
	ctx.Header("Content-Type", frontend.ChannelStaticAssetContentType(rel))
	ctx.Header("Cache-Control", frontend.ChannelSrcAssetCacheControl)
	ctx.Header("ETag", etag)
	if strings.Contains(ctx.GetHeader("If-None-Match"), etag) {
		ctx.Status(http.StatusNotModified)
		return
	}
	if ctx.Request.Method == http.MethodHead {
		ctx.Status(http.StatusOK)
		return
	}
	ctx.Data(http.StatusOK, frontend.ChannelStaticAssetContentType(rel), data)
}

func (c *APIClient) handlePlatformStaticAsset(ctx *gin.Context) {
	c.staticAssets.ServeHTTP(ctx.Writer, ctx.Request)
}

func (c *APIClient) handleChannelSrcAsset(ctx *gin.Context) {
	rel := ctx.Param("filepath")
	data, err := frontend.Assets.ReadSrc(rel)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}
	etag := frontend.ChannelStaticAssetETag(data)
	ctx.Header("Content-Type", frontend.ChannelStaticAssetContentType(rel))
	ctx.Header("Cache-Control", frontend.ChannelSrcAssetCacheControl)
	ctx.Header("ETag", etag)
	if strings.Contains(ctx.GetHeader("If-None-Match"), etag) {
		ctx.Status(http.StatusNotModified)
		return
	}
	if ctx.Request.Method == http.MethodHead {
		ctx.Status(http.StatusOK)
		return
	}
	ctx.Data(http.StatusOK, frontend.ChannelStaticAssetContentType(rel), data)
}

// ---------------------------------------------------------------------------
// dbTaskStore is the GORM adapter for hermes.Store.
// ---------------------------------------------------------------------------

type dbTaskStore struct {
	db     *gorm.DB
	logger *zerolog.Logger
}

func (s *dbTaskStore) debug(format string, args ...interface{}) {
	if s.logger != nil {
		s.logger.Info().Msgf("[dbTaskStore] "+format, args...)
	}
}

var _ hermes.Store = (*dbTaskStore)(nil)
var _ hermes.OutputNameStore = (*dbTaskStore)(nil)

func (s *dbTaskStore) LoadTask(taskID int) (*hermes.Task, error) {
	var task model.DownloadTaskV1
	if err := s.db.Where("id = ?", taskID).First(&task).Error; err != nil {
		return nil, err
	}
	var resources []model.DownloadResource
	if err := s.db.Where("task_id = ?", task.Id).Order("merge_order ASC, id ASC").Find(&resources).Error; err != nil {
		return nil, err
	}
	if len(resources) == 0 {
		return nil, errors.New("任务没有下载资源")
	}
	resourceIDs := make([]int, len(resources))
	for i, resource := range resources {
		resourceIDs[i] = resource.Id
	}
	var endpoints []model.DownloadEndpoint
	if err := s.db.Where("resource_id IN ? AND enabled = ?", resourceIDs, 1).
		Order("resource_id ASC, priority ASC, id ASC").Find(&endpoints).Error; err != nil {
		return nil, err
	}
	if len(endpoints) == 0 {
		return nil, errors.New("任务没有已启用的下载端点")
	}
	endpointsByResource := make(map[int][]hermes.Endpoint, len(resources))
	for _, endpoint := range endpoints {
		headers := make(map[string]string)
		if strings.TrimSpace(endpoint.Headers) != "" {
			if err := json.Unmarshal([]byte(endpoint.Headers), &headers); err != nil {
				return nil, fmt.Errorf("解析端点 %d headers 失败: %w", endpoint.Id, err)
			}
		}
		endpointsByResource[endpoint.ResourceId] = append(endpointsByResource[endpoint.ResourceId], hermes.Endpoint{
			ID:       endpoint.Id,
			Protocol: endpoint.Protocol,
			URL:      endpoint.URL,
			Priority: endpoint.Priority,
			Headers:  headers,
			Cookies:  endpoint.Cookies,
		})
	}
	resourceInfos := make([]hermes.Resource, 0, len(resources))
	for _, resource := range resources {
		resourceEndpoints := endpointsByResource[resource.Id]
		if len(resourceEndpoints) == 0 {
			return nil, fmt.Errorf("资源 %d 没有已启用的下载端点", resource.Id)
		}
			extra := parseExtra(resource.Extra)
		resourceInfos = append(resourceInfos, hermes.Resource{
			ID:           resource.Id,
			Name:         resource.Name,
			Kind:         resource.Kind,
			ResourceType: resource.ResourceType,
			UniqueID:     resource.UniqueID,
			Endpoints:    resourceEndpoints,
			Extra:        extra,
		})
	}
	primary := resourceInfos[0]
	return &hermes.Task{
		ID:         task.Id,
		Name:       task.Name,
		ResourceID: primary.ID,
		Platform:   task.PlatformId,
		Endpoints:  primary.Endpoints,
		Resources:  resourceInfos,
		Config:     task.ConfigJSON,
		Metadata:   task.MetadataJSON,
	}, nil
}

func (s *dbTaskStore) UpdateStatus(taskID int, status int) error {
	now := time.Now().UnixMilli()
	return s.db.Model(&model.DownloadTaskV1{}).Where("id = ?", taskID).
		Updates(map[string]any{"status": status, "updated_at": now}).Error
}

func (s *dbTaskStore) ActivateTask(taskID int) error {
	now := time.Now().UnixMilli()

	// Get all endpoints for this task.
	var endpoints []model.DownloadEndpoint
	if err := s.db.Where("resource_id IN (SELECT id FROM download_resource WHERE task_id = ?)", taskID).Find(&endpoints).Error; err != nil {
		return err
	}

	// Create connections for endpoints that don't have one yet.
	for _, ep := range endpoints {
		var count int64
		if err := s.db.Model(&model.DownloadConnection{}).Where("endpoint_id = ?", ep.Id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			host := ""
			if parsedURL, err := url.Parse(ep.URL); err == nil {
				host = parsedURL.Host
			}
			conn := model.DownloadConnection{
				EndpointId: ep.Id,
				WorkerId:   "worker-" + strconv.Itoa(ep.Id),
				Host:       host,
				Status:     1,
				Bytes:      0,
				Speed:      0,
				LastActive: now,
			}
			conn.CreatedAt = now
			conn.UpdatedAt = now
			if err := s.db.Create(&conn).Error; err != nil {
				return err
			}
		}
	}

	// Set start_time for all resources.
	if err := s.db.Model(&model.DownloadResource{}).
		Where("task_id = ? AND start_time IS NULL", taskID).
		Updates(map[string]any{"start_time": now, "updated_at": now}).Error; err != nil {
		return err
	}

	// Activate all endpoints.
	if err := s.db.Model(&model.DownloadEndpoint{}).
		Where("resource_id IN (SELECT id FROM download_resource WHERE task_id = ?)", taskID).
		Updates(map[string]any{"status": 1, "updated_at": now}).Error; err != nil {
		return err
	}

	// Activate all connections.
	return s.db.Model(&model.DownloadConnection{}).
		Where("endpoint_id IN (SELECT id FROM download_endpoint WHERE resource_id IN (SELECT id FROM download_resource WHERE task_id = ?))", taskID).
		Updates(map[string]any{"status": 1, "last_active": now, "updated_at": now}).Error
}

func (s *dbTaskStore) UpdateProgress(taskID int, downloaded int64, speed int64) error {
	now := time.Now().UnixMilli()
	if err := s.db.Exec(`UPDATE download_connection SET speed = ?, bytes = ?, last_active = ?, updated_at = ?
		WHERE endpoint_id IN (
			SELECT id FROM download_endpoint WHERE resource_id IN (
				SELECT id FROM download_resource WHERE task_id = ?
			)
		)`, speed, downloaded, now, now, taskID).Error; err != nil {
		return err
	}
	return s.db.Exec(`UPDATE download_resource SET status = 1, updated_at = ? WHERE task_id = ? AND status IN (0,1)`,
		now, taskID).Error
}

func (s *dbTaskStore) UpdateResourceSize(taskID int, size int64) error {
	now := time.Now().UnixMilli()
	return s.db.Exec(`UPDATE download_resource SET size = ?, updated_at = ? WHERE task_id = ?`,
		size, now, taskID).Error
}

func (s *dbTaskStore) UpdateOutputName(update hermes.OutputNameUpdate) error {
	s.debug("UpdateOutputName called: task_id=%d resource_id=%d task_name=%q resource_name=%q save_path=%q",
		update.TaskID, update.ResourceID, update.TaskName, update.ResourceName, update.SavePath)

	if update.TaskID <= 0 || update.ResourceID <= 0 || strings.TrimSpace(update.ResourceName) == "" {
		s.debug("UpdateOutputName invalid parameters, skipping update")
		return errors.New("下载文件名更新参数无效")
	}

	now := time.Now().UnixMilli()
	s.debug("UpdateOutputName starting transaction to update download_resource: id=%d task_id=%d name=%q",
		update.ResourceID, update.TaskID, update.ResourceName)

	err := s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.DownloadResource{}).
			Where("id = ? AND task_id = ?", update.ResourceID, update.TaskID).
			Updates(map[string]any{"name": update.ResourceName, "updated_at": now})
		if result.Error != nil {
			s.debug("UpdateOutputName download_resource update failed: %v", result.Error)
			return result.Error
		}
		s.debug("UpdateOutputName download_resource RowsAffected=%d", result.RowsAffected)
		if result.RowsAffected == 0 {
			return fmt.Errorf("更新资源名未影响任何行: resource_id=%d task_id=%d new_name=%q",
				update.ResourceID, update.TaskID, update.ResourceName)
		}
		if update.TaskName == "" {
			return nil
		}
		return tx.Model(&model.DownloadTaskV1{}).Where("id = ?", update.TaskID).
			Updates(map[string]any{"name": update.TaskName, "save_path": update.SavePath, "updated_at": now}).Error
	})
	if err != nil {
		s.debug("UpdateOutputName transaction failed: %v", err)
	} else {
		s.debug("UpdateOutputName transaction succeeded")
	}
	return err
}

func (s *dbTaskStore) UpdateResourceProgress(resourceID int, downloaded int64, speed int64) error {
	now := time.Now().UnixMilli()
	if err := s.db.Exec(`UPDATE download_connection SET speed = ?, bytes = ?, last_active = ?, updated_at = ?
		WHERE endpoint_id IN (SELECT id FROM download_endpoint WHERE resource_id = ?)`,
		speed, downloaded, now, now, resourceID).Error; err != nil {
		return err
	}
	return s.db.Model(&model.DownloadResource{}).Where("id = ?", resourceID).
		Updates(map[string]any{"status": 1, "downloaded": downloaded, "speed": speed, "updated_at": now}).Error
}

func (s *dbTaskStore) UpdateResourceSizeByID(resourceID int, size int64) error {
	now := time.Now().UnixMilli()
	return s.db.Model(&model.DownloadResource{}).Where("id = ?", resourceID).
		Updates(map[string]any{"size": size, "status": 1, "updated_at": now}).Error
}

func (s *dbTaskStore) FinishResource(resourceID int) error {
	now := time.Now().UnixMilli()
	if err := s.db.Model(&model.DownloadResource{}).Where("id = ?", resourceID).
		Updates(map[string]any{"status": 2, "finish_time": now, "updated_at": now}).Error; err != nil {
		return err
	}
	return s.db.Exec(`UPDATE download_connection SET speed = 0, status = 2, updated_at = ?
		WHERE endpoint_id IN (SELECT id FROM download_endpoint WHERE resource_id = ?)`, now, resourceID).Error
}

func (s *dbTaskStore) DeactivateConnections(taskID int) error {
	now := time.Now().UnixMilli()

	// Update resource status for this task's downloading resources.
	if err := s.db.Model(&model.DownloadResource{}).
		Where("task_id = ? AND status = 1", taskID).
		Updates(map[string]any{"status": 1, "updated_at": now}).Error; err != nil {
		return err
	}

	// Update segment status for this task's active segments.
	if err := s.db.Model(&model.DownloadSegment{}).
		Where("resource_id IN (SELECT id FROM download_resource WHERE task_id = ?) AND status = 1", taskID).
		Updates(map[string]any{"status": 1, "updated_at": now}).Error; err != nil {
		return err
	}

	// Deactivate connections.
	return s.db.Exec(`UPDATE download_connection SET speed = 0, status = 2, updated_at = ?
		WHERE endpoint_id IN (
			SELECT id FROM download_endpoint WHERE resource_id IN (
				SELECT id FROM download_resource WHERE task_id = ?
			)
		)`, now, taskID).Error
}

func (s *dbTaskStore) FinishTask(taskID int) error {
	now := time.Now().UnixMilli()
	s.db.Model(&model.DownloadTaskV1{}).Where("id = ?", taskID).
		Updates(map[string]any{"status": model.TaskStatusFinished, "updated_at": now})
	s.db.Exec(`UPDATE download_segment SET downloaded = CASE WHEN size > 0 THEN size ELSE downloaded END, status = 2, updated_at = ?
		WHERE resource_id IN (SELECT id FROM download_resource WHERE task_id = ?)`, now, taskID)
	s.db.Exec(`UPDATE download_connection SET speed = 0, status = 2, updated_at = ?
		WHERE endpoint_id IN (
			SELECT id FROM download_endpoint WHERE resource_id IN (
				SELECT id FROM download_resource WHERE task_id = ?
			)
		)`, now, taskID)
	s.db.Exec(`UPDATE download_resource SET status = 2, updated_at = ? WHERE task_id = ?`, now, taskID)
	return nil
}

func (s *dbTaskStore) RecordError(taskID int, errMsg string) error {
	return s.db.Model(&model.DownloadTaskV1{}).Where("id = ?", taskID).
		Updates(map[string]any{"error_message": errMsg, "updated_at": time.Now().UnixMilli()}).Error
}

func (s *dbTaskStore) CreateSegments(resourceID int, url string, ranges []hermes.SegmentRange) ([]int, error) {
	now := time.Now().UnixMilli()
	var ids []int
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("resource_id = ?", resourceID).Delete(&model.DownloadSegment{}).Error; err != nil {
			return err
		}
		for _, r := range ranges {
			seg := model.DownloadSegment{
				ResourceId:  resourceID,
				Index:       r.Index,
				URL:         url,
				OffsetStart: r.OffsetStart,
				OffsetEnd:   r.OffsetEnd,
				Size:        r.Size,
				Downloaded:  0,
				Status:      1,
			}
			seg.CreatedAt = now
			seg.UpdatedAt = now
			if err := tx.Create(&seg).Error; err != nil {
				return err
			}
			ids = append(ids, seg.Id)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (s *dbTaskStore) UpdateSegmentProgress(segID int, downloaded int64) error {
	now := time.Now().UnixMilli()
	return s.db.Model(&model.DownloadSegment{}).Where("id = ?", segID).
		Updates(map[string]any{"downloaded": downloaded, "updated_at": now}).Error
}

func (s *dbTaskStore) LoadSegmentInfo(resourceID int) ([]hermes.Segment, error) {
	var segs []model.DownloadSegment
	if err := s.db.Where("resource_id = ?", resourceID).Order("`index` ASC").Find(&segs).Error; err != nil {
		return nil, err
	}
	infos := make([]hermes.Segment, len(segs))
	for i, s := range segs {
		infos[i] = hermes.Segment{
			ID:          s.Id,
			Index:       s.Index,
			URL:         s.URL,
			OffsetStart: s.OffsetStart,
			OffsetEnd:   s.OffsetEnd,
			Size:        s.Size,
			Downloaded:  s.Downloaded,
		}
	}
	return infos, nil
}

// parseExtra parses a JSON string into map[string]string for passing through user-defined fields.
func parseExtra(raw string) map[string]string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var attrs map[string]string
	if err := json.Unmarshal([]byte(raw), &attrs); err != nil {
		return nil
	}
	return attrs
}
