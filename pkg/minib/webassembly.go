package minib

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dop251/goja"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type webassembly_host struct {
	runtime       *page_runtime
	wasm_runtimes []wazero.Runtime
	modules       map[*goja.Object][]byte
}

func (runtime *page_runtime) install_webassembly(window *goja.Object) error {
	host := &webassembly_host{
		runtime: runtime,
		modules: make(map[*goja.Object][]byte),
	}
	runtime.webassembly = host
	webassembly_object := runtime.vm.NewObject()

	if err := webassembly_object.Set("instantiate", func(call goja.FunctionCall) (result goja.Value) {
		promise, resolve, reject := runtime.vm.NewPromise()
		result = runtime.vm.ToValue(promise)
		defer func() {
			if recovered := recover(); recovered != nil {
				_ = reject(recovered)
			}
		}()
		_ = resolve(host.instantiate(call))
		return result
	}); err != nil {
		return err
	}
	if err := webassembly_object.Set("compile", func(call goja.FunctionCall) goja.Value {
		promise, resolve, reject := runtime.vm.NewPromise()
		wasm_bytes, err := javascript_bytes(call.Argument(0))
		if err != nil {
			_ = reject(runtime.vm.NewTypeError(err.Error()))
			return runtime.vm.ToValue(promise)
		}
		if err := host.validate(wasm_bytes); err != nil {
			_ = reject(runtime.vm.NewGoError(err))
			return runtime.vm.ToValue(promise)
		}
		module_object := runtime.vm.NewObject()
		_ = module_object.SetSymbol(goja.SymToStringTag, runtime.vm.ToValue("WebAssembly.Module"))
		host.modules[module_object] = append([]byte(nil), wasm_bytes...)
		_ = resolve(module_object)
		return runtime.vm.ToValue(promise)
	}); err != nil {
		return err
	}
	if err := webassembly_object.Set("validate", func(call goja.FunctionCall) goja.Value {
		wasm_bytes, err := javascript_bytes(call.Argument(0))
		return runtime.vm.ToValue(err == nil && host.validate(wasm_bytes) == nil)
	}); err != nil {
		return err
	}
	return window.Set("WebAssembly", webassembly_object)
}

func (runtime *page_runtime) close_webassembly() {
	if runtime == nil || runtime.webassembly == nil {
		return
	}
	runtime.webassembly.close()
	runtime.webassembly = nil
}

func (host *webassembly_host) close() {
	for _, wasm_runtime := range host.wasm_runtimes {
		_ = wasm_runtime.Close(context.Background())
	}
	host.wasm_runtimes = nil
	host.modules = nil
}

func (host *webassembly_host) operation_context() context.Context {
	if host != nil && host.runtime != nil && host.runtime.ctx != nil {
		return host.runtime.ctx
	}
	return context.Background()
}

func new_wazero_runtime(ctx context.Context) wazero.Runtime {
	runtime_config := wazero.NewRuntimeConfig().
		WithCloseOnContextDone(true).
		WithMemoryLimitPages(1024).
		WithMemoryCapacityFromMax(true)
	return wazero.NewRuntimeWithConfig(ctx, runtime_config)
}

func (host *webassembly_host) validate(wasm_bytes []byte) error {
	ctx := host.operation_context()
	wasm_runtime := new_wazero_runtime(ctx)
	defer wasm_runtime.Close(context.Background())
	compiled, err := wasm_runtime.CompileModule(ctx, wasm_bytes)
	if err != nil {
		return fmt.Errorf("compile WebAssembly: %w", err)
	}
	return compiled.Close(ctx)
}

func (host *webassembly_host) instantiate(call goja.FunctionCall) goja.Value {
	vm := host.runtime.vm
	module_input := false
	var wasm_bytes []byte
	if object, ok := call.Argument(0).(*goja.Object); ok {
		wasm_bytes, module_input = host.modules[object]
	}
	if !module_input {
		var err error
		wasm_bytes, err = javascript_bytes(call.Argument(0))
		if err != nil {
			panic(vm.NewTypeError(err.Error()))
		}
	}

	ctx := host.operation_context()
	wasm_runtime := new_wazero_runtime(ctx)
	keep_runtime := false
	defer func() {
		if !keep_runtime {
			_ = wasm_runtime.Close(context.Background())
		}
	}()
	compiled, err := wasm_runtime.CompileModule(ctx, wasm_bytes)
	if err != nil {
		panic(vm.NewTypeError(fmt.Sprintf("compile WebAssembly: %v", err)))
	}
	defer compiled.Close(ctx)
	if len(compiled.ImportedMemories()) != 0 {
		panic(vm.NewTypeError("WebAssembly modules with imported memory are not supported"))
	}
	imports := call.Argument(1)
	if imports == nil || goja.IsUndefined(imports) || goja.IsNull(imports) {
		imports = vm.NewObject()
	}
	imports_object := imports.ToObject(vm)

	type imported_function struct {
		definition api.FunctionDefinition
		callback   goja.Callable
		name       string
	}
	modules := make(map[string][]imported_function)
	for _, definition := range compiled.ImportedFunctions() {
		module_name, import_name, imported := definition.Import()
		if !imported {
			continue
		}
		module_value := imports_object.Get(module_name)
		if module_value == nil || goja.IsUndefined(module_value) || goja.IsNull(module_value) {
			panic(vm.NewTypeError(fmt.Sprintf("WebAssembly import module %q is missing", module_name)))
		}
		callback, ok := goja.AssertFunction(module_value.ToObject(vm).Get(import_name))
		if !ok {
			panic(vm.NewTypeError(fmt.Sprintf("WebAssembly import %s.%s is not a function", module_name, import_name)))
		}
		modules[module_name] = append(modules[module_name], imported_function{
			definition: definition,
			callback:   callback,
			name:       import_name,
		})
	}

	for module_name, functions := range modules {
		builder := wasm_runtime.NewHostModuleBuilder(module_name)
		for _, imported := range functions {
			imported := imported
			builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(
				func(_ context.Context, _ api.Module, stack []uint64) {
					arguments := make([]goja.Value, len(imported.definition.ParamTypes()))
					for index, value_type := range imported.definition.ParamTypes() {
						arguments[index] = webassembly_to_javascript(vm, stack[index], value_type)
					}
					result, call_err := imported.callback(goja.Undefined(), arguments...)
					if call_err != nil {
						panic(call_err)
					}
					result_types := imported.definition.ResultTypes()
					if len(result_types) == 1 {
						encoded, encode_err := javascript_to_webassembly(result, result_types[0])
						if encode_err != nil {
							panic(encode_err)
						}
						stack[0] = encoded
					} else if len(result_types) > 1 {
						result_object := result.ToObject(vm)
						for index, value_type := range result_types {
							encoded, encode_err := javascript_to_webassembly(result_object.Get(fmt.Sprintf("%d", index)), value_type)
							if encode_err != nil {
								panic(encode_err)
							}
							stack[index] = encoded
						}
					}
				}), imported.definition.ParamTypes(), imported.definition.ResultTypes()).Export(imported.name)
		}
		if _, err := builder.Instantiate(ctx); err != nil {
			panic(vm.NewGoError(fmt.Errorf("instantiate WebAssembly host module %s: %w", module_name, err)))
		}
	}

	module_config := wazero.NewModuleConfig().WithStartFunctions()
	instance, err := wasm_runtime.InstantiateModule(ctx, compiled, module_config)
	if err != nil {
		panic(vm.NewGoError(fmt.Errorf("instantiate WebAssembly module: %w", err)))
	}
	instance_object := vm.NewObject()
	exports_object := vm.NewObject()
	for export_name, definition := range instance.ExportedFunctionDefinitions() {
		function := instance.ExportedFunction(export_name)
		definition := definition
		if err := exports_object.Set(export_name, func(call goja.FunctionCall) goja.Value {
			params := make([]uint64, len(definition.ParamTypes()))
			for index, value_type := range definition.ParamTypes() {
				encoded, encode_err := javascript_to_webassembly(call.Argument(index), value_type)
				if encode_err != nil {
					panic(vm.NewTypeError(encode_err.Error()))
				}
				params[index] = encoded
			}
			results, call_err := function.Call(host.operation_context(), params...)
			if call_err != nil {
				panic(vm.NewGoError(call_err))
			}
			if len(results) == 0 {
				return goja.Undefined()
			}
			if len(results) == 1 {
				return webassembly_to_javascript(vm, results[0], definition.ResultTypes()[0])
			}
			values := make([]any, len(results))
			for index, result := range results {
				values[index] = webassembly_to_javascript(vm, result, definition.ResultTypes()[index])
			}
			return vm.ToValue(values)
		}); err != nil {
			panic(vm.NewGoError(err))
		}
	}
	for export_name := range instance.ExportedMemoryDefinitions() {
		memory := instance.ExportedMemory(export_name)
		memory_object := vm.NewObject()
		var memory_buffer goja.ArrayBuffer
		var memory_size uint32
		has_memory_buffer := false
		getter := vm.ToValue(func(goja.FunctionCall) goja.Value {
			current_size := memory.Size()
			if !has_memory_buffer || current_size != memory_size {
				if has_memory_buffer {
					memory_buffer.Detach()
				}
				data, ok := memory.Read(0, current_size)
				if !ok {
					panic(vm.NewTypeError("WebAssembly memory is unavailable"))
				}
				memory_buffer = vm.NewArrayBuffer(data)
				memory_size = current_size
				has_memory_buffer = true
			}
			return vm.ToValue(memory_buffer)
		})
		if err := memory_object.DefineAccessorProperty("buffer", getter, nil, goja.FLAG_FALSE, goja.FLAG_TRUE); err != nil {
			panic(vm.NewGoError(err))
		}
		if err := memory_object.Set("grow", func(call goja.FunctionCall) goja.Value {
			previous, ok := memory.Grow(uint32(call.Argument(0).ToInteger()))
			if !ok {
				panic(vm.NewTypeError("WebAssembly memory growth failed"))
			}
			if has_memory_buffer {
				memory_buffer.Detach()
				has_memory_buffer = false
			}
			return vm.ToValue(previous)
		}); err != nil {
			panic(vm.NewGoError(err))
		}
		if err := exports_object.Set(export_name, memory_object); err != nil {
			panic(vm.NewGoError(err))
		}
	}
	if err := instance_object.Set("exports", exports_object); err != nil {
		panic(vm.NewGoError(err))
	}
	_ = instance_object.SetSymbol(goja.SymToStringTag, vm.ToValue("WebAssembly.Instance"))
	host.wasm_runtimes = append(host.wasm_runtimes, wasm_runtime)
	keep_runtime = true
	if module_input {
		return instance_object
	}
	module_object := vm.NewObject()
	_ = module_object.SetSymbol(goja.SymToStringTag, vm.ToValue("WebAssembly.Module"))
	host.modules[module_object] = append([]byte(nil), wasm_bytes...)
	result := vm.NewObject()
	_ = result.Set("module", module_object)
	_ = result.Set("instance", instance_object)
	return result
}

func javascript_bytes(value goja.Value) ([]byte, error) {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return nil, errors.New("expected an ArrayBuffer or typed array")
	}
	switch exported := value.Export().(type) {
	case []byte:
		return exported, nil
	case goja.ArrayBuffer:
		return exported.Bytes(), nil
	}
	object, ok := value.(*goja.Object)
	if !ok {
		return nil, errors.New("expected an ArrayBuffer or typed array")
	}
	buffer_value := object.Get("buffer")
	if buffer_value == nil || goja.IsUndefined(buffer_value) || goja.IsNull(buffer_value) {
		return nil, errors.New("expected an ArrayBuffer or typed array")
	}
	var data []byte
	switch buffer := buffer_value.Export().(type) {
	case goja.ArrayBuffer:
		data = buffer.Bytes()
	case []byte:
		data = buffer
	default:
		return nil, errors.New("typed array has an invalid buffer")
	}
	offset := object.Get("byteOffset").ToInteger()
	length := object.Get("byteLength").ToInteger()
	if offset < 0 || length < 0 || offset+length > int64(len(data)) {
		return nil, errors.New("typed array is outside its buffer")
	}
	return data[offset : offset+length], nil
}

func javascript_to_webassembly(value goja.Value, value_type api.ValueType) (uint64, error) {
	if value == nil {
		value = goja.Undefined()
	}
	switch value_type {
	case api.ValueTypeI32:
		return api.EncodeI32(int32(value.ToInteger())), nil
	case api.ValueTypeI64:
		switch exported := value.Export().(type) {
		case *big.Int:
			return exported.Uint64(), nil
		case big.Int:
			return exported.Uint64(), nil
		case int64:
			return api.EncodeI64(exported), nil
		case uint64:
			return exported, nil
		default:
			integer, ok := new(big.Int).SetString(strings.TrimSuffix(value.String(), "n"), 10)
			if !ok {
				return 0, errors.New("WebAssembly i64 value must be a BigInt")
			}
			return integer.Uint64(), nil
		}
	case api.ValueTypeF32:
		return api.EncodeF32(float32(value.ToFloat())), nil
	case api.ValueTypeF64:
		return api.EncodeF64(value.ToFloat()), nil
	case api.ValueTypeExternref:
		return api.EncodeExternref(uintptr(value.ToInteger())), nil
	default:
		return 0, fmt.Errorf("unsupported WebAssembly value type %s", api.ValueTypeName(value_type))
	}
}

func webassembly_to_javascript(vm *goja.Runtime, value uint64, value_type api.ValueType) goja.Value {
	switch value_type {
	case api.ValueTypeI32:
		return vm.ToValue(api.DecodeI32(value))
	case api.ValueTypeI64:
		integer := new(big.Int).SetUint64(value)
		if value&(uint64(1)<<63) != 0 {
			integer.Sub(integer, new(big.Int).Lsh(big.NewInt(1), 64))
		}
		return vm.ToValue(integer)
	case api.ValueTypeF32:
		return vm.ToValue(api.DecodeF32(value))
	case api.ValueTypeF64:
		return vm.ToValue(api.DecodeF64(value))
	case api.ValueTypeExternref:
		return vm.ToValue(uint64(api.DecodeExternref(value)))
	default:
		return goja.Undefined()
	}
}
