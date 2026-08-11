//go:build windows

package system

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
)

var internet_set_option = windows.NewLazySystemDLL("wininet.dll").NewProc("InternetSetOptionW")

const (
	internet_option_refresh          = 37
	internet_option_settings_changed = 39
)

func enable_proxy(args ProxySettings) error {
	args = merge_default_settings(args)
	path := `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	proxy_server_url := fmt.Sprintf("%v:%v", args.Hostname, args.Port)

	if err := run_reg_command("add", path, "/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "1", "/f"); err != nil {
		return fmt.Errorf("设置系统代理时发生错误，%v", err)
	}

	if err := run_reg_command("add", path, "/v", "ProxyServer", "/t", "REG_SZ", "/d", proxy_server_url, "/f"); err != nil {
		return fmt.Errorf("设置 HTTP 代理失败，%v", err)
	}
	return notify_proxy_settings_changed()
}

func disable_proxy(args ProxySettings) error {
	path := `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`

	if err := run_reg_command("add", path, "/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "0", "/f"); err != nil {
		return fmt.Errorf("设置 HTTP 代理失败，%v", err)
	}
	return notify_proxy_settings_changed()
}

func notify_proxy_settings_changed() error {
	if err := internet_set_option.Find(); err != nil {
		return fmt.Errorf("failed to load InternetSetOptionW: %w", err)
	}
	for _, option := range []uintptr{internet_option_settings_changed, internet_option_refresh} {
		result, _, call_err := internet_set_option.Call(0, option, 0, 0)
		if result == 0 {
			return fmt.Errorf("failed to refresh Windows proxy settings: %w", call_err)
		}
	}
	return nil
}

// run_reg_command executes a "reg" command (e.g. "reg add ...").
// If the direct call fails (e.g. due to group policy lock), it retries with
// elevated privileges via PowerShell Start-Process -Verb RunAs -Wait.
// This keeps the calling process (HTTP server) alive while only elevating
// the individual registry operation through a UAC dialog.
func run_reg_command(args ...string) error {
	// Attempt 1: direct reg call (no elevation, no UAC prompt)
	cmd := exec.Command("reg", args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	// Attempt 2: elevate just this reg command via PowerShell
	psCmd := "Start-Process -Verb RunAs -Wait -FilePath 'reg'"
	for _, arg := range args {
		psCmd += " -ArgumentList " + powershellEscape(arg)
	}

	psExec := exec.Command("powershell", "-NoProfile", "-Command", psCmd)
	output2, err2 := psExec.CombinedOutput()
	if err2 != nil {
		return fmt.Errorf(
			"普通执行失败: %s\n提权执行失败: %s",
			strings.TrimSpace(string(output)),
			strings.TrimSpace(string(output2)),
		)
	}
	return nil
}

// powershellEscape wraps a string in single quotes for use in PowerShell
// -ArgumentList, doubling any embedded single quotes per PS escaping rules.
func powershellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func fetch_cur_proxy(args ProxySettings) (*ProxySettings, error) {
	path := `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	enableValue, err := read_reg_value(path, "ProxyEnable")
	if err != nil {
		return nil, err
	}
	if enableValue == "" {
		return nil, nil
	}
	enabled, err := parse_reg_dword(enableValue)
	if err != nil {
		return nil, err
	}
	if enabled == 0 {
		return nil, nil
	}
	serverValue, err := read_reg_value(path, "ProxyServer")
	if err != nil {
		return nil, err
	}
	if serverValue == "" {
		return nil, nil
	}
	host, port, err := parse_proxy_server_value(serverValue)
	if err != nil {
		return nil, err
	}
	if host == "" || port == "" {
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

// ProxyTargetDescription has nothing to report on Windows: the proxy lives in the registry and
// applies globally, so there is no network service to pick and no way to pick the wrong one.
func ProxyTargetDescription(configured string) (service string, warning string) {
	return "", ""
}

func read_reg_value(path string, name string) (string, error) {
	cmd := exec.Command("reg", "query", path, "/v", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputText := string(output)
		lower := strings.ToLower(outputText)
		if strings.Contains(lower, "unable to find") || strings.Contains(outputText, "找不到") || strings.Contains(outputText, "无法找到") {
			return "", nil
		}
		return "", fmt.Errorf("读取系统代理失败，%v", outputText)
	}
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.Contains(line, name) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		return fields[len(fields)-1], nil
	}
	return "", nil
}

func parse_reg_dword(value string) (int64, error) {
	num, err := strconv.ParseInt(strings.TrimSpace(value), 0, 64)
	if err != nil {
		return 0, fmt.Errorf("解析系统代理开关失败: %v", err)
	}
	return num, nil
}

func parse_proxy_server_value(value string) (string, string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "", "", nil
	}
	parts := strings.Split(raw, ";")
	candidate := pick_proxy_candidate(parts, "http=")
	if candidate == "" {
		candidate = pick_proxy_candidate(parts, "https=")
	}
	if candidate == "" {
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if idx := strings.Index(part, "="); idx >= 0 {
				candidate = strings.TrimSpace(part[idx+1:])
				break
			}
		}
	}
	if candidate == "" {
		candidate = raw
	}
	return split_host_port(candidate)
}

func pick_proxy_candidate(parts []string, prefix string) string {
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), prefix) {
			return strings.TrimSpace(part[len(prefix):])
		}
	}
	return ""
}

func split_host_port(value string) (string, string, error) {
	candidate := strings.TrimSpace(value)
	if candidate == "" {
		return "", "", nil
	}
	if strings.HasPrefix(candidate, "[") {
		host, port, err := net.SplitHostPort(candidate)
		if err != nil {
			return "", "", fmt.Errorf("解析系统代理地址失败: %v", err)
		}
		return host, port, nil
	}
	idx := strings.LastIndex(candidate, ":")
	if idx <= 0 || idx == len(candidate)-1 {
		return "", "", fmt.Errorf("解析系统代理地址失败: %s", candidate)
	}
	return candidate[:idx], candidate[idx+1:], nil
}
