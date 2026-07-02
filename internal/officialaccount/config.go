package officialaccount

import (
	"strconv"

	"github.com/spf13/viper"

	"wx_channel/internal/config"
)

type OfficialAccountConfig struct {
	RootDir                   string
	Enabled                   bool `json:"officialAccountEnabled"`
	DebugShowError            bool
	PagespyEnabled            bool
	Protocol                  string
	Hostname                  string
	Port                      int
	Addr                      string
	RemoteMode                bool
	RemoteServerEnabled       bool   `json:"remoteServerEnabled"`
	RemoteServerProtocol      string `json:"remoteServerProtocol"`
	RemoteServerHostname      string `json:"remoteServerHostname"`
	RemoteServerPort          int    `json:"remoteServerPort"`
	RefreshToken              string `json:"officialServerRefreshToken"`
	TokenFilepath             string
	RefreshSkipMinutes        int
	MaxWebsocketClients       int
	AccountIdsRefreshInterval []string
}

func NewOfficialAccountConfig(c *config.Config, remote_mode bool) *OfficialAccountConfig {
	protocol := viper.GetString("api.protocol")
	hostname := viper.GetString("api.hostname")
	port := viper.GetInt("api.port")
	enabled := config.IsMPEnabled()
	cfg := &OfficialAccountConfig{
		RootDir:                   c.RootDir,
		Enabled:                   enabled,
		DebugShowError:            viper.GetBool("debug.error"),
		PagespyEnabled:            viper.GetBool("pagespy.enabled"),
		Protocol:                  protocol,
		Hostname:                  hostname,
		Port:                      port,
		Addr:                      hostname + ":" + strconv.Itoa(port),
		RemoteMode:                remote_mode,
		RemoteServerEnabled:       viper.GetBool("download.remoteServer.enabled"),
		RemoteServerProtocol:      viper.GetString("download.remoteServer.protocol"),
		RemoteServerHostname:      viper.GetString("download.remoteServer.hostname"),
		RemoteServerPort:          viper.GetInt("download.remoteServer.port"),
		RefreshToken:              viper.GetString("mp.refreshToken"),
		TokenFilepath:             viper.GetString("mp.tokenFilepath"),
		RefreshSkipMinutes:        viper.GetInt("mp.refreshSkipMinutes"),
		MaxWebsocketClients:       viper.GetInt("mp.maxWebsocketClients"),
		AccountIdsRefreshInterval: viper.GetStringSlice("mp.accountIdsRefreshInterval"),
	}
	return cfg
}
