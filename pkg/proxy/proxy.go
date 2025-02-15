package proxy

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type ProxySettings struct {
	Device   string
	Hostname string
	Port     string
}

func (p ProxySettings) WithDefaults() ProxySettings {
	if p.Device == "" {
		p.Device = "Wi-Fi" // 默认使用 Wi-Fi 设备
		device, err := getNetworkInterfaces()
		if err == nil {
			p.Device = device.Port
		}
	}
	if p.Hostname == "" {
		p.Hostname = "127.0.0.1"
	}
	if p.Port == "" {
		p.Port = "2023"
	}
	return p
}

type HardwarePort struct {
	Device    string
	Port      string
	Interface string
}

func EnableProxyInMacOS(args ProxySettings) error {
	args = args.WithDefaults()
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

func DisableProxyInMacOS(args ProxySettings) error {
	args = args.WithDefaults()
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

func getNetworkInterfaces() (*HardwarePort, error) {
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
