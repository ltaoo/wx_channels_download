package config

import (
	"fmt"
	"os"
	"path"

	"github.com/spf13/viper"
)

type Config struct {
	DownloadDefaultHighest   bool   `json:"defaultHighest"`           // 默认下载最高画质
	DownloadFilenameTemplate string `json:"downloadFilenameTemplate"` // 下载文件名模板
	ProxySystem              bool
	Port                     int
	PageSpyServerProtocol    string `json:"pagespyServerProtocol"` // pagespy调试地址协议，如 http
	PageSpyServerAPI         string `json:"pagespyServerAPI"`      // pagespy调试地址，如 debug.weixin.qq.com
	GlobalUserScript         string // 全局用户脚本
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

	config := &Config{
		DownloadDefaultHighest:   viper.GetBool("download.defaultHighest"),
		DownloadFilenameTemplate: viper.GetString("download.filenameTemplate"),
		ProxySystem:              viper.GetBool("proxy.system"),
		Port:                     viper.GetInt("proxy.port"),
		PageSpyServerProtocol:    viper.GetString("debug.protocol"),
		PageSpyServerAPI:         viper.GetString("debug.api"),
		GlobalUserScript:         viper.GetString("globalUserScript"),
	}

	return config, nil
}
