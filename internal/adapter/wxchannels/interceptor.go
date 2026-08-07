package wxchannelsadapter

import (
	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	"wx_channel/pkg/scraper/wxchannels"
)

// InterceptorPluginConfig contains the video-channel interceptor configuration.
// It keeps scraper-specific settings out of the application startup layer.
type InterceptorPluginConfig struct {
	settings *wxchannels.ChannelsConfig
}

func NewConfig(cfg *config.Config) *InterceptorPluginConfig {
	return &InterceptorPluginConfig{settings: wxchannels.NewChannelsConfig(cfg)}
}

// GetPlugins returns the video-channel scraper plugins with callbacks wired
// to adapter-owned persistence and browse record events.
func (c *InterceptorPluginConfig) GetPlugins(ctx adapter.AdapterContext) []interface{} {
	if c == nil || c.settings == nil {
		return nil
	}

	raw := wxchannels.CreateInterceptorPlugins(c.settings)
	plugins := make([]interface{}, len(raw))
	for i, p := range raw {
		plugins[i] = p
	}
	return plugins
}
