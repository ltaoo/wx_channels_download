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
	definition engine.FlowDefinition
	engine     *engine.FlowEngine
	pages      map[string]map[string]any
	downloads  []string
	recordings int
	initial    map[string]any
	notified   []string
	fail_send  int
	sends      int
	fail_save  bool
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
		initial: map[string]any{"keep": "preserved", "__automation": map[string]any{"type": "Cron", "start_node": "start"}},
	}
	flow_engine.FlowDefinitions[definition.ID] = definition
	flowengine.RegisterServiceNode(flow_engine, func(_ context.Context, tool string, arguments map[string]any) (any, error) {
		switch tool {
		case "get_wxchannels_account_videos":
			if arguments["username"] != "test@finder" {
				return nil, fmt.Errorf("unexpected username: %v", arguments)
			}
			cursor, _ := arguments["next_marker"].(string)
			page, ok := fixture.pages[cursor]
			if !ok {
				return nil, fmt.Errorf("unexpected cursor %q", cursor)
			}
			return map[string]any{"errCode": 0, "data": page}, nil
		case "download_wxchannels_video":
			id, ok := arguments["oid"].(string)
			if !ok || arguments["nid"] != "nonce-"+id || arguments["existing_action"] != "skip" {
				return nil, fmt.Errorf("invalid download arguments: %v", arguments)
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
	id, err := f.engine.StartFlow(f.definition.ID, nil)
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
	if fmt.Sprint(summary["skipped"]) != "2" || fmt.Sprint(summary["submitted"]) != "0" {
		t.Fatalf("repeat run did not skip: %v", summary)
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
