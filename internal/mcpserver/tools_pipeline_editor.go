package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func (s *ToolSet) get_pipeline_editor_tool(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args struct {
		EditorID string `json:"editor_id"`
		FlowID   string `json:"flow_id"`
	}
	if err := decode_tool_arguments(raw, &args); err != nil {
		return nil, err
	}
	value, err := s.api_client.client.PipelineEditor(ctx, strings.TrimSpace(args.EditorID), strings.TrimSpace(args.FlowID))
	if err != nil {
		return nil, map_api_error(err)
	}
	return successful_tool_result(raw_json_value(value))
}

func (s *ToolSet) update_pipeline_editor_tool(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var args struct {
		EditorID string          `json:"editor_id"`
		Revision int64           `json:"revision"`
		Draft    json.RawMessage `json:"draft"`
	}
	if err := decode_tool_arguments(raw, &args); err != nil {
		return nil, err
	}
	if strings.TrimSpace(args.EditorID) == "" || args.Revision < 1 || !has_editor_draft(args.Draft) {
		return nil, fmt.Errorf("必须先获取编辑器，再提供 editor_id、revision 和完整 draft")
	}
	value, err := s.api_client.client.UpdatePipelineEditor(ctx, args.EditorID, args.Revision, args.Draft)
	if err != nil {
		return nil, map_api_error(err)
	}
	return successful_tool_result(raw_json_value(value))
}

func has_editor_draft(raw json.RawMessage) bool {
	var value map[string]json.RawMessage
	return json.Unmarshal(raw, &value) == nil && len(value) > 0
}

var tools_pipeline_editor = []tool{
	{
		name: "get_pipeline_editor", title: "获取当前编辑中的 Pipeline",
		description:  `读取已打开编辑页面的实时草稿，包含未保存节点、连线、触发器、选中节点、尚未应用的配置文本和版本号，并返回真实节点/服务目录。可按 flow_id 过滤；多个页面时返回 editors 候选，需指定 editor_id 后重新读取。修改前必须先调用本工具，不能用数据库记录替代页面草稿。`,
		input_schema: json.RawMessage(`{"type":"object","properties":{"editor_id":{"type":"string","description":"候选编辑页面 ID；单页面时可省略"},"flow_id":{"type":"string","description":"只查找此 Pipeline 的编辑页面"}},"additionalProperties":false}`),
		annotations:  json.RawMessage(`{"readOnlyHint":true,"destructiveHint":false,"idempotentHint":true,"openWorldHint":false}`),
		supports:     supports_api_client, handle: (*ToolSet).get_pipeline_editor_tool,
	},
	{
		name: "update_pipeline_editor", title: "实时修改 Pipeline 画布",
		description:  `修改 get_pipeline_editor 返回的指定编辑页面草稿，等页面应用后返回实际 snapshot。必须携带读取到的 revision；手动修改后旧版本会被拒绝，需要重新读取并合并。draft.nodes 为完整节点列表，保留未要求修改的节点、ID、配置、input_schema 和位置；连线用 next_ids，条件分支须同步 config.rules 的 target_id。context_schema 使用读取到的输入定义。triggers 省略/null 保留当前触发器，[] 清空。只更新画布并标记未保存，不保存、不运行；尚未应用的配置文本需要用户先处理。超时或断线不表示修改成功，也不能直接重放，先重新读取核对。`,
		input_schema: json.RawMessage(`{"type":"object","required":["editor_id","revision","draft"],"properties":{"editor_id":{"type":"string"},"revision":{"type":"integer","minimum":1},"draft":{"type":"object","required":["start_node_id","nodes"],"properties":{"start_node_id":{"type":"string"},"context_schema":{"type":"array","items":{"type":"object","additionalProperties":true}},"nodes":{"type":"array","minItems":1,"maxItems":500,"items":{"type":"object","required":["id","type","config","next_ids"],"properties":{"id":{"type":"string"},"type":{"type":"string"},"name":{"type":"string"},"config":{"type":"object","additionalProperties":true},"position":{"type":["object","null"],"properties":{"x":{"type":"number"},"y":{"type":"number"}}},"input_schema":{"type":"array","items":{"type":"object","additionalProperties":true}},"output_schema":{"type":"array","items":{"type":"object","additionalProperties":true}},"error_next_node":{"type":"string"},"retry":{"type":["object","null"],"properties":{"max_attempts":{"type":"integer","minimum":1,"maximum":20},"delay_ms":{"type":"integer","minimum":0},"backoff_multiplier":{"type":"number","minimum":0,"maximum":10}},"additionalProperties":false},"next_ids":{"type":"array","items":{"type":"string"}}},"additionalProperties":false}},"triggers":{"type":["array","null"],"items":{"type":"object","additionalProperties":true}}},"additionalProperties":false}},"additionalProperties":false}`),
		annotations:  json.RawMessage(`{"readOnlyHint":false,"destructiveHint":false,"idempotentHint":false,"openWorldHint":false}`),
		supports:     supports_api_client, handle: (*ToolSet).update_pipeline_editor_tool,
	},
}
