package wxmp

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/frontend"
	"wx_channel/internal/config"
	"wx_channel/internal/events"
	"wx_channel/internal/interceptor"
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

// RegisterPlugins adds official-account injection and callback plugins. The
// callback is translated to adapter-owned persistence and browse events.
func (c *InterceptorPluginConfig) RegisterPlugins(target *interceptor.Interceptor, db *gorm.DB, logger zerolog.Logger, bus *events.Bus) {
	if c == nil || c.settings == nil || target == nil {
		return
	}

	onArticleLoaded := func(profile *scraper.OfficialAccountArticleProfile) {
		HandleArticleProfileLoaded(db, logger, profile)
		if bus != nil {
			bus.Publish(events.BrowseHistoryRecorded{Browse: BuildBrowseRecord(profile)})
		}
	}
	target.AddPostPlugin(scraper.CreateOfficialAccountArticleLoadedPlugin(onArticleLoaded))
	target.AddPostPlugin(scraper.CreateOfficialAccountInterceptorPlugin(c.settings, frontend.Assets, c.version))
}
