package hermes

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dop251/goja"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/rs/zerolog"
)

// Script is a compiled local JS module. Each Run gets an isolated Hook runtime.
type Script struct {
	program *goja.Program
}

func CompileScript(filename, source string) (*Script, error) {
	transformed := api.Transform(source, api.TransformOptions{
		Sourcefile: filename, Loader: api.LoaderJS, Target: api.ES2020,
		Charset: api.CharsetUTF8, Format: api.FormatCommonJS, TreeShaking: api.TreeShakingFalse,
	})
	if len(transformed.Errors) > 0 {
		return nil, fmt.Errorf("compile %s: %s", filename, transformed.Errors[0].Text)
	}
	program, err := goja.Compile(filename, string(transformed.Code), false)
	if err != nil {
		return nil, err
	}
	return &Script{program: program}, nil
}

// Run installs host bindings, evaluates the module and awaits invoke's result.
func (script *Script) Run(ctx context.Context, logger *zerolog.Logger, install func(*goja.Runtime) error, invoke func(*goja.Runtime) (goja.Value, error)) (any, error) {
	log := zerolog.Nop()
	if logger != nil {
		log = *logger
	}
	runtime, err := new_hook_runtime(log)
	if err != nil {
		return nil, err
	}
	result, err := runtime.run_context(ctx, func() (goja.Value, error) {
		if err := install(runtime.vm); err != nil {
			return nil, err
		}
		if _, err := runtime.vm.RunProgram(script.program); err != nil {
			return nil, err
		}
		return invoke(runtime.vm)
	})
	if err != nil {
		return nil, err
	}
	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return nil, nil
	}
	exported := result.Export()
	if _, err := json.Marshal(exported); err != nil {
		return nil, fmt.Errorf("script result must be JSON serializable: %w", err)
	}
	return exported, nil
}
