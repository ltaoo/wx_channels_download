package douyinadapter

import (
	"wx_channel/internal/config"
)

// DouyinPluginConfig implements config.Configurable for douyin plugin config.
type DouyinPluginConfig struct {
	Enabled bool
	Cookie  string
}

func (c *DouyinPluginConfig) ConfigNamespace() string { return "douyin" }

func (c *DouyinPluginConfig) ConfigSchema() []config.ConfigField {
	return []config.ConfigField{
		{
			Key:         "enabled",
			Type:        config.ConfigTypeBool,
			Default:     false,
			Description: "是否启用抖音视频解析和下载",
			Title:       "启用抖音下载",
			Group:       "Douyin",
		},
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
	c.Enabled = sub.GetBool("enabled")
	c.Cookie = sub.GetString("cookie")
	return nil
}

// GetDouyinConfig returns the registered douyin plugin config if available.
func GetDouyinConfig() *DouyinPluginConfig {
	return douyinPluginConfig
}

var douyinPluginConfig *DouyinPluginConfig

func init() {
	douyinPluginConfig = &DouyinPluginConfig{}
	config.RegisterPlugin(douyinPluginConfig)
}
