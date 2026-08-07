package wxmp

import (
	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	scraper "wx_channel/pkg/scraper/wxmp"
)

// InterceptorPluginConfig owns the official-account scraper configuration used
// by the local interceptor.
type InterceptorPluginConfig struct {
	settings *scraper.OfficialAccountConfig
	version  string
}

func NewConfig(cfg *config.Config) *InterceptorPluginConfig {
	if cfg == nil {
		return &InterceptorPluginConfig{}
	}
	return &InterceptorPluginConfig{
		settings: scraper.NewOfficialAccountConfig(cfg),
		version:  cfg.Version,
	}
}

// GetPlugins returns the official-account injection plugin.
func (c *InterceptorPluginConfig) GetPlugins(ctx adapter.AdapterContext) []interface{} {
	if c == nil || c.settings == nil {
		return nil
	}

	return []interface{}{
		scraper.CreateOfficialAccountInterceptorPlugin(c.settings, c.version),
	}
}
