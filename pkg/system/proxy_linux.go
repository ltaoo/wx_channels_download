//go:build linux

package system

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

type linuxProxySession struct {
	User string
	UID  string
	Env  []string
}

// EnableProxyInLinux sets GNOME / Deepin proxy with correct user context
func enable_proxy(ps ProxySettings) error {
	if ps.Hostname == "" {
		ps.Hostname = "127.0.0.1"
	}
	if ps.Port == "" {
		ps.Port = "8888"
	}
	port_int, err := strconv.Atoi(ps.Port)
	if err != nil {
		return fmt.Errorf("无效端口: %s", ps.Port)
	}

	session, err := currentLinuxProxySession()
	if err != nil {
		return err
	}

func fetch_cur_proxy(arg ProxySettings) (*ProxySettings, error) {
	session, err := current_linux_proxy_session()
	if err != nil {
		return nil, err
	}
	var errs []string
	for _, backend := range linux_proxy_backends() {
		if backend.fetch == nil {
			continue
		}
		proxy, err := backend.fetch(session, arg)
		if err == nil {
			return proxy, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", backend.name, err))
	}
	return nil, fmt.Errorf("读取 Linux 系统代理失败，已依次尝试 %s。%s", linux_proxy_context(), strings.Join(errs, "；"))
}

func get_network_interfaces() (*HardwarePort, error) {
	return nil, errors.New("not support")
}

func linux_proxy_backends() []linux_proxy_backend {
	backends := []linux_proxy_backend{
		{
			name:    "GNOME/GSettings",
			enable:  enable_gsettings_proxy,
			disable: disable_gsettings_proxy,
			fetch:   fetch_gsettings_proxy,
		},
		{
			name:    "KDE/KIO",
			enable:  enable_kde_proxy,
			disable: disable_kde_proxy,
			fetch:   fetch_kde_proxy,
		},
	}

	desktop := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP") + ":" + os.Getenv("DESKTOP_SESSION"))
	if strings.Contains(desktop, "kde") || strings.Contains(desktop, "plasma") {
		return []linux_proxy_backend{backends[1], backends[0]}
	}
	return backends
}

func linux_proxy_context() string {
	items := []string{}
	if desktop := strings.TrimSpace(os.Getenv("XDG_CURRENT_DESKTOP")); desktop != "" {
		items = append(items, "XDG_CURRENT_DESKTOP="+desktop)
	}
	if session := strings.TrimSpace(os.Getenv("DESKTOP_SESSION")); session != "" {
		items = append(items, "DESKTOP_SESSION="+session)
	}
	if distro := linux_distro_id(); distro != "" {
		items = append(items, "distro="+distro)
	}
	if len(items) == 0 {
		return "未知桌面/发行版"
	}
	return strings.Join(items, ", ")
}

func linux_distro_id() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[key] = strings.Trim(value, "\"'")
	}
	if pretty := values["PRETTY_NAME"]; pretty != "" {
		return pretty
	}
	return values["ID"]
}

func enable_gsettings_proxy(session linux_proxy_session, ps ProxySettings, port_int int) error {
	if !ExistingCommand("gsettings") {
		return fmt.Errorf("未找到 gsettings")
	}

	cmds := [][]string{
		{"gsettings", "set", "org.gnome.system.proxy", "mode", "manual"},
		{"gsettings", "set", "org.gnome.system.proxy.http", "host", ps.Hostname},
		{"gsettings", "set", "org.gnome.system.proxy.http", "port", fmt.Sprintf("%d", port_int)},
		{"gsettings", "set", "org.gnome.system.proxy.https", "host", ps.Hostname},
		{"gsettings", "set", "org.gnome.system.proxy.https", "port", fmt.Sprintf("%d", port_int)},
	}

	if strings.Contains(strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP")), "deepin") {
		cmds = append(cmds, []string{
			"dbus-send", "--session", "--dest=com.deepin.daemon.Proxy",
			"--type=method_call", "/com/deepin/daemon/Proxy",
			"com.deepin.daemon.Proxy.Apply",
		})
	}

	// 用登录用户执行所有命令（带上 DBUS 环境）
	for _, c := range cmds {
		cmd := proxySessionCommand(session, c)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("命令失败: %s\n错误: %v\n输出: %s", strings.Join(cmd.Args, " "), err, string(output))
		}
	}

	fmt.Println("✅ 已成功设置系统代理（Linux GNOME / Deepin）")
	return nil
}

// DisableProxyInLinux 关闭 GNOME / Deepin 的系统代理
func disable_proxy(arg ProxySettings) error {
	session, err := currentLinuxProxySession()
	if err != nil {
		return err
	}

	cmds := [][]string{
		{"gsettings", "set", "org.gnome.system.proxy", "mode", "none"},
	}

	if strings.Contains(strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP")), "deepin") {
		cmds = append(cmds, []string{
			"dbus-send", "--session", "--dest=com.deepin.daemon.Proxy",
			"--type=method_call", "/com/deepin/daemon/Proxy",
			"com.deepin.daemon.Proxy.Apply",
		})
	}

	// 执行每条命令
	for _, c := range cmds {
		cmd := proxySessionCommand(session, c)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("关闭代理命令失败: %s\n错误: %v\n输出: %s", strings.Join(cmd.Args, " "), err, string(output))
		}
	}

	fmt.Println("✅ 已关闭系统代理（Linux）")
	return nil
}

func fetch_cur_proxy(arg ProxySettings) (*ProxySettings, error) {
	session, err := currentLinuxProxySession()
	if err != nil {
		return nil, err
	}
	mode, err := read_gsettings(session, "org.gnome.system.proxy", "mode")
	if err != nil {
		return nil, err
	}
	mode = strings.Trim(mode, "\"'")
	if mode != "manual" {
		return nil, nil
	}
	host, port, err := read_proxy_host_port(session, "org.gnome.system.proxy.http")
	if err != nil {
		return nil, err
	}
	if host == "" || port == "" || port == "0" {
		host, port, err = read_proxy_host_port(session, "org.gnome.system.proxy.https")
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

func enable_kde_proxy(session linux_proxy_session, ps ProxySettings, port_int int) error {
	kwriteconfig, err := kde_write_config_command()
	if err != nil {
		return err
	}

	proxy_url := fmt.Sprintf("http://%s:%d", ps.Hostname, port_int)
	cmds := [][]string{
		{kwriteconfig, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "ProxyType", "1"},
		{kwriteconfig, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "httpProxy", proxy_url},
		{kwriteconfig, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "httpsProxy", proxy_url},
		{kwriteconfig, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "ftpProxy", proxy_url},
	}
	if err := run_linux_proxy_commands(session.without_launcher(), cmds); err != nil {
		return err
	}
	notify_kde_proxy_changed(session)
	return nil
}

func read_proxy_host_port(session linuxProxySession, schema string) (string, string, error) {
	host, err := read_gsettings(session, schema, "host")
	if err != nil {
		return "", "", err
	}
	host = strings.Trim(host, "\"'")
	portValue, err := read_gsettings(session, schema, "port")
	if err != nil {
		return "", "", err
	}
	port_value = strings.Trim(port_value, "\"'")
	if host == "" {
		return "", "", nil
	}
	if port_value == "" {
		return "", "", nil
	}
	if _, err := strconv.Atoi(port_value); err != nil {
		return "", "", nil
	}
	return host, port_value, nil
}

func read_gsettings(session linuxProxySession, schema string, key string) (string, error) {
	cmd := proxySessionCommand(session, []string{"gsettings", "get", schema, key})
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("读取系统代理失败: %s\n错误: %v\n输出: %s", strings.Join(cmd.Args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

func currentLinuxProxySession() (linuxProxySession, error) {
	u, err := currentDesktopUser()
	if err != nil {
		return linuxProxySession{}, err
	}
	if strings.TrimSpace(u.Uid) == "" {
		return linuxProxySession{}, fmt.Errorf("获取 UID 失败: 用户 %s 没有 UID", u.Username)
	}
	runtimeDir := "/run/user/" + u.Uid
	busPath := runtimeDir + "/bus"
	if _, err := os.Stat(busPath); err != nil {
		return linuxProxySession{}, fmt.Errorf("获取 DBus 会话失败: %s 不存在，Linux 服务器无图形桌面会话时无法通过 gsettings 设置系统代理", busPath)
	}
	return linuxProxySession{
		User: u.Username,
		UID:  u.Uid,
		Env: []string{
			"DBUS_SESSION_BUS_ADDRESS=unix:path=" + busPath,
			"XDG_RUNTIME_DIR=" + runtimeDir,
		},
	}, nil
}

func currentDesktopUser() (*user.User, error) {
	candidates := []string{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || value == "root" {
			return
		}
		for _, item := range candidates {
			if item == value {
				return
			}
		}
		candidates = append(candidates, value)
	}

	add(os.Getenv("SUDO_USER"))
	add(os.Getenv("USER"))
	add(os.Getenv("LOGNAME"))

	for _, name := range candidates {
		if u, err := user.Lookup(name); err == nil {
			return u, nil
		}
	}
	if pkexecUID := strings.TrimSpace(os.Getenv("PKEXEC_UID")); pkexecUID != "" {
		if u, err := user.LookupId(pkexecUID); err == nil {
			return u, nil
		}
	}
	if u, err := user.Current(); err == nil {
		return u, nil
	}
	return nil, fmt.Errorf("获取登录用户失败: 无法从 SUDO_USER/USER/LOGNAME/PKEXEC_UID 获取用户")
}

func proxySessionCommand(session linuxProxySession, command []string) *exec.Cmd {
	if len(command) == 0 {
		return exec.Command("true")
	}
	currentUID := strconv.Itoa(os.Geteuid())
	if currentUID == session.UID {
		cmd := exec.Command(command[0], command[1:]...)
		cmd.Env = append(os.Environ(), session.Env...)
		return cmd
	}
	envCommand := append([]string{"env"}, append([]string{}, session.Env...)...)
	envCommand = append(envCommand, command...)
	if os.Geteuid() != 0 {
		cmd := exec.Command("sudo", append([]string{"-n", "-u", session.User}, envCommand...)...)
		cmd.Env = os.Environ()
		return cmd
	}
	if ExistingCommand("runuser") {
		cmd := exec.Command("runuser", append([]string{"-u", session.User, "--"}, envCommand...)...)
		cmd.Env = os.Environ()
		return cmd
	}
	cmd := exec.Command("sudo", append([]string{"-u", session.User}, envCommand...)...)
	cmd.Env = os.Environ()
	return cmd
}
