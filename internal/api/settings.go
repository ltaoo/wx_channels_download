package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/adrg/xdg"
	"github.com/spf13/viper"

	"wx_channel/config"
)

type APISettings struct {
	RootDir     string
	DownloadDir string
	MaxRunning  int // 最多同时下载的任务数
	Addr        string
}

func RegisterSettings(cfg *config.Config) {
	config.Register(config.ConfigItem{
		Key:         "download.dir",
		Type:        config.ConfigTypeString,
		Default:     "%UserDownloads%",
		Description: "下载目录",
	})
}
func NewAPISettings(c *config.Config) *APISettings {
	dir := viper.GetString("download.dir")
	dir = strings.ReplaceAll(dir, "%UserDownloads%", xdg.UserDirs.Download)
	dir = strings.ReplaceAll(dir, "%CWD%", c.RootDir)
	dir = filepath.Clean(dir)

	if !filepath.IsAbs(dir) {
		dir = filepath.Join(c.RootDir, dir)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Warning: Failed to create download directory: %s, error: %v\n", dir, err)
	}

	api_settings := &APISettings{
		RootDir:     c.RootDir,
		DownloadDir: dir,
		MaxRunning:  3,
		Addr:        viper.GetString("api.hostname") + ":" + strconv.Itoa(viper.GetInt("api.port")),
	}
	return api_settings
}
