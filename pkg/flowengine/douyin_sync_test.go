package flowengine_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"wx_channel/pkg/flowengine"
	"wx_channel/pkg/flowengine/engine"
	"wx_channel/pkg/flowengine/nodes"
)

// stub_douyin_list_node emulates the built-in DouyinFetchListNode for the
// pipeline test. The real node lives in internal/services (which pkg must not
// import) and calls the douyin adapter directly; here each invocation consumes
// the next scripted page and returns the same normalized payload shape.
type stub_douyin_list_node struct {
	t        *testing.T
	id       string
	config   map[string]interface{}
	pages    []string
	requests *int
}

func (n *stub_douyin_list_node) ID() string   { return n.id }
func (n *stub_douyin_list_node) Type() string { return "DouyinFetchListNode" }

func (n *stub_douyin_list_node) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	output, err := n.next_page(ctx)
	if err != nil {
		return false, nil, err
	}
	output_key, _ := n.config["output_key"].(string)
	if output_key == "" {
		output_key = "list"
	}
	ctx.SetOutput(n.id, output_key, output)
	return true, ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.id), nil
}

func (n *stub_douyin_list_node) next_page(ctx *engine.ProcessContext) (map[string]any, error) {
	index := *n.requests
	if index >= len(n.pages) {
		return nil, fmt.Errorf("requested a page after scanning should have stopped")
	}
	var page map[string]any
	if err := json.Unmarshal([]byte(n.pages[index]), &page); err != nil {
		return nil, err
	}

	// The pipeline threads the previous page's next_marker into the node's page
	// config; assert it reached us unchanged.
	expected_cursor := ""
	if index > 0 {
		var previous map[string]any
		if err := json.Unmarshal([]byte(n.pages[index-1]), &previous); err == nil {
			expected_cursor, _ = previous["next_marker"].(string)
		}
	}
	actual_cursor, err := n.resolved_config_string(ctx, "page")
	if err != nil {
		return nil, err
	}
	if actual_cursor != expected_cursor {
		n.t.Errorf("cursor was not forwarded: got %q want %q", actual_cursor, expected_cursor)
	}

	*n.requests = index + 1

	if _, has_code := page["code"]; has_code {
		message, _ := page["msg"].(string)
		if message == "" {
			message = "列表接口返回错误"
		}
		return nil, fmt.Errorf("%s", message)
	}

	expected_uid, err := n.resolved_config_string(ctx, "profile")
	if err != nil {
		return nil, err
	}
	if owner, exists := page["owner_id"]; exists {
		owner_id, _ := owner.(string)
		if owner_id != expected_uid {
			return nil, fmt.Errorf("抖音列表响应账号或 scope 不匹配")
		}
	}

	items, ok := page["items"].([]any)
	if !ok {
		return nil, fmt.Errorf("抖音列表响应缺少 data.items")
	}

	next_marker := ""
	if raw, exists := page["next_marker"]; exists {
		marker, is_string := raw.(string)
		if !is_string {
			return nil, fmt.Errorf("抖音分页游标必须为字符串")
		}
		next_marker = marker
	}

	list := make([]any, 0, len(items))
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		external_id, is_string := item["id"].(string)
		if !is_string {
			return nil, fmt.Errorf("抖音视频 ID 必须为数字字符串")
		}
		content_type := "video"
		switch item["type"] {
		case "video":
		case "note":
			content_type = "album"
		default:
			return nil, fmt.Errorf("抖音作品类型无效")
		}
		metadata, exists := item["metadata"]
		if !exists {
			aweme := map[string]any{
				"aweme_id": external_id,
				"video": map[string]any{"play_addr": map[string]any{
					"url_list": []string{"https://media.example/" + external_id + ".mp4"},
				}},
			}
			if content_type == "album" {
				delete(aweme, "video")
				aweme["images"] = []any{
					map[string]any{"url_list": []string{"https://media.example/" + external_id + "_1.jpg"}},
					map[string]any{"url_list": []string{"https://media.example/" + external_id + "_2.jpg"}},
				}
			}
			raw, err := json.Marshal(aweme)
			if err != nil {
				return nil, err
			}
			metadata = string(raw)
		}
		list = append(list, map[string]any{
			"id":          "douyin:" + external_id,
			"platform_id": "douyin",
			"type":        content_type,
			"external_id": external_id,
			"title":       "",
			"url":         "https://www.douyin.com/video/" + external_id,
			"metadata":    metadata,
		})
	}

	return map[string]any{
		"platform":    "douyin",
		"scope":       "posts",
		"total":       len(list),
		"list":        list,
		"next_marker": next_marker,
	}, nil
}

func (n *stub_douyin_list_node) resolved_config_string(ctx *engine.ProcessContext, key string) (string, error) {
	raw, exists := n.config[key]
	if !exists || raw == nil {
		return "", nil
	}
	ctx.Mu.Lock()
	value, err := nodes.ResolveTemplates(raw, ctx)
	ctx.Mu.Unlock()
	if err != nil {
		return "", err
	}
	if value == nil {
		return "", nil
	}
	return fmt.Sprint(value), nil
}

func TestDouyinCreatorSync(t *testing.T) {
	for _, test_case := range []struct {
		name             string
		pages            []string
		want_downloads   []string
		want_error       string
		sec_uid          string
		max_pages        int
		broken_checks    bool
		download_failure bool
	}{
		{name: "pagination with videos albums and existing works", pages: []string{
			`{"items":[{"id":"1","type":"video"},{"id":"1","type":"video"},{"id":"8","type":"note"}],"next_marker":"10"}`,
			`{"items":[{"id":"1","type":"video"},{"id":"2","type":"video"}],"next_marker":"20"}`,
			`{"items":[{"id":"3","type":"video"}]}`,
		}, want_downloads: []string{"1", "8", "2"}},
		{name: "last page flushes pending downloads", pages: []string{`{"items":[{"id":"1","type":"video","url":"https://untrusted.example/1"}]}`}, want_downloads: []string{"1"}},
		{name: "empty pages continue and album pages download", pages: []string{
			`{"items":[],"next_marker":"10"}`, `{"items":[{"id":"8","type":"note"}],"next_marker":"20"}`, `{"items":[{"id":"2","type":"video"}]}`,
		}, want_downloads: []string{"8", "2"}},
		{name: "existing video page continues to missing older albums", pages: []string{`{"items":[{"id":"3","type":"video"}],"next_marker":"10"}`, `{"items":[{"id":"8","type":"note"}]}`}, want_downloads: []string{"8"}},
		{name: "last page flushes albums and removes duplicates", pages: []string{`{"items":[{"id":"8","type":"note"},{"id":"8","type":"note"}]}`}, want_downloads: []string{"8"}},
		{name: "existing album is skipped", pages: []string{`{"items":[{"id":"3","type":"note"}]}`}},
		{name: "repeated cursor", pages: []string{`{"items":[],"next_marker":"10"}`, `{"items":[],"next_marker":"10"}`}, want_error: "分页游标缺失或重复"},
		{name: "business error", pages: []string{`{"code":400,"msg":"Cookie 已失效"}`}, want_error: "Cookie 已失效"},
		{name: "missing items", pages: []string{`{}`}, want_error: "data.items"},
		{name: "invalid metadata", pages: []string{`{"items":[{"id":"1","type":"video","metadata":"broken"}]}`}, want_error: "元数据不是有效 JSON"},
		{name: "missing media", pages: []string{`{"items":[{"id":"1","type":"video","metadata":"{\"aweme_id\":\"1\"}"}]}`}, want_error: "缺少匹配的视频媒体数据"},
		{name: "wrong media ID", pages: []string{`{"items":[{"id":"1","type":"video","metadata":"{\"aweme_id\":\"2\"}"}]}`}, want_error: "元数据与作品 ID 不匹配"},
		{name: "album missing images", pages: []string{`{"items":[{"id":"8","type":"note","metadata":"{\"aweme_id\":\"8\",\"images\":[]}"}]}`}, want_error: "图集缺少图片媒体数据"},
		{name: "numeric video id rejected", pages: []string{`{"items":[{"id":9007199254740993,"type":"video"}]}`}, want_error: "ID 必须为数字字符串"},
		{name: "numeric cursor rejected", pages: []string{`{"items":[],"next_marker":10}`}, want_error: "分页游标必须为字符串"},
		{name: "wrong account", pages: []string{`{"owner_id":"another","items":[]}`}, want_error: "账号或 scope 不匹配"},
		{name: "invalid account", sec_uid: "https://www.douyin.com/user/test", want_error: "请填写抖音主页"},
		{name: "page limit", pages: []string{`{"items":[],"next_marker":"10"}`}, max_pages: 1, want_error: "达到 max_pages"},
		{name: "incomplete checks", pages: []string{`{"items":[{"id":"1","type":"video"}]}`}, broken_checks: true, want_error: "批量作品存在性检查结果不完整"},
		{name: "download failure propagates", pages: []string{`{"items":[{"id":"1","type":"video"}]}`}, download_failure: true, want_error: "下载测试失败"},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			raw, err := os.ReadFile("../../docs/examples/pipelines/01_douyin_creator_video_sync.json")
			if err != nil {
				t.Fatal(err)
			}
			var definition engine.FlowDefinition
			if err := json.Unmarshal(raw, &definition); err != nil {
				t.Fatal(err)
			}
			if err := nodes.ValidateFlowScopes(definition); err != nil {
				t.Fatal(err)
			}
			requests := 0
			validate := definition.Nodes["validate"]
			sec_uid := " douyin:test_uid "
			if test_case.sec_uid != "" {
				sec_uid = test_case.sec_uid
			}
			validate.ConfigValues = map[string]any{"sec_uid": sec_uid}
			definition.Nodes["validate"] = validate
			if test_case.max_pages > 0 {
				definition.Nodes["config"].Config["variables"].(map[string]any)["max_pages"] = test_case.max_pages
			}
			flow_engine := flowengine.NewWorkflowEngine()
			flow_engine.RegisterNode("DouyinFetchListNode", func(config map[string]interface{}) engine.Node {
				id, _ := config["id"].(string)
				return &stub_douyin_list_node{t: t, id: id, config: config, pages: test_case.pages, requests: &requests}
			})
			existing := map[string]bool{"douyin:3": true}
			var downloads []string
			flowengine.RegisterServiceNode(flow_engine, func(ctx context.Context, tool string, arguments map[string]any) (any, error) {
				switch tool {
				case "check_download_task_contents":
					raw_ids, _ := json.Marshal(arguments["content_ids"])
					var content_ids []string
					if err := json.Unmarshal(raw_ids, &content_ids); err != nil {
						return nil, err
					}
					list := []any{}
					for _, content_id := range content_ids {
						if !strings.HasPrefix(content_id, "douyin:") {
							return nil, fmt.Errorf("wrong platform ID: %s", content_id)
						}
						count := 0
						if existing[content_id] {
							count = 1
						}
						list = append(list, map[string]any{"content_id": content_id, "exists": count > 0, "task_count": count})
					}
					if test_case.broken_checks {
						list = nil
					}
					return map[string]any{"list": list, "total": len(content_ids)}, nil
				case "create_download_task":
					if test_case.download_failure {
						return nil, fmt.Errorf("下载测试失败")
					}
					config, _ := arguments["config"].(map[string]any)
					if arguments["platform"] != "douyin" || arguments["build_from_fetch"] != true || config["existing_action"] != "skip" {
						return nil, fmt.Errorf("invalid download arguments: %v", arguments)
					}
					raw_content, err := json.Marshal(arguments["content"])
					if err != nil {
						return nil, err
					}
					var content struct {
						AwemeDetail struct {
							ID    string `json:"aweme_id"`
							Video struct {
								PlayAddr struct {
									URLs []string `json:"url_list"`
								} `json:"play_addr"`
							} `json:"video"`
							Images []struct {
								URLs []string `json:"url_list"`
							} `json:"images"`
						} `json:"aweme_detail"`
					}
					if err := json.Unmarshal(raw_content, &content); err != nil {
						return nil, err
					}
					video_id := content.AwemeDetail.ID
					if len(content.AwemeDetail.Images) > 0 {
						if len(content.AwemeDetail.Images) != 2 || len(content.AwemeDetail.Images[0].URLs) != 1 || len(content.AwemeDetail.Images[1].URLs) != 1 || content.AwemeDetail.Images[0].URLs[0] != "https://media.example/"+video_id+"_1.jpg" || content.AwemeDetail.Images[1].URLs[0] != "https://media.example/"+video_id+"_2.jpg" {
							return nil, fmt.Errorf("album download must retain all list images")
						}
					} else if video_id == "" || len(content.AwemeDetail.Video.PlayAddr.URLs) != 1 || content.AwemeDetail.Video.PlayAddr.URLs[0] != "https://media.example/"+video_id+".mp4" {
						return nil, fmt.Errorf("video download must reuse the list media data")
					}
					downloads = append(downloads, video_id)
					existing["douyin:"+video_id] = true
					return map[string]any{"created": true, "skipped": false}, nil
				default:
					return nil, fmt.Errorf("unexpected tool: %s", tool)
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			run_id, err := flow_engine.StartFlowWithOptions(definition.ID, nil, engine.StartFlowOptions{Definition: &definition, Context: ctx, ValidateSchemas: true})
			if test_case.want_error != "" {
				if err == nil || !strings.Contains(err.Error(), test_case.want_error) {
					t.Fatalf("want error %q, got %v", test_case.want_error, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if requests != len(test_case.pages) || !reflect.DeepEqual(downloads, test_case.want_downloads) {
				t.Fatalf("requests=%d, downloads=%v; want %d, %v", requests, downloads, len(test_case.pages), test_case.want_downloads)
			}
			record, ok := flow_engine.GetRunSnapshot(run_id)
			if !ok || record.Status != engine.RunStatusCompleted {
				t.Fatalf("run did not complete: %+v", record)
			}
			if test_case.name == "pagination with videos albums and existing works" {
				raw_summary, err := json.Marshal(record.NodeOutputs)
				if err != nil {
					t.Fatal(err)
				}
				var output struct {
					Summary struct {
						Works  int `json:"works"`
						Videos int `json:"videos"`
						Albums int `json:"albums"`
					} `json:"sync_summary"`
				}
				if err := json.Unmarshal(raw_summary, &output); err != nil {
					t.Fatal(err)
				}
				if output.Summary.Works != 4 || output.Summary.Videos != 3 || output.Summary.Albums != 1 {
					t.Fatalf("incorrect mixed content counts: %+v", output.Summary)
				}
			}
		})
	}
}
