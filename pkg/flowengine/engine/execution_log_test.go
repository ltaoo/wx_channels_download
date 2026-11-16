package engine

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCompactNodeLogPreservesUsefulValuesAndExecutionData(t *testing.T) {
	items := make([]interface{}, 1000)
	for index := range items {
		items[index] = map[string]interface{}{"id": index, "title": "video", "token": "nested-secret"}
	}
	input := map[string]interface{}{
		"username": "owner", "page": 17, "has_more": true, "token": "top-secret",
		"items": items, "body": strings.Repeat("正文", 100000),
		"nested": map[string]interface{}{"password": "nested-password"},
	}
	entry := NodeExecutionLog{NodeID: "fetch", Attempt: 2, Outcome: "failed", DurationMs: 123,
		Error: "upstream failed", Input: input, Output: map[string]interface{}{"count": 1000},
		RemovedOutputKeys: []string{"stale"}, NextNodeIDs: []string{"on_error"}}
	before, _ := json.Marshal(input)
	got := compact_node_execution_log(entry)
	if got.Input["username"] != "owner" || got.Input["page"] != 17 || got.Input["has_more"] != true || got.Output["count"] != 1000 {
		t.Fatalf("lost useful scalar values: %+v", got)
	}
	if got.Attempt != entry.Attempt || got.Outcome != entry.Outcome || got.Error != entry.Error || got.DurationMs != entry.DurationMs || !reflect.DeepEqual(got.RemovedOutputKeys, entry.RemovedOutputKeys) || !reflect.DeepEqual(got.NextNodeIDs, entry.NextNodeIDs) {
		t.Fatal("execution identity, result, error or routing changed")
	}
	summary := got.Input["items"].(map[string]interface{})
	if summary["count"] != 1000 || len(summary["sample"].([]interface{})) != 3 {
		t.Fatalf("array count/sample missing: %v", summary)
	}
	raw, err := json.Marshal(got)
	if err != nil || len(raw) > 3*log_section_bytes+1024 || !utf8.Valid(raw) || strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "nested-password") {
		t.Fatalf("unbounded, invalid or unredacted log: bytes=%d err=%v", len(raw), err)
	}
	after, _ := json.Marshal(input)
	if string(before) != string(after) {
		t.Fatal("compacting an audit log changed execution/checkpoint data")
	}
}

func TestCompactNodeLogBoundsWideDeepAndTypedValues(t *testing.T) {
	wide := map[string]interface{}{}
	for index := 0; index < 100; index++ {
		wide[fmt.Sprintf("field-%03d", index)] = strings.Repeat("\x00", 10000)
	}
	deep := map[string]interface{}{"value": "leaf"}
	for index := 0; index < 100; index++ {
		deep = map[string]interface{}{"child": deep}
	}
	cyclic := map[string]interface{}{}
	cyclic["self"] = cyclic
	for name, input := range map[string]map[string]interface{}{
		"wide": wide, "deep": deep, "cyclic": cyclic,
		"typed":    {"items": []map[string]interface{}{{"Password": "typed-secret", "value": "visible"}}},
		"long-key": {strings.Repeat("key", 10000): "value", "count": 3},
	} {
		t.Run(name, func(t *testing.T) {
			first := compact_log_map(input)
			raw, err := json.Marshal(first)
			if err != nil || len(raw) > log_section_bytes || strings.Contains(string(raw), "typed-secret") {
				t.Fatalf("invalid/oversized section: bytes=%d err=%v", len(raw), err)
			}
			second, _ := json.Marshal(compact_log_map(input))
			if string(raw) != string(second) {
				t.Fatal("summary changes with map iteration order")
			}
		})
	}
}

func TestNodeBehaviorOmitsDuplicatedGraphAndRouting(t *testing.T) {
	config := map[string]interface{}{"workflow": map[string]interface{}{"nodes": strings.Repeat("graph", 10000)}, "arguments": map[string]interface{}{"username": "{{input.username}}"}}
	definition := NodeDefinition{Config: config, NextNodeIDs: []string{"next"}}
	behavior := node_execution_behavior(definition, map[string]interface{}{"arguments": map[string]interface{}{"username": "owner"}})
	got := compact_log_map(behavior)
	saved_config := got["config"].(map[string]interface{})
	if _, exists := saved_config["workflow"]; exists {
		t.Fatal("nested graph repeated in every invocation")
	}
	if _, exists := got["next_node_ids"]; exists {
		t.Fatal("routing duplicated in behavior and next_node_ids")
	}
	if saved_config["arguments"].(map[string]interface{})["username"] != "{{input.username}}" || got["arguments"].(map[string]interface{})["username"] != "owner" {
		t.Fatal("lost configured and resolved service arguments")
	}
	if _, exists := config["workflow"]; !exists {
		t.Fatal("summary removed the actual workflow definition")
	}
}

func TestCompactNodeLogPrioritizesSmallParametersOverLargeSnapshots(t *testing.T) {
	input := map[string]interface{}{"username": "owner", "page": 17, "has_more": true}
	for index := 0; index < 100; index++ {
		input[fmt.Sprintf("a-snapshot-%03d", index)] = map[string]interface{}{"body": strings.Repeat("payload", 10000)}
	}
	got := compact_log_map(input)
	if got["username"] != "owner" || got["page"] != 17 || got["has_more"] != true || got["__omitted_fields__"] == nil {
		t.Fatalf("important parameters displaced by snapshots: %v", got)
	}
}
