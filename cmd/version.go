package cmd

import (
	"fmt"
)

func run_version(version string, args []string) error {
	flags := new_command_flag_set("version", "查看当前应用版本")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := reject_command_args(flags); err != nil {
		return err
	}
	version_command(version)
	return nil
}

func version_command(version string) {
	fmt.Println(version)
}
