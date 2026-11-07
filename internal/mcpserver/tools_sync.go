package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func (s *ToolSet) get_sync_servers(ctx context.Context) (map[string]any, error) {
	raw, err := s.api_client.client.SyncServers(ctx)
	if err != nil {
		return nil, map_api_error(err)
	}
	return successful_tool_result(raw_json_value(raw))
}

func (s *ToolSet) preview_sync(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments struct {
		TaskIDs []int `json:"task_ids"`
	}
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	if err := validate_sync_task_ids(arguments.TaskIDs); err != nil {
		return nil, err
	}
	raw, err := s.api_client.client.SyncPreview(ctx, arguments.TaskIDs...)
	if err != nil {
		return nil, map_api_error(err)
	}
	return successful_tool_result(raw_json_value(raw))
}

func (s *ToolSet) configure_sync_server(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	if strings.TrimSpace(arguments.Name) == "" || strings.TrimSpace(arguments.URL) == "" {
		return nil, fmt.Errorf("name、url 均不能为空")
	}
	raw, err := s.api_client.client.ConfigureSyncServer(ctx, arguments.Name, arguments.URL)
	if err != nil {
		return nil, map_api_error(err)
	}
	return successful_tool_result(raw_json_value(raw))
}

func (s *ToolSet) sync_to_server(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments struct {
		ServerID *int  `json:"server_id"`
		TaskIDs  []int `json:"task_ids"`
		All      bool  `json:"all"`
	}
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	if arguments.ServerID == nil || *arguments.ServerID < 0 {
		return nil, fmt.Errorf("server_id 必须是 get_sync_servers 返回的非负整数")
	}
	if len(arguments.TaskIDs) == 0 && !arguments.All {
		return nil, fmt.Errorf("请指定 task_ids；只有全量同步时才设置 all=true")
	}
	if len(arguments.TaskIDs) > 0 && arguments.All {
		return nil, fmt.Errorf("task_ids 和 all=true 不能同时指定")
	}
	if err := validate_sync_task_ids(arguments.TaskIDs); err != nil {
		return nil, err
	}
	raw, err := s.api_client.client.StartSync(ctx, *arguments.ServerID, arguments.TaskIDs...)
	if err != nil {
		return nil, map_api_error(err)
	}
	return successful_tool_result(raw_json_value(raw))
}

func validate_sync_task_ids(ids []int) error {
	if len(ids) > 1000 {
		return fmt.Errorf("一次最多选择 1000 个任务")
	}
	for _, id := range ids {
		if id <= 0 {
			return fmt.Errorf("task_ids 必须是正整数")
		}
	}
	return nil
}

func (s *ToolSet) get_sync_status(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments struct {
		JobID string `json:"job_id"`
	}
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	if strings.TrimSpace(arguments.JobID) == "" {
		return nil, fmt.Errorf("job_id 不能为空")
	}
	raw, err := s.api_client.client.SyncStatus(ctx, arguments.JobID)
	if err != nil {
		return nil, map_api_error(err)
	}
	return successful_tool_result(raw_json_value(raw))
}

var tools_sync = []tool{
	{
		name: "get_sync_skill", title: "读取下载同步 Skill", description: "读取 MCP Server 内置的 sync-downloads SKILL.md。同步已有下载、单条测试或指定任务前，先读取此流程说明；也可通过 resources/read 读取 dm://skills/sync-downloads。",
		input_schema: json.RawMessage(`{"type":"object","additionalProperties":false}`),
		annotations:  json.RawMessage(`{"readOnlyHint":true,"destructiveHint":false,"idempotentHint":true,"openWorldHint":false}`),
		supports:     supports_api_client, handle: without_arguments((*ToolSet).get_sync_skill),
	},
	{
		name: "get_sync_servers", title: "获取数据同步目标", description: "列出已配置的同步目标及 server_id。",
		input_schema: json.RawMessage(`{"type":"object","additionalProperties":false}`),
		annotations:  json.RawMessage(`{"readOnlyHint":true,"destructiveHint":false,"idempotentHint":true,"openWorldHint":false}`),
		supports:     supports_api_client, handle: without_arguments((*ToolSet).get_sync_servers),
	},
	{
		name: "preview_sync", title: "预览下载任务同步", description: "只读统计将重建的下载任务、资源、已有文件数量和字节数，以及任务关联的内容和账号。不传输数据；不包含浏览记录或无关账号、内容，缺失文件和 .part 不计入文件数。",
		input_schema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"task_ids":{"type":"array","items":{"type":"integer","minimum":1},"minItems":1,"maxItems":1000,"uniqueItems":true,"description":"只预览这些任务；省略时预览全部"}}}`),
		annotations:  json.RawMessage(`{"readOnlyHint":true,"destructiveHint":false,"idempotentHint":true,"openWorldHint":false}`),
		supports:     supports_api_client, handle: (*ToolSet).preview_sync,
	},
	{
		name: "configure_sync_server", title: "配置数据同步目标", description: "只需名称和地址，添加或按名称更新同步目标，立即生效，不启动同步；无需接收令牌。",
		input_schema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["name","url"],"properties":{"name":{"type":"string","minLength":1},"url":{"type":"string","minLength":1}}}`),
		annotations:  json.RawMessage(`{"readOnlyHint":false,"destructiveHint":false,"idempotentHint":true,"openWorldHint":false}`),
		supports:     supports_api_client, handle: (*ToolSet).configure_sync_server,
	},
	{
		name: "sync_to_server", title: "重建下载记录并上传已有文件", description: "经用户授权后，在目标服务复用创建下载任务的保存流程，重建本机下载任务及其关联内容、账号和资源，上传已有文件；不调用平台解析，不启动下载。目标分配自增 ID 并重建关联，不要求两端 ID 相同。完整文件上传成功后保留完成状态；缺失或未完成的资源设为暂停。不包含浏览记录或无关账号、内容。后台执行返回 job_id；必须调用 get_sync_status，只有 status=completed 才表示成功。两端均需支持同步协议 v2。",
		input_schema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["server_id"],"properties":{"server_id":{"type":"integer","minimum":0,"description":"get_sync_servers 返回的目标 ID"},"task_ids":{"type":"array","items":{"type":"integer","minimum":1},"minItems":1,"maxItems":1000,"uniqueItems":true,"description":"仅同步这些任务；未选父任务不会自动加入"},"all":{"type":"boolean","description":"用户明确要求全量时设为 true，与 task_ids 二选一"}}}`),
		annotations:  json.RawMessage(`{"readOnlyHint":false,"destructiveHint":true,"idempotentHint":false,"openWorldHint":true}`),
		supports:     supports_api_client, handle: (*ToolSet).sync_to_server,
	},
	{
		name: "get_sync_status", title: "查询数据同步结果", description: "查询 sync_to_server 返回的 job_id。running 表示执行中，completed 表示目标已提交记录和文件，failed 表示失败。任务结果最多保留最近 20 条，重启后失效；状态丢失不能视为成功，应先核对目标数据。",
		input_schema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["job_id"],"properties":{"job_id":{"type":"string","minLength":1}}}`),
		annotations:  json.RawMessage(`{"readOnlyHint":true,"destructiveHint":false,"idempotentHint":true,"openWorldHint":false}`),
		supports:     supports_api_client, handle: (*ToolSet).get_sync_status,
	},
}
