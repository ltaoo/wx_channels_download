//go:build windows

package proxy

import (
	"errors"
	"fmt"
	"os/exec"
)

func enable_proxy(args ProxySettings) error {
	args = merge_default_settings(args)
	// netsh winhttp set proxy 127.0.0.1:8080
	path := `HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	proxy_server_url := fmt.Sprintf("%v:%v", args.Hostname, args.Port)
	// # 启用代理
	// Set-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings" -Name ProxyEnable -Value 1
	// Set-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings" -Name ProxyServer -Value "127.0.0.1:8080"
	// cmd1 := exec.Command("netsh", "winhttp", "set", "proxy", fmt.Sprintf("%v:%v", args.Hostname, args.Port))
	cmd1 := exec.Command("Set-ItemProperty", "-Path", path, "-Name", "ProxyEnable", "-Value", "1")
	//  fmt.Sprintf("%v:%v", args.Hostname, args.Port)
	_, err := cmd1.Output()
	if err != nil {
		return fmt.Errorf("设置 HTTP 代理失败，%v", err.Error())
	}
	cmd2 := exec.Command("Set-ItemProperty", "-Path", path, "-Name", "ProxyServer", "-Value", proxy_server_url)
	_, err = cmd2.Output()
	if err != nil {
		return fmt.Errorf("设置 HTTP 代理失败，%v", err.Error())
	}
	return nil
}

func disable_proxy(args ProxySettings) error {
	path := `HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	// # 禁用代理
	// Set-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings" -Name ProxyEnable -Value 0
	cmd1 := exec.Command("Set-ItemProperty", "-Path", path, "-Name", "ProxyEnable", "-Value", "0")
	//  fmt.Sprintf("%v:%v", args.Hostname, args.Port)
	_, err := cmd1.Output()
	if err != nil {
		return fmt.Errorf("设置 HTTP 代理失败，%v", err.Error())
	}
	return nil
}

func get_network_interfaces() (*HardwarePort, error) {
	return nil, errors.New("not support")
}
