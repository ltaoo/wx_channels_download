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
	"github.com/pterm/pterm"

	"github.com/ltaoo/velo"

	"wx_channel/frontend"
	"wx_channel/internal/adapter"
	_ "wx_channel/internal/adapter/builtin"
	"wx_channel/internal/api"
	"wx_channel/internal/buildtags"
	"wx_channel/internal/config"
	"wx_channel/internal/database"
	"wx_channel/internal/events"
	"wx_channel/internal/interceptor"
	"wx_channel/internal/services"
	"wx_channel/internal/webassets"
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
	cfg.LogGlobalScriptPath()

	b := velo.NewApp(&velo.VeloAppOpt{Mode: velo.ModeHttp})
	if err := b.Migrate(&velo.VeloDatabaseOpt{DBType: velo.DBTypeSQLite, DBPath: cfg.DBPath, Migrations: &database.Migrations}); err != nil {
		return fmt.Errorf("database initialization failed: %w", err)
	}

	api_cfg := api.NewAPIConfig(cfg)
	static_assets := webassets.NewRegistry()
	if cfg.GlobalScriptPath == "" {
		logger.Info().
			Str("file", "internal/application/start.go").
			Msg("global script path is empty; skipping global script asset registration")
	} else {
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
		if err := hook_manager.Load(script); err != nil {
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
	} else {
		logger.Info().
			Str("file", "internal/application/start.go").
			Str("resolved_config_path", configured_hook_path).
			Msg("download hook script not found")
	}

	// --- Database store ---
	task_store := database.NewDBTaskStore(b.DB, logger)

	// --- Download engine ---
	downloader := hermes.New(hermes.HermesNewConfig{
		Store:  task_store,
		Logger: logger,
		Config: hermes.HermesEngineConfig{
			MaxConcurrent:    api_cfg.MaxRunning,
			FilenameTemplate: api_cfg.FilenameTemplate,
			BasePath:         api_cfg.DownloadDir,
			// SpeedLimit:       500 * 1024,
		},
	})
	downloader.RegisterProtocol(protocol.NewHTTPDriver())
	downloader.RegisterProtocol(protocol.NewStreamDriver())
	downloader.RegisterProtocol(protocol.NewInlineDriver())
	downloader.SetHooks(hook_manager)
	downloader.SetPostprocessor(adapter.NewPlatformPostprocessor(b.DB, *logger, api_cfg.DownloadDir))

	// --- API service ---
	api_srv := api.NewAPIServer(api_cfg, logger, b.DB, static_assets, downloader, hook_manager)
	api_srv.SubscribeEvents(bus)
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
		handle, err := runtime_adapter.RegisterRuntime(adapter.RuntimeDeps{
			StaticAssets: static_assets,
			Routes:       api_srv.APIClient,
			Interceptor:  interceptor_srv.Interceptor,
			DB:           b.DB,
			Logger:       logger,
			Bus:          bus,
			Config:       cfg,
		})
		if err != nil {
			return fmt.Errorf("failed to register platform %s: %w", platform_id, err)
		}
		adapter_handles = append(adapter_handles, handle)
	}

	var cleanup_once sync.Once
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
			for i := len(adapter_handles) - 1; i >= 0; i-- {
				adapter_handles[i].Stop()
			}
			if api_started {
				if err := api_srv.Stop(); err != nil {
					color.Red(fmt.Sprintf("Failed to stop API service: %v\n", err))
				}
			}
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
	if err := api_srv.Start(); err != nil {
		cleanup()
		return fmt.Errorf("failed to start API service: %w", err)
	}
	api_started = true
	color.Green(fmt.Sprintf("API service started successfully, address: %v", api_srv.Addr()))

	interceptor_start_attempted = true
	unregister_console_close, err := registerConsoleCloseHandler(stop)
	if err != nil {
		interceptor_start_attempted = false
		cleanup()
		return fmt.Errorf("failed to register console close handler: %w", err)
	}
	defer unregister_console_close()

	if err := interceptor_srv.Start(); err != nil {
		cleanup()
		return fmt.Errorf("failed to start proxy service: %w", err)
	}
	color.Green(fmt.Sprintf("Proxy service started successfully, address: %v", interceptor_srv.Addr()))

	if !buildtags.UsingSunnyNet {
		if interceptor_srv.ProxyTun() {
			color.Green("TUN mode enabled, traffic will be auto-forwarded through virtual NIC")
			color.Green("Please open the page you want to download")
		} else if !interceptor_srv.ProxySetSystem() {
			color.Red(fmt.Sprintf("System proxy is not set, please forward traffic to %v via software", interceptor_srv.Addr()))
			color.Red("Open the page to download after setting the proxy")
		} else {
			color.Green("System proxy has been set to the proxy service address")
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
						cur, err := system.FetchCurProxy(system.ProxySettings{})
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

	fmt.Println("\nPress Ctrl+C to exit...")
	<-ctx.Done()
	cleanup()
	return nil
}
