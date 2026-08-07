package cmd

import (
	"errors"
	"flag"
	"runtime"

	"github.com/rs/zerolog"

	"wx_channel/internal/application"
	"wx_channel/internal/buildtags"
	"wx_channel/internal/config"
	"wx_channel/pkg/configapi"
	"wx_channel/pkg/platform"
)

type start_command_options struct {
	config_filepath string
	device          string
	workdir         string
	hostname        string
	port            int
	debug           bool
}

func run_start(version string, mode string, args []string, logger *zerolog.Logger) error {
	options, values, err := parse_start_command(args)
	if err != nil {
		return err
	}
	values["version"] = version
	values["mode"] = mode

	cfg := config.New(options.config_filepath, values)
	if err := cfg.LoadConfig(); err != nil {
		return err
	}
	should_exit, err := prepare_start_privileges(cfg)
	if err != nil {
		return err
	}
	if should_exit {
		return nil
	}

	application.Start(cfg, logger)
	return nil
}

func prepare_start_privileges(provider configapi.Provider) (should_exit bool, err error) {
	var proxy_config struct {
		Tun bool `json:"tun"`
	}
	if err := configapi.Declare("proxy").Decode(provider, "proxy", &proxy_config); err != nil {
		return false, err
	}
	need_admin_for_proxy := proxy_config.Tun || buildtags.UsingSunnyNet
	if runtime.GOOS != "windows" || !need_admin_for_proxy || platform.IsAdmin() {
		return false, nil
	}
	if !platform.RequestAdminPermission() {
		return true, errors.New("startup failed; right-click and select \"Run as administrator\"")
	}
	return true, nil
}

func parse_start_command(args []string) (start_command_options, map[string]any, error) {
	options := start_command_options{hostname: "127.0.0.1", port: 2023}
	flags := new_command_flag_set("start", "启动下载器")
	add_config_flags(flags, &options.config_filepath)
	flags.StringVar(&options.device, "dev", "", "代理服务器网络设备")
	flags.StringVar(&options.workdir, "workdir", "", "运行时工作目录")
	flags.StringVar(&options.hostname, "hostname", options.hostname, "代理服务器主机名")
	flags.IntVar(&options.port, "port", options.port, "代理服务器端口")
	flags.BoolVar(&options.debug, "debug", false, "是否开启调试")
	if err := flags.Parse(args); err != nil {
		return start_command_options{}, nil, err
	}
	if err := reject_command_args(flags); err != nil {
		return start_command_options{}, nil, err
	}

	values := make(map[string]any)
	flags.Visit(func(item *flag.Flag) {
		switch item.Name {
		case "dev":
			values["proxy.device"] = options.device
		case "workdir":
			values["workdir"] = options.workdir
		case "hostname":
			values["proxy.hostname"] = options.hostname
		case "port":
			values["proxy.port"] = options.port
		case "debug":
			values["debug.error"] = options.debug
		}
	})
	return options, values, nil
}
