package api

import (
	"github.com/adrg/xdg"

	"wx_channel/config"
)

type APISettings struct {
	RootDir     string
	DownloadDir string
	MaxRunning  int // 最多同时下载的任务数
	Addr        string
}

func SetDefaultSettings(cfg *config.Config) {
	config.Register(config.ConfigItem{
		Key:         "api.defaultDownloadDir",
		Type:        config.ConfigTypeString,
		Default:     xdg.UserDirs.Download,
		Description: "默认下载目录",
	})
}
func NewAPISettings(c *config.Config) *APISettings {
	api_settings := &APISettings{
		RootDir:     c.RootDir,
		DownloadDir: c.GetString("download.defaultDownloadDir"),
		MaxRunning:  c.GetInt("download.maxRunning"),
		Addr:        c.GetString("api.addr"),
	}
	return api_settings
}
