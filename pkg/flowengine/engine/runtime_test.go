package engine_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"wx_channel/pkg/flowengine"
)

func runtime_node(id, node_type string, config map[string]interface{}, next ...string) flowengine.NodeDefinition {
	targets := []flowengine.TargetNode{}
	for _, id := range next {
		targets = append(targets, flowengine.TargetNode{TargetID: id})
	}
	return flowengine.NodeDefinition{ID: id, Type: node_type, Config: config, NextNodeIDs: next, NextNodes: targets}
}

func TestWorkflowCancelsJavaScriptAndSkipsSuccessors(t *testing.T) {
	engine := flowengine.NewWorkflowEngine()
	var called atomic.Int32
	definition := flowengine.FlowDefinition{ID: "cancel", StartNodeID: "js", Nodes: map[string]flowengine.NodeDefinition{
		"js":    runtime_node("js", "JSCodeNode", map[string]interface{}{"code": "while (true) {}", "output_key": "result"}, "after"),
		"after": runtime_node("after", "FuncNode", map[string]interface{}{"func": func(map[string]interface{}) (interface{}, error) { called.Add(1); return nil, nil }}),
	}}
	engine.RegisterFlow(definition)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := engine.StartFlowWithOptions("cancel", nil, flowengine.StartFlowOptions{Context: ctx})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
	if called.Load() != 0 {
		t.Fatal("successor ran after cancellation")
	}
}

func TestParallelBranchesActuallyOverlapAndJoinOnce(t *testing.T) {
	engine := flowengine.NewWorkflowEngine()
	entered := make(chan string, 2)
	release := make(chan struct{})
	var joined atomic.Int32
	branch := func(id string) flowengine.NodeDefinition {
		return runtime_node(id, "FuncNode", map[string]interface{}{"output_key": id, "func": func(map[string]interface{}) (interface{}, error) { entered <- id; <-release; return id, nil }}, "join")
	}
	engine.RegisterFlow(flowengine.FlowDefinition{ID: "parallel", StartNodeID: "fork", Nodes: map[string]flowengine.NodeDefinition{
		"fork": runtime_node("fork", "GatewayNode", map[string]interface{}{"gateway_type": "Parallel", "join_node_id": "join"}, "a", "b"),
		"a":    branch("a"), "b": branch("b"),
		"join": runtime_node("join", "FuncNode", map[string]interface{}{"func": func(data map[string]interface{}) (interface{}, error) {
			joined.Add(1)
			if data["a"] != "a" || data["b"] != "b" {
				return nil, fmt.Errorf("missing branch output: %v", data)
			}
			return nil, nil
		}}),
	}})
	done := make(chan error, 1)
	go func() { _, err := engine.StartFlow("parallel", nil); done <- err }()
	for index := 0; index < 2; index++ {
		select {
		case <-entered:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("parallel branch was serialized")
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if joined.Load() != 1 {
		t.Fatalf("join ran %d times", joined.Load())
	}
}

func TestWaitingCheckpointRestoresScopesAndPendingSiblings(t *testing.T) {
	first := flowengine.NewWorkflowEngine()
	definition := flowengine.FlowDefinition{ID: "manual", StartNodeID: "seed", Nodes: map[string]flowengine.NodeDefinition{
		"seed":    runtime_node("seed", "SetVariableNode", map[string]interface{}{"variables": map[string]interface{}{"flag": "preserved"}}, "confirm", "sibling"),
		"confirm": runtime_node("confirm", "ManualNode", map[string]interface{}{}, "answer"),
		"answer":  runtime_node("answer", "JSCodeNode", map[string]interface{}{"code": "global.flag + ':' + output.confirm.response.answer", "output_key": "answer_value"}),
		"sibling": runtime_node("sibling", "JSCodeNode", map[string]interface{}{"code": "'sibling ran'", "output_key": "sibling_value"}),
	}}
	first.RegisterFlow(definition)
	var checkpoint *flowengine.RunCheckpoint
	first.SetCheckpointHandler(func(value *flowengine.RunCheckpoint) error { checkpoint = value; return nil })
	id, err := first.StartFlow("manual", nil)
	if err != nil {
		t.Fatal(err)
	}
	if checkpoint == nil || !checkpoint.Waiting || len(checkpoint.PendingNodes) != 1 {
		t.Fatalf("lost continuation: %+v", checkpoint)
	}
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	restored := &flowengine.RunCheckpoint{}
	if err := json.Unmarshal(raw, restored); err != nil {
		t.Fatal(err)
	}
	second := flowengine.NewWorkflowEngine()
	if err := second.ResumeCheckpoint(context.Background(), restored, "confirm", map[string]interface{}{"answer": "yes"}); err != nil {
		t.Fatal(err)
	}
	run, ok := second.GetRunSnapshot(id)
	if !ok || run.NodeOutputs["answer_value"] != "preserved:yes" || run.NodeOutputs["sibling_value"] != "sibling ran" {
		t.Fatalf("restored run: %+v", run)
	}
}

func TestLoopIterationAndStepLimit(t *testing.T) {
	engine := flowengine.NewWorkflowEngine()
	child := flowengine.FlowDefinition{ID: "child", Name: "child", StartNodeID: "calc", ContextSchema: []flowengine.FieldSchema{{Key: "item", Type: "number"}, {Key: "index", Type: "number"}}, Nodes: map[string]flowengine.NodeDefinition{
		"calc": runtime_node("calc", "JSCodeNode", map[string]interface{}{"code": "input.item * 2", "output_key": "double"}),
	}}
	engine.RegisterFlow(flowengine.FlowDefinition{ID: "loop", StartNodeID: "loop", Nodes: map[string]flowengine.NodeDefinition{
		"loop": runtime_node("loop", "LoopNode", map[string]interface{}{"items": []interface{}{1, 2, 3}, "workflow": child, "output_key": "items", "concurrency": 2}),
	}})
	id, err := engine.StartFlow("loop", nil)
	if err != nil {
		t.Fatal(err)
	}
	run, _ := engine.GetRunSnapshot(id)
	raw, _ := json.Marshal(run.NodeOutputs["items"])
	if !strings.Contains(string(raw), `"double":6`) {
		t.Fatalf("missing iteration output: %s", raw)
	}
	engine.RegisterFlow(flowengine.FlowDefinition{ID: "cycle", StartNodeID: "a", Nodes: map[string]flowengine.NodeDefinition{"a": runtime_node("a", "StartNode", nil, "a")}})
	if _, err := engine.StartFlowWithOptions("cycle", nil, flowengine.StartFlowOptions{MaxSteps: 5}); err == nil || !strings.Contains(err.Error(), "step limit") {
		t.Fatalf("unbounded cycle: %v", err)
	}
}

func TestRetryAndErrorRoute(t *testing.T) {
	engine := flowengine.NewWorkflowEngine()
	var attempts atomic.Int32
	failing := runtime_node("failing", "FuncNode", map[string]interface{}{"func": func(map[string]interface{}) (interface{}, error) {
		attempts.Add(1)
		return nil, errors.New("upstream failed")
	}})
	failing.RetryPolicy = &flowengine.RetryPolicy{MaxAttempts: 3, DelayMs: 1, BackoffMultiplier: 2}
	failing.ErrorNextNodeID = "recover"
	engine.RegisterFlow(flowengine.FlowDefinition{ID: "retry", StartNodeID: "failing", Nodes: map[string]flowengine.NodeDefinition{
		"failing": failing,
		"recover": runtime_node("recover", "JSCodeNode", map[string]interface{}{"code": "output.failing.error.message", "output_key": "recovered"}),
	}})
	id, err := engine.StartFlow("retry", nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, _ := engine.GetRunSnapshot(id)
	if attempts.Load() != 3 || snapshot.NodeOutputs["recovered"] != "upstream failed" {
		t.Fatalf("retry/route failed: %+v, attempts %d", snapshot, attempts.Load())
	}
}
