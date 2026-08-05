package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
	"github.com/spf13/viper"

	"wx_channel/internal/config"
)

type APIConfig struct {
	Version                      string
	Mode                         string
	Original                     *config.Config
	RootDir                      string
	WorkDir                      string
	DownloadDir                  string
	PlayDoneAudio                bool
	MaxRunning                   int // maximum number of concurrent download tasks
	Protocol                     string
	Hostname                     string
	Port                         int
	RemoteServerEnabled          bool   `json:"remoteServerEnabled"`
	RemoteServerProtocol         string `json:"remoteServerProtocol"`
	RemoteServerHostname         string `json:"remoteServerHostname"`
	RemoteServerPort             int    `json:"remoteServerPort"`
	CloudflareSphCookie          string
	FilenameTemplate string

	HooksScript string

	DBType         string
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	DBPath         string
	MigrationsPath string
}

func NewAPIConfig(c *config.Config) *APIConfig {
	dir := viper.GetString("download.dir")
	dir = strings.ReplaceAll(dir, "%UserDownloads%", xdg.UserDirs.Download)
	dir = strings.ReplaceAll(dir, "%CWD%", c.WorkDir)
	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(c.WorkDir, dir)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Warning: Failed to create download directory: %s, error: %v\n", dir, err)
	}
	cloudflare_sph_cookie := viper.GetString("cloudflare.sphCookie")

	dbPath := viper.GetString("db.filepath")
	dbPath = strings.ReplaceAll(dbPath, "%CWD%", c.WorkDir)
	dbPath = filepath.Clean(dbPath)
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(c.WorkDir, dbPath)
	}

	migPath := viper.GetString("db.migration")
	migPath = strings.ReplaceAll(migPath, "%CWD%", c.WorkDir)
	migPath = filepath.Clean(migPath)
	if !filepath.IsAbs(migPath) {
		migPath = filepath.Join(c.WorkDir, migPath)
	}

	api_cfg := &APIConfig{
		Version:                      c.Version,
		Mode:                         c.Mode,
		Original:                     c,
		RootDir:                      c.RootDir,
		WorkDir:                      c.WorkDir,
		DownloadDir:                  dir,
		PlayDoneAudio:                viper.GetBool("download.playDoneAudio"),
		MaxRunning:                   3,
		Protocol:                     viper.GetString("api.protocol"),
		Hostname:                     viper.GetString("api.hostname"),
		Port:                         viper.GetInt("api.port"),
		RemoteServerEnabled:          viper.GetBool("download.remoteServer.enabled"),
		RemoteServerProtocol:         viper.GetString("download.remoteServer.protocol"),
		RemoteServerHostname:         viper.GetString("download.remoteServer.hostname"),
		RemoteServerPort:             viper.GetInt("download.remoteServer.port"),
		CloudflareSphCookie:          cloudflare_sph_cookie,

		FilenameTemplate: viper.GetString("download.filenameTemplate"),
		HooksScript:      viper.GetString("download.hooksScript"),

		DBType:         viper.GetString("db.type"),
		DBHost:         viper.GetString("db.host"),
		DBPort:         viper.GetString("db.port"),
		DBUser:         viper.GetString("db.username"),
		DBPassword:     viper.GetString("db.password"),
		DBName:         viper.GetString("db.filename"),
		DBPath:         dbPath,
		MigrationsPath: migPath,
	}
	return api_cfg
}
