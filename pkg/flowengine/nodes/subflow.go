package nodes

import (
	"encoding/json"
	"fmt"
	"wx_channel/pkg/flowengine/engine"
)

type WorkflowNode struct {
	Id     string
	Config map[string]interface{}
}

func NewWorkflowNode(config map[string]interface{}) engine.Node {
	id, _ := config["id"].(string)
	return &WorkflowNode{Id: id, Config: config}
}
func (n *WorkflowNode) ID() string   { return n.Id }
func (n *WorkflowNode) Type() string { return "WorkflowNode" }

func configured_workflow(config map[string]interface{}, ctx *engine.ProcessContext) (engine.FlowDefinition, error) {
	if flow_id, _ := config["flow_id"].(string); flow_id != "" {
		definition, ok := ctx.EngineRef.GetFlowDefinition(flow_id)
		if !ok {
			return definition, fmt.Errorf("subflow not found: %s", flow_id)
		}
		return definition, nil
	}
	return decode_workflow(config["workflow"])
}

func decode_workflow(raw interface{}) (engine.FlowDefinition, error) {
	if definition, ok := raw.(engine.FlowDefinition); ok {
		return definition, nil
	}
	definition := engine.FlowDefinition{}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return definition, err
	}
	if err := json.Unmarshal(encoded, &definition); err != nil {
		return definition, err
	}
	if definition.StartNodeID == "" || len(definition.Nodes) == 0 {
		return definition, fmt.Errorf("workflow must contain start_node and nodes")
	}
	// Imported child definitions need the same normalized edge representation.
	for id, node := range definition.Nodes {
		node.ID = id
		if len(node.NextNodes) == 0 {
			for _, next := range node.NextNodeIDs {
				node.NextNodes = append(node.NextNodes, engine.TargetNode{TargetID: next})
			}
		}
		if len(node.NextNodeIDs) == 0 {
			for _, next := range node.NextNodes {
				node.NextNodeIDs = append(node.NextNodeIDs, next.TargetID)
			}
		}
		definition.Nodes[id] = node
	}
	return definition, nil
}

func (n *WorkflowNode) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	definition, err := configured_workflow(n.Config, ctx)
	if err != nil {
		return false, nil, err
	}
	output_key, _ := n.Config["output_key"].(string)
	_, has_arguments := n.Config["arguments"]
	_, has_flow_id := n.Config["flow_id"]
	if output_key == "" && !has_arguments && !has_flow_id {
		// Preserve the shared-data contract of built-in post-processing flows.
		if err := ctx.EngineRef.ExecuteSharedSubflow(ctx, definition); err != nil {
			return false, nil, err
		}
	} else {
		arguments, err := service_arguments(n.Config, ctx)
		if err != nil {
			return false, nil, err
		}
		result, err := ctx.EngineRef.ExecuteSubflow(ctx, definition, arguments)
		if err != nil {
			return false, nil, err
		}
		if output_key == "" {
			output_key = "workflow_result"
		}
		ctx.SetOutput(n.Id, output_key, result)
	}
	return true, ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.Id), nil
}
