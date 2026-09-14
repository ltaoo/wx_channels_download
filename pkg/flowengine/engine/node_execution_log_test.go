package engine

import "testing"

type node_execution_log_test_node struct {
	id string
}

func (n *node_execution_log_test_node) ID() string   { return n.id }
func (n *node_execution_log_test_node) Type() string { return "log-test" }

func (n *node_execution_log_test_node) Execute(ctx *ProcessContext) (bool, []string, error) {
	ctx.Data["result"] = map[string]interface{}{
		"value":        "done",
		"access_token": "do-not-log",
	}
	delete(ctx.Data, "temporary")
	return true, nil, nil
}

func TestFlowEngineLogsNodeInputBehaviorAndOutput(t *testing.T) {
	flow_engine := &FlowEngine{}
	flow_engine.RegisterNode("log-test", func(config map[string]interface{}) Node {
		id, _ := config["id"].(string)
		return &node_execution_log_test_node{id: id}
	})
	flow_engine.SetFlowDefinitions(map[string]FlowDefinition{
		"flow": {
			ID:          "flow",
			StartNodeID: "node",
			Nodes: map[string]NodeDefinition{
				"node": {
					ID:   "node",
					Name: "logged node",
					Type: "log-test",
					Config: map[string]interface{}{
						"mode":      "verify",
						"api_token": "do-not-log",
						"func":      func() {},
					},
				},
			},
		},
	})

	entries := make([]NodeExecutionLog, 0, 1)
	statuses := make([]NodeExecutionStatus, 0, 2)
	flow_engine.SetNodeExecutionLogHandler(func(entry NodeExecutionLog) {
		entries = append(entries, entry)
	})
	flow_engine.SetNodeExecutionStatusHandler(func(status NodeExecutionStatus) {
		statuses = append(statuses, status)
	})
	_, err := flow_engine.StartFlowWithOptions(
		"flow",
		map[string]interface{}{
			"username":  "alice",
			"password":  "do-not-log",
			"temporary": true,
		},
		StartFlowOptions{RunID: "run-log-test"},
	)
	if err != nil {
		t.Fatalf("StartFlow failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected one node log entry, got %d", len(entries))
	}
	if len(statuses) != 2 || statuses[0].Status != StateRunning || statuses[1].Status != StateCompleted {
		t.Fatalf("expected running/completed node status updates, got %#v", statuses)
	}
	if statuses[0].RunID != "run-log-test" || statuses[1].RunID != "run-log-test" {
		t.Fatalf("expected the caller run id in status updates, got %#v", statuses)
	}
	entry := entries[0]
	if entry.FlowID != "flow" || entry.NodeID != "node" || entry.NodeType != "log-test" {
		t.Fatalf("unexpected node identity: %#v", entry)
	}
	if entry.RunID != "run-log-test" {
		t.Fatalf("expected caller run id in execution log, got %q", entry.RunID)
	}
	if entry.Outcome != "completed" || !entry.Success || entry.Attempt != 1 {
		t.Fatalf("unexpected node result: %#v", entry)
	}
	if entry.Input["username"] != "alice" || entry.Input["password"] != "[REDACTED]" {
		t.Fatalf("unexpected or unsafe input log: %#v", entry.Input)
	}
	config, ok := entry.Behavior["config"].(map[string]interface{})
	if !ok || config["mode"] != "verify" || config["api_token"] != "[REDACTED]" {
		t.Fatalf("unexpected or unsafe behavior log: %#v", entry.Behavior)
	}
	if config["func"] != "[UNSERIALIZABLE:func()]" {
		t.Fatalf("expected non-JSON behavior to have a safe placeholder, got %#v", config["func"])
	}
	if _, exists := entry.Output["username"]; exists {
		t.Fatalf("unchanged input must not be reported as node output: %#v", entry.Output)
	}
	result, ok := entry.Output["result"].(map[string]interface{})
	if !ok || result["value"] != "done" || result["access_token"] != "[REDACTED]" {
		t.Fatalf("unexpected or unsafe output log: %#v", entry.Output)
	}
	if len(entry.RemovedOutputKeys) != 1 || entry.RemovedOutputKeys[0] != "temporary" {
		t.Fatalf("expected removed context key to be logged, got %#v", entry.RemovedOutputKeys)
	}
}

func TestFlowEngineIgnoresNodeLogHandlerPanic(t *testing.T) {
	flow_engine := &FlowEngine{}
	flow_engine.RegisterNode("log-test", func(config map[string]interface{}) Node {
		id, _ := config["id"].(string)
		return &node_execution_log_test_node{id: id}
	})
	flow_engine.SetFlowDefinitions(map[string]FlowDefinition{
		"flow": {
			ID:          "flow",
			StartNodeID: "node",
			Nodes: map[string]NodeDefinition{
				"node": {ID: "node", Type: "log-test"},
			},
		},
	})
	flow_engine.SetNodeExecutionLogHandler(func(NodeExecutionLog) {
		panic("broken log sink")
	})

	if _, err := flow_engine.StartFlow("flow", map[string]interface{}{}); err != nil {
		t.Fatalf("logging panic must not fail the flow: %v", err)
	}
}
