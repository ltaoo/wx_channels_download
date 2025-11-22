package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	DownloadDefaultHighest bool // 默认下载最高画质
	ProxySystem            bool // 是否设置系统代理
	Port                   int  // 代理端口
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

	viper.SetDefault("download.defaultHighest", false)
	viper.SetDefault("proxy.system", true)
	viper.SetDefault("proxy.port", 2023)

	config := &Config{
		DownloadDefaultHighest: viper.GetBool("download.defaultHighest"),
		ProxySystem:            viper.GetBool("proxy.system"),
		Port:                   viper.GetInt("proxy.port"),
	}

	return config, nil
}
