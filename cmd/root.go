package cmd

import (
	"bufio"
	"fmt"
	"io"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"wx_channel/internal/application"
	"wx_channel/internal/config"
)

var (
	Version               string
	Cfg                   *config.Config
	device                string
	config_filepath       string
	workdir               string
	hostname              string
	port                  int
	debug                 bool
	start_command_invoked bool
	start_transferred     bool
)

var error_prefix = color.RedString("[ERROR]")

var root_cmd = &cobra.Command{
	Use:   "wx_video_download",
	Short: "启动下载程序",
	Long:  "\n启动后将对网络请求进行代理，在微信视频号详情页面注入下载按钮",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		is_root_start_command := !cmd.HasParent()
		if is_root_start_command {
			start_command_invoked = true
		}
		if err := application.PrepareConfig(Cfg, config_filepath); err != nil {
			return err
		}
		should_exit, err := application.PrepareStartPrivileges(is_root_start_command)
		if err != nil {
			return err
		}
		start_transferred = should_exit
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if start_transferred {
			return nil
		}
		return application.Start(Cfg)
	},
	SilenceErrors: true,
	SilenceUsage:  true,
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
	start_command_invoked = false
	start_transferred = false

	err := root_cmd.Execute()
	if err != nil && start_command_invoked {
		fmt.Fprintf(root_cmd.ErrOrStderr(), "%s %v\n", error_prefix, err)
		wait_for_start_failure(root_cmd.InOrStdin(), root_cmd.OutOrStdout())
		// The startup error has already been displayed and acknowledged. Keep the
		// historical zero exit status instead of making main print it a second time.
		return nil
	}
	return err
}

func wait_for_start_failure(in io.Reader, out io.Writer) {
	fmt.Fprint(out, "\nStartup failed. This window will remain open; press Enter to exit...")
	_, _ = bufio.NewReader(in).ReadString('\n')
}

func Register(cmd *cobra.Command) {
	root_cmd.AddCommand(cmd)
}
