package flowengine

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func write_custom_script(t *testing.T, directory, name, source string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(directory, "nodes"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "nodes", name), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCustomNodesRegistrationIsolationAndFrozenSource(t *testing.T) {
	directory := t.TempDir()
	source := `import {createHmac} from "node:crypto";
let count = 0;
registerNode({type: "ExampleNode", name: "示例", default_config: {arguments: {}, output_key: "result"},
  async execute(args) {
    await new Promise(resolve => setTimeout(resolve, 1));
    return {count: ++count, value: args.value, signature: createHmac("sha1", "key").update("test").digest("hex")};
  }
});`
	write_custom_script(t, directory, "example.js", source)
	flow_engine := NewWorkflowEngine()
	registry, err := LoadCustomNodes(context.Background(), flow_engine, directory, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !registry.Has("ExampleNode") || registry.Has("MissingNode") || len(registry.Catalog()) != 1 || registry.Catalog()[0].Name != "示例" {
		t.Fatalf("incorrect catalog: %+v", registry.Catalog())
	}
	write_custom_script(t, directory, "example.js", `throw new Error("changed on disk");`)
	var workers sync.WaitGroup
	for index := 0; index < 4; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			result, err := registry.execute(context.Background(), "ExampleNode", map[string]any{"value": "original"}, nil)
			if err != nil {
				t.Error(err)
				return
			}
			data := result.(map[string]any)
			if data["count"] != int64(1) || data["value"] != "original" || data["signature"] == "" {
				t.Errorf("runtime state leaked: %+v", data)
			}
		}()
	}
	workers.Wait()
	other := NewWorkflowEngine()
	if _, exists := other.NodeRegistry["ExampleNode"]; exists {
		t.Fatal("custom nodes leaked into another engine")
	}
}

func TestCustomNodesRejectInvalidRegistrations(t *testing.T) {
	for name, source := range map[string]string{
		"reserved":           `registerNode({type: "StartNode", name: "bad", execute() {}});`,
		"service":            `registerNode({type: "ServiceNode", name: "bad", execute() {}});`,
		"duplicate":          `for (let i = 0; i < 2; i++) registerNode({type: "ExampleNode", name: "bad", execute() {}});`,
		"missing execute":    `registerNode({type: "ExampleNode", name: "bad"});`,
		"bad type":           `registerNode({type: "../bad", name: "bad", execute() {}});`,
		"syntax":             `registerNode({`,
		"unsupported import": `import child from "child_process";`,
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			write_custom_script(t, directory, "bad.js", source)
			flow_engine := NewWorkflowEngine()
			if _, err := LoadCustomNodes(context.Background(), flow_engine, directory, nil); err == nil {
				t.Fatal("invalid script accepted")
			}
			if _, exists := flow_engine.NodeRegistry["ExampleNode"]; exists {
				t.Fatal("failed load partially changed engine")
			}
		})
	}
	if registry, err := LoadCustomNodes(context.Background(), NewWorkflowEngine(), t.TempDir(), nil); err != nil || len(registry.Catalog()) != 0 {
		t.Fatalf("missing nodes directory: %+v %v", registry, err)
	}
}

func TestCustomNodeCancellation(t *testing.T) {
	for _, code := range []string{`while (true) {}`, `await new Promise(() => {});`} {
		directory := t.TempDir()
		write_custom_script(t, directory, "cancel.js", `registerNode({type: "CancelNode", name: "cancel", async execute() {`+code+`}});`)
		registry, err := LoadCustomNodes(context.Background(), NewWorkflowEngine(), directory, nil)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
		started_at := time.Now()
		_, err = registry.execute(ctx, "CancelNode", nil, nil)
		cancel()
		if err == nil || !errors.Is(err, context.DeadlineExceeded) || time.Since(started_at) > time.Second {
			t.Fatalf("script did not stop: %v", err)
		}
	}
	directory := t.TempDir()
	write_custom_script(t, directory, "error.js", `registerNode({type: "ErrorNode", name: "error", async execute() {throw new Error("upload failed");}});`)
	registry, err := LoadCustomNodes(context.Background(), NewWorkflowEngine(), directory, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.execute(context.Background(), "ErrorNode", nil, nil); err == nil || !strings.Contains(err.Error(), "upload failed") {
		t.Fatalf("promise rejection lost: %v", err)
	}
}

func TestCustomNodeCancelsHTTPAndRejectsNonJSONOutput(t *testing.T) {
	started := make(chan struct{})
	request_done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		close(started)
		<-request.Context().Done()
		close(request_done)
	}))
	defer server.Close()
	directory := t.TempDir()
	write_custom_script(t, directory, "fetch.js", `registerNode({type: "FetchNode", name: "fetch", async execute(args) {
  return await (await fetch(args.url)).text();
}});
registerNode({type: "InvalidOutputNode", name: "invalid", execute() {return {fn() {}};}});`)
	registry, err := LoadCustomNodes(context.Background(), NewWorkflowEngine(), directory, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := registry.execute(ctx, "FetchNode", map[string]any{"url": server.URL}, nil)
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("fetch did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation lost: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not stop script")
	}
	select {
	case <-request_done:
	case <-time.After(time.Second):
		t.Fatal("cancel did not stop HTTP request")
	}
	if _, err := registry.execute(context.Background(), "InvalidOutputNode", nil, nil); err == nil || !strings.Contains(err.Error(), "JSON serializable") {
		t.Fatalf("invalid output was not rejected: %v", err)
	}
}

func TestCustomNodeRequiredSettingsAndDefaults(t *testing.T) {
	directory := t.TempDir()
	write_custom_script(t, directory, "settings.js", `registerNode({
  type: "SettingsNode", name: "settings",
  config_schema: [
    {key: "token", type: "string", required: true, secret: true},
    {key: "enabled", type: "boolean", default: false},
    {key: "count", type: "integer", default: 0},
    {key: "options", type: "object", default: {count: 0}}
  ],
  execute(args, config) { config.options.count++; return {enabled: config.enabled, count: config.count, changed: config.options.count}; }
});`)
	registry, err := LoadCustomNodes(context.Background(), NewWorkflowEngine(), directory, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, settings := range []map[string]any{nil, {"token": " "}, {"token": "test-token", "enabled": "bad"}} {
		if _, err := registry.execute(context.Background(), "SettingsNode", nil, settings); err == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
	for index := 0; index < 2; index++ {
		result, err := registry.execute(context.Background(), "SettingsNode", nil, map[string]any{"token": "test-token"})
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(result)
		if err != nil || string(encoded) != `{"changed":1,"count":0,"enabled":false}` {
			t.Fatalf("incorrect or shared defaults: %s %v", encoded, err)
		}
	}
	write_custom_script(t, directory, "settings.js", `registerNode({type: "SettingsNode", name: "bad", config_schema: [{key: "count", type: "integer", default: "bad"}], execute() {}});`)
	if _, err := LoadCustomNodes(context.Background(), NewWorkflowEngine(), directory, nil); err == nil {
		t.Fatal("invalid schema default accepted")
	}
}
