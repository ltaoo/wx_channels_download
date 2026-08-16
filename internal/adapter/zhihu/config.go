package zhihuadapter

import (
	"wx_channel/internal/config"
)

// ZhihuPluginConfig implements config.Configurable for zhihu plugin config.
type ZhihuPluginConfig struct {
	Enabled bool
}

func (c *ZhihuPluginConfig) ConfigNamespace() string { return "zhihu" }

func (c *ZhihuPluginConfig) ConfigSchema() []config.ConfigField {
	return []config.ConfigField{
		{
			Key:         "enabled",
			Type:        config.ConfigTypeBool,
			Default:     false,
			Description: "是否记录知乎页面浏览记录",
			Title:       "记录知乎浏览",
			Group:       "Zhihu",
		},
	}
}

func (c *ZhihuPluginConfig) ApplyConfig(sub *config.SubViper) error {
	c.Enabled = sub.GetBool("enabled")
	return nil
}

func init() {
	config.RegisterPlugin(&ZhihuPluginConfig{})
}
