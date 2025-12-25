package interceptor

import (
	"os"
	"path"
	"path/filepath"

	"github.com/spf13/viper"

	"wx_channel/config"
)

type InterceptorSettings struct {
	Version                      string `json:"version"`
	FilePath                     string // 配置文件路径
	DownloadDefaultHighest       bool   `json:"defaultHighest"`            // 默认下载最高画质
	DownloadFilenameTemplate     string `json:"downloadFilenameTemplate"`  // 下载文件名模板
	DownloadPauseWhenDownload    bool   `json:"downloadPauseWhenDownload"` // 下载时暂停播放
	APIServerAddr                string `json:"apiServerAddr"`             // API服务器地址
	ProxyDevice                  string
	ProxySetSystem               bool
	ProxyServerHostname          string
	ProxyServerPort              int
	PagespyEnabled               bool
	PageppyServerProtocol        string `json:"pagespyServerProtocol"` // pagespy调试地址协议，如 http
	PageppyServerAPI             string `json:"pagespyServerAPI"`      // pagespy调试地址，如 debug.weixin.qq.com
	DebugShowError               bool
	ChannelDisableLocationToHome bool   // 禁止从feed重定向到home
	InjectExtraScriptAfterJSMain string // 额外注入的 js
	InjectGlobalScript           string // 全局用户脚本

	// CertFiles *certificate.CertFileAndKeyFile
	t *config.Config
}

func SetDefaultSettings(cfg *config.Config) {
	config.Register(config.ConfigItem{
		Key:         "download.defaultHighest",
		Type:        config.ConfigTypeBool,
		Default:     false,
		Description: "默认下载原始视频",
		Title:       "原始视频",
		Group:       "Download",
	})
	config.Register(config.ConfigItem{
		Key:         "download.filenameTemplate",
		Type:        config.ConfigTypeString,
		Default:     "{{filename}}_{{spec}}",
		Description: "下载文件名模板，支持 {{filename}} 和 {{spec}} 变量",
		Title:       "文件名模板",
		Group:       "Download",
	})
	config.Register(config.ConfigItem{
		Key:         "download.pauseWhenDownload",
		Type:        config.ConfigTypeBool,
		Default:     false,
		Description: "下载时暂停播放",
		Title:       "暂停播放",
		Group:       "Download",
	})
	config.Register(config.ConfigItem{
		Key:         "download.localServer.enabled",
		Type:        config.ConfigTypeBool,
		Default:     false,
		Description: "是否开启本地服务器",
		Title:       "本地服务器",
		Group:       "Download",
	})
	config.Register(config.ConfigItem{
		Key:         "download.localServer.addr",
		Type:        config.ConfigTypeString,
		Default:     "127.0.0.1:8080",
		Description: "本地服务器地址",
		Title:       "本地服务器地址",
		Group:       "Download",
	})
	config.Register(config.ConfigItem{
		Key:         "proxy.system",
		Type:        config.ConfigTypeBool,
		Default:     true,
		Description: "是否设置系统代理",
		Title:       "系统代理",
		Group:       "Proxy",
	})
	config.Register(config.ConfigItem{
		Key:         "proxy.hostname",
		Type:        config.ConfigTypeString,
		Default:     "127.0.0.1",
		Description: "代理主机名",
		Title:       "代理主机",
		Group:       "Proxy",
	})
	config.Register(config.ConfigItem{
		Key:         "proxy.port",
		Type:        config.ConfigTypeInt,
		Default:     2080,
		Description: "代理端口",
		Title:       "代理端口",
		Group:       "Proxy",
	})
	config.Register(config.ConfigItem{
		Key:         "pagespy.enabled",
		Type:        config.ConfigTypeSelect,
		Default:     false,
		Description: "是否开启 PageSpy",
		Title:       "启用",
		Group:       "Pagespy",
	})
	config.Register(config.ConfigItem{
		Key:         "pagespy.protocol",
		Type:        config.ConfigTypeSelect,
		Default:     "https",
		Options:     []string{"http", "https"},
		Description: "PageSpy 调试协议",
		Title:       "协议头",
		Group:       "Pagespy",
	})
	config.Register(config.ConfigItem{
		Key:         "pagespy.api",
		Type:        config.ConfigTypeString,
		Default:     "debug.weixin.qq.com",
		Description: "PageSpy 调试 API 地址",
		Title:       "API 地址",
		Group:       "Pagespy",
	})
	config.Register(config.ConfigItem{
		Key:         "debug.error",
		Type:        config.ConfigTypeBool,
		Default:     true,
		Description: "在弹窗展示错误信息",
		Title:       "错误展示",
		Group:       "Debug",
	})
	config.Register(config.ConfigItem{
		Key:         "channel.disableLocationToHome",
		Type:        config.ConfigTypeBool,
		Default:     false,
		Description: "禁止从 Feed 重定向到 Home",
		Title:       "禁止重定向",
		Group:       "Channel",
	})
	config.Register(config.ConfigItem{
		Key:         "inject.extraScript.afterJSMain",
		Type:        config.ConfigTypeString,
		Default:     "",
		Description: "额外注入的 JS 脚本路径",
		Title:       "注入脚本",
		Group:       "Inject",
	})
	config.Register(config.ConfigItem{
		Key:         "inject.globalScript",
		Type:        config.ConfigTypeString,
		Default:     "",
		Description: "全局用户脚本",
		Title:       "全局脚本",
		Group:       "Inject",
	})
}

func NewInterceptorSettings(c *config.Config) *InterceptorSettings {
	settings := &InterceptorSettings{
		DownloadDefaultHighest:       viper.GetBool("download.defaultHighest"),
		DownloadFilenameTemplate:     viper.GetString("download.filenameTemplate"),
		DownloadPauseWhenDownload:    viper.GetBool("download.pauseWhenDownload"),
		APIServerAddr:                viper.GetString("api.addr"),
		ProxySetSystem:               viper.GetBool("proxy.system"),
		ProxyServerPort:              viper.GetInt("proxy.port"),
		ProxyServerHostname:          viper.GetString("proxy.hostname"),
		PagespyEnabled:               viper.GetBool("pagespy.enabled"),
		PageppyServerProtocol:        viper.GetString("pagespy.protocol"),
		PageppyServerAPI:             viper.GetString("pagespy.api"),
		DebugShowError:               viper.GetBool("debug.error"),
		ChannelDisableLocationToHome: viper.GetBool("channel.disableLocationToHome"),
		InjectExtraScriptAfterJSMain: viper.GetString("inject.extraScript.afterJSMain"),
		InjectGlobalScript:           viper.GetString("inject.globalScript"),
		// CertFiles:                    cert,
		t: c,
	}
	global_script_path := path.Join(c.RootDir, "global.js")
	if _, err := os.Stat(global_script_path); err == nil {
		script_byte, err := os.ReadFile(global_script_path)
		if err == nil {
			settings.InjectGlobalScript = string(script_byte)
		}
	}
	extra_js_filepath := settings.InjectExtraScriptAfterJSMain
	if extra_js_filepath != "" {
		// If it's a relative path, resolve it against the current working directory
		if !filepath.IsAbs(extra_js_filepath) {
			extra_js_filepath = filepath.Join(c.RootDir, extra_js_filepath)
		}
		if _, err := os.Stat(extra_js_filepath); err == nil {
			script_byte, err := os.ReadFile(extra_js_filepath)
			if err == nil {
				settings.InjectExtraScriptAfterJSMain = string(script_byte)
			}
		}
	}
	return settings
}
