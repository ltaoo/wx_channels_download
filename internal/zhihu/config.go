package zhihu

import (
	"github.com/spf13/viper"

	"wx_channel/internal/config"
)

type ZhihuConfig struct {
	RootDir  string
	Disabled bool `json:"zhihuDisabled"`
}

func NewZhihuConfig(c *config.Config) *ZhihuConfig {
	return &ZhihuConfig{
		RootDir:  c.RootDir,
		Disabled: viper.GetBool("zhihu.disabled"),
	}
}
