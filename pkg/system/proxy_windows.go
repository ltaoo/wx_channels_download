//go:build windows

package system

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

func enable_proxy(args ProxySettings) error {
	args = merge_default_settings(args)
	path := `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	proxy_server_url := fmt.Sprintf("%v:%v", args.Hostname, args.Port)
	// # 启用代理
	// reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyEnable /t REG_DWORD /d 1 /f
	// reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyServer /t REG_SZ /d "127.0.0.1:8080" /f

	// 使用 reg 命令替代 powershell，以提高兼容性（支持 Win7）并提升性能
	cmd := exec.Command("reg", "add", path, "/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "1", "/f")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("设置系统代理时发生错误，%v\n", string(output))
	}

	cmd = exec.Command("reg", "add", path, "/v", "ProxyServer", "/t", "REG_SZ", "/d", proxy_server_url, "/f")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("设置 HTTP 代理失败，%v", string(output))
	}
	return nil
}

func disable_proxy(args ProxySettings) error {
	path := `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	// # 禁用代理
	// reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings" /v ProxyEnable /t REG_DWORD /d 0 /f

	// 使用 reg 命令替代 powershell
	cmd := exec.Command("reg", "add", path, "/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "0", "/f")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("设置 HTTP 代理失败，%v", string(output))
	}
	return nil
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
