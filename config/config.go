package config

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	DownloadDefaultHighest       bool   `json:"defaultHighest"`           // 默认下载最高画质
	DownloadFilenameTemplate     string `json:"downloadFilenameTemplate"` // 下载文件名模板
	ProxySystem                  bool
	Port                         int
	PageSpyServerProtocol        string `json:"pagespyServerProtocol"` // pagespy调试地址协议，如 http
	PageSpyServerAPI             string `json:"pagespyServerAPI"`      // pagespy调试地址，如 debug.weixin.qq.com
	GlobalUserScript             string // 全局用户脚本
	Debug                        bool
	InjectExtraScriptAfterJSMain string // 额外注入的 js
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	cwd, _ := os.Getwd()
	global_script_path := path.Join(cwd, "global.js")
	if _, err := os.Stat(global_script_path); err == nil {
		script_byte, err := os.ReadFile(global_script_path)
		if err == nil {
			viper.Set("globalUserScript", string(script_byte))
		}
	}

	viper.SetDefault("download.defaultHighest", false)
	viper.SetDefault("download.filenameTemplate", "{{filename}}-{{spec}}")
	viper.SetDefault("proxy.system", true)
	viper.SetDefault("proxy.port", 2023)
	viper.SetDefault("debug.protocol", "https")
	viper.SetDefault("debug.api", "debug.weixin.qq.com")
	viper.SetDefault("globalUserScript", "")
	viper.SetDefault("debug", false)
	viper.SetDefault("inject.extraScript.afterJSMain", "")

	config := &Config{
		DownloadDefaultHighest:       viper.GetBool("download.defaultHighest"),
		DownloadFilenameTemplate:     viper.GetString("download.filenameTemplate"),
		ProxySystem:                  viper.GetBool("proxy.system"),
		Port:                         viper.GetInt("proxy.port"),
		PageSpyServerProtocol:        viper.GetString("debug.protocol"),
		PageSpyServerAPI:             viper.GetString("debug.api"),
		GlobalUserScript:             viper.GetString("globalUserScript"),
		Debug:                        viper.GetBool("debug"),
		InjectExtraScriptAfterJSMain: viper.GetString("inject.extraScript.afterJSMain"),
	}

	extra_js_filepath := config.InjectExtraScriptAfterJSMain
	if extra_js_filepath != "" {
		// If it's a relative path, resolve it against the current working directory
		if !filepath.IsAbs(extra_js_filepath) {
			extra_js_filepath = filepath.Join(cwd, extra_js_filepath)
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
