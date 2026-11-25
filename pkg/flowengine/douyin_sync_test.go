package flowengine_test

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
	"time"

	"wx_channel/pkg/flowengine"
	"wx_channel/pkg/flowengine/engine"
	"wx_channel/pkg/flowengine/nodes"
)

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
		{name: "pagination and existing page", pages: []string{
			`{"items":[{"id":"1","type":"video"},{"id":"1","type":"video"},{"id":"8","type":"note"}],"next_marker":"10"}`,
			`{"items":[{"id":"1","type":"video"},{"id":"2","type":"video"}],"next_marker":"20"}`,
			`{"items":[{"id":"3","type":"video"}],"next_marker":"30"}`,
		}, want_downloads: []string{"1", "2"}},
		{name: "last page flushes pending downloads", pages: []string{`{"items":[{"id":"1","type":"video","url":"https://untrusted.example/1"}]}`}, want_downloads: []string{"1"}},
		{name: "empty and album pages continue", pages: []string{
			`{"items":[],"next_marker":"10"}`, `{"items":[{"id":"8","type":"note"}],"next_marker":"20"}`, `{"items":[{"id":"2","type":"video"}]}`,
		}, want_downloads: []string{"2"}},
		{name: "all existing stops immediately", pages: []string{`{"items":[{"id":"3","type":"video"}],"next_marker":"10"}`}},
		{name: "repeated cursor", pages: []string{`{"items":[],"next_marker":"10"}`, `{"items":[],"next_marker":"10"}`}, want_error: "分页游标缺失或重复"},
		{name: "business error", pages: []string{`{"code":400,"msg":"Cookie 已失效"}`}, want_error: "Cookie 已失效"},
		{name: "missing items", pages: []string{`{}`}, want_error: "data.items"},
		{name: "numeric video id rejected", pages: []string{`{"items":[{"id":9007199254740993,"type":"video"}]}`}, want_error: "ID 必须为数字字符串"},
		{name: "numeric cursor rejected", pages: []string{`{"items":[],"next_marker":10}`}, want_error: "分页游标必须为字符串"},
		{name: "wrong account", pages: []string{`{"owner_id":"another","items":[]}`}, want_error: "账号或 scope 不匹配"},
		{name: "invalid account", sec_uid: "https://www.douyin.com/user/test", want_error: "请填写抖音主页"},
		{name: "page limit", pages: []string{`{"items":[],"next_marker":"10"}`}, max_pages: 1, want_error: "达到 max_pages"},
		{name: "incomplete checks", pages: []string{`{"items":[{"id":"1","type":"video"}]}`}, broken_checks: true, want_error: "批量视频存在性检查结果不完整"},
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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Query().Get("sec_uid") != "test_uid" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("list authentication was not forwarded")
				}
				if requests >= len(test_case.pages) {
					t.Error("requested a page after scanning should have stopped")
					http.Error(w, "unexpected page", 500)
					return
				}
				expected_cursor := ""
				if requests > 0 {
					var previous map[string]any
					_ = json.Unmarshal([]byte(test_case.pages[requests-1]), &previous)
					expected_cursor, _ = previous["next_marker"].(string)
				}
				if r.URL.Query().Get("next_marker") != expected_cursor {
					t.Errorf("cursor was not forwarded: %s", r.URL)
				}
				var page map[string]any
				if err := json.Unmarshal([]byte(test_case.pages[requests]), &page); err != nil {
					t.Error(err)
				}
				requests++
				if _, exists := page["code"]; exists {
					_ = json.NewEncoder(w).Encode(page)
					return
				}
				if _, exists := page["owner_id"]; !exists {
					page["owner_id"] = "test_uid"
				}
				page["scope"] = "posts"
				_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": page})
			}))
			defer server.Close()
			validate := definition.Nodes["validate"]
			sec_uid := " douyin:test_uid "
			if test_case.sec_uid != "" {
				sec_uid = test_case.sec_uid
			}
			validate.ConfigValues = map[string]any{"sec_uid": sec_uid, "url": server.URL, "token": "test-token"}
			definition.Nodes["validate"] = validate
			if test_case.max_pages > 0 {
				definition.Nodes["config"].Config["variables"].(map[string]any)["max_pages"] = test_case.max_pages
			}
			flow_engine := flowengine.NewWorkflowEngine()
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
				case "download_content":
					if test_case.download_failure {
						return nil, fmt.Errorf("下载测试失败")
					}
					video_url, _ := arguments["url"].(string)
					if !strings.HasPrefix(video_url, "https://www.douyin.com/video/") || arguments["existing_action"] != "skip" || arguments["wait_for_completion"] != false {
						return nil, fmt.Errorf("invalid download arguments: %v", arguments)
					}
					video_id := strings.TrimPrefix(video_url, "https://www.douyin.com/video/")
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
		})
	}
}
