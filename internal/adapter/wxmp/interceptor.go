package wxmp

import (
	"wx_channel/frontend"
	"wx_channel/internal/adapterctx"
	"wx_channel/internal/config"
	"wx_channel/internal/events"
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
		settings: scraper.NewOfficialAccountConfig(cfg, false),
		version:  cfg.Version,
	}
}

// GetPlugins returns official-account injection and callback plugins wired to
// adapter-owned persistence and browse events.
func (c *InterceptorPluginConfig) GetPlugins(ctx adapterctx.AdapterContext) []interface{} {
	if c == nil || c.settings == nil {
		return nil
	}

	onArticleLoaded := func(profile *scraper.OfficialAccountArticleProfile) {
		HandleArticleProfileLoaded(ctx.DB, ctx.Logger, profile)
		if ctx.Bus != nil {
			ctx.Bus.Publish(events.BrowseHistoryRecorded{Browse: BuildBrowseRecord(profile)})
		}
	}
	return []interface{}{
		scraper.CreateOfficialAccountArticleLoadedPlugin(onArticleLoaded),
		scraper.CreateOfficialAccountInterceptorPlugin(c.settings, frontend.Assets, c.version),
	}
}
