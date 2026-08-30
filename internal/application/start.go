package application

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/ltaoo/velo"
	"github.com/pterm/pterm"

	"wx_channel/frontend"
	"wx_channel/internal/adapter"
	"wx_channel/internal/api"
	"wx_channel/internal/buildtags"
	"wx_channel/internal/config"
	"wx_channel/internal/database"
	"wx_channel/internal/events"
	"wx_channel/internal/interceptor"
	"wx_channel/internal/interceptor/proxy"
	"wx_channel/internal/services"
	"wx_channel/internal/webassets"
	"wx_channel/pkg/cache"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/hermes"
	"wx_channel/pkg/hermes/protocol"
	"wx_channel/pkg/system"
)

// Start initializes and runs the local admin, API, and interceptor services.
func Start(cfg *config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Printf("\nv%v\n", cfg.Version)
	fmt.Printf("Feedback/Issues https://github.com/ltaoo/wx_channels_download/issues\n\n")

	logger := cfg.Logger()
	cache_registry, err := cache.NewProviderRegistry(cfg.WorkDir)
	if err != nil {
		return fmt.Errorf("persistent cache initialization failed: %w", err)
	}
	cookie_reader := cookies.NewPersistentReader(cfg.WorkDir)

	b := velo.NewApp(&velo.VeloAppOpt{Mode: velo.ModeHttp})
	if err := b.Migrate(&velo.VeloDatabaseOpt{
		DBType:                    velo.DBTypeSQLite,
		DBPath:                    database.SQLiteDSN(cfg.DBPath),
		Migrations:                &database.Migrations,
		DisableTimestampCallbacks: true,
	}); err != nil {
		return fmt.Errorf("database initialization failed: %w", err)
	}
	if err := database.ConfigureSQLiteRuntime(b.DB); err != nil {
		return fmt.Errorf("database sqlite configuration failed: %w", err)
	}

	api_cfg := api.NewAPIConfig(cfg)
	proxy_enabled := cfg.GetBool("proxy.enabled")
	static_assets := webassets.NewRegistry()
	if cfg.GlobalScriptPath != "" {
		global_script_asset_path := frontend.UserGlobalScriptAssetPath(cfg.GlobalScriptPath)
		logger.Info().
			Str("file", "internal/application/start.go").
			Str("path", cfg.GlobalScriptPath).
			Str("asset_path", global_script_asset_path).
			Msg("registering global script asset")
		if err := static_assets.RegisterFile(
			global_script_asset_path,
			os.DirFS(filepath.Dir(cfg.GlobalScriptPath)),
			filepath.Base(cfg.GlobalScriptPath),
		); err != nil {
			logger.Warn().
				Err(err).
				Str("file", "internal/application/start.go").
				Str("path", cfg.GlobalScriptPath).
				Str("asset_path", global_script_asset_path).
				Msg("failed to register global script asset")
		} else {
			logger.Info().
				Str("file", "internal/application/start.go").
				Str("path", cfg.GlobalScriptPath).
				Str("asset_path", global_script_asset_path).
				Msg("global script asset registered")
		}
	}
	bus := events.NewBus()
	cert_files := services.LoadCertFiles()
	interceptor_srv := interceptor.NewInterceptorServer(cfg, cert_files, logger)
	interceptor_srv.SubscribeEvents(bus)

	if cfg.GetBool("download.remoteServer.enabled") {
		target_protocol := cfg.GetString("download.remoteServer.protocol")
		target_hostname := cfg.GetString("download.remoteServer.hostname")
		target_port := cfg.GetInt("download.remoteServer.port")
		logger.Info().
			Str("file", "internal/application/start.go").
			Str("protocol", target_protocol).
			Str("hostname", target_hostname).
			Int("port", target_port).
			Msg("enable remote server")
		plugin := &proxy.Plugin{
			Match: "weixin110.qq.com",
			Target: &proxy.TargetConfig{
				Protocol: target_protocol,
				Host:     target_hostname,
				Port:     target_port,
			},
		}
		interceptor_srv.Interceptor.AddPostPlugin(plugin)
	}

	table_data := pterm.TableData{{"Item", "Path"}, {"Work Dir", cfg.WorkDir}, {"Data Path", cfg.DBPath}}
	if cfg.LogPath() != "" {
		table_data = append(table_data, []string{"Log File", cfg.LogPath()})
	}
	if cfg.FullPath != "" {
		table_data = append(table_data, []string{"Config File", cfg.FullPath})
	}
	table_data = append(table_data, []string{"Download Dir", api_cfg.DownloadDir})
	// --- Hook manager ---
	hook_manager := hermes.NewHookManager()
	configured_hook_path := cfg.GetString("download.hooksScript")
	logger.Info().
		Str("file", "internal/application/start.go").
		Str("configured_path", api_cfg.HooksScript).
		Str("resolved_config_path", configured_hook_path).
		Bool("exists", api_cfg.HooksScript != "").
		Msg("download hook script discovery")
	if script := api_cfg.HooksScript; script != "" {
		if err := hook_manager.LoadFile(script); err != nil {
			logger.Warn().
				Err(err).
				Str("file", "internal/application/start.go").
				Str("path", script).
				Msg("Failed to load hook script")
		} else {
			logger.Info().
				Str("file", "internal/application/start.go").
				Str("path", script).
				Str("source", "download.hooksScript").
				Msg("download hook script loaded")
		}
	}

	// --- Database store ---
	task_store := database.NewDBTaskStore(b.DB, logger)
	account_service := services.NewAccountService(b.DB)
	content_service := services.NewContentService(b.DB)
	browse_history_service := services.NewBrowseService(b.DB, *logger)
	fs_service := services.NewFSService()
	certificate_service := services.NewCertificateService(cfg)
	scraper_job_service := services.NewScraperJobService(
		new_scraper_platform_checker(),
		api.BroadcastScraperJobEvent,
		logger,
	)
	scraper_job_service.SetRetentionLimit(cfg.GetInt("scraper.retainedJobs"))

	// --- Download engine ---
	downloader := hermes.New(hermes.HermesNewConfig{
		Store:  task_store,
		Logger: logger,
		Config: hermes.HermesEngineConfig{
			MaxConcurrent:         api_cfg.MaxRunning,
			ResourceConcurrency:   api_cfg.ResourceConcurrency,
			SegmentConcurrency:    api_cfg.SegmentConcurrency,
			ConnectionConcurrency: api_cfg.ConnectionConcurrency,
			FilenameTemplate:      api_cfg.FilenameTemplate,
			BasePath:              api_cfg.DownloadDir,
			// SpeedLimit:            10 * 1024,
		},
	})
	downloader.RegisterProtocol(protocol.NewHTTPDriver())
	downloader.RegisterProtocol(protocol.NewStreamDriver())
	downloader.RegisterProtocol(protocol.NewInlineDriver())
	downloader.RegisterProtocol(protocol.NewFileDriver())
	downloader.SetHooks(hook_manager)
	downloader.SetPostprocessor(adapter.NewPlatformPostprocessor(b.DB, *logger, api_cfg.DownloadDir))
	download_task_service := services.NewDownloadTaskService(
		b.DB,
		logger,
		downloader,
		hook_manager,
		cfg.WorkDir,
		api_cfg.DownloadDir,
		bus,
	)
	runtime_status_service := services.NewRuntimeStatusService()
	download_task_broadcaster := api.NewDownloadTaskBroadcaster(b.DB, logger, download_task_service)
	bus.Subscribe(events.TypeDownloadTaskCreated, func(event events.Event) {
		created, ok := event.(events.DownloadTaskCreated)
		if ok {
			download_task_broadcaster.NotifyCreated(created.TaskID)
		}
	})
	bus.Subscribe(events.TypeDownloadTaskDeleted, func(event events.Event) {
		deleted, ok := event.(events.DownloadTaskDeleted)
		if ok {
			download_task_broadcaster.NotifyDeleted(deleted.TaskID)
		}
	})
	downloader.OnEvent(func(event hermes.EventType, data hermes.EventData) {
		task_id, progress, finished_resources, ok := download_task_event_data(event, data)
		if !ok {
			return
		}
		logger.Info().Str("file", "/application/start.go").Int("task_id", task_id).Str("event", string(event)).Msg("Hermes task event")
		download_task_broadcaster.Notify(task_id, event, progress, finished_resources)
		if event == hermes.EventFinished {
			go bus.Publish(events.DownloadTaskFinished{TaskID: task_id})
		}
	})
	bridge_service := services.NewBridgeService(services.BridgeServiceOptions{
		ApplicationConfig:   cfg,
		DownloadTaskService: download_task_service,
		Logger:              logger,
	})
	data_service := services.NewDataQueryService(services.DataQueryServiceConfig{
		DB:                   b.DB,
		AccountService:       account_service,
		DownloadTaskService:  download_task_service,
		BrowseHistoryService: browse_history_service,
		CertificateService:   certificate_service,
		LogPath:              api_cfg.LogPath,
		WorkDir:              api_cfg.WorkDir,
	})
	mcp_service, err := new_mcp_service(api_cfg, data_service, download_task_service, scraper_job_service, cfg.GetBool("mcp.enabled"))
	if err != nil {
		task_store.Shutdown()
		return fmt.Errorf("failed to initialize MCP service: %w", err)
	}

	// --- API service ---
	restart_service := services.NewApplicationRestartService(services.ApplicationRestartServiceOptions{
		RequestRestart: func() error {
			return restart_current_process(stop)
		},
	})
	application_update_service := new_application_update_service(api_cfg.Version, restart_service)
	api_srv := api.NewAPIServer(
		api_cfg,
		logger,
		b.DB,
		static_assets,
		download_task_broadcaster,
		bus,
		runtime_status_service,
		account_service,
		content_service,
		browse_history_service,
		download_task_service,
		fs_service,
		scraper_job_service,
		bridge_service,
		certificate_service,
		mcp_service,
		application_update_service,
		restart_service,
	)
	bus.Subscribe(events.TypeProxyStatusChanged, func(event events.Event) {
		status, ok := event.(events.ProxyStatusChanged)
		if ok {
			runtime_status_service.UpdateProxyStatus(status)
		}
	})
	bus.Subscribe(events.TypeServiceStatusChanged, func(event events.Event) {
		status, ok := event.(events.ServiceStatusChanged)
		if ok {
			runtime_status_service.UpdateServiceStatus(status)
		}
	})
	bus.Subscribe(events.TypeScraperFetchProgress, func(event events.Event) {
		progress, ok := event.(events.ScraperFetchProgress)
		if ok {
			scraper_job_service.UpdateProgress(progress)
		}
	})
	bus.Subscribe(events.TypePlatformStatusChanged, func(event events.Event) {
		status, ok := event.(events.PlatformStatusChanged)
		if !ok {
			return
		}
		status, changed := runtime_status_service.UpdatePlatformStatus(status)
		if changed {
			api.BroadcastPlatformStatus(&status)
		}
	})
	bus.Subscribe(events.TypeServiceCommand, func(event events.Event) {
		command, ok := event.(events.ServiceCommand)
		if !ok || command.Name != "api" {
			return
		}
		switch command.Action {
		case "start":
			_ = api_srv.Start()
		case "stop":
			_ = api_srv.Stop()
		}
	})
	publish_registered_adapter_statuses(bus)
	// admin_srv := admin.NewAdminServer(cfg, b, bus)
	if cfg.GlobalScriptPath != "" {
		table_data = append(table_data, []string{"Global Script", cfg.GlobalScriptPath})
	}
	if api_cfg.HooksScript != "" {
		table_data = append(table_data, []string{"Hooks Script", api_cfg.HooksScript})
	}
	pterm.DefaultTable.WithHasHeader().WithData(table_data).Render()
	fmt.Println()

	adapter_handles := make([]adapter.RuntimeHandle, 0)
	for _, platform_id := range adapter.IDs() {
		handler := adapter.Get(platform_id)
		runtime_adapter, ok := handler.(adapter.RuntimeAdapter)
		if !ok {
			continue
		}
		cache_provider, err := cache_registry.Namespace(platform_id)
		if err != nil {
			return fmt.Errorf("failed to create cache namespace for platform %s: %w", platform_id, err)
		}
		adapter_options := &adapter.AdapterOptions{
			StaticAssets: static_assets,
			Routes:       api_srv.APIClient,
			Interceptor:  interceptor_srv.Interceptor,
			DB:           b.DB,
			Logger:       logger,
			Bus:          bus,
			Config:       cfg,
			Cache:        cache_provider,
			Cookies:      cookie_reader,
			Hooks:        hook_manager,
		}
		handle, err := runtime_adapter.RegisterRuntime(adapter_options)
		if err != nil {
			return fmt.Errorf("failed to register platform %s: %w", platform_id, err)
		}
		adapter_handles = append(adapter_handles, handle)
	}

	var cleanup_once sync.Once
	bridge_started := false
	api_started := false
	interceptor_start_attempted := false
	cleanup := func() {
		cleanup_once.Do(func() {
			fmt.Printf("\nShutting down downloader...\n")
			// Reset the system proxy first. Windows only gives console close
			// handlers a short grace period before terminating the process.
			if interceptor_start_attempted {
				if err := interceptor_srv.Stop(); err != nil {
					color.Red(fmt.Sprintf("Failed to stop proxy service: %v\n", err))
				}
			}
			if bridge_started {
				bridge_service.Close()
			}
			scraper_job_service.InterruptAll()
			for i := len(adapter_handles) - 1; i >= 0; i-- {
				adapter_handles[i].Stop()
			}
			if api_started {
				if err := api_srv.Stop(); err != nil {
					color.Red(fmt.Sprintf("Failed to stop API service: %v\n", err))
				}
			}
			// Match the previous shutdown behavior: request all tasks to pause, but do
			// not hold up shutdown while task goroutines finish.
			downloader.RequestPauseAllTask()
			task_store.Shutdown()
			// if err := admin_srv.Stop(); err != nil {
			// 	color.Red(fmt.Sprintf("Failed to stop GUI/Admin service: %v\n", err))
			// }
			color.Green("Downloader has been shut down")
		})
	}

	// if err := admin_srv.Start(); err != nil {
	// 	color.Red(fmt.Sprintf("ERROR Failed to start GUI/Admin service: %v\n", err))
	// 	cleanup()
	// 	os.Exit(0)
	// 	return
	// }
	// color.Green(fmt.Sprintf("GUI/Admin service started successfully, address: %v", admin_srv.Addr()))
	bridge_started = true
	if err := bridge_service.Start(ctx); err != nil {
		logger.Error().Err(err).Msg("Bridge 服务启动失败，应用将继续启动")
	}
	if err := api_srv.Start(); err != nil {
		cleanup()
		return fmt.Errorf("failed to start API service: %w", err)
	}
	api_started = true
	api_url := http_service_url(api_srv.Addr())
	color.Green(fmt.Sprintf("API service started successfully, address: %v", api_url))
	if mcp_service.Enabled() {
		color.Green(fmt.Sprintf("MCP server started successfully, address: %v/mcp", api_url))
	}

	if proxy_enabled {
		interceptor_start_attempted = true
		unregister_console_close, err := registerConsoleCloseHandler(stop)
		if err != nil {
			interceptor_start_attempted = false
			cleanup()
			return fmt.Errorf("failed to register console close handler: %w", err)
		}
		defer unregister_console_close()

		// Resolve the target network service once, before the proxy is enabled, and pin it for
		// the rest of the run. Start and Stop both resolve an empty device on their own, so
		// leaving it empty would let them disagree when the primary service changes mid-run:
		// the proxy would be cleared on the service that is primary at exit while staying
		// enabled on the one it was written to, which breaks connectivity once that service
		// becomes primary again.
		proxy_service, proxy_warning := system.ProxyTargetDescription(interceptor_srv.ProxyDevice())
		if proxy_service != "" {
			// Empty means the platform has no per-service proxy (Windows, Linux); leave whatever
			// was configured alone there rather than overwriting it.
			interceptor_srv.SetProxyDevice(proxy_service)
		}

		if err := interceptor_srv.Start(); err != nil {
			cleanup()
			return fmt.Errorf("failed to start proxy service: %w", err)
		}
		color.Green(fmt.Sprintf("Proxy service started successfully, address: %v", http_service_url(interceptor_srv.Addr())))

		if !buildtags.UsingSunnyNet {
			if interceptor_srv.ProxyTun() {
				color.Green("TUN mode enabled, traffic will be auto-forwarded through virtual NIC")
				color.Green("Please open the page you want to download")
			} else if !interceptor_srv.ProxySetSystem() {
				color.Yellow(fmt.Sprintf("System proxy is not set, please forward traffic to %v via software", interceptor_srv.Addr()))
				color.Yellow("Open the page to download after setting the proxy")
			} else {
				if proxy_warning != "" {
					color.Yellow("Warning: " + proxy_warning)
				}
				if proxy_service != "" {
					color.Green(fmt.Sprintf("System proxy for network service %v has been set to the proxy service address", proxy_service))
				} else {
					color.Green("System proxy has been set to the proxy service address")
				}
				color.Green("Please open the page you want to download")
				has_changed := false
				expected_addr := interceptor_srv.Addr()
				go func() {
					ticker := time.NewTicker(10 * time.Second)
					defer ticker.Stop()
					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							if !interceptor_srv.ProxySetSystem() {
								has_changed = false
								continue
							}
							// Poll the same network service the proxy was written to; with an empty
							// Device this would inspect the fallback service instead and never notice
							// a change.
							cur, err := system.FetchCurProxy(system.ProxySettings{Device: interceptor_srv.ProxyDevice()})
							if err != nil || cur == nil {
								continue
							}
							if cur.Hostname+":"+cur.Port != expected_addr {
								if !has_changed {
									color.Red("\nSystem proxy has been modified, please restart the downloader")
								}
								has_changed = true
							}
						}
					}
				}()
			}
		}
	}

	fmt.Println("\nPress Ctrl+C to exit...")
	<-ctx.Done()
	cleanup()
	return nil
}

func http_service_url(addr string) string {
	return "http://" + addr
}

func publish_registered_adapter_statuses(bus *events.Bus) {
	if bus == nil {
		return
	}
	for _, descriptor := range adapter.StatusDescriptors() {
		bus.Publish(events.PlatformStatusChanged{
			Platform:  descriptor.Platform,
			Key:       descriptor.Key,
			Name:      descriptor.Name,
			Status:    "unavailable",
			Available: false,
			Reason:    "等待 adapter 状态上报",
		})
	}
}

func new_scraper_platform_checker() services.ScraperPlatformChecker {
	return func(platform_id string, _ string) error {
		if adapter.Get(platform_id) == nil {
			return fmt.Errorf("未注册的平台 adapter: %s", platform_id)
		}
		return nil
	}
}

func download_task_event_data(event hermes.EventType, data hermes.EventData) (int, *hermes.TaskProgress, []hermes.TaskFinishedResource, bool) {
	switch event {
	case hermes.EventCreated:
		event_data, ok := data.(hermes.TaskCreatedEventData)
		return event_data.TaskID, nil, nil, ok
	case hermes.EventFinished:
		event_data, ok := data.(hermes.TaskFinishedEventData)
		return event_data.TaskID, nil, event_data.Resources, ok
	case hermes.EventFailed:
		event_data, ok := data.(hermes.TaskFailedEventData)
		return event_data.TaskID, nil, nil, ok
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
