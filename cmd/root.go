package cmd

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"wx_channel/config"
	"wx_channel/internal/interceptor"
)

var (
	Version        string
	device         string
	hostname       string
	port           int
	debug          bool
	cert_files     *interceptor.ServerCertFiles
	channel_files  *interceptor.ChannelInjectedFiles
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
		cfg.Debug = viper.GetBool("debug")
		root_command(RootCommandArg{
			InterceptorConfig: interceptor.InterceptorConfig{
				Version:        Version,
				SetSystemProxy: viper.GetBool("proxy.system"),
				Device:         device,
				Hostname:       viper.GetString("proxy.hostname"),
				Port:           viper.GetInt("proxy.port"),
				Debug:          cfg.Debug,
				CertFiles:      cert_files,
				CertFileName:   cert_file_name,
				ChannelFiles:   channel_files,
				Cfg:            cfg,
			},
		})
	},
}

func init() {
	root_cmd.PersistentFlags().StringVar(&device, "dev", "", "代理服务器网络设备")
	root_cmd.PersistentFlags().StringVar(&hostname, "hostname", "127.0.0.1", "代理服务器主机名")
	root_cmd.PersistentFlags().IntVar(&port, "port", 2023, "代理服务器端口")
	root_cmd.PersistentFlags().BoolVar(&debug, "debug", false, "是否开启调试")

	viper.BindPFlag("proxy.port", root_cmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("proxy.hostname", root_cmd.PersistentFlags().Lookup("hostname"))
	viper.BindPFlag("debug", root_cmd.PersistentFlags().Lookup("debug"))
}

func Execute(app_ver string, cert_filename string, files1 *interceptor.ChannelInjectedFiles, files2 *interceptor.ServerCertFiles, c *config.Config) error {
	cobra.MousetrapHelpText = ""

	Version = app_ver
	cert_file_name = cert_filename
	channel_files = files1
	cert_files = files2
	cfg = c

	return root_cmd.Execute()
}
func Register(cmd *cobra.Command) {
	root_cmd.AddCommand(cmd)
}

type RootCommandArg struct {
	interceptor.InterceptorConfig
}

func root_command(args RootCommandArg) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signal_chan := make(chan os.Signal, 1)
	err_chan := make(chan error, 1)

	signal.Notify(signal_chan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signal_chan)

	fmt.Printf("\nv%v\n", Version)
	fmt.Printf("问题反馈 https://github.com/ltaoo/wx_channels_download/issues\n\n")

	client, err := interceptor.NewInterceptor(args.InterceptorConfig)
	if err != nil {
		fmt.Printf("ERROR 初始化客户端失败: %v\n", err.Error())
		os.Exit(1)
	}
	if err := client.Start(); err != nil {
		fmt.Printf("ERROR 启动客户端失败: %v\n", err.Error())
		os.Exit(1)
	}
	proxy_server_addr := "127.0.0.1:" + strconv.Itoa(args.Port)

	var buf bytes.Buffer
	// 为了不在终端输出 http server 的日志
	log.SetOutput(&buf)
	server := &http.Server{
		Addr: proxy_server_addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			client.ServeHTTP(w, r)
		}),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	cleanup := func() {
		fmt.Printf("\n正在关闭下载服务...\n")
		if err := client.Stop(); err != nil {
			fmt.Printf("⚠️ 关闭客户端失败: %v\n", err)
		}
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			fmt.Printf("⚠️ 代理服务器关闭失败: %v\n", err)
			server.Close()
		}
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		fmt.Printf("ERROR 服务器启动失败: %v\n", err.Error())
		cleanup()
		os.Exit(1)
	}

	color.Green("下载服务启动成功")
	if !args.SetSystemProxy {
		color.Red(fmt.Sprintf("当前未设置系统代理,请通过软件将流量转发至 %v", proxy_server_addr))
		color.Red("设置成功后再打开视频号页面下载")
	} else {
		color.Green(fmt.Sprintf("已修改系统代理为 %v", proxy_server_addr))
		color.Green("请打开需要下载的视频号页面进行下载")
	}
	fmt.Println("\n按 Ctrl+C 退出...")

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			err_chan <- fmt.Errorf("服务器运行错误: %w", err)
		}
	}()

	select {
	case _ = <-signal_chan:
		// fmt.Printf("\n收到信号: %v\n", sig)
		cleanup()
	case err := <-err_chan:
		fmt.Printf("ERROR %v\n", err.Error())
		cleanup()
		os.Exit(1)
	case <-ctx.Done():
		cleanup()
	}
}
