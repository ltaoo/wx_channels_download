package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"wx_channel/internal/application"
	"wx_channel/internal/handler"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/echo"
	"wx_channel/pkg/proxy"
)

var (
	device         string
	port           int
	debug          bool
	version        string
	files          *application.BizFiles
	cert_file_name string
)

var root_cmd = &cobra.Command{
	Use:   "wx_video_download",
	Short: "启动下载程序",
	Long:  "\n启动后将对网络请求进行代理，在微信视频号详情页面注入下载按钮",
	Run: func(cmd *cobra.Command, args []string) {
		root_command(RootCommandArg{
			Device: device,
			Port:   port,
			Debug:  debug,
		})
	},
}

func init() {
	root_cmd.Flags().StringVar(&device, "dev", "", "代理服务器网络设备")
	root_cmd.Flags().IntVar(&port, "port", 2023, "代理服务器端口")
	root_cmd.Flags().BoolVar(&debug, "debug", false, "是否开启调试")

}

func Initialize(app_ver string, cert_file_name string, files *application.BizFiles) {
	version = app_ver
	cert_file_name = cert_file_name
	files = files
}
func Register(cmd *cobra.Command) {
	root_cmd.AddCommand(cmd)
}
func Execute() error {
	cobra.MousetrapHelpText = ""

	return root_cmd.Execute()
}

type RootCommandArg struct {
	Device string
	Port   int
	Debug  bool
}

func root_command(args RootCommandArg) {
	os_env := runtime.GOOS

	signal_chan := make(chan os.Signal, 1)
	// Notify the signal channel on SIGINT (Ctrl+C) and SIGTERM
	signal.Notify(signal_chan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-signal_chan
		fmt.Printf("\n正在关闭服务...%v\n\n", sig)
		arg := proxy.ProxySettings{
			Device:   args.Device,
			Hostname: "127.0.0.1",
			Port:     strconv.Itoa(args.Port),
		}
		err := proxy.DisableProxy(arg)
		if err != nil {
			fmt.Printf("⚠️ 关闭系统代理失败: %v\n", err)
		}
		os.Exit(0)
	}()

	fmt.Printf("\nv" + version)
	fmt.Printf("\n问题反馈 https://github.com/ltaoo/wx_channels_download/issues\n")
	existing, err1 := certificate.CheckHasCertificate(cert_file_name)
	if err1 != nil {
		fmt.Printf("\nERROR %v\v", err1.Error())
		fmt.Printf("按 Ctrl+C 退出...\n")
		select {}
	}
	if !existing {
		fmt.Printf("\n\n正在安装证书...\n")
		err := certificate.InstallCertificate(files.CertFile)
		time.Sleep(3 * time.Second)
		if err != nil {
			fmt.Printf("\nERROR %v\n", err.Error())
			fmt.Printf("按 Ctrl+C 退出...\n")
			select {}
		}
	}
	biz := application.NewBiz(version, files)
	echo, err := echo.NewEcho(files.CertFile, files.PrivateKeyFile)
	if err != nil {
		fmt.Printf("\nERROR %v\n", err.Error())
		fmt.Printf("按 Ctrl+C 退出...\n")
		select {}
	}
	echo.SetHTTPHandler(handler.HandleHttpRequestEcho(biz))
	// Sunny.SetPort(args.Port).Start()
	go func() {
		server := &http.Server{
			Addr: ":" + strconv.Itoa(port),
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				echo.ServeHTTP(w, r)
			}),
		}
		fmt.Printf("Proxy server listening on :%v\n", port)
		err := server.ListenAndServe()
		if err != nil {
			fmt.Printf("\nERROR %v\n", err.Error())
			fmt.Printf("按 Ctrl+C 退出...\n")
		}
	}()

	if os_env == "windows" {
		// ok := Sunny.OpenDrive(true)
		// if !ok {
		// 	fmt.Printf("\nERROR 启动进程代理失败，检查是否以管理员身份运行\n")
		// 	fmt.Printf("按 Ctrl+C 退出...\n")
		// 	select {}
		// }
		// Sunny.ProcessAddName("WeChatAppEx.exe")
		err = proxy.EnableProxy(proxy.ProxySettings{
			Device:   args.Device,
			Hostname: "127.0.0.1",
			Port:     strconv.Itoa(args.Port),
		})
		if err != nil {
			fmt.Printf("\nERROR 设置代理失败 %v\n", err.Error())
			fmt.Printf("按 Ctrl+C 退出...\n")
			select {}
		}
	} else {
		err = proxy.EnableProxy(proxy.ProxySettings{
			Device:   args.Device,
			Hostname: "127.0.0.1",
			Port:     strconv.Itoa(args.Port),
		})
		if err != nil {
			fmt.Printf("\nERROR 设置代理失败 %v\n", err.Error())
			fmt.Printf("按 Ctrl+C 退出...\n")
			select {}
		}
	}
	color.Green(fmt.Sprintf("\n\n服务已正确启动，请打开需要下载的视频号页面进行下载"))
	fmt.Println("\n\n服务正在运行，按 Ctrl+C 退出...")
	select {}
}
