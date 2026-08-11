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

var service_uuid_re = regexp.MustCompile(`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$`)

// scutil_field runs `scutil show <key>` and returns the value of the `<field> : <value>` line.
func scutil_field(key string, field string) (string, error) {
	cmd := exec.Command("scutil")
	cmd.Stdin = strings.NewReader("show " + key + "\n")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("执行 scutil show %s 失败: %v", key, err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.TrimSpace(parts[0]) == field {
			return strings.TrimSpace(parts[1]), nil
		}
	}
	return "", nil
}

// global_ipv4_key and global_ipv6_key hold the primary service per protocol. They are
// independent: an IPv6-only network has no IPv4 global key at all, so both are consulted.
const (
	global_ipv4_key = "State:/Network/Global/IPv4"
	global_ipv6_key = "State:/Network/Global/IPv6"
)

// get_network_interfaces returns the primary network service. Port holds the service name that
// can be passed straight to networksetup (such as "Wi-Fi"), and Device holds the matching
// interface name (such as "en0" or "utun4").
//
// This deliberately avoids looking the primary interface up in
// `networksetup -listallhardwareports`. That list only covers physical hardware, so a VPN
// service does not appear in it at all and a utun primary interface never matches, leaving the
// caller on a hardcoded fallback service. Since macOS stores the proxy per network service and
// only the primary service applies, that fallback writes the proxy somewhere that is not in
// effect, and networksetup reports no error. The list is also keyed by hardware port name,
// while networksetup addresses services by service name; the two coincide for physical
// adapters on a default setup but are separate fields. Reading the primary service from the
// configuration store avoids both problems and treats physical adapters and VPNs alike.
func get_network_interfaces() (*HardwarePort, error) {
	for _, global_key := range []string{global_ipv4_key, global_ipv6_key} {
		uuid, err := scutil_field(global_key, "PrimaryService")
		if err != nil {
			return nil, err
		}
		if uuid == "" {
			continue
		}
		// The value is interpolated into the next command, so validate its shape first.
		if !service_uuid_re.MatchString(uuid) {
			return nil, fmt.Errorf("主网络服务标识格式异常: %q", uuid)
		}
		name, err := scutil_field("Setup:/Network/Service/"+uuid, "UserDefinedName")
		if err != nil {
			return nil, err
		}
		if name == "" {
			return nil, fmt.Errorf("主网络服务 %s 没有名称", uuid)
		}
		iface, _ := scutil_field(global_key, "PrimaryInterface")
		return &HardwarePort{Port: name, Device: iface, Interface: iface}, nil
	}
	return nil, fmt.Errorf("未找到主网络服务，当前可能没有活动网络")
}

// ProxyTargetDescription reports the network service the system proxy will be written to,
// along with a warning to surface when detection had to fall back. macOS keeps proxy settings
// per network service and only honours the primary one, so a wrong guess leaves interception
// silently inactive with no error from networksetup.
func ProxyTargetDescription(configured string) (service string, warning string) {
	if configured != "" {
		return configured, ""
	}
	port, err := get_network_interfaces()
	if err != nil {
		return default_network_service, fmt.Sprintf(
			"could not determine the primary network service (%v), falling back to %s.\n"+
				"If the download button never appears (common while a VPN is connected), pass --dev "+
				"with the service name; run `networksetup -listallnetworkservices` to list them",
			err, default_network_service)
	}
	return port.Port, ""
}
