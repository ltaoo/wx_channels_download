package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

type ConfigType string

type ConfigFieldSource string

const (
	ConfigTypeString ConfigType = "string"
	ConfigTypeBool   ConfigType = "boolean"
	ConfigTypeInt    ConfigType = "number"
	ConfigTypeFloat  ConfigType = "number"
	ConfigTypeSelect ConfigType = "select"
	ConfigTypeFile   ConfigType = "file"
	ConfigTypeText   ConfigType = "textarea"

	ConfigFieldSourceApplication ConfigFieldSource = "application"
	ConfigFieldSourcePlugin      ConfigFieldSource = "plugin"
)

type ConfigField struct {
	Key          string               `json:"key"`
	Type         ConfigType           `json:"type"`
	Default      interface{}          `json:"default"`
	Description  string               `json:"description"`
	Title        string               `json:"title"`
	Group        string               `json:"group"` // e.g., "Network", "Download"
	Source       ConfigFieldSource    `json:"source"`
	Namespace    string               `json:"namespace,omitempty"`
	Options      []string             `json:"options,omitempty"` // For select type
	Deprecated   bool                 `json:"deprecated,omitempty"`
	Readonly     bool                 `json:"readonly,omitempty"`
	Sensitive    bool                 `json:"sensitive,omitempty"`
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
	if item.Source == "" {
		item.Source = ConfigFieldSourceApplication
	}
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

// NormalizeValue validates and normalizes a JSON-decoded value according to a
// registered configuration field.
func NormalizeValue(field ConfigField, value interface{}) (interface{}, error) {
	if field.Readonly {
		return nil, fmt.Errorf("配置项 %s 为只读", field.Key)
	}

	switch field.Type {
	case ConfigTypeBool:
		bool_value, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("配置项 %s 必须是布尔值", field.Key)
		}
		return bool_value, nil
	case ConfigTypeInt:
		return normalize_number(field, value)
	case ConfigTypeSelect:
		string_value, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("配置项 %s 必须是字符串", field.Key)
		}
		for _, option := range field.Options {
			if string_value == option {
				return string_value, nil
			}
		}
		return nil, fmt.Errorf("配置项 %s 必须是以下值之一: %s", field.Key, strings.Join(field.Options, ", "))
	case ConfigTypeString, ConfigTypeFile, ConfigTypeText:
		string_value, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("配置项 %s 必须是字符串", field.Key)
		}
		return string_value, nil
	default:
		return nil, fmt.Errorf("配置项 %s 使用了不支持的类型 %s", field.Key, field.Type)
	}
}

func normalize_number(field ConfigField, value interface{}) (interface{}, error) {
	json_number, ok := value.(json.Number)
	if ok {
		value = string(json_number)
	}

	integer_field := true
	switch field.Default.(type) {
	case float32, float64:
		integer_field = false
	}

	if integer_field {
		switch number_value := value.(type) {
		case int:
			return number_value, nil
		case int8:
			return int(number_value), nil
		case int16:
			return int(number_value), nil
		case int32:
			return int(number_value), nil
		case int64:
			return int(number_value), nil
		case float64:
			integer_value := int(number_value)
			if number_value == float64(integer_value) {
				return integer_value, nil
			}
		case string:
			integer_value, err := strconv.Atoi(number_value)
			if err == nil {
				return integer_value, nil
			}
		}
		return nil, fmt.Errorf("配置项 %s 必须是整数", field.Key)
	}

	switch number_value := value.(type) {
	case float64:
		return number_value, nil
	case float32:
		return float64(number_value), nil
	case int:
		return float64(number_value), nil
	case string:
		float_value, err := strconv.ParseFloat(number_value, 64)
		if err == nil {
			return float_value, nil
		}
	}
	return nil, fmt.Errorf("配置项 %s 必须是数字", field.Key)
}
