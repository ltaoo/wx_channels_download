package interceptor

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/viper"

	"wx_channel/internal/config"
)

type InterceptorConfig struct {
	Version                             string `json:"version"`
	FilePath                            string // 配置文件路径
	DownloadDefaultHighest              bool   `json:"defaultHighest"`             // 默认下载最高画质
	DownloadFilenameTemplate            string `json:"downloadFilenameTemplate"`   // 下载文件名模板
	DownloadPauseWhenDownload           bool   `json:"downloadPauseWhenDownload"`  // 下载时暂停播放
	DownloadInFrontend                  bool   `json:"downloadInFrontend"`         // 在前端下载
	DownloadMaxRunning                  int    `json:"downloadMaxRunning"`         // 最大同时下载数
	DownloadForceCheckAllFeeds          bool   `json:"downloadForceCheckAllFeeds"` // 批量下载时是否强制检查所有视频
	APIServerProtocol                   string `json:"apiServerProtocol"`          // API服务器主机名
	APIServerHostname                   string `json:"apiServerHostname"`          // API服务器主机名
	APIServerPort                       int    `json:"apiServerPort"`              // API服务器端口
	APIServerAddr                       string `json:"apiServerAddr"`
	RemoteServerEnabled                 bool   `json:"remoteServerEnabled"`
	RemoteServerProtocol                string `json:"remoteServerProtocol"`
	RemoteServerHostname                string `json:"remoteServerHostname"`
	RemoteServerPort                    int    `json:"remoteServerPort"`
	OfficialAccountServerRefreshToken   string `json:"officialServerRefreshToken"`
	OfficialAccountEnabled              bool   `json:"officialAccountEnabled"`
	OfficialAccountRemoteServerProtocol string `json:"officialRemoteServerProtocol"`
	OfficialAccountRemoteServerHostname string `json:"officialRemoteServerHostname"`
	OfficialAccountRemoteServerPort     int    `json:"officialRemoteServerPort"`
	ProxyDevice                         string
	ProxySetSystem                      bool
	ProxyTun                            bool
	ProxyDefaultInterface               string
	ProxyServerHostname                 string
	ProxyServerPort                     int
	ProxyTCPRelayEnabled                bool
	ProxyTCPRelayHostname               string
	ProxyTCPRelayPort                   int
	ProxySkipInstallRootCert            bool
	ProxyUpstreamProxy                  string
	PagespyEnabled                      bool
	PageppyServerProtocol               string `json:"pagespyServerProtocol"` // pagespy调试地址协议，如 http
	PageppyServerAPI                    string `json:"pagespyServerAPI"`      // pagespy调试地址，如 debug.weixin.qq.com
	DebugShowError                      bool
	EchoLogEnabled                      bool
	InDocker                            bool   `json:"inDocker"` // 是否在 Docker 容器中运行
	ChannelsDisableLocationToHome       bool   // 禁止从feed重定向到home
	InjectExtraScriptAfterJSMain        string // 额外注入的 js
	InjectGlobalScriptFilepath          string // 全局脚本路径
	InjectGlobalScript                  string // 全局用户脚本

	// CertFiles *certificate.CertFileAndKeyFile
	t *config.Config
}

func NewInterceptorSettings(c *config.Config) *InterceptorConfig {
	settings := &InterceptorConfig{
		Version:                             c.Version,
		DebugShowError:                      viper.GetBool("debug.error"),
		EchoLogEnabled:                      viper.GetBool("debug.echolog"),
		PagespyEnabled:                      viper.GetBool("pagespy.enabled"),
		PageppyServerProtocol:               viper.GetString("pagespy.protocol"),
		PageppyServerAPI:                    viper.GetString("pagespy.api"),
		ChannelsDisableLocationToHome:       viper.GetBool("channel.disableLocationToHome"),
		DownloadDefaultHighest:              viper.GetBool("download.defaultHighest"),
		DownloadFilenameTemplate:            viper.GetString("download.filenameTemplate"),
		DownloadPauseWhenDownload:           viper.GetBool("download.pauseWhenDownload"),
		DownloadInFrontend:                  viper.GetBool("download.frontend"),
		DownloadMaxRunning:                  viper.GetInt("download.maxRunning"),
		DownloadForceCheckAllFeeds:          viper.GetBool("download.forceCheckAllFeeds"),
		APIServerProtocol:                   viper.GetString("api.protocol"),
		APIServerHostname:                   viper.GetString("api.hostname"),
		APIServerPort:                       viper.GetInt("api.port"),
		APIServerAddr:                       viper.GetString("api.hostname") + ":" + strconv.Itoa(viper.GetInt("api.port")),
		RemoteServerEnabled:                 viper.GetBool("download.remoteServer.enabled"),
		RemoteServerProtocol:                viper.GetString("download.remoteServer.protocol"),
		RemoteServerHostname:                viper.GetString("download.remoteServer.hostname"),
		RemoteServerPort:                    viper.GetInt("download.remoteServer.port"),
		OfficialAccountEnabled:              config.IsMPEnabled(),
		OfficialAccountServerRefreshToken:   viper.GetString("mp.refreshToken"),
		OfficialAccountRemoteServerProtocol: viper.GetString("mp.remoteServer.protocol"),
		OfficialAccountRemoteServerHostname: viper.GetString("mp.remoteServer.hostname"),
		OfficialAccountRemoteServerPort:     viper.GetInt("mp.remoteServer.port"),
		ProxySetSystem:                      viper.GetBool("proxy.system"),
		ProxyTun:                            viper.GetBool("proxy.tun"),
		ProxyDefaultInterface:               viper.GetString("proxy.defaultInterface"),
		ProxyServerPort:                     viper.GetInt("proxy.port"),
		ProxyServerHostname:                 viper.GetString("proxy.hostname"),
		ProxyTCPRelayEnabled:                viper.GetBool("proxy.tcpRelay.enabled"),
		ProxyTCPRelayHostname:               viper.GetString("proxy.tcpRelay.hostname"),
		ProxyTCPRelayPort:                   viper.GetInt("proxy.tcpRelay.port"),
		ProxySkipInstallRootCert:            viper.GetBool("proxy.skipInstallRootCert"),
		ProxyUpstreamProxy:                  viper.GetString("proxy.upstreamProxy"),
		InjectExtraScriptAfterJSMain:        viper.GetString("inject.extraScript.afterJSMain"),
		InjectGlobalScriptFilepath:          viper.GetString("inject.globalScript"),
		InDocker:                            os.Getenv("WX_IN_DOCKER") == "1",
		t:                                   c,
	}
	if viper.GetBool("channels.disableLocationToHome") {
		// channels.disableLocationToHome 是新的写法，之前 channel.disableLocationToHome 不对
		// 所以这里做个兼容，保证旧的配置项仍然有效
		settings.ChannelsDisableLocationToHome = true
	}
	globalScriptFilepath := resolveScriptPath(c.RootDir, settings.InjectGlobalScriptFilepath)
	if script, ok := readScriptFile(globalScriptFilepath); ok {
		settings.InjectGlobalScriptFilepath = globalScriptFilepath
		settings.InjectGlobalScript = script
	}
	extra_js_filepath := settings.InjectExtraScriptAfterJSMain
	if extra_js_filepath != "" {
		extra_js_filepath = resolveScriptPath(c.RootDir, extra_js_filepath)
		if script, ok := readScriptFile(extra_js_filepath); ok {
			settings.InjectExtraScriptAfterJSMain = script
		}
	}
	return settings
}

func resolveScriptPath(rootDir, scriptPath string) string {
	if scriptPath == "" || filepath.IsAbs(scriptPath) {
		return scriptPath
	}
	return filepath.Join(rootDir, scriptPath)
}

func readScriptFile(scriptPath string) (string, bool) {
	if scriptPath == "" {
		return "", false
	}
	scriptByte, err := os.ReadFile(scriptPath)
	if err != nil {
		return "", false
	}
	return string(scriptByte), true
}
