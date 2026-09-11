package api

import (
	"strings"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
	"wx_channel/internal/services"
	"wx_channel/pkg/flowengine/engine"
)

type user_flow_create_body struct {
	Name          string               `json:"name"`
	Description   string               `json:"description"`
	TriggerType   string               `json:"trigger_type"`
	EventKey      string               `json:"event_key"`
	ContextSchema []engine.FieldSchema `json:"context_schema"`
}

type user_flow_update_body struct {
	Name          *string                      `json:"name"`
	Description   *string                      `json:"description"`
	ContextSchema []engine.FieldSchema         `json:"context_schema"`
	StartNodeID   string                       `json:"start_node_id"`
	Nodes         []services.UserFlowNodeInput `json:"nodes"`
}

func (c *APIClient) handle_list_user_flows(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	flows, err := service.ListUserFlows()
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"list": flows})
}

func (c *APIClient) handle_create_user_flow(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body user_flow_create_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	flow, err := service.CreateUserFlow(services.CreateUserFlowInput{
		Name:          body.Name,
		Description:   body.Description,
		TriggerType:   body.TriggerType,
		EventKey:      body.EventKey,
		ContextSchema: body.ContextSchema,
	})
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, flow)
}

func (c *APIClient) handle_get_user_flow(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	flow, err := service.GetUserFlow(ctx.Param("id"))
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, flow)
}

func (c *APIClient) handle_update_user_flow(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body user_flow_update_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	flow, err := service.UpdateUserFlow(ctx.Param("id"), services.UpdateUserFlowInput{
		Name:          body.Name,
		Description:   body.Description,
		ContextSchema: body.ContextSchema,
		StartNodeID:   body.StartNodeID,
		Nodes:         body.Nodes,
	})
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, flow)
}

func (c *APIClient) handle_delete_user_flow(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	if err := service.DeleteUserFlow(ctx.Param("id")); err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"id": ctx.Param("id")})
}

// handle_get_user_flow_graph renders a user pipeline in the visualization
// payload shape the automation page renders for built-in flows.
func (c *APIClient) handle_get_user_flow_graph(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	payload, err := service.UserFlowVisualization(ctx.Query("flow_id"))
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, payload)
}

func (c *APIClient) handle_get_user_flow_node_catalog(ctx *gin.Context) {
	if _, ok := c.automation_service_or_error(ctx); !ok {
		return
	}
	result.Ok(ctx, gin.H{"nodes": services.SortedUserFlowNodeCatalog()})
}

// handle_trigger_user_flow runs a user pipeline immediately without a schedule.
func (c *APIClient) handle_trigger_user_flow(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	flow_id := strings.TrimSpace(ctx.Param("id"))
	flow, err := service.GetUserFlow(flow_id)
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	// Reuse the schedule execution path by materializing an ephemeral manual
	// schedule bound to this flow.
	run, err := service.TriggerFlowDirect(flow.ID, flow.TriggerType, flow.EventKey)
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, run)
}
