package nodes

import (
	"errors"
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

func (n *WorkflowNode) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	wf, ok := n.Config["workflow"].(engine.FlowDefinition)
	if !ok {
		return false, nil, errors.New("missing workflow")
	}

	// Register/ensure sub-flow exists in engine
	ctx.EngineRef.FlowDefinitions[wf.ID] = wf

	// Use the parent context's Data so sub-flow can read/write shared variables
	subCtx := &engine.ProcessContext{
		InstanceID: ctx.InstanceID + ":sub:" + n.Id,
		FlowID:     wf.ID,
		Data:       ctx.Data,
		NodeStates: map[string]engine.NodeState{},
		EngineRef:  ctx.EngineRef,
	}

	// Simple inline driver for the sub-workflow
	queue := []string{wf.StartNodeID}
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]

		nodeDef, exists := wf.Nodes[nodeID]
		if !exists {
			return false, nil, fmt.Errorf("subflow node not found: %s", nodeID)
		}

		// Build node implementation using engine registry
		if nodeDef.Config == nil {
			nodeDef.Config = map[string]interface{}{}
		}
		nodeDef.Config["id"] = nodeDef.ID
		if len(nodeDef.NextNodes) > 0 {
			nodeDef.Config["next_nodes"] = nodeDef.NextNodes
		}
		if len(nodeDef.NextNodeIDs) > 0 {
			nodeDef.Config["next_node_ids"] = nodeDef.NextNodeIDs
		}

		constructor := ctx.EngineRef.NodeRegistry[nodeDef.Type]
		if constructor == nil {
			return false, nil, fmt.Errorf("node type not registered: %s", nodeDef.Type)
		}
		impl := constructor(nodeDef.Config)

		ok, nextIDs, err := impl.Execute(subCtx)
		if err != nil || !ok {
			if err == nil {
				err = errors.New("subflow node execution failed")
			}
			return false, nil, err
		}

		if len(nextIDs) > 0 {
			queue = append(queue, nextIDs...)
		}
	}

	// Continue with parent flow
	parentNext := ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.Id)
	return true, parentNext, nil
}
