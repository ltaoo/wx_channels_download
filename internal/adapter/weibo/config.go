package weiboadapter

import (
	"wx_channel/internal/config"
)

// WeiboPluginConfig implements config.Configurable for weibo plugin config.
type WeiboPluginConfig struct {
	Enabled bool
	Cookie  string
}

func (c *WeiboPluginConfig) ConfigNamespace() string { return "weibo" }

func (c *WeiboPluginConfig) ConfigSchema() []config.ConfigItem {
	return []config.ConfigItem{
		{
			Key:         "enabled",
			Type:        config.ConfigTypeBool,
			Default:     false,
			Description: "是否记录微博页面浏览记录",
			Title:       "记录微博浏览",
			Group:       "Weibo",
		},
		{
			Key:         "cookie",
			Type:        config.ConfigTypeText,
			Default:     "",
			Description: "微博请求 Cookie，用于访问需要登录态的微博列表接口；不会输出到日志",
			Title:       "微博 Cookie",
			Group:       "Weibo",
			HotReload:   true,
		},
	}
}

func (c *WeiboPluginConfig) ApplyConfig(sub *config.SubViper) error {
	c.Enabled = sub.GetBool("enabled")
	c.Cookie = sub.GetString("cookie")
	return nil
}

// GetWeiboConfig returns the registered weibo plugin config if available.
func GetWeiboConfig() *WeiboPluginConfig {
	return weiboPluginConfig
}

var weiboPluginConfig *WeiboPluginConfig

func init() {
	weiboPluginConfig = &WeiboPluginConfig{}
	config.RegisterPlugin(weiboPluginConfig)
}
