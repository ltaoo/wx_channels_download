package hermes

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/dop251/goja"
)

// HookManager manages the lifecycle of JS hook functions, compiling and executing user-defined
// onTaskCreate / onTaskFinish / onFilename hooks via the goja VM.
type HookManager struct {
	mu                sync.Mutex
	vm                *goja.Runtime
	has_create_hook   bool
	has_finish_hook   bool
	has_filename_hook bool
}

// TaskInfo is the task information exposed to hooks.
type TaskInfo struct {
	Name     string         `json:"name"`
	SavePath string         `json:"save_path"`
	Config   map[string]any `json:"config"`
}

// ResourceInfo is the resource information exposed to hooks.
type ResourceInfo struct {
	ID        int               `json:"id"`
	Name      string            `json:"name"`
	Kind      string            `json:"kind"`
	Size      int64             `json:"size"`
	UniqueID  string            `json:"unique_id"`
	Extra     map[string]string `json:"extra"`
	Endpoints []EndpointInfo    `json:"endpoints"`
}

// EndpointInfo is the endpoint information exposed to hooks.
type EndpointInfo struct {
	Protocol string `json:"protocol"`
	URL      string `json:"url"`
}

// TaskInput is the input/output type for the onTaskCreate hook.
type TaskInput struct {
	Task      TaskInfo       `json:"task"`
	Config    map[string]any `json:"config"`
	Metadata  map[string]any `json:"metadata"`
	Resources []ResourceInfo `json:"resources"`
}

// FinishContext is the context for the onTaskFinish hook.
type FinishContext struct {
	Task      TaskInfo       `json:"task"`
	Config    map[string]any `json:"config"`
	Metadata  map[string]any `json:"metadata"`
	Resources []ResourceInfo `json:"resources"`
	FilePaths []string       `json:"filePaths"`
	SavePath  string         `json:"savePath"`
}

// ResourceMeta is the arbitrary resource metadata exposed to onFilename.
// It mirrors resource.Extra and adds download_at at hook invocation time.
type ResourceMeta map[string]any

// FilenameParams holds the parameters for the onFilename hook.
// The hook returns a string (filename) or null/empty string (to use the default logic).
type FilenameParams struct {
	Meta   ResourceMeta   `json:"meta"`
	Task   TaskInfo       `json:"task"`
	Config map[string]any `json:"config"`
}

// NewHookManager creates an empty HookManager.
func NewHookManager() *HookManager {
	return &HookManager{}
}

// HasCreateHook returns whether an onTaskCreate hook is registered.
func (hm *HookManager) HasCreateHook() bool {
	return hm.has_create_hook
}

// HasFinishHook returns whether an onTaskFinish hook is registered.
func (hm *HookManager) HasFinishHook() bool {
	return hm.has_finish_hook
}

// HasFilenameHook returns whether an onFilename hook is registered.
func (hm *HookManager) HasFilenameHook() bool {
	return hm.has_filename_hook
}

// Load reads and compiles a JS hook script, detecting whether onTaskCreate / onTaskFinish are defined.
func (hm *HookManager) Load(script_path string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	data, err := os.ReadFile(script_path)
	if err != nil {
		return fmt.Errorf("failed to read hook script %s: %w", script_path, err)
	}

	vm := goja.New()
	register_builtins(vm)

	if _, err := vm.RunString(string(data)); err != nil {
		return fmt.Errorf("failed to execute hook script: %w", err)
	}

	hm.vm = vm
	hm.has_create_hook = is_defined_function(vm, "onTaskCreate")
	hm.has_finish_hook = is_defined_function(vm, "onTaskFinish")
	hm.has_filename_hook = is_defined_function(vm, "onFilename")

	return nil
}

// InvokeCreateHook calls onTaskCreate with the original task/resources/config and returns the modified result.
// Returning nil means no modifications; the original task should be kept unchanged.
func (hm *HookManager) InvokeCreateHook(input *TaskInput) (*TaskInput, error) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if !hm.has_create_hook || hm.vm == nil {
		return nil, nil
	}

	hm.vm.Set("__basePath", input.Task.SavePath)

	fn, ok := goja.AssertFunction(hm.vm.Get("onTaskCreate"))
	if !ok {
		return nil, fmt.Errorf("onTaskCreate is not a function")
	}

	task_val, err := hm.to_hook_value(input.Task)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare onTaskCreate task value: %w", err)
	}
	resources_val, err := hm.to_hook_value(input.Resources)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare onTaskCreate resources value: %w", err)
	}
	config_val := hm.vm.ToValue(input.Config)

	result, err := fn(goja.Undefined(), task_val, resources_val, config_val)
	if err != nil {
		return nil, fmt.Errorf("onTaskCreate execution failed: %w", err)
	}

	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return nil, nil
	}

	exported := result.Export()
	json_bytes, err := json.Marshal(exported)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize onTaskCreate result: %w", err)
	}

	var modified TaskInput
	if err := json.Unmarshal(json_bytes, &modified); err != nil {
		return nil, fmt.Errorf("failed to deserialize onTaskCreate result: %w", err)
	}

	return &modified, nil
}

// InvokeFinishHook calls onTaskFinish to perform post-download processing (zip, cleanup, etc.).
func (hm *HookManager) InvokeFinishHook(ctx *FinishContext) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if !hm.has_finish_hook || hm.vm == nil {
		return nil
	}

	hm.vm.Set("__basePath", ctx.SavePath)

	fn, ok := goja.AssertFunction(hm.vm.Get("onTaskFinish"))
	if !ok {
		return fmt.Errorf("onTaskFinish is not a function")
	}

	ctx_val, err := hm.to_hook_value(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare onTaskFinish context value: %w", err)
	}
	_, err = fn(goja.Undefined(), ctx_val)
	if err != nil {
		return fmt.Errorf("onTaskFinish execution failed: %w", err)
	}

	return nil
}

// InvokeFilenameHook calls onFilename and returns the generated filename.
// systemName is the default system filename (after template processing); the user can adjust based on it.
// Returning an empty string means the default logic is used.
func (hm *HookManager) InvokeFilenameHook(params *FilenameParams, system_name string) (string, error) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if !hm.has_filename_hook || hm.vm == nil {
		return "", nil
	}

	hm.vm.Set("__basePath", params.Task.SavePath)

	fn, ok := goja.AssertFunction(hm.vm.Get("onFilename"))
	if !ok {
		return "", fmt.Errorf("onFilename is not a function")
	}

	system_val := hm.vm.ToValue(system_name)
	meta_val, err := hm.to_hook_value(params.Meta)
	if err != nil {
		return "", fmt.Errorf("failed to prepare onFilename meta value: %w", err)
	}
	task_val, err := hm.to_hook_value(params.Task)
	if err != nil {
		return "", fmt.Errorf("failed to prepare onFilename task value: %w", err)
	}
	config_val := hm.vm.ToValue(params.Config)

	result, err := fn(goja.Undefined(), system_val, meta_val, task_val, config_val)
	if err != nil {
		return "", fmt.Errorf("onFilename execution failed: %w", err)
	}

	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return "", nil
	}

	s := strings.TrimSpace(result.String())
	return s, nil
}

func (hm *HookManager) to_hook_value(v any) (goja.Value, error) {
	json_bytes, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var data any
	if err := json.Unmarshal(json_bytes, &data); err != nil {
		return nil, err
	}
	return hm.vm.ToValue(data), nil
}

func is_defined_function(vm *goja.Runtime, name string) bool {
	val := vm.Get(name)
	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return false
	}
	_, ok := goja.AssertFunction(val)
	return ok
}
