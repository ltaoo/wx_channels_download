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
)

type ConfigItem struct {
	Key         string      `json:"key"`
	Type        ConfigType  `json:"type"`
	Default     interface{} `json:"default"`
	Description string      `json:"description"`
	Title       string      `json:"title"`
	Group       string      `json:"group"`             // e.g., "Network", "Download"
	Options     []string    `json:"options,omitempty"` // For select type
}

var Registry []ConfigItem

func Register(item ConfigItem) {
	Registry = append(Registry, item)
	viper.SetDefault(item.Key, item.Default)
}

func GetSchema() []ConfigItem {
	return Registry
}
