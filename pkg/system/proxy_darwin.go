//go:build darwin

package system

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

func enable_proxy(args ProxySettings) error {
	args = merge_default_settings(args)
	cmd1 := exec.Command("networksetup", "-setwebproxy", args.Device, args.Hostname, args.Port)
	_, err1 := cmd1.Output()
	if err1 != nil {
		return fmt.Errorf("设置 HTTP 代理失败，%v", err1.Error())
	}
	cmd2 := exec.Command("networksetup", "-setsecurewebproxy", args.Device, args.Hostname, args.Port)
	output, err2 := cmd2.Output()
	if err2 != nil {
		return fmt.Errorf("设置 HTTPS 代理失败，%v", output)
	}
	return nil
}

func disable_proxy(args ProxySettings) error {
	args = merge_default_settings(args)
	cmd1 := exec.Command("networksetup", "-setwebproxystate", args.Device, "off")
	_, err1 := cmd1.Output()
	if err1 != nil {
		return fmt.Errorf("禁用 HTTP 代理失败，%v", err1.Error())
	}
	cmd2 := exec.Command("networksetup", "-setsecurewebproxystate", args.Device, "off")
	_, err2 := cmd2.Output()
	if err2 != nil {
		return fmt.Errorf("禁用 HTTPS 代理失败，%v", err2.Error())
	}
	return nil
}

func fetch_cur_proxy(args ProxySettings) (*ProxySettings, error) {
	device := args.Device
	if device == "" {
		if port, err := get_network_interfaces(); err == nil && port != nil {
			device = port.Port
		}
	}
	if device == "" {
		device = "Wi-Fi"
	}
	webProxy, err := read_network_proxy(device, false)
	if err != nil {
		return nil, err
	}
	if webProxy.Enabled && webProxy.Server != "" && webProxy.Port != "" {
		return &ProxySettings{
			Device:   device,
			Hostname: webProxy.Server,
			Port:     webProxy.Port,
		}, nil
	}
	secureProxy, err := read_network_proxy(device, true)
	if err != nil {
		return nil, err
	}
	if secureProxy.Enabled && secureProxy.Server != "" && secureProxy.Port != "" {
		return &ProxySettings{
			Device:   device,
			Hostname: secureProxy.Server,
			Port:     secureProxy.Port,
		}, nil
	}
	return nil, nil
}

type network_proxy_info struct {
	Enabled bool
	Server  string
	Port    string
}

func read_network_proxy(device string, secure bool) (*network_proxy_info, error) {
	command := "-getwebproxy"
	if secure {
		command = "-getsecurewebproxy"
	}
	output, err := exec.Command("networksetup", command, device).Output()
	if err != nil {
		return nil, fmt.Errorf("读取系统代理失败，%v", err)
	}
	info := &network_proxy_info{}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		switch key {
		case "enabled":
			info.Enabled = strings.EqualFold(value, "yes")
		case "server":
			info.Server = value
		case "port":
			info.Port = value
		}
	}
	return info, nil
}

func get_network_interfaces() (*HardwarePort, error) {
	// 获取所有硬件端口信息
	cmd := exec.Command("networksetup", "-listallhardwareports")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行 networksetup 命令失败: %v", err)
	}
	// 解析硬件端口信息
	var ports []HardwarePort
	lines := strings.Split(string(output), "\n")

	var cur_port HardwarePort
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Hardware Port:") {
			if cur_port.Port != "" {
				ports = append(ports, cur_port)
			}
			cur_port = HardwarePort{}
			cur_port.Port = strings.TrimPrefix(line, "Hardware Port: ")
		} else if strings.HasPrefix(line, "Device:") {
			cur_port.Device = strings.TrimPrefix(line, "Device: ")
		}
	}
	if cur_port.Port != "" {
		ports = append(ports, cur_port)
	}
	// 获取网络接口信息
	cmd = exec.Command("scutil", "--nwi")
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行 scutil 命令失败: %v", err)
	}
	// 使用正则解析接口信息
	re := regexp.MustCompile(`Network interfaces{0,1}: ([0-9a-zA-Z]{1,})`)
	matches := re.FindStringSubmatch(string(output))
	// 将接口信息与硬件端口匹配
	if len(matches) >= 2 {
		for i := range ports {
			if ports[i].Device == matches[1] {
				return &ports[i], nil
			}
		}
	}
	return nil, fmt.Errorf("未找到硬件端口信息")
}
