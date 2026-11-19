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

	"github.com/dop251/goja"

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
		{name: "raw existing video", method: "POST", existing_video: true},
		{name: "missing video without client socket", missing_video: true},
		{name: "last page missing video without client socket", missing_video: true, last_page: true},
		{name: "HTTP failure", failure: "http", want_error: "HTTP request returned status 401"},
		{name: "API failure", failure: "api", want_error: "视频列表响应缺少 object"},
		{name: "platform failure", failure: "platform", want_error: "视频列表响应缺少 object"},
		{name: "service response rejected", failure: "service", want_error: "视频列表响应缺少 object"},
		{name: "camel pagination rejected", failure: "camel", want_error: "视频列表缺少有效 continue_flag"},
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
					objects = `[{"id":"9007199254740993","object_nonce_id":"test-nonce","createtime":1788086792,"object_desc":{"description":"missing video","media_type":4,"media":[{"url":"https://example.com/video.mp4","media_type":4,"decode_key":"12345678901234567890","url_token":"?token=from-list","thumb_url":"https://example.com/thumb.jpg","cover_url":"https://example.com/cover.jpg","video_play_len":7,"file_size":9818110,"width":1080,"height":1920,"spec":[{"file_format":"xWT111","duration_ms":7100,"video_bitrate":382,"level_order":100,"width":1080,"height":1920},{"file_format":"xWT112","duration_ms":7084,"video_bitrate":212,"level_order":200,"width":576,"height":1024}],"scaling_info":{"up_percent_position":0.059085141867399216}}]}}]`
				}
				if test_case.failure == "service" {
					_, _ = writer.Write([]byte(`{"errCode":0,"data":{"BaseResponse":{"Ret":0},"object":[],"continueFlag":0,"lastBuffer":""}}`))
					return
				}
				if test_case.failure == "camel" {
					_, _ = writer.Write([]byte(`{"BaseResponse":{"Ret":0},"object":[],"continueFlag":0,"lastBuffer":""}`))
					return
				}
				_, _ = fmt.Fprintf(writer, `{"BaseResponse":{"Ret":%d},"object":%s,"continue_flag":%d,"last_buffer":"cursor +/&="}`, ret, objects, 2-requests)
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
					var object wxchannelsadapter.ChannelsObject
					if err := json.Unmarshal(content, &object); err != nil {
						return nil, err
					}
					media := object.ObjectDesc.Media[0]
					if object.CreateTime != 1788086792 || media.FileSize != 9818110 || media.VideoPlayLen != 7 || media.ThumbUrl != "https://example.com/thumb.jpg" || media.CoverUrl != "https://example.com/cover.jpg" || len(media.Spec) != 2 || media.Spec[0].DurationMs != 7100 || media.Spec[0].VideoBitrate != 382 || media.Spec[1].Width != 576 || wxchannelsadapter.PickHighestSpec(&object) != "xWT111" {
						return nil, fmt.Errorf("raw video media and quality metadata was not preserved: %+v", media)
					}
					// Use the real adapter without a connected scraper or any detail lookup.
					info, err := wxchannelsadapter.NewChannelsAdapter().BuildDownloadTask(content, json.RawMessage(`{}`))
					if err != nil {
						return nil, err
					}
					if info.Content.Id != "wxchannels:9007199254740993" || len(info.Resources) != 1 || len(info.Resources[0].Endpoints) == 0 || !strings.Contains(info.Resources[0].Endpoints[0].URL, "token=from-list") || !strings.Contains(info.Resources[0].Resource.Extra, "12345678901234567890") || info.Account.ExternalId != "creator@finder" {
						return nil, fmt.Errorf("video download metadata was not preserved")
					}
					if !strings.Contains(info.Resources[0].Endpoints[0].URL, "X-snsvideoflag=xWT111") {
						return nil, fmt.Errorf("raw video quality was not selected")
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

func TestCreatorVideoSyncServicePagination(t *testing.T) {
	raw, err := os.ReadFile("../../../examples/pipelines/01_creator_video_sync_service.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, existing_video := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing=%t", existing_video), func(t *testing.T) {
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
			validate := definition.Nodes["validate"]
			validate.ConfigValues = map[string]interface{}{"username": " wxchannels:creator "}
			definition.Nodes["validate"] = validate
			requests, downloads, checks := 0, 0, 0
			flow_engine := flowengine.NewWorkflowEngine()
			flowengine.RegisterServiceNode(flow_engine, func(_ context.Context, tool_name string, arguments map[string]any) (any, error) {
				switch tool_name {
				case "get_wxchannels_account_videos":
					requests++
					marker := ""
					if requests == 2 {
						marker = "next-page"
					}
					if requests > 2 || len(arguments) != 2 || arguments["username"] != "creator@finder" || arguments["next_marker"] != marker {
						return nil, fmt.Errorf("incorrect service list arguments: %v", arguments)
					}
					var response map[string]interface{}
					err := json.Unmarshal([]byte(fmt.Sprintf(`{"errCode":0,"data":{"BaseResponse":{"Ret":0},"object":[{"id":"9007199254740993","objectNonceId":"test-nonce","objectDesc":{"description":"video","mediaType":4,"media":[{"url":"https://example.com/video.mp4","mediaType":4,"decodeKey":"123","urlToken":"?token=from-list"}]}}],"continueFlag":%d,"lastBuffer":"next-page"}}`, 2-requests)), &response)
					if err == nil && requests == 2 {
						response["data"].(map[string]interface{})["object"] = []interface{}{}
					}
					return response, err
				case "check_download_task_contents":
					checks++
					ids := arguments["content_ids"].([]interface{})
					if len(ids) == 0 && requests == 2 {
						return map[string]interface{}{"list": []interface{}{}, "total": 0}, nil
					}
					if len(ids) != 1 || ids[0] != "wxchannels:9007199254740993" {
						return nil, fmt.Errorf("incorrect service video IDs: %v", ids)
					}
					count := 0
					if existing_video {
						count = 1
					}
					return map[string]interface{}{"list": []interface{}{map[string]interface{}{"content_id": ids[0], "exists": existing_video, "task_count": count}}, "total": 1}, nil
				case "create_download_task":
					content, err := json.Marshal(arguments["content"])
					if err != nil {
						return nil, err
					}
					info, err := wxchannelsadapter.NewChannelsAdapter().BuildDownloadTask(content, json.RawMessage(`{}`))
					if err != nil {
						return nil, err
					}
					if info.Content.Id != "wxchannels:9007199254740993" || info.Account.ExternalId != "creator@finder" || !strings.Contains(info.Resources[0].Endpoints[0].URL, "token=from-list") {
						return nil, fmt.Errorf("service list download metadata was not preserved")
					}
					downloads++
					return map[string]interface{}{"created": true, "skipped": false}, nil
				default:
					return nil, fmt.Errorf("unexpected service call: %s", tool_name)
				}
			})
			result, err := flow_engine.ExecuteSubflow(&engine.ProcessContext{Context: context.Background(), EngineRef: flow_engine, ValidateSchemas: true}, definition, nil)
			if err != nil {
				t.Fatal(err)
			}
			output := result["output"].(map[string]map[string]interface{})
			if existing_video {
				summary := output["summary"]["sync_summary"].(map[string]interface{})
				if requests != 1 || checks != 1 || downloads != 0 || summary["stop_reason"] != "all_existing" {
					t.Fatalf("existing page did not stop service sync: requests=%d checks=%d downloads=%d summary=%v", requests, checks, downloads, summary)
				}
			} else {
				summary := output["finish_summary"]["sync_summary"].(map[string]interface{})
				if requests != 2 || checks != 2 || downloads != 1 || fmt.Sprint(summary["submitted"]) != "1" || summary["stop_reason"] != "no_more_pages" {
					t.Fatalf("incorrect service pagination or download count: requests=%d checks=%d downloads=%d summary=%v", requests, checks, downloads, summary)
				}
			}
		})
	}
}

func TestCreatorVideoSyncServiceMatchesHTTP(t *testing.T) {
	definitions := make([]map[string]interface{}, 2)
	for index, filename := range []string{"01_creator_video_sync.json", "01_creator_video_sync_service.json"} {
		raw, err := os.ReadFile("../../../examples/pipelines/" + filename)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &definitions[index]); err != nil {
			t.Fatal(err)
		}
	}
	http_definition, service_definition := definitions[0], definitions[1]
	subflows := service_definition["subflows"].(map[string]interface{})
	page_config := service_definition["nodes"].(map[string]interface{})["fetch_page"].(map[string]interface{})["config"].(map[string]interface{})
	page_workflow := subflows[page_config["flow_id"].(string)].(map[string]interface{})
	page_config["workflow"] = page_workflow
	delete(page_config, "flow_id")
	for _, node_id := range []string{"download_missing", "fetch_list"} {
		config := page_workflow["nodes"].(map[string]interface{})[node_id].(map[string]interface{})["config"].(map[string]interface{})
		config["workflow"] = subflows[config["flow_id"].(string)]
		delete(config, "flow_id")
	}
	delete(service_definition, "subflows")
	service_definition["id"] = http_definition["id"]
	service_definition["name"] = http_definition["name"]
	http_nodes := http_definition["nodes"].(map[string]interface{})
	service_nodes := service_definition["nodes"].(map[string]interface{})
	if len(service_nodes["validate"].(map[string]interface{})["config_schema"].([]interface{})) != 2 {
		t.Fatal("service configuration must only contain username and remark")
	}
	service_nodes["validate"] = http_nodes["validate"]
	service_nodes["parse_page"] = http_nodes["parse_page"]
	for _, nodes := range []map[string]interface{}{http_nodes, service_nodes} {
		page := nodes["fetch_page"].(map[string]interface{})["config"].(map[string]interface{})
		page_workflow := page["workflow"].(map[string]interface{})
		list := page_workflow["nodes"].(map[string]interface{})["fetch_list"].(map[string]interface{})["config"].(map[string]interface{})
		for _, config := range []map[string]interface{}{page, list} {
			arguments := config["arguments"].(map[string]interface{})
			for _, key := range []string{"url", "method", "authentication"} {
				delete(arguments, key)
			}
			workflow := config["workflow"].(map[string]interface{})
			schema := []interface{}{}
			for _, field := range workflow["context_schema"].([]interface{}) {
				key := field.(map[string]interface{})["key"]
				if key != "url" && key != "method" && key != "authentication" {
					schema = append(schema, field)
				}
			}
			workflow["context_schema"] = schema
		}
		list_workflow := list["workflow"].(map[string]interface{})
		fetch := list_workflow["nodes"].(map[string]interface{})["fetch_list"].(map[string]interface{})
		delete(fetch, "type")
		delete(fetch, "config")
	}
	if !reflect.DeepEqual(http_definition, service_definition) {
		t.Fatal("HTTP and service sync differ outside list fetching, response parsing and request configuration")
	}
}

func TestCreatorVideoSyncRejectsOtherResponseFormats(t *testing.T) {
	for _, test_case := range []struct {
		filename string
		response string
	}{
		{"01_creator_video_sync.json", `{"errCode":0,"data":{"BaseResponse":{"Ret":0},"object":[],"continueFlag":0}}`},
		{"01_creator_video_sync.json", `{"BaseResponse":{"Ret":0},"object":[{"id":"1","objectNonceId":"nonce","objectDesc":{"mediaType":4,"media":[]}}],"continue_flag":0}`},
		{"01_creator_video_sync_service.json", `{"BaseResponse":{"Ret":0},"object":[],"continue_flag":0}`},
		{"01_creator_video_sync_service.json", `{"errCode":0,"data":{"BaseResponse":{"Ret":0},"object":[],"continue_flag":0}}`},
		{"01_creator_video_sync_service.json", `{"errCode":0,"data":{"BaseResponse":{"Ret":0},"object":[{"id":"1","object_nonce_id":"nonce","object_desc":{"media_type":4,"media":[]}}],"continueFlag":0}}`},
	} {
		raw, err := os.ReadFile("../../../examples/pipelines/" + test_case.filename)
		if err != nil {
			t.Fatal(err)
		}
		var definition engine.FlowDefinition
		if err := json.Unmarshal(raw, &definition); err != nil {
			t.Fatal(err)
		}
		var response map[string]interface{}
		if err := json.Unmarshal([]byte(test_case.response), &response); err != nil {
			t.Fatal(err)
		}
		global := definition.Nodes["config"].Config["variables"].(map[string]interface{})
		global["page_response"] = response
		runtime := goja.New()
		if err := runtime.Set("global", global); err != nil {
			t.Fatal(err)
		}
		if _, err := runtime.RunString(definition.Nodes["parse_page"].Config["code"].(string)); err == nil {
			t.Errorf("%s accepted another response format: %s", test_case.filename, test_case.response)
		}
	}
}
