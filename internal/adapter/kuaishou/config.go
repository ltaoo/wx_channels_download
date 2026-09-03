package kuaishouadapter

import "wx_channel/internal/config"

// KuaishouPluginConfig exposes optional request credentials for deployments
// where Kuaishou applies stricter visitor verification.
type KuaishouPluginConfig struct {
	Cookie string
}

func (c *KuaishouPluginConfig) ConfigNamespace() string { return PlatformID }

func (c *KuaishouPluginConfig) ConfigSchema() []config.ConfigField {
	return []config.ConfigField{
		{
			Key:         "cookie",
			Type:        config.ConfigTypeText,
			Default:     "",
			Description: "可选的快手网页 Cookie；留空时使用应用已同步的 kuaishou.com Cookie",
			Title:       "快手 Cookie",
			Group:       "Kuaishou",
			Sensitive:   true,
			HotReload:   true,
		},
	}
}

func (c *KuaishouPluginConfig) ApplyConfig(sub *config.SubViper) error {
	c.Cookie = sub.GetString("cookie")
	return nil
}

func init() {
	config.RegisterPlugin(&KuaishouPluginConfig{})
}
