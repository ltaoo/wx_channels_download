package officialaccount

import (
	"strconv"

	"github.com/spf13/viper"

	"wx_channel/internal/config"
)

type OfficialAccountConfig struct {
	RootDir                   string
	Disabled                  bool // 是否禁用公众号服务
	DebugShowError            bool
	PagespyEnabled            bool
	Protocol                  string
	Hostname                  string
	Port                      int
	Addr                      string
	RemoteMode                bool
	RemoteServerProtocol      string
	RemoteServerHostname      string
	RemoteServerPort          int
	RefreshToken              string
	TokenFilepath             string
	RefreshSkipMinutes        int
	MaxWebsocketClients       int
	AccountIdsRefreshInterval []string
}

func NewOfficialAccountConfig(c *config.Config, remote_mode bool) *OfficialAccountConfig {
	protocol := viper.GetString("api.protocol")
	hostname := viper.GetString("api.hostname")
	port := viper.GetInt("api.port")
	cfg := &OfficialAccountConfig{
		RootDir:                   c.RootDir,
		DebugShowError:            viper.GetBool("debug.error"),
		PagespyEnabled:            viper.GetBool("pagespy.enabled"),
		Protocol:                  protocol,
		Hostname:                  hostname,
		Port:                      port,
		Addr:                      hostname + ":" + strconv.Itoa(port),
		RemoteMode:                remote_mode,
		RefreshToken:              viper.GetString("mp.refreshToken"),
		TokenFilepath:             viper.GetString("mp.tokenFilepath"),
		Disabled:                  viper.GetBool("mp.disabled"),
		RemoteServerProtocol:      viper.GetString("mp.remoteServer.protocol"),
		RemoteServerHostname:      viper.GetString("mp.remoteServer.hostname"),
		RemoteServerPort:          viper.GetInt("mp.remoteServer.port"),
		RefreshSkipMinutes:        viper.GetInt("mp.refreshSkipMinutes"),
		MaxWebsocketClients:       viper.GetInt("mp.maxWebsocketClients"),
		AccountIdsRefreshInterval: viper.GetStringSlice("mp.accountIdsRefreshInterval"),
	}
	return cfg
}
