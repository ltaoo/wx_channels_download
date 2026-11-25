package services

import (
	"encoding/json"
	"testing"

	"wx_channel/internal/database/model"
)

func TestUpdateTask(t *testing.T) {
	db := new_task_safety_db(t)
	if err := db.AutoMigrate(&model.DownloadEndpoint{}); err != nil {
		t.Fatal(err)
	}
	task := model.DownloadTask{Name: "old", PlatformId: "test", Status: model.TaskStatusFailed, Timestamps: model.Timestamps{CreatedAt: 1}}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	resource := model.DownloadResource{TaskId: &task.Id, Type: model.ResourceTypeStream, StreamURL: "https://example.com/old", Timestamps: model.Timestamps{CreatedAt: 1}}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	endpoint := model.DownloadEndpoint{ResourceId: resource.Id, Protocol: "HTTPS", URL: resource.StreamURL, Headers: `{"Referer":"https://example.com"}`, Cookies: "session=one", Enabled: 1}
	if err := db.Create(&endpoint).Error; err != nil {
		t.Fatal(err)
	}
	service := NewDownloadTaskService(db, nil, nil, nil, "", "", nil)
	var body UpdateDownloadTaskBody
	if err := json.Unmarshal([]byte(`{"id":1,"name":"new","files":[{"id":1,"url":"https://example.com/new?token=ok"}]}`), &body); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateTask(body); err != nil {
		t.Fatal(err)
	}
	db.First(&task, task.Id)
	db.First(&resource, resource.Id)
	db.First(&endpoint, endpoint.Id)
	if task.Name != "new" || task.Status != model.TaskStatusFailed || endpoint.URL != body.Files[0].URL || resource.StreamURL != endpoint.URL || endpoint.Headers == "" || endpoint.Cookies != "session=one" {
		t.Fatalf("updated task/resource/endpoint incorrectly: %+v %+v %+v", task, resource, endpoint)
	}
	for _, status := range []int{model.TaskStatusWaiting, model.TaskStatusDownloading, model.TaskStatusFinished} {
		db.Model(&task).Update("status", status)
		if err := service.UpdateTask(body); err == nil {
			t.Fatalf("editable status %d", status)
		}
	}
	db.Model(&task).Update("status", model.TaskStatusCancelled)
	body.Name = "should roll back"
	body.Files[0].ID = resource.Id + 1
	if err := service.UpdateTask(body); err == nil {
		t.Fatal("accepted a resource outside the task")
	}
	db.First(&task, task.Id)
	if task.Name != "new" {
		t.Fatal("invalid resource did not roll back the task name")
	}
	body.Files[0].ID = resource.Id
	body.Files[0].URL = "javascript:alert(1)"
	if err := service.UpdateTask(body); err == nil {
		t.Fatal("accepted a non-HTTP URL")
	}
	body.Files[0].URL = "https://example.com/replaced"
	if err := db.Delete(&endpoint).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateTask(body); err != nil {
		t.Fatalf("could not add a missing endpoint: %v", err)
	}
	var added model.DownloadEndpoint
	if err := db.Where("resource_id = ?", resource.Id).First(&added).Error; err != nil || added.URL != body.Files[0].URL || added.Enabled != 1 {
		t.Fatalf("missing replacement endpoint: %+v, %v", added, err)
	}
}
