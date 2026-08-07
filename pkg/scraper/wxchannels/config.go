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
	api_hostname := viper.GetString("api.hostname")
	api_port := viper.GetInt("api.port")
	settings := &ChannelsConfig{
		Version:                       c.Version,
		DebugShowError:                viper.GetBool("debug.error"),
		PagespyEnabled:                viper.GetBool("pagespy.enabled"),
		ChannelsDisableLocationToHome: viper.GetBool("channel.disableLocationToHome"),
		GlobalScriptPath:              c.GlobalScriptPath,
		InjectContentScript:           c.ContentScriptContent,
		FrontendVariables: map[string]any{
			"defaultHighest":             viper.GetBool("channels.download.defaultHighest"),
			"downloadFilenameTemplate":   viper.GetString("download.filenameTemplate"),
			"downloadPauseWhenDownload":  viper.GetBool("channels.download.pauseWhenDownload"),
			"downloadInFrontend":         viper.GetBool("channels.download.frontend"),
			"downloadForceCheckAllFeeds": viper.GetBool("channels.download.forceCheckAllFeeds"),
			"apiServerProtocol":          viper.GetString("api.protocol"),
			"apiServerAddr":              api_hostname + ":" + strconv.Itoa(api_port),
			"remoteServerEnabled":        viper.GetBool("download.remoteServer.enabled"),
			"MaxRunning":                 viper.GetInt("download.maxRunning"),
			"pagespyServerProtocol":      viper.GetString("pagespy.protocol"),
			"pagespyServerAPI":           viper.GetString("pagespy.api"),
		},
		Logger: logger,
	}
	if viper.GetBool("channels.disableLocationToHome") {
		settings.ChannelsDisableLocationToHome = true
	}

	if logger != nil {
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
	}

	return settings
}

func (c *ChannelsConfig) AddVariable(key string, value any) {
	if c.FrontendVariables == nil {
		c.FrontendVariables = make(map[string]any)
	}
	c.FrontendVariables[key] = value
}
