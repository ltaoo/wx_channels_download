package api

import (
	"wx_channel/config"

	"github.com/adrg/xdg"
)

type APISettings struct {
	RootDir     string
	DownloadDir string
	MaxRunning  int // 最多同时下载的任务数
	Addr        string
}

func SetDefaultSettings(cfg *config.Config) {
	config.Register(config.ConfigItem{
		Key:         "download.defaultDownloadDir",
		Type:        config.ConfigTypeString,
		Default:     xdg.UserDirs.Download,
		Description: "下载目录",
	})
	config.Register(config.ConfigItem{
		Key:         "download.maxRunning",
		Type:        config.ConfigTypeInt,
		Default:     4,
		Description: "最大同时下载任务数",
	})
	config.Register(config.ConfigItem{
		Key:         "api.addr",
		Type:        config.ConfigTypeString,
		Default:     "127.0.0.1:2022",
		Description: "服务地址",
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
