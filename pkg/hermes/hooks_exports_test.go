package hermes

import (
	"testing"

	"wx_channel/pkg/dm"
)

func TestHooksRequireNamedFunctionExports(t *testing.T) {
	for _, test_case := range []struct {
		name            string
		script          string
		expected        bool
		legacy_filename bool
	}{
		{name: "private functions", legacy_filename: true, script: `
function onTaskCreate() { throw new Error("private hook called"); }
function onTaskSuccess() { throw new Error("private hook called"); }
function onTaskFailed() { throw new Error("private hook called"); }
function onTaskFinish() { throw new Error("private hook called"); }
function onFilename() {}
`},
		{name: "export declarations", expected: true, script: `
export function onTaskCreate() {}
export async function onTaskSuccess() { await Promise.resolve(); }
export function onTaskFailed() {}
export function onTaskFinish() {}
export function onFilename() {}
`},
		{name: "export bindings and aliases", expected: true, script: `
export const onTaskCreate = () => null;
const success = async () => { await Promise.resolve(); };
const failed = () => {}, finish = () => {}, filename = () => null;
export {success as onTaskSuccess, failed as onTaskFailed, finish as onTaskFinish, filename as onFilename};
`},
		{name: "default export", script: `export default function onTaskSuccess() { throw new Error("default hook called"); }`},
		{name: "non-function exports", script: `
export const onTaskCreate = null, onTaskSuccess = 1, onTaskFailed = {}, onTaskFinish = "value", onFilename = true;
`},
		{name: "inherited function", script: `
Object.prototype.onTaskSuccess = () => { throw new Error("inherited hook called"); };
export const other = true;
`},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			hooks := NewHookManager()
			if err := hooks.Load(test_case.script); err != nil {
				t.Fatal(err)
			}
			for name, found := range map[string]bool{
				"create": hooks.HasCreateHook(), "success": hooks.HasSuccessHook(), "failed": hooks.HasFailedHook(),
				"finish": hooks.HasFinishHook(), "filename": hooks.HasFilenameHook(),
			} {
				expected := test_case.expected || name == "filename" && test_case.legacy_filename
				if found != expected {
					t.Errorf("%s registered=%t, want %t", name, found, expected)
				}
			}
			if _, err := hooks.InvokeCreateHook(&TaskInput{}); err != nil {
				t.Fatal(err)
			}
			task := &dm.DownloadTask{ID: 42}
			for _, invoke := range []func(*dm.DownloadTask, string) error{hooks.InvokeSuccessHook, hooks.InvokeFailedHook, hooks.InvokeFinishHook} {
				if err := invoke(task, ""); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := hooks.InvokeFilenameHook(&FilenameParams{}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOnFilenameLegacyCompatibility(t *testing.T) {
	hooks := NewHookManager()
	for _, test_case := range []struct {
		name     string
		script   string
		expected string
	}{
		{name: "legacy", script: `function onFilename(meta) { return {name: meta.title, directories: []}; }`, expected: "legacy"},
		{name: "legacy with imports", script: `import fs from "fs"; function onFilename(meta) { return {name: meta.title, directories: []}; }`, expected: "legacy"},
		{name: "export takes priority", script: `
function onFilename() { throw new Error("legacy function must not override export"); }
const handler = () => ({name: "exported", directories: []});
export {handler as onFilename};`, expected: "exported"},
		{name: "legacy after reload", script: `function onFilename(meta) { return {name: meta.title, directories: []}; }`, expected: "legacy"},
		{name: "non-function export", script: `function onFilename() { throw new Error("invalid export must not fall back"); } const handler = 1; export {handler as onFilename};`},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			if err := hooks.Load(test_case.script + `
function onTaskSuccess() { throw new Error("task hooks still require export"); }
`); err != nil {
				t.Fatal(err)
			}
			if hooks.HasSuccessHook() || hooks.HasFilenameHook() != (test_case.expected != "") {
				t.Fatal("incorrect hook registration")
			}
			result, err := hooks.InvokeFilenameHook(&FilenameParams{Meta: ResourceMeta{"title": "legacy"}})
			if err != nil {
				t.Fatal(err)
			}
			if test_case.expected == "" {
				if result != nil {
					t.Fatalf("invalid export was called: %+v", result)
				}
			} else if result == nil || result.Name != test_case.expected {
				t.Fatalf("filename=%+v, want %s", result, test_case.expected)
			}
		})
	}
}

func TestHookInvocationUsesExportAndReloadClearsIt(t *testing.T) {
	hooks := NewHookManager()
	if err := hooks.Load(`
function onTaskSuccess() { throw new Error("private global used instead of export"); }
const handler = task => { globalThis.exported_task_id = task.id; };
export {handler as onTaskSuccess};
`); err != nil {
		t.Fatal(err)
	}
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{ID: 42}, ""); err != nil {
		t.Fatal(err)
	}
	if value := hooks.vm.Get("exported_task_id"); value == nil || value.ToInteger() != 42 {
		t.Fatalf("exported function was not called: %v", value)
	}
	if err := hooks.Load(`function onTaskSuccess() { throw new Error("private hook called after reload"); }`); err != nil {
		t.Fatal(err)
	}
	if hooks.HasSuccessHook() {
		t.Fatal("previous export remained registered after reload")
	}
	if err := hooks.InvokeSuccessHook(&dm.DownloadTask{}, ""); err != nil {
		t.Fatal(err)
	}
}
