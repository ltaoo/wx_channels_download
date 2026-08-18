package cmd

import (
	"github.com/spf13/cobra"

	"wx_channel/internal/application"
)

var update_cmd = &cobra.Command{
	Use:   "update",
	Short: "检查并更新到最新版本",
	Run: func(cmd *cobra.Command, args []string) {
		application.Update(Version)
	},
}

func init() {
	root_cmd.AddCommand(update_cmd)
}
