package engine_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"wx_channel/pkg/flowengine"
)

func TestNodeStatusDemoLoopsThreeTimes(t *testing.T) {
	raw, err := os.ReadFile("../../../examples/pipelines/node_status_demo.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), `"seconds": 3`) != 3 || strings.Count(string(raw), `"seconds": 1`) != 3 {
		t.Fatal("demo must keep three-second parent waits and three one-second steps in the reusable subflow")
	}
	// Exercise the real graph without spending time in its demonstration delays.
	raw = []byte(strings.ReplaceAll(string(raw), `"seconds": 3`, `"seconds": 0`))
	raw = []byte(strings.ReplaceAll(string(raw), `"seconds": 1`, `"seconds": 0`))
	var definition flowengine.FlowDefinition
	if err := json.Unmarshal(raw, &definition); err != nil {
		t.Fatal(err)
	}
	for node_id, node := range definition.Nodes {
		node.ID = node_id
		for _, next_id := range node.NextNodeIDs {
			node.NextNodes = append(node.NextNodes, flowengine.TargetNode{TargetID: next_id})
		}
		definition.Nodes[node_id] = node
	}
	flow_engine := flowengine.NewWorkflowEngine()
	resets := 0
	downstream_resets := 0
	counts := []string{}
	flow_engine.SetNodeExecutionStatusHandler(func(status flowengine.NodeExecutionStatus) {
		if status.FlowID == definition.ID && status.NodeID == "child" && status.Status == flowengine.StateRunning && status.Attempt > 1 {
			downstream_resets++
			_, states := flow_engine.GetRunContext(status.RunID)
			for _, node_id := range []string{"process", "increment_count", "store_count", "check_count"} {
				if states[node_id] != flowengine.StatePending || status.NodeStates[node_id] != flowengine.StatePending {
					t.Fatalf("downstream node %s retained its previous state: snapshot=%v, update=%v", node_id, states, status.NodeStates)
				}
			}
			if states["child"] != flowengine.StateRunning || states["prepare"] != flowengine.StateCompleted || states["init_count"] != flowengine.StateCompleted {
				t.Fatalf("reset affected the active node or upstream nodes: %v", states)
			}
		}
		if status.FlowID == "node-status-demo-child" && status.NodeStates != nil {
			resets++
			if status.NodeStates["child_wait"] != flowengine.StatePending {
				t.Fatal("reentered child retained its previous status")
			}
		}
	})
	flow_engine.SetNodeExecutionLogHandler(func(entry flowengine.NodeExecutionLog) {
		if entry.DurationMs < 800 {
			t.Fatalf("node %s completed in %dms, before the engine minimum", entry.NodeID, entry.DurationMs)
		}
		if entry.FlowID == "node-status-demo-child" && entry.NodeID == "child_wait" {
			counts = append(counts, fmt.Sprint(entry.Input["count"]))
		}
	})
	flow_engine.RegisterFlow(definition)
	run_id, err := flow_engine.StartFlowWithOptions(definition.ID, nil, flowengine.StartFlowOptions{MaxSteps: 50})
	if err != nil {
		t.Fatal(err)
	}
	run, ok := flow_engine.GetRunSnapshot(run_id)
	if !ok || run.Status != flowengine.RunStatusCompleted || run.NodeAttempts["child"] != 3 || run.NodeAttempts["check_count"] != 4 || run.NodeAttempts["finish"] != 1 {
		t.Fatalf("loop did not finish after exactly three child calls: %+v", run)
	}
	if resets != 3 || downstream_resets != 2 || !reflect.DeepEqual(counts, []string{"0", "1", "2"}) {
		t.Fatalf("incorrect child reentries: resets=%d, downstream_resets=%d, counts=%v", resets, downstream_resets, counts)
	}
	data, _ := flow_engine.GetRunContext(run_id)
	if fmt.Sprint(data["count"]) != "3" {
		t.Fatalf("final count is not three: %v", data["count"])
	}
}

func TestNodeMinimumDuration(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancelled=%v", cancelled), func(t *testing.T) {
			flow_engine := flowengine.NewWorkflowEngine()
			definition := flowengine.FlowDefinition{ID: "minimum-duration", StartNodeID: "first", Nodes: map[string]flowengine.NodeDefinition{
				"first": runtime_node("first", "StartNode", nil, "last"),
				"last":  runtime_node("last", "EndNode", nil),
			}}
			flow_engine.RegisterFlow(definition)
			var duration_ms int64
			flow_engine.SetNodeExecutionLogHandler(func(entry flowengine.NodeExecutionLog) {
				if entry.NodeID == "first" {
					duration_ms = entry.DurationMs
				}
			})
			run_context := context.Background()
			if cancelled {
				var cancel context.CancelFunc
				run_context, cancel = context.WithTimeout(run_context, 50*time.Millisecond)
				defer cancel()
			}
			run_id, err := flow_engine.StartFlowWithOptions(definition.ID, nil, flowengine.StartFlowOptions{Context: run_context})
			if cancelled {
				if !errors.Is(err, context.DeadlineExceeded) || duration_ms >= 800 {
					t.Fatalf("minimum duration blocked cancellation: duration=%d, err=%v", duration_ms, err)
				}
				run, _ := flow_engine.GetRunSnapshot(run_id)
				if run.NodeAttempts["last"] != 0 {
					t.Fatal("successor executed after cancellation")
				}
			} else if err != nil || duration_ms < 800 {
				t.Fatalf("node completed too early: duration=%d, err=%v", duration_ms, err)
			}
		})
	}
}
