package wxchannels

import (
	"wx_channel/frontend"
	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	scraper "wx_channel/pkg/scraper/wxchannels"
)

// InterceptorPluginConfig contains the video-channel interceptor configuration.
// It keeps scraper-specific settings out of the application startup layer.
type InterceptorPluginConfig struct {
	settings *scraper.InterceptorConfig
}

func NewConfig(cfg *config.Config) *InterceptorPluginConfig {
	return &InterceptorPluginConfig{settings: scraper.NewInterceptorSettings(cfg)}
}

// GetPlugins returns the video-channel scraper plugins with callbacks wired
// to adapter-owned persistence and browse record events.
func (c *InterceptorPluginConfig) GetPlugins(ctx adapter.AdapterContext) []interface{} {
	if c == nil || c.settings == nil {
		return nil
	}

	raw := scraper.CreateInterceptorPlugins(c.settings, frontend.Assets, nil)
	plugins := make([]interface{}, len(raw))
	for i, p := range raw {
		plugins[i] = p
	}
	return plugins
}
