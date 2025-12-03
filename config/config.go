package config

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
)

type Config struct {
	FilePath                     string // 配置文件路径
	DownloadDefaultHighest       bool   `json:"defaultHighest"`             // 默认下载最高画质
	DownloadFilenameTemplate     string `json:"downloadFilenameTemplate"`   // 下载文件名模板
	DownloadPauseWhenDownload    bool   `json:"downloadPauseWhenDownload"`  // 下载时暂停播放
	DownloadLocalServerEnabled   bool   `json:"downloadLocalServerEnabled"` // 下载时是否使用本地服务器
	DownloadLocalServerAddr      string `json:"downloadLocalServerAddr"`    // 下载时本地服务器地址
	ProxySystem                  bool
	Hostname                     string
	Port                         int
	PageSpyServerProtocol        string `json:"pagespyServerProtocol"` // pagespy调试地址协议，如 http
	PageSpyServerAPI             string `json:"pagespyServerAPI"`      // pagespy调试地址，如 debug.weixin.qq.com
	Debug                        bool
	ChannelDisableLocationToHome bool   // 禁止从feed重定向到home
	InjectExtraScriptAfterJSMain string // 额外注入的 js
	InjectGlobalScript           string // 全局用户脚本
}

func LoadConfig() (*Config, error) {
	exe, _ := os.Executable()
	exe_dir := filepath.Dir(exe)
	base_dir := exe_dir
	var candidates []string
	candidates = append(candidates, exe_dir)
	if _, caller_file, _, ok := runtime.Caller(1); ok {
		caller_dir := filepath.Dir(caller_file)
		candidates = append(candidates, caller_dir)
	}
	if _, this_file, _, ok2 := runtime.Caller(0); ok2 {
		cfg_dir := filepath.Dir(this_file)
		proj_root := filepath.Dir(cfg_dir)
		candidates = append(candidates, proj_root)
	}
	var config_filepath string
	var has_config bool
	for _, dir := range candidates {
		p := filepath.Join(dir, "config.yaml")
		if _, err := os.Stat(p); err == nil {
			base_dir = dir
			config_filepath = p
			has_config = true
			break
		}
	}
	if config_filepath == "" {
		config_filepath = filepath.Join(base_dir, "config.yaml")
	}
	viper.SetConfigFile(config_filepath)
	if has_config {
		if err := viper.ReadInConfig(); err != nil {
			var nf viper.ConfigFileNotFoundError
			if !(errors.As(err, &nf) || errors.Is(err, os.ErrNotExist)) {
				return nil, fmt.Errorf("error reading config file: %w", err)
			}
		}
	}

	global_script_path := path.Join(base_dir, "global.js")
	if _, err := os.Stat(global_script_path); err == nil {
		script_byte, err := os.ReadFile(global_script_path)
		if err == nil {
			viper.Set("globalUserScript", string(script_byte))
		}
	}

	viper.SetDefault("download.defaultHighest", false)
	viper.SetDefault("download.filenameTemplate", "{{filename}}_{{spec}}")
	viper.SetDefault("download.pauseWhenDownload", false)
	viper.SetDefault("download.localServer.enabled", false)
	viper.SetDefault("download.localServer.addr", "127.0.0.1:8080")
	viper.SetDefault("proxy.system", true)
	viper.SetDefault("proxy.port", 2023)
	viper.SetDefault("proxy.hostname", "127.0.0.1")
	viper.SetDefault("debug.protocol", "https")
	viper.SetDefault("debug.api", "debug.weixin.qq.com")
	viper.SetDefault("debug", false)
	viper.SetDefault("channel.disableLocationToHome", false)
	viper.SetDefault("inject.extraScript.afterJSMain", "")
	viper.SetDefault("inject.globalScript", "")

	config := &Config{
		DownloadDefaultHighest:       viper.GetBool("download.defaultHighest"),
		DownloadFilenameTemplate:     viper.GetString("download.filenameTemplate"),
		DownloadPauseWhenDownload:    viper.GetBool("download.pauseWhenDownload"),
		DownloadLocalServerEnabled:   viper.GetBool("download.localServer.enabled"),
		DownloadLocalServerAddr:      viper.GetString("download.localServer.addr"),
		ProxySystem:                  viper.GetBool("proxy.system"),
		Port:                         viper.GetInt("proxy.port"),
		Hostname:                     viper.GetString("proxy.hostname"),
		PageSpyServerProtocol:        viper.GetString("debug.protocol"),
		PageSpyServerAPI:             viper.GetString("debug.api"),
		Debug:                        viper.GetBool("debug"),
		ChannelDisableLocationToHome: viper.GetBool("channel.disableLocationToHome"),
		InjectExtraScriptAfterJSMain: viper.GetString("inject.extraScript.afterJSMain"),
		InjectGlobalScript:           viper.GetString("inject.globalScript"),
	}
	if has_config {
		config.FilePath = config_filepath
	}

	extra_js_filepath := config.InjectExtraScriptAfterJSMain
	if extra_js_filepath != "" {
		// If it's a relative path, resolve it against the current working directory
		if !filepath.IsAbs(extra_js_filepath) {
			extra_js_filepath = filepath.Join(base_dir, extra_js_filepath)
		}
		if _, err := os.Stat(extra_js_filepath); err == nil {
			script_byte, err := os.ReadFile(extra_js_filepath)
			if err == nil {
				config.InjectExtraScriptAfterJSMain = string(script_byte)
			}
		}
	}

	return config, nil
}
