package zhihuadapter

import "wx_channel/internal/config"

// ZhihuPluginConfig implements config.Configurable for zhihu.
type ZhihuPluginConfig struct{}

func (c *ZhihuPluginConfig) ConfigNamespace() string { return "zhihu" }

func (c *ZhihuPluginConfig) ConfigSchema() []config.ConfigField { return nil }

func (c *ZhihuPluginConfig) ApplyConfig(_ *config.SubViper) error { return nil }

func init() {
	config.RegisterPlugin(&ZhihuPluginConfig{})
}
