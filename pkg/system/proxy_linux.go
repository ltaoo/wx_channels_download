//go:build linux

package system

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type linux_proxy_backend struct {
	name    string
	enable  func(linux_proxy_session, ProxySettings, int) error
	disable func(linux_proxy_session, ProxySettings) error
	fetch   func(linux_proxy_session, ProxySettings) (*ProxySettings, error)
}

type linux_proxy_session struct {
	user     string
	uid      string
	env      []string
	launcher []string
}

// enable_proxy sets GNOME / Deepin proxy with correct user context.
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

	session, err := current_linux_proxy_session()
	if err != nil {
		return err
	}

	var errs []string
	for _, backend := range linux_proxy_backends() {
		if err := backend.enable(session, ps, port_int); err == nil {
			fmt.Printf("✅ 已成功设置系统代理（Linux: %s）\n", backend.name)
			return nil
		} else {
			errs = append(errs, fmt.Sprintf("%s: %v", backend.name, err))
		}
	}
	return fmt.Errorf("设置 Linux 系统代理失败，已依次尝试 %s。%s", linux_proxy_context(), strings.Join(errs, "；"))
}

// disable_proxy closes GNOME / Deepin system proxy.
func disable_proxy(arg ProxySettings) error {
	session, err := current_linux_proxy_session()
	if err != nil {
		return err
	}

	var errs []string
	for _, backend := range linux_proxy_backends() {
		if err := backend.disable(session, arg); err == nil {
			fmt.Printf("✅ 已关闭系统代理（Linux: %s）\n", backend.name)
			return nil
		} else {
			errs = append(errs, fmt.Sprintf("%s: %v", backend.name, err))
		}
	}
	return fmt.Errorf("关闭 Linux 系统代理失败，已依次尝试 %s。%s", linux_proxy_context(), strings.Join(errs, "；"))
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

	return run_linux_proxy_commands(session, cmds)
}

func disable_gsettings_proxy(session linux_proxy_session, arg ProxySettings) error {
	if !ExistingCommand("gsettings") {
		return fmt.Errorf("未找到 gsettings")
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

	return run_linux_proxy_commands(session, cmds)
}

func fetch_gsettings_proxy(session linux_proxy_session, arg ProxySettings) (*ProxySettings, error) {
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

func disable_kde_proxy(session linux_proxy_session, arg ProxySettings) error {
	kwriteconfig, err := kde_write_config_command()
	if err != nil {
		return err
	}

	cmds := [][]string{
		{kwriteconfig, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "ProxyType", "0"},
	}
	if err := run_linux_proxy_commands(session.without_launcher(), cmds); err != nil {
		return err
	}
	notify_kde_proxy_changed(session)
	return nil
}

func fetch_kde_proxy(session linux_proxy_session, arg ProxySettings) (*ProxySettings, error) {
	kreadconfig, err := kde_read_config_command()
	if err != nil {
		return nil, err
	}
	mode, err := run_linux_proxy_command_output(session.without_launcher(), []string{kreadconfig, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "ProxyType"})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(mode) != "1" {
		return nil, nil
	}
	value, err := run_linux_proxy_command_output(session.without_launcher(), []string{kreadconfig, "--file", "kioslaverc", "--group", "Proxy Settings", "--key", "httpProxy"})
	if err != nil {
		return nil, err
	}
	host, port := parse_proxy_url(strings.TrimSpace(value))
	if host == "" || port == "" {
		return nil, nil
	}
	return &ProxySettings{Hostname: host, Port: port}, nil
}

func kde_write_config_command() (string, error) {
	for _, name := range []string{"kwriteconfig6", "kwriteconfig5", "kwriteconfig"} {
		if ExistingCommand(name) {
			return name, nil
		}
	}
	return "", fmt.Errorf("未找到 kwriteconfig6/kwriteconfig5/kwriteconfig")
}

func kde_read_config_command() (string, error) {
	for _, name := range []string{"kreadconfig6", "kreadconfig5", "kreadconfig"} {
		if ExistingCommand(name) {
			return name, nil
		}
	}
	return "", fmt.Errorf("未找到 kreadconfig6/kreadconfig5/kreadconfig")
}

func notify_kde_proxy_changed(session linux_proxy_session) {
	for _, qdbus := range []string{"qdbus6", "qdbus-qt6", "qdbus", "qdbus-qt5"} {
		if !ExistingCommand(qdbus) {
			continue
		}
		_, _ = run_linux_proxy_command_output(session, []string{
			qdbus,
			"org.kde.KIO",
			"/KIO/Scheduler",
			"org.kde.KIO.Scheduler.reparseSlaveConfiguration",
			"",
		})
		return
	}
}

func parse_proxy_url(value string) (string, string) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimRight(value, "/")
	host, port, ok := strings.Cut(value, ":")
	if !ok {
		return "", ""
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", ""
	}
	return host, port
}

func run_linux_proxy_commands(session linux_proxy_session, cmds [][]string) error {
	for _, c := range cmds {
		if _, err := run_linux_proxy_command_output(session, c); err != nil {
			return err
		}
	}
	return nil
}

func run_linux_proxy_command_output(session linux_proxy_session, command []string) (string, error) {
	cmd := proxy_session_command(session, command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("命令失败: %s\n错误: %v\n输出: %s", strings.Join(cmd.Args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

func read_proxy_host_port(session linux_proxy_session, schema string) (string, string, error) {
	host, err := read_gsettings(session, schema, "host")
	if err != nil {
		return "", "", err
	}
	host = strings.Trim(host, "\"'")
	port_value, err := read_gsettings(session, schema, "port")
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

func read_gsettings(session linux_proxy_session, schema string, key string) (string, error) {
	cmd := proxy_session_command(session, []string{"gsettings", "get", schema, key})
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("读取系统代理失败: %s\n错误: %v\n输出: %s", strings.Join(cmd.Args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

func current_linux_proxy_session() (linux_proxy_session, error) {
	u, err := current_desktop_user()
	if err != nil {
		return linux_proxy_session{}, err
	}
	if strings.TrimSpace(u.Uid) == "" {
		return linux_proxy_session{}, fmt.Errorf("获取 UID 失败: 用户 %s 没有 UID", u.Username)
	}
	env, err := linux_proxy_session_env(u.Uid)
	launcher := []string{}
	if err != nil {
		if found_launcher, launcher_err := linux_proxy_session_launcher(); launcher_err == nil {
			launcher = found_launcher
		}
	}
	return linux_proxy_session{
		user:     u.Username,
		uid:      u.Uid,
		env:      env,
		launcher: launcher,
	}, nil
}

func (session linux_proxy_session) without_launcher() linux_proxy_session {
	session.launcher = nil
	return session
}

func linux_proxy_session_env(uid string) ([]string, error) {
	if env := proxy_session_env_from_values(os.Getenv("DBUS_SESSION_BUS_ADDRESS"), os.Getenv("XDG_RUNTIME_DIR"), uid); len(env) != 0 {
		return env, nil
	}

	runtime_dir := os.Getenv("XDG_RUNTIME_DIR")
	if strings.TrimSpace(runtime_dir) == "" {
		runtime_dir = "/run/user/" + uid
	}
	if env := proxy_session_env_from_runtime_dir(runtime_dir); len(env) != 0 {
		return env, nil
	}

	if env := proxy_session_env_from_proc(uid); len(env) != 0 {
		return env, nil
	}

	return nil, fmt.Errorf("获取 DBus 会话失败: 未找到 DBUS_SESSION_BUS_ADDRESS 或 %s/bus。请确认在同一个图形桌面会话中启动，或先执行 export DBUS_SESSION_BUS_ADDRESS=... 后再运行", runtime_dir)
}

func linux_proxy_session_launcher() ([]string, error) {
	if ExistingCommand("dbus-run-session") {
		return []string{"dbus-run-session", "--"}, nil
	}
	if ExistingCommand("dbus-launch") {
		return []string{"dbus-launch"}, nil
	}
	return nil, fmt.Errorf("未找到 dbus-run-session 或 dbus-launch")
}

func proxy_session_env_from_values(dbus_address string, runtime_dir string, uid string) []string {
	dbus_address = strings.TrimSpace(dbus_address)
	if dbus_address == "" {
		return nil
	}
	env := []string{"DBUS_SESSION_BUS_ADDRESS=" + dbus_address}
	runtime_dir = strings.TrimSpace(runtime_dir)
	if runtime_dir == "" {
		runtime_dir = "/run/user/" + uid
	}
	if runtime_dir != "" {
		env = append(env, "XDG_RUNTIME_DIR="+runtime_dir)
	}
	return env
}

func proxy_session_env_from_runtime_dir(runtime_dir string) []string {
	runtime_dir = strings.TrimSpace(runtime_dir)
	if runtime_dir == "" {
		return nil
	}
	bus_path := filepath.Join(runtime_dir, "bus")
	if stat, err := os.Stat(bus_path); err == nil && !stat.IsDir() {
		return []string{
			"DBUS_SESSION_BUS_ADDRESS=unix:path=" + bus_path,
			"XDG_RUNTIME_DIR=" + runtime_dir,
		}
	}
	return nil
}

func proxy_session_env_from_proc(uid string) []string {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		proc_dir := filepath.Join("/proc", entry.Name())
		info, err := os.Stat(proc_dir)
		if err != nil {
			continue
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || strconv.FormatUint(uint64(stat.Uid), 10) != uid {
			continue
		}
		data, err := os.ReadFile(filepath.Join(proc_dir, "environ"))
		if err != nil {
			continue
		}
		values := parse_proc_environ(data)
		if env := proxy_session_env_from_values(values["DBUS_SESSION_BUS_ADDRESS"], values["XDG_RUNTIME_DIR"], uid); len(env) != 0 {
			return env
		}
	}
	return nil
}

func parse_proc_environ(data []byte) map[string]string {
	values := make(map[string]string)
	for _, item := range strings.Split(string(data), "\x00") {
		if item == "" {
			continue
		}
		key, value, ok := strings.Cut(item, "=")
		if !ok || key == "" {
			continue
		}
		values[key] = value
	}
	return values
}

func current_desktop_user() (*user.User, error) {
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
	if pkexec_uid := strings.TrimSpace(os.Getenv("PKEXEC_UID")); pkexec_uid != "" {
		if u, err := user.LookupId(pkexec_uid); err == nil {
			return u, nil
		}
	}
	if u, err := user.Current(); err == nil {
		return u, nil
	}
	return nil, fmt.Errorf("获取登录用户失败: 无法从 SUDO_USER/USER/LOGNAME/PKEXEC_UID 获取用户")
}

func proxy_session_command(session linux_proxy_session, command []string) *exec.Cmd {
	if len(command) == 0 {
		return exec.Command("true")
	}
	command = proxy_session_command_with_launcher(session, command)
	current_uid := strconv.Itoa(os.Geteuid())
	if current_uid == session.uid {
		cmd := exec.Command(command[0], command[1:]...)
		cmd.Env = append(os.Environ(), session.env...)
		return cmd
	}
	env_command := append([]string{"env"}, append([]string{}, session.env...)...)
	env_command = append(env_command, command...)
	if os.Geteuid() != 0 {
		cmd := exec.Command("sudo", append([]string{"-n", "-u", session.user}, env_command...)...)
		cmd.Env = os.Environ()
		return cmd
	}
	if ExistingCommand("runuser") {
		cmd := exec.Command("runuser", append([]string{"-u", session.user, "--"}, env_command...)...)
		cmd.Env = os.Environ()
		return cmd
	}
	cmd := exec.Command("sudo", append([]string{"-u", session.user}, env_command...)...)
	cmd.Env = os.Environ()
	return cmd
}

func proxy_session_command_with_launcher(session linux_proxy_session, command []string) []string {
	if len(session.launcher) == 0 {
		return command
	}
	wrapped := append([]string{}, session.launcher...)
	wrapped = append(wrapped, command...)
	return wrapped
}
