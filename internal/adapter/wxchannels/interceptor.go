package wxchannels

import (
	"wx_channel/frontend"
	"wx_channel/internal/adapterctx"
	"wx_channel/internal/config"
	"wx_channel/internal/events"
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

func (c *InterceptorPluginConfig) GlobalScriptFilepath() string {
	if c == nil || c.settings == nil {
		return ""
	}
	return c.settings.InjectGlobalScriptFilepath
}

func (c *InterceptorPluginConfig) HasGlobalScript() bool {
	return c != nil && c.settings != nil && c.settings.InjectGlobalScript != ""
}

// GetPlugins returns the video-channel scraper plugins with callbacks wired
// to adapter-owned persistence and browse record events.
func (c *InterceptorPluginConfig) GetPlugins(ctx adapterctx.AdapterContext) []interface{} {
	if c == nil || c.settings == nil {
		return nil
	}

	onFeedProfileLoaded := func(profile *scraper.MediaProfile) {
		HandleFeedProfileLoaded(ctx.DB, ctx.Logger, profile)
		if ctx.Bus != nil {
			ctx.Bus.Publish(events.BrowseHistoryRecorded{
				Browse: BuildBrowseRecord(profile),
			})
		}
	}
	raw := scraper.CreateInterceptorPlugins(c.settings, frontend.Assets, onFeedProfileLoaded)
	plugins := make([]interface{}, len(raw))
	for i, p := range raw {
		plugins[i] = p
	}
	return plugins
}
