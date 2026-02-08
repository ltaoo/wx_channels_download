//go:build windows

package system

import (
	"errors"
	"fmt"
	"os/exec"
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

func get_network_interfaces() (*HardwarePort, error) {
	return nil, errors.New("not support")
}
