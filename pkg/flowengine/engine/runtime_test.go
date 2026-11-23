package engine

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type parallel_test_node struct{ node_id string }

func (n *parallel_test_node) ID() string   { return n.node_id }
func (n *parallel_test_node) Type() string { return "test" }
func (n *parallel_test_node) Execute(ctx *ProcessContext) (bool, []string, error) {
	ctx.SetOutput(n.node_id, n.node_id, float64(ctx.NodeAttempts[n.node_id]))
	return true, ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.node_id), nil
}

func new_parallel_test_engine() *FlowEngine {
	e := &FlowEngine{}
	e.ensureMaps()
	e.RegisterNode("test", func(config map[string]interface{}) Node {
		return &parallel_test_node{node_id: config["id"].(string)}
	})
	return e
}

func TestParallelMergesOnlyBranchChanges(t *testing.T) {
	e := new_parallel_test_engine()
	definition := FlowDefinition{ID: "parallel", Nodes: map[string]NodeDefinition{
		"a": {ID: "a", Type: "test"}, "b": {ID: "b", Type: "test"},
	}}
	parent := &ProcessContext{
		Context: context.Background(), Definition: &definition, EngineRef: e,
		FlowID: definition.ID, InstanceID: "parallel-test", step_count: &atomic.Int64{},
		Data:        map[string]interface{}{"a": float64(0), "b": float64(0)},
		NodeOutputs: map[string]map[string]interface{}{"a": {"a": float64(0)}, "b": {"b": float64(0)}},
		NodeStates:  map[string]NodeState{"a": StateFailed, "b": StateFailed}, NodeAttempts: map[string]int{},
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err := e.run_parallel(parent, NodeDefinition{}, []string{"a", "b"}); err != nil {
			t.Fatal(err)
		}
		for _, node_id := range []string{"a", "b"} {
			if parent.Data[node_id] != float64(attempt) || parent.NodeOutputs[node_id][node_id] != float64(attempt) || parent.NodeStates[node_id] != StateCompleted {
				t.Fatalf("attempt %d lost branch %s: data=%v outputs=%v states=%v", attempt, node_id, parent.Data, parent.NodeOutputs, parent.NodeStates)
			}
		}
	}
}

func TestLightweightNodesDoNotWait(t *testing.T) {
	e := new_parallel_test_engine()
	definition := FlowDefinition{ID: "fast", StartNodeID: "a", Nodes: map[string]NodeDefinition{
		"a": {ID: "a", Type: "test", NextNodeIDs: []string{"b"}}, "b": {ID: "b", Type: "test"},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := e.StartFlowWithOptions(definition.ID, nil, StartFlowOptions{Context: ctx, Definition: &definition}); err != nil {
		t.Fatalf("two lightweight nodes exceeded the run deadline: %v", err)
	}
}
