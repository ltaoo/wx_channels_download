package xiaohongshu

import (
	"wx_channel/internal/config"
)

// XiaohongshuPluginConfig implements config.Configurable for xiaohongshu plugin config.
type XiaohongshuPluginConfig struct {
	Enabled bool
}

func (c *XiaohongshuPluginConfig) ConfigNamespace() string { return "xiaohongshu" }

func (c *XiaohongshuPluginConfig) ConfigSchema() []config.ConfigItem {
	return []config.ConfigItem{
		{
			Key:         "enabled",
			Type:        config.ConfigTypeBool,
			Default:     false,
			Description: "是否记录小红书页面浏览记录",
			Title:       "记录小红书浏览",
			Group:       "Xiaohongshu",
		},
	}
}

func (c *XiaohongshuPluginConfig) ApplyConfig(sub *config.SubViper) error {
	c.Enabled = sub.GetBool("enabled")
	return nil
}

// GetXiaohongshuConfig returns the registered xiaohongshu plugin config if available.
func GetXiaohongshuConfig() *XiaohongshuPluginConfig {
	return xiaohongshuPluginConfig
}

var xiaohongshuPluginConfig *XiaohongshuPluginConfig

func init() {
	xiaohongshuPluginConfig = &XiaohongshuPluginConfig{}
	config.RegisterPlugin(xiaohongshuPluginConfig)
}
