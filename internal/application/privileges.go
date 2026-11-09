package application

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"

	"github.com/spf13/viper"

	"wx_channel/internal/buildtags"
	"wx_channel/internal/config"
	"wx_channel/internal/services"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/platform"
	"wx_channel/pkg/system"
)

// PrepareStartPrivileges requests elevation when a local proxy start requires it.
// should_exit is true after an elevation attempt because the current process must end.
func PrepareStartPrivileges(cfg *config.Config, is_start_command bool) (should_exit bool, err error) {
	if !is_start_command || runtime.GOOS != "windows" || !viper.GetBool("proxy.enabled") || platform.IsAdmin() {
		return false, nil
	}
	need_admin_for_proxy := viper.GetBool("proxy.tun") || buildtags.UsingSunnyNet
	need_admin_for_certificate := false
	if cfg.GetString("cert.file") != "" || cfg.GetString("cert.key") != "" {
		if err := services.EnsureLocalCertificate(cfg); err != nil {
			return false, err
		}
	} else {
		cert_files := services.LoadCertFiles()
		installed, check_err := certificate.CheckHasCertificateData(cert_files.Cert)
		if check_err != nil {
			return false, fmt.Errorf("failed to check root certificate before elevation: %w", check_err)
		}
		trusted := false
		if installed {
			trusted, check_err = certificate.CheckCertificateDataTrusted(cert_files.Cert, cert_files.Name)
			if check_err != nil {
				return false, fmt.Errorf("failed to check root certificate trust before elevation: %w", check_err)
			}
		}
		need_admin_for_certificate = !trusted
	}
	if !need_admin_for_proxy && !need_admin_for_certificate {
		return false, nil
	}
	started, exited := platform.RequestAdminPermissionAndWait()
	if !started {
		return true, errors.New("运行失败，请右键选择「以管理员身份运行」")
	}
	if exited {
		// The elevated console can be forcibly terminated before its in-process
		// close handler finishes. The original, non-elevated process survives as
		// a guardian and resets only the proxy address owned by this application.
		_, _ = system.DisableProxyIfMatches(system.ProxySettings{
			// Empty falls back to detecting the primary service, which is all a separate process
			// can do; an explicitly configured service still has to be honoured, otherwise this
			// clears the proxy somewhere it was never written.
			Device:   viper.GetString("proxy.networkService"),
			Hostname: viper.GetString("proxy.hostname"),
			Port:     strconv.Itoa(viper.GetInt("proxy.port")),
		})
	}
	return true, nil
}
