package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"wx_channel/internal/mcpserver"
)

func editor_test_client(pool *pipeline_editor_pool, flow_id string) *automation_ws_client {
	client := &automation_ws_client{send: make(chan []byte, 8)}
	raw, _ := json.Marshal(map[string]string{"type": "pipeline_editor_register", "flow_id": flow_id})
	pool.receive(client, raw)
	return client
}
func editor_test_reply(pool *pipeline_editor_pool, client *automation_ws_client, raw []byte, revision int) {
	var request map[string]interface{}
	_ = json.Unmarshal(raw, &request)
	encoded, _ := json.Marshal(map[string]interface{}{"type": "pipeline_editor_reply", "request_id": request["request_id"], "snapshot": map[string]interface{}{"flow_id": request["flow_id"], "revision": revision, "dirty": true, "draft": request["draft"]}})
	pool.receive(client, encoded)
}
func TestPipelineEditorMCPRoundTripWaitsForPage(t *testing.T) {
	previous := pipeline_editors
	pipeline_editors = new_pipeline_editor_pool()
	defer func() { pipeline_editors = previous }()
	page := editor_test_client(pipeline_editors, "flow-test")
	id := pipeline_editors.list("")[0].ID
	router := gin.New()
	client := &APIClient{engine: router}
	router.POST("/api/v1/automation/get_editor", client.handle_get_pipeline_editor)
	router.POST("/api/v1/automation/update_editor", client.handle_update_pipeline_editor)
	server := httptest.NewServer(router)
	defer server.Close()
	tools, err := mcpserver.NewToolSet(mcpserver.Config{APIBaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	reply_read := make(chan struct{})
	go func() { raw := <-page.send; editor_test_reply(pipeline_editors, page, raw, 1); close(reply_read) }()
	read, err := tools.ExecuteTool(context.Background(), "get_pipeline_editor", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	<-reply_read
	encoded, _ := json.Marshal(read)
	if !strings.Contains(string(encoded), id) || !strings.Contains(string(encoded), "catalog") {
		t.Fatalf("missing live context: %s", encoded)
	}
	finished := make(chan error, 1)
	go func() {
		_, err := tools.ExecuteTool(context.Background(), "update_pipeline_editor", map[string]any{"editor_id": id, "revision": 1, "draft": map[string]any{"start_node_id": "start", "nodes": []map[string]any{{"id": "start", "type": "StartNode", "config": map[string]any{}, "next_ids": []string{}}}, "triggers": []any{}}})
		finished <- err
	}()
	select {
	case raw := <-page.send:
		select {
		case err := <-finished:
			t.Fatalf("returned before page acknowledgement: %v", err)
		default:
		}
		var request map[string]interface{}
		_ = json.Unmarshal(raw, &request)
		draft := request["draft"].(map[string]interface{})
		if _, ok := draft["triggers"].([]interface{}); !ok {
			t.Fatal("empty trigger list was lost")
		}
		editor_test_reply(pipeline_editors, page, raw, 2)
	case <-time.After(time.Second):
		t.Fatal("write never reached page")
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}
func TestPipelineEditorIsolationAndDisconnect(t *testing.T) {
	pool := new_pipeline_editor_pool()
	page := editor_test_client(pool, "flow-a")
	other := editor_test_client(pool, "flow-a")
	sessions := pool.list("flow-a")
	if len(sessions) != 2 {
		t.Fatal("multiple tabs were merged")
	}
	var id string
	for client, session := range pool.sessions {
		if client == page {
			id = session.ID
		}
	}
	finished := make(chan error, 1)
	go func() {
		_, err := pool.request(context.Background(), id, map[string]interface{}{"type": "pipeline_editor_read"})
		finished <- err
	}()
	raw := <-page.send
	editor_test_reply(pool, other, raw, 999)
	select {
	case err := <-finished:
		t.Fatalf("accepted another page's reply: %v", err)
	default:
	}
	pool.remove(page)
	if err := <-finished; err == nil || !strings.Contains(err.Error(), "断开") {
		t.Fatalf("disconnect not reported: %v", err)
	}
	if len(pool.list("flow-a")) != 1 {
		t.Fatal("closed editor still discoverable")
	}
	if len(pool.pending) != 0 {
		t.Fatal("pending request leaked")
	}
}
func TestPipelineEditorRejectsInvalidGraphBeforeSending(t *testing.T) {
	client := &APIClient{}
	router := gin.New()
	router.POST("/update", client.handle_update_pipeline_editor)
	request := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader(`{"editor_id":"editor","revision":1,"draft":{"start_node_id":"start","nodes":[{"id":"start","type":"StartNode","next_ids":["missing"]}]}}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if !strings.Contains(response.Body.String(), "不存在") {
		t.Fatalf("invalid graph not rejected: %s", response.Body.String())
	}
}

func TestPipelineEditorMultipleTabsRequireSelection(t *testing.T) {
	previous := pipeline_editors
	pipeline_editors = new_pipeline_editor_pool()
	defer func() { pipeline_editors = previous }()
	editor_test_client(pipeline_editors, "same-flow")
	editor_test_client(pipeline_editors, "same-flow")
	router := gin.New()
	client := &APIClient{}
	router.POST("/get", client.handle_get_pipeline_editor)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/get", strings.NewReader(`{"flow_id":"same-flow"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	if !strings.Contains(response.Body.String(), `"selection_required":true`) {
		t.Fatalf("ambiguous editor auto-selected: %s", response.Body.String())
	}
}
