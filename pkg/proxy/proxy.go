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

func EnableProxy(arg ProxySettings) error {
	return enableProxy(arg)
}

func DisableProxy(arg ProxySettings) error {
	return disableProxy(arg)
}
