//go:build linux

package system

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

func fetch_cur_proxy(arg ProxySettings) (*ProxySettings, error) {
	loginUserBytes, err := exec.Command("logname").Output()
	if err != nil {
		return nil, fmt.Errorf("获取登录用户失败（logname）: %v", err)
	}
	loginUser := strings.TrimSpace(string(loginUserBytes))
	uidBytes, err := exec.Command("id", "-u", loginUser).Output()
	if err != nil {
		return nil, fmt.Errorf("获取 UID 失败: %v", err)
	}
	uid := strings.TrimSpace(string(uidBytes))
	dbusEnv := "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/" + uid + "/bus"
	mode, err := read_gsettings(loginUser, dbusEnv, "org.gnome.system.proxy", "mode")
	if err != nil {
		return nil, err
	}
	mode = strings.Trim(mode, "\"'")
	if mode != "manual" {
		return nil, nil
	}
	host, port, err := read_proxy_host_port(loginUser, dbusEnv, "org.gnome.system.proxy.http")
	if err != nil {
		return nil, err
	}
	if host == "" || port == "" || port == "0" {
		host, port, err = read_proxy_host_port(loginUser, dbusEnv, "org.gnome.system.proxy.https")
		if err != nil {
			return nil, err
		}
	}
	if host == "" || port == "" || port == "0" {
		return nil, nil
	}
	return &ProxySettings{
		Hostname: host,
		Port:     port,
	}, nil
}

func get_network_interfaces() (*HardwarePort, error) {
	return nil, errors.New("not support")
}

func read_proxy_host_port(loginUser string, dbusEnv string, schema string) (string, string, error) {
	host, err := read_gsettings(loginUser, dbusEnv, schema, "host")
	if err != nil {
		return "", "", err
	}
	host = strings.Trim(host, "\"'")
	portValue, err := read_gsettings(loginUser, dbusEnv, schema, "port")
	if err != nil {
		return "", "", err
	}
	portValue = strings.Trim(portValue, "\"'")
	if host == "" {
		return "", "", nil
	}
	if portValue == "" {
		return "", "", nil
	}
	if _, err := strconv.Atoi(portValue); err != nil {
		return "", "", nil
	}
	return host, portValue, nil
}

func read_gsettings(loginUser string, dbusEnv string, schema string, key string) (string, error) {
	fullCmd := []string{"-u", loginUser, "env", dbusEnv, "gsettings", "get", schema, key}
	cmd := exec.Command("sudo", fullCmd...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("读取系统代理失败: sudo %s\n错误: %v\n输出: %s", strings.Join(fullCmd, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}
