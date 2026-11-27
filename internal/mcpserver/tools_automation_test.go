package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"wx_channel/pkg/mcp"
)

func TestCreateAutomationInstance(t *testing.T) {
	for _, test_case := range []struct {
		name         string
		arguments    string
		api_response string
		want_name    string
		want_error   string
		want_request bool
	}{
		{name: "default name", arguments: `{"flow_id":" flow-one "}`, want_request: true},
		{name: "custom name", arguments: `{"flow_id":"flow-one","name":" My instance "}`, want_name: "My instance", want_request: true},
		{name: "missing flow", arguments: `{}`, want_error: "flow_id"},
		{name: "blank flow", arguments: `{"flow_id":"  "}`, want_error: "flow_id"},
		{name: "blank name", arguments: `{"flow_id":"flow-one","name":"  "}`, want_error: "name"},
		{name: "invalid type", arguments: `{"flow_id":123}`, want_error: "工具参数无效"},
		{name: "unknown field", arguments: `{"flow_id":"flow-one","instance_id":"wrong"}`, want_error: "工具参数无效"},
		{name: "API error", arguments: `{"flow_id":"flow-one"}`, api_response: `{"code":400,"msg":"流程不存在","data":{"flow_id":"flow-one"}}`, want_error: "流程不存在", want_request: true},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			request_count := 0
			api_server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				request_count++
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/automation/create_flow_instance" {
					t.Errorf("unexpected API request: %s %s", r.Method, r.URL.Path)
				}
				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["flow_id"] != "flow-one" || body["name"] != test_case.want_name {
					t.Errorf("unexpected creation body: %#v", body)
				}
				w.Header().Set("Content-Type", "application/json")
				if test_case.api_response != "" {
					_, _ = w.Write([]byte(test_case.api_response))
					return
				}
				_, _ = w.Write([]byte(`{"code":0,"data":{"id":"instance-one","flow_id":"flow-one","inherit_triggers":true}}`))
			}))
			defer api_server.Close()
			server, _, err := NewRuntime(Config{APIBaseURL: api_server.URL})
			if err != nil {
				t.Fatal(err)
			}
			handler := mcp.NewHTTPHandler(server)
			list_response := httptest.NewRecorder()
			list_request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
			list_request.Header.Set("Content-Type", "application/json")
			handler.ServeHTTP(list_response, list_request)
			if !bytes.Contains(list_response.Body.Bytes(), []byte(`"name":"create_automation_instance"`)) {
				t.Fatal("creation tool missing from tools/list")
			}
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"create_automation_instance","arguments":`+test_case.arguments+`}}`))
			request.Header.Set("Content-Type", "application/json")
			handler.ServeHTTP(response, request)
			var result struct {
				Error  any `json:"error"`
				Result struct {
					IsError           bool           `json:"isError"`
					StructuredContent map[string]any `json:"structuredContent"`
				} `json:"result"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusOK || result.Error != nil || result.Result.IsError != (test_case.want_error != "") {
				t.Fatalf("unexpected MCP response: %s", response.Body.String())
			}
			if test_case.want_error != "" {
				if !strings.Contains(response.Body.String(), test_case.want_error) {
					t.Fatalf("missing error %q: %s", test_case.want_error, response.Body.String())
				}
				if test_case.api_response != "" && result.Result.StructuredContent["details"] == nil {
					t.Fatal("API error details were lost")
				}
			} else if result.Result.StructuredContent["id"] != "instance-one" || result.Result.StructuredContent["flow_id"] != "flow-one" {
				t.Fatalf("missing created instance: %#v", result.Result.StructuredContent)
			}
			if (request_count == 1) != test_case.want_request || request_count > 1 {
				t.Fatalf("unexpected API request count: %d", request_count)
			}
		})
	}
}

func TestAutomationInstanceManagement(t *testing.T) {
	for _, test_case := range []struct {
		name       string
		tool       string
		arguments  string
		path       string
		body       string
		api_error  bool
		want_error string
	}{
		{name: "list", tool: "list_automation_instances", arguments: `{}`, path: "list_flow_instances"},
		{name: "get", tool: "get_automation_instance", arguments: `{"id":" instance-one "}`, path: "get_flow_instance", body: `{"id":"instance-one"}`},
		{name: "rename only", tool: "update_automation_instance", arguments: `{"id":" instance-one ","name":" New title "}`, path: "update_flow_instance", body: `{"id":"instance-one","name":"New title"}`},
		{name: "clear description", tool: "update_automation_instance", arguments: `{"id":"instance-one","description":""}`, path: "update_flow_instance", body: `{"id":"instance-one","description":""}`},
		{name: "disable instance", tool: "update_automation_instance", arguments: `{"id":"instance-one","disabled":true}`, path: "update_flow_instance", body: `{"id":"instance-one","disabled":true}`},
		{name: "enable instance", tool: "update_automation_instance", arguments: `{"id":"instance-one","disabled":false}`, path: "update_flow_instance", body: `{"id":"instance-one","disabled":false}`},
		{name: "invalid disabled type", tool: "update_automation_instance", arguments: `{"id":"instance-one","disabled":"false"}`, want_error: "工具参数无效"},
		{name: "configuration", tool: "update_automation_instance", arguments: `{"id":"instance-one","node_configs":{"validate":{"username":"creator@finder","count":10,"enabled":false}}}`, path: "update_flow_instance", body: `{"id":"instance-one","node_configs":{"validate":{"username":"creator@finder","count":10,"enabled":false}}}`},
		{name: "clear configuration", tool: "update_automation_instance", arguments: `{"id":"instance-one","node_configs":{}}`, path: "update_flow_instance", body: `{"id":"instance-one","node_configs":{}}`},
		{name: "clear triggers", tool: "update_automation_instance", arguments: `{"id":"instance-one","triggers":[]}`, path: "update_flow_instance", body: `{"id":"instance-one","triggers":[]}`},
		{name: "custom trigger", tool: "update_automation_instance", arguments: `{"id":"instance-one","triggers":[{"name":"daily","type":"Cron","cron_expr":"0 9 * * *","enabled":false}]}`, path: "update_flow_instance", body: `{"id":"instance-one","triggers":[{"name":"daily","type":"Cron","cron_expr":"0 9 * * *","enabled":false}]}`},
		{name: "cancel inheritance", tool: "update_automation_instance", arguments: `{"id":"instance-one","inherit_triggers":false}`, path: "update_flow_instance", body: `{"id":"instance-one","inherit_triggers":false}`},
		{name: "restore inheritance", tool: "update_automation_instance", arguments: `{"id":"instance-one","inherit_triggers":true}`, path: "update_flow_instance", body: `{"id":"instance-one","inherit_triggers":true}`},
		{name: "clear trigger overrides", tool: "update_automation_instance", arguments: `{"id":"instance-one","trigger_overrides":{}}`, path: "update_flow_instance", body: `{"id":"instance-one","trigger_overrides":{}}`},
		{name: "trigger overrides", tool: "update_automation_instance", arguments: `{"id":"instance-one","trigger_overrides":{"daily":{"enabled":false}}}`, path: "update_flow_instance", body: `{"id":"instance-one","trigger_overrides":{"daily":{"enabled":false}}}`},
		{name: "delete", tool: "delete_automation_instance", arguments: `{"id":" instance-one "}`, path: "delete_flow_instance", body: `{"id":"instance-one"}`},
		{name: "get missing id", tool: "get_automation_instance", arguments: `{}`, want_error: "id 不能为空"},
		{name: "delete missing id", tool: "delete_automation_instance", arguments: `{"id":" "}`, want_error: "id 不能为空"},
		{name: "reject flow id", tool: "delete_automation_instance", arguments: `{"flow_id":"flow-one"}`, want_error: "工具参数无效"},
		{name: "update missing id", tool: "update_automation_instance", arguments: `{"name":"title"}`, want_error: "id 不能为空"},
		{name: "empty update", tool: "update_automation_instance", arguments: `{"id":"instance-one"}`, want_error: "至少提供一个"},
		{name: "null update", tool: "update_automation_instance", arguments: `{"id":"instance-one","node_configs":null}`, want_error: "至少提供一个"},
		{name: "empty title", tool: "update_automation_instance", arguments: `{"id":"instance-one","name":" "}`, want_error: "name 不能为空"},
		{name: "invalid configuration type", tool: "update_automation_instance", arguments: `{"id":"instance-one","node_configs":[]}`, want_error: "工具参数无效"},
		{name: "reject shared graph", tool: "update_automation_instance", arguments: `{"id":"instance-one","nodes":[]}`, want_error: "工具参数无效"},
		{name: "reject shared defaults", tool: "update_automation_instance", arguments: `{"id":"instance-one","default_node_configs":{}}`, want_error: "工具参数无效"},
		{name: "update API error", tool: "update_automation_instance", arguments: `{"id":"instance-one","name":"title"}`, path: "update_flow_instance", body: `{"id":"instance-one","name":"title"}`, api_error: true, want_error: "实例不存在"},
		{name: "delete API error", tool: "delete_automation_instance", arguments: `{"id":"instance-one"}`, path: "delete_flow_instance", body: `{"id":"instance-one"}`, api_error: true, want_error: "实例不存在"},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			request_count := 0
			api_server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				request_count++
				if r.Method != http.MethodPost || r.URL.Path != "/api/v1/automation/"+test_case.path {
					t.Errorf("unexpected API request: %s %s", r.Method, r.URL.Path)
				}
				if test_case.body != "" {
					var got_body, want_body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&got_body); err != nil {
						t.Error(err)
					}
					if err := json.Unmarshal([]byte(test_case.body), &want_body); err != nil {
						t.Error(err)
					}
					if !reflect.DeepEqual(got_body, want_body) {
						t.Errorf("request changed omitted or empty fields: got %#v want %#v", got_body, want_body)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				if test_case.api_error {
					_, _ = w.Write([]byte(`{"code":400,"msg":"实例不存在","data":{"id":"instance-one"}}`))
				} else {
					_, _ = w.Write([]byte(`{"code":0,"data":{"id":"instance-one","flow_id":"flow-one"}}`))
				}
			}))
			defer api_server.Close()
			toolset, err := NewToolSet(Config{APIBaseURL: api_server.URL})
			if err != nil {
				t.Fatal(err)
			}
			if !toolset.supports_tool(test_case.tool) {
				t.Fatal("instance tool was not registered")
			}
			var arguments map[string]any
			if err := json.Unmarshal([]byte(test_case.arguments), &arguments); err != nil {
				t.Fatal(err)
			}
			value, err := toolset.ExecuteTool(context.Background(), test_case.tool, arguments)
			if test_case.want_error != "" {
				if err == nil || !strings.Contains(err.Error(), test_case.want_error) {
					t.Fatalf("expected %q, got %v", test_case.want_error, err)
				}
				if test_case.api_error {
					var tool_error *tool_execution_error
					if !errors.As(err, &tool_error) || mcp.ErrorResult(err)["structuredContent"].(map[string]any)["details"] == nil {
						t.Fatalf("lost structured API error: %v", err)
					}
				}
			} else if err != nil || value.(map[string]any)["id"] != "instance-one" {
				t.Fatalf("unexpected result: %#v, %v", value, err)
			}
			if (request_count == 1) != (test_case.path != "") || request_count > 1 {
				t.Fatalf("unexpected API request count: %d", request_count)
			}
		})
	}
}
