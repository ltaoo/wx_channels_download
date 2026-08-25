package application

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"

	"github.com/spf13/viper"

	"wx_channel/internal/buildtags"
	"wx_channel/internal/services"
	"wx_channel/pkg/certificate"
	"wx_channel/pkg/platform"
	"wx_channel/pkg/system"
)

// PrepareStartPrivileges requests elevation when a local proxy start requires it.
// shouldExit is true after an elevation attempt because the current process must end.
func PrepareStartPrivileges(isStartCommand bool) (shouldExit bool, err error) {
	needAdminForProxy := viper.GetBool("proxy.enabled") && ((viper.GetBool("proxy.tun") || buildtags.UsingSunnyNet))
	if !isStartCommand || runtime.GOOS != "windows" || !needAdminForProxy || platform.IsAdmin() {
		return false, nil
	}
	needAdminForCertificate := false
	if !viper.GetBool("proxy.skipInstallRootCert") {
		certFiles := services.LoadCertFiles()
		installed, checkErr := certificate.CheckHasCertificate(certFiles.Name)
		if checkErr != nil {
			return false, fmt.Errorf("failed to check root certificate before elevation: %w", checkErr)
		}
		needAdminForCertificate = !installed
	}
	if !needAdminForProxy && !needAdminForCertificate {
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
