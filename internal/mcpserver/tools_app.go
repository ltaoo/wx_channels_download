package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type update_config_arguments struct {
	Values map[string]any `json:"values"`
}

type get_restart_status_arguments struct {
	RestartToken string `json:"restart_token"`
}

func (s *ToolSet) get_config(ctx context.Context) (map[string]any, error) {
	raw_config, err := s.api_client.get_config(ctx)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(raw_json_value(raw_config))
}

func (s *ToolSet) update_config(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments update_config_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	if len(arguments.Values) == 0 {
		return nil, fmt.Errorf("values 至少需要包含一个配置项")
	}
	raw_result, err := s.api_client.update_config(ctx, arguments.Values)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(raw_json_value(raw_result))
}

func (s *ToolSet) get_restart_status(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments get_restart_status_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	arguments.RestartToken = strings.TrimSpace(arguments.RestartToken)
	if arguments.RestartToken == "" {
		return nil, fmt.Errorf("restart_token 不能为空")
	}
	raw_result, err := s.api_client.get_restart_status(ctx, arguments.RestartToken)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(raw_json_value(raw_result))
}

// tools_app declares the application-level tools whose handlers live in this
// file. The row order here is filesystem-local only; the published order is
// fixed by the concatenation in tool_declarations (registry.go).
var tools_app = []tool{
	{
		name:         "get_config",
		title:        `获取应用配置`,
		description:  `获取 config.yaml 的可配置字段、类型、说明、当前值和解析后的生效值。application_fields 是 internal/config/config.go 注册的应用配置，plugin_fields 是 adapter 插件配置，fields 是两者的兼容合集。敏感字段只返回是否已配置，不返回明文。修改前应先调用此工具确认字段名称和允许值。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":true}`),
		supports:     supports_api_client,
		handle:       without_arguments((*ToolSet).get_config),
	},
	{
		name:         "update_config",
		title:        `修改应用配置并重启`,
		description:  `批量修改 get_config 返回的非只读配置字段。配置保存成功后只代表重启已安排，不代表重启已完成；必须保存返回的 restart_token，在连接恢复后调用 get_restart_status，且仅当 status=completed、restart_completed=true、config_applied=true 时才能告诉用户重启完成且配置生效。当前 MCP 连接可能短暂断开；相同值不会触发重启。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"values":{"additionalProperties":true,"description":"配置键到新值的映射；键必须来自 get_config，值必须符合对应字段类型。","minProperties":1,"type":"object"}},"required":["values"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":true,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":false}`),
		supports:     supports_api_client,
		handle:       (*ToolSet).update_config,
	},
	{
		name:         "get_restart_status",
		title:        `确认应用重启与配置生效`,
		description:  `使用 update_config 返回的 restart_token 确认重启结果。status=pending 表示仍是旧进程，必须在连接恢复后重试；status=failed 表示重启请求失败；status=config_mismatch 表示已重启但配置不一致；只有 status=completed、restart_completed=true、config_applied=true 才能向用户确认重启完成且新配置已生效。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"restart_token":{"description":"update_config 返回的重启确认令牌。","type":"string"}},"required":["restart_token"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":true}`),
		supports:     supports_api_client,
		handle:       (*ToolSet).get_restart_status,
	},
}
