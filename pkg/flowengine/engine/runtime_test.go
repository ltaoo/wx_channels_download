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

func TestCheckpointDueCoalescesBoundaries(t *testing.T) {
	ctx := &ProcessContext{NodeStates: map[string]NodeState{"a": StateRunning}, CurrentNode: "a"}
	if !ctx.checkpoint_due() {
		t.Fatal("first boundary of a run must be written")
	}
	due := 0
	for i := 1; i < checkpoint_boundary_interval; i++ {
		if ctx.checkpoint_due() {
			due++
		}
	}
	if due != 0 {
		t.Fatalf("boundaries inside the interval must be coalesced, got %d writes", due)
	}
	if !ctx.checkpoint_due() {
		t.Fatalf("the %dth boundary must be written", checkpoint_boundary_interval)
	}

	// A human wait is the only checkpoint a restart resumes from: always write.
	ctx.NodeStates["a"] = StateWaitingForUser
	for i := 0; i < 3; i++ {
		if !ctx.checkpoint_due() {
			t.Fatal("a waiting node must always persist its checkpoint")
		}
	}

	// A slow step must not leave a fast node chain uncheckpointed forever.
	ctx.NodeStates["a"] = StateRunning
	ctx.checkpoint_boundaries_pending = 0
	ctx.checkpoint_written_at = time.Now().Add(-checkpoint_time_interval - time.Second)
	if !ctx.checkpoint_due() {
		t.Fatal("a boundary older than the time interval must be written")
	}
}

func TestSubflowResetEmitsOncePerRun(t *testing.T) {
	e := new_parallel_test_engine()
	resets := 0
	e.SetNodeExecutionStatusHandler(func(status NodeExecutionStatus) {
		if status.NodeID == "" && status.NodeStates != nil {
			resets++
		}
	})
	parent := &ProcessContext{InstanceID: "run-x", reset_flows: &reset_flow_set{}}
	body := FlowDefinition{ID: "body", Nodes: map[string]NodeDefinition{"a": {ID: "a", Type: "test"}}}
	for i := 0; i < 5; i++ {
		child := &ProcessContext{Definition: &body, FlowID: body.ID, InstanceID: "run-x"}
		e.reset_subflow_states(parent, child)
		if len(child.NodeStates) != 0 {
			t.Fatalf("iteration %d must still start sparse, got %v", i, child.NodeStates)
		}
	}
	if resets != 1 {
		t.Fatalf("expected one whole-scope reset for repeated loop items, got %d", resets)
	}
	other := FlowDefinition{ID: "other", Nodes: map[string]NodeDefinition{"b": {ID: "b", Type: "test"}}}
	e.reset_subflow_states(parent, &ProcessContext{Definition: &other, FlowID: other.ID, InstanceID: "run-x"})
	if resets != 2 {
		t.Fatalf("expected a reset when a newly entered graph runs, got %d", resets)
	}
}
