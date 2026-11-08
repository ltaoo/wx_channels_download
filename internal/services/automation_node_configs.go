package services

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"

	"wx_channel/pkg/flowengine/engine"
)

var node_config_key_pattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validate_node_config_schema(schema []engine.NodeConfigField) error {
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
	}
	return nil
}

func decode_node_configs(raw string) map[string]map[string]interface{} {
	values := map[string]map[string]interface{}{}
	_ = json.Unmarshal([]byte(raw), &values)
	return values
}

// Runtime values must not enter a reusable definition through import or editing.
func clear_runtime_node_configs(definition *engine.FlowDefinition) {
	for node_id, node := range definition.Nodes {
		node.ConfigValues = nil
		if child, ok := node.Config["workflow"].(engine.FlowDefinition); ok {
			clear_runtime_node_configs(&child)
			node.Config["workflow"] = child
		}
		definition.Nodes[node_id] = node
	}
}

// normalize_node_configs prunes removed declarations and validates typed values.
// Graph editing may leave required settings unfilled; saving settings and running may not.
func normalize_node_configs(definition engine.FlowDefinition, values map[string]map[string]interface{}, require_complete bool) (map[string]map[string]interface{}, error) {
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("节点配置无法序列化: %w", err)
	}
	values = decode_node_configs(string(encoded))
	normalized := map[string]map[string]interface{}{}
	for node_id, node := range definition.Nodes {
		if err := validate_node_config_schema(node.ConfigSchema); err != nil {
			return nil, fmt.Errorf("节点 %s: %w", node_id, err)
		}
		for _, field := range node.ConfigSchema {
			value, exists := values[node_id][field.Key]
			text, is_text := value.(string)
			if !exists || value == nil || (is_text && strings.TrimSpace(text) == "") {
				if require_complete && field.Required {
					return nil, fmt.Errorf("节点 %s 的配置项 %s 不能为空", node.Name, field.Key)
				}
				continue
			}
			valid := false
			switch field.Type {
			case "string":
				valid = is_text
			case "number", "integer":
				number, ok := value.(float64)
				valid = ok && !math.IsInf(number, 0) && !math.IsNaN(number) && (field.Type != "integer" || math.Trunc(number) == number)
			case "boolean":
				_, valid = value.(bool)
			case "object":
				_, valid = value.(map[string]interface{})
			case "array":
				_, valid = value.([]interface{})
			}
			if !valid {
				if !require_complete {
					// A changed declaration invalidates the previous value; the
					// automation settings form will collect a replacement.
					continue
				}
				return nil, fmt.Errorf("节点 %s 的配置项 %s 必须为 %s", node.Name, field.Key, field.Type)
			}
			if len(field.Options) > 0 {
				matched := false
				for _, option := range field.Options {
					matched = matched || value == option.Value
				}
				if !matched {
					if !require_complete {
						continue
					}
					return nil, fmt.Errorf("节点 %s 的配置项 %s 必须选择声明的选项", node.Name, field.Key)
				}
			}
			if normalized[node_id] == nil {
				normalized[node_id] = map[string]interface{}{}
			}
			normalized[node_id][field.Key] = value
		}
	}
	return normalized, nil
}

func (s *AutomationService) bind_node_configs(definition *engine.FlowDefinition) error {
	flow, err := s.GetUserFlow(definition.ID)
	if err != nil {
		return err
	}
	values, err := normalize_node_configs(*definition, decode_node_configs(flow.NodeConfigs), true)
	if err != nil {
		return err
	}
	for node_id, node := range definition.Nodes {
		node.ConfigValues = values[node_id]
		definition.Nodes[node_id] = node
	}
	return nil
}
