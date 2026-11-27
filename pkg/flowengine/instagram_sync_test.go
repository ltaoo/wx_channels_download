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

type stub_instagram_sync_node struct {
	id        string
	node_type string
	config    map[string]any
	fetch     func(map[string]any) (any, error)
}

func (n *stub_instagram_sync_node) ID() string   { return n.id }
func (n *stub_instagram_sync_node) Type() string { return n.node_type }
func (n *stub_instagram_sync_node) Execute(ctx *engine.ProcessContext) (bool, []string, error) {
	ctx.Mu.Lock()
	resolved, err := nodes.ResolveTemplates(n.config, ctx)
	ctx.Mu.Unlock()
	if err != nil {
		return false, nil, err
	}
	config := resolved.(map[string]any)
	output, err := n.fetch(config)
	if err != nil {
		return false, nil, err
	}
	ctx.SetOutput(n.id, config["output_key"].(string), output)
	return true, ctx.EngineRef.GetNextNodeIDsFromDefinition(ctx, n.id), nil
}

func instagram_sync_post(id string, content_type string) map[string]any {
	return map[string]any{
		"id": "instagram:" + id, "platform_id": "instagram", "external_id": id,
		"external_id2": "post_" + id, "type": content_type,
		"source_url": "https://www.instagram.com/p/post_" + id + "/",
	}
}

func TestInstagramCreatorSync(t *testing.T) {
	for _, test_case := range []struct {
		name             string
		posts            []map[string]any
		pages            [][]map[string]any
		next_markers     []any
		max_pages        int
		max_posts        int
		username         string
		want_downloads   []string
		want_error       string
		broken_checks    bool
		mismatch_detail  bool
		download_failure bool
		race_skip        bool
	}{
		{name: "videos albums photos duplicates and existing posts", posts: []map[string]any{
			instagram_sync_post("9007199254740993", "video"), instagram_sync_post("8", "album"),
			instagram_sync_post("9", "album"), instagram_sync_post("3", "video"), instagram_sync_post("8", "album"),
		}, want_downloads: []string{"9007199254740993", "8", "9"}},
		{name: "empty page", posts: []map[string]any{}},
		{name: "existing post needs no detail", posts: []map[string]any{instagram_sync_post("3", "album")}},
		{name: "concurrent creation is skipped", posts: []map[string]any{instagram_sync_post("8", "album")}, want_downloads: []string{"8"}, race_skip: true},
		{name: "invalid username", username: "https://www.instagram.com/p/post_1/", want_error: "有效的 HTTPS 主页链接"},
		{name: "incomplete existence checks", posts: []map[string]any{instagram_sync_post("8", "album")}, broken_checks: true, want_error: "存在性检查结果不完整"},
		{name: "detail belongs to another post", posts: []map[string]any{instagram_sync_post("8", "album")}, mismatch_detail: true, want_error: "详情与待下载帖子不匹配"},
		{name: "download failure propagates", posts: []map[string]any{instagram_sync_post("8", "album")}, download_failure: true, want_error: "下载测试失败"},
		{name: "pagination deduplicates across pages", pages: [][]map[string]any{
			{instagram_sync_post("3", "video")}, {}, {instagram_sync_post("8", "album"), instagram_sync_post("3", "video")},
			{instagram_sync_post("8", "album"), instagram_sync_post("9", "album")},
		}, next_markers: []any{"opaque+/cursor==", "page_3", "page_4", ""}, want_downloads: []string{"8", "9"}},
		{name: "repeated cursor", pages: [][]map[string]any{{}, {}}, next_markers: []any{"cursor", "cursor"}, want_error: "分页游标重复"},
		{name: "numeric cursor", posts: []map[string]any{}, next_markers: []any{123}, want_error: "分页游标必须为字符串"},
		{name: "page limit", posts: []map[string]any{}, next_markers: []any{"next"}, max_pages: 1, want_error: "达到 max_pages"},
		{name: "post limit", posts: []map[string]any{instagram_sync_post("8", "album"), instagram_sync_post("9", "album")}, max_posts: 1, want_error: "超过 max_posts"},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			raw, err := os.ReadFile("../../docs/examples/pipelines/01_instagram_creator_video_sync.json")
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
			username := " https://www.instagram.com/Example.Creator/?hl=en "
			if test_case.username != "" {
				username = test_case.username
			}
			validate := definition.Nodes["validate"]
			validate.ConfigValues = map[string]any{"username": username}
			definition.Nodes["validate"] = validate
			definition.Nodes["config"].Config["variables"].(map[string]any)["download_dir"] = "/downloads/instagram"
			if test_case.max_pages > 0 {
				definition.Nodes["config"].Config["variables"].(map[string]any)["max_pages"] = test_case.max_pages
			}
			if test_case.max_posts > 0 {
				definition.Nodes["config"].Config["variables"].(map[string]any)["max_posts"] = test_case.max_posts
			}
			pages := test_case.pages
			if pages == nil {
				pages = [][]map[string]any{test_case.posts}
			}

			flow_engine := flowengine.NewWorkflowEngine()
			list_requests := 0
			var detail_requests, downloads []string
			media_by_id := map[string][]any{
				"9007199254740993": {map[string]any{"id": "video", "type": "video", "url": "https://media.example/fresh.mp4"}},
				"8": {map[string]any{"id": "image", "type": "image", "url": "https://media.example/1.jpg"},
					map[string]any{"id": "video", "type": "video", "url": "https://media.example/2.mp4"}},
				"9": {map[string]any{"id": "photo", "type": "image", "url": "https://media.example/photo.jpg"}},
			}
			for node_type, fetch := range map[string]func(map[string]any) (any, error){
				"InstagramFetchListNode": func(config map[string]any) (any, error) {
					page_index := list_requests
					list_requests++
					if page_index >= len(pages) {
						return nil, fmt.Errorf("unexpected page request: %v", config)
					}
					var expected_cursor any = ""
					if page_index > 0 {
						expected_cursor = test_case.next_markers[page_index-1]
					}
					if config["profile"] != "example.creator" || config["page"] != expected_cursor || config["scope"] != "posts" {
						return nil, fmt.Errorf("invalid profile arguments: %v", config)
					}
					var next_cursor any = ""
					if page_index < len(test_case.next_markers) {
						next_cursor = test_case.next_markers[page_index]
					}
					return map[string]any{"platform": "instagram", "scope": "posts", "list": pages[page_index], "next_marker": next_cursor}, nil
				},
				"InstagramFetchDetailNode": func(config map[string]any) (any, error) {
					url, _ := config["url"].(string)
					id := strings.TrimSuffix(strings.TrimPrefix(url, "https://www.instagram.com/p/post_"), "/")
					detail_requests = append(detail_requests, id)
					if id == "3" || media_by_id[id] == nil {
						return nil, fmt.Errorf("unexpected detail request: %s", url)
					}
					content_id := "instagram:" + id
					if test_case.mismatch_detail {
						content_id = "instagram:another"
					}
					return map[string]any{"platform": "instagram", "content": map[string]any{"id": content_id},
						"result": map[string]any{"external_id": id, "shortcode": "post_" + id, "source_url": url, "media": media_by_id[id]}}, nil
				},
			} {
				node_type, fetch := node_type, fetch
				flow_engine.RegisterNode(node_type, func(config map[string]any) engine.Node {
					return &stub_instagram_sync_node{id: config["id"].(string), node_type: node_type, config: config, fetch: fetch}
				})
			}
			flowengine.RegisterServiceNode(flow_engine, func(_ context.Context, tool string, arguments map[string]any) (any, error) {
				switch tool {
				case "check_download_task_contents":
					raw_ids, _ := json.Marshal(arguments["content_ids"])
					var content_ids []string
					if err := json.Unmarshal(raw_ids, &content_ids); err != nil {
						return nil, err
					}
					list := []any{}
					for _, content_id := range content_ids {
						count := 0
						if content_id == "instagram:3" {
							count = 1
						}
						list = append(list, map[string]any{"content_id": content_id, "exists": count > 0, "task_count": count})
					}
					if test_case.broken_checks {
						list = []any{}
					}
					return map[string]any{"list": list, "total": len(content_ids)}, nil
				case "create_download_task":
					if test_case.download_failure {
						return nil, fmt.Errorf("下载测试失败")
					}
					config, _ := arguments["config"].(map[string]any)
					if arguments["platform"] != "instagram" || arguments["build_from_fetch"] != true || arguments["download_dir"] != "/downloads/instagram" || config["existing_action"] != "skip" {
						return nil, fmt.Errorf("invalid download arguments: %v", arguments)
					}
					raw_content, _ := json.Marshal(arguments["content"])
					var content struct {
						ID    string `json:"external_id"`
						Media []any  `json:"media"`
					}
					if err := json.Unmarshal(raw_content, &content); err != nil {
						return nil, err
					}
					if !reflect.DeepEqual(content.Media, media_by_id[content.ID]) {
						return nil, fmt.Errorf("download lost fresh detail media: %s", raw_content)
					}
					downloads = append(downloads, content.ID)
					return map[string]any{"created": !test_case.race_skip, "skipped": test_case.race_skip}, nil
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
			if list_requests != len(pages) || !reflect.DeepEqual(downloads, test_case.want_downloads) || !reflect.DeepEqual(detail_requests, test_case.want_downloads) {
				t.Fatalf("list requests=%d, details=%v, downloads=%v; want %v", list_requests, detail_requests, downloads, test_case.want_downloads)
			}
			record, ok := flow_engine.GetRunSnapshot(run_id)
			if !ok || record.Status != engine.RunStatusCompleted {
				t.Fatalf("run did not complete: %+v", record)
			}
			raw_summary, _ := json.Marshal(record.NodeOutputs)
			var output struct {
				Summary struct {
					Works     int    `json:"works"`
					Pages     int    `json:"pages"`
					Submitted int    `json:"submitted"`
					Skipped   int    `json:"skipped"`
					Reason    string `json:"stop_reason"`
				} `json:"sync_summary"`
			}
			if err := json.Unmarshal(raw_summary, &output); err != nil {
				t.Fatal(err)
			}
			want_submitted := len(downloads)
			if test_case.race_skip {
				want_submitted = 0
			}
			if output.Summary.Pages != len(pages) || output.Summary.Reason != "no_more_pages" || output.Summary.Submitted != want_submitted || output.Summary.Works != output.Summary.Submitted+output.Summary.Skipped {
				t.Fatalf("incorrect sync summary: %+v", output.Summary)
			}
		})
	}
}
