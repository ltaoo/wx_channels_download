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

type ConfigField struct {
	Key          string               `json:"key"`
	Type         ConfigType           `json:"type"`
	Default      interface{}          `json:"default"`
	Description  string               `json:"description"`
	Title        string               `json:"title"`
	Group        string               `json:"group"`             // e.g., "Network", "Download"
	Options      []string             `json:"options,omitempty"` // For select type
	Deprecated   bool                 `json:"deprecated,omitempty"`
	Readonly     bool                 `json:"readonly,omitempty"`
	Accept       string               `json:"accept,omitempty"`    // For file type
	HotReload    bool                 `json:"hotReload,omitempty"` // Whether hot reload is supported; true means changes take effect immediately without restart
	ProcessValue ConfigValueProcessor `json:"-"`                   // Converts the configured value to its runtime value.
}

type ConfigValueProcessor func(value interface{}, ctx ConfigValueContext) interface{}

type ConfigValueContext struct {
	Config *Config
}

var Registry []ConfigField

func Register(item ConfigField) {
	Registry = append(Registry, item)
	viper.SetDefault(item.Key, item.Default)
}

func Lookup(key string) (ConfigField, bool) {
	for i := len(Registry) - 1; i >= 0; i-- {
		if Registry[i].Key == key {
			return Registry[i], true
		}
	}
	return ConfigField{}, false
}

func GetSchema() []ConfigField {
	return Registry
}
