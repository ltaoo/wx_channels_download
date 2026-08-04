package cmd

import (
	"github.com/spf13/cobra"

	"wx_channel/internal/application"
)

var start_cmd = &cobra.Command{
	Use:   "start",
	Short: "启动下载程序",
	Long:  "启动管理界面、API 和本地代理服务",
	Run: func(cmd *cobra.Command, args []string) {
		application.Start(Cfg)
	},
}

func init() {
	root_cmd.AddCommand(start_cmd)
}
