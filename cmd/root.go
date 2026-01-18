package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"wx_channel/internal/api"
	"wx_channel/internal/config"
	"wx_channel/internal/interceptor"
	"wx_channel/internal/interceptor/proxy"
	"wx_channel/internal/manager"
	"wx_channel/internal/officialaccount"
	"wx_channel/pkg/certificate"
)

var (
	Version   string
	Cfg       *config.Config
	CertFiles *certificate.CertFileAndKeyFile
	device    string
	hostname  string
	port      int
	debug     bool
)

var root_cmd = &cobra.Command{
	Use:   "wx_video_download",
	Short: "启动下载程序",
	Long:  "\n启动后将对网络请求进行代理，在微信视频号详情页面注入下载按钮",
	PreRun: func(cmd *cobra.Command, args []string) {
	},
	Run: func(cmd *cobra.Command, args []string) {
		root_command(Cfg)
	},
}

func init() {
	root_cmd.PersistentFlags().StringVar(&device, "dev", "", "代理服务器网络设备")
	root_cmd.PersistentFlags().StringVar(&hostname, "hostname", "127.0.0.1", "代理服务器主机名")
	root_cmd.PersistentFlags().IntVar(&port, "port", 2023, "代理服务器端口")
	root_cmd.PersistentFlags().BoolVar(&debug, "debug", false, "是否开启调试")

	viper.BindPFlag("debug.error", root_cmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("proxy.hostname", root_cmd.PersistentFlags().Lookup("hostname"))
	viper.BindPFlag("proxy.port", root_cmd.PersistentFlags().Lookup("port"))
}

func Execute(cert *certificate.CertFileAndKeyFile, cfg *config.Config) error {
	cobra.MousetrapHelpText = ""

	Version = cfg.Version
	CertFiles = cert
	Cfg = cfg

	return root_cmd.Execute()
}
func Register(cmd *cobra.Command) {
	root_cmd.AddCommand(cmd)
}

type RootCommandArg struct {
}

func root_command(cfg *config.Config) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Printf("\nv%v\n", cfg.Version)
	fmt.Printf("问题反馈 https://github.com/ltaoo/wx_channels_download/issues\n\n")

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zerolog.TimeFieldFormat = time.RFC3339Nano
	log_filepath := filepath.Join(cfg.RootDir, "app.log")
	log_file, err := os.OpenFile(log_filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		color.Red(fmt.Sprintf("创建日志文件失败，%s\n\n", err))
		return
	}
	defer log_file.Close()
	logger := zerolog.New(log_file).With().Timestamp().Logger()
	log.Logger = log.Output(os.Stderr)
	log.Logger = log.With().
		Str("service", "WechatHelper").
		Str("version", cfg.Version).
		Logger()

	if cfg.FullPath != "" {
		fmt.Printf("配置文件 %s\n", color.New(color.Underline).Sprint(cfg.FullPath))
	}
	api_cfg := api.NewAPIConfig(Cfg, false)
	interceptor_cfg := interceptor.NewInterceptorSettings(cfg)
	official_cfg := officialaccount.NewOfficialAccountConfig(Cfg, false)
	if script_byte := interceptor_cfg.InjectGlobalScript; script_byte != "" {
		fmt.Printf("全局脚本 %s\n", color.New(color.Underline).Sprint(interceptor_cfg.InjectGlobalScriptFilepath))
	}
	mgr := manager.NewServerManager()
	interceptor_srv := interceptor.NewInterceptorServer(interceptor_cfg, CertFiles)
	if api_cfg.RemoteServerEnabled {
		interceptor_srv.Interceptor.AddPostPlugin(&proxy.Plugin{
			Match: "api.weixin.qq.com",
			Target: &proxy.TargetConfig{
				Protocol: api_cfg.RemoteServerProtocol,
				Host:     api_cfg.RemoteServerHostname,
				Port:     api_cfg.RemoteServerPort,
			},
		})
		interceptor_srv.Interceptor.AddVariable("remoteServerEnabled", api_cfg.RemoteServerEnabled)
		interceptor_srv.Interceptor.AddVariable("remoteServerProtocol", api_cfg.RemoteServerProtocol)
		interceptor_srv.Interceptor.AddVariable("remoteServerHostname", api_cfg.RemoteServerHostname)
		interceptor_srv.Interceptor.AddVariable("remoteServerPort", api_cfg.RemoteServerPort)
	} else {
		interceptor_srv.Interceptor.AddPostPlugin(&proxy.Plugin{
			Match: "api.weixin.qq.com",
			Target: &proxy.TargetConfig{
				Protocol: interceptor_cfg.APIServerProtocol,
				Host:     interceptor_cfg.APIServerHostname,
				Port:     interceptor_cfg.APIServerPort,
			},
		})
	}
	if !official_cfg.Disabled {
		interceptor_srv.Interceptor.AddPostPlugin(officialaccount.CreateOfficialAccountInterceptorPlugin(official_cfg, interceptor.Assets))
		interceptor_srv.Interceptor.AddPostPlugin(&proxy.Plugin{
			Match: "official.weixin.qq.com",
			Target: &proxy.TargetConfig{
				Protocol: official_cfg.RemoteServerProtocol,
				Host:     official_cfg.RemoteServerHostname,
				Port:     official_cfg.RemoteServerPort,
			},
		})
	}
	mgr.RegisterServer(interceptor_srv)
	interceptor_cfg.DownloadMaxRunning = api_cfg.MaxRunning
	if api_cfg.RemoteServerEnabled {
		fmt.Printf("启用了远端服务，视频将下载至远端服务器目录\n\n")
	} else {
		fmt.Printf("下载目录 %s\n\n", color.New(color.Underline).Sprint(api_cfg.DownloadDir))
	}
	api_addr := fmt.Sprintf("%s:%d", api_cfg.Hostname, api_cfg.Port)
	l, err := net.Listen("tcp", api_addr)
	if err != nil {
		color.Red(fmt.Sprintf("启动API服务失败，%s 被占用\n\n", api_addr))
		os.Exit(0)
		return
	}
	l.Close()
	api_srv := api.NewAPIServer(api_cfg, &logger)
	mgr.RegisterServer(api_srv)
	interceptor_srv.Interceptor.AddVariable("downloadMaxRunning", api_cfg.MaxRunning)
	interceptor_srv.Interceptor.AddVariable("downloadDir", api_cfg.DownloadDir)

	cleanup := func() {
		fmt.Printf("\n正在关闭服务...\n")
		if err := mgr.StopServer("interceptor"); err != nil {
			color.Red(fmt.Sprintf("⚠️ 关闭代理服务失败: %v\n", err))
		}
		if err := mgr.StopServer("api"); err != nil {
			color.Red(fmt.Sprintf("⚠️ 关闭API服务失败: %v\n", err))
		}
		color.Green("服务已关闭")
	}

	if err := mgr.StartServer("api"); err != nil {
		color.Red(fmt.Sprintf("ERROR 启动API服务失败: %v\n", err.Error()))
		cleanup()
		os.Exit(0)
	}
	color.Green(fmt.Sprintf("API服务启动成功, 地址: %v", api_srv.Addr()))
	if err := mgr.StartServer("interceptor"); err != nil {
		color.Red(fmt.Sprintf("ERROR 启动代理服务失败: %v\n", err.Error()))
		cleanup()
		os.Exit(0)
	}
	color.Green("代理服务启动成功")

	if !interceptor_cfg.ProxySetSystem {
		color.Red(fmt.Sprintf("当前未设置系统代理,请通过软件将流量转发至 %v", interceptor_srv.Addr()))
		color.Red("设置成功后再打开视频号页面下载")
	} else {
		color.Green(fmt.Sprintf("已修改系统代理为 %v", interceptor_srv.Addr()))
		color.Green("请打开需要下载的视频号页面进行下载")
	}
	fmt.Println("\n按 Ctrl+C 退出...")
	<-ctx.Done()
	cleanup()
}
