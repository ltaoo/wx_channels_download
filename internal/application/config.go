package application

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/viper"

	"wx_channel/internal/config"
)

// PrepareConfig applies an optional config file path and loads the application configuration.
func PrepareConfig(cfg *config.Config, configFilepath string) error {
	if configFilepath != "" {
		abs, err := filepath.Abs(configFilepath)
		if err != nil {
			return fmt.Errorf("配置文件路径无效 %w", err)
		}
		viper.SetConfigFile(abs)
		cfg.Filename = filepath.Base(abs)
		cfg.FullPath = abs
		cfg.RootDir = filepath.Dir(abs)
		if _, err := os.Stat(abs); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("配置文件 %v 不存在", color.New(color.FgBlue, color.Underline).Sprint(abs))
			}
			return fmt.Errorf("读取配置文件失败 %w", err)
		}
		cfg.Existing = true
	}
	if err := cfg.LoadConfig(); err != nil {
		return fmt.Errorf("加载配置文件失败 %w", err)
	}
	return nil
}
