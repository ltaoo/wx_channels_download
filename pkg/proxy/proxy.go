package proxy

type ProxySettings struct {
	Device   string
	Hostname string
	Port     string
}

type HardwarePort struct {
	Device    string
	Port      string
	Interface string
}

func merge_default_settings(p ProxySettings) ProxySettings {
	if p.Device == "" {
		p.Device = "Wi-Fi" // 默认使用 Wi-Fi 设备
		device, err := get_network_interfaces()
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

func EnableProxy(arg ProxySettings) error {
	return enable_proxy(arg)
}

func DisableProxy(arg ProxySettings) error {
	return disable_proxy(arg)
}
