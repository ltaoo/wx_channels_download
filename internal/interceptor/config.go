package interceptor

import (
	"github.com/spf13/viper"

	"wx_channel/internal/config"
)

type InterceptorConfig struct {
	Version                  string
	Debug                    bool
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

func NewInterceptorSettings(c *config.Config) *InterceptorConfig {
	return &InterceptorConfig{
		Version:                  c.Version,
		Debug:                    viper.GetBool("debug.error"),
		ProxySetSystem:           viper.GetBool("proxy.system"),
		ProxyTun:                 viper.GetBool("proxy.tun"),
		ProxyDefaultInterface:    viper.GetString("proxy.defaultInterface"),
		ProxyServerPort:          viper.GetInt("proxy.port"),
		ProxyServerHostname:      viper.GetString("proxy.hostname"),
		ProxyTCPRelayEnabled:     viper.GetBool("proxy.tcpRelay.enabled"),
		ProxyTCPRelayHostname:    viper.GetString("proxy.tcpRelay.hostname"),
		ProxyTCPRelayPort:        viper.GetInt("proxy.tcpRelay.port"),
		ProxySkipInstallRootCert: viper.GetBool("proxy.skipInstallRootCert"),
		ProxyUpstreamProxy:       viper.GetString("proxy.upstreamProxy"),
	}
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
