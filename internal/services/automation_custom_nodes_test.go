package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"wx_channel/internal/database/model"
	"wx_channel/internal/events"
	"wx_channel/pkg/dm"
	"wx_channel/pkg/flowengine"
)

func custom_node_test_service(t *testing.T) (*AutomationService, string) {
	t.Helper()
	base := new_trigger_test_service(t)
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "nodes"), 0700); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join("..", "..", "examples", "nodes", "qiniu_upload.js"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "nodes", "qiniu.js"), source, 0600); err != nil {
		t.Fatal(err)
	}
	flow_engine := flowengine.NewWorkflowEngine()
	registry, err := flowengine.LoadCustomNodes(context.Background(), flow_engine, directory, nil)
	if err != nil {
		t.Fatal(err)
	}
	service := NewAutomationService(base.db, nil, flow_engine, nil, registry)
	t.Cleanup(service.Stop)
	return service, directory
}

func configure_qiniu_test_node(t *testing.T, service *AutomationService, flow_id, upload_url string) *model.UserFlow {
	t.Helper()
	settings := map[string]any{"access_key": "test-access-key", "secret_key": "test-secret-key", "bucket": "test-bucket", "domain": "https://cdn.example.com"}
	if upload_url != "" {
		settings["upload_url"] = upload_url
	}
	flow, err := service.UpdateUserFlow(flow_id, UpdateUserFlowInput{NodeConfigs: map[string]map[string]any{"upload": settings}})
	if err != nil {
		t.Fatal(err)
	}
	return flow
}

func TestAutomationCustomQiniuWorkflow(t *testing.T) {
	for _, scenario := range []string{"success", "second-upload-fails", "empty-files", "webhook-fails"} {
		t.Run(scenario, func(t *testing.T) {
			service, directory := custom_node_test_service(t)
			task := &dm.DownloadTask{ID: 42, Status: model.TaskStatusFinished, Name: "最终任务", Files: []dm.DownloadTaskFile{}}
			task.ConfigJSON = marshal_json(map[string]any{"download_dir": directory})
			contents := [][]byte{[]byte("finished-video\x00\xff"), {}}
			if scenario != "empty-files" {
				for index, name := range []string{"后处理成片.mp4", "empty.txt"} {
					path := filepath.Join(directory, name)
					if err := os.WriteFile(path, contents[index], 0600); err != nil {
						t.Fatal(err)
					}
					task.Files = append(task.Files, dm.DownloadTaskFile{ID: index + 1, Name: name, FilePath: path, DownloadDir: directory, Kind: "application/octet-stream"})
				}
			}
			upload_count, webhook_count := 0, 0
			var webhook_task dm.DownloadTask
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/webhook" {
					webhook_count++
					if upload_count != len(task.Files) {
						t.Error("webhook sent before all files uploaded")
					}
					if err := json.NewDecoder(request.Body).Decode(&webhook_task); err != nil {
						t.Error(err)
					}
					if scenario == "webhook-fails" {
						writer.WriteHeader(http.StatusBadGateway)
					} else {
						writer.WriteHeader(http.StatusNoContent)
					}
					return
				}
				if err := request.ParseMultipartForm(1024); err != nil {
					t.Error(err)
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				defer request.MultipartForm.RemoveAll()
				file, header, err := request.FormFile("file")
				if err != nil {
					t.Error(err)
					writer.WriteHeader(http.StatusBadRequest)
					return
				}
				defer file.Close()
				data, err := io.ReadAll(file)
				if upload_count >= len(contents) || err != nil || !bytes.Equal(data, contents[upload_count]) || header.Filename != task.Files[upload_count].Name {
					t.Error("incorrect final file uploaded")
				}
				token := strings.Split(request.FormValue("token"), ":")
				if len(token) != 3 || token[0] != "test-access-key" {
					t.Error("invalid token")
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				mac := hmac.New(sha1.New, []byte("test-secret-key"))
				_, _ = mac.Write([]byte(token[2]))
				if token[1] != base64.URLEncoding.EncodeToString(mac.Sum(nil)) {
					t.Error("invalid signature")
				}
				policy_data, err := base64.URLEncoding.DecodeString(token[2])
				if err != nil {
					t.Error(err)
				}
				var policy struct {
					Scope      string `json:"scope"`
					Deadline   int64  `json:"deadline"`
					InsertOnly int    `json:"insertOnly"`
					FsizeLimit int64  `json:"fsizeLimit"`
				}
				key := request.FormValue("key")
				if err := json.Unmarshal(policy_data, &policy); err != nil || policy.Scope != "test-bucket:"+key || policy.Deadline <= time.Now().Unix() || policy.InsertOnly != 1 || policy.FsizeLimit != int64(len(data)) {
					t.Error("invalid upload policy")
				}
				upload_count++
				if scenario == "second-upload-fails" && upload_count == 2 {
					writer.WriteHeader(http.StatusForbidden)
					return
				}
				_ = json.NewEncoder(writer).Encode(map[string]string{"key": key, "hash": "test-etag"})
			}))
			defer server.Close()
			raw, err := os.ReadFile(filepath.Join("..", "..", "examples", "pipelines", "download_finished_qiniu_webhook.json"))
			if err != nil {
				t.Fatal(err)
			}
			raw = bytes.ReplaceAll(raw, []byte("http://localhost:8080/webhook/download_task/success"), []byte(server.URL+"/webhook"))
			flow, err := service.ImportUserFlow(string(raw))
			if err != nil {
				t.Fatal(err)
			}
			configure_qiniu_test_node(t, service, flow.ID, server.URL+"/upload")
			// Exercise the actual application-event JSON envelope and rule lookup.
			service.HandleEvent(events.DownloadTaskFinished{TaskID: task.ID, Task: task, DownloadDir: directory})
			service.wg.Wait()
			var runs []model.FlowRunRecord
			if err := service.db.Where("flow_id = ?", flow.ID).Find(&runs).Error; err != nil || len(runs) != 1 {
				t.Fatalf("event did not create one run: %+v %v", runs, err)
			}
			run := runs[0]
			expected_status := "COMPLETED"
			if scenario == "second-upload-fails" || scenario == "webhook-fails" {
				expected_status = "FAILED"
			}
			if run.Status != expected_status || upload_count != len(task.Files) {
				t.Fatalf("unexpected result: status=%s error=%s uploads=%d", run.Status, run.Error, upload_count)
			}
			if scenario == "second-upload-fails" {
				if webhook_count != 0 || !strings.Contains(run.Error, "HTTP 403") {
					t.Fatalf("failure did not stop webhook: count=%d error=%s", webhook_count, run.Error)
				}
			} else if webhook_count != 1 || !reflect.DeepEqual(webhook_task, *task) {
				t.Fatalf("webhook body differs from hook context: count=%d body=%+v", webhook_count, webhook_task)
			}
			if scenario == "empty-files" {
				return
			}
			manifest, err := os.ReadFile(filepath.Join(directory, "qiniu-upload-42.json"))
			if err != nil {
				t.Fatal(err)
			}
			var uploaded []map[string]any
			if err := json.Unmarshal(manifest, &uploaded); err != nil {
				t.Fatal(err)
			}
			expected_count := 2
			if scenario == "second-upload-fails" {
				expected_count = 1
			}
			if len(uploaded) != expected_count || uploaded[0]["file_path"] != task.Files[0].FilePath || !strings.Contains(fmt.Sprint(uploaded[0]["url"]), "%") {
				t.Fatalf("incorrect upload manifest: %s", manifest)
			}
			if strings.Contains(run.NodeOutputs+run.NodeLogs, "test-secret-key") {
				t.Fatal("credentials exposed in run data")
			}
		})
	}
}

func TestCustomNodeEditorAndValidation(t *testing.T) {
	service, _ := custom_node_test_service(t)
	var custom *UserFlowNodeCatalogItem
	for _, item := range service.NodeCatalog() {
		if item.Type == "QiniuUploadNode" {
			copied := item
			custom = &copied
		}
	}
	if custom == nil || custom.Name != "上传到七牛云" || custom.DefaultConfig["output_key"] != "uploaded" {
		t.Fatalf("missing editor metadata: %+v", custom)
	}
	if len(custom.ConfigSchema) != 5 || !custom.ConfigSchema[0].Secret || !custom.ConfigSchema[1].Secret || custom.ConfigSchema[4].Default != "https://upload-z2.qiniup.com" {
		t.Fatalf("missing required configuration metadata: %+v", custom.ConfigSchema)
	}
	definition := &flowengine.FlowDefinition{Name: "custom", StartNodeID: "start", Nodes: map[string]flowengine.NodeDefinition{
		"start":  runtime_test_node("start", "StartNode", nil, "upload"),
		"upload": runtime_test_node("upload", "QiniuUploadNode", map[string]any{"arguments": map[string]any{"task": "{{input.missing}}"}}),
	}}
	if err := normalize_user_flow_definition(definition, service.custom_nodes); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("invalid custom argument scope accepted: %v", err)
	}
	node := definition.Nodes["upload"]
	node.Config["arguments"] = map[string]any{}
	definition.Nodes["upload"] = node
	if err := normalize_user_flow_definition(definition); err == nil {
		t.Fatal("custom type accepted without registration")
	}
	if err := normalize_user_flow_definition(definition, service.custom_nodes); err != nil {
		t.Fatal(err)
	}
	draft := PipelineEditorDraft{StartNodeID: "start", Nodes: []UserFlowNodeInput{
		{ID: "start", Type: "StartNode", NextIDs: []string{"upload"}},
		{ID: "upload", Type: "QiniuUploadNode", Config: map[string]any{"arguments": map[string]any{}}},
	}}
	if err := service.ValidateEditorDraft(&draft); err != nil {
		t.Fatal(err)
	}
	for _, config := range []map[string]any{{"arguments": "bad"}, {"output_key": 1}, {"timeout_seconds": 0}} {
		if err := validate_user_flow_node_config("QiniuUploadNode", config, service.custom_nodes); err == nil {
			t.Fatalf("invalid configuration accepted: %+v", config)
		}
	}
	flow := trigger_test_flow(t, service)
	flow, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{Nodes: draft.Nodes, StartNodeID: "start"})
	if err != nil {
		t.Fatalf("editor cannot save custom node: %v", err)
	}
	if _, err := service.TriggerFlowDirect(flow.ID, nil); err == nil || !strings.Contains(err.Error(), "access_key") {
		t.Fatalf("unconfigured custom node ran: %v", err)
	}
	for _, missing := range []string{"access_key", "secret_key", "bucket", "domain"} {
		settings := map[string]any{"access_key": "key", "secret_key": "secret", "bucket": "bucket", "domain": "https://cdn.example.com"}
		delete(settings, missing)
		if _, err := service.UpdateUserFlow(flow.ID, UpdateUserFlowInput{NodeConfigs: map[string]map[string]any{"upload": settings}}); err == nil || !strings.Contains(err.Error(), missing) {
			t.Fatalf("missing %s was accepted: %v", missing, err)
		}
	}
	configured := configure_qiniu_test_node(t, service, flow.ID, "")
	if decode_node_configs(configured.NodeConfigs)["upload"]["upload_url"] != "https://upload-z2.qiniup.com" || strings.Contains(configured.Definition, "test-secret-key") {
		t.Fatal("default not applied or secrets entered reusable definition")
	}
	// Simulate an older persisted flow and ensure startup refreshes its declarations.
	stored, _ := decode_user_flow_definition(configured.Definition)
	old_node := stored.Nodes["upload"]
	old_node.ConfigSchema = nil
	stored.Nodes["upload"] = old_node
	if err := service.db.Model(&model.UserFlow{}).Where("id = ?", flow.ID).Update("definition", marshal_json(stored)).Error; err != nil {
		t.Fatal(err)
	}
	service.load_user_flows()
	loaded, _ := service.GetUserFlow(flow.ID)
	refreshed, _ := decode_user_flow_definition(loaded.Definition)
	if len(refreshed.Nodes["upload"].ConfigSchema) != 5 || !strings.Contains(loaded.NodeConfigs, "test-secret-key") {
		t.Fatal("startup did not refresh schema or lost settings")
	}
}

func TestDownloadCompletionEventsWaitInsteadOfDroppingBusyTasks(t *testing.T) {
	service, _ := custom_node_test_service(t)
	first_webhook := make(chan struct{})
	release_first := make(chan struct{})
	var release_once sync.Once
	release := func() { release_once.Do(func() { close(release_first) }) }
	defer release()
	var task_ids []int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var task dm.DownloadTask
		if err := json.NewDecoder(request.Body).Decode(&task); err != nil {
			t.Error(err)
		}
		task_ids = append(task_ids, task.ID)
		if len(task_ids) == 1 {
			close(first_webhook)
			<-release_first
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer func() { release(); server.Close() }()
	raw, err := os.ReadFile(filepath.Join("..", "..", "examples", "pipelines", "download_finished_qiniu_webhook.json"))
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.ReplaceAll(raw, []byte("http://localhost:8080/webhook/download_task/success"), []byte(server.URL))
	flow, err := service.ImportUserFlow(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	configure_qiniu_test_node(t, service, flow.ID, "")
	service.HandleEvent(events.DownloadTaskFinished{TaskID: 1, Task: &dm.DownloadTask{ID: 1, Files: []dm.DownloadTaskFile{}}})
	select {
	case <-first_webhook:
	case <-time.After(5 * time.Second):
		t.Fatal("first task never reached webhook")
	}
	service.HandleEvent(events.DownloadTaskFinished{TaskID: 2, Task: &dm.DownloadTask{ID: 2, Files: []dm.DownloadTaskFile{}}})
	release()
	service.wg.Wait()
	var completed int64
	if err := service.db.Model(&model.FlowRunRecord{}).Where("flow_id = ? AND status = ?", flow.ID, "COMPLETED").Count(&completed).Error; err != nil || completed != 2 || !reflect.DeepEqual(task_ids, []int{1, 2}) {
		t.Fatalf("busy task dropped or reordered: runs=%d tasks=%v error=%v", completed, task_ids, err)
	}
}
