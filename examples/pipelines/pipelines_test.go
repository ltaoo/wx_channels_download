package pipelines_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/internal/services"
	"wx_channel/pkg/flowengine"
	"wx_channel/pkg/flowengine/engine"
)

type round_trip_func func(*http.Request) (*http.Response, error)

func (f round_trip_func) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type pipeline_fixture struct {
	definition  engine.FlowDefinition
	engine      *engine.FlowEngine
	pages       map[string]map[string]any
	downloads   []string
	existing    map[string]bool
	checked     []string
	requested   []string
	before_tool func(string, map[string]any) error
	recordings  int
	initial     map[string]any
	notified    []string
	fail_send   int
	sends       int
	fail_save   bool
}

func new_fixture(t *testing.T, filename string) *pipeline_fixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "pipelines.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if err := db.AutoMigrate(&model.UserFlow{}, &model.FlowSchedule{}, &model.FlowRunRecord{}); err != nil {
		t.Fatal(err)
	}
	flow_engine := flowengine.NewWorkflowEngine()
	service := services.NewAutomationService(db, nil, flow_engine, nil)
	raw, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	flow, err := service.ImportUserFlow(string(raw))
	if err != nil {
		t.Fatalf("import %s: %v", filename, err)
	}
	var definition engine.FlowDefinition
	if err := json.Unmarshal([]byte(flow.Definition), &definition); err != nil {
		t.Fatal(err)
	}
	// Exercise the editor save path too: it reconstructs nodes and edges.
	editor_nodes := make([]services.UserFlowNodeInput, 0, len(definition.Nodes))
	for _, node := range definition.Nodes {
		editor_nodes = append(editor_nodes, services.UserFlowNodeInput{
			ID: node.ID, Type: node.Type, Name: node.Name, Config: node.Config,
			NextIDs: node.NextNodeIDs, Position: node.Position,
		})
	}
	saved, err := service.UpdateUserFlow(flow.ID, services.UpdateUserFlowInput{Nodes: editor_nodes})
	if err != nil {
		t.Fatalf("editor save %s: %v", filename, err)
	}
	if err := json.Unmarshal([]byte(saved.Definition), &definition); err != nil {
		t.Fatal(err)
	}
	variables := definition.Nodes["config"].Config["variables"].(map[string]any)
	variables["creator_username"] = "test@finder"
	variables["monitor_schedule_id"] = "sched-test"
	fixture := &pipeline_fixture{
		definition: definition, engine: flow_engine, pages: map[string]map[string]any{},
		existing: map[string]bool{},
		initial:  map[string]any{"keep": "preserved", "__automation": map[string]any{"type": "Cron", "start_node": "start"}},
	}
	flow_engine.FlowDefinitions[definition.ID] = definition
	flowengine.RegisterServiceNode(flow_engine, func(_ context.Context, tool string, arguments map[string]any) (any, error) {
		if fixture.before_tool != nil {
			if err := fixture.before_tool(tool, arguments); err != nil {
				return nil, err
			}
		}
		switch tool {
		case "get_wxchannels_account_videos":
			if arguments["username"] != "test@finder" {
				return nil, fmt.Errorf("unexpected username: %v", arguments)
			}
			cursor, _ := arguments["next_marker"].(string)
			fixture.requested = append(fixture.requested, cursor)
			page, ok := fixture.pages[cursor]
			if !ok {
				return nil, fmt.Errorf("unexpected cursor %q", cursor)
			}
			return map[string]any{"errCode": 0, "data": page}, nil
		case "get_download_tasks":
			content_id, _ := arguments["content_id"].(string)
			if !strings.HasPrefix(content_id, "wxchannels:") {
				return nil, fmt.Errorf("invalid content ID: %v", arguments)
			}
			id := strings.TrimPrefix(content_id, "wxchannels:")
			fixture.checked = append(fixture.checked, id)
			exists := fixture.existing[id]
			for _, downloaded := range fixture.downloads {
				exists = exists || downloaded == id
			}
			total := 0
			list := []any{}
			if exists {
				total = 1
				list = append(list, map[string]any{"content_id": content_id})
			}
			return map[string]any{"total": total, "list": list}, nil
		case "download_wxchannels_video":
			id, ok := arguments["oid"].(string)
			if !ok || arguments["nid"] != "nonce-"+id || arguments["existing_action"] != "skip" {
				return nil, fmt.Errorf("invalid download arguments: %v", arguments)
			}
			if fixture.existing[id] {
				return map[string]any{"skipped": true}, nil
			}
			for _, previous := range fixture.downloads {
				if previous == id {
					return map[string]any{"skipped": true}, nil
				}
			}
			fixture.downloads = append(fixture.downloads, id)
			return map[string]any{"created": true, "started": true}, nil
		case "download_wxchannels_live":
			if arguments["account"] != "test@finder" || arguments["existing_action"] != "skip" {
				return nil, fmt.Errorf("invalid live arguments: %v", arguments)
			}
			fixture.recordings++
			return map[string]any{"created": true}, nil
		default:
			return nil, fmt.Errorf("unexpected tool %s", tool)
		}
	})
	previous_transport := http.DefaultTransport
	http.DefaultTransport = round_trip_func(fixture.round_trip)
	t.Cleanup(func() { http.DefaultTransport = previous_transport })
	return fixture
}

func (f *pipeline_fixture) round_trip(request *http.Request) (*http.Response, error) {
	var body map[string]any
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		return nil, err
	}
	var result any
	switch request.URL.Path {
	case "/api/v1/automation/get_schedule":
		if body["id"] != "sched-test" {
			return nil, fmt.Errorf("wrong schedule: %v", body)
		}
		initial, _ := json.Marshal(f.initial)
		result = map[string]any{"code": 0, "data": map[string]any{"id": "sched-test", "initial_data": string(initial)}}
	case "/api/v1/automation/update_schedule":
		if f.fail_save {
			result = map[string]any{"code": 1, "msg": "save failed"}
			break
		}
		if body["id"] != "sched-test" {
			return nil, fmt.Errorf("wrong checkpoint schedule: %v", body)
		}
		f.initial = body["initial_data"].(map[string]any)
		result = map[string]any{"code": 0}
	case "/cgi-bin/webhook/send":
		f.sends++
		if f.sends == f.fail_send {
			result = map[string]any{"errcode": 45009, "errmsg": "rate limited"}
			break
		}
		if body["msgtype"] != "text" {
			return nil, fmt.Errorf("wrong webhook body: %v", body)
		}
		f.notified = append(f.notified, body["text"].(map[string]any)["content"].(string))
		result = map[string]any{"errcode": 0}
	default:
		return nil, fmt.Errorf("unexpected HTTP request: %s", request.URL)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(raw))), Request: request}, nil
}

func (f *pipeline_fixture) run(t *testing.T, want_error bool) *engine.RunRecord {
	t.Helper()
	id, err := f.engine.StartFlow(f.definition.ID, map[string]any{"username": "test@finder"})
	if (err != nil) != want_error {
		t.Fatalf("run error = %v, want_error = %v", err, want_error)
	}
	snapshot, ok := f.engine.GetRunSnapshot(id)
	if !ok || (!want_error && snapshot.Status != engine.RunStatusCompleted) || (want_error && snapshot.Status != engine.RunStatusFailed) {
		t.Fatalf("unexpected run snapshot: %+v", snapshot)
	}
	return snapshot
}

func video(id string) map[string]any {
	return map[string]any{"id": id, "objectNonceId": "nonce-" + id, "createtime": 1700000000, "objectDesc": map[string]any{"mediaType": 4, "description": "视频 " + id}}
}

func page(cursor string, more int, videos ...map[string]any) map[string]any {
	items := make([]any, 0, len(videos))
	for _, item := range videos {
		items = append(items, item)
	}
	return map[string]any{"object": items, "continueFlag": more, "lastBuffer": cursor, "liveObjects": []any{}}
}

func TestCreatorVideoSync(t *testing.T) {
	f := new_fixture(t, "01_creator_video_sync.json")
	image := map[string]any{"id": "picture", "objectDesc": map[string]any{"mediaType": 2}}
	f.pages[""] = page("page-2", 1, video("18446744073709551614"), image)
	f.pages["page-2"] = page("", 0, video("18446744073709551614"), video("18446744073709551613"))
	f.run(t, false)
	if !reflect.DeepEqual(f.downloads, []string{"18446744073709551614", "18446744073709551613"}) {
		t.Fatalf("wrong downloads or ID precision loss: %v", f.downloads)
	}
	snapshot := f.run(t, false)
	summary := snapshot.NodeOutputs["sync_summary"].(map[string]any)
	if fmt.Sprint(summary["skipped"]) != "1" || fmt.Sprint(summary["submitted"]) != "0" || fmt.Sprint(summary["pages"]) != "1" || summary["stop_reason"] != "all_existing" {
		t.Fatalf("repeat run did not skip: %v", summary)
	}
}

func TestCreatorVideoSyncStopsAtExistingPage(t *testing.T) {
	f := new_fixture(t, "01_creator_video_sync.json")
	f.existing["a"], f.existing["b"] = true, true
	f.pages[""] = page("older", 1, video("a"), video("b"))
	snapshot := f.run(t, false)
	summary := snapshot.NodeOutputs["sync_summary"].(map[string]any)
	if !reflect.DeepEqual(f.requested, []string{""}) || !reflect.DeepEqual(f.checked, []string{"a", "b"}) || len(f.downloads) != 0 || summary["stop_reason"] != "all_existing" {
		t.Fatalf("existing page did not stop scanning: requests=%v checks=%v downloads=%v summary=%v", f.requested, f.checked, f.downloads, summary)
	}
}

func TestCreatorVideoSyncChecksWholePageBeforeDownloading(t *testing.T) {
	f := new_fixture(t, "01_creator_video_sync.json")
	f.existing["old"] = true
	f.pages[""] = page("page-2", 1, video("old"), video("new-1"), video("new-2"))
	f.pages["page-2"] = page("older", 1, video("old"))
	f.before_tool = func(tool string, arguments map[string]any) error {
		if tool == "download_wxchannels_video" && (len(f.checked) != 3 || arguments["oid"] == "old") {
			return fmt.Errorf("download started before complete page checks, or selected an existing video: %v %v", f.checked, arguments)
		}
		return nil
	}
	snapshot := f.run(t, false)
	summary := snapshot.NodeOutputs["sync_summary"].(map[string]any)
	if !reflect.DeepEqual(f.downloads, []string{"new-1", "new-2"}) || !reflect.DeepEqual(f.requested, []string{"", "page-2"}) || fmt.Sprint(summary["submitted"]) != "2" || fmt.Sprint(summary["skipped"]) != "2" {
		t.Fatalf("incorrect mixed page handling: requests=%v downloads=%v summary=%v", f.requested, f.downloads, summary)
	}
}

func TestCreatorVideoSyncPrefetchesWhileSubmittingDownloads(t *testing.T) {
	f := new_fixture(t, "01_creator_video_sync.json")
	f.pages[""] = page("page-2", 1, video("a"))
	f.pages["page-2"] = page("page-3", 1, video("b"))
	f.pages["page-3"] = page("", 0, video("c"))
	download_started, prefetch_started := make(chan struct{}), make(chan struct{})
	f.before_tool = func(tool string, arguments map[string]any) error {
		var peer <-chan struct{}
		if tool == "download_wxchannels_video" && arguments["oid"] == "a" {
			close(download_started)
			peer = prefetch_started
		}
		if tool == "get_wxchannels_account_videos" && arguments["next_marker"] == "page-2" {
			close(prefetch_started)
			peer = download_started
		}
		if peer != nil {
			select {
			case <-peer:
			case <-time.After(2 * time.Second):
				return fmt.Errorf("download and prefetch were serialized")
			}
		}
		return nil
	}
	snapshot := f.run(t, false)
	summary := snapshot.NodeOutputs["sync_summary"].(map[string]any)
	if !reflect.DeepEqual(f.downloads, []string{"a", "b", "c"}) || !reflect.DeepEqual(f.requested, []string{"", "page-2", "page-3"}) || fmt.Sprint(summary["submitted"]) != "3" || summary["stop_reason"] != "no_more_pages" {
		t.Fatalf("page iterations leaked results or failed to stop: requests=%v downloads=%v summary=%v", f.requested, f.downloads, summary)
	}
}

func TestCreatorVideoSyncContinuesPastPagesWithoutVideos(t *testing.T) {
	for _, non_videos := range []bool{false, true} {
		t.Run(fmt.Sprint(non_videos), func(t *testing.T) {
			f := new_fixture(t, "01_creator_video_sync.json")
			f.pages[""] = page("next", 1)
			if non_videos {
				f.pages[""] = page("next", 1, map[string]any{"id": "picture", "objectDesc": map[string]any{"mediaType": 2}})
			}
			f.pages["next"] = page("", 0, video("a"))
			f.run(t, false)
			if !reflect.DeepEqual(f.downloads, []string{"a"}) || !reflect.DeepEqual(f.requested, []string{"", "next"}) {
				t.Fatalf("page without videos stopped scanning: requests=%v downloads=%v", f.requested, f.downloads)
			}
		})
	}
}

func TestCreatorVideoSyncRejectsFailedChecksBeforeDownloading(t *testing.T) {
	f := new_fixture(t, "01_creator_video_sync.json")
	f.pages[""] = page("next", 1, video("a"), video("b"))
	f.before_tool = func(tool string, arguments map[string]any) error {
		if tool == "get_download_tasks" && arguments["content_id"] == "wxchannels:b" {
			return fmt.Errorf("task database unavailable")
		}
		return nil
	}
	f.run(t, true)
	if len(f.downloads) != 0 || !reflect.DeepEqual(f.requested, []string{""}) {
		t.Fatalf("incomplete checks caused downloads or prefetch: requests=%v downloads=%v", f.requested, f.downloads)
	}
}

func TestCreatorVideoSyncRequiresUsername(t *testing.T) {
	f := new_fixture(t, "01_creator_video_sync.json")
	for _, username := range []any{nil, "", "  ", 123} {
		if _, err := f.engine.StartFlow(f.definition.ID, map[string]any{"username": username}); err == nil {
			t.Fatalf("invalid username accepted: %#v", username)
		}
	}
	if len(f.requested) != 0 || len(f.downloads) != 0 {
		t.Fatal("invalid username reached a service node")
	}
}

func TestCreatorScanRejectsIncompletePagination(t *testing.T) {
	for _, filename := range []string{"01_creator_video_sync.json", "03_creator_new_video_notify.json"} {
		t.Run(filename, func(t *testing.T) {
			f := new_fixture(t, filename)
			f.pages[""] = page("same-cursor", 1, video("a"))
			f.pages["same-cursor"] = page("same-cursor", 1, video("b"))
			f.run(t, true)
			if len(f.notified) != 0 || f.initial["creator_video_monitor"] != nil {
				t.Fatal("incomplete scan sent notifications or established a baseline")
			}
		})
	}
}

func TestCreatorScanBoundaries(t *testing.T) {
	for _, filename := range []string{"01_creator_video_sync.json", "03_creator_new_video_notify.json"} {
		t.Run(filename, func(t *testing.T) {
			t.Run("empty", func(t *testing.T) {
				f := new_fixture(t, filename)
				f.pages[""] = page("", 0)
				f.run(t, false)
				if len(f.downloads) != 0 || len(f.notified) != 0 {
					t.Fatal("empty creator caused a download or notification")
				}
			})
			t.Run("missing-pagination", func(t *testing.T) {
				f := new_fixture(t, filename)
				f.pages[""] = page("", 0)
				delete(f.pages[""], "continueFlag")
				f.run(t, true)
			})
			t.Run("page-limit", func(t *testing.T) {
				f := new_fixture(t, filename)
				f.definition.Nodes["config"].Config["variables"].(map[string]any)["max_pages"] = 1
				f.pages[""] = page("next", 1, video("a"))
				f.run(t, true)
				if f.initial["creator_video_monitor"] != nil {
					t.Fatal("truncated scan established a baseline")
				}
			})
			t.Run("unsafe-numeric-id", func(t *testing.T) {
				f := new_fixture(t, filename)
				item := video("a")
				item["id"] = float64(18446744073709551614)
				f.pages[""] = page("", 0, item)
				f.run(t, true)
			})
		})
	}
}

func TestCreatorLiveRecord(t *testing.T) {
	f := new_fixture(t, "02_creator_live_record.json")
	f.pages[""] = page("", 0)
	f.run(t, false)
	if f.recordings != 0 {
		t.Fatal("offline creator was recorded")
	}
	f.pages[""]["liveObjects"] = nil
	f.pages[""]["contact"] = map[string]any{"liveStatus": 0}
	f.run(t, false)
	if f.recordings != 0 {
		t.Fatal("explicitly offline creator was recorded")
	}
	f.pages[""]["liveObjects"] = []any{map[string]any{"username": "other@finder", "liveInfo": map[string]any{"liveId": "other-live"}}}
	f.run(t, false)
	if f.recordings != 0 {
		t.Fatal("another creator was recorded")
	}
	f.pages[""]["liveObjects"] = []any{map[string]any{"contact": map[string]any{"username": "test@finder"}, "liveInfo": map[string]any{"liveId": "123"}}}
	f.run(t, false)
	if f.recordings != 1 {
		t.Fatalf("live creator was not recorded: %d", f.recordings)
	}
}

func TestCreatorNotificationPersistsAndRetries(t *testing.T) {
	f := new_fixture(t, "03_creator_new_video_notify.json")
	f.pages[""] = page("", 0, video("old"))
	f.run(t, false)
	if len(f.notified) != 0 || f.initial["creator_video_monitor"] == nil {
		t.Fatal("first run must establish a baseline without notifying history")
	}
	// Two new videos, with one on a later page. Preserve acknowledgments when
	// the second send fails, then load them on the next independent run.
	f.pages[""] = page("page-2", 1, video("new-1"), video("old"))
	f.pages["page-2"] = page("", 0, video("new-2"))
	f.fail_send = 2
	f.run(t, true)
	if len(f.notified) != 1 || !strings.Contains(f.notified[0], "new-1") {
		t.Fatalf("unexpected partial notifications: %v", f.notified)
	}
	f.run(t, false)
	if len(f.notified) != 2 || !strings.Contains(f.notified[1], "new-2") {
		t.Fatalf("did not retry only the unacknowledged item: %v", f.notified)
	}
	f.run(t, false)
	if len(f.notified) != 2 {
		t.Fatalf("unchanged videos were notified again: %v", f.notified)
	}
	if f.initial["keep"] != "preserved" || f.initial["__automation"].(map[string]any)["type"] != "Cron" {
		t.Fatalf("checkpoint overwrote trigger data: %v", f.initial)
	}
}

func TestCreatorNotificationCheckpointFailure(t *testing.T) {
	f := new_fixture(t, "03_creator_new_video_notify.json")
	f.pages[""] = page("", 0, video("old"))
	f.fail_save = true
	f.run(t, true)
	if f.initial["creator_video_monitor"] != nil {
		t.Fatal("failed checkpoint was persisted")
	}
}
