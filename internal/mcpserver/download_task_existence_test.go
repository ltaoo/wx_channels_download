package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type batch_check_reader struct {
	DataReader
	ids []string
}

func (r *batch_check_reader) CheckDownloadTaskContents(ctx context.Context, content_ids []string) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.ids = content_ids
	return map[string]any{"list": []any{map[string]any{"content_id": content_ids[0], "exists": true, "task_count": 2}}, "total": 1}, nil
}

func TestCheckDownloadTaskContentsToolBackends(t *testing.T) {
	reader := &batch_check_reader{}
	local, err := NewToolSet(Config{DataReader: reader})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := local.ExecuteTool(context.Background(), "check_download_task_contents", map[string]any{"content_ids": []string{" wxchannels:1 "}}); err != nil || !reflect.DeepEqual(reader.ids, []string{"wxchannels:1"}) {
		t.Fatalf("local batch reader failed: %v %v", reader.ids, err)
	}
	for _, arguments := range []map[string]any{{}, {"content_ids": []string{" "}}, {"content_ids": []int{1}}} {
		if _, err := local.ExecuteTool(context.Background(), "check_download_task_contents", arguments); err == nil {
			t.Fatalf("invalid arguments accepted: %+v", arguments)
		}
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		calls++
		if request.Method != "POST" || request.URL.Path != "/api/v1/download_task/check_contents" {
			t.Error("wrong batch endpoint")
		}
		var body download_task_contents_arguments
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil || !reflect.DeepEqual(body.ContentIDs, []string{"wxchannels:1", "wxchannels:2"}) {
			t.Errorf("wrong batch body: %+v %v", body, err)
		}
		_, _ = fmt.Fprint(response, `{"code":0,"data":{"list":[{"content_id":"wxchannels:1","exists":true,"task_count":1},{"content_id":"wxchannels:2","exists":false,"task_count":0}],"total":2}}`)
	}))
	defer server.Close()
	remote, err := NewToolSet(Config{APIBaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	result, err := remote.ExecuteTool(context.Background(), "check_download_task_contents", map[string]any{"content_ids": []string{"wxchannels:1", "wxchannels:2"}})
	if err != nil || calls != 1 || result == nil {
		t.Fatalf("HTTP tool did not make one batch request: %d %+v %v", calls, result, err)
	}
}
