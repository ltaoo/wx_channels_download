package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version_cmd = &cobra.Command{
	Use:   "version",
	Short: "查看版本",
	Long:  "查看当前应用版本",
	Run: func(cmd *cobra.Command, args []string) {
		command := cmd.Name()
		if command != "version" {
			return
		}
		version_command()
	},
}

func init() {
	root_cmd.AddCommand(version_cmd)
}

func version_command() {
	fmt.Println(Version)
}
