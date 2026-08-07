package api

import (
	"fmt"
	"os"

	"wx_channel/internal/config"
)

type APIConfig struct {
	Version              string
	Mode                 string
	Original             *config.Config
	RootDir              string
	WorkDir              string
	DownloadDir          string
	PlayDoneAudio        bool
	MaxRunning           int // maximum number of concurrent download tasks
	Protocol             string
	Hostname             string
	Port                 int
	RemoteServerEnabled  bool   `json:"remoteServerEnabled"`
	RemoteServerProtocol string `json:"remoteServerProtocol"`
	RemoteServerHostname string `json:"remoteServerHostname"`
	RemoteServerPort     int    `json:"remoteServerPort"`
	CloudflareSphCookie  string
	FilenameTemplate     string

	HooksScript string

	DBType     string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBPath     string
}

func NewAPIConfig(c *config.Config) *APIConfig {
	dir := c.GetString("download.dir")
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Warning: Failed to create download directory: %s, error: %v\n", dir, err)
	}
	cloudflare_sph_cookie := c.GetString("cloudflare.sphCookie")

	api_cfg := &APIConfig{
		Version:              c.Version,
		Mode:                 c.Mode,
		Original:             c,
		RootDir:              c.RootDir,
		WorkDir:              c.WorkDir,
		DownloadDir:          dir,
		PlayDoneAudio:        c.GetBool("download.playDoneAudio"),
		MaxRunning:           3,
		Protocol:             c.GetString("api.protocol"),
		Hostname:             c.GetString("api.hostname"),
		Port:                 c.GetInt("api.port"),
		RemoteServerEnabled:  c.GetBool("download.remoteServer.enabled"),
		RemoteServerProtocol: c.GetString("download.remoteServer.protocol"),
		RemoteServerHostname: c.GetString("download.remoteServer.hostname"),
		RemoteServerPort:     c.GetInt("download.remoteServer.port"),
		CloudflareSphCookie:  cloudflare_sph_cookie,

		FilenameTemplate: c.GetString("download.filenameTemplate"),
		HooksScript:      c.HookScriptPath,

		DBType:     c.GetString("db.type"),
		DBHost:     c.GetString("db.host"),
		DBPort:     c.GetString("db.port"),
		DBUser:     c.GetString("db.username"),
		DBPassword: c.GetString("db.password"),
		DBName:     c.GetString("db.filename"),
		DBPath:     c.GetString("db.filepath"),
	}
	return api_cfg
}
