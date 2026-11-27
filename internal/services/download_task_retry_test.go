package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
	"wx_channel/pkg/hermes"
)

type retry_test_adapter struct {
	adapter.AdapterHandler
	platform_id string
	fetch       func(string) (any, error)
	build       func(any, json.RawMessage) (*adapter.DownloadTaskResult, error)
}

func (a *retry_test_adapter) PlatformID() string                { return a.platform_id }
func (a *retry_test_adapter) Fetch(raw_url string) (any, error) { return a.fetch(raw_url) }
func (a *retry_test_adapter) BuildDownloadTaskFromFetch(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
	return a.build(data, config_json)
}

type retry_test_context_adapter struct{ *retry_test_adapter }

func (a *retry_test_context_adapter) FetchWithProgressContext(ctx context.Context, raw_url string, options adapter.FetchOptions) (any, error) {
	if !options.ForceRefresh || ctx.Err() != nil {
		return nil, errors.New("retry must force a fresh fetch")
	}
	return a.fetch(raw_url)
}

func new_retry_test_task(t *testing.T) (*DownloadTaskService, model.DownloadTask, model.DownloadResource, *retry_test_adapter) {
	t.Helper()
	db := new_task_safety_db(t)
	if err := db.AutoMigrate(&model.DownloadEndpoint{}, &model.DownloadConnection{}, &model.DownloadSegment{}, &model.Content{}, &model.ContentArticle{}, &model.ContentAsset{}); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.DownloadResourceAsset{}); err != nil {
		t.Fatal(err)
	}
	platform_id := fmt.Sprintf("retry_test_%p", t)
	content := model.Content{Id: platform_id + ":item", PlatformId: platform_id, ExternalId: "item", Type: "article", Title: "old detail"}
	if err := db.Create(&content).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ContentArticle{Id: content.Id, Text: "old body"}).Error; err != nil {
		t.Fatal(err)
	}
	task := model.DownloadTask{PlatformId: platform_id, ContentId: &content.Id, SourceURL: "https://example.com/item", Name: "old task", UniqueID: "item", Status: model.TaskStatusFailed, ErrorMessage: "expired", ConfigJSON: `{"download_dir":"/downloads","quality":"original"}`, Timestamps: model.Timestamps{CreatedAt: 1, UpdatedAt: 1}}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	resource := model.DownloadResource{TaskId: &task.Id, ContentId: &content.Id, UniqueID: "file", Name: "saved.bin", DownloadDir: "/custom", Kind: "application/octet-stream", Type: model.ResourceTypeFile, Downloaded: 4, Size: 10, Extra: `{"_hermes_download_name":"original","decrypt_key":"old"}`, Timestamps: model.Timestamps{CreatedAt: 1}}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	endpoint := model.DownloadEndpoint{ResourceId: resource.Id, Protocol: "HTTPS", URL: "https://cdn.example.com/expired", Enabled: 1}
	if err := db.Create(&endpoint).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.DownloadSegment{ResourceId: resource.Id, Downloaded: 4, Size: 10}).Error; err != nil {
		t.Fatal(err)
	}
	impl := &retry_test_adapter{platform_id: platform_id}
	impl.fetch = func(raw_url string) (any, error) {
		if raw_url != task.SourceURL {
			t.Fatalf("unexpected source: %s", raw_url)
		}
		return "fresh platform response", nil
	}
	impl.build = func(data any, config_json json.RawMessage) (*adapter.DownloadTaskResult, error) {
		if data != "fresh platform response" || string(config_json) != task.ConfigJSON {
			t.Fatal("did not reuse fresh data and original config")
		}
		return &adapter.DownloadTaskResult{
			Task:          &model.DownloadTask{Name: "new task", SourceURL: task.SourceURL, CoverURL: "new cover", MetadataJSON: `{"fresh":true}`},
			Content:       &model.Content{Id: content.Id, PlatformId: platform_id, ExternalId: "item", Type: "article", Title: "new detail"},
			ContentDetail: &model.ContentArticle{Id: content.Id, Text: "new body"},
			Resources:     []*adapter.ResourceInfo{{Resource: model.DownloadResource{UniqueID: "file", Name: "new name", Kind: resource.Kind, Type: resource.Type, Extra: `{"decrypt_key":"fresh"}`}, Endpoints: []model.DownloadEndpoint{{Protocol: "HTTPS", URL: "https://cdn.example.com/fresh", Enabled: 1, Headers: `{"Referer":"fresh"}`, Cookies: "new-cookie"}}}},
		}, nil
	}
	adapter.Register(&retry_test_context_adapter{impl})
	return NewDownloadTaskService(db, nil, nil, nil, "", "", nil), task, resource, impl
}

func TestRetryRefreshesDetailsAndEndpointsPreservingProgress(t *testing.T) {
	service, task, original, _ := new_retry_test_task(t)
	if err := service.reset_retry_task(&task); err != nil {
		t.Fatal(err)
	}
	if task.Status != model.TaskStatusWaiting || task.ErrorMessage != "" || task.Name != "new task" || task.CreatedAt != 1 || task.ConfigJSON != `{"download_dir":"/downloads","quality":"original"}` {
		t.Fatalf("incorrect refreshed task: %+v", task)
	}
	var resource model.DownloadResource
	if err := service.db.First(&resource, original.Id).Error; err != nil {
		t.Fatal(err)
	}
	if resource.Name != original.Name || resource.DownloadDir != original.DownloadDir || resource.Downloaded != 4 || resource.Size != 10 || !strings.Contains(resource.Extra, `"decrypt_key":"fresh"`) || !strings.Contains(resource.Extra, `"_hermes_download_name":"original"`) {
		t.Fatalf("lost resume state or fresh decrypt key: %+v", resource)
	}
	var endpoints []model.DownloadEndpoint
	service.db.Where("resource_id = ?", resource.Id).Find(&endpoints)
	if len(endpoints) != 1 || endpoints[0].URL != "https://cdn.example.com/fresh" || endpoints[0].Cookies != "new-cookie" || endpoints[0].Headers != `{"Referer":"fresh"}` {
		t.Fatalf("stale endpoints: %+v", endpoints)
	}
	var detail model.ContentArticle
	service.db.First(&detail, "id = ?", *task.ContentId)
	if detail.Text != "new body" {
		t.Fatalf("stale detail: %+v", detail)
	}
	var content model.Content
	service.db.First(&content, "id = ?", *task.ContentId)
	if content.Title != "new detail" {
		t.Fatalf("stale content: %+v", content)
	}
	var segment model.DownloadSegment
	service.db.Where("resource_id = ?", resource.Id).First(&segment)
	if segment.Downloaded != 4 {
		t.Fatal("lost segment progress")
	}
}

func TestRetryRefreshFailureDoesNotChangeStoredTask(t *testing.T) {
	for _, failure := range []string{"fetch", "missing endpoints", "wrong content", "persist", "changed task"} {
		t.Run(failure, func(t *testing.T) {
			service, task, original, impl := new_retry_test_task(t)
			build := impl.build
			impl.build = func(data any, config json.RawMessage) (*adapter.DownloadTaskResult, error) {
				info, err := build(data, config)
				if failure == "missing endpoints" {
					info.Resources[0].Endpoints = nil
				}
				if failure == "wrong content" {
					info.Content.Id = "another item"
				}
				return info, err
			}
			if failure == "fetch" {
				impl.fetch = func(string) (any, error) { return nil, errors.New("offline") }
			}
			if failure == "persist" {
				if err := service.db.Migrator().DropTable(&model.ContentArticle{}); err != nil {
					t.Fatal(err)
				}
			}
			if failure == "changed task" {
				service.db.Model(&task).Update("updated_at", 2)
				task.UpdatedAt = 1
			}
			if err := service.reset_retry_task(&task); err == nil {
				t.Fatal("accepted failed refresh")
			}
			var saved model.DownloadTask
			service.db.First(&saved, task.Id)
			if saved.Name != "old task" || saved.Status != model.TaskStatusFailed || saved.ErrorMessage != "expired" {
				t.Fatalf("failed refresh changed task: %+v", saved)
			}
			var endpoint model.DownloadEndpoint
			if err := service.db.Where("resource_id = ?", original.Id).First(&endpoint).Error; err != nil || endpoint.URL != "https://cdn.example.com/expired" {
				t.Fatalf("lost old endpoint on rollback: %+v %v", endpoint, err)
			}
		})
	}
}

func TestRetryKeepsResourceSelectionAndRejectsMissingSelection(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprint(missing), func(t *testing.T) {
			service, task, original, impl := new_retry_test_task(t)
			task.UniqueID = "item_resources_selected"
			service.db.Model(&task).Update("unique_id", task.UniqueID)
			build := impl.build
			impl.build = func(data any, config json.RawMessage) (*adapter.DownloadTaskResult, error) {
				info, err := build(data, config)
				other := *info.Resources[0]
				other.Resource.UniqueID = "unselected"
				if missing {
					info.Resources = nil
				}
				info.Resources = append(info.Resources, &other)
				return info, err
			}
			err := service.reset_retry_task(&task)
			if missing {
				if err == nil {
					t.Fatal("accepted missing selected resource")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var resources []model.DownloadResource
			service.db.Where("task_id = ?", task.Id).Find(&resources)
			if len(resources) != 1 || resources[0].Id != original.Id {
				t.Fatalf("changed selection: %+v", resources)
			}
		})
	}
}

func TestRetryReplacesObsoleteResourcesWithoutDuplicatingTheTask(t *testing.T) {
	service, task, original, impl := new_retry_test_task(t)
	build := impl.build
	impl.build = func(data any, config json.RawMessage) (*adapter.DownloadTaskResult, error) {
		info, err := build(data, config)
		info.Resources[0].Resource.UniqueID = "new resource"
		return info, err
	}
	if err := service.reset_retry_task(&task); err != nil {
		t.Fatal(err)
	}
	var resources []model.DownloadResource
	service.db.Where("task_id = ?", task.Id).Find(&resources)
	if len(resources) != 1 || resources[0].Id == original.Id || resources[0].DownloadDir != "/downloads" {
		t.Fatalf("wrong replacement: %+v", resources)
	}
	var count int64
	service.db.Model(&model.DownloadTask{}).Count(&count)
	if count != 1 {
		t.Fatal("created another task")
	}
	service.db.Model(&model.DownloadSegment{}).Where("resource_id = ?", original.Id).Count(&count)
	if count != 0 {
		t.Fatal("retained orphaned segments")
	}
}

func TestRetryDirectTaskUsesExistingLinks(t *testing.T) {
	service, task, resource, impl := new_retry_test_task(t)
	task.PlatformId, task.SourceURL = "", ""
	if err := service.db.Save(&task).Error; err != nil {
		t.Fatal(err)
	}
	impl.fetch = func(string) (any, error) { t.Fatal("direct URL task must not fetch platform details"); return nil, nil }
	if err := service.reset_retry_task(&task); err != nil {
		t.Fatal(err)
	}
	var endpoint model.DownloadEndpoint
	service.db.Where("resource_id = ?", resource.Id).First(&endpoint)
	if endpoint.URL != "https://cdn.example.com/expired" || task.Status != model.TaskStatusWaiting {
		t.Fatalf("changed direct task: %+v %+v", task, endpoint)
	}
}

type retry_start_test_store struct {
	hermes.Store
	check_start func(int, int) error
}

func (s *retry_start_test_store) UpdateStatus(task_id, status int) error {
	return s.check_start(task_id, status)
}

func TestRetryRefreshesBeforeSubmittingToEngine(t *testing.T) {
	for _, action := range []string{"retry", "start", "start all"} {
		t.Run(action, func(t *testing.T) {
			service, task, resource, _ := new_retry_test_task(t)
			called := false
			service.downloader = hermes.New(hermes.HermesNewConfig{Store: &retry_start_test_store{check_start: func(task_id, status int) error {
				called = true
				var endpoint model.DownloadEndpoint
				service.db.Where("resource_id = ?", resource.Id).First(&endpoint)
				if task_id != task.Id || status != model.TaskStatusPreparing || endpoint.URL != "https://cdn.example.com/fresh" {
					t.Fatal("engine started before refreshing the endpoint")
				}
				return errors.New("stop before scheduling")
			}}})
			if action == "start all" {
				started, total, err := service.StartAllTasks("failed")
				if err != nil || started != 0 || total != 1 {
					t.Fatalf("incorrect batch result: %d/%d %v", started, total, err)
				}
			} else {
				start := service.RetryTask
				if action == "start" {
					start = service.StartTask
				}
				if _, err := start(task.Id); err == nil {
					t.Fatal("expected scheduling failure")
				}
			}
			if !called {
				t.Fatal("did not submit refreshed task")
			}
			if err := service.db.First(&task, task.Id).Error; err != nil || task.Status != model.TaskStatusFailed {
				t.Fatalf("scheduler failure left task unretryable: %+v %v", task, err)
			}
		})
	}
}
