package wxchannelsadapter

import (
	"github.com/spf13/viper"

	"wx_channel/internal/config"
)

// ChannelsPluginConfig implements config.Configurable for wxchannels plugin config.
type ChannelsPluginConfig struct {
	DisableLocationToHome      bool
	RefreshInterval            int
	DownloadDefaultHighest     bool
	DownloadCover              bool
	DownloadFrontend           bool
	DownloadForceCheckAllFeeds bool
	DownloadPauseWhenDownload  bool
}

func (c *ChannelsPluginConfig) ConfigNamespace() string { return "channels" }

func (c *ChannelsPluginConfig) ConfigSchema() []config.ConfigField {
	return []config.ConfigField{
		{
			Key:         "disableLocationToHome",
			Type:        config.ConfigTypeBool,
			Default:     false,
			Description: "是否禁止从视频号详情页重定向到首页（视频号默认行为）",
			Title:       "禁止重定向",
			Group:       "Channels",
			HotReload:   true,
		},
		{
			Key:         "refreshInterval",
			Type:        config.ConfigTypeInt,
			Default:     0,
			Description: "视频号页面定时刷新时间间隔（秒），0 为不刷新",
			Title:       "定时刷新间隔",
			Group:       "Channels",
			HotReload:   true,
		},
		{
			Key:         "download.defaultHighest",
			Type:        config.ConfigTypeBool,
			Default:     false,
			Description: "点击下载图标时是否下载原始视频（该配置不再生效）",
			Title:       "原始视频",
			Group:       "Channels",
			HotReload:   true,
		},
		{
			Key:         "download.frontend",
			Type:        config.ConfigTypeBool,
			Default:     false,
			Description: "是否通过前端解密、下载，不调用后台下载能力",
			Title:       "前端下载",
			Group:       "Channels",
			HotReload:   true,
		},
		{
			Key:         "download.cover",
			Type:        config.ConfigTypeBool,
			Default:     false,
			Description: "下载视频或图集时是否同时下载封面",
			Title:       "同时下载封面",
			Group:       "Channels",
			HotReload:   true,
		},
		{
			Key:         "download.forceCheckAllFeeds",
			Type:        config.ConfigTypeBool,
			Default:     false,
			Description: "批量下载时是否强制检查所有视频",
			Title:       "检查所有视频",
			Group:       "Channels",
			HotReload:   true,
		},
		{
			Key:         "download.pauseWhenDownload",
			Type:        config.ConfigTypeBool,
			Default:     false,
			Description: "点击下载时是否暂停播放",
			Title:       "暂停播放",
			Group:       "Channels",
			HotReload:   true,
		},
	}
}

func (c *ChannelsPluginConfig) ApplyConfig(sub *config.SubViper) error {
	c.DisableLocationToHome = sub.GetBool("disableLocationToHome")
	c.RefreshInterval = sub.GetInt("refreshInterval")
	c.DownloadDefaultHighest = sub.GetBool("download.defaultHighest") || viper.GetBool("download.defaultHighest")
	c.DownloadCover = sub.GetBool("download.cover")
	c.DownloadFrontend = sub.GetBool("download.frontend")
	c.DownloadForceCheckAllFeeds = sub.GetBool("download.forceCheckAllFeeds")
	c.DownloadPauseWhenDownload = sub.GetBool("download.pauseWhenDownload")
	return nil
}

// channels_plugin_config is the singleton instance populated during config loading.
var channels_plugin_config = &ChannelsPluginConfig{}

func init() {
	config.RegisterPlugin(channels_plugin_config)

	// Legacy alias for backward compatibility; registered with its flat key directly
	// to avoid the namespace auto-prefix applied by LoadPluginConfigs.
	config.Register(config.ConfigField{
		Key:         "channel.disableLocationToHome",
		Type:        config.ConfigTypeBool,
		Default:     false,
		Description: "是否禁止从视频号详情页重定向到首页（视频号默认行为）",
		Title:       "禁止重定向",
		Group:       "Channels",
		HotReload:   true,
	})
}
