package main

import (
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"wx_channel/internal/application"
	"wx_channel/internal/handler"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/decrypt"
	"wx_channel/pkg/echo"
	"wx_channel/pkg/proxy"
)

//go:embed certs/SunnyRoot.cer
var cert_data []byte

//go:embed certs/private.key
var private_key_data []byte

//go:embed lib/FileSaver.min.js
var js_file_saver []byte

//go:embed lib/jszip.min.js
var js_zip []byte

//go:embed inject/pagespy.min.js
var js_pagespy1 []byte

//go:embed inject/pagespy.js
var js_pagespy2 []byte

//go:embed inject/error.js
var js_error []byte

//go:embed inject/main.js
var js_main []byte

var version = "250913"
var DefaultPort = 2023
var RootCertificateName = "SunnyNet"

func main() {
	cobra.MousetrapHelpText = ""

	var (
		device string
		port   int
		debug  bool
	)

	root_cmd := &cobra.Command{
		Use:   "wx_video_download",
		Short: "启动下载程序",
		Long:  "启动后将对网络请求进行代理，在微信视频号详情页面注入下载按钮",
		Run: func(cmd *cobra.Command, args []string) {
			root_command(RootCommandArg{
				Device: device,
				Port:   port,
				Debug:  debug,
			})
		},
	}
	root_cmd.Flags().StringVar(&device, "dev", "", "代理服务器网络设备")
	root_cmd.Flags().IntVar(&port, "port", DefaultPort, "代理服务器端口")
	root_cmd.Flags().BoolVar(&debug, "debug", false, "是否开启调试")

	uninstall_certificate_cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "删除证书",
		Long:  "删除初始化时自动安装的证书",
		Run: func(cmd *cobra.Command, args []string) {
			command := cmd.Name()
			if command != "uninstall" {
				return
			}
			uninstall_certificate_command(UninstallCertificateCommandArgs{})
		},
	}

	var (
		video_url         string
		filename          string
		video_decrypt_key int
	)
	download_cmd := &cobra.Command{
		Use:   "download",
		Short: "下载视频",
		Long:  "从指定URL下载视频文件",
		Run: func(cmd *cobra.Command, args []string) {
			command := cmd.Name()
			if command != "download" {
				return
			}
			download_command(DownloadCommandArgs{
				URL:        video_url,
				DecryptKey: video_decrypt_key,
				Filename:   filename,
			})
		},
	}
	now := int(time.Now().Unix())
	download_cmd.Flags().StringVar(&video_url, "url", "", "视频URL（必需）")
	download_cmd.Flags().IntVar(&video_decrypt_key, "key", 0, "解密密钥（未加密的视频不用传该参数）")
	download_cmd.Flags().StringVar(&filename, "filename", strconv.Itoa(now)+".mp4", "下载后的文件名")
	download_cmd.MarkFlagRequired("url")

	var (
		filepath           string
		video_decrypt_key2 int
	)
	decrypt_cmd := &cobra.Command{
		Use:   "decrypt",
		Short: "解密视频",
		Long:  "使用 key 对本地加密视频进行解密",
		Run: func(cmd *cobra.Command, args []string) {
			command := cmd.Name()
			if command != "decrypt" {
				return
			}
			decrypt_command(DecryptCOmmandArgs{
				Filepath:   filepath,
				DecryptKey: video_decrypt_key2,
			})
		},
	}
	decrypt_cmd.Flags().StringVar(&filepath, "filepath", "", "视频地址（必需）")
	decrypt_cmd.Flags().IntVar(&video_decrypt_key2, "key", 0, "解密密钥（必需）")
	decrypt_cmd.MarkFlagRequired("filepath")

	root_cmd.AddCommand(uninstall_certificate_cmd)
	root_cmd.AddCommand(download_cmd)
	root_cmd.AddCommand(decrypt_cmd)
	if err := root_cmd.Execute(); err != nil {
		fmt.Printf("初始化失败 %v", err.Error())
		fmt.Printf("按 Ctrl+C 退出...\n")
		select {}
	}
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
	existing, err1 := certificate.CheckHasCertificate(RootCertificateName)
	if err1 != nil {
		fmt.Printf("\nERROR %v\v", err1.Error())
		fmt.Printf("按 Ctrl+C 退出...\n")
		select {}
	}
	if !existing {
		fmt.Printf("\n\n正在安装证书...\n")
		err := certificate.InstallCertificate(cert_data)
		time.Sleep(3 * time.Second)
		if err != nil {
			fmt.Printf("\nERROR %v\n", err.Error())
			fmt.Printf("按 Ctrl+C 退出...\n")
			select {}
		}
	}
	biz := application.NewBiz(version, cert_data, private_key_data, js_file_saver, js_zip, js_pagespy1, js_pagespy2, js_error, js_main)
	echo, err := echo.NewEcho(cert_data, private_key_data)
	if err != nil {
		fmt.Printf("\nERROR %v\n", err.Error())
		fmt.Printf("按 Ctrl+C 退出...\n")
		select {}
	}
	echo.SetHTTPHandler(handler.HandleHttpRequestEcho(biz))
	// Sunny.SetPort(args.Port).Start()
	go func() {
		server := &http.Server{
			Addr: ":" + strconv.Itoa(DefaultPort),
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				echo.ServeHTTP(w, r)
			}),
		}
		fmt.Printf("Proxy server listening on :%v\n", DefaultPort)
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

type UninstallCertificateCommandArgs struct {
}

func uninstall_certificate_command(args UninstallCertificateCommandArgs) {
	settings := proxy.ProxySettings{}
	if err := proxy.DisableProxy(settings); err != nil {
		fmt.Printf("\nERROR 取消代理失败 %v\n", err.Error())
		return
	}
	if err := certificate.UninstallCertificate(RootCertificateName); err != nil {
		fmt.Printf("\nERROR 删除根证书失败 %v\n", err.Error())
		return
	}
	color.Green(fmt.Sprintf("\n\n删除根证书 '%v' 成功\n", RootCertificateName))
}

type DownloadCommandArgs struct {
	URL        string
	Filename   string
	DecryptKey int
}

func download_command(args DownloadCommandArgs) {
	resp, err := http.Get(args.URL)
	if err != nil {
		fmt.Printf("[ERROR]下载失败 %v\n", err.Error())
		return
	}
	defer resp.Body.Close()
	homedir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("[ERROR]获取下载路径失败 %v\n", err.Error())
		return
	}
	tmp_filename := "wx_" + strconv.Itoa(int(time.Now().Unix()))
	tmp_dest_filepath := path.Join(homedir, "Downloads", tmp_filename)
	dest_filepath := path.Join(homedir, "Downloads", args.Filename)
	file, err := os.Create(tmp_dest_filepath)
	if err != nil {
		fmt.Printf("[ERROR]下载文件失败 %v\n", err.Error())
		os.Exit(0)
		return
	}
	defer file.Close()
	content_length := resp.Header.Get("Content-Length")
	total_size := int64(-1)
	if content_length != "" {
		total_size, _ = strconv.ParseInt(content_length, 10, 64)
	}
	buf := make([]byte, 32*1024) // 32KB buffer
	var downloaded int64 = 0
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, werr := file.Write(buf[:n])
			if werr != nil {
				fmt.Printf("[ERROR]写入文件失败 %v\n", werr.Error())
				return
			}
			downloaded += int64(n)
			if total_size > 0 {
				percent := float64(downloaded) / float64(total_size) * 100
				fmt.Printf("\r\033[K已下载: %d/%d 字节 (%.2f%%)", downloaded, total_size, percent)
			} else {
				fmt.Printf("\r\033[K已下载: %d 字节", downloaded)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("[ERROR]下载文件失败2 %v\n", err.Error())
			return
		}
	}
	fmt.Println()
	if args.DecryptKey != 0 {
		fmt.Printf("开始对文件解密 %s", tmp_dest_filepath)
		length := uint32(131072)
		enclen_str := resp.Header.Get("X-enclen")
		if enclen_str != "" {
			v, err := strconv.ParseUint(enclen_str, 10, 32)
			if err == nil {
				length = uint32(v)
			}
		}
		key := uint64(args.DecryptKey)
		data, err := os.ReadFile(tmp_dest_filepath)
		if err != nil {
			fmt.Printf("[ERROR]读取已下载的文件失败 %v\n", err.Error())
			return
		}
		decrypt.DecryptData(data, length, key)
		err = os.WriteFile(dest_filepath, data, 0644)
		if err != nil {
			fmt.Printf("[ERROR]写入文件失败 %v\n", err.Error())
			return
		}
		file.Close()
		err = os.Remove(tmp_dest_filepath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("[ERROR]临时文件不存在")
			} else if os.IsPermission(err) {
				fmt.Println("[ERROR]没有权限删除临时文件")
			} else {
				fmt.Printf("[ERROR]临时文件删除失败 %v\n", err.Error())
			}
		}
		fmt.Printf("解密完成，文件路径为 %s\n", dest_filepath)
		return
	}
	file.Close()
	err = os.Rename(tmp_dest_filepath, dest_filepath)
	if err != nil {
		fmt.Printf("[ERROR]重命名文件失败 %v\n", err.Error())
		return
	}
	fmt.Printf("下载完成，件路径为 %s\n", dest_filepath)
}

type DecryptCOmmandArgs struct {
	Filepath   string
	DecryptKey int
}

func decrypt_command(args DecryptCOmmandArgs) {
	fmt.Printf("开始对文件解密 %s", args.Filepath)
	length := uint32(131072)
	key := uint64(args.DecryptKey)
	data, err := os.ReadFile(args.Filepath)
	if err != nil {
		fmt.Printf("[ERROR]读取已下载的文件失败 %v\n", err.Error())
		return
	}
	decrypt.DecryptData(data, length, key)
	err = os.WriteFile(args.Filepath, data, 0644)
	if err != nil {
		fmt.Printf("[ERROR]写入文件失败 %v\n", err.Error())
		return
	}
	fmt.Printf("解密完成 %s", args.Filepath)
}
