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
			Description: "用于作品列表及视频详情 web API；作品列表需要 UIFID 或 UIFID_TEMP。留空时读取同步的 .douyin.com Cookie，填写时优先使用此配置；不会输出到日志",
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
