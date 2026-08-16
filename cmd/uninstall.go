package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

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
			CertNames: uninstall_certificate_names(viper.GetString("cert.name")),
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
	CertNames []string
}

func uninstall_certificate_names(configured_name string) []string {
	candidates := []string{certificate.DefaultCertFiles.Name, "SunnyRoot", strings.TrimSpace(configured_name)}
	names := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, name := range candidates {
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

func uninstall_certificate_command(args *UninstallCertificateCommandArgs) {
	// Empty falls back to detecting the primary service; an explicitly configured service still
	// has to be honoured, otherwise this clears the proxy somewhere it was never written.
	settings := system.ProxySettings{Device: viper.GetString("proxy.networkService")}
	if err := system.DisableProxy(settings); err != nil {
		fmt.Printf("\nERROR: Failed to cancel proxy: %v\n", err.Error())
		return
	}
	removed := make([]string, 0, len(args.CertNames))
	for _, name := range args.CertNames {
		installed, err := certificate.CheckHasCertificate(name)
		if err != nil {
			fmt.Printf("\nERROR: Failed to check root certificate '%v': %v\n", name, err.Error())
			return
		}
		if !installed {
			continue
		}
		if err := certificate.UninstallCertificate(name); err != nil {
			fmt.Printf("\nERROR: Failed to delete root certificate '%v': %v\n", name, err.Error())
			return
		}
		removed = append(removed, name)
	}
	if len(removed) == 0 {
		color.Yellow("\n\n未找到需要删除的根证书\n")
		return
	}
	for _, name := range removed {
		color.Green(fmt.Sprintf("\n\n删除根证书 '%v' 成功\n", name))
	}
}
