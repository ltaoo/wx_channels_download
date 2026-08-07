package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"

	"wx_channel/pkg/configapi"
)

// ConfigDeclaration explicitly lists every runtime configuration namespace
// consumed by the API module.
var ConfigDeclaration = configapi.Declare("api", "download", "cloudflare", "proxy", "cert")

var api_config_declaration = configapi.DeclareModule(
	"api",
	configapi.Item{
		Key:         "api.protocol",
		Type:        configapi.TypeString,
		Default:     "http",
		Description: "指定 API 服务的协议头",
		Title:       "API 服务协议",
		Group:       "API",
		Readonly:    true,
		Reload:      configapi.ReloadProcess,
	},
	configapi.Item{
		Key:         "api.hostname",
		Type:        configapi.TypeString,
		Default:     "127.0.0.1",
		Description: "指定 API 服务的主机名",
		Title:       "API 服务主机",
		Group:       "API",
		Reload:      configapi.ReloadProcess,
	},
	configapi.Item{
		Key:         "api.port",
		Type:        configapi.TypeInt,
		Default:     2022,
		Description: "指定 API 服务的端口",
		Title:       "API 服务端口",
		Group:       "API",
		Reload:      configapi.ReloadProcess,
	},
	configapi.Item{
		Key:         "filehelper.enabled",
		Type:        configapi.TypeBool,
		Default:     true,
		Description: "是否开启文件传输助手自动下载视频号功能",
		Title:       "自动下载",
		Group:       "FileHelper",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "filehelper.callbackUrl",
		Type:        configapi.TypeString,
		Default:     "",
		Description: "文件传输助手消息回调地址",
		Title:       "回调地址",
		Group:       "FileHelper",
		Reload:      configapi.ReloadHot,
	},
	configapi.Item{
		Key:         "filehelper.syncInterval",
		Type:        configapi.TypeInt,
		Default:     5,
		Description: "消息同步间隔（秒）",
		Title:       "同步间隔",
		Group:       "FileHelper",
		Reload:      configapi.ReloadHot,
	},
)

type APIConfigSource struct {
	Provider configapi.Provider
	Runtime  configapi.Runtime
}

// ConfigStore is the explicit configuration control dependency needed by the
// API's configuration-management endpoints. Runtime reads still go through the
// namespace-scoped Provider contract.
type ConfigStore interface {
	configapi.Controller
}

type APIConfig struct {
	Version              string
	Mode                 string
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
}

func NewAPIConfig(source APIConfigSource) (api_config *APIConfig, err error) {
	var module *configapi.ModuleHandle
	if host, ok := source.Provider.(configapi.ModuleHost); ok {
		module, err = host.RegisterModule(api_config_declaration)
		if err != nil {
			return nil, fmt.Errorf("register API config: %w", err)
		}
		defer func() {
			if err != nil {
				module.Unregister()
			}
		}()
		if _, err = host.Refresh(context.Background()); err != nil {
			return nil, fmt.Errorf("load API config: %w", err)
		}
	}

	var api_values struct {
		Protocol string `json:"protocol"`
		Hostname string `json:"hostname"`
		Port     int    `json:"port"`
	}
	if err := ConfigDeclaration.Decode(source.Provider, "api", &api_values); err != nil {
		return nil, fmt.Errorf("api config: %w", err)
	}

	var download_values struct {
		Dir              string `json:"dir"`
		PlayDoneAudio    bool   `json:"playDoneAudio"`
		MaxRunning       int    `json:"maxRunning"`
		FilenameTemplate string `json:"filenameTemplate"`
		HooksScript      string `json:"hooksScript"`
		RemoteServer     struct {
			Enabled  bool   `json:"enabled"`
			Protocol string `json:"protocol"`
			Hostname string `json:"hostname"`
			Port     int    `json:"port"`
		} `json:"remoteServer"`
	}
	if err := ConfigDeclaration.Decode(source.Provider, "download", &download_values); err != nil {
		return nil, fmt.Errorf("download config: %w", err)
	}

	var cloudflare_values struct {
		YuanbaoCookie string `json:"sphCookie"`
	}
	if err := ConfigDeclaration.Decode(source.Provider, "cloudflare", &cloudflare_values); err != nil {
		return nil, fmt.Errorf("cloudflare config: %w", err)
	}

	dir := download_values.Dir
	dir = strings.ReplaceAll(dir, "%UserDownloads%", xdg.UserDirs.Download)
	dir = strings.ReplaceAll(dir, "%CWD%", source.Runtime.WorkDir)
	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(source.Runtime.WorkDir, dir)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create download directory %s: %w", dir, err)
	}

	max_running := download_values.MaxRunning
	if max_running <= 0 {
		max_running = 3
	}

	api_config = &APIConfig{
		Version:              source.Runtime.Version,
		Mode:                 source.Runtime.Mode,
		RootDir:              source.Runtime.RootDir,
		WorkDir:              source.Runtime.WorkDir,
		DownloadDir:          dir,
		PlayDoneAudio:        download_values.PlayDoneAudio,
		MaxRunning:           max_running,
		Protocol:             api_values.Protocol,
		Hostname:             api_values.Hostname,
		Port:                 api_values.Port,
		RemoteServerEnabled:  download_values.RemoteServer.Enabled,
		RemoteServerProtocol: download_values.RemoteServer.Protocol,
		RemoteServerHostname: download_values.RemoteServer.Hostname,
		RemoteServerPort:     download_values.RemoteServer.Port,
		CloudflareSphCookie:  cloudflare_values.YuanbaoCookie,
		FilenameTemplate:     download_values.FilenameTemplate,
		HooksScript:          download_values.HooksScript,
	}
	return api_config, nil
}
