package config

import (
	"github.com/spf13/viper"
)

type ConfigType string

const (
	ConfigTypeString ConfigType = "string"
	ConfigTypeBool   ConfigType = "boolean"
	ConfigTypeInt    ConfigType = "number"
	ConfigTypeFloat  ConfigType = "number"
	ConfigTypeSelect ConfigType = "select"
	ConfigTypeFile   ConfigType = "file"
	ConfigTypeText   ConfigType = "textarea"
)

type ConfigItem struct {
	Key         string      `json:"key"`
	Type        ConfigType  `json:"type"`
	Default     interface{} `json:"default"`
	Description string      `json:"description"`
	Title       string      `json:"title"`
	Group       string      `json:"group"`             // e.g., "Network", "Download"
	Options     []string    `json:"options,omitempty"` // For select type
	Deprecated  bool        `json:"deprecated,omitempty"`
	Readonly    bool        `json:"readonly,omitempty"`
	Accept      string      `json:"accept,omitempty"` // For file type
	HotReload   bool        `json:"hotReload,omitempty"` // 是否支持热重载，true 表示修改后立即生效无需重启
}

var Registry []ConfigItem

func Register(item ConfigItem) {
	Registry = append(Registry, item)
	viper.SetDefault(item.Key, item.Default)
}

func GetSchema() []ConfigItem {
	return Registry
}
