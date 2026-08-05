package zhihu

import (
	"wx_channel/frontend"
	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	scraper "wx_channel/pkg/scraper/zhihu"
)

// InterceptorPluginConfig owns the zhihu scraper configuration used
// by the local interceptor.
type InterceptorPluginConfig struct {
	settings *ZhihuPluginConfig
	version  string
}

// NewConfig creates an InterceptorPluginConfig from the application config.
func NewConfig(cfg *config.Config) *InterceptorPluginConfig {
	if cfg == nil {
		return &InterceptorPluginConfig{}
	}
	return &InterceptorPluginConfig{
		settings: GetZhihuConfig(),
		version:  cfg.Version,
	}
}

// GetPlugins returns zhihu injection and callback plugins wired to
// adapter-owned persistence and browse events.
func (c *InterceptorPluginConfig) GetPlugins(ctx adapter.AdapterContext) []interface{} {
	if c == nil || c.settings == nil || !c.settings.Enabled {
		return nil
	}

	// Create zhihu interceptor plugin for injecting zhihu.main.js
	plugin := scraper.CreateZhihuInterceptorPlugin(c.settings.Cookie, frontend.Assets, c.version)
	if plugin == nil {
		return nil
	}
	return []interface{}{plugin}
}
