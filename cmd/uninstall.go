package cmd

import (
	"bufio"
	"fmt"
	"io"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"wx_channel/internal/services"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/platform"
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
		if !platform.IsAdmin() {
			if !platform.RequestAdminPermission() {
				fmt.Printf("\nERROR: Failed to run. Please right-click and select \"Run as administrator\".\n")
				wait_for_uninstall_exit(cmd.InOrStdin(), cmd.OutOrStdout())
			}
			return
		}
		uninstall_certificate_command(&UninstallCertificateCommandArgs{
			CertFiles: services.LoadCertFiles(),
		})
		wait_for_uninstall_exit(cmd.InOrStdin(), cmd.OutOrStdout())
	},
}

func wait_for_uninstall_exit(in io.Reader, out io.Writer) {
	fmt.Fprint(out, "\nPress Enter to close the window...")
	_, _ = bufio.NewReader(in).ReadString('\n')
}

func init() {
	root_cmd.AddCommand(uninstall_certificate_cmd)
}

type UninstallCertificateCommandArgs struct {
	CertFiles *certificate.CertFileAndKeyFile
}

func uninstall_certificate_command(args *UninstallCertificateCommandArgs) {
	// Empty falls back to detecting the primary service; an explicitly configured service still
	// has to be honoured, otherwise this clears the proxy somewhere it was never written.
	settings := system.ProxySettings{Device: viper.GetString("proxy.networkService")}
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
