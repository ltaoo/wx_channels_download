package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
	"wx_channel/internal/services"
)

type bridge_wxchannels_submit_request = services.SubmitBridgeWXChannelsTaskRequest

type bridge_download_submit_request = services.SubmitBridgeDownloadTaskRequest

type bridge_call_submit_request = services.SubmitBridgeCallRequest

func (c *APIClient) handle_bridge_status(ctx *gin.Context) {
	status, err := c.bridge_service.Status()
	if err != nil {
		result.Err(ctx, http.StatusServiceUnavailable, err.Error())
		return
	}
	result.Ok(ctx, status)
}

func (c *APIClient) handle_bridge_call_submit(ctx *gin.Context) {
	var request bridge_call_submit_request
	if err := ctx.ShouldBindJSON(&request); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	request.Method = strings.TrimSpace(request.Method)
	if request.Method == "" {
		result.Err(ctx, 400, "method 不能为空")
		return
	}
	task, err := c.bridge_service.Call(ctx.Request.Context(), request)
	if err != nil {
		result.Err(ctx, bridge_operation_error_code(err), err.Error())
		return
	}
	result.Ok(ctx, task)
}

func (c *APIClient) handle_bridge_wxchannels_submit(ctx *gin.Context) {
	var request bridge_wxchannels_submit_request
	if err := ctx.ShouldBindJSON(&request); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	request.URL = strings.TrimSpace(request.URL)
	if request.URL == "" {
		result.Err(ctx, 400, "url 不能为空")
		return
	}
	task, err := c.bridge_service.SubmitWXChannelsTask(ctx.Request.Context(), request)
	if err != nil {
		result.Err(ctx, bridge_operation_error_code(err), err.Error())
		return
	}
	result.Ok(ctx, task)
}

func (c *APIClient) handle_bridge_download_submit(ctx *gin.Context) {
	var request bridge_download_submit_request
	if err := ctx.ShouldBindJSON(&request); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	request.TargetDeviceID = strings.TrimSpace(request.TargetDeviceID)
	request.LegacyTargetClientID = strings.TrimSpace(request.LegacyTargetClientID)
	if request.TargetDeviceID == "" && request.LegacyTargetClientID == "" {
		result.Err(ctx, 400, "target_device_id 不能为空")
		return
	}
	if (request.Request == nil) == (request.URLRequest == nil) {
		result.Err(ctx, 400, "request 和 url_request 必须且只能提供一个")
		return
	}
	task, err := c.bridge_service.SubmitDownloadTask(ctx.Request.Context(), request)
	if err != nil {
		result.Err(ctx, bridge_operation_error_code(err), err.Error())
		return
	}
	result.Ok(ctx, task)
}

func (c *APIClient) handle_bridge_task_get(ctx *gin.Context) {
	task, err := c.bridge_service.GetTask(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		result.Err(ctx, bridge_operation_error_code(err), err.Error())
		return
	}
	result.Ok(ctx, task)
}

func (c *APIClient) handle_bridge_task_list(ctx *gin.Context) {
	limit := 50
	if value := strings.TrimSpace(ctx.Query("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 || parsed > 200 {
			result.Err(ctx, 400, "limit 必须是 1 到 200 之间的整数")
			return
		}
		limit = parsed
	}
	tasks, err := c.bridge_service.ListTasks(ctx.Request.Context(), ctx.Query("status"), limit)
	if err != nil {
		result.Err(ctx, bridge_operation_error_code(err), err.Error())
		return
	}
	result.Ok(ctx, gin.H{"tasks": tasks})
}

func bridge_operation_error_code(err error) int {
	if services.IsBridgeUnavailableError(err) {
		return http.StatusServiceUnavailable
	}
	return http.StatusBadGateway
}
