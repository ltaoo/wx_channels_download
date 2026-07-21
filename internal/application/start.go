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
	velodatabase "github.com/ltaoo/velo/database"

	webchannels "wx_channel/internal/adapter/wxchannels"
	"wx_channel/internal/admin"
	"wx_channel/internal/api"
	"wx_channel/internal/buildtags"
	"wx_channel/internal/config"
	"wx_channel/internal/database"
	"wx_channel/internal/events"
	"wx_channel/internal/interceptor"
	"wx_channel/internal/webassets"
	"wx_channel/pkg/scraper/wxchannels"
	"wx_channel/pkg/scraper/wxmp"
	"wx_channel/pkg/system"
)

// Start initializes and runs the local admin, API, and interceptor services.
func Start(cfg *config.Config) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	certFiles := config.LoadCertFiles()

	fmt.Printf("\nv%v\n", cfg.Version)
	fmt.Printf("问题反馈 https://github.com/ltaoo/wx_channels_download/issues\n\n")

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.TimeFieldFormat = time.RFC3339Nano
	log_filepath := filepath.Join(cfg.WorkDir, "app.log")
	log_file, err := os.OpenFile(log_filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		color.Red(fmt.Sprintf("创建日志文件失败，%s\n\n", err))
		return
	}
	defer log_file.Close()
	logger := zerolog.New(log_file).With().Timestamp().Logger()
	log.Logger = zerolog.New(zerolog.MultiLevelWriter(os.Stderr, log_file)).With().
		Timestamp().
		Str("service", "box").
		Str("version", cfg.Version).
		Logger()

	b := velo.NewApp(&velo.VeloAppOpt{Mode: velo.ModeHttp})
	dbCfg := &velodatabase.DBConfig{Type: velodatabase.DBTypeSQLite, Path: cfg.DBPath}
	if err := b.UseDatabase(dbCfg, &database.Migrations); err != nil {
		color.Red(fmt.Sprintf("数据库初始化失败，%s\n\n", err))
		os.Exit(0)
		return
	}

	api_cfg := api.NewAPIConfig(cfg, false)
	staticAssets := webassets.NewRegistry()
	if err := wxchannels.RegisterStaticAssets(staticAssets); err != nil {
		color.Red(fmt.Sprintf("注册视频号静态资源失败: %v", err))
		return
	}
	if err := wxmp.RegisterStaticAssets(staticAssets); err != nil {
		color.Red(fmt.Sprintf("注册公众号静态资源失败: %v", err))
		return
	}
	bus := events.NewBus()
	interceptor_srv := interceptor.NewInterceptorServer(cfg, certFiles)
	interceptor_srv.SetLog(log_file)
	interceptor_srv.SubscribeEvents(bus)

	tableData := pterm.TableData{{"项目", "路径"}, {"工作目录", cfg.WorkDir}, {"数据路径", cfg.DBPath}}
	if cfg.FullPath != "" {
		tableData = append(tableData, []string{"配置文件", cfg.FullPath})
	}
	if api_cfg.RemoteServerEnabled {
		tableData = append(tableData, []string{"下载目录", "远端服务器"})
	} else {
		tableData = append(tableData, []string{"下载目录", api_cfg.DownloadDir})
	}
	channels_interceptor_cfg := webchannels.NewConfig(cfg)
	channels_interceptor_cfg.RegisterPlugins(interceptor_srv.Interceptor, b.DB, logger, bus)
	if channels_interceptor_cfg.HasGlobalScript() {
		tableData = append(tableData, []string{"全局脚本", channels_interceptor_cfg.GlobalScriptFilepath()})
	}
	pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
	fmt.Println()

	api_srv := api.NewAPIServer(api_cfg, &logger, b.DB, staticAssets)
	channelsWebsocketRoutes := webchannels.NewWebsocketRoutes(api_cfg.ChannelsRefreshInterval, b.DB)
	channelsWebsocketRoutes.RegisterRoutes(api_srv.APIClient)
	api_srv.SubscribeEvents(bus)
	api_srv.APIClient.SubscribeEvents(bus)
	admin_srv := admin.NewAdminServer(cfg, b, bus)

	cleanup := func() {
		fmt.Printf("\n正在关闭下载器...\n")
		channelsWebsocketRoutes.Stop()
		if err := interceptor_srv.Stop(); err != nil {
			color.Red(fmt.Sprintf("⚠️ 关闭代理服务失败: %v\n", err))
		}
		if err := api_srv.Stop(); err != nil {
			color.Red(fmt.Sprintf("⚠️ 关闭API服务失败: %v\n", err))
		}
		if err := admin_srv.Stop(); err != nil {
			color.Red(fmt.Sprintf("⚠️ 关闭GUI/Admin服务失败: %v\n", err))
		}
		color.Green("下载器已关闭")
	}

	if err := admin_srv.Start(); err != nil {
		color.Red(fmt.Sprintf("ERROR 启动GUI/Admin服务失败: %v\n", err))
		cleanup()
		os.Exit(0)
		return
	}
	color.Green(fmt.Sprintf("GUI/Admin服务启动成功, 地址: %v", admin_srv.Addr()))
	if err := api_srv.Start(); err != nil {
		color.Red(fmt.Sprintf("ERROR 启动API服务失败: %v\n", err))
		cleanup()
		os.Exit(0)
		return
	}
	color.Green(fmt.Sprintf("API服务启动成功, 地址: %v", api_srv.Addr()))
	if err := interceptor_srv.Start(); err != nil {
		color.Red(fmt.Sprintf("ERROR 启动代理服务失败: %v\n", err))
		cleanup()
		os.Exit(0)
		return
	}
	color.Green(fmt.Sprintf("代理服务启动成功, 地址: %v", interceptor_srv.Addr()))

	if !buildtags.UsingSunnyNet {
		if interceptor_srv.ProxyTun() {
			color.Green("已启用 TUN 模式，流量将通过虚拟网卡自动转发")
			color.Green("请打开需要下载的视频号页面进行下载")
		} else if !interceptor_srv.ProxySetSystem() {
			color.Red(fmt.Sprintf("当前未设置系统代理,请通过软件将流量转发至 %v", interceptor_srv.Addr()))
			color.Red("设置成功后再打开视频号页面下载")
		} else {
			color.Green("已修改系统代理为代理服务地址")
			color.Green("请打开需要下载的视频号页面进行下载")
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
								color.Red("\n系统代理已被修改，请重新启动下载器")
							}
							has_changed = true
						}
					}
				}
			}()
		}
	}

	fmt.Println("\n按 Ctrl+C 退出...")
	<-ctx.Done()
	cleanup()
}
