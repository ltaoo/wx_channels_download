package api

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
	"wx_channel/internal/hub"
	"wx_channel/internal/services"
)

type hub_wxchannels_submit_request = services.SubmitHubWXChannelsTaskRequest

type hub_download_submit_request = services.SubmitHubDownloadTaskRequest

type hub_task_view struct {
	hub.Task
	Hub string `json:"hub"`
}

type hub_named_status struct {
	hub.Status
	Name string `json:"name"`
}

type hub_status_response struct {
	hub.Status
	DefaultHub string             `json:"default_hub,omitempty"`
	Hubs       []hub_named_status `json:"hubs"`
}

func (c *APIClient) handle_hub_status(ctx *gin.Context) {
	if c.hub_config_error != nil {
		result.Err(ctx, 503, c.hub_config_error.Error())
		return
	}
	snapshot, err := c.hub_service.Status(ctx.Query("hub"))
	if err != nil {
		result.Err(ctx, hub_selection_error_code(ctx.Query("hub")), err.Error())
		return
	}
	statuses := make([]hub_named_status, 0, len(snapshot.Hubs))
	for _, named_status := range snapshot.Hubs {
		statuses = append(statuses, hub_named_status{
			Status: named_status.Status,
			Name:   named_status.Name,
		})
	}
	response := hub_status_response{
		Status:     snapshot.Status,
		DefaultHub: snapshot.DefaultHub,
		Hubs:       statuses,
	}
	result.Ok(ctx, response)
}

func (c *APIClient) handle_hub_wxchannels_submit(ctx *gin.Context) {
	var request hub_wxchannels_submit_request
	if err := ctx.ShouldBindJSON(&request); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	request.URL = strings.TrimSpace(request.URL)
	if request.URL == "" {
		result.Err(ctx, 400, "url 不能为空")
		return
	}
	if c.reject_hub_config_error(ctx, request.Hub) {
		return
	}
	hub_name, task, err := c.hub_service.SubmitWXChannelsTask(ctx.Request.Context(), request)
	if err != nil {
		result.Err(ctx, hub_operation_error_code(request.Hub, err), err.Error())
		return
	}
	result.Ok(ctx, hub_task_view{Task: *task, Hub: hub_name})
}

func (c *APIClient) handle_hub_download_submit(ctx *gin.Context) {
	var request hub_download_submit_request
	if err := ctx.ShouldBindJSON(&request); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	request.TargetClientID = strings.TrimSpace(request.TargetClientID)
	if request.TargetClientID == "" {
		result.Err(ctx, 400, "target_client_id 不能为空")
		return
	}
	if (request.Request == nil) == (request.URLRequest == nil) {
		result.Err(ctx, 400, "request 和 url_request 必须且只能提供一个")
		return
	}
	if c.reject_hub_config_error(ctx, request.Hub) {
		return
	}
	hub_name, task, err := c.hub_service.SubmitDownloadTask(ctx.Request.Context(), request)
	if err != nil {
		result.Err(ctx, hub_operation_error_code(request.Hub, err), err.Error())
		return
	}
	result.Ok(ctx, hub_task_view{Task: *task, Hub: hub_name})
}

func (c *APIClient) handle_hub_task_get(ctx *gin.Context) {
	requested_hub := ctx.Query("hub")
	if c.reject_hub_config_error(ctx, requested_hub) {
		return
	}
	hub_name, task, err := c.hub_service.GetTask(ctx.Request.Context(), requested_hub, ctx.Param("id"))
	if err != nil {
		result.Err(ctx, hub_operation_error_code(requested_hub, err), err.Error())
		return
	}
	result.Ok(ctx, hub_task_view{Task: *task, Hub: hub_name})
}

func (c *APIClient) handle_hub_task_list(ctx *gin.Context) {
	requested_hub := ctx.Query("hub")
	if c.reject_hub_config_error(ctx, requested_hub) {
		return
	}
	limit := 50
	if value := strings.TrimSpace(ctx.Query("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 || parsed > 200 {
			result.Err(ctx, 400, "limit 必须是 1 到 200 之间的整数")
			return
		}
		limit = parsed
	}
	hub_name, tasks, err := c.hub_service.ListTasks(ctx.Request.Context(), requested_hub, ctx.Query("status"), limit)
	if err != nil {
		result.Err(ctx, hub_operation_error_code(requested_hub, err), err.Error())
		return
	}
	task_views := make([]hub_task_view, 0, len(tasks))
	for _, task := range tasks {
		task_views = append(task_views, hub_task_view{Task: task, Hub: hub_name})
	}
	result.Ok(ctx, gin.H{"hub": hub_name, "tasks": task_views})
}

func hub_operation_error_code(requested_name string, err error) int {
	if services.IsHubSelectionError(err) {
		return hub_selection_error_code(requested_name)
	}
	return 502
}

func (c *APIClient) reject_hub_config_error(ctx *gin.Context, requested_name string) bool {
	if c == nil || c.hub_config_error == nil {
		return false
	}
	result.Err(ctx, hub_selection_error_code(requested_name), c.hub_config_error.Error())
	return true
}

func hub_selection_error_code(requested_name string) int {
	if strings.TrimSpace(requested_name) != "" {
		return 400
	}
	return 503
}
