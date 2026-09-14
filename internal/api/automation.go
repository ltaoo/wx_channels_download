package api

import (
	"errors"
	"io"
	"strings"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
	"wx_channel/internal/database/model"
	"wx_channel/internal/services"
)

type automation_schedule_create_body struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	CronExpr    string                 `json:"cron_expr"`
	FlowID      string                 `json:"flow_id"`
	InitialData map[string]interface{} `json:"initial_data"`
	Enabled     *bool                  `json:"enabled"`
	TimeoutSec  int                    `json:"timeout_sec"`
}

type automation_schedule_list_body struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	FlowID   string `json:"flow_id"`
	Keyword  string `json:"keyword"`
	Enabled  *bool  `json:"enabled"`
}

type automation_id_body struct {
	ID string `json:"id"`
}

type automation_schedule_trigger_body struct {
	ID          string `json:"id"`
	TriggerType string `json:"trigger_type"`
	EventKey    string `json:"event_key"`
}

type automation_schedule_update_body struct {
	ID          string                 `json:"id"`
	Name        *string                `json:"name"`
	Description *string                `json:"description"`
	CronExpr    *string                `json:"cron_expr"`
	FlowID      *string                `json:"flow_id"`
	InitialData map[string]interface{} `json:"initial_data"`
	Enabled     *bool                  `json:"enabled"`
	TimeoutSec  *int                   `json:"timeout_sec"`
}

type automation_run_list_body struct {
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	ScheduleID string `json:"schedule_id"`
	FlowID     string `json:"flow_id"`
	Status     string `json:"status"`
}

// automation_list_payload mirrors the paging envelope used by the other v1 list
// endpoints so clients can reuse the same parsing.
func automation_list_payload(list interface{}, total int64, page int, page_size int) gin.H {
	return gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": page_size,
	}
}

func (c *APIClient) automation_service_or_error(ctx *gin.Context) (*services.AutomationService, bool) {
	if c == nil || c.automation_service == nil {
		result.Err(ctx, api_code_invalid_params, "自动化服务未初始化")
		return nil, false
	}
	return c.automation_service, true
}

func (c *APIClient) handle_list_automation_schedules(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body automation_schedule_list_body
	if err := ctx.ShouldBindJSON(&body); err != nil && !errors.Is(err, io.EOF) {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	input := services.ListSchedulesInput{
		Page:     body.Page,
		PageSize: body.PageSize,
		FlowID:   strings.TrimSpace(body.FlowID),
		Keyword:  strings.TrimSpace(body.Keyword),
		Enabled:  body.Enabled,
	}
	schedules, total, err := service.ListSchedules(input)
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, automation_list_payload(schedules, total, input.Page, input.PageSize))
}

func (c *APIClient) handle_create_automation_schedule(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body automation_schedule_create_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	schedule, err := service.CreateSchedule(services.CreateScheduleInput{
		Name:        body.Name,
		Description: body.Description,
		CronExpr:    body.CronExpr,
		FlowID:      body.FlowID,
		InitialData: body.InitialData,
		Enabled:     body.Enabled,
		TimeoutSec:  body.TimeoutSec,
	})
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, schedule)
}

func (c *APIClient) handle_get_automation_schedule(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body automation_id_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	schedule, err := service.GetSchedule(strings.TrimSpace(body.ID))
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, schedule)
}

func (c *APIClient) handle_update_automation_schedule(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body automation_schedule_update_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	schedule, err := service.UpdateSchedule(strings.TrimSpace(body.ID), services.UpdateScheduleInput{
		Name:        body.Name,
		Description: body.Description,
		CronExpr:    body.CronExpr,
		FlowID:      body.FlowID,
		InitialData: body.InitialData,
		Enabled:     body.Enabled,
		TimeoutSec:  body.TimeoutSec,
	})
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, schedule)
}

func (c *APIClient) handle_delete_automation_schedule(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body automation_id_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	id := strings.TrimSpace(body.ID)
	if err := service.DeleteSchedule(id); err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"id": id})
}

func (c *APIClient) handle_toggle_automation_schedule(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body automation_id_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	schedule, err := service.ToggleSchedule(strings.TrimSpace(body.ID))
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, schedule)
}

func (c *APIClient) handle_trigger_automation_schedule(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body automation_schedule_trigger_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	trigger_type := strings.TrimSpace(body.TriggerType)
	if trigger_type == "" {
		trigger_type = model.FlowRunTriggerManual
	}
	run, err := service.TriggerScheduleAs(strings.TrimSpace(body.ID), trigger_type, strings.TrimSpace(body.EventKey))
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, run)
}

func (c *APIClient) handle_list_automation_runs(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body automation_run_list_body
	if err := ctx.ShouldBindJSON(&body); err != nil && !errors.Is(err, io.EOF) {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	input := services.ListRunsInput{
		Page:       body.Page,
		PageSize:   body.PageSize,
		ScheduleID: strings.TrimSpace(body.ScheduleID),
		FlowID:     strings.TrimSpace(body.FlowID),
		Status:     strings.TrimSpace(body.Status),
	}
	runs, total, err := service.ListRuns(input)
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, automation_list_payload(runs, total, input.Page, input.PageSize))
}

func (c *APIClient) handle_get_automation_run(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body automation_id_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	run, err := service.GetRun(strings.TrimSpace(body.ID))
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, run)
}

func (c *APIClient) handle_cancel_automation_run(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body automation_id_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	id := strings.TrimSpace(body.ID)
	if err := service.CancelRun(id); err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"id": id, "cancelled": true})
}
