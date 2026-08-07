package douyinadapter

import (
	"wx_channel/internal/config"
	"wx_channel/pkg/configapi"
)

// DouyinPluginConfig implements config.Configurable for douyin plugin config.
type DouyinPluginConfig struct {
	Enabled bool
	Cookie  string
}

func (c *DouyinPluginConfig) ConfigNamespace() string { return "douyin" }

func (c *DouyinPluginConfig) ConfigSchema() []configapi.Item {
	return []configapi.Item{
		{
			Key:         "enabled",
			Type:        configapi.TypeBool,
			Default:     false,
			Description: "是否启用抖音视频解析和下载",
			Title:       "启用抖音下载",
			Group:       "Douyin",
		},
		{
			Key:         "cookie",
			Type:        configapi.TypeText,
			Default:     "",
			Description: "抖音请求 Cookie，用于 web 端 API 回退（mobile 端优先，不需要 cookie）；不会输出到日志",
			Title:       "抖音 Cookie",
			Group:       "Douyin",
			Reload:      configapi.ReloadHot,
		},
	}
}

func (c *DouyinPluginConfig) ApplyConfig(sub *config.ScopedConfig) error {
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
