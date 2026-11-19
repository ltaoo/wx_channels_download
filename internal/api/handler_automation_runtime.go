package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	result "wx_channel/internal/apiresult"
)

func (c *APIClient) handle_resume_automation_run(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body struct {
		ID     string                 `json:"id"`
		NodeID string                 `json:"node_id"`
		Input  map[string]interface{} `json:"input"`
	}
	if ctx.ShouldBindJSON(&body) != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	run, err := service.ResumeRun(strings.TrimSpace(body.ID), strings.TrimSpace(body.NodeID), body.Input)
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, run)
}

func (c *APIClient) handle_retry_automation_run(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body struct {
		ID                   string `json:"id"`
		UseCurrentDefinition bool   `json:"use_current_definition"`
	}
	if ctx.ShouldBindJSON(&body) != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	run, err := service.RetryRun(strings.TrimSpace(body.ID), body.UseCurrentDefinition)
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, run)
}

func (c *APIClient) handle_dispatch_automation_event(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body struct {
		EventKey string                 `json:"event_key"`
		Data     map[string]interface{} `json:"data"`
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 1024*1024)
	if ctx.ShouldBindJSON(&body) != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	runs, err := service.DispatchEvent(body.EventKey, body.Data)
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"list": runs})
}

func (c *APIClient) handle_automation_webhook(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 1024*1024)
	payload := map[string]interface{}{}
	decoder := json.NewDecoder(ctx.Request.Body)
	if err := decoder.Decode(&payload); err != nil || payload == nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "请求体必须为 JSON 对象"})
		return
	}
	var trailing interface{}
	if decoder.Decode(&trailing) != io.EOF {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "请求体必须仅包含一个 JSON 对象"})
		return
	}
	if !strings.HasPrefix(ctx.GetHeader("Authorization"), "Bearer ") {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "需要 Bearer 令牌"})
		return
	}
	token := strings.TrimPrefix(ctx.GetHeader("Authorization"), "Bearer ")
	run, err := service.TriggerWebhook(ctx.Param("id"), token, payload)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "令牌") {
			status = http.StatusUnauthorized
		}
		ctx.JSON(status, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusAccepted, gin.H{"data": run})
}
