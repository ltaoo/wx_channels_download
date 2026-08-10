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
	SavePath string         `json:"save_path,omitempty"`
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
type FilenameParams struct {
	Meta   ResourceMeta   `json:"meta"`
	Task   TaskInfo       `json:"task"`
	Config map[string]any `json:"config"`
}

// FilenameHookResult is the structured output returned by onFilename.
// Directories lists relative directory components from outermost to innermost.
type FilenameHookResult struct {
	Name        string   `json:"name"`
	Directories []string `json:"directories"`
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

// Load compiles a JS hook script, detecting the hook functions it defines.
func (hm *HookManager) Load(script string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	vm := goja.New()
	register_builtins(vm)

	if _, err := vm.RunString(script); err != nil {
		return fmt.Errorf("failed to execute hook script: %w", err)
	}

	hm.vm = vm
	hm.has_create_hook = is_defined_function(vm, "onTaskCreate")
	hm.has_finish_hook = is_defined_function(vm, "onTaskFinish")
	hm.has_filename_hook = is_defined_function(vm, "onFilename")

	return nil
}

// LoadFile reads a JS hook script from path and loads its contents.
func (hm *HookManager) LoadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read hook script %s: %w", path, err)
	}

	return hm.Load(string(data))
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

// InvokeFilenameHook calls onFilename and returns its structured output.
// Returning null or undefined means the default naming logic is used.
func (hm *HookManager) InvokeFilenameHook(params *FilenameParams) (*FilenameHookResult, error) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if !hm.has_filename_hook || hm.vm == nil {
		return nil, nil
	}

	// Filename resolution is intentionally isolated from the task's output
	// container. Clear any base path left by another hook invocation so filename
	// hooks cannot accidentally perform filesystem operations against it.
	hm.vm.Set("__basePath", goja.Undefined())

	fn, ok := goja.AssertFunction(hm.vm.Get("onFilename"))
	if !ok {
		return nil, fmt.Errorf("onFilename is not a function")
	}

	meta_val, err := hm.to_hook_value(params.Meta)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare onFilename meta value: %w", err)
	}
	hook_task := params.Task
	hook_task.SavePath = ""
	hook_task.Config = filename_hook_config(hook_task.Config)
	task_val, err := hm.to_hook_value(hook_task)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare onFilename task value: %w", err)
	}
	config_val := hm.vm.ToValue(filename_hook_config(params.Config))

	result, err := fn(goja.Undefined(), meta_val, task_val, config_val)
	if err != nil {
		return nil, fmt.Errorf("onFilename execution failed: %w", err)
	}

	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return nil, nil
	}

	json_bytes, err := json.Marshal(result.Export())
	if err != nil {
		return nil, fmt.Errorf("failed to serialize onFilename result: %w", err)
	}
	var output FilenameHookResult
	if err := json.Unmarshal(json_bytes, &output); err != nil {
		return nil, fmt.Errorf("onFilename must return {name, directories}: %w", err)
	}
	output.Name = strings.TrimSpace(output.Name)
	if output.Name == "" {
		return nil, fmt.Errorf("onFilename result name cannot be empty")
	}
	if output.Directories == nil {
		return nil, fmt.Errorf("onFilename result directories must be an array")
	}
	for i := range output.Directories {
		output.Directories[i] = strings.TrimSpace(output.Directories[i])
		if output.Directories[i] == "" {
			return nil, fmt.Errorf("onFilename result directories[%d] cannot be empty", i)
		}
	}

	return &output, nil
}

func filename_hook_config(config map[string]any) map[string]any {
	if config == nil {
		return nil
	}
	filtered := make(map[string]any, len(config))
	for key, value := range config {
		if key != "save_path" {
			filtered[key] = value
		}
	}
	return filtered
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
