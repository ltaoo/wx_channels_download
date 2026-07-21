package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"wx_channel/internal/application"
	"wx_channel/internal/config"
)

var (
	Version         string
	Cfg             *config.Config
	device          string
	config_filepath string
	workdir         string
	hostname        string
	port            int
	debug           bool
)

var error_prefix = color.RedString("[ERROR]")

var root_cmd = &cobra.Command{
	Use:   "wx_video_download",
	Short: "启动下载程序",
	Long:  "\n启动后将对网络请求进行代理，在微信视频号详情页面注入下载按钮",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := application.PrepareConfig(Cfg, config_filepath); err != nil {
			fmt.Println(fmt.Sprintf("%s%v", error_prefix, err))
			os.Exit(0)
		}
		shouldExit, err := application.PrepareStartPrivileges(!cmd.HasParent() || cmd.Name() == "start")
		if err != nil {
			fmt.Println(error_prefix + err.Error())
		}
		if shouldExit {
			os.Exit(0)
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		application.Start(Cfg)
	},
}

func init() {
	root_cmd.PersistentFlags().StringVar(&device, "dev", "", "代理服务器网络设备")
	root_cmd.PersistentFlags().StringVarP(&config_filepath, "config", "c", "", "配置文件路径")
	root_cmd.PersistentFlags().StringVar(&workdir, "workdir", "", "运行时工作目录")
	root_cmd.PersistentFlags().StringVar(&hostname, "hostname", "127.0.0.1", "代理服务器主机名")
	root_cmd.PersistentFlags().IntVar(&port, "port", 2023, "代理服务器端口")
	root_cmd.PersistentFlags().BoolVar(&debug, "debug", false, "是否开启调试")

	viper.BindPFlag("workdir", root_cmd.PersistentFlags().Lookup("workdir"))
	viper.BindPFlag("debug.error", root_cmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("proxy.hostname", root_cmd.PersistentFlags().Lookup("hostname"))
	viper.BindPFlag("proxy.port", root_cmd.PersistentFlags().Lookup("port"))
}

func Execute(cfg *config.Config) error {
	cobra.MousetrapHelpText = ""

	Version = cfg.Version
	Cfg = cfg

	return root_cmd.Execute()
}

func Register(cmd *cobra.Command) {
	root_cmd.AddCommand(cmd)
}
