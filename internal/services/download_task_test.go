package services

import (
	"testing"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
)

func TestValidateDownloadTaskEndpointsRejectsEmptyURL(t *testing.T) {
	info := &adapter.DownloadTaskResult{Resources: []*adapter.ResourceInfo{{
		Resource: model.DownloadResource{Name: "a.mp4"},
		Endpoints: []model.DownloadEndpoint{{
			Protocol: "https",
		}},
	}}}

	err := validate_download_task_endpoints(info)
	if err == nil || err.Error() != "资源 a.mp4 的下载端点缺少 URL" {
		t.Fatalf("unexpected error: %v", err)
	}
}

type fake_resource_preparer struct {
	adapter.AdapterHandler
}

func (fake_resource_preparer) PrepareDownloadTaskResources(
	_ any,
	_ []int,
) (any, error) {
	return "prepared", nil
}

func TestPrepareDownloadTaskResourcesUsesPreparedContent(t *testing.T) {
	content, err := prepare_download_task_resources(
		fake_resource_preparer{},
		"original",
		[]int{2},
	)
	if err != nil {
		t.Fatal(err)
	}
	if content != "prepared" {
		t.Fatalf("unexpected content: %#v", content)
	}
}
