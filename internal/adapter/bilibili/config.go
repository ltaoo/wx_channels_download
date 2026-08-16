package bilibiliadapter

import (
	"wx_channel/internal/config"
)

// BilibiliPluginConfig implements config.Configurable for bilibili plugin config.
type BilibiliPluginConfig struct {
	Enabled bool
	Cookie  string
}

func (c *BilibiliPluginConfig) ConfigNamespace() string { return "bilibili" }

func (c *BilibiliPluginConfig) ConfigSchema() []config.ConfigField {
	return []config.ConfigField{
		{
			Key:         "enabled",
			Type:        config.ConfigTypeBool,
			Default:     false,
			Description: "是否记录 B 站页面浏览记录",
			Title:       "记录 B 站浏览",
			Group:       "Bilibili",
		},
		{
			Key:         "cookie",
			Type:        config.ConfigTypeText,
			Default:     "",
			Description: "B 站请求 Cookie，用于访问账号可看的高清清晰度；不会输出到日志",
			Title:       "B 站 Cookie",
			Group:       "Bilibili",
			HotReload:   true,
		},
	}
}

func (c *BilibiliPluginConfig) ApplyConfig(sub *config.SubViper) error {
	c.Enabled = sub.GetBool("enabled")
	c.Cookie = sub.GetString("cookie")
	return nil
}

func init() {
	config.RegisterPlugin(&BilibiliPluginConfig{})
}
