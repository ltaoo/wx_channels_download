package engine

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

var node_config_key_pattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func ValidateNodeConfigSchema(schema []NodeConfigField) error {
	seen := map[string]bool{}
	for _, field := range schema {
		if !node_config_key_pattern.MatchString(field.Key) || seen[field.Key] {
			return fmt.Errorf("配置项名称 %q 必须唯一，且只能使用字母、数字、下划线，不能以数字开头", field.Key)
		}
		seen[field.Key] = true
		switch field.Type {
		case "string", "number", "integer", "boolean", "object", "array":
		default:
			return fmt.Errorf("配置项 %s 类型不支持: %s", field.Key, field.Type)
		}
		if field.Secret && field.Type != "string" {
			return fmt.Errorf("配置项 %s 只有 string 类型支持 secret", field.Key)
		}
		if field.Multiline && (field.Type != "string" || field.Secret || len(field.Options) > 0) {
			return fmt.Errorf("配置项 %s 多行输入仅用于非密钥且无选项的 string 类型", field.Key)
		}
		if len(field.Options) > 0 && (field.Type != "string" || field.Secret) {
			return fmt.Errorf("配置项 %s 选项仅用于非密钥的 string 类型", field.Key)
		}
		options := map[string]bool{}
		for _, option := range field.Options {
			if strings.TrimSpace(option.Value) == "" || options[option.Value] {
				return fmt.Errorf("配置项 %s 选项值不能为空或重复", field.Key)
			}
			options[option.Value] = true
		}
		if field.Default != nil && !ValidNodeConfigValue(field, field.Default) {
			return fmt.Errorf("配置项 %s 默认值必须为 %s，且符合声明的选项", field.Key, field.Type)
		}
	}
	return nil
}

// ValidNodeConfigValue checks a declared setting, including false and zero defaults.
func ValidNodeConfigValue(field NodeConfigField, value interface{}) bool {
	valid := false
	switch field.Type {
	case "string":
		_, valid = value.(string)
	case "number", "integer":
		number := 0.0
		switch typed := value.(type) {
		case float64:
			number, valid = typed, true
		case int:
			number, valid = float64(typed), true
		case int64:
			number, valid = float64(typed), true
		}
		valid = valid && !math.IsInf(number, 0) && !math.IsNaN(number) && (field.Type != "integer" || math.Trunc(number) == number)
	case "boolean":
		_, valid = value.(bool)
	case "object":
		_, valid = value.(map[string]interface{})
	case "array":
		_, valid = value.([]interface{})
	}
	if len(field.Options) > 0 {
		matched := false
		for _, option := range field.Options {
			matched = matched || value == option.Value
		}
		valid = valid && matched
	}
	return valid
}
