package youtubeadapter

import (
	"wx_channel/internal/config"
)

// YouTubePluginConfig implements config.Configurable for youtube plugin config.
type YouTubePluginConfig struct {
	Cookie  string
	PoToken string
}

func (c *YouTubePluginConfig) ConfigNamespace() string { return "youtube" }

func (c *YouTubePluginConfig) ConfigSchema() []config.ConfigField {
	return []config.ConfigField{
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
			Description: "YouTube PO Token：视频使用 client.gvs+TOKEN，网页字幕使用 web.subs+TOKEN；多个 Token 用逗号分隔，须匹配视频及会话，字幕 Token 不能用视频 Token 代替",
			Title:       "YouTube PO Token",
			Group:       "YouTube",
			HotReload:   true,
		},
	}
}

func (c *YouTubePluginConfig) ApplyConfig(sub *config.SubViper) error {
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
