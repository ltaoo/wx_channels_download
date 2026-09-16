package engine

import "testing"

type start_node_test_node struct {
	id string
}

func (n *start_node_test_node) ID() string   { return n.id }
func (n *start_node_test_node) Type() string { return "start-node-test" }

func (n *start_node_test_node) Execute(ctx *ProcessContext) (bool, []string, error) {
	return true, nil, nil
}

func TestStartFlowWithOptionsUsesStartNodeID(t *testing.T) {
	flow_engine := &FlowEngine{}
	flow_engine.RegisterNode("start-node-test", func(config map[string]interface{}) Node {
		id, _ := config["id"].(string)
		return &start_node_test_node{id: id}
	})
	flow_engine.SetFlowDefinitions(map[string]FlowDefinition{
		"flow": {
			ID:          "flow",
			StartNodeID: "start",
			Nodes: map[string]NodeDefinition{
				"start":  {ID: "start", Type: "start-node-test", Config: map[string]interface{}{"id": "start"}},
				"middle": {ID: "middle", Type: "start-node-test", Config: map[string]interface{}{"id": "middle"}},
				"end":    {ID: "end", Type: "start-node-test", Config: map[string]interface{}{"id": "end"}},
			},
		},
	})

	instance_id, err := flow_engine.StartFlowWithOptions(
		"flow",
		map[string]interface{}{},
		StartFlowOptions{StartNodeID: "middle"},
	)
	if err != nil {
		t.Fatalf("StartFlowWithOptions failed: %v", err)
	}
	run, ok := flow_engine.GetRunSnapshot(instance_id)
	if !ok {
		t.Fatal("expected a run snapshot")
	}
	if run.Status != RunStatusCompleted {
		t.Fatalf("expected a completed run, got %s (%s)", run.Status, run.Error)
	}

	ctx, ok := flow_engine.ContextSnapshots[instance_id]
	if !ok {
		t.Fatal("expected a context snapshot")
	}
	if _, exists := ctx.NodeStates["start"]; exists {
		t.Fatalf("expected the default start node to remain unset, got %s", ctx.NodeStates["start"])
	}
	if ctx.NodeStates["middle"] != StateCompleted {
		t.Fatalf("expected the override start node to complete, got %s", ctx.NodeStates["middle"])
	}

	if _, err := flow_engine.StartFlowWithOptions(
		"flow",
		map[string]interface{}{},
		StartFlowOptions{StartNodeID: "missing"},
	); err == nil {
		t.Fatal("expected an invalid start node to be rejected")
	}
}
