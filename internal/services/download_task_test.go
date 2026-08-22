package services

import (
	"testing"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
)

func Test_select_download_task_resources(t *testing.T) {
	info := &adapter.DownloadTaskResult{
		Task: &model.DownloadTask{UniqueID: "content-1"},
		Resources: []*adapter.ResourceInfo{
			{Resource: model.DownloadResource{UniqueID: "resource-1"}},
			{Resource: model.DownloadResource{UniqueID: "resource-2"}},
		},
	}

	if err := SelectDownloadTaskResources(info, []int{1}); err != nil {
		t.Fatalf("SelectDownloadTaskResources() error = %v", err)
	}
	if len(info.Resources) != 1 {
		t.Fatalf("resource count = %d, want 1", len(info.Resources))
	}
	if got := info.Resources[0].Resource.UniqueID; got != "resource-2" {
		t.Fatalf("selected resource = %q, want resource-2", got)
	}
	if got := info.Task.UniqueID; got == "content-1" || got == "" {
		t.Fatalf("selected task unique id = %q, want selection-specific id", got)
	}
}

func Test_select_download_task_resources_rejects_invalid_index(t *testing.T) {
	info := &adapter.DownloadTaskResult{
		Resources: []*adapter.ResourceInfo{
			{Resource: model.DownloadResource{UniqueID: "resource-1"}},
		},
	}

	if err := SelectDownloadTaskResources(info, []int{1}); err == nil {
		t.Fatal("SelectDownloadTaskResources() error = nil, want invalid index error")
	}
}
