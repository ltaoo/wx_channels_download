package application

import (
	"errors"
	"runtime"

	"github.com/spf13/viper"

	"wx_channel/internal/buildtags"
	"wx_channel/pkg/platform"
)

// PrepareStartPrivileges requests elevation when a local proxy start requires it.
// shouldExit is true after an elevation attempt because the current process must end.
func PrepareStartPrivileges(isStartCommand bool) (shouldExit bool, err error) {
	needAdminForProxy := viper.GetBool("proxy.system") || viper.GetBool("proxy.tun") || buildtags.UsingSunnyNet
	if !isStartCommand || runtime.GOOS != "windows" || !needAdminForProxy || platform.IsAdmin() {
		return false, nil
	}
	if !platform.RequestAdminPermission() {
		return true, errors.New("运行失败，请右键选择「以管理员身份运行」")
	}
	return true, nil
}
