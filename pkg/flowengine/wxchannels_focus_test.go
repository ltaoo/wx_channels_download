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
	"sync/atomic"
	"testing"
	"time"

	"wx_channel/pkg/flowengine"
	"wx_channel/pkg/flowengine/engine"
	"wx_channel/pkg/flowengine/nodes"
)

func bool_ptr(value bool) *bool { return &value }

// wxchannels_page builds a minimal 视频号 feed-list response matching the shape
// parse_page expects: object, BaseResponse, continue_flag and last_buffer.
func wxchannels_page(continue_flag int, last_buffer string, ids ...string) map[string]any {
	items := make([]any, 0, len(ids))
	for _, id := range ids {
		items = append(items, map[string]any{
			"id":              id,
			"object_nonce_id": "nonce_" + id,
			"object_desc": map[string]any{
				"description": "video " + id,
				"media_type":  4,
				"media": []any{map[string]any{
					"media_type": 4,
					"url":        "https://example.com/" + id,
				}},
			},
			"contact": map[string]any{"username": "tester@finder"},
		})
	}
	return map[string]any{
		"object":        items,
		"BaseResponse":  map[string]any{"Ret": 0},
		"continue_flag": continue_flag,
		"last_buffer":   last_buffer,
	}
}

func TestWXChannelsCreatorSyncFocus(t *testing.T) {
	for _, test_case := range []struct {
		name           string
		focus          *bool
		pages          []map[string]any
		want_requests  int
		want_downloads []string
	}{
		{
			name:          "all existing page stops without focus",
			pages:         []map[string]any{wxchannels_page(1, "10", "1")},
			want_requests: 1,
		},
		{
			name:  "focus keeps scanning past an all-existing page",
			focus: bool_ptr(true),
			pages: []map[string]any{
				wxchannels_page(1, "10", "1"),
				wxchannels_page(0, "", "2"),
			},
			want_requests:  2,
			want_downloads: []string{"2"},
		},
		{
			name:          "focus still stops when there are no more pages",
			focus:         bool_ptr(true),
			pages:         []map[string]any{wxchannels_page(0, "", "1")},
			want_requests: 1,
		},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			raw, err := os.ReadFile("../../docs/examples/pipelines/01_creator_video_sync.json")
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
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("list authentication was not forwarded")
				}
				index := int(requests.Load())
				if index >= len(test_case.pages) {
					t.Error("requested a page after scanning should have stopped")
					http.Error(w, "unexpected page", 500)
					return
				}
				expected_marker := ""
				if index > 0 {
					expected_marker, _ = test_case.pages[index-1]["last_buffer"].(string)
				}
				if got := r.URL.Query().Get("next_marker"); got != expected_marker {
					t.Errorf("cursor was not forwarded: got %q want %q", got, expected_marker)
				}
				requests.Add(1)
				_ = json.NewEncoder(w).Encode(test_case.pages[index])
			}))
			defer server.Close()

			validate := definition.Nodes["validate"]
			validate.ConfigValues = map[string]any{"username": "tester@finder", "url": server.URL, "method": "GET", "token": "test-token"}
			definition.Nodes["validate"] = validate

			existing := map[string]bool{"wxchannels:1": true}
			var downloads []string
			flow_engine := flowengine.NewWorkflowEngine()
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
						if !strings.HasPrefix(content_id, "wxchannels:") {
							return nil, fmt.Errorf("wrong platform ID: %s", content_id)
						}
						count := 0
						if existing[content_id] {
							count = 1
						}
						list = append(list, map[string]any{"content_id": content_id, "exists": count > 0, "task_count": count})
					}
					return map[string]any{"list": list, "total": len(content_ids)}, nil
				case "create_download_task":
					if arguments["platform"] != "wxchannels" {
						return nil, fmt.Errorf("wrong platform: %v", arguments["platform"])
					}
					raw_content, _ := json.Marshal(arguments["content"])
					var content map[string]any
					if err := json.Unmarshal(raw_content, &content); err != nil {
						return nil, err
					}
					content_id, _ := content["id"].(string)
					if content_id == "" {
						return nil, fmt.Errorf("download content missing id: %v", content)
					}
					downloads = append(downloads, content_id)
					existing["wxchannels:"+content_id] = true
					return map[string]any{"created": true, "skipped": false}, nil
				default:
					return nil, fmt.Errorf("unexpected tool: %s", tool)
				}
			})

			initial_data := map[string]any{}
			if test_case.focus != nil {
				initial_data["focus"] = *test_case.focus
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			run_id, err := flow_engine.StartFlowWithOptions(definition.ID, initial_data, engine.StartFlowOptions{Definition: &definition, Context: ctx, ValidateSchemas: true})
			if err != nil {
				t.Fatal(err)
			}
			if got := int(requests.Load()); got != test_case.want_requests || !reflect.DeepEqual(downloads, test_case.want_downloads) {
				t.Fatalf("requests=%d, downloads=%v; want %d, %v", got, downloads, test_case.want_requests, test_case.want_downloads)
			}
			record, ok := flow_engine.GetRunSnapshot(run_id)
			if !ok || record.Status != engine.RunStatusCompleted {
				t.Fatalf("run did not complete: %+v", record)
			}
		})
	}
}
