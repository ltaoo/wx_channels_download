package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"wx_channel/pkg/dm"
)

// AutomationScheduleSummary is the compact schedule view returned by listings.
type AutomationScheduleSummary struct {
	InstanceID    string `json:"instance_id"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	CronExpr      string `json:"cron_expr"`
	FlowID        string `json:"flow_id"`
	Enabled       bool   `json:"enabled"`
	NextRunAt     *int64 `json:"next_run_at,omitempty"`
	LastRunID     string `json:"last_run_id,omitempty"`
	LastRunStatus string `json:"last_run_status,omitempty"`
}

// AutomationScheduleDetail adds the fields that only matter when inspecting a
// single schedule.
type AutomationScheduleDetail struct {
	AutomationScheduleSummary
	InitialData map[string]interface{} `json:"initial_data,omitempty"`
	TimeoutSec  int                    `json:"timeout_sec"`
	CreatedAt   int64                  `json:"created_at"`
	UpdatedAt   int64                  `json:"updated_at"`
}

// AutomationRunSummary is one execution attempt of a schedule.
type AutomationRunSummary struct {
	InstanceID  string `json:"instance_id"`
	ID          string `json:"id"`
	ScheduleID  string `json:"schedule_id,omitempty"`
	FlowID      string `json:"flow_id"`
	TriggerType string `json:"trigger_type,omitempty"`
	Status      string `json:"status"`
	CurrentNode string `json:"current_node,omitempty"`
	Error       string `json:"error,omitempty"`
	StartedAt   *int64 `json:"started_at,omitempty"`
	CompletedAt *int64 `json:"completed_at,omitempty"`
}

// AutomationCreateScheduleInput describes a new schedule created through MCP.
type AutomationCreateScheduleInput struct {
	InstanceID  string
	Name        string
	Description string
	CronExpr    string
	FlowID      string
	InitialData map[string]interface{}
	Enabled     *bool
	TimeoutSec  int
}

// AutomationBackend provides process-local access to scheduled workflows.
type AutomationBackend interface {
	ListSchedules(ctx context.Context) ([]AutomationScheduleSummary, error)
	GetSchedule(ctx context.Context, id string) (*AutomationScheduleDetail, error)
	CreateSchedule(ctx context.Context, input AutomationCreateScheduleInput) (*AutomationScheduleDetail, error)
	ToggleSchedule(ctx context.Context, id string) (*AutomationScheduleDetail, error)
	TriggerSchedule(ctx context.Context, id string) (*AutomationRunSummary, error)
	ListRuns(ctx context.Context, scheduleID string, limit int) ([]AutomationRunSummary, error)
}

const (
	list_automation_schedules_tool_name   = "list_automation_schedules"
	get_automation_schedule_tool_name     = "get_automation_schedule"
	create_automation_schedule_tool_name  = "create_automation_schedule"
	toggle_automation_schedule_tool_name  = "toggle_automation_schedule"
	trigger_automation_schedule_tool_name = "trigger_automation_schedule"
	list_automation_runs_tool_name        = "list_automation_runs"
)

type get_automation_schedule_arguments struct {
	ID string `json:"id"`
}

type create_automation_schedule_arguments struct {
	InstanceID  string                 `json:"instance_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	CronExpr    string                 `json:"cron_expr"`
	FlowID      string                 `json:"flow_id"`
	InitialData map[string]interface{} `json:"initial_data"`
	Enabled     *bool                  `json:"enabled"`
	TimeoutSec  int                    `json:"timeout_sec"`
}

type toggle_automation_schedule_arguments struct {
	ID string `json:"id"`
}

type trigger_automation_schedule_arguments struct {
	ID string `json:"id"`
}

type list_automation_runs_arguments struct {
	ScheduleID string `json:"schedule_id"`
	Limit      int    `json:"limit"`
}

func (s *ToolSet) create_automation_instance_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments struct {
		FlowID string `json:"flow_id"`
		Name   string `json:"name"`
	}
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	flow_id := strings.TrimSpace(arguments.FlowID)
	if flow_id == "" {
		return nil, fmt.Errorf("flow_id 不能为空")
	}
	name := strings.TrimSpace(arguments.Name)
	if arguments.Name != "" && name == "" {
		return nil, fmt.Errorf("name 不能为空白字符")
	}
	instance, err := s.api_client.client.CreateFlowInstance(ctx, flow_id, name)
	if err != nil {
		return nil, map_api_error(err)
	}
	return successful_tool_result(raw_json_value(instance))
}

func (s *ToolSet) list_automation_instances_tool(ctx context.Context) (map[string]any, error) {
	instances, err := s.api_client.client.FlowInstances(ctx)
	if err != nil {
		return nil, map_api_error(err)
	}
	return successful_tool_result(raw_json_value(instances))
}

func automation_instance_id_argument(raw json.RawMessage) (string, error) {
	var arguments struct {
		ID string `json:"id"`
	}
	if err := decode_tool_arguments(raw, &arguments); err != nil {
		return "", err
	}
	id := strings.TrimSpace(arguments.ID)
	if id == "" {
		return "", fmt.Errorf("id 不能为空，请提供实例 ID")
	}
	return id, nil
}

func (s *ToolSet) get_automation_instance_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	id, err := automation_instance_id_argument(raw_arguments)
	if err != nil {
		return nil, err
	}
	instance, err := s.api_client.client.FlowInstance(ctx, id)
	if err != nil {
		return nil, map_api_error(err)
	}
	return successful_tool_result(raw_json_value(instance))
}

func (s *ToolSet) update_automation_instance_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments dm.UpdateFlowInstanceRequest
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	arguments.ID = strings.TrimSpace(arguments.ID)
	if arguments.ID == "" {
		return nil, fmt.Errorf("id 不能为空，请提供实例 ID")
	}
	if arguments.Name != nil {
		*arguments.Name = strings.TrimSpace(*arguments.Name)
		if *arguments.Name == "" {
			return nil, fmt.Errorf("name 不能为空")
		}
	}
	if arguments.Name == nil && arguments.Description == nil && arguments.NodeConfigs == nil && arguments.Triggers == nil && arguments.InheritTriggers == nil && arguments.TriggerOverrides == nil {
		return nil, fmt.Errorf("请至少提供一个要修改的实例字段")
	}
	instance, err := s.api_client.client.UpdateFlowInstance(ctx, arguments)
	if err != nil {
		return nil, map_api_error(err)
	}
	return successful_tool_result(raw_json_value(instance))
}

func (s *ToolSet) delete_automation_instance_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	id, err := automation_instance_id_argument(raw_arguments)
	if err != nil {
		return nil, err
	}
	instance, err := s.api_client.client.DeleteFlowInstance(ctx, id)
	if err != nil {
		return nil, map_api_error(err)
	}
	return successful_tool_result(raw_json_value(instance))
}

func (s *ToolSet) list_automation_schedules_tool(ctx context.Context) (map[string]any, error) {
	schedules, err := s.automation.ListSchedules(ctx)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(map[string]any{"schedules": schedules})
}

func (s *ToolSet) get_automation_schedule_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments get_automation_schedule_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	if strings.TrimSpace(arguments.ID) == "" {
		return nil, fmt.Errorf("id 不能为空")
	}
	schedule, err := s.automation.GetSchedule(ctx, strings.TrimSpace(arguments.ID))
	if err != nil {
		return nil, err
	}
	return successful_tool_result(schedule)
}

func (s *ToolSet) create_automation_schedule_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments create_automation_schedule_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	schedule, err := s.automation.CreateSchedule(ctx, AutomationCreateScheduleInput{
		InstanceID:  arguments.InstanceID,
		Name:        arguments.Name,
		Description: arguments.Description,
		CronExpr:    arguments.CronExpr,
		FlowID:      arguments.FlowID,
		InitialData: arguments.InitialData,
		Enabled:     arguments.Enabled,
		TimeoutSec:  arguments.TimeoutSec,
	})
	if err != nil {
		return nil, err
	}
	return successful_tool_result(schedule)
}

func (s *ToolSet) toggle_automation_schedule_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	id, err := automation_schedule_id_argument(raw_arguments)
	if err != nil {
		return nil, err
	}
	schedule, err := s.automation.ToggleSchedule(ctx, id)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(schedule)
}

func (s *ToolSet) trigger_automation_schedule_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	id, err := automation_schedule_id_argument(raw_arguments)
	if err != nil {
		return nil, err
	}
	run, err := s.automation.TriggerSchedule(ctx, id)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(run)
}

func (s *ToolSet) list_automation_runs_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments list_automation_runs_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	limit := arguments.Limit
	if limit <= 0 {
		limit = 20
	}
	runs, err := s.automation.ListRuns(ctx, strings.TrimSpace(arguments.ScheduleID), limit)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(map[string]any{"runs": runs})
}

func automation_schedule_id_argument(raw_arguments json.RawMessage) (string, error) {
	var arguments get_automation_schedule_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return "", err
	}
	id := strings.TrimSpace(arguments.ID)
	if id == "" {
		return "", fmt.Errorf("id 不能为空")
	}
	return id, nil
}

// tools_automation declares the tools whose handlers live in this file. The row
// order here is filesystem-local only; the published order is fixed
// by the concatenation in tool_declarations (registry.go).
var tools_automation = []tool{
	{
		name: "list_automation_instances", title: "列出流程实例",
		description:  `列出已有自动化流程配置实例，返回 list，包含实例 id、flow_id、标题、说明、节点配置覆盖和默认配置；不重复返回共享流程图。修改或删除前用本工具定位实例。`,
		input_schema: json.RawMessage(`{"type":"object","additionalProperties":false}`),
		annotations:  json.RawMessage(`{"readOnlyHint":true,"destructiveHint":false,"idempotentHint":true,"openWorldHint":false}`),
		supports:     supports_api_client, handle: without_arguments((*ToolSet).list_automation_instances_tool),
	},
	{
		name: "get_automation_instance", title: "获取流程实例详情",
		description:  `按实例 id 读取当前配置及共享流程定义，返回 node_configs、default_node_configs、default_triggers、trigger_overrides、inherit_triggers 等字段。配置和流程定义可能是 JSON 字符串，编辑前先解析并保留未要求修改的覆盖值。`,
		input_schema: json.RawMessage(`{"type":"object","required":["id"],"properties":{"id":{"type":"string","minLength":1,"description":"实例 ID，非 flow_id。"}},"additionalProperties":false}`),
		annotations:  json.RawMessage(`{"readOnlyHint":true,"destructiveHint":false,"idempotentHint":true,"openWorldHint":false}`),
		supports:     supports_api_client, handle: (*ToolSet).get_automation_instance_tool,
	},
	{
		name:         "create_automation_instance",
		title:        `创建流程实例`,
		description:  `为已有自动化流程创建配置实例，返回实例 id 和 flow_id。flow_id 必须是流程定义 ID，不是实例 ID。实例共享流程图并继承默认配置和触发器；已启用的默认定时或事件触发器会按原规则生效，不会立即手动执行流程。name 省略或为空字符串时使用服务端默认名称。每次成功调用都会新增实例；超时或断线后应先核对实例列表，不要直接重放。`,
		input_schema: json.RawMessage(`{"type":"object","required":["flow_id"],"properties":{"flow_id":{"type":"string","minLength":1,"description":"已有自动化流程的定义 ID，非 instance_id。"},"name":{"type":"string","description":"实例名称；省略或为空字符串时使用流程名称加（实例）。"}},"additionalProperties":false}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":false,"openWorldHint":false,"readOnlyHint":false}`),
		supports:     supports_api_client,
		handle:       (*ToolSet).create_automation_instance_tool,
	},
	{
		name: "update_automation_instance", title: "编辑流程实例",
		description:  `修改实例标题 name、说明 description、节点配置覆盖及触发规则；先用 get_automation_instance 读取最新值。只提交要修改的字段，省略的字段保持原值。node_configs 是完整覆盖值集合，按节点 ID 到配置对象组织；{} 清空全部覆盖，单项修改须合并原有覆盖后提交。triggers 是完整的自定义触发器列表，[] 清空规则并取消继承。inherit_triggers=true 恢复默认触发器并清空触发器覆盖；false 不会停用已有规则。trigger_overrides 是按默认触发器 ID 组织的完整覆盖集合，{} 恢复默认；不能和 triggers 或 inherit_triggers 同时提交。触发规则修改立即生效。仅编辑实例，不修改共享流程图或流程默认配置。`,
		input_schema: json.RawMessage(`{"type":"object","required":["id"],"properties":{"id":{"type":"string","minLength":1,"description":"实例 ID，非 flow_id。"},"name":{"type":"string","minLength":1,"description":"实例标题。"},"description":{"type":"string","description":"实例说明，空字符串清空。"},"node_configs":{"type":"object","additionalProperties":{"type":"object","additionalProperties":true},"description":"完整节点配置覆盖集合，{} 清空覆盖。"},"inherit_triggers":{"type":"boolean","description":"true 恢复继承默认触发器；false 仅取消继承，不停用现有规则。"},"trigger_overrides":{"type":"object","additionalProperties":{"type":"object","additionalProperties":true},"description":"按默认触发器 ID 保存覆盖；不能与 triggers 或 inherit_triggers 同时提交。"},"triggers":{"type":"array","items":{"type":"object","additionalProperties":true},"description":"完整自定义触发器列表，[] 清空规则。字段遵循现有流程触发器格式。"}},"additionalProperties":false}`),
		annotations:  json.RawMessage(`{"readOnlyHint":false,"destructiveHint":true,"idempotentHint":false,"openWorldHint":false}`),
		supports:     supports_api_client, handle: (*ToolSet).update_automation_instance_tool,
	},
	{
		name: "delete_automation_instance", title: "删除流程实例",
		description:  `删除用户指定的单个实例及其触发规则，保留共享流程、默认配置和历史执行记录；不会中止已启动的执行。调用前使用 get_automation_instance 核对实例 id 和标题，仅在用户明确要求删除该实例时调用。`,
		input_schema: json.RawMessage(`{"type":"object","required":["id"],"properties":{"id":{"type":"string","minLength":1,"description":"要删除的实例 ID，非 flow_id。"}},"additionalProperties":false}`),
		annotations:  json.RawMessage(`{"readOnlyHint":false,"destructiveHint":true,"idempotentHint":false,"openWorldHint":false}`),
		supports:     supports_api_client, handle: (*ToolSet).delete_automation_instance_tool,
	},
	{
		name:         list_automation_schedules_tool_name,
		title:        `列出定时任务`,
		description:  `列出已保存的工作流定时任务，包含 cron 表达式、启用状态、下次执行时间和最近一次执行结果。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":true}`),
		supports:     supports_automation,
		handle:       without_arguments((*ToolSet).list_automation_schedules_tool),
	},
	{
		name:         get_automation_schedule_tool_name,
		title:        `获取定时任务`,
		description:  `按 ID 查询单个定时任务的完整配置，包含初始数据和超时时间。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"id":{"description":"list_automation_schedules 返回的定时任务 ID。","type":"string"}},"required":["id"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":true}`),
		supports:     supports_automation,
		handle:       (*ToolSet).get_automation_schedule_tool,
	},
	{
		name:         create_automation_schedule_tool_name,
		title:        `创建定时任务`,
		description:  `创建一个按 cron 表达式周期性执行工作流的定时任务。cron_expr 支持标准 5 字段表达式，也支持 @every 1m、@daily 等简写。创建前应先获得用户确认。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"cron_expr":{"description":"cron 表达式，例如 \"0 9 * * *\" 表示每天 9 点，\"@every 30m\" 表示每 30 分钟。","type":"string"},"description":{"description":"定时任务说明。","type":"string"},"enabled":{"description":"创建后是否立即启用，默认 true。","type":"boolean"},"instance_id":{"description":"要执行的自动化流程实例 ID。","type":"string"},"flow_id":{"description":"所属流程 ID，可省略；由实例确定。","type":"string"},"initial_data":{"additionalProperties":true,"description":"传给工作流的初始上下文数据。","type":"object"},"name":{"description":"定时任务名称。","type":"string"},"timeout_sec":{"description":"单次执行的超时秒数，默认 3600。","type":"integer"}},"required":["name","cron_expr","instance_id"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":false,"openWorldHint":false,"readOnlyHint":false}`),
		supports:     supports_automation,
		handle:       (*ToolSet).create_automation_schedule_tool,
	},
	{
		name:         toggle_automation_schedule_tool_name,
		title:        `启停定时任务`,
		description:  `切换定时任务的启用状态。停用后不再按 cron 计划触发，任务本身和它的历史记录都会保留。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"id":{"description":"要切换状态的定时任务 ID。","type":"string"}},"required":["id"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":false,"openWorldHint":false,"readOnlyHint":false}`),
		supports:     supports_automation,
		handle:       (*ToolSet).toggle_automation_schedule_tool,
	},
	{
		name:         trigger_automation_schedule_tool_name,
		title:        `立即执行定时任务`,
		description:  `不等待 cron 计划，立即按定时任务的配置执行一次工作流，并返回本次运行记录。执行前应先获得用户确认。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"id":{"description":"要立即执行的定时任务 ID。","type":"string"}},"required":["id"],"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":false,"openWorldHint":false,"readOnlyHint":false}`),
		supports:     supports_automation,
		handle:       (*ToolSet).trigger_automation_schedule_tool,
	},
	{
		name:         list_automation_runs_tool_name,
		title:        `列出定时任务执行记录`,
		description:  `列出定时任务的执行历史，可按 schedule_id 过滤。用于确认一次执行是否成功以及失败原因。`,
		input_schema: json.RawMessage(`{"additionalProperties":false,"properties":{"limit":{"description":"返回条数上限，默认 20。","type":"integer"},"schedule_id":{"description":"只返回该定时任务的执行记录；留空返回全部。","type":"string"}},"type":"object"}`),
		annotations:  json.RawMessage(`{"destructiveHint":false,"idempotentHint":true,"openWorldHint":false,"readOnlyHint":true}`),
		supports:     supports_automation,
		handle:       (*ToolSet).list_automation_runs_tool,
	},
}
