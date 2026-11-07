package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSyncSkillAndExplicitScope(t *testing.T) {
	server, tools, err := NewRuntime(Config{APIBaseURL: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	resources := server.Resources()
	if len(resources) != 1 || resources[0].URI != sync_skill_uri {
		t.Fatalf("skill resource missing: %+v", resources)
	}
	skill, err := tools.ExecuteTool(context.Background(), "get_sync_skill", nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(skill)
	if err != nil || len(encoded) < 100 {
		t.Fatalf("skill content missing: %s %v", encoded, err)
	}
	parts := strings.SplitN(sync_skill_text, "---\n", 3)
	if len(parts) != 3 {
		t.Fatal("embedded skill is missing frontmatter")
	}
	var metadata map[string]string
	if err := yaml.Unmarshal([]byte(parts[1]), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["name"] != "sync-downloads" || strings.TrimSpace(metadata["description"]) == "" || len([]rune(metadata["description"])) > 1024 {
		t.Fatalf("invalid skill metadata: %+v", metadata)
	}
	for _, arguments := range []map[string]any{
		{"server_id": 0},
		{"server_id": 0, "task_ids": []int{}},
		{"server_id": 0, "task_ids": []int{-1}},
		{"server_id": 0, "task_ids": []int{1}, "all": true},
	} {
		_, err := tools.ExecuteTool(context.Background(), "sync_to_server", arguments)
		if err == nil || !strings.Contains(err.Error(), "task_ids") {
			t.Fatalf("invalid scope accepted: %+v", arguments)
		}
	}
}
