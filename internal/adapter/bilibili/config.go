package bilibiliadapter

import (
	"wx_channel/internal/config"
	"wx_channel/pkg/configapi"
)

// BilibiliPluginConfig implements config.Configurable for bilibili plugin config.
type BilibiliPluginConfig struct {
	Enabled bool
	Cookie  string
}

func (c *BilibiliPluginConfig) ConfigNamespace() string { return "bilibili" }

func (c *BilibiliPluginConfig) ConfigSchema() []configapi.Item {
	return []configapi.Item{
		{
			Key:         "enabled",
			Type:        configapi.TypeBool,
			Default:     false,
			Description: "是否记录 B 站页面浏览记录",
			Title:       "记录 B 站浏览",
			Group:       "Bilibili",
		},
		{
			Key:         "cookie",
			Type:        configapi.TypeText,
			Default:     "",
			Description: "B 站请求 Cookie，用于访问账号可看的高清清晰度；不会输出到日志",
			Title:       "B 站 Cookie",
			Group:       "Bilibili",
			Reload:      configapi.ReloadHot,
		},
	}
}

func (c *BilibiliPluginConfig) ApplyConfig(sub *config.ScopedConfig) error {
	c.Enabled = sub.GetBool("enabled")
	c.Cookie = sub.GetString("cookie")
	return nil
}

// GetBilibiliConfig returns the registered bilibili plugin config if available.
func GetBilibiliConfig() *BilibiliPluginConfig {
	return bilibiliPluginConfig
}

var bilibiliPluginConfig *BilibiliPluginConfig

func init() {
	bilibiliPluginConfig = &BilibiliPluginConfig{}
	config.RegisterPlugin(bilibiliPluginConfig)
}
