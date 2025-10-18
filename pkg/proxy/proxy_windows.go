//go:build windows

package proxy

import (
	"errors"
	"fmt"
	"os/exec"
)

func enable_proxy(args ProxySettings) error {
	args = merge_default_settings(args)
	path := `HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	proxy_server_url := fmt.Sprintf("%v:%v", args.Hostname, args.Port)
	// # 启用代理
	// Set-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings" -Name ProxyEnable -Value 1
	// Set-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings" -Name ProxyServer -Value "127.0.0.1:8080"
	cmd := fmt.Sprintf(`Set-ItemProperty -Path "%v" -Name ProxyEnable -Value 1`, path)
	ps := exec.Command("powershell.exe", "-Command", cmd)
	output, err := ps.CombinedOutput()
	if err != nil {
		return errors.New(fmt.Sprintf("设置系统代理时发生错误，%v\n", string(output)))
	}
	cmd = fmt.Sprintf(`Set-ItemProperty -Path "%v" -Name ProxyServer -Value %v`, path, proxy_server_url)
	ps = exec.Command("powershell.exe", "-Command", cmd)
	output, err = ps.CombinedOutput()
	if err != nil {
		return fmt.Errorf("设置 HTTP 代理失败，%v", string(output))
	}
	return nil
}

func disable_proxy(args ProxySettings) error {
	path := `HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	// # 禁用代理
	// Set-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings" -Name ProxyEnable -Value 0
	cmd := fmt.Sprintf(`Set-ItemProperty -Path "%v" -Name ProxyEnable -Value 0`, path)
	ps := exec.Command("powershell.exe", "-Command", cmd)
	output, err := ps.CombinedOutput()
	if err != nil {
		return fmt.Errorf("设置 HTTP 代理失败，%v", string(output))
	}
	return nil
}

func get_network_interfaces() (*HardwarePort, error) {
	return nil, errors.New("not support")
}
