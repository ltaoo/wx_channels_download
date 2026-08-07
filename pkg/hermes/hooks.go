package hermes

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/dop251/goja"
)

// HookManager manages the lifecycle of JS hook functions, compiling and executing user-defined
// onTaskCreate / onTaskFinish / onFilename hooks via the goja VM.
type HookManager struct {
	vm              *goja.Runtime
	hasCreateHook   bool
	hasFinishHook   bool
	hasFilenameHook bool
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

// ResourceMeta is the second argument for the onFilename hook, containing flat video metadata expanded from resource.Extra.
// Fields vary by platform and are all optional.
type ResourceMeta struct {
	ID         string `json:"id"`          // video ID
	Title      string `json:"title"`       // video title
	Spec       string `json:"spec"`        // video quality, e.g. "original", "xWT111"
	CreatedAt  int64  `json:"created_at"`  // video publish time (seconds)
	DownloadAt int64  `json:"download_at"` // download time (seconds)
	Author     string `json:"author"`      // creator/uploader name
	Platform   string `json:"platform"`    // platform identifier, e.g. "wxchannels"
	Idx        int    `json:"idx"`         // media index for multi-resource posts
}

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
	return hm.hasCreateHook
}

// HasFinishHook returns whether an onTaskFinish hook is registered.
func (hm *HookManager) HasFinishHook() bool {
	return hm.hasFinishHook
}

// HasFilenameHook returns whether an onFilename hook is registered.
func (hm *HookManager) HasFilenameHook() bool {
	return hm.hasFilenameHook
}

// Load reads and compiles a JS hook script, detecting whether onTaskCreate / onTaskFinish are defined.
func (hm *HookManager) Load(scriptPath string) error {
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to read hook script %s: %w", scriptPath, err)
	}

	vm := goja.New()
	registerBuiltins(vm)

	if _, err := vm.RunString(string(data)); err != nil {
		return fmt.Errorf("failed to execute hook script: %w", err)
	}

	hm.vm = vm
	hm.hasCreateHook = isDefinedFunction(vm, "onTaskCreate")
	hm.hasFinishHook = isDefinedFunction(vm, "onTaskFinish")
	hm.hasFilenameHook = isDefinedFunction(vm, "onFilename")

	return nil
}

// InvokeCreateHook calls onTaskCreate with the original task/resources/config and returns the modified result.
// Returning nil means no modifications; the original task should be kept unchanged.
func (hm *HookManager) InvokeCreateHook(input *TaskInput) (*TaskInput, error) {
	if !hm.hasCreateHook || hm.vm == nil {
		return nil, nil
	}

	hm.vm.Set("__basePath", input.Task.SavePath)

	fn, ok := goja.AssertFunction(hm.vm.Get("onTaskCreate"))
	if !ok {
		return nil, fmt.Errorf("onTaskCreate is not a function")
	}

	taskVal, err := hm.toHookValue(input.Task)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare onTaskCreate task value: %w", err)
	}
	resourcesVal, err := hm.toHookValue(input.Resources)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare onTaskCreate resources value: %w", err)
	}
	configVal := hm.vm.ToValue(input.Config)

	result, err := fn(goja.Undefined(), taskVal, resourcesVal, configVal)
	if err != nil {
		return nil, fmt.Errorf("onTaskCreate execution failed: %w", err)
	}

	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return nil, nil
	}

	exported := result.Export()
	jsonBytes, err := json.Marshal(exported)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize onTaskCreate result: %w", err)
	}

	var modified TaskInput
	if err := json.Unmarshal(jsonBytes, &modified); err != nil {
		return nil, fmt.Errorf("failed to deserialize onTaskCreate result: %w", err)
	}

	return &modified, nil
}

// InvokeFinishHook calls onTaskFinish to perform post-download processing (zip, cleanup, etc.).
func (hm *HookManager) InvokeFinishHook(ctx *FinishContext) error {
	if !hm.hasFinishHook || hm.vm == nil {
		return nil
	}

	hm.vm.Set("__basePath", ctx.SavePath)

	fn, ok := goja.AssertFunction(hm.vm.Get("onTaskFinish"))
	if !ok {
		return fmt.Errorf("onTaskFinish is not a function")
	}

	ctxVal, err := hm.toHookValue(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare onTaskFinish context value: %w", err)
	}
	_, err = fn(goja.Undefined(), ctxVal)
	if err != nil {
		return fmt.Errorf("onTaskFinish execution failed: %w", err)
	}

	return nil
}

// InvokeFilenameHook calls onFilename and returns the generated filename.
// systemName is the default system filename (after template processing); the user can adjust based on it.
// Returning an empty string means the default logic is used.
func (hm *HookManager) InvokeFilenameHook(params *FilenameParams, systemName string) (string, error) {
	if !hm.hasFilenameHook || hm.vm == nil {
		return "", nil
	}

	hm.vm.Set("__basePath", params.Task.SavePath)

	fn, ok := goja.AssertFunction(hm.vm.Get("onFilename"))
	if !ok {
		return "", fmt.Errorf("onFilename is not a function")
	}

	systemVal := hm.vm.ToValue(systemName)
	metaVal, err := hm.toHookValue(params.Meta)
	if err != nil {
		return "", fmt.Errorf("failed to prepare onFilename meta value: %w", err)
	}
	taskVal, err := hm.toHookValue(params.Task)
	if err != nil {
		return "", fmt.Errorf("failed to prepare onFilename task value: %w", err)
	}
	configVal := hm.vm.ToValue(params.Config)

	result, err := fn(goja.Undefined(), systemVal, metaVal, taskVal, configVal)
	if err != nil {
		return "", fmt.Errorf("onFilename execution failed: %w", err)
	}

	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return "", nil
	}

	s := strings.TrimSpace(result.String())
	return s, nil
}

func (hm *HookManager) toHookValue(v any) (goja.Value, error) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var data any
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return nil, err
	}
	return hm.vm.ToValue(data), nil
}

func isDefinedFunction(vm *goja.Runtime, name string) bool {
	val := vm.Get(name)
	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return false
	}
	_, ok := goja.AssertFunction(val)
	return ok
}
