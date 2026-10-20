package services

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"wx_channel/pkg/flowengine/engine"
)

func automation_additional_node_catalog() []UserFlowNodeCatalogItem {
	return []UserFlowNodeCatalogItem{
		{Type: "WorkflowNode", Name: "子流程", Description: "调用独立作用域的子流程，共享取消与步骤预算", ConfigKeys: []UserFlowConfigKey{
			{Key: "flow_id", Type: "string", Description: "已保存流程 ID；与内嵌 workflow 二选一"},
			{Key: "workflow", Type: "object", Description: "内嵌 FlowDefinition"},
			{Key: "arguments", Type: "object", Description: "子流程入参，支持作用域模板"},
			{Key: "output_key", Type: "string", Description: "默认 workflow_result，含 data/output/global"},
		}},
		{Type: "LoopNode", Name: "数组迭代", Description: "逐项执行子流程，按原顺序汇总结果", ConfigKeys: []UserFlowConfigKey{
			{Key: "items", Type: "array", Required: true, Description: "数组或引用数组的完整模板"},
			{Key: "flow_id", Type: "string", Description: "子流程 ID；或使用 workflow"},
			{Key: "workflow", Type: "object", Description: "子流程须声明 item/index 入参"},
			{Key: "arguments", Type: "object", Description: "附加子流程入参"},
			{Key: "max_iterations", Type: "number", Description: "默认 1000，最大 10000；超出直接失败"},
			{Key: "concurrency", Type: "number", Description: "默认 1，最大 32"},
			{Key: "continue_on_error", Type: "boolean", Description: "默认 false；true 时失败项输出 error/index"},
			{Key: "output_key", Type: "string", Description: "默认 loop_result"},
		}},
		{Type: "WaitNode", Name: "延时等待", Description: "等待指定秒数，响应运行取消与整体超时", ConfigKeys: []UserFlowConfigKey{{Key: "seconds", Type: "number", Required: true, Description: "0～86400 秒；重启中断后需显式重试"}}},
		{Type: "FailNode", Name: "终止并报错", Description: "按业务条件主动失败，可接重试和错误分支", ConfigKeys: []UserFlowConfigKey{{Key: "message", Type: "string", Description: "错误说明，支持作用域模板"}}},
	}
}

func validate_node_execution_policy(node engine.NodeDefinition) error {
	if policy := node.RetryPolicy; policy != nil {
		if policy.MaxAttempts < 1 || policy.MaxAttempts > 20 {
			return fmt.Errorf("retry.max_attempts 必须为 1～20")
		}
		if policy.DelayMs < 0 || policy.DelayMs > 3600000 {
			return fmt.Errorf("retry.delay_ms 必须为 0～3600000")
		}
		if math.IsNaN(policy.BackoffMultiplier) || math.IsInf(policy.BackoffMultiplier, 0) || policy.BackoffMultiplier < 0 || policy.BackoffMultiplier > 10 {
			return fmt.Errorf("retry.backoff_multiplier 必须为 0～10")
		}
	}
	return nil
}

func node_config_integer(config map[string]interface{}, key string, fallback, minimum, maximum int) error {
	raw, exists := config[key]
	if !exists {
		return nil
	}
	value := 0.0
	switch number := raw.(type) {
	case float64:
		value = number
	case int:
		value = float64(number)
	case int64:
		value = float64(number)
	default:
		return fmt.Errorf("%s 必须为整数", key)
	}
	if value != math.Trunc(value) || value < float64(minimum) || value > float64(maximum) {
		return fmt.Errorf("%s 必须为 %d～%d 的整数", key, minimum, maximum)
	}
	return nil
}

func validate_automation_node_config(node_type string, config map[string]interface{}) error {
	switch node_type {
	case "APICallNode":
		if address, _ := config["url"].(string); strings.TrimSpace(address) == "" {
			return fmt.Errorf("url 不能为空")
		}
		method, _ := config["method"].(string)
		method = strings.ToUpper(method)
		switch method {
		case "", "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		default:
			return fmt.Errorf("HTTP method 不支持: %s", method)
		}
		for _, key := range []string{"headers", "query", "authentication", "keys"} {
			if raw, exists := config[key]; exists && raw != nil {
				if _, ok := raw.(map[string]interface{}); !ok {
					return fmt.Errorf("%s 必须为 JSON 对象", key)
				}
			}
		}
		body_type, _ := config["body_type"].(string)
		switch body_type {
		case "", "json", "form", "raw":
		default:
			return fmt.Errorf("body_type 仅支持 json/form/raw")
		}
		if err := node_config_integer(config, "timeout_seconds", 15, 1, 86400); err != nil {
			return err
		}
	case "JSCodeNode":
		if code, _ := config["code"].(string); strings.TrimSpace(code) == "" {
			return fmt.Errorf("code 不能为空")
		}
		if key, _ := config["output_key"].(string); strings.TrimSpace(key) == "" {
			return fmt.Errorf("JSCodeNode 必须声明 output_key")
		}
	case "ExprNode":
		if expression, _ := config["expression"].(string); strings.TrimSpace(expression) == "" {
			return fmt.Errorf("expression 不能为空")
		}
	case "SetVariableNode":
		if _, ok := config["variables"].(map[string]interface{}); !ok {
			return fmt.Errorf("variables 必须为 JSON 对象")
		}
	case "GatewayNode":
		gateway_type, _ := config["gateway_type"].(string)
		if gateway_type == "" {
			config["gateway_type"] = "Exclusive"
			gateway_type = "Exclusive"
		}
		if gateway_type != "Exclusive" && gateway_type != "Parallel" {
			return fmt.Errorf("gateway_type 仅支持 Exclusive/Parallel")
		}
	case "LoopNode", "WorkflowNode":
		flow_id, _ := config["flow_id"].(string)
		if config["workflow"] == nil && strings.TrimSpace(flow_id) == "" {
			return fmt.Errorf("需要配置 flow_id 或 workflow")
		}
		if _, exists := config["arguments"]; exists {
			if _, ok := config["arguments"].(map[string]interface{}); !ok {
				return fmt.Errorf("arguments 必须为 JSON 对象")
			}
		}
		if node_type == "WorkflowNode" {
			if _, exists := config["output_key"]; !exists {
				config["output_key"] = "workflow_result"
			}
		}
		if node_type == "LoopNode" {
			if _, exists := config["items"]; !exists {
				return fmt.Errorf("items 不能为空")
			}
			if err := node_config_integer(config, "max_iterations", 1000, 1, 10000); err != nil {
				return err
			}
			if err := node_config_integer(config, "concurrency", 1, 1, 32); err != nil {
				return err
			}
		}
	case "WaitNode":
		return node_config_integer(config, "seconds", 1, 0, 86400)
	}
	return nil
}

// Freeze referenced dependencies too, so retries never silently call a newer child.
func (s *AutomationService) snapshot_flow_dependencies(definition *engine.FlowDefinition, depth int) error {
	if depth > 16 {
		return fmt.Errorf("子流程引用循环或超过 16 层")
	}
	for id, node := range definition.Nodes {
		if node.Type != "WorkflowNode" && node.Type != "LoopNode" {
			continue
		}
		var child engine.FlowDefinition
		if flow_id, _ := node.Config["flow_id"].(string); flow_id != "" {
			value, ok := s.flow_engine.GetFlowDefinition(flow_id)
			if !ok {
				return fmt.Errorf("子流程不存在: %s", flow_id)
			}
			child = value
		} else {
			raw, err := json.Marshal(node.Config["workflow"])
			if err != nil {
				return err
			}
			if err := json.Unmarshal(raw, &child); err != nil {
				return err
			}
		}
		if err := normalize_user_flow_definition(&child); err != nil {
			return err
		}
		for _, child_node := range child.Nodes {
			if child_node.Type == "ManualNode" {
				return fmt.Errorf("子流程内暂不支持人工等待")
			}
		}
		if err := s.snapshot_flow_dependencies(&child, depth+1); err != nil {
			return err
		}
		delete(node.Config, "flow_id")
		node.Config["workflow"] = child
		definition.Nodes[id] = node
	}
	return nil
}
