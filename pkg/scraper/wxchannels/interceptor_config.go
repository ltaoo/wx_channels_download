package wxchannels

import (
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/spf13/viper"

	"wx_channel/internal/config"
)

type InterceptorConfig struct {
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
	InjectExtraScriptAfterJSMain        string         `json:"-"`
	InjectGlobalScriptFilepath          string         `json:"-"`
	InjectGlobalScript                  string         `json:"-"`
	FrontendVariables                   map[string]any `json:"-"`
}

func NewInterceptorSettings(c *config.Config) *InterceptorConfig {
	settings := &InterceptorConfig{
		Version:                             c.Version,
		DebugShowError:                      viper.GetBool("debug.error"),
		PagespyEnabled:                      viper.GetBool("pagespy.enabled"),
		PageppyServerProtocol:               viper.GetString("pagespy.protocol"),
		PageppyServerAPI:                    viper.GetString("pagespy.api"),
		ChannelsDisableLocationToHome:       viper.GetBool("channel.disableLocationToHome"),
		DownloadDefaultHighest:              viper.GetBool("download.defaultHighest"),
		DownloadFilenameTemplate:            viper.GetString("download.filenameTemplate"),
		DownloadPauseWhenDownload:           viper.GetBool("download.pauseWhenDownload"),
		DownloadInFrontend:                  viper.GetBool("download.frontend"),
		DownloadMaxRunning:                  viper.GetInt("download.maxRunning"),
		DownloadForceCheckAllFeeds:          viper.GetBool("download.forceCheckAllFeeds"),
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
		InjectExtraScriptAfterJSMain:        viper.GetString("inject.extraScript.afterJSMain"),
		InjectGlobalScriptFilepath:          viper.GetString("inject.globalScript"),
		FrontendVariables:                   make(map[string]any),
	}
	if viper.GetBool("channels.disableLocationToHome") {
		settings.ChannelsDisableLocationToHome = true
	}

	globalScriptPath := path.Join(c.RootDir, "global.js")
	if _, err := os.Stat(globalScriptPath); err == nil {
		scriptByte, err := os.ReadFile(globalScriptPath)
		if err == nil {
			settings.InjectGlobalScriptFilepath = globalScriptPath
			settings.InjectGlobalScript = string(scriptByte)
		}
	}

	extraJSFilepath := settings.InjectExtraScriptAfterJSMain
	if extraJSFilepath != "" {
		if !filepath.IsAbs(extraJSFilepath) {
			extraJSFilepath = filepath.Join(c.RootDir, extraJSFilepath)
		}
		if _, err := os.Stat(extraJSFilepath); err == nil {
			scriptByte, err := os.ReadFile(extraJSFilepath)
			if err == nil {
				settings.InjectExtraScriptAfterJSMain = string(scriptByte)
			}
		}
	}
	return settings
}

func (c *InterceptorConfig) AddVariable(key string, value any) {
	if c.FrontendVariables == nil {
		c.FrontendVariables = make(map[string]any)
	}
	c.FrontendVariables[key] = value
}
