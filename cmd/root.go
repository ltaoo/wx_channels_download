package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/fatih/color"
	"github.com/ltaoo/echo"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"wx_channel/config"
	"wx_channel/internal/api"
	"wx_channel/internal/interceptor"
	"wx_channel/internal/manager"
	"wx_channel/pkg/certificate"
)

var (
	Version   string
	device    string
	hostname  string
	port      int
	debug     bool
	CertFiles *certificate.CertFileAndKeyFile
	Settings  *interceptor.InterceptorSettings
	Cfg       *config.Config
)

var root_cmd = &cobra.Command{
	Use:   "wx_video_download",
	Short: "启动下载程序",
	Long:  "\n启动后将对网络请求进行代理，在微信视频号详情页面注入下载按钮",
	PreRun: func(cmd *cobra.Command, args []string) {
	},
	Run: func(cmd *cobra.Command, args []string) {
		Settings.Version = Version
		Settings.DebugShowError = viper.GetBool("debug.error")
		Settings.ProxySetSystem = viper.GetBool("proxy.system")
		Settings.ProxyServerHostname = viper.GetString("proxy.hostname")
		Settings.ProxyServerPort = viper.GetInt("proxy.port")
		root_command(Settings)
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

func Execute(app_ver string, cert *certificate.CertFileAndKeyFile, cfg *config.Config, settings *interceptor.InterceptorSettings) error {
	cobra.MousetrapHelpText = ""

	Version = app_ver
	CertFiles = cert
	Settings = settings
	Cfg = cfg

	return root_cmd.Execute()
}
func Register(cmd *cobra.Command) {
	root_cmd.AddCommand(cmd)
}

type RootCommandArg struct {
}

func root_command(args *interceptor.InterceptorSettings) {
	c, err := config.New()
	if err != nil {
		return
	}
	interceptor.SetDefaultSettings(c)
	err = c.LoadConfig()
	if err != nil {
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Printf("\nv%v\n", Version)
	fmt.Printf("问题反馈 https://github.com/ltaoo/wx_channels_download/issues\n\n")
	if args.FilePath != "" {
		fmt.Printf("配置文件 %s\n", args.FilePath)
	}
	if script_byte := viper.Get("globalUserScript"); script_byte != nil {
		fmt.Printf("存在全局脚本\n\n")
	}
	mgr := manager.NewServerManager()
	interceptor_srv := interceptor.NewInterceptorServer(args, CertFiles)
	interceptor_srv.Interceptor.AddPostPlugin(&echo.Plugin{
		Match: "api.channels.qq.com",
		Target: &echo.TargetConfig{
			Protocol: "http",
			Host:     args.APIServerHostname,
			Port:     args.APIServerPort,
		},
	})
	mgr.RegisterServer(interceptor_srv)
	api_settings := api.NewAPISettings(Cfg)
	api_srv := api.NewAPIServer(api_settings)
	mgr.RegisterServer(api_srv)
	interceptor_srv.Interceptor.FrontendVariables["downloadMaxRunning"] = api_settings.MaxRunning
	interceptor_srv.Interceptor.FrontendVariables["downloadDir"] = api_settings.DownloadDir

	cleanup := func() {
		fmt.Printf("\n正在关闭服务...\n")
		if err := mgr.StopServer("interceptor"); err != nil {
			fmt.Printf("⚠️ 关闭代理服务失败: %v\n", err)
		}
		if err := mgr.StopServer("api"); err != nil {
			fmt.Printf("⚠️ 关闭API服务失败: %v\n", err)
		}
		color.Green("服务已关闭")
	}
	if err := mgr.StartServer("api"); err != nil {
		fmt.Printf("ERROR 启动API服务失败: %v\n", err.Error())
		cleanup()
		os.Exit(1)
	}
	color.Green(fmt.Sprintf("API服务启动成功, 地址: %v", api_srv.Addr()))
	if err := mgr.StartServer("interceptor"); err != nil {
		fmt.Printf("ERROR 启动代理服务失败: %v\n", err.Error())
		cleanup()
		os.Exit(1)
	}
	color.Green("代理服务启动成功")

	if !args.ProxySetSystem {
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
