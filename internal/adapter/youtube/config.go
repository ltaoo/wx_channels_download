package youtube

import (
	"wx_channel/internal/config"
)

// YouTubePluginConfig implements config.Configurable for youtube plugin config.
type YouTubePluginConfig struct {
	Enabled bool
	Cookie  string
	PoToken string
}

func (c *YouTubePluginConfig) ConfigNamespace() string { return "youtube" }

func (c *YouTubePluginConfig) ConfigSchema() []config.ConfigItem {
	return []config.ConfigItem{
		{
			Key:         "enabled",
			Type:        config.ConfigTypeBool,
			Default:     false,
			Description: "是否记录 YouTube 页面浏览记录",
			Title:       "记录 YouTube 浏览",
			Group:       "YouTube",
		},
		{
			Key:         "cookie",
			Type:        config.ConfigTypeText,
			Default:     "",
			Description: "YouTube 请求 Cookie，用于访问需要登录态的视频；不会输出到日志",
			Title:       "YouTube Cookie",
			Group:       "YouTube",
			HotReload:   true,
		},
		{
			Key:         "poToken",
			Type:        config.ConfigTypeText,
			Default:     "",
			Description: "YouTube GVS PO Token，兼容 yt-dlp 的 client.gvs+TOKEN 格式；用于避免部分 videoplayback 403",
			Title:       "YouTube PO Token",
			Group:       "YouTube",
			HotReload:   true,
		},
	}
}

func (c *YouTubePluginConfig) ApplyConfig(sub *config.SubViper) error {
	c.Enabled = sub.GetBool("enabled")
	c.Cookie = sub.GetString("cookie")
	c.PoToken = sub.GetString("poToken")
	return nil
}

// GetYouTubeConfig returns the registered youtube plugin config if available.
func GetYouTubeConfig() *YouTubePluginConfig {
	return youtubePluginConfig
}

var youtubePluginConfig *YouTubePluginConfig

func init() {
	youtubePluginConfig = &YouTubePluginConfig{}
	config.RegisterPlugin(youtubePluginConfig)
}
