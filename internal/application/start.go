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
	"github.com/ltaoo/velo"
	"github.com/pterm/pterm"
	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	_ "wx_channel/internal/adapter/builtin"
	"wx_channel/internal/api"
	"wx_channel/internal/buildtags"
	"wx_channel/internal/config"
	"wx_channel/internal/database"
	"wx_channel/internal/interceptor"
	"wx_channel/internal/webassets"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/configapi"
	"wx_channel/pkg/events"
	"wx_channel/pkg/hermes"
	"wx_channel/pkg/hermes/protocol"
	"wx_channel/pkg/system"
)

var start_proxy_config_items = []configapi.Item{
	configapi.Item{
		Key:         "proxy.device",
		Type:        configapi.TypeString,
		Default:     "",
		Description: "代理服务使用的网络设备",
		Title:       "代理网络设备",
		Group:       "Proxy",
		Reload:      configapi.ReloadProcess,
	},
	configapi.Item{
		Key:         "proxy.system",
		Type:        configapi.TypeBool,
		Default:     true,
		Description: "是否设置系统代理为代理服务",
		Title:       "设置系统代理",
		Group:       "Proxy",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "proxy.hostname",
		Type:        configapi.TypeString,
		Default:     "127.0.0.1",
		Description: "代理服务的主机名",
		Title:       "代理主机",
		Group:       "Proxy",
		Reload:      configapi.ReloadProcess,
	},
	configapi.Item{
		Key:         "proxy.port",
		Type:        configapi.TypeInt,
		Default:     2023,
		Description: "代理服务的端口",
		Title:       "代理端口",
		Group:       "Proxy",
		Reload:      configapi.ReloadProcess,
	},
	configapi.Item{
		Key:         "proxy.tcpRelay.enabled",
		Type:        configapi.TypeBool,
		Default:     false,
		Description: "是否启用 TCP relay，用于接收 iptables/nftables 透明重定向的原始 TCP 流量",
		Title:       "启用 TCP Relay",
		Group:       "Proxy",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "proxy.tcpRelay.hostname",
		Type:        configapi.TypeString,
		Default:     "127.0.0.1",
		Description: "TCP relay 监听主机名",
		Title:       "TCP Relay 主机",
		Group:       "Proxy",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "proxy.tcpRelay.port",
		Type:        configapi.TypeInt,
		Default:     9900,
		Description: "TCP relay 监听端口，必须与代理端口不同",
		Title:       "TCP Relay 端口",
		Group:       "Proxy",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "proxy.tun",
		Type:        configapi.TypeBool,
		Default:     false,
		Description: "启用 TUN 模式（网络层流量转发），开启后不会设置系统代理",
		Title:       "TUN 模式",
		Group:       "Proxy",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "proxy.defaultInterface",
		Type:        configapi.TypeString,
		Default:     "",
		Description: "TUN 模式下指定默认出口网卡名称，留空时自动检测",
		Title:       "默认网卡",
		Group:       "Proxy",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "proxy.skipInstallRootCert",
		Type:        configapi.TypeBool,
		Default:     false,
		Description: "是否跳过安装根证书（需要自行手动信任/导入证书）",
		Title:       "不安装根证书",
		Group:       "Proxy",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "proxy.upstreamProxy",
		Type:        configapi.TypeString,
		Default:     "",
		Description: "上游代理地址，用于转发所有请求到指定代理（如 http://127.0.0.1:7890）",
		Title:       "上游代理",
		Group:       "Proxy",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "cert.file",
		Type:        configapi.TypeFile,
		Default:     "",
		Description: "自定义证书文件绝对路径",
		Title:       "证书文件",
		Group:       "Proxy",
		Accept:      ".pem,.cer,.crt,.key",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "cert.key",
		Type:        configapi.TypeFile,
		Default:     "",
		Description: "自定义私钥文件绝对路径",
		Title:       "私钥文件",
		Group:       "Proxy",
		Accept:      ".pem,.key",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "cert.name",
		Type:        configapi.TypeString,
		Default:     "Echo",
		Description: "自定义证书名称",
		Title:       "证书名称",
		Group:       "Proxy",
		Reload:      configapi.ReloadHot,
	},
}

var start_download_config_items = []configapi.Item{
	configapi.Item{
		Key:         "download.dir",
		Type:        configapi.TypeString,
		Default:     "%UserDownloads%",
		Description: "指定下载目录",
		Title:       "下载目录",
		Group:       "Download",
		Reload:      configapi.ReloadProcess,
	},
	configapi.Item{
		Key:         "download.filenameTemplate",
		Type:        configapi.TypeString,
		Default:     "{{filename}}_{{spec}}",
		Description: "用于配置下载文件的名称，支持 {{filename}} 和 {{spec}} 等变量",
		Title:       "文件名模板",
		Group:       "Download",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "download.playDoneAudio",
		Type:        configapi.TypeBool,
		Default:     true,
		Description: "下载完成时是否播放完成音效",
		Title:       "播放完成音效",
		Group:       "Download",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "download.maxRunning",
		Type:        configapi.TypeInt,
		Default:     3,
		Description: "同时运行的最大下载任务数",
		Title:       "并发下载数",
		Group:       "Download",
		Reload:      configapi.ReloadComponent,
	},
	configapi.Item{
		Key:         "download.hooksScript",
		Type:        configapi.TypeString,
		Default:     "",
		Description: "下载任务钩子脚本路径",
		Title:       "钩子脚本",
		Group:       "Download",
		Reload:      configapi.ReloadComponent,
	},
	configapi.Item{
		Key:         "download.remoteServer.enabled",
		Type:        configapi.TypeBool,
		Default:     false,
		Description: "是否通过远程服务下载",
		Title:       "启用远程下载",
		Group:       "Download",
		Reload:      configapi.ReloadComponent,
	},
	configapi.Item{
		Key:         "download.remoteServer.protocol",
		Type:        configapi.TypeSelect,
		Default:     "http",
		Options:     []string{"http", "https"},
		Description: "远程下载服务协议",
		Title:       "远程服务协议",
		Group:       "Download",
		Reload:      configapi.ReloadComponent,
	},
	configapi.Item{
		Key:         "download.remoteServer.hostname",
		Type:        configapi.TypeString,
		Default:     "",
		Description: "远程下载服务主机名",
		Title:       "远程服务主机",
		Group:       "Download",
		Reload:      configapi.ReloadComponent,
	},
	configapi.Item{
		Key:         "download.remoteServer.port",
		Type:        configapi.TypeInt,
		Default:     2022,
		Description: "远程下载服务端口",
		Title:       "远程服务端口",
		Group:       "Download",
		Reload:      configapi.ReloadComponent,
	},
}

// StartDatabaseConfig is the database connection required while booting the
// application. Database changes are boot-only and produce a new value on the
// next StartConfig construction.
type StartDatabaseConfig struct {
	Type     string
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	Path     string
}

// StartConfig is the complete set of resolved parameters needed to start the
// application. It is derived once from configapi and is not a configuration
// loader or runtime configuration store.
type StartConfig struct {
	Version              string
	Mode                 string
	RootDir              string
	WorkDir              string
	ConfigFilePath       string
	ConfigFileExists     bool
	Database             StartDatabaseConfig
	GlobalScriptPath     string
	GlobalScriptContent  string
	ContentScriptPath    string
	ContentScriptContent string
}

// Start initializes and runs the local admin, API, and interceptor services.
func Start(provider *config.Config, logger *zerolog.Logger) {
	if logger == nil {
		color.Red("Application logger is not initialized")
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	start_cfg, err := NewStartConfig(provider)
	if err != nil {
		logger.Error().Err(err).Msg("application configuration initialization failed")
		color.Red(fmt.Sprintf("Application configuration initialization failed, %s\n\n", err))
		return
	}

	fmt.Printf("\nv%v\n", start_cfg.Version)
	fmt.Printf("Feedback/Issues https://github.com/ltaoo/wx_channels_download/issues\n\n")

	b := velo.NewApp(&velo.VeloAppOpt{Mode: velo.ModeHttp})
	if err := b.UseDatabase(&velo.DBConfig{
		Type:     velo.DBType(start_cfg.Database.Type),
		Host:     start_cfg.Database.Host,
		Port:     start_cfg.Database.Port,
		User:     start_cfg.Database.User,
		Password: start_cfg.Database.Password,
		Name:     start_cfg.Database.Name,
		Path:     start_cfg.Database.Path,
	}, &database.Migrations); err != nil {
		color.Red(fmt.Sprintf("Database initialization failed, %s\n\n", err))
		os.Exit(0)
		return
	}
	if err := provider.AttachDatabaseSource(ctx, b.DB); err != nil {
		color.Red(fmt.Sprintf("Runtime configuration database initialization failed, %s\n\n", err))
		return
	}

	runtime_config := configapi.Runtime{
		Version:              start_cfg.Version,
		Mode:                 start_cfg.Mode,
		RootDir:              start_cfg.RootDir,
		WorkDir:              start_cfg.WorkDir,
		GlobalScriptContent:  start_cfg.GlobalScriptContent,
		ContentScriptContent: start_cfg.ContentScriptContent,
	}
	api_cfg, err := api.NewAPIConfig(api.APIConfigSource{
		Provider: provider,
		Runtime:  runtime_config,
	})
	if err != nil {
		color.Red(fmt.Sprintf("Failed to initialize API config, %s\n\n", err))
		return
	}
	static_assets := webassets.NewRegistry()
	bus := events.NewBus()
	interceptor_srv, err := interceptor.NewInterceptorServer(interceptor.ServerDeps{
		ConfigProvider: provider,
		Runtime:        runtime_config,
		CertificateLoader: func() *certificate.CertFileAndKeyFile {
			return config.LoadCertFiles(provider)
		},
	})
	if err != nil {
		color.Red(fmt.Sprintf("Failed to initialize interceptor config, %s\n\n", err))
		return
	}
	interceptor_srv.SetLogger(logger)
	interceptor_srv.SubscribeEvents(bus)

	table_data := pterm.TableData{{"Item", "Path"}, {"Work Dir", start_cfg.WorkDir}, {"Data Path", start_cfg.Database.Path}}
	if start_cfg.ConfigFilePath != "" {
		table_data = append(table_data, []string{"Config File", start_cfg.ConfigFilePath})
	}
	table_data = append(table_data, []string{"Download Dir", api_cfg.DownloadDir})
	// --- Hook manager ---
	hook_manager := hermes.NewHookManager()
	if script := api_cfg.HooksScript; script != "" {
		if err := hook_manager.Load(script); err != nil {
			logger.Warn().Err(err).Str("path", script).Msg("Failed to load hook script")
		}
	} else {
		convention_path := filepath.Join(start_cfg.WorkDir, "hooks.js")
		if _, err := os.Stat(convention_path); err == nil {
			if err := hook_manager.Load(convention_path); err != nil {
				logger.Warn().Err(err).Str("path", convention_path).Msg("Failed to load hook script")
			}
		}
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
			SpeedLimit:       100 * 1024, // 100kb/s
		},
	})
	downloader.RegisterProtocol(protocol.NewHTTPDriver())
	downloader.RegisterProtocol(protocol.NewStreamDriver())
	downloader.RegisterProtocol(protocol.NewInlineDriver())
	downloader.SetHooks(hook_manager)
	downloader.SetPostprocessor(adapter.NewPlatformPostprocessor(b.DB, *logger, api_cfg.DownloadDir))

	// --- API service ---
	api_srv := api.NewAPIServer(api_cfg, provider, logger, b.DB, static_assets, downloader, hook_manager)
	api_srv.SubscribeEvents(bus)
	// admin_srv := admin.NewAdminServer(cfg, b, bus)
	if start_cfg.GlobalScriptPath != "" {
		table_data = append(table_data, []string{"Global Script", start_cfg.GlobalScriptPath})
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
			StaticAssets:   static_assets,
			Routes:         api_srv.APIClient,
			Interceptor:    interceptor_srv.Interceptor,
			DB:             b.DB,
			Logger:         logger,
			Bus:            bus,
			ConfigProvider: provider,
			Runtime:        runtime_config,
			BasePath:       api_cfg.DownloadDir,
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
		if err := interceptor_srv.Close(); err != nil {
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
