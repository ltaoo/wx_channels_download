package system

import "strings"

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

// default_network_service is the fallback used when the primary network service cannot be
// determined. Only macOS reads it: Windows writes the proxy to the registry and Linux goes
// through gsettings, and both are global rather than per network service.
const default_network_service = "Wi-Fi"

func merge_default_settings(p ProxySettings) ProxySettings {
	if p.Device == "" {
		p.Device = default_network_service
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

func FetchCurProxy(arg ProxySettings) (*ProxySettings, error) {
	return fetch_cur_proxy(arg)
}

// DisableProxyIfMatches disables the system proxy only when it still points
// at the supplied address. This avoids overwriting a proxy the user selected
// while the application was running.
func DisableProxyIfMatches(expected ProxySettings) (bool, error) {
	current, err := FetchCurProxy(expected)
	if err != nil || current == nil {
		return false, err
	}
	if !same_proxy_address(*current, expected) {
		return false, nil
	}
	if err := DisableProxy(expected); err != nil {
		return false, err
	}
	return true, nil
}

func same_proxy_address(current ProxySettings, expected ProxySettings) bool {
	return strings.EqualFold(strings.TrimSpace(current.Hostname), strings.TrimSpace(expected.Hostname)) &&
		strings.TrimSpace(current.Port) == strings.TrimSpace(expected.Port)
}
