package officialaccount

import (
	"strconv"

	"github.com/spf13/viper"

	"wx_channel/internal/config"
)

type OfficialAccountConfig struct {
	RootDir              string
	DisableServer        bool // 本地不启用服务
	Protocol             string
	Hostname             string
	Port                 int
	Addr                 string
	RemoteMode           bool
	RemoteServerProtocol string
	RemoteServerHostname string
	RemoteServerPort     int
	RefreshToken         string
	TokenFilepath        string
}

func NewOfficialAccountConfig(c *config.Config, remote_mode bool) *OfficialAccountConfig {
	protocol := viper.GetString("mp.protocol")
	hostname := viper.GetString("mp.hostname")
	port := viper.GetInt("mp.port")
	cfg := &OfficialAccountConfig{
		RootDir:              c.RootDir,
		Protocol:             protocol,
		Hostname:             hostname,
		Port:                 port,
		Addr:                 hostname + ":" + strconv.Itoa(port),
		RefreshToken:         viper.GetString("mp.refreshToken"),
		TokenFilepath:        viper.GetString("mp.tokenFilepath"),
		RemoteMode:           remote_mode,
		RemoteServerProtocol: viper.GetString("mp.remoteServer.protocol"),
		RemoteServerHostname: viper.GetString("mp.remoteServer.hostname"),
		RemoteServerPort:     viper.GetInt("mp.remoteServer.port"),
	}
	return cfg
}
