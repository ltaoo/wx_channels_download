package wxmpadapter

import (
	"wx_channel/internal/config"
	"wx_channel/pkg/configapi"
)

// MPPluginConfig implements config.Configurable for wxmp (official account) plugin config.
type MPPluginConfig struct {
	Enabled                   bool
	RemoteServerProtocol      string
	RemoteServerHostname      string
	RemoteServerPort          int
	RefreshToken              string
	TokenFilepath             string
	AccountIdsRefreshInterval []string
	RefreshSkipMinutes        int
}

func (c *MPPluginConfig) ConfigNamespace() string { return "mp" }

func (c *MPPluginConfig) ConfigSchema() []configapi.Item {
	return []configapi.Item{
		{
			Key:         "enabled",
			Type:        configapi.TypeBool,
			Default:     false,
			Description: "是否启用公众号本地服务，本地服务会提供接口、RSS 等功能",
			Title:       "启用本地服务",
			Group:       "OfficialAccount",
			Deprecated:  true,
		},
		{
			Key:         "remoteServer.protocol",
			Type:        configapi.TypeString,
			Default:     "http",
			Description: "公众号远端服务协议头",
			Title:       "服务协议头",
			Group:       "OfficialAccount",
		},
		{
			Key:         "remoteServer.hostname",
			Type:        configapi.TypeString,
			Default:     "",
			Description: "公众号远端服务主机名",
			Title:       "服务主机名",
			Group:       "OfficialAccount",
		},
		{
			Key:         "remoteServer.port",
			Type:        configapi.TypeInt,
			Default:     80,
			Description: "公众号远端服务端口",
			Title:       "服务端口",
			Group:       "OfficialAccount",
		},
		{
			Key:         "refreshToken",
			Type:        configapi.TypeString,
			Default:     "",
			Description: "公众号远端服务刷新凭证",
			Title:       "刷新凭证",
			Group:       "OfficialAccount",
			Reload:      configapi.ReloadHot,
		},
		{
			Key:         "tokenFilepath",
			Type:        configapi.TypeString,
			Default:     "",
			Description: "公众号远端服务授权凭证",
			Title:       "授权凭证",
			Group:       "OfficialAccount",
			Reload:      configapi.ReloadHot,
		},
		{
			Key:         "accountIdsRefreshInterval",
			Type:        configapi.TypeStringSlice,
			Default:     []string{},
			Description: "需要定时刷新的帐号列表",
			Title:       "定时刷新列表",
			Group:       "OfficialAccount",
			Reload:      configapi.ReloadHot,
		},
		{
			Key:         "refreshSkipMinutes",
			Type:        configapi.TypeInt,
			Default:     20,
			Description: "刷新时若账号在最近 N 分钟已更新则跳过",
			Title:       "刷新跳过时间（分钟）",
			Group:       "OfficialAccount",
			Reload:      configapi.ReloadHot,
		},
	}
}

func (c *MPPluginConfig) ApplyConfig(sub *config.ScopedConfig) error {
	c.Enabled = sub.GetBool("enabled")
	c.RemoteServerProtocol = sub.GetString("remoteServer.protocol")
	c.RemoteServerHostname = sub.GetString("remoteServer.hostname")
	c.RemoteServerPort = sub.GetInt("remoteServer.port")
	c.RefreshToken = sub.GetString("refreshToken")
	c.TokenFilepath = sub.GetString("tokenFilepath")
	c.AccountIdsRefreshInterval = sub.GetStringSlice("accountIdsRefreshInterval")
	c.RefreshSkipMinutes = sub.GetInt("refreshSkipMinutes")
	return nil
}

// GetMPConfig returns the registered mp plugin config if available.
func GetMPConfig() *MPPluginConfig {
	return mpPluginConfig
}

var mpPluginConfig *MPPluginConfig

func init() {
	mpPluginConfig = &MPPluginConfig{}
	config.RegisterPlugin(mpPluginConfig)
}
