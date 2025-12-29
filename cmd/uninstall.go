package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"wx_channel/pkg/certificate"
	"wx_channel/pkg/system"
)

var uninstall_certificate_cmd = &cobra.Command{
	Use:   "uninstall",
	Short: "删除证书",
	Long:  "删除初始化时自动安装的证书",
	Run: func(cmd *cobra.Command, args []string) {
		command := cmd.Name()
		if command != "uninstall" {
			return
		}
		uninstall_certificate_command(&UninstallCertificateCommandArgs{
			CertFiles: CertFiles,
		})
	},
}

func init() {
	root_cmd.AddCommand(uninstall_certificate_cmd)
}

type UninstallCertificateCommandArgs struct {
	CertFiles *certificate.CertFileAndKeyFile
}

func uninstall_certificate_command(args *UninstallCertificateCommandArgs) {
	settings := system.ProxySettings{}
	if err := system.DisableProxy(settings); err != nil {
		fmt.Printf("\nERROR 取消代理失败 %v\n", err.Error())
		return
	}
	if err := certificate.UninstallCertificate(args.CertFiles.Name); err != nil {
		fmt.Printf("\nERROR 删除根证书失败 %v\n", err.Error())
		return
	}
	color.Green(fmt.Sprintf("\n\n删除根证书 '%v' 成功\n", args.CertFiles.Name))
}
