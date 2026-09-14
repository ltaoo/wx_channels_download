package api

import (
	"errors"
	"io"
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
	ID            string                       `json:"id"`
	Name          *string                      `json:"name"`
	Description   *string                      `json:"description"`
	ContextSchema []engine.FieldSchema         `json:"context_schema"`
	StartNodeID   string                       `json:"start_node_id"`
	Nodes         []services.UserFlowNodeInput `json:"nodes"`
}

type user_flow_id_body struct {
	ID string `json:"id"`
}

type user_flow_trigger_body struct {
	ID          string                 `json:"id"`
	InitialData map[string]interface{} `json:"initial_data"`
}

type user_flow_import_body struct {
	Definition string `json:"definition"`
}

type user_flow_graph_body struct {
	FlowID string `json:"flow_id"`
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
	var body user_flow_id_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	flow, err := service.GetUserFlow(strings.TrimSpace(body.ID))
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
	flow, err := service.UpdateUserFlow(strings.TrimSpace(body.ID), services.UpdateUserFlowInput{
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
	var body user_flow_id_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	id := strings.TrimSpace(body.ID)
	if err := service.DeleteUserFlow(id); err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, gin.H{"id": id})
}

// handle_get_user_flow_graph renders a user pipeline in the visualization
// payload shape the automation page renders for built-in flows.
func (c *APIClient) handle_get_user_flow_graph(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body user_flow_graph_body
	if err := ctx.ShouldBindJSON(&body); err != nil && !errors.Is(err, io.EOF) {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	payload, err := service.UserFlowVisualization(strings.TrimSpace(body.FlowID))
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
	var body user_flow_trigger_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	flow_id := strings.TrimSpace(body.ID)
	// This endpoint is an explicit manual/debug action. The persisted flow may
	// still be configured for Cron or Event execution; that configuration must
	// not restrict or change this one-off run.
	run, err := service.TriggerFlowDirect(flow_id, body.InitialData)
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, run)
}

// handle_import_user_flow creates a pipeline from a pasted flow definition JSON.
func (c *APIClient) handle_import_user_flow(ctx *gin.Context) {
	service, ok := c.automation_service_or_error(ctx)
	if !ok {
		return
	}
	var body user_flow_import_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, api_code_invalid_params, "请求参数无效")
		return
	}
	flow, err := service.ImportUserFlow(body.Definition)
	if err != nil {
		result.Err(ctx, api_code_invalid_params, err.Error())
		return
	}
	result.Ok(ctx, flow)
}
