package wxchannels

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/frontend"
	"wx_channel/internal/config"
	"wx_channel/internal/events"
	"wx_channel/internal/interceptor"
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

// RegisterPlugins adds the video-channel scraper plugins and translates their
// profile callback into adapter-owned persistence and browse record events.
func (c *InterceptorPluginConfig) RegisterPlugins(target *interceptor.Interceptor, db *gorm.DB, logger zerolog.Logger, bus *events.Bus) {
	if c == nil || c.settings == nil || target == nil {
		return
	}

	onFeedProfileLoaded := func(profile *scraper.MediaProfile) {
		HandleFeedProfileLoaded(db, logger, profile)
		if bus != nil {
			bus.Publish(events.BrowseHistoryRecorded{
				Browse: BuildBrowseRecord(profile),
			})
		}
	}
	for _, plugin := range scraper.CreateInterceptorPlugins(c.settings, frontend.Assets, onFeedProfileLoaded) {
		target.AddPostPlugin(plugin)
	}
}
