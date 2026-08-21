package douyinadapter

import (
	"wx_channel/internal/config"
)

// DouyinPluginConfig implements config.Configurable for douyin plugin config.
type DouyinPluginConfig struct {
	Cookie string
}

func (c *DouyinPluginConfig) ConfigNamespace() string { return "douyin" }

func (c *DouyinPluginConfig) ConfigSchema() []config.ConfigField {
	return []config.ConfigField{
		{
			Key:         "cookie",
			Type:        config.ConfigTypeText,
			Default:     "",
			Description: "抖音请求 Cookie，用于 web 端 API 回退（mobile 端优先，不需要 cookie）；不会输出到日志",
			Title:       "抖音 Cookie",
			Group:       "Douyin",
			HotReload:   true,
		},
	}
}

func (c *DouyinPluginConfig) ApplyConfig(sub *config.SubViper) error {
	c.Cookie = sub.GetString("cookie")
	return nil
}

func init() {
	config.RegisterPlugin(&DouyinPluginConfig{})
}
