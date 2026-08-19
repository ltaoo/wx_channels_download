package api

import (
	"fmt"
	"os"

	"wx_channel/internal/config"
)

type APIConfig struct {
	Version                   string
	Mode                      string
	Original                  *config.Config
	RootDir                   string
	WorkDir                   string
	LogPath                   string
	DownloadDir               string
	PlayDoneAudio             bool
	MaxRunning                int // maximum number of concurrent download tasks
	ResourceConcurrency       int // maximum number of resources across running tasks
	SegmentConcurrency        int // maximum number of segments inside each resource
	ConnectionConcurrency     int // maximum number of protocol connections across running tasks
	Protocol                  string
	Hostname                  string
	Port                      int
	RemoteServerEnabled       bool   `json:"remoteServerEnabled"`
	RemoteServerProtocol      string `json:"remoteServerProtocol"`
	RemoteServerHostname      string `json:"remoteServerHostname"`
	RemoteServerPort          int    `json:"remoteServerPort"`
	CloudflareSphCookie       string
	CookieUUID                string
	CookiePassword            string
	CookieKey                 string
	FilenameTemplate          string
	DefaultActionWhenExisting string

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
	resource_concurrency := c.GetInt("download.resourceConcurrency")
	if resource_concurrency <= 0 {
		resource_concurrency = 5
	}
	segment_concurrency := c.GetInt("download.segmentConcurrency")
	if segment_concurrency <= 0 {
		segment_concurrency = 5
	}
	connection_concurrency := c.GetInt("download.connectionConcurrency")

	api_cfg := &APIConfig{
		Version:               c.Version,
		Mode:                  c.Mode,
		Original:              c,
		RootDir:               c.RootDir,
		WorkDir:               c.WorkDir,
		LogPath:               c.LogPath(),
		DownloadDir:           dir,
		PlayDoneAudio:         c.GetBool("download.playDoneAudio"),
		MaxRunning:            3,
		ResourceConcurrency:   resource_concurrency,
		SegmentConcurrency:    segment_concurrency,
		ConnectionConcurrency: connection_concurrency,
		Protocol:              c.GetString("api.protocol"),
		Hostname:              c.GetString("api.hostname"),
		Port:                  c.GetInt("api.port"),
		RemoteServerEnabled:   c.GetBool("download.remoteServer.enabled"),
		RemoteServerProtocol:  c.GetString("download.remoteServer.protocol"),
		RemoteServerHostname:  c.GetString("download.remoteServer.hostname"),
		RemoteServerPort:      c.GetInt("download.remoteServer.port"),
		CloudflareSphCookie:   cloudflare_sph_cookie,
		CookieUUID:            c.GetString("cookie.uuid"),
		CookiePassword:        c.GetString("cookie.password"),
		CookieKey:             c.GetString("cookie.key"),

		FilenameTemplate:          c.GetString("download.filenameTemplate"),
		DefaultActionWhenExisting: c.GetString("download.defaultActionWhenExisting"),
		HooksScript:               c.HookScriptPath,

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
