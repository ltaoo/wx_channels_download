package wxmp

import (
	"fmt"
	"strconv"

	"wx_channel/pkg/configapi"
)

var ConfigDeclaration = configapi.Declare("debug", "pagespy", "api", "download", "mp")

type OfficialAccountConfig struct {
	RootDir                   string
	WorkDir                   string
	Enabled                   bool `json:"officialAccountEnabled"`
	DebugShowError            bool
	PagespyEnabled            bool
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
	InjectContentScript       string
}

func NewOfficialAccountConfig(provider configapi.Provider, runtime configapi.Runtime) (*OfficialAccountConfig, error) {
	var debug_config struct {
		ShowError bool `json:"error"`
	}
	if err := ConfigDeclaration.Decode(provider, "debug", &debug_config); err != nil {
		return nil, fmt.Errorf("debug config: %w", err)
	}
	var pagespy_config struct {
		Enabled bool `json:"enabled"`
	}
	if err := ConfigDeclaration.Decode(provider, "pagespy", &pagespy_config); err != nil {
		return nil, fmt.Errorf("pagespy config: %w", err)
	}
	var api_config struct {
		Protocol string `json:"protocol"`
		Hostname string `json:"hostname"`
		Port     int    `json:"port"`
	}
	if err := ConfigDeclaration.Decode(provider, "api", &api_config); err != nil {
		return nil, fmt.Errorf("api config: %w", err)
	}
	var download_config struct {
		RemoteServer struct {
			Enabled  bool   `json:"enabled"`
			Protocol string `json:"protocol"`
			Hostname string `json:"hostname"`
			Port     int    `json:"port"`
		} `json:"remoteServer"`
	}
	if err := ConfigDeclaration.Decode(provider, "download", &download_config); err != nil {
		return nil, fmt.Errorf("download config: %w", err)
	}
	var mp_config struct {
		Enabled                   bool     `json:"enabled"`
		RefreshToken              string   `json:"refreshToken"`
		TokenFilepath             string   `json:"tokenFilepath"`
		RefreshSkipMinutes        int      `json:"refreshSkipMinutes"`
		MaxWebsocketClients       int      `json:"maxWebsocketClients"`
		AccountIdsRefreshInterval []string `json:"accountIdsRefreshInterval"`
	}
	if err := ConfigDeclaration.Decode(provider, "mp", &mp_config); err != nil {
		return nil, fmt.Errorf("official account config: %w", err)
	}

	return &OfficialAccountConfig{
		RootDir:                   runtime.RootDir,
		Enabled:                   mp_config.Enabled,
		WorkDir:                   runtime.WorkDir,
		DebugShowError:            debug_config.ShowError,
		PagespyEnabled:            pagespy_config.Enabled,
		Protocol:                  api_config.Protocol,
		Hostname:                  api_config.Hostname,
		Port:                      api_config.Port,
		Addr:                      api_config.Hostname + ":" + strconv.Itoa(api_config.Port),
		RemoteServerEnabled:       download_config.RemoteServer.Enabled,
		RemoteServerProtocol:      download_config.RemoteServer.Protocol,
		RemoteServerHostname:      download_config.RemoteServer.Hostname,
		RemoteServerPort:          download_config.RemoteServer.Port,
		RefreshToken:              mp_config.RefreshToken,
		TokenFilepath:             mp_config.TokenFilepath,
		RefreshSkipMinutes:        mp_config.RefreshSkipMinutes,
		MaxWebsocketClients:       mp_config.MaxWebsocketClients,
		AccountIdsRefreshInterval: mp_config.AccountIdsRefreshInterval,
		InjectContentScript:       runtime.ContentScriptContent,
	}, nil
}
