package xiaohongshuadapter

import "wx_channel/internal/config"

// XiaohongshuPluginConfig implements config.Configurable for xiaohongshu.
type XiaohongshuPluginConfig struct{}

func (c *XiaohongshuPluginConfig) ConfigNamespace() string { return "xiaohongshu" }

func (c *XiaohongshuPluginConfig) ConfigSchema() []config.ConfigField { return nil }

func (c *XiaohongshuPluginConfig) ApplyConfig(_ *config.SubViper) error { return nil }

// GetXiaohongshuConfig returns the registered xiaohongshu plugin config if available.
func GetXiaohongshuConfig() *XiaohongshuPluginConfig {
	return xiaohongshu_plugin_config
}

var xiaohongshu_plugin_config *XiaohongshuPluginConfig

func init() {
	xiaohongshu_plugin_config = &XiaohongshuPluginConfig{}
	config.RegisterPlugin(xiaohongshu_plugin_config)
}
