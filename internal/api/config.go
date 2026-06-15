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
	MaxRunning                   int // 最多同时下载的任务数
	Protocol                     string
	Hostname                     string
	Port                         int
	RemoteServerEnabled          bool
	RemoteServerProtocol         string
	RemoteServerHostname         string
	RemoteServerPort             int
	RemoteServerMode             bool // 是否为服务器模式
	OfficialAccountRefreshToken  string
	OfficialAccountTokenFilepath string
	ChannelsRefreshInterval      int
	CloudflareSphCookie          string
	Shuba69Cookie                string
	Shuba69Fetcher               string
	Shuba69CDPEndpoint           string
	Shuba69CDPTimeout            int
	Shuba69CDPWait               int
	Shuba69SandboxAPIBaseURL     string
	Shuba69SandboxID             string
	BrowserDockerImage           string
	BrowserDockerEntrypoint      string
	BrowserDockerNetwork         string
	BrowserCDPPortMin            int
	BrowserCDPPortMax            int
	BrowserDesktopPortMin        int
	BrowserDesktopPortMax        int
	BrowserDesktopResolution     string
	BrowserDockerShmSize         string
	BrowserDockerMemoryLimit     string
	BrowserDockerChromeCommand   string

	DBType         string
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	DBPath         string
	MigrationsPath string
}

func NewAPIConfig(c *config.Config, remote_mode bool) *APIConfig {
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
	mp_refresh_token := viper.GetString("mp.refreshToken")
	mp_token_filepath := viper.GetString("mp.tokenFilepath")
	cloudflare_sph_cookie := viper.GetString("cloudflare.sphCookie")
	shuba69_cookie := viper.GetString("69shuba.cookie")
	shuba69_fetcher := strings.ToLower(strings.TrimSpace(viper.GetString("69shuba.fetcher")))

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
		RemoteServerMode:             remote_mode,
		OfficialAccountTokenFilepath: mp_token_filepath,
		OfficialAccountRefreshToken:  mp_refresh_token,
		ChannelsRefreshInterval:      viper.GetInt("channels.refreshInterval"),
		CloudflareSphCookie:          cloudflare_sph_cookie,
		Shuba69Cookie:                shuba69_cookie,
		Shuba69Fetcher:               shuba69_fetcher,
		Shuba69CDPEndpoint:           viper.GetString("69shuba.cdpEndpoint"),
		Shuba69CDPTimeout:            viper.GetInt("69shuba.cdpTimeout"),
		Shuba69CDPWait:               viper.GetInt("69shuba.cdpWait"),
		Shuba69SandboxAPIBaseURL:     viper.GetString("69shuba.sandboxAPIBaseURL"),
		Shuba69SandboxID:             viper.GetString("69shuba.sandboxID"),
		BrowserDockerImage:           viper.GetString("sandbox.dockerImage"),
		BrowserDockerEntrypoint:      viper.GetString("sandbox.dockerEntrypoint"),
		BrowserDockerNetwork:         viper.GetString("sandbox.dockerNetwork"),
		BrowserCDPPortMin:            viper.GetInt("sandbox.cdpPortMin"),
		BrowserCDPPortMax:            viper.GetInt("sandbox.cdpPortMax"),
		BrowserDesktopPortMin:        viper.GetInt("sandbox.desktopPortMin"),
		BrowserDesktopPortMax:        viper.GetInt("sandbox.desktopPortMax"),
		BrowserDesktopResolution:     viper.GetString("sandbox.resolution"),
		BrowserDockerShmSize:         viper.GetString("sandbox.shmSize"),
		BrowserDockerMemoryLimit:     viper.GetString("sandbox.memoryLimit"),
		BrowserDockerChromeCommand:   viper.GetString("sandbox.chromeCommand"),

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
