package zhihuadapter

import (
	"wx_channel/internal/config"
)

// ZhihuPluginConfig implements config.Configurable for zhihu plugin config.
type ZhihuPluginConfig struct {
	Enabled bool
	Cookie  string
}

func (c *ZhihuPluginConfig) ConfigNamespace() string { return "zhihu" }

func (c *ZhihuPluginConfig) ConfigSchema() []config.ConfigItem {
	return []config.ConfigItem{
		{
			Key:         "enabled",
			Type:        config.ConfigTypeBool,
			Default:     false,
			Description: "是否记录知乎页面浏览记录",
			Title:       "记录知乎浏览",
			Group:       "Zhihu",
		},
		{
			Key:         "cookie",
			Type:        config.ConfigTypeText,
			Default:     "",
			Description: "知乎请求 Cookie，用于访问需要登录态的知乎接口",
			Title:       "知乎 Cookie",
			Group:       "Zhihu",
			HotReload:   true,
		},
	}
}

func (c *ZhihuPluginConfig) ApplyConfig(sub *config.SubViper) error {
	c.Enabled = sub.GetBool("enabled")
	c.Cookie = sub.GetString("cookie")
	return nil
}

// GetZhihuConfig returns the registered zhihu plugin config if available.
func GetZhihuConfig() *ZhihuPluginConfig {
	return zhihuPluginConfig
}

var zhihuPluginConfig *ZhihuPluginConfig

func init() {
	zhihuPluginConfig = &ZhihuPluginConfig{}
	config.RegisterPlugin(zhihuPluginConfig)
}
