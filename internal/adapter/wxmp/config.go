package wxmpadapter

import (
	"net"
	"strconv"

	"wx_channel/frontend"
	"wx_channel/internal/config"
	"wx_channel/pkg/scraper/wxmp"
)

// MPPluginConfig implements config.Configurable for wxmp (official account) plugin config.
type MPPluginConfig struct {
	RemoteServerProtocol      string
	RemoteServerHostname      string
	RemoteServerPort          int
	RefreshToken              string
	TokenFilepath             string
	AccountIdsRefreshInterval []string
	RefreshSkipMinutes        int
}

func (c *MPPluginConfig) ConfigNamespace() string { return "mp" }

func (c *MPPluginConfig) ConfigSchema() []config.ConfigField {
	return []config.ConfigField{
		{
			Key:         "remoteServer.protocol",
			Type:        config.ConfigTypeString,
			Default:     "http",
			Description: "公众号远端服务协议头",
			Title:       "服务协议头",
			Group:       "OfficialAccount",
		},
		{
			Key:         "remoteServer.hostname",
			Type:        config.ConfigTypeString,
			Default:     "",
			Description: "公众号远端服务主机名",
			Title:       "服务主机名",
			Group:       "OfficialAccount",
		},
		{
			Key:         "remoteServer.port",
			Type:        config.ConfigTypeInt,
			Default:     80,
			Description: "公众号远端服务端口",
			Title:       "服务端口",
			Group:       "OfficialAccount",
		},
		{
			Key:         "refreshToken",
			Type:        config.ConfigTypeString,
			Default:     "",
			Description: "公众号远端服务刷新凭证",
			Title:       "刷新凭证",
			Group:       "OfficialAccount",
			HotReload:   true,
		},
		{
			Key:         "tokenFilepath",
			Type:        config.ConfigTypeString,
			Default:     "",
			Description: "公众号远端服务授权凭证",
			Title:       "授权凭证",
			Group:       "OfficialAccount",
			HotReload:   true,
		},
		{
			Key:         "accountIdsRefreshInterval",
			Type:        config.ConfigTypeText,
			Default:     []string{},
			Description: "需要定时刷新的帐号列表",
			Title:       "定时刷新列表",
			Group:       "OfficialAccount",
			HotReload:   true,
		},
		{
			Key:         "refreshSkipMinutes",
			Type:        config.ConfigTypeInt,
			Default:     20,
			Description: "刷新时若账号在最近 N 分钟已更新则跳过",
			Title:       "刷新跳过时间（分钟）",
			Group:       "OfficialAccount",
			HotReload:   true,
		},
	}
}

func (c *MPPluginConfig) ApplyConfig(sub *config.SubViper) error {
	c.RemoteServerProtocol = sub.GetString("remoteServer.protocol")
	c.RemoteServerHostname = sub.GetString("remoteServer.hostname")
	c.RemoteServerPort = sub.GetInt("remoteServer.port")
	c.RefreshToken = sub.GetString("refreshToken")
	c.TokenFilepath = sub.GetString("tokenFilepath")
	c.AccountIdsRefreshInterval = sub.GetStringSlice("accountIdsRefreshInterval")
	c.RefreshSkipMinutes = sub.GetInt("refreshSkipMinutes")
	return nil
}

func new_official_account_config(cfg *config.Config) *wxmp.OfficialAccountConfig {
	protocol := cfg.GetString("api.protocol")
	bind_hostname := cfg.GetString("api.hostname")
	hostname := config.APIClientHostname(bind_hostname)
	port := cfg.GetInt("api.port")
	settings := &wxmp.OfficialAccountConfig{
		RootDir:                   cfg.RootDir,
		Enabled:                   true,
		WorkDir:                   cfg.WorkDir,
		DebugShowError:            cfg.GetBool("debug.error"),
		Protocol:                  protocol,
		Hostname:                  hostname,
		Port:                      port,
		Addr:                      net.JoinHostPort(hostname, strconv.Itoa(port)),
		RemoteServerEnabled:       cfg.GetBool("download.remoteServer.enabled"),
		RemoteServerProtocol:      cfg.GetString("download.remoteServer.protocol"),
		RemoteServerHostname:      cfg.GetString("download.remoteServer.hostname"),
		RemoteServerPort:          cfg.GetInt("download.remoteServer.port"),
		RefreshToken:              cfg.GetString("mp.refreshToken"),
		TokenFilepath:             cfg.GetString("mp.tokenFilepath"),
		RefreshSkipMinutes:        cfg.GetInt("mp.refreshSkipMinutes"),
		MaxWebsocketClients:       cfg.GetInt("mp.maxWebsocketClients"),
		AccountIdsRefreshInterval: cfg.GetStringSlice("mp.accountIdsRefreshInterval"),
		GlobalScriptPath:          cfg.GlobalScriptPath,
		InjectContentScript:       cfg.ContentScriptContent,
	}
	if cfg.GlobalScriptPath != "" {
		settings.GlobalScriptURL = frontend.UserGlobalScriptAssetPath(cfg.GlobalScriptPath)
	}
	return settings
}

func new_interceptor_config(cfg *config.Config) wxmp.InterceptorConfig {
	return wxmp.InterceptorConfig{
		Version:  cfg.Version,
		Mode:     cfg.Mode,
		Settings: *new_official_account_config(cfg),
	}
}

func init() {
	config.RegisterPlugin(&MPPluginConfig{})
}
