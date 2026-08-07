package wxchannels

import (
	"strconv"

	"github.com/spf13/viper"

	"wx_channel/internal/config"
)

type ChannelsConfig struct {
	Version                             string         `json:"version"`
	DownloadDefaultHighest              bool           `json:"defaultHighest"`
	DownloadFilenameTemplate            string         `json:"downloadFilenameTemplate"`
	DownloadPauseWhenDownload           bool           `json:"downloadPauseWhenDownload"`
	DownloadInFrontend                  bool           `json:"downloadInFrontend"`
	DownloadMaxRunning                  int            `json:"downloadMaxRunning"`
	DownloadForceCheckAllFeeds          bool           `json:"downloadForceCheckAllFeeds"`
	APIServerProtocol                   string         `json:"apiServerProtocol"`
	APIServerHostname                   string         `json:"apiServerHostname"`
	APIServerPort                       int            `json:"apiServerPort"`
	APIServerAddr                       string         `json:"apiServerAddr"`
	RemoteServerEnabled                 bool           `json:"remoteServerEnabled"`
	RemoteServerProtocol                string         `json:"remoteServerProtocol"`
	RemoteServerHostname                string         `json:"remoteServerHostname"`
	RemoteServerPort                    int            `json:"remoteServerPort"`
	OfficialAccountServerRefreshToken   string         `json:"officialServerRefreshToken"`
	OfficialAccountServerDisabled       bool           `json:"officialServerDisabled"`
	OfficialAccountRemoteServerProtocol string         `json:"officialRemoteServerProtocol"`
	OfficialAccountRemoteServerHostname string         `json:"officialRemoteServerHostname"`
	OfficialAccountRemoteServerPort     int            `json:"officialRemoteServerPort"`
	PagespyEnabled                      bool           `json:"pagespyEnabled"`
	PageppyServerProtocol               string         `json:"pagespyServerProtocol"`
	PageppyServerAPI                    string         `json:"pagespyServerAPI"`
	DebugShowError                      bool           `json:"debugShowError"`
	ChannelsDisableLocationToHome       bool           `json:"channelsDisableLocationToHome"`
	InjectGlobalScript                  string         `json:"-"`
	InjectContentScript                 string         `json:"-"`
	FrontendVariables                   map[string]any `json:"-"`
}

func NewChannelsConfig(c *config.Config) *ChannelsConfig {
	settings := &ChannelsConfig{
		Version:                             c.Version,
		DebugShowError:                      viper.GetBool("debug.error"),
		PagespyEnabled:                      viper.GetBool("pagespy.enabled"),
		PageppyServerProtocol:               viper.GetString("pagespy.protocol"),
		PageppyServerAPI:                    viper.GetString("pagespy.api"),
		ChannelsDisableLocationToHome:       viper.GetBool("channel.disableLocationToHome"),
		DownloadDefaultHighest:              viper.GetBool("channels.download.defaultHighest"),
		DownloadFilenameTemplate:            viper.GetString("download.filenameTemplate"),
		DownloadPauseWhenDownload:           viper.GetBool("channels.download.pauseWhenDownload"),
		DownloadInFrontend:                  viper.GetBool("channels.download.frontend"),
		DownloadMaxRunning:                  viper.GetInt("download.maxRunning"),
		DownloadForceCheckAllFeeds:          viper.GetBool("channels.download.forceCheckAllFeeds"),
		APIServerProtocol:                   viper.GetString("api.protocol"),
		APIServerHostname:                   viper.GetString("api.hostname"),
		APIServerPort:                       viper.GetInt("api.port"),
		APIServerAddr:                       viper.GetString("api.hostname") + ":" + strconv.Itoa(viper.GetInt("api.port")),
		RemoteServerEnabled:                 viper.GetBool("download.remoteServer.enabled"),
		RemoteServerProtocol:                viper.GetString("download.remoteServer.protocol"),
		RemoteServerHostname:                viper.GetString("download.remoteServer.hostname"),
		RemoteServerPort:                    viper.GetInt("download.remoteServer.port"),
		OfficialAccountServerDisabled:       !viper.GetBool("mp.enabled"),
		OfficialAccountServerRefreshToken:   viper.GetString("mp.refreshToken"),
		OfficialAccountRemoteServerProtocol: viper.GetString("mp.remoteServer.protocol"),
		OfficialAccountRemoteServerHostname: viper.GetString("mp.remoteServer.hostname"),
		OfficialAccountRemoteServerPort:     viper.GetInt("mp.remoteServer.port"),
		FrontendVariables:                   make(map[string]any),
	}
	if viper.GetBool("channels.disableLocationToHome") {
		settings.ChannelsDisableLocationToHome = true
	}

	settings.InjectGlobalScript = c.GlobalScriptContent
	settings.InjectContentScript = c.ContentScriptContent
	return settings
}

func (c *ChannelsConfig) AddVariable(key string, value any) {
	if c.FrontendVariables == nil {
		c.FrontendVariables = make(map[string]any)
	}
	c.FrontendVariables[key] = value
}
