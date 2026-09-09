package api

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
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

type automation_schedule_update_body struct {
	Name        *string                `json:"name"`
	Description *string                `json:"description"`
	CronExpr    *string                `json:"cron_expr"`
	FlowID      *string                `json:"flow_id"`
	InitialData map[string]interface{} `json:"initial_data"`
	Enabled     *bool                  `json:"enabled"`
	TimeoutSec  *int                   `json:"timeout_sec"`
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

// automation_query_page reads page/page_size query parameters. Invalid values
// fall back to the service defaults so a bad client request still returns data.
func automation_query_page(ctx *gin.Context) (int, int) {
	page := 0
	page_size := 0
	if value, err := strconv.Atoi(strings.TrimSpace(ctx.Query("page"))); err == nil {
		page = value
	}
	if value, err := strconv.Atoi(strings.TrimSpace(ctx.Query("page_size"))); err == nil {
		page_size = value
	}
	return page, page_size
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
	page, page_size := automation_query_page(ctx)
	input := services.ListSchedulesInput{
		Page:     page,
		PageSize: page_size,
		FlowID:   strings.TrimSpace(ctx.Query("flow_id")),
		Keyword:  strings.TrimSpace(ctx.Query("keyword")),
	}
	if enabled_value := strings.TrimSpace(ctx.Query("enabled")); enabled_value != "" {
		if enabled, err := strconv.ParseBool(enabled_value); err == nil {
			input.Enabled = &enabled
		} else {
			result.Err(ctx, api_code_invalid_params, "参数 enabled 必须是布尔值")
			return
		}
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
	schedule, err := service.GetSchedule(ctx.Param("id"))
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
	schedule, err := service.UpdateSchedule(ctx.Param("id"), services.UpdateScheduleInput{
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
	if err := service.DeleteSchedule(ctx.Param("id")); err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"id": ctx.Param("id")})
}

func (c *APIClient) handle_toggle_automation_schedule(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	schedule, err := service.ToggleSchedule(ctx.Param("id"))
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
	run, err := service.TriggerSchedule(ctx.Param("id"))
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
	page, page_size := automation_query_page(ctx)
	input := services.ListRunsInput{
		Page:       page,
		PageSize:   page_size,
		ScheduleID: strings.TrimSpace(ctx.Query("schedule_id")),
		FlowID:     strings.TrimSpace(ctx.Query("flow_id")),
		Status:     strings.TrimSpace(ctx.Query("status")),
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
	run, err := service.GetRun(ctx.Param("id"))
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
	if err := service.CancelRun(ctx.Param("id")); err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"id": ctx.Param("id"), "cancelled": true})
}
