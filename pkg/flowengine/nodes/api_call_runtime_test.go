package nodes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"wx_channel/pkg/flowengine/engine"
)

func TestHTTPNodeRequestConfigurationAndStatusHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != "PATCH" || request.URL.Query().Get("who") != "v2_abc@finder" || request.Header.Get("Authorization") != "Bearer token-value" || request.Header.Get("X-Context") != "3" {
			t.Errorf("wrong request: %s %v %v", request.Method, request.URL, request.Header)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["nested"].(map[string]interface{})["count"] != float64(3) {
			t.Error("nested template lost type")
		}
		writer.Header().Set("X-Reply", "yes")
		writer.WriteHeader(503)
		_, _ = writer.Write([]byte(`{"error":"unavailable"}`))
	}))
	defer server.Close()
	config := map[string]interface{}{"id": "http", "url": server.URL, "method": "PATCH",
		"query":          map[string]interface{}{"who": "{{input.username}}"},
		"headers":        map[string]interface{}{"X-Context": "{{input.count}}"},
		"authentication": map[string]interface{}{"type": "bearer", "token": "token-value"},
		"body":           map[string]interface{}{"nested": map[string]interface{}{"count": "{{input.count}}"}},
	}
	ctx := template_context()
	ctx.EngineRef = &engine.FlowEngine{}
	node := NewAPICallNode(config)
	if _, _, err := node.Execute(ctx); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("status did not fail: %v", err)
	}
	if ctx.NodeOutputs["http"]["api_response_status"] != float64(503) {
		t.Fatal("failed response not exposed to recovery path")
	}
	config["fail_on_status"] = false
	if success, _, err := node.Execute(ctx); err != nil || !success {
		t.Fatalf("explicit status handling failed: %v", err)
	}
}

func TestHTTPNodeCancellationAndFormBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/wait" {
			<-request.Context().Done()
			return
		}
		if err := request.ParseForm(); err != nil {
			t.Error(err)
		}
		if request.Form.Get("count") != "3" {
			t.Error("form template missing")
		}
		username, password, ok := request.BasicAuth()
		if !ok || username != "user" || password != "pass" {
			t.Error("basic auth missing")
		}
		_, _ = writer.Write([]byte(`[1,2]`))
	}))
	defer server.Close()
	ctx := template_context()
	ctx.EngineRef = &engine.FlowEngine{}
	node := NewAPICallNode(map[string]interface{}{"id": "form", "url": server.URL, "method": "POST", "body_type": "form", "body": map[string]interface{}{"count": "{{input.count}}"}, "authentication": map[string]interface{}{"type": "basic", "username": "user", "password": "pass"}})
	if _, _, err := node.Execute(ctx); err != nil {
		t.Fatal(err)
	}
	if len(ctx.NodeOutputs["form"]["api_response"].([]interface{})) != 2 {
		t.Fatal("JSON array was not decoded")
	}
	request_context, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	ctx.Context = request_context
	node = NewAPICallNode(map[string]interface{}{"id": "wait", "url": server.URL + "/wait"})
	started := time.Now()
	if _, _, err := node.Execute(ctx); err == nil || time.Since(started) > time.Second {
		t.Fatalf("HTTP cancellation failed: %v", err)
	}
}
