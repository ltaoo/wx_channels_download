package wxchannels

import (
	"strconv"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"wx_channel/frontend"
	"wx_channel/internal/config"
)

type ChannelsConfig struct {
	Version                       string
	PagespyEnabled                bool
	DebugShowError                bool
	ChannelsDisableLocationToHome bool
	GlobalScriptPath              string
	InjectContentScript           string
	FrontendVariables             map[string]any
	Logger                        *zerolog.Logger
}

func NewChannelsConfig(c *config.Config, logger *zerolog.Logger) *ChannelsConfig {
	api_protocol := viper.GetString("api.protocol")
	api_hostname := viper.GetString("api.hostname")
	api_port := viper.GetInt("api.port")
	remote_server_protocol := viper.GetString("download.remoteServer.protocol")
	remote_server_hostname := viper.GetString("download.remoteServer.hostname")
	remote_server_port := viper.GetInt("download.remoteServer.port")
	max_running := viper.GetInt("download.maxRunning")
	if max_running == 0 {
		max_running = 3
	}
	settings := &ChannelsConfig{
		Version:                       c.Version,
		DebugShowError:                viper.GetBool("debug.error"),
		PagespyEnabled:                viper.GetBool("pagespy.enabled"),
		ChannelsDisableLocationToHome: viper.GetBool("channels.disableLocationToHome"),
		GlobalScriptPath:              c.GlobalScriptPath,
		InjectContentScript:           c.ContentScriptContent,
		FrontendVariables: map[string]any{
			"apiHost":                    api_hostname + ":" + strconv.Itoa(api_port),
			"apiOrigin":                  api_protocol + "://" + api_hostname + ":" + strconv.Itoa(api_port),
			"apiProtocol":                api_protocol,
			"pagespyServerProtocol":      viper.GetString("pagespy.protocol"),
			"pagespyServerAPI":           viper.GetString("pagespy.api"),
			"remoteServerEnabled":        viper.GetBool("download.remoteServer.enabled"),
			"remoteServerOrigin":         remote_server_protocol + "://" + remote_server_hostname + ":" + strconv.Itoa(remote_server_port),
			"maxRunning":                 max_running,
			"downloadFilenameTemplate":   viper.GetString("download.filenameTemplate"),
			"defaultHighest":             viper.GetBool("channels.download.defaultHighest") || viper.GetBool("download.defaultHighest"),
			"downloadPauseWhenDownload":  viper.GetBool("channels.download.pauseWhenDownload"),
			"downloadInFrontend":         viper.GetBool("channels.download.frontend"),
			"downloadForceCheckAllFeeds": viper.GetBool("channels.download.forceCheckAllFeeds"),
		},
		Logger: logger,
	}

	if settings.GlobalScriptPath == "" {
		logger.Info().
			Str("file", "pkg/scraper/wxchannels/config.go").
			Msg("wxchannels global script path is empty")
	} else {
		logger.Info().
			Str("file", "pkg/scraper/wxchannels/config.go").
			Str("path", settings.GlobalScriptPath).
			Str("asset_path", frontend.UserGlobalScriptAssetPath(settings.GlobalScriptPath)).
			Msg("wxchannels global script path configured")
	}

	return settings
}

func (c *ChannelsConfig) AddVariable(key string, value any) {
	if c.FrontendVariables == nil {
		c.FrontendVariables = make(map[string]any)
	}
	c.FrontendVariables[key] = value
}
