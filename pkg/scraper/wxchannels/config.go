package wxchannels

import (
	"fmt"
	"strconv"

	"wx_channel/pkg/configapi"
)

var ConfigDeclaration = configapi.Declare("debug", "pagespy", "channel", "channels", "download", "api", "mp")

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

func NewChannelsConfig(provider configapi.Provider, runtime configapi.Runtime) (*ChannelsConfig, error) {
	var debug_config struct {
		ShowError bool `json:"error"`
	}
	if err := ConfigDeclaration.Decode(provider, "debug", &debug_config); err != nil {
		return nil, fmt.Errorf("debug config: %w", err)
	}
	var pagespy_config struct {
		Enabled  bool   `json:"enabled"`
		Protocol string `json:"protocol"`
		API      string `json:"api"`
	}
	if err := ConfigDeclaration.Decode(provider, "pagespy", &pagespy_config); err != nil {
		return nil, fmt.Errorf("pagespy config: %w", err)
	}
	var legacy_channel_config struct {
		DisableLocationToHome bool `json:"disableLocationToHome"`
	}
	if err := ConfigDeclaration.Decode(provider, "channel", &legacy_channel_config); err != nil {
		return nil, fmt.Errorf("legacy channel config: %w", err)
	}
	var channels_config struct {
		DisableLocationToHome bool `json:"disableLocationToHome"`
		Download              struct {
			DefaultHighest     bool `json:"defaultHighest"`
			PauseWhenDownload  bool `json:"pauseWhenDownload"`
			Frontend           bool `json:"frontend"`
			ForceCheckAllFeeds bool `json:"forceCheckAllFeeds"`
		} `json:"download"`
	}
	if err := ConfigDeclaration.Decode(provider, "channels", &channels_config); err != nil {
		return nil, fmt.Errorf("channels config: %w", err)
	}
	var download_config struct {
		FilenameTemplate string `json:"filenameTemplate"`
		MaxRunning       int    `json:"maxRunning"`
		RemoteServer     struct {
			Enabled  bool   `json:"enabled"`
			Protocol string `json:"protocol"`
			Hostname string `json:"hostname"`
			Port     int    `json:"port"`
		} `json:"remoteServer"`
	}
	if err := ConfigDeclaration.Decode(provider, "download", &download_config); err != nil {
		return nil, fmt.Errorf("download config: %w", err)
	}
	var api_config struct {
		Protocol string `json:"protocol"`
		Hostname string `json:"hostname"`
		Port     int    `json:"port"`
	}
	if err := ConfigDeclaration.Decode(provider, "api", &api_config); err != nil {
		return nil, fmt.Errorf("api config: %w", err)
	}
	var mp_config struct {
		Enabled      bool   `json:"enabled"`
		RefreshToken string `json:"refreshToken"`
		RemoteServer struct {
			Protocol string `json:"protocol"`
			Hostname string `json:"hostname"`
			Port     int    `json:"port"`
		} `json:"remoteServer"`
	}
	if err := ConfigDeclaration.Decode(provider, "mp", &mp_config); err != nil {
		return nil, fmt.Errorf("official account config: %w", err)
	}

	settings := &ChannelsConfig{
		Version:                             runtime.Version,
		DebugShowError:                      debug_config.ShowError,
		PagespyEnabled:                      pagespy_config.Enabled,
		PageppyServerProtocol:               pagespy_config.Protocol,
		PageppyServerAPI:                    pagespy_config.API,
		ChannelsDisableLocationToHome:       legacy_channel_config.DisableLocationToHome || channels_config.DisableLocationToHome,
		DownloadDefaultHighest:              channels_config.Download.DefaultHighest,
		DownloadFilenameTemplate:            download_config.FilenameTemplate,
		DownloadPauseWhenDownload:           channels_config.Download.PauseWhenDownload,
		DownloadInFrontend:                  channels_config.Download.Frontend,
		DownloadMaxRunning:                  download_config.MaxRunning,
		DownloadForceCheckAllFeeds:          channels_config.Download.ForceCheckAllFeeds,
		APIServerProtocol:                   api_config.Protocol,
		APIServerHostname:                   api_config.Hostname,
		APIServerPort:                       api_config.Port,
		APIServerAddr:                       api_config.Hostname + ":" + strconv.Itoa(api_config.Port),
		RemoteServerEnabled:                 download_config.RemoteServer.Enabled,
		RemoteServerProtocol:                download_config.RemoteServer.Protocol,
		RemoteServerHostname:                download_config.RemoteServer.Hostname,
		RemoteServerPort:                    download_config.RemoteServer.Port,
		OfficialAccountServerDisabled:       !mp_config.Enabled,
		OfficialAccountServerRefreshToken:   mp_config.RefreshToken,
		OfficialAccountRemoteServerProtocol: mp_config.RemoteServer.Protocol,
		OfficialAccountRemoteServerHostname: mp_config.RemoteServer.Hostname,
		OfficialAccountRemoteServerPort:     mp_config.RemoteServer.Port,
		InjectGlobalScript:                  runtime.GlobalScriptContent,
		InjectContentScript:                 runtime.ContentScriptContent,
		FrontendVariables:                   make(map[string]any),
	}
	return settings, nil
}

func (c *ChannelsConfig) AddVariable(key string, value any) {
	if c.FrontendVariables == nil {
		c.FrontendVariables = make(map[string]any)
	}
	c.FrontendVariables[key] = value
}
