package api

import (
	"github.com/gin-gonic/gin"
	"strings"
	result "wx_channel/internal/apiresult"
	"wx_channel/internal/services"
)

type flow_instance_create_body struct {
	FlowID string `json:"flow_id"`
	Name   string `json:"name"`
}

func (c *APIClient) handle_create_user_flow_instance(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body flow_instance_create_body
	if ctx.ShouldBindJSON(&body) != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	name := strings.TrimSpace(body.Name)
	if body.Name != "" && name == "" {
		result.Err(ctx, api_code_invalid_params, "name 不能为空")
		return
	}
	instance, err := service.CreateUserFlowInstance(body.FlowID)
	if err == nil && body.Name != "" {
		instance, err = service.UpdateUserFlowInstance(instance.ID, services.UpdateUserFlowInput{Name: &name})
	}
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, instance)
}
func (c *APIClient) handle_list_user_flow_instances(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	list, err := service.ListUserFlowInstances()
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"list": list})
}
func (c *APIClient) handle_get_user_flow_instance(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body user_flow_id_body
	if ctx.ShouldBindJSON(&body) != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	instance, err := service.GetUserFlowInstance(body.ID)
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, instance)
}
func (c *APIClient) handle_update_user_flow_instance(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body user_flow_update_body
	if ctx.ShouldBindJSON(&body) != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	if body.DefaultNodeConfigs != nil {
		result.Err(ctx, api_code_invalid_params, "流程默认配置请使用 update_flow 修改")
		return
	}
	instance, err := service.UpdateUserFlowInstance(body.ID, services.UpdateUserFlowInput{Name: body.Name, Description: body.Description, NodeConfigs: body.NodeConfigs, Triggers: body.Triggers, InheritTriggers: body.InheritTriggers, TriggerOverrides: body.TriggerOverrides, Nodes: body.Nodes, StartNodeID: body.StartNodeID, Subflows: body.Subflows, ContextSchema: body.ContextSchema})
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, instance)
}
func (c *APIClient) handle_delete_user_flow_instance(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body user_flow_id_body
	if ctx.ShouldBindJSON(&body) != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	if err := service.DeleteUserFlowInstance(body.ID); err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"id": body.ID})
}
func (c *APIClient) handle_copy_user_flow_instance(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body user_flow_id_body
	if ctx.ShouldBindJSON(&body) != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	instance, err := service.CopyUserFlowInstance(body.ID)
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, instance)
}
