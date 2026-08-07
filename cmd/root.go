package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// Execute dispatches one command from args. With no explicit command, start is
// used so running the executable directly keeps the existing behavior.
func Execute(version string, mode string, args []string, logger *zerolog.Logger) error {
	err := execute(version, mode, args, logger)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	return err
}

func execute(version string, mode string, args []string, logger *zerolog.Logger) error {
	logger.Info().Strs("args", args).Msg("execute command")
	if len(args) == 0 {
		return run_start(version, mode, nil, logger)
	}

	switch args[0] {
	case "start":
		return run_start(version, mode, args[1:], logger)
	case "update":
		return run_update(version, args[1:])
	case "deploy":
		return run_deploy(args[1:])
	case "uninstall":
		return run_uninstall(args[1:])
	case "version":
		return run_version(version, args[1:])
	case "help", "-h", "--help":
		print_root_usage(os.Stdout)
		return nil
	default:
		if strings.HasPrefix(args[0], "-") {
			return run_start(version, mode, args, logger)
		}
		return fmt.Errorf("unknown command %q; run with --help to list commands", args[0])
	}
}

func new_command_flag_set(name string, description string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: wx_video_download %s [options]\n\n%s\n\nOptions:\n", name, description)
		flags.PrintDefaults()
	}
	return flags
}

func add_config_flags(flags *flag.FlagSet, config_filepath *string) {
	flags.StringVar(config_filepath, "config", "", "配置文件路径")
	flags.StringVar(config_filepath, "c", "", "配置文件路径（--config 的简写）")
}

func reject_command_args(flags *flag.FlagSet) error {
	if flags.NArg() == 0 {
		return nil
	}
	return fmt.Errorf("%s: unexpected arguments: %v", flags.Name(), flags.Args())
}

func print_root_usage(output io.Writer) {
	fmt.Fprintln(output, "Usage: wx_video_download [command] [options]")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Commands:")
	fmt.Fprintln(output, "  start         启动管理界面、API 和本地代理服务（默认）")
	fmt.Fprintln(output, "  update        检查并安装最新版本")
	fmt.Fprintln(output, "  deploy mp     部署公众号 Cloudflare Worker")
	fmt.Fprintln(output, "  deploy sph    部署视频号查询 Cloudflare Worker")
	fmt.Fprintln(output, "  uninstall     删除根证书")
	fmt.Fprintln(output, "  version       查看当前版本")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Run 'wx_video_download <command> --help' for command options.")
}
