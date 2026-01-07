package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/adrg/xdg"
	"github.com/spf13/viper"

	"wx_channel/internal/config"
)

type APIConfig struct {
	Original                     *config.Config
	RootDir                      string
	DownloadDir                  string
	MaxRunning                   int // 最多同时下载的任务数
	Addr                         string
	OfficialAccountRemote        bool // 是否为公众号远端服务模式
	OfficialAccountRefreshToken  string
	OfficialAccountTokenFilepath string
}

func NewAPIConfig(c *config.Config, mp_remote_mode bool) *APIConfig {
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
	mp_refresh_token := viper.GetString("mp.refreshToken")
	mp_token_filepath := viper.GetString("mp.tokenFilepath")
	api_cfg := &APIConfig{
		Original:                     c,
		RootDir:                      c.RootDir,
		DownloadDir:                  dir,
		MaxRunning:                   3,
		Addr:                         viper.GetString("api.hostname") + ":" + strconv.Itoa(viper.GetInt("api.port")),
		OfficialAccountRemote:        mp_remote_mode,
		OfficialAccountTokenFilepath: mp_token_filepath,
		OfficialAccountRefreshToken:  mp_refresh_token,
	}
	return api_cfg
}
