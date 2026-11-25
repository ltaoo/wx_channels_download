package dm

import (
	"context"
	"encoding/json"
	"net/http"
)

// UpdateFlowInstanceRequest contains instance-owned fields. Pointers preserve
// the difference between omitted values and explicit {}, [] or false.
type UpdateFlowInstanceRequest struct {
	ID               string                     `json:"id"`
	Name             *string                    `json:"name,omitempty"`
	Description      *string                    `json:"description,omitempty"`
	NodeConfigs      *map[string]map[string]any `json:"node_configs,omitempty"`
	Triggers         *[]map[string]any          `json:"triggers,omitempty"`
	InheritTriggers  *bool                      `json:"inherit_triggers,omitempty"`
	TriggerOverrides *map[string]map[string]any `json:"trigger_overrides,omitempty"`
}

// CreateFlowInstance creates an instance inheriting the flow's defaults and triggers.
func (c *Client) CreateFlowInstance(ctx context.Context, flow_id string, name string) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/automation/create_flow_instance", nil, map[string]string{"flow_id": flow_id, "name": name})
}

func (c *Client) FlowInstances(ctx context.Context) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/automation/list_flow_instances", nil, nil)
}

func (c *Client) FlowInstance(ctx context.Context, id string) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/automation/get_flow_instance", nil, map[string]string{"id": id})
}

func (c *Client) UpdateFlowInstance(ctx context.Context, input UpdateFlowInstanceRequest) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/automation/update_flow_instance", nil, input)
}

func (c *Client) DeleteFlowInstance(ctx context.Context, id string) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, "/api/v1/automation/delete_flow_instance", nil, map[string]string{"id": id})
}
