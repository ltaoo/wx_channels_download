package engine_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	wxchannelsadapter "wx_channel/internal/adapter/wxchannels"
	"wx_channel/pkg/flowengine"
	"wx_channel/pkg/flowengine/engine"
)

func TestCreatorVideoSyncHTTPPagination(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/feature/pipelines/01_creator_video_sync.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, test_case := range []struct {
		name           string
		token          string
		method         string
		raw_response   bool
		existing_video bool
		missing_video  bool
		last_page      bool
		failure        string
		want_error     string
	}{
		{name: "without token"},
		{name: "with token", token: "test-token"},
		{name: "POST with token", method: " post ", token: "test-token"},
		{name: "PATCH", method: "PATCH"},
		{name: "raw POST pagination", method: "POST", token: "test-token", raw_response: true},
		{name: "raw existing video", method: "POST", raw_response: true, existing_video: true},
		{name: "missing video without client socket", missing_video: true},
		{name: "raw missing video without client socket", method: "POST", token: "test-token", raw_response: true, missing_video: true},
		{name: "last page missing video without client socket", missing_video: true, last_page: true},
		{name: "raw last page missing video without client socket", raw_response: true, missing_video: true, last_page: true},
		{name: "raw BaseResponse failure", method: "POST", raw_response: true, failure: "base", want_error: "视频列表 BaseResponse 返回错误"},
		{name: "HTTP failure", failure: "http", want_error: "HTTP request returned status 401"},
		{name: "API failure", failure: "api", want_error: "列表 HTTP 接口返回错误"},
		{name: "platform failure", failure: "platform", want_error: "获取视频列表失败"},
		{name: "BaseResponse failure", failure: "base", want_error: "视频列表 BaseResponse 返回错误"},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			var definition engine.FlowDefinition
			if err := json.Unmarshal(raw, &definition); err != nil {
				t.Fatal(err)
			}
			for node_id, node := range definition.Nodes {
				for _, next_id := range node.NextNodeIDs {
					node.NextNodes = append(node.NextNodes, engine.TargetNode{TargetID: next_id})
				}
				definition.Nodes[node_id] = node
			}
			requests := 0
			downloads := 0
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				requests++
				query := request.URL.Query()
				marker := ""
				if requests == 2 {
					marker = "cursor +/&="
				}
				want_method := strings.ToUpper(strings.TrimSpace(test_case.method))
				if want_method == "" {
					want_method = "GET"
				}
				if requests > 2 || request.Method != want_method || request.URL.Path != "/api/channels/contact/feed/list" || query.Get("username") != "creator@finder" || query.Get("next_marker") != marker || query.Get("keep") != "yes" {
					t.Errorf("unexpected page request: %s %s", request.Method, request.URL)
				}
				if want_method != "GET" && want_method != "HEAD" {
					var body map[string]string
					if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
						t.Errorf("decode POST body: %v", err)
					}
					if len(body) != 2 || body["username"] != "creator@finder" || body["next_marker"] != marker || request.Header.Get("Content-Type") != "application/json" {
						t.Errorf("incorrect JSON request body: %v", body)
					}
				} else if request.ContentLength != 0 {
					t.Error("GET requests must not send a body")
				}
				want_auth := ""
				if test_case.token != "" {
					want_auth = "Bearer " + test_case.token
				}
				if request.Header.Get("Authorization") != want_auth || query.Has("token") {
					t.Error("incorrect token handling")
				}
				if test_case.failure == "http" {
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				if test_case.failure == "api" {
					_, _ = writer.Write([]byte(`{"code":400,"msg":"unavailable"}`))
					return
				}
				if test_case.failure == "platform" {
					_, _ = writer.Write([]byte(`{"code":0,"data":{"errCode":1}}`))
					return
				}
				ret := 0
				if test_case.failure == "base" {
					ret = 1
				}
				objects := "[]"
				if test_case.existing_video {
					objects = `[{"id":"9007199254740993","object_nonce_id":"test-nonce","object_desc":{"description":"existing video","media_type":4,"media":[]}}]`
				}
				video_page := 1
				if test_case.last_page {
					video_page = 2
				}
				if test_case.missing_video && requests == video_page {
					objects = `[{"id":"9007199254740993","objectNonceId":"test-nonce","objectDesc":{"description":"missing video","mediaType":4,"media":[{"url":"https://example.com/video.mp4","mediaType":4,"decodeKey":"12345678901234567890","urlToken":"?token=from-list"}]}}]`
					if test_case.raw_response {
						objects = `[{"id":"9007199254740993","object_nonce_id":"test-nonce","object_desc":{"description":"missing video","media_type":4,"media":[{"url":"https://example.com/video.mp4","media_type":4,"decode_key":"12345678901234567890","url_token":"?token=from-list"}]}}]`
					}
				}
				if test_case.raw_response {
					_, _ = fmt.Fprintf(writer, `{"BaseResponse":{"Ret":%d},"object":%s,"continue_flag":%d,"last_buffer":"cursor +/&="}`, ret, objects, 2-requests)
					return
				}
				_, _ = fmt.Fprintf(writer, `{"code":0,"data":{"errCode":0,"data":{"BaseResponse":{"Ret":%d},"object":%s,"continueFlag":%d,"lastBuffer":"cursor +/&="}}}`, ret, objects, 2-requests)
			}))
			defer server.Close()
			validate := definition.Nodes["validate"]
			validate.ConfigValues = map[string]interface{}{"username": " wxchannels:creator ", "url": " " + server.URL + "/api/channels/contact/feed/list?keep=yes ", "token": " " + test_case.token + " ", "method": test_case.method}
			definition.Nodes["validate"] = validate
			flow_engine := flowengine.NewWorkflowEngine()
			flowengine.RegisterServiceNode(flow_engine, func(_ context.Context, tool_name string, arguments map[string]any) (any, error) {
				if tool_name == "create_download_task" && test_case.missing_video {
					if arguments["platform"] != "wxchannels" || arguments["auto_start"] != true || arguments["config"].(map[string]interface{})["existing_action"] != "skip" {
						return nil, fmt.Errorf("incorrect download arguments: %v", arguments)
					}
					content, err := json.Marshal(arguments["content"])
					if err != nil {
						return nil, err
					}
					// Use the real adapter without a connected scraper or any detail lookup.
					info, err := wxchannelsadapter.NewChannelsAdapter().BuildDownloadTask(content, json.RawMessage(`{}`))
					if err != nil {
						return nil, err
					}
					if info.Content.Id != "wxchannels:9007199254740993" || len(info.Resources) != 1 || len(info.Resources[0].Endpoints) == 0 || !strings.Contains(info.Resources[0].Endpoints[0].URL, "token=from-list") || !strings.Contains(info.Resources[0].Resource.Extra, "12345678901234567890") || info.Account.ExternalId != "creator@finder" {
						return nil, fmt.Errorf("video download metadata was not preserved")
					}
					downloads++
					return map[string]interface{}{"created": true, "skipped": false}, nil
				}
				if tool_name != "check_download_task_contents" {
					return nil, fmt.Errorf("unexpected service call: %s", tool_name)
				}
				ids := arguments["content_ids"].([]interface{})
				if test_case.existing_video || (test_case.missing_video && len(ids) > 0) {
					if len(ids) != 1 || ids[0] != "wxchannels:9007199254740993" {
						return nil, fmt.Errorf("raw video ID was not preserved: %v", ids)
					}
					task_count := 0
					if test_case.existing_video {
						task_count = 1
					}
					return map[string]interface{}{"list": []interface{}{map[string]interface{}{"content_id": ids[0], "exists": test_case.existing_video, "task_count": task_count}}, "total": 1}, nil
				}
				if len(ids) != 0 {
					return nil, fmt.Errorf("unexpected content IDs: %v", ids)
				}
				return map[string]interface{}{"list": []interface{}{}, "total": 0}, nil
			})
			parent := &engine.ProcessContext{Context: context.Background(), EngineRef: flow_engine, ValidateSchemas: true}
			result, err := flow_engine.ExecuteSubflow(parent, definition, nil)
			if test_case.want_error != "" {
				if err == nil || !strings.Contains(err.Error(), test_case.want_error) || requests != 1 {
					t.Fatalf("expected %q after one request, got %v (%d requests)", test_case.want_error, err, requests)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			output := result["output"].(map[string]map[string]interface{})
			summary, _ := output["finish_summary"]["sync_summary"].(map[string]interface{})
			if test_case.existing_video {
				summary, _ = output["summary"]["sync_summary"].(map[string]interface{})
				if requests != 1 || fmt.Sprint(summary["videos"]) != "1" || summary["stop_reason"] != "all_existing" {
					t.Fatalf("raw existing video was not detected: %v (%d requests)", summary, requests)
				}
				return
			}
			if requests != 2 || fmt.Sprint(summary["pages"]) != "2" || summary["stop_reason"] != "no_more_pages" {
				t.Fatalf("incorrect pagination result: %v (%d requests)", summary, requests)
			}
			if test_case.missing_video && (downloads != 1 || fmt.Sprint(summary["submitted"]) != "1") {
				t.Fatalf("missing video was not submitted during the second page call: %d, %v", downloads, summary)
			}
		})
	}
}

func TestCreatorVideoSyncExampleMatchesDocs(t *testing.T) {
	example, err := os.ReadFile("../../../examples/pipelines/01_creator_video_sync.json")
	if err != nil {
		t.Fatal(err)
	}
	documented, err := os.ReadFile("../../../docs/feature/pipelines/01_creator_video_sync.json")
	if err != nil {
		t.Fatal(err)
	}
	var example_definition, documented_definition map[string]interface{}
	if err := json.Unmarshal(example, &example_definition); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(documented, &documented_definition); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(example_definition, documented_definition) {
		t.Fatal("creator video sync example and documented template differ")
	}
}

func TestCreatorVideoSyncDefaultHTTPConfiguration(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/feature/pipelines/01_creator_video_sync.json")
	if err != nil {
		t.Fatal(err)
	}
	var definition engine.FlowDefinition
	if err := json.Unmarshal(raw, &definition); err != nil {
		t.Fatal(err)
	}
	validate := definition.Nodes["validate"]
	validate.ConfigValues = map[string]interface{}{"username": "creator"}
	validate.NextNodeIDs = nil
	definition.Nodes = map[string]engine.NodeDefinition{"validate": validate}
	definition.StartNodeID = "validate"
	flow_engine := flowengine.NewWorkflowEngine()
	result, err := flow_engine.ExecuteSubflow(&engine.ProcessContext{Context: context.Background(), EngineRef: flow_engine}, definition, nil)
	if err != nil {
		t.Fatal(err)
	}
	output := result["output"].(map[string]map[string]interface{})
	creator := output["validate"]["creator"].(map[string]interface{})
	authentication := creator["authentication"].(map[string]interface{})
	if creator["url"] != "http://127.0.0.1:2022/api/channels/contact/feed/list" || creator["method"] != "GET" || authentication["type"] != "none" || authentication["token"] != "" {
		t.Fatalf("incorrect HTTP defaults: %v", creator)
	}
	validate.ConfigValues["method"] = "TRACE"
	definition.Nodes["validate"] = validate
	if _, err := flow_engine.ExecuteSubflow(&engine.ProcessContext{Context: context.Background(), EngineRef: flow_engine}, definition, nil); err == nil || !strings.Contains(err.Error(), "method 不支持") {
		t.Fatalf("unsupported HTTP method was accepted: %v", err)
	}
}
