package hermes

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/dop251/goja"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/rs/zerolog"

	"wx_channel/pkg/dm"
)

// HookManager manages the lifecycle of JS hook functions, compiling and executing user-defined
// onTaskCreate / onTaskSuccess / onTaskFailed / onTaskFinish / onFilename hooks via the goja VM.
type HookManager struct {
	mu                sync.Mutex
	vm                *goja.Runtime
	runtime           *hook_runtime
	logger            zerolog.Logger
	exports           *goja.Object
	has_create_hook   bool
	has_success_hook  bool
	has_failed_hook   bool
	has_finish_hook   bool
	has_filename_hook bool
}

// TaskInfo is the task information exposed to hooks.
type TaskInfo struct {
	ID          int            `json:"id,omitempty"`
	Name        string         `json:"name"`
	DownloadDir string         `json:"download_dir,omitempty"`
	Config      map[string]any `json:"config"`
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
func NewHookManager(loggers ...*zerolog.Logger) *HookManager {
	logger := zerolog.Nop()
	if len(loggers) > 0 && loggers[0] != nil {
		logger = *loggers[0]
	}
	return &HookManager{logger: logger}
}

// HasCreateHook returns whether an onTaskCreate hook is registered.
func (hm *HookManager) HasCreateHook() bool {
	return hm.has_create_hook
}

// HasSuccessHook returns whether an onTaskSuccess hook is registered.
func (hm *HookManager) HasSuccessHook() bool {
	return hm.has_success_hook
}

// HasFailedHook returns whether an onTaskFailed hook is registered.
func (hm *HookManager) HasFailedHook() bool {
	return hm.has_failed_hook
}

// HasFinishHook returns whether an onTaskFinish terminal hook is registered.
func (hm *HookManager) HasFinishHook() bool {
	return hm.has_finish_hook
}

// HasFilenameHook returns whether an onFilename hook is registered.
func (hm *HookManager) HasFilenameHook() bool {
	return hm.has_filename_hook
}

// Load registers named function exports, with legacy global onFilename support.
func (hm *HookManager) Load(script string) error {
	return hm.load(script, "hooks.js")
}

func (hm *HookManager) load(script, filename string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	transformed := api.Transform(script, api.TransformOptions{
		Sourcefile: filename, Loader: api.LoaderJS, Target: api.ES2020,
		Charset: api.CharsetUTF8, Format: api.FormatCommonJS, TreeShaking: api.TreeShakingFalse,
	})
	if len(transformed.Errors) > 0 {
		return fmt.Errorf("failed to compile hook script %s: %s", filename, transformed.Errors[0].Text)
	}
	runtime, err := new_hook_runtime(hm.logger)
	if err != nil {
		return fmt.Errorf("failed to initialize hook runtime: %w", err)
	}
	vm := runtime.vm

	if _, err := runtime.run(func() (goja.Value, error) { return vm.RunScript(filename, string(transformed.Code)) }); err != nil {
		return fmt.Errorf("failed to execute hook script: %w", err)
	}

	hm.vm = vm
	hm.runtime = runtime
	hm.exports = vm.NewObject()
	if module, ok := vm.Get("module").(*goja.Object); ok {
		if exports, ok := module.Get("exports").(*goja.Object); ok {
			hm.exports = exports
		}
	}
	_, hm.has_create_hook = hm.hook_function("onTaskCreate")
	_, hm.has_success_hook = hm.hook_function("onTaskSuccess")
	_, hm.has_failed_hook = hm.hook_function("onTaskFailed")
	_, hm.has_finish_hook = hm.hook_function("onTaskFinish")
	_, hm.has_filename_hook = hm.hook_function("onFilename")

	return nil
}

// LoadFile reads a JS hook script from path and loads its contents.
func (hm *HookManager) LoadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read hook script %s: %w", path, err)
	}

	return hm.load(string(data), path)
}

// InvokeCreateHook calls onTaskCreate with the original task/resources/config and returns the modified result.
// Returning nil means no modifications; the original task should be kept unchanged.
func (hm *HookManager) InvokeCreateHook(input *TaskInput) (*TaskInput, error) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if !hm.has_create_hook || hm.vm == nil {
		return nil, nil
	}

	hm.vm.Set("__basePath", input.Task.DownloadDir)

	fn, ok := hm.hook_function("onTaskCreate")
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

	result, err := hm.runtime.run(func() (goja.Value, error) {
		return fn(goja.Undefined(), task_val, resources_val, config_val)
	})
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

// InvokeSuccessHook calls onTaskSuccess after a task has completed successfully.
func (hm *HookManager) InvokeSuccessHook(task *dm.DownloadTask, download_dir string) error {
	return hm.invoke_terminal_hook("onTaskSuccess", task, download_dir)
}

// InvokeFailedHook calls onTaskFailed after a task has failed.
func (hm *HookManager) InvokeFailedHook(task *dm.DownloadTask, download_dir string) error {
	return hm.invoke_terminal_hook("onTaskFailed", task, download_dir)
}

// InvokeFinishHook calls onTaskFinish after a task reaches either terminal state.
func (hm *HookManager) InvokeFinishHook(task *dm.DownloadTask, download_dir string) error {
	return hm.invoke_terminal_hook("onTaskFinish", task, download_dir)
}

func (hm *HookManager) invoke_terminal_hook(hook_name string, task *dm.DownloadTask, download_dir string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if hm.vm == nil {
		return nil
	}
	has_hook := false
	switch hook_name {
	case "onTaskSuccess":
		has_hook = hm.has_success_hook
	case "onTaskFailed":
		has_hook = hm.has_failed_hook
	case "onTaskFinish":
		has_hook = hm.has_finish_hook
	}
	if !has_hook {
		return nil
	}
	if task == nil {
		return fmt.Errorf("%s context is nil", hook_name)
	}

	hm.vm.Set("__basePath", download_dir)

	fn, ok := hm.hook_function(hook_name)
	if !ok {
		return fmt.Errorf("%s is not a function", hook_name)
	}

	ctx_val, err := hm.to_hook_value(task)
	if err != nil {
		return fmt.Errorf("failed to prepare %s context value: %w", hook_name, err)
	}
	_, err = hm.runtime.run(func() (goja.Value, error) { return fn(goja.Undefined(), ctx_val) })
	if err != nil {
		return fmt.Errorf("%s execution failed: %w", hook_name, err)
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

	// Legacy task-scoped helpers have no output directory during filename resolution.
	hm.vm.Set("__basePath", goja.Undefined())

	fn, ok := hm.hook_function("onFilename")
	if !ok {
		return nil, fmt.Errorf("onFilename is not a function")
	}

	meta_val, err := hm.to_hook_value(params.Meta)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare onFilename meta value: %w", err)
	}
	hook_task := params.Task
	hook_task.DownloadDir = ""
	hook_task.Config = filename_hook_config(hook_task.Config)
	task_val, err := hm.to_hook_value(hook_task)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare onFilename task value: %w", err)
	}
	config_val := hm.vm.ToValue(filename_hook_config(params.Config))

	result, err := hm.runtime.run(func() (goja.Value, error) {
		return fn(goja.Undefined(), meta_val, task_val, config_val)
	})
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
		if key != "download_dir" {
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

func (hm *HookManager) hook_function(name string) (goja.Callable, bool) {
	if hm.exports != nil {
		for _, exported_name := range hm.exports.GetOwnPropertyNames() {
			if exported_name == name {
				return goja.AssertFunction(hm.exports.Get(name))
			}
		}
	}
	if name == "onFilename" && hm.vm != nil {
		return goja.AssertFunction(hm.vm.Get(name))
	}
	return nil, false
}
