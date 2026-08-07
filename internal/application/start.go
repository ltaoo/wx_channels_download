package application

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/pterm/pterm"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ltaoo/velo"

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
func Start(cfg *config.Config) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Printf("\nv%v\n", cfg.Version)
	fmt.Printf("Feedback/Issues https://github.com/ltaoo/wx_channels_download/issues\n\n")

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.TimeFieldFormat = time.RFC3339Nano
	log_filepath := filepath.Join(cfg.WorkDir, "app.log")
	log_file, err := os.OpenFile(log_filepath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		color.Red(fmt.Sprintf("Failed to create log file, %s\n\n", err))
		return
	}
	defer log_file.Close()
	logger := zerolog.New(log_file).With().Timestamp().Logger()
	log.Logger = zerolog.New(zerolog.MultiLevelWriter(os.Stderr, log_file)).With().
		Timestamp().
		Str("version", cfg.Version).
		Logger()

	b := velo.NewApp(&velo.VeloAppOpt{Mode: velo.ModeHttp})
	if err := b.Migrate(&velo.VeloDatabaseOpt{DBType: velo.DBTypeSQLite, DBPath: cfg.DBPath, Migrations: &database.Migrations}); err != nil {
		color.Red(fmt.Sprintf("Database initialization failed, %s\n\n", err))
		os.Exit(0)
		return
	}

	api_cfg := api.NewAPIConfig(cfg)
	static_assets := webassets.NewRegistry()
	bus := events.NewBus()
	cert_files := services.LoadCertFiles()
	interceptor_srv := interceptor.NewInterceptorServer(cfg, cert_files)
	interceptor_srv.SetLog(log_file)
	interceptor_srv.SubscribeEvents(bus)

	table_data := pterm.TableData{{"Item", "Path"}, {"Work Dir", cfg.WorkDir}, {"Data Path", cfg.DBPath}}
	if cfg.FullPath != "" {
		table_data = append(table_data, []string{"Config File", cfg.FullPath})
	}
	table_data = append(table_data, []string{"Download Dir", api_cfg.DownloadDir})
	// --- Hook manager ---
	hook_manager := hermes.NewHookManager()
	if script := api_cfg.HooksScript; script != "" {
		if err := hook_manager.Load(script); err != nil {
			logger.Warn().Err(err).Str("path", script).Msg("Failed to load hook script")
		}
	} else {
		convention_path := filepath.Join(cfg.WorkDir, "hooks.js")
		if _, err := os.Stat(convention_path); err == nil {
			if err := hook_manager.Load(convention_path); err != nil {
				logger.Warn().Err(err).Str("path", convention_path).Msg("Failed to load hook script")
			}
		}
	}

	// --- Database store ---
	task_store := database.NewDBTaskStore(b.DB, &logger)

	// --- Download engine ---
	downloader := hermes.New(hermes.HermesNewConfig{
		Store:  task_store,
		Logger: &logger,
		Config: hermes.HermesEngineConfig{
			MaxConcurrent:    api_cfg.MaxRunning,
			FilenameTemplate: api_cfg.FilenameTemplate,
			BasePath:         api_cfg.DownloadDir,
			SpeedLimit:       100 * 1024, // 100kb/s
		},
	})
	downloader.RegisterProtocol(protocol.NewHTTPDriver())
	downloader.RegisterProtocol(protocol.NewStreamDriver())
	downloader.RegisterProtocol(protocol.NewInlineDriver())
	downloader.SetHooks(hook_manager)
	downloader.SetPostprocessor(adapter.NewPlatformPostprocessor(b.DB, logger, api_cfg.DownloadDir))

	// --- API service ---
	api_srv := api.NewAPIServer(api_cfg, &logger, b.DB, static_assets, downloader, hook_manager)
	api_srv.SubscribeEvents(bus)
	// admin_srv := admin.NewAdminServer(cfg, b, bus)
	if cfg.GlobalScriptPath != "" {
		table_data = append(table_data, []string{"Global Script", cfg.GlobalScriptPath})
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
			Logger:       &logger,
			Bus:          bus,
			Config:       cfg,
		})
		if err != nil {
			color.Red(fmt.Sprintf("Failed to register platform %s: %v", platform_id, err))
			return
		}
		adapter_handles = append(adapter_handles, handle)
	}

	cleanup := func() {
		fmt.Printf("\nShutting down downloader...\n")
		for i := len(adapter_handles) - 1; i >= 0; i-- {
			adapter_handles[i].Stop()
		}
		if err := interceptor_srv.Stop(); err != nil {
			color.Red(fmt.Sprintf("Failed to stop proxy service: %v\n", err))
		}
		if err := api_srv.Stop(); err != nil {
			color.Red(fmt.Sprintf("Failed to stop API service: %v\n", err))
		}
		task_store.Shutdown()
		// if err := admin_srv.Stop(); err != nil {
		// 	color.Red(fmt.Sprintf("Failed to stop GUI/Admin service: %v\n", err))
		// }
		color.Green("Downloader has been shut down")
	}

	// if err := admin_srv.Start(); err != nil {
	// 	color.Red(fmt.Sprintf("ERROR Failed to start GUI/Admin service: %v\n", err))
	// 	cleanup()
	// 	os.Exit(0)
	// 	return
	// }
	// color.Green(fmt.Sprintf("GUI/Admin service started successfully, address: %v", admin_srv.Addr()))
	if err := api_srv.Start(); err != nil {
		color.Red(fmt.Sprintf("ERROR Failed to start API service: %v\n", err))
		cleanup()
		os.Exit(0)
		return
	}
	color.Green(fmt.Sprintf("API service started successfully, address: %v", api_srv.Addr()))
	if err := interceptor_srv.Start(); err != nil {
		color.Red(fmt.Sprintf("ERROR Failed to start proxy service: %v\n", err))
		cleanup()
		os.Exit(0)
		return
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
}
