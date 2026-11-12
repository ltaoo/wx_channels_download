package services

import (
	"encoding/json"
	"fmt"
	"strings"

	"wx_channel/pkg/flowengine/engine"
)

func validate_node_config_schema(schema []engine.NodeConfigField) error {
	return engine.ValidateNodeConfigSchema(schema)
}

func decode_node_configs(raw string) map[string]map[string]interface{} {
	values := map[string]map[string]interface{}{}
	_ = json.Unmarshal([]byte(raw), &values)
	return values
}

// Runtime values must not enter a reusable definition through import or editing.
func clear_runtime_node_configs(definition *engine.FlowDefinition) {
	for id, child := range definition.Subflows {
		clear_runtime_node_configs(&child)
		definition.Subflows[id] = child
	}
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
func normalize_node_configs(definition engine.FlowDefinition, values map[string]map[string]interface{}, require_complete, apply_defaults bool) (map[string]map[string]interface{}, error) {
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
			if apply_defaults && (!exists || value == nil || (is_text && strings.TrimSpace(text) == "")) {
				value, exists = field.Default, field.Default != nil
			}
			text, is_text = value.(string)
			empty_text := is_text && strings.TrimSpace(text) == ""
			if apply_defaults && !field.Required && field.Default == "" {
				empty_text = false
			}
			if !exists || value == nil || empty_text {
				if require_complete && apply_defaults && field.Required {
					return nil, fmt.Errorf("节点 %s 的配置项 %s 不能为空", node.Name, field.Key)
				}
				continue
			}
			valid := engine.ValidNodeConfigValue(field, value)
			if !valid {
				if !require_complete {
					// A changed declaration invalidates the previous value; the
					// automation settings form will collect a replacement.
					continue
				}
				return nil, fmt.Errorf("节点 %s 的配置项 %s 必须为 %s 且符合声明的选项", node.Name, field.Key, field.Type)
			}
			if normalized[node_id] == nil {
				normalized[node_id] = map[string]interface{}{}
			}
			normalized[node_id][field.Key] = value
		}
	}
	return normalized, nil
}

// Override fields replace default fields, including whole object/array values.
func merge_node_configs(defaults, overrides map[string]map[string]interface{}) map[string]map[string]interface{} {
	merged := map[string]map[string]interface{}{}
	for _, layer := range []map[string]map[string]interface{}{defaults, overrides} {
		for node_id, fields := range layer {
			if merged[node_id] == nil {
				merged[node_id] = map[string]interface{}{}
			}
			for key, value := range fields {
				merged[node_id][key] = value
			}
		}
	}
	return merged
}

func (s *AutomationService) bind_node_configs(definition *engine.FlowDefinition, instance_ids ...string) error {
	flow, err := s.GetFlowDefinition(definition.ID)
	if err != nil {
		return err
	}
	current, err := decode_user_flow_definition(flow.Definition)
	if err != nil {
		return err
	}
	*definition = *current
	overrides := map[string]map[string]interface{}{}
	if len(instance_ids) > 0 && instance_ids[0] != "" {
		instance, err := s.GetUserFlowInstance(instance_ids[0])
		if err != nil {
			return err
		}
		if instance.FlowID != flow.ID {
			return fmt.Errorf("实例不属于指定流程")
		}
		// The instance view reads the graph and defaults from the same database snapshot.
		current, err = decode_user_flow_definition(instance.Definition)
		if err != nil {
			return err
		}
		*definition = *current
		flow.DefaultNodeConfigs = instance.DefaultNodeConfigs
		overrides = decode_node_configs(instance.NodeConfigs)
	}
	values, err := normalize_node_configs(*definition, merge_node_configs(decode_node_configs(flow.DefaultNodeConfigs), overrides), true, true)
	if err != nil {
		return err
	}
	for node_id, node := range definition.Nodes {
		node.ConfigValues = values[node_id]
		definition.Nodes[node_id] = node
	}
	return nil
}
