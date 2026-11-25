package services

import (
	"encoding/json"
	"testing"

	"wx_channel/pkg/flowengine"
)

const automation_node_states_definition = `{
  "id": "flow-root",
  "nodes": {
    "start": {"config": {}},
    "sub": {"config": {"workflow": {
      "id": "flow-child",
      "nodes": {
        "child_start": {"config": {}},
        "nested": {"config": {"workflow": {
          "id": "flow-grand",
          "nodes": {"grand_start": {"config": {}}}
        }}}
      }
    }}}
  }
}`

func TestCacheNodeStatusResetsDownstreamScopes(t *testing.T) {
	service := &AutomationService{node_states: map[string]*automation_run_node_states{}}
	service.prime_run_node_states("run-test", `{"flow-root":{"start":"PENDING","sub":"PENDING"}}`, automation_node_states_definition)

	service.cache_node_status(flowengine.NodeExecutionStatus{
		RunID: "run-test", FlowID: "flow-root", NodeID: "start", Status: flowengine.StateRunning,
	})
	if states, _ := service.node_states_for_scope("run-test", "flow-root"); states["start"] != "RUNNING" {
		t.Fatalf("expected start to run, got %v", states)
	}

	// A subflow handoff re-enters its child scope: every descendant node must
	// drop back to PENDING, including grandchildren.
	service.cache_node_status(flowengine.NodeExecutionStatus{
		RunID: "run-test", FlowID: "flow-root", NodeID: "sub", Status: flowengine.StateRunning,
		NodeStates: map[string]flowengine.NodeState{
			"start": flowengine.StatePending,
			"sub":   flowengine.StatePending,
		},
	})
	child, ok := service.node_states_for_scope("run-test", "flow-child")
	if !ok || child["child_start"] != "PENDING" || child["nested"] != "PENDING" {
		t.Fatalf("expected child scope reset, got %v (cached=%v)", child, ok)
	}
	grand, _ := service.node_states_for_scope("run-test", "flow-grand")
	if grand["grand_start"] != "PENDING" {
		t.Fatalf("expected grandchild scope reset, got %v", grand)
	}

	// Callers receive a copy so the detail endpoint cannot mutate the cache.
	states, _ := service.node_states_for_scope("run-test", "flow-root")
	states["start"] = "MUTATED"
	if fresh, _ := service.node_states_for_scope("run-test", "flow-root"); fresh["start"] != "PENDING" {
		t.Fatalf("expected cached states to stay isolated, got %v", fresh)
	}

	// A cancelled or expired run drops its cache; later events are ignored.
	service.forget_run_node_states("run-test")
	service.cache_node_status(flowengine.NodeExecutionStatus{
		RunID: "run-test", FlowID: "flow-root", NodeID: "start", Status: flowengine.StateCompleted,
	})
	if _, ok := service.node_states_for_scope("run-test", "flow-root"); ok {
		t.Fatal("expected forgotten run to stay uncached")
	}
}

func TestPrimeRunNodeStatesToleratesMissingDefinition(t *testing.T) {
	service := &AutomationService{node_states: map[string]*automation_run_node_states{}}
	service.prime_run_node_states("run-plain", `{"flow-root":{"start":"PENDING"}}`, "")
	service.cache_node_status(flowengine.NodeExecutionStatus{
		RunID: "run-plain", FlowID: "flow-root", NodeID: "start",
		Status: flowengine.StateCompleted, NodeStates: map[string]flowengine.NodeState{"start": flowengine.StateCompleted},
	})
	raw, ok := service.node_states_snapshot("run-plain")
	if !ok {
		t.Fatal("expected snapshot for primed run")
	}
	var states map[string]map[string]string
	if err := json.Unmarshal([]byte(raw), &states); err != nil {
		t.Fatal(err)
	}
	if states["flow-root"]["start"] != "COMPLETED" {
		t.Fatalf("unexpected states: %v", states)
	}
}
