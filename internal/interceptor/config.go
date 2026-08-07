package interceptor

import (
	"fmt"
	"os"
	"path/filepath"

	"wx_channel/pkg/configapi"
)

// ConfigDeclaration explicitly lists every runtime configuration namespace
// consumed by the interceptor module.
var ConfigDeclaration = configapi.Declare("debug", "proxy")

type InterceptorConfig struct {
	Version                  string
	DebugShowError           bool
	EchoLogEnabled           bool
	ProxyDevice              string
	ProxySetSystem           bool
	ProxyTun                 bool
	ProxyDefaultInterface    string
	ProxyServerHostname      string
	ProxyServerPort          int
	ProxyTCPRelayEnabled     bool
	ProxyTCPRelayHostname    string
	ProxyTCPRelayPort        int
	ProxySkipInstallRootCert bool
	ProxyUpstreamProxy       string
}

func NewInterceptorSettings(provider configapi.Provider, runtime configapi.Runtime) (*InterceptorConfig, error) {
	var debug_config struct {
		ShowError bool `json:"error"`
		EchoLog   bool `json:"echolog"`
	}
	if err := ConfigDeclaration.Decode(provider, "debug", &debug_config); err != nil {
		return nil, fmt.Errorf("debug config: %w", err)
	}

	var proxy_config struct {
		Device              string `json:"device"`
		SetSystem           bool   `json:"system"`
		Tun                 bool   `json:"tun"`
		DefaultInterface    string `json:"defaultInterface"`
		Hostname            string `json:"hostname"`
		Port                int    `json:"port"`
		SkipInstallRootCert bool   `json:"skipInstallRootCert"`
		UpstreamProxy       string `json:"upstreamProxy"`
		TCPRelay            struct {
			Enabled  bool   `json:"enabled"`
			Hostname string `json:"hostname"`
			Port     int    `json:"port"`
		} `json:"tcpRelay"`
	}
	if err := ConfigDeclaration.Decode(provider, "proxy", &proxy_config); err != nil {
		return nil, fmt.Errorf("proxy config: %w", err)
	}
	if proxy_config.Hostname == "" {
		proxy_config.Hostname = "127.0.0.1"
	}
	if proxy_config.Port <= 0 {
		proxy_config.Port = 2023
	}
	if proxy_config.TCPRelay.Hostname == "" {
		proxy_config.TCPRelay.Hostname = "127.0.0.1"
	}
	if proxy_config.TCPRelay.Port <= 0 {
		proxy_config.TCPRelay.Port = 9900
	}

	return &InterceptorConfig{
		Version:                  runtime.Version,
		DebugShowError:           debug_config.ShowError,
		EchoLogEnabled:           debug_config.EchoLog,
		ProxyDevice:              proxy_config.Device,
		ProxySetSystem:           proxy_config.SetSystem,
		ProxyTun:                 proxy_config.Tun,
		ProxyDefaultInterface:    proxy_config.DefaultInterface,
		ProxyServerPort:          proxy_config.Port,
		ProxyServerHostname:      proxy_config.Hostname,
		ProxyTCPRelayEnabled:     proxy_config.TCPRelay.Enabled,
		ProxyTCPRelayHostname:    proxy_config.TCPRelay.Hostname,
		ProxyTCPRelayPort:        proxy_config.TCPRelay.Port,
		ProxySkipInstallRootCert: proxy_config.SkipInstallRootCert,
		ProxyUpstreamProxy:       proxy_config.UpstreamProxy,
	}, nil
}

func resolveScriptPath(rootDir, scriptPath string) string {
	if scriptPath == "" || filepath.IsAbs(scriptPath) {
		return scriptPath
	}
	return filepath.Join(rootDir, scriptPath)
}

func readScriptFile(scriptPath string) (string, bool) {
	if scriptPath == "" {
		return "", false
	}
	scriptByte, err := os.ReadFile(scriptPath)
	if err != nil {
		return "", false
	}
	return string(scriptByte), true
}
