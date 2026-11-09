package dm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestDownloadTaskDetailAndWaitShareContract(t *testing.T) {
	content_id := "wxchannels:42"
	expected := DownloadTask{
		ID: 42, Name: "测试任务", PlatformID: "wxchannels", Status: 5,
		ContentID: &content_id, ContentType: "video", RootTaskID: 42,
		Content:    &DownloadTaskContent{ID: content_id, Title: "测试内容", Accounts: []DownloadTaskAccount{{ID: "author", Nickname: "作者"}}},
		ConfigJSON: `{"suffix":".mp4"}`, MetadataJSON: `{"title":"测试内容"}`,
		Size: 123, Downloaded: 123, Progress: 100, CreatedAt: 1000, UpdatedAt: 2000, FileCount: 1,
		Files: []DownloadTaskFile{{ID: 7, Name: "成片.mp4", DownloadDir: "/downloads", FilePath: "/downloads/成片.mp4", LocalPath: "/downloads/成片.mp4", Exists: true, Kind: "video/mp4", Type: "FILE", Status: "finished", Size: 123, Downloaded: 123, Progress: 100, FileType: "video", FileURL: "/api/file?path=example"}},
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/download_task/detail" || request.URL.Query().Get("id") != "42" {
			t.Errorf("unexpected endpoint: %s", request.URL)
		}
		task := expected
		if requests.Add(1) <= 2 {
			task.Status = 4
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"code": 0, "data": task})
	}))
	defer server.Close()
	client, err := NewClient(ClientOptions{BaseURL: server.URL, PollInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := client.DownloadTaskDetail(context.Background(), 42)
	if err != nil || detail.Status != 4 || detail.Content.Accounts[0].Nickname != "作者" {
		t.Fatalf("detail=%+v, err=%v", detail, err)
	}
	completed, err := client.WaitDownloadTask(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 3 || !reflect.DeepEqual(completed, &expected) {
		t.Fatalf("completed=%+v, requests=%d; want %+v", completed, requests.Load(), expected)
	}
	queried, err := client.DownloadTaskDetail(context.Background(), 42)
	if err != nil || !reflect.DeepEqual(queried, completed) {
		t.Fatalf("detail=%+v, completed=%+v, err=%v", queried, completed, err)
	}
}

func TestWaitDownloadTaskFailureIncludesDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]any{"code": 0, "data": DownloadTask{ID: 42, Status: 6, Error: "后处理失败", Files: []DownloadTaskFile{{ID: 7, Name: "original.part"}}}})
	}))
	defer server.Close()
	client, err := NewClient(ClientOptions{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.WaitDownloadTask(context.Background(), 42)
	var failed *TaskFailedError
	if !errors.As(err, &failed) || failed.Status != 6 || failed.Message != "后处理失败" || failed.Task.Files[0].ID != 7 {
		t.Fatalf("expected typed failure with task detail, got %v", err)
	}
}
