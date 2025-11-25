package cmd

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/ltaoo/echo"
	"github.com/ltaoo/echo/plugin"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"wx_channel/config"
	"wx_channel/internal/application"
	"wx_channel/internal/handler"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/proxy"
)

var (
	Version        string
	device         string
	port           int
	debug          bool
	files          *application.BizFiles
	cert_file_name string
	cfg            *config.Config
)

var root_cmd = &cobra.Command{
	Use:   "wx_video_download",
	Short: "启动下载程序",
	Long:  "\n启动后将对网络请求进行代理，在微信视频号详情页面注入下载按钮",
	PreRun: func(cmd *cobra.Command, args []string) {
	},
	Run: func(cmd *cobra.Command, args []string) {
		root_command(RootCommandArg{
			Device:         device,
			Port:           viper.GetInt("proxy.port"),
			Debug:          debug,
			SetSystemProxy: viper.GetBool("proxy.system"),
			cfg:            cfg,
		})
	},
}

func init() {
	root_cmd.PersistentFlags().StringVar(&device, "dev", "", "代理服务器网络设备")
	root_cmd.PersistentFlags().IntVar(&port, "port", 2023, "代理服务器端口")
	root_cmd.PersistentFlags().BoolVar(&debug, "debug", false, "是否开启调试")

	viper.BindPFlag("proxy.port", root_cmd.PersistentFlags().Lookup("port"))
}

func Initialize(app_ver string, cert_file string, file *application.BizFiles, c *config.Config) {
	Version = app_ver
	cert_file_name = cert_file
	files = file
	port = c.Port
	cfg = c
}
func Execute() error {
	cobra.MousetrapHelpText = ""

	return root_cmd.Execute()
}
func Register(cmd *cobra.Command) {
	root_cmd.AddCommand(cmd)
}

type RootCommandArg struct {
	Device         string
	Port           int
	Debug          bool
	SetSystemProxy bool
	cfg            *config.Config
}

func root_command(args RootCommandArg) {
	_, cancel := context.WithCancel(context.Background())
	var server *http.Server
	signal_chan := make(chan os.Signal, 1)
	// Notify the signal channel on SIGINT (Ctrl+C) and SIGTERM
	signal.Notify(signal_chan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-signal_chan
		fmt.Printf("\n正在关闭下载服务...%v\n\n", sig)
		signal.Stop(signal_chan)
		if args.SetSystemProxy {
			arg := proxy.ProxySettings{
				Device:   args.Device,
				Hostname: "127.0.0.1",
				Port:     strconv.Itoa(args.Port),
			}
			err := proxy.DisableProxy(arg)
			if err != nil {
				fmt.Printf("⚠️ 关闭系统代理失败: %v\n", err)
			}
		}
		if server != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				fmt.Printf("⚠️ 代理服务器关闭失败: %v\n", err)
				// 如果优雅关闭失败，强制关闭
				server.Close()
			}
		}
		// 注意：cancel 在信号处理 goroutine 中调用，不需要 defer
		cancel()
		os.Exit(0)
	}()

	fmt.Printf("\nv%v\n", Version)
	fmt.Printf("问题反馈 https://github.com/ltaoo/wx_channels_download/issues\n\n")
	existing, err1 := certificate.CheckHasCertificate(cert_file_name)
	if err1 != nil {
		fmt.Printf("ERROR %v\n", err1.Error())
		fmt.Printf("按 Ctrl+C 退出...\n")
		select {}
	}
	if !existing {
		fmt.Printf("正在安装证书...\n")
		err := certificate.InstallCertificate(files.CertFile)
		time.Sleep(3 * time.Second)
		if err != nil {
			fmt.Printf("ERROR %v\n", err.Error())
			fmt.Printf("按 Ctrl+C 退出...\n")
			select {}
		}
	}
	biz := application.NewBiz(Version, files)
	echo, err := echo.NewEcho(files.CertFile, files.PrivateKeyFile)
	if err != nil {
		fmt.Printf("ERROR %v\n", err.Error())
		fmt.Printf("按 Ctrl+C 退出...\n")
		select {}
	}
	biz.SetDebug(args.Debug)
	echo.AddPlugin(handler.HandleHttpRequestEcho(biz, args.cfg))
	if args.Debug {
		echo.AddPlugin(&plugin.Plugin{
			Match: "debug.weixin.qq.com",
			Target: &plugin.TargetConfig{
				Protocol: "http",
				Host:     "127.0.0.1",
				Port:     6752,
			},
		})
	}

	var buf bytes.Buffer
	// 为了不在终端输出 http server 的日志
	log.SetOutput(&buf)
	server = &http.Server{
		Addr: "127.0.0.1:" + strconv.Itoa(args.Port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			echo.ServeHTTP(w, r)
		}),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("ERROR %v\n", err.Error())
			fmt.Printf("按 Ctrl+C 退出...\n")
			cancel()
		}
	}()

	if args.SetSystemProxy {
		err = proxy.EnableProxy(proxy.ProxySettings{
			Device:   args.Device,
			Hostname: "127.0.0.1",
			Port:     strconv.Itoa(args.Port),
		})
		if err != nil {
			fmt.Printf("ERROR 设置代理失败 %v\n", err.Error())
			fmt.Printf("按 Ctrl+C 退出...\n")
			select {}
		}
	}
	proxy_server_url := "http://127.0.0.1:" + strconv.Itoa(args.Port)
	color.Green("下载服务启动成功")
	if !args.SetSystemProxy {
		color.Red(fmt.Sprintf("当前未设置系统代理，请通过软件将流量转发至 %v", proxy_server_url))
		color.Red("设置成功后再打开视频号页面下载")
	} else {
		color.Green(fmt.Sprintf("已修改系统代理为 %v", proxy_server_url))
		color.Green("请打开需要下载的视频号页面进行下载")
	}
	fmt.Println("\n\n按 Ctrl+C 退出...")
	select {}
}
