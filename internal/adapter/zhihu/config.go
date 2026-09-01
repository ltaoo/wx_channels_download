package zhihuadapter

import (
	"wx_channel/internal/config"
	"wx_channel/pkg/scraper/zhihu"
)

// ZhihuPluginConfig implements config.Configurable for zhihu.
type ZhihuPluginConfig struct{}

func (c *ZhihuPluginConfig) ConfigNamespace() string { return "zhihu" }

func (c *ZhihuPluginConfig) ConfigSchema() []config.ConfigField { return nil }

func (c *ZhihuPluginConfig) ApplyConfig(_ *config.SubViper) error { return nil }

func new_interceptor_config(cfg *config.Config) zhihu.InterceptorConfig {
	api_protocol := cfg.GetString("api.protocol")
	api_bind_hostname := cfg.GetString("api.hostname")
	api_port := cfg.GetInt("api.port")
	return zhihu.InterceptorConfig{
		Version:          cfg.Version,
		Mode:             cfg.Mode,
		GlobalScriptPath: cfg.GlobalScriptPath,
		FrontendVariables: map[string]any{
			"apiHost":     config.APIClientHost(api_bind_hostname, api_port),
			"apiOrigin":   config.APIClientOrigin(api_protocol, api_bind_hostname, api_port),
			"apiProtocol": api_protocol,
		},
	}
}

func init() {
	config.RegisterPlugin(&ZhihuPluginConfig{})
}
