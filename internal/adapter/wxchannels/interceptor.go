package wxchannelsadapter

import (
	"github.com/rs/zerolog"

	"wx_channel/internal/adapter"
	"wx_channel/internal/config"
	"wx_channel/pkg/scraper/wxchannels"
)

// InterceptorPluginConfig contains the video-channel interceptor configuration.
// It keeps scraper-specific settings out of the application startup layer.
type InterceptorPluginConfig struct {
	settings *wxchannels.ChannelsConfig
	logger   *zerolog.Logger
}

func NewConfig(cfg *config.Config, logger *zerolog.Logger) *InterceptorPluginConfig {
	return &InterceptorPluginConfig{
		settings: wxchannels.NewChannelsConfig(cfg, logger),
		logger:   logger,
	}
}

// GetPlugins returns the video-channel scraper plugins with callbacks wired
// to adapter-owned persistence and browse record events.
func (c *InterceptorPluginConfig) GetPlugins(ctx adapter.AdapterContext) []interface{} {
	if c == nil || c.settings == nil {
		return nil
	}
	if c.logger != nil {
		c.logger.Info().
			Str("file", "internal/adapter/wxchannels/interceptor.go").
			Bool("global_script_configured", c.settings.GlobalScriptPath != "").
			Str("global_script_path", c.settings.GlobalScriptPath).
			Msg("wxchannels interceptor config: creating proxy plugins")
	}

	raw := wxchannels.CreateInterceptorPlugins(c.settings)
	plugins := make([]interface{}, len(raw))
	if c.logger != nil {
		c.logger.Info().
			Str("file", "internal/adapter/wxchannels/interceptor.go").
			Int("plugin_count", len(raw)).
			Msg("wxchannels interceptor config: proxy plugins created")
	}
	for i, p := range raw {
		plugins[i] = p
	}
	return plugins
}
