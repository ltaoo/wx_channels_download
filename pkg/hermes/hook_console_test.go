package hermes

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"wx_channel/pkg/dm"
)

func TestHookConsoleLevelsFormattingWithoutImplicitContext(t *testing.T) {
	var output bytes.Buffer
	logger := zerolog.New(&output).Level(zerolog.DebugLevel)
	hooks := NewHookManager(&logger)
	if err := hooks.Load(`import console from "node:console";
console.info("script loaded");
export async function onTaskSuccess(task) {
  console.log("task %d %s %j", task.id, "ready", {file: "成片.mp4"});
  console.info("files", {count: 2, task_id: task.id, hook: "explicitHook", file: "manual.js"});
  console.warn("warn");
  console.debug("debug");
  const value = {}; value.self = value;
  console.log("circular", value);
  await Promise.resolve();
  console.error("failure", new Error("upload failed"));
}
export function onTaskFinish() { console.log("finished"); }
`); err != nil {
		t.Fatal(err)
	}
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{ID: 42}, ""); err != nil {
		t.Fatal(err)
	}
	if err := hooks.InvokeFinishHook(&dm.DownloadTask{ID: 43}, ""); err != nil {
		t.Fatal(err)
	}
	var entries []map[string]any
	decoder := json.NewDecoder(&output)
	for decoder.More() {
		var entry map[string]any
		if err := decoder.Decode(&entry); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	if len(entries) != 8 {
		t.Fatalf("expected eight log entries, got %d: %v", len(entries), entries)
	}
	for _, entry := range entries {
		if len(entry) != 2 || entry["level"] == nil || entry["message"] == nil {
			t.Fatalf("logger added implicit fields: %v", entry)
		}
	}
	for index, level := range []string{"info", "info", "warn", "debug", "info", "error"} {
		entry := entries[index+1]
		if entry["level"] != level {
			t.Errorf("wrong log level: %v", entry)
		}
	}
	if entries[1]["message"] != `task 42 ready {"file":"成片.mp4"}` || entries[2]["message"] != `files {"count":2,"task_id":42,"hook":"explicitHook","file":"manual.js"}` {
		t.Fatalf("wrong formatting: %v", entries)
	}
	if !strings.Contains(entries[5]["message"].(string), "[Circular]") || !strings.Contains(entries[6]["message"].(string), "hooks.js:") || !strings.Contains(entries[6]["message"].(string), "upload failed") {
		t.Fatalf("circular object or error stack was lost: %v", entries)
	}
	if entries[7]["message"] != "finished" {
		t.Fatalf("unexpected log message: %v", entries[7])
	}
}

func TestHookRejectionKeepsStack(t *testing.T) {
	hooks := NewHookManager()
	if err := hooks.Load(`export async function onTaskSuccess() { await Promise.resolve(); throw new Error("upload failed"); }`); err != nil {
		t.Fatal(err)
	}
	err := hooks.InvokeSuccessHook(&dm.DownloadTask{}, "")
	if err == nil || !strings.Contains(err.Error(), "upload failed") || !strings.Contains(err.Error(), "hooks.js:") {
		t.Fatalf("rejection lost its script stack: %v", err)
	}
}
