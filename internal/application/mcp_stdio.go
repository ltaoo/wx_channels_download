package application

import (
	"context"
	"fmt"
	"io"

	"github.com/ltaoo/velo"
	gorm_logger "gorm.io/gorm/logger"

	"wx_channel/internal/adapter"
	"wx_channel/internal/api"
	"wx_channel/internal/config"
	"wx_channel/internal/database"
	"wx_channel/internal/events"
	"wx_channel/internal/mcpserver"
	"wx_channel/internal/services"
	"wx_channel/pkg/cache"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/hermes"
	"wx_channel/pkg/hermes/protocol"
)

// MCPStdioConfig configures a process-local MCP server that does not start the
// HTTP API listener.
type MCPStdioConfig struct {
	Input       io.Reader
	Output      io.Writer
	ErrorOutput io.Writer
}

type mcp_stdio_runtime struct {
	server              *mcpserver.Server
	scraper_job_service *services.ScraperJobService
	downloader          *hermes.HermesEngine
	task_store          *database.DBTaskStore
	adapter_handles     []adapter.RuntimeHandle
}

// ServeMCPStdio initializes the query and scraper services in-process and
// serves newline-delimited JSON-RPC over stdin/stdout without starting the API
// or proxy listeners.
func ServeMCPStdio(ctx context.Context, cfg *config.Config, stdio_config MCPStdioConfig) error {
	runtime, err := new_mcp_stdio_runtime(cfg, stdio_config)
	if err != nil {
		return err
	}
	defer runtime.close()
	return runtime.server.Serve(ctx)
}

func new_mcp_stdio_runtime(cfg *config.Config, stdio_config MCPStdioConfig) (*mcp_stdio_runtime, error) {
	if cfg == nil {
		return nil, fmt.Errorf("应用配置不能为空")
	}
	logger := cfg.Logger()
	cache_registry, err := cache.NewProviderRegistry(cfg.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("persistent cache initialization failed: %w", err)
	}
	cookie_reader := cookies.NewPersistentReader(cfg.WorkDir)

	app := velo.NewApp(&velo.VeloAppOpt{Mode: velo.ModeHttp})
	if err := app.Migrate(&velo.VeloDatabaseOpt{
		DBType:     velo.DBTypeSQLite,
		DBPath:     cfg.DBPath,
		Migrations: &database.Migrations,
	}); err != nil {
		return nil, fmt.Errorf("database initialization failed: %w", err)
	}
	// A stdio protocol must keep stdout reserved for JSON-RPC frames. Velo's
	// default GORM logger writes callback warnings to stdout during SQLite setup.
	app.DB.Logger = gorm_logger.Default.LogMode(gorm_logger.Silent)
	if err := database.ConfigureSQLiteRuntime(app.DB); err != nil {
		return nil, fmt.Errorf("database sqlite configuration failed: %w", err)
	}

	api_config := api.NewAPIConfig(cfg)
	hook_manager := hermes.NewHookManager()
	if script := api_config.HooksScript; script != "" {
		if err := hook_manager.LoadFile(script); err != nil {
			return nil, fmt.Errorf("failed to load download hook script: %w", err)
		}
	}

	task_store := database.NewDBTaskStore(app.DB, logger)
	downloader := hermes.New(hermes.HermesNewConfig{
		Store:  task_store,
		Logger: logger,
		Config: hermes.HermesEngineConfig{
			MaxConcurrent:         api_config.MaxRunning,
			ResourceConcurrency:   api_config.ResourceConcurrency,
			SegmentConcurrency:    api_config.SegmentConcurrency,
			ConnectionConcurrency: api_config.ConnectionConcurrency,
			FilenameTemplate:      api_config.FilenameTemplate,
			BasePath:              api_config.DownloadDir,
		},
	})
	downloader.RegisterProtocol(protocol.NewHTTPDriver())
	downloader.RegisterProtocol(protocol.NewStreamDriver())
	downloader.RegisterProtocol(protocol.NewInlineDriver())
	downloader.RegisterProtocol(protocol.NewFileDriver())
	downloader.SetHooks(hook_manager)
	downloader.SetPostprocessor(adapter.NewPlatformPostprocessor(app.DB, *logger, api_config.DownloadDir))

	account_service := services.NewAccountService(app.DB)
	browse_history_service := services.NewBrowseService(app.DB, *logger)
	certificate_service := services.NewCertificateService(cfg)
	download_task_service := services.NewDownloadTaskService(
		app.DB,
		logger,
		downloader,
		hook_manager,
		cfg.WorkDir,
		api_config.DownloadDir,
	)
	scraper_job_service := services.NewScraperJobService(new_scraper_platform_checker(cfg), nil, logger)
	scraper_job_service.SetRetentionLimit(cfg.GetInt("scraper.retainedJobs"))
	data_service := services.NewDataQueryService(services.DataQueryServiceConfig{
		DB:                   app.DB,
		AccountService:       account_service,
		DownloadTaskService:  download_task_service,
		BrowseHistoryService: browse_history_service,
		CertificateService:   certificate_service,
		LogPath:              api_config.LogPath,
		WorkDir:              api_config.WorkDir,
	})

	bus := events.NewBus()
	adapter_handles := make([]adapter.RuntimeHandle, 0)
	for _, platform_id := range adapter.IDs() {
		handler := adapter.Get(platform_id)
		if !adapter.RuntimeEnabled(handler, cfg) {
			continue
		}
		runtime_adapter, ok := handler.(adapter.RuntimeAdapter)
		if !ok {
			continue
		}
		cache_provider, cache_err := cache_registry.Namespace(platform_id)
		if cache_err != nil {
			stop_adapter_handles(adapter_handles)
			downloader.RequestPauseAllTask()
			task_store.Shutdown()
			return nil, fmt.Errorf("failed to create cache namespace for platform %s: %w", platform_id, cache_err)
		}
		handle, register_err := runtime_adapter.RegisterRuntime(&adapter.AdapterOptions{
			DB:      app.DB,
			Logger:  logger,
			Bus:     bus,
			Config:  cfg,
			Cache:   cache_provider,
			Cookies: cookie_reader,
			Hooks:   hook_manager,
		})
		if register_err != nil {
			stop_adapter_handles(adapter_handles)
			downloader.RequestPauseAllTask()
			task_store.Shutdown()
			return nil, fmt.Errorf("failed to register platform %s: %w", platform_id, register_err)
		}
		adapter_handles = append(adapter_handles, handle)
	}

	server, err := mcpserver.NewServer(mcpserver.Config{
		Version:     api_config.Version,
		Input:       stdio_config.Input,
		Output:      stdio_config.Output,
		ErrorOutput: stdio_config.ErrorOutput,
		DataReader:  new_mcp_data_reader(data_service),
		ScraperJobs: new_mcp_scraper_job_backend(scraper_job_service),
	})
	if err != nil {
		stop_adapter_handles(adapter_handles)
		downloader.RequestPauseAllTask()
		task_store.Shutdown()
		return nil, err
	}
	return &mcp_stdio_runtime{
		server:              server,
		scraper_job_service: scraper_job_service,
		downloader:          downloader,
		task_store:          task_store,
		adapter_handles:     adapter_handles,
	}, nil
}

func (r *mcp_stdio_runtime) close() {
	if r == nil {
		return
	}
	if r.scraper_job_service != nil {
		r.scraper_job_service.InterruptAll()
	}
	stop_adapter_handles(r.adapter_handles)
	if r.downloader != nil {
		r.downloader.RequestPauseAllTask()
	}
	if r.task_store != nil {
		r.task_store.Shutdown()
	}
}

func stop_adapter_handles(handles []adapter.RuntimeHandle) {
	for handle_index := len(handles) - 1; handle_index >= 0; handle_index-- {
		if handles[handle_index] != nil {
			handles[handle_index].Stop()
		}
	}
}
