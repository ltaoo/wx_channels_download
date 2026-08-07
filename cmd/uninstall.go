package cmd

import (
	"fmt"

	"github.com/fatih/color"

	"wx_channel/internal/config"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/system"
)

func run_uninstall(args []string) error {
	flags := new_command_flag_set("uninstall", "删除初始化时自动安装的证书")
	var config_filepath string
	add_config_flags(flags, &config_filepath)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := reject_command_args(flags); err != nil {
		return err
	}
	cfg := config.New(config_filepath, nil)
	if err := cfg.LoadConfig(); err != nil {
		return err
	}
	uninstall_certificate_command(&UninstallCertificateCommandArgs{
		CertFiles: config.LoadCertFiles(cfg),
	})
	return nil
}

type UninstallCertificateCommandArgs struct {
	CertFiles *certificate.CertFileAndKeyFile
}

func uninstall_certificate_command(args *UninstallCertificateCommandArgs) {
	settings := system.ProxySettings{}
	if err := system.DisableProxy(settings); err != nil {
		fmt.Printf("\nERROR: Failed to cancel proxy: %v\n", err.Error())
		return
	}
	if err := certificate.UninstallCertificate(args.CertFiles.Name); err != nil {
		fmt.Printf("\nERROR: Failed to delete root certificate: %v\n", err.Error())
		return
	}
	color.Green(fmt.Sprintf("\n\n删除根证书 '%v' 成功\n", args.CertFiles.Name))
}
