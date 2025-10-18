//go:build linux

package proxy

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// EnableProxyInLinux sets GNOME / Deepin proxy with correct user context
func enable_proxy(ps ProxySettings) error {
	if ps.Hostname == "" {
		ps.Hostname = "127.0.0.1"
	}
	if ps.Port == "" {
		ps.Port = "8888"
	}
	portInt, err := strconv.Atoi(ps.Port)
	if err != nil {
		return fmt.Errorf("无效端口: %s", ps.Port)
	}

	// 获取当前图形用户（非 root）
	loginUserBytes, err := exec.Command("logname").Output()
	if err != nil {
		return fmt.Errorf("获取登录用户失败（logname）: %v", err)
	}
	loginUser := strings.TrimSpace(string(loginUserBytes))

	// 获取 UID（用于构造 DBUS 路径）
	uidBytes, err := exec.Command("id", "-u", loginUser).Output()
	if err != nil {
		return fmt.Errorf("获取 UID 失败: %v", err)
	}
	uid := strings.TrimSpace(string(uidBytes))

	// 构造 DBUS_SESSION_BUS_ADDRESS
	dbusEnv := "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/" + uid + "/bus"

	// 检测桌面环境（是否 Deepin）
	desktopEnv := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP"))
	isDeepin := strings.Contains(desktopEnv, "deepin")

	// 构造代理设置命令
	cmds := [][]string{
		{"gsettings", "set", "org.gnome.system.proxy", "mode", "manual"},
		{"gsettings", "set", "org.gnome.system.proxy.http", "host", ps.Hostname},
		{"gsettings", "set", "org.gnome.system.proxy.http", "port", fmt.Sprintf("%d", portInt)},
		{"gsettings", "set", "org.gnome.system.proxy.https", "host", ps.Hostname},
		{"gsettings", "set", "org.gnome.system.proxy.https", "port", fmt.Sprintf("%d", portInt)},
	}

	if isDeepin {
		cmds = append(cmds, []string{
			"dbus-send", "--session", "--dest=com.deepin.daemon.Proxy",
			"--type=method_call", "/com/deepin/daemon/Proxy",
			"com.deepin.daemon.Proxy.Apply",
		})
	}

	// 用登录用户执行所有命令（带上 DBUS 环境）
	for _, c := range cmds {
		fullCmd := append([]string{"-u", loginUser, "env", dbusEnv, c[0]}, c[1:]...)
		cmd := exec.Command("sudo", fullCmd...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("命令失败: sudo %s\n错误: %v\n输出: %s", strings.Join(fullCmd, " "), err, string(output))
		}
	}

	fmt.Println("✅ 已成功设置系统代理（Linux GNOME / Deepin）")
	return nil
}

// DisableProxyInLinux 关闭 GNOME / Deepin 的系统代理
func disable_proxy(arg ProxySettings) error {
	loginUserBytes, err := exec.Command("logname").Output()
	if err != nil {
		return fmt.Errorf("获取登录用户失败（logname）: %v", err)
	}
	loginUser := strings.TrimSpace(string(loginUserBytes))

	uidBytes, err := exec.Command("id", "-u", loginUser).Output()
	if err != nil {
		return fmt.Errorf("获取 UID 失败: %v", err)
	}
	uid := strings.TrimSpace(string(uidBytes))
	dbusEnv := "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/" + uid + "/bus"

	cmds := [][]string{
		{"gsettings", "set", "org.gnome.system.proxy", "mode", "none"},
	}

	// 检测 Deepin 环境，添加刷新命令
	desktopEnv := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP"))
	isDeepin := strings.Contains(desktopEnv, "deepin")
	if isDeepin {
		cmds = append(cmds, []string{
			"dbus-send", "--session", "--dest=com.deepin.daemon.Proxy",
			"--type=method_call", "/com/deepin/daemon/Proxy",
			"com.deepin.daemon.Proxy.Apply",
		})
	}

	// 执行每条命令
	for _, c := range cmds {
		fullCmd := append([]string{"-u", loginUser, "env", dbusEnv, c[0]}, c[1:]...)
		cmd := exec.Command("sudo", fullCmd...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("关闭代理命令失败: sudo %s\n错误: %v\n输出: %s", strings.Join(fullCmd, " "), err, string(output))
		}
	}

	fmt.Println("✅ 已关闭系统代理（Linux）")
	return nil
}

func get_network_interfaces() (*HardwarePort, error) {
	return nil, errors.New("not support")
}
