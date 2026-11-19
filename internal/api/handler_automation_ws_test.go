package api

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"wx_channel/pkg/flowengine"
)

func TestAutomationWebsocketSummaryOmitsNodePayloads(t *testing.T) {
	pool := new_automation_ws_pool()
	summary := &automation_ws_client{flow_id: "target", summary: true, send: make(chan []byte, 1)}
	full := &automation_ws_client{send: make(chan []byte, 1)}
	other := &automation_ws_client{flow_id: "other", summary: true, send: make(chan []byte, 1)}
	for _, client := range []*automation_ws_client{summary, full, other} {
		pool.add(client)
	}
	payload := strings.Repeat("large-payload", 10000)
	entry := &flowengine.NodeExecutionLog{NodeID: "fetch", Attempt: 2, Outcome: "completed",
		Input: map[string]interface{}{"input": payload}, Behavior: map[string]interface{}{"config": payload}, Output: map[string]interface{}{"result": payload}}
	pool.broadcast(automation_ws_message{Type: automation_node_log_ws_update, FlowID: "target", RunID: "run-view", ExecutionLog: entry})
	if len(other.send) != 0 || len(summary.send) != 1 || len(full.send) != 1 {
		t.Fatal("flow filtering or delivery failed")
	}
	small := <-summary.send
	if len(small) > 1024 || bytes.Contains(small, []byte("large-payload")) {
		t.Fatalf("summary streamed large payload: %d bytes", len(small))
	}
	var message automation_ws_message
	if err := json.Unmarshal(small, &message); err != nil || message.ExecutionLog.NodeID != "fetch" || message.ExecutionLog.Attempt != 2 {
		t.Fatalf("summary lost node metadata: %s", small)
	}
	if !bytes.Contains(<-full.send, []byte(payload)) || entry.Output["result"] != payload {
		t.Fatal("full log subscribers or source event changed")
	}
}
