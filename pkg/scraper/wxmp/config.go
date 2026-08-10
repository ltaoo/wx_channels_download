package wxmp

import (
	"strconv"

	"github.com/spf13/viper"

	"wx_channel/frontend"
	"wx_channel/internal/config"
)

type OfficialAccountConfig struct {
	RootDir                   string
	WorkDir                   string
	Enabled                   bool `json:"officialAccountEnabled"`
	DebugShowError            bool
	Protocol                  string
	Hostname                  string
	Port                      int
	Addr                      string
	RemoteServerEnabled       bool   `json:"remoteServerEnabled"`
	RemoteServerProtocol      string `json:"remoteServerProtocol"`
	RemoteServerHostname      string `json:"remoteServerHostname"`
	RemoteServerPort          int    `json:"remoteServerPort"`
	RefreshToken              string `json:"officialServerRefreshToken"`
	TokenFilepath             string
	RefreshSkipMinutes        int
	MaxWebsocketClients       int
	AccountIdsRefreshInterval []string
	GlobalScriptPath          string `json:"-"`
	GlobalScriptURL           string `json:"-"`
	InjectContentScript       string
}

func NewOfficialAccountConfig(c *config.Config) *OfficialAccountConfig {
	protocol := viper.GetString("api.protocol")
	hostname := viper.GetString("api.hostname")
	port := viper.GetInt("api.port")
	enabled := config.IsMPEnabled()
	cfg := &OfficialAccountConfig{
		RootDir:                   c.RootDir,
		Enabled:                   enabled,
		WorkDir:                   c.WorkDir,
		DebugShowError:            viper.GetBool("debug.error"),
		Protocol:                  protocol,
		Hostname:                  hostname,
		Port:                      port,
		Addr:                      hostname + ":" + strconv.Itoa(port),
		RemoteServerEnabled:       viper.GetBool("download.remoteServer.enabled"),
		RemoteServerProtocol:      viper.GetString("download.remoteServer.protocol"),
		RemoteServerHostname:      viper.GetString("download.remoteServer.hostname"),
		RemoteServerPort:          viper.GetInt("download.remoteServer.port"),
		RefreshToken:              viper.GetString("mp.refreshToken"),
		TokenFilepath:             viper.GetString("mp.tokenFilepath"),
		RefreshSkipMinutes:        viper.GetInt("mp.refreshSkipMinutes"),
		MaxWebsocketClients:       viper.GetInt("mp.maxWebsocketClients"),
		AccountIdsRefreshInterval: viper.GetStringSlice("mp.accountIdsRefreshInterval"),
		GlobalScriptPath:          c.GlobalScriptPath,
		InjectContentScript:       c.ContentScriptContent,
	}
	if c.GlobalScriptPath != "" {
		cfg.GlobalScriptURL = frontend.UserGlobalScriptAssetPath(c.GlobalScriptPath)
	}
	return cfg
}
