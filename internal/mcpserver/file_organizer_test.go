package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"wx_channel/pkg/dm"
)

type test_file_organizer struct {
	request dm.OrganizeAccountFilesRequest
}

func (o *test_file_organizer) OrganizeAccountFiles(_ context.Context, request dm.OrganizeAccountFilesRequest) (*dm.OrganizeAccountFilesResult, error) {
	o.request = request
	return &dm.OrganizeAccountFilesResult{AccountID: request.AccountID, DryRun: request.DryRun == nil || *request.DryRun}, nil
}

func TestOrganizeAccountFilesTool(t *testing.T) {
	organizer := &test_file_organizer{}
	local, err := NewToolSet(Config{FileOrganizer: organizer})
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	arguments := map[string]any{"account_id": " wxchannels:author ", "output_dir": directory}
	if _, err := local.ExecuteTool(context.Background(), "organize_account_files", arguments); err != nil || organizer.request.AccountID != "wxchannels:author" || organizer.request.DryRun != nil {
		t.Fatalf("wrong local request: %+v %v", organizer.request, err)
	}
	for _, invalid := range []map[string]any{{}, {"account_id": "x", "output_dir": " "}, {"account_id": "x", "output_dir": directory, "dry_run": "false"}} {
		if _, err := local.ExecuteTool(context.Background(), "organize_account_files", invalid); err == nil {
			t.Fatalf("accepted invalid arguments: %+v", invalid)
		}
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		calls++
		if request.Method != "POST" || request.URL.Path != "/api/account/organize_files" {
			t.Errorf("wrong endpoint: %s %s", request.Method, request.URL.Path)
		}
		var body dm.OrganizeAccountFilesRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body.AccountID != "wxchannels:author" || body.OutputDir != directory || body.DryRun == nil || *body.DryRun {
			t.Errorf("wrong remote arguments: %+v %v", body, err)
		}
		_, _ = fmt.Fprint(response, `{"code":0,"data":{"account_id":"wxchannels:author","dry_run":false,"files":[]}}`)
	}))
	defer server.Close()
	remote, err := NewToolSet(Config{APIBaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	arguments["dry_run"] = false
	result, err := remote.ExecuteTool(context.Background(), "organize_account_files", arguments)
	if err != nil || result == nil || calls != 1 {
		t.Fatalf("remote tool failed: %+v %v calls=%d", result, err, calls)
	}
	for _, runtime := range []*ToolSet{local, remote} {
		if !runtime.supports_tool("organize_account_files") {
			t.Fatal("tool unavailable")
		}
	}
}
