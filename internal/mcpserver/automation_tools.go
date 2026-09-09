package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AutomationScheduleSummary is the compact schedule view returned by listings.
type AutomationScheduleSummary struct {
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

func automation_tool_definitions() []any {
	return []any{
		map[string]any{
			"name":        list_automation_schedules_tool_name,
			"title":       "列出定时任务",
			"description": "列出已保存的工作流定时任务，包含 cron 表达式、启用状态、下次执行时间和最近一次执行结果。",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
			},
			"annotations": map[string]any{
				"readOnlyHint":    true,
				"destructiveHint": false,
				"idempotentHint":  true,
				"openWorldHint":   false,
			},
		},
		map[string]any{
			"name":        get_automation_schedule_tool_name,
			"title":       "获取定时任务",
			"description": "按 ID 查询单个定时任务的完整配置，包含初始数据和超时时间。",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"id": map[string]any{
						"type":        "string",
						"description": "list_automation_schedules 返回的定时任务 ID。",
					},
				},
				"required": []string{"id"},
			},
			"annotations": map[string]any{
				"readOnlyHint":    true,
				"destructiveHint": false,
				"idempotentHint":  true,
				"openWorldHint":   false,
			},
		},
		map[string]any{
			"name":        create_automation_schedule_tool_name,
			"title":       "创建定时任务",
			"description": "创建一个按 cron 表达式周期性执行工作流的定时任务。cron_expr 支持标准 5 字段表达式，也支持 @every 1m、@daily 等简写。创建前应先获得用户确认。",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "定时任务名称。",
					},
					"description": map[string]any{
						"type":        "string",
						"description": "定时任务说明。",
					},
					"cron_expr": map[string]any{
						"type":        "string",
						"description": "cron 表达式，例如 \"0 9 * * *\" 表示每天 9 点，\"@every 30m\" 表示每 30 分钟。",
					},
					"flow_id": map[string]any{
						"type":        "string",
						"description": "要执行的工作流 ID，必须已在流程引擎中注册。",
					},
					"initial_data": map[string]any{
						"type":                 "object",
						"description":          "传给工作流的初始上下文数据。",
						"additionalProperties": true,
					},
					"enabled": map[string]any{
						"type":        "boolean",
						"description": "创建后是否立即启用，默认 true。",
					},
					"timeout_sec": map[string]any{
						"type":        "integer",
						"description": "单次执行的超时秒数，默认 3600。",
					},
				},
				"required": []string{"name", "cron_expr", "flow_id"},
			},
			"annotations": map[string]any{
				"readOnlyHint":    false,
				"destructiveHint": false,
				"idempotentHint":  false,
				"openWorldHint":   false,
			},
		},
		map[string]any{
			"name":        toggle_automation_schedule_tool_name,
			"title":       "启停定时任务",
			"description": "切换定时任务的启用状态。停用后不再按 cron 计划触发，任务本身和它的历史记录都会保留。",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"id": map[string]any{
						"type":        "string",
						"description": "要切换状态的定时任务 ID。",
					},
				},
				"required": []string{"id"},
			},
			"annotations": map[string]any{
				"readOnlyHint":    false,
				"destructiveHint": false,
				"idempotentHint":  false,
				"openWorldHint":   false,
			},
		},
		map[string]any{
			"name":        trigger_automation_schedule_tool_name,
			"title":       "立即执行定时任务",
			"description": "不等待 cron 计划，立即按定时任务的配置执行一次工作流，并返回本次运行记录。执行前应先获得用户确认。",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"id": map[string]any{
						"type":        "string",
						"description": "要立即执行的定时任务 ID。",
					},
				},
				"required": []string{"id"},
			},
			"annotations": map[string]any{
				"readOnlyHint":    false,
				"destructiveHint": false,
				"idempotentHint":  false,
				"openWorldHint":   false,
			},
		},
		map[string]any{
			"name":        list_automation_runs_tool_name,
			"title":       "列出定时任务执行记录",
			"description": "列出定时任务的执行历史，可按 schedule_id 过滤。用于确认一次执行是否成功以及失败原因。",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"schedule_id": map[string]any{
						"type":        "string",
						"description": "只返回该定时任务的执行记录；留空返回全部。",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "返回条数上限，默认 20。",
					},
				},
			},
			"annotations": map[string]any{
				"readOnlyHint":    true,
				"destructiveHint": false,
				"idempotentHint":  true,
				"openWorldHint":   false,
			},
		},
	}
}

func (s *Server) list_automation_schedules_tool(ctx context.Context) (map[string]any, error) {
	schedules, err := s.automation.ListSchedules(ctx)
	if err != nil {
		return nil, err
	}
	return successful_tool_result(map[string]any{"schedules": schedules})
}

func (s *Server) get_automation_schedule_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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

func (s *Server) create_automation_schedule_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
	var arguments create_automation_schedule_arguments
	if err := decode_tool_arguments(raw_arguments, &arguments); err != nil {
		return nil, err
	}
	schedule, err := s.automation.CreateSchedule(ctx, AutomationCreateScheduleInput{
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

func (s *Server) toggle_automation_schedule_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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

func (s *Server) trigger_automation_schedule_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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

func (s *Server) list_automation_runs_tool(ctx context.Context, raw_arguments json.RawMessage) (map[string]any, error) {
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
