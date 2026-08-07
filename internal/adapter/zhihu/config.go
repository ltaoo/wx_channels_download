package zhihuadapter

import (
	"wx_channel/internal/config"
	"wx_channel/pkg/configapi"
)

// ZhihuPluginConfig implements config.Configurable for zhihu plugin config.
type ZhihuPluginConfig struct {
	Enabled bool
	Cookie  string
}

func (c *ZhihuPluginConfig) ConfigNamespace() string { return "zhihu" }

func (c *ZhihuPluginConfig) ConfigSchema() []configapi.Item {
	return []configapi.Item{
		{
			Key:         "enabled",
			Type:        configapi.TypeBool,
			Default:     false,
			Description: "是否记录知乎页面浏览记录",
			Title:       "记录知乎浏览",
			Group:       "Zhihu",
		},
		{
			Key:         "cookie",
			Type:        configapi.TypeText,
			Default:     "",
			Description: "知乎请求 Cookie，用于访问需要登录态的知乎接口",
			Title:       "知乎 Cookie",
			Group:       "Zhihu",
			Reload:      configapi.ReloadHot,
		},
	}
}

func (c *ZhihuPluginConfig) ApplyConfig(sub *config.ScopedConfig) error {
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
