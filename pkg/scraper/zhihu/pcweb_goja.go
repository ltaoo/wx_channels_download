package zhihu

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"math/big"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

const pcweb_goja_polyfills = `
(function installPcwebHostPrimitives(global) {
  "use strict";

  class URLSearchParams {
    constructor(value = "") {
      this._pairs = [];
      const source = String(value).replace(/^\?/, "");
      if (!source) return;
      for (const part of source.split("&")) {
        const index = part.indexOf("=");
        const key = index < 0 ? part : part.slice(0, index);
        const item = index < 0 ? "" : part.slice(index + 1);
        this._pairs.push([
          decodeURIComponent(key.replace(/\+/g, " ")),
          decodeURIComponent(item.replace(/\+/g, " ")),
        ]);
      }
    }
    append(key, value) { this._pairs.push([String(key), String(value)]); }
    delete(key) { key = String(key); this._pairs = this._pairs.filter((pair) => pair[0] !== key); }
    get(key) { key = String(key); const pair = this._pairs.find((item) => item[0] === key); return pair ? pair[1] : null; }
    getAll(key) { key = String(key); return this._pairs.filter((item) => item[0] === key).map((item) => item[1]); }
    has(key) { return this.get(String(key)) !== null; }
    set(key, value) {
      key = String(key); value = String(value);
      let found = false;
      this._pairs = this._pairs.filter((pair) => {
        if (pair[0] !== key) return true;
        if (found) return false;
        pair[1] = value; found = true; return true;
      });
      if (!found) this._pairs.push([key, value]);
    }
    sort() { this._pairs.sort((left, right) => left[0].localeCompare(right[0])); }
    entries() { return this._pairs[Symbol.iterator](); }
    keys() { return this._pairs.map((pair) => pair[0])[Symbol.iterator](); }
    values() { return this._pairs.map((pair) => pair[1])[Symbol.iterator](); }
    forEach(callback, thisArg) { for (const pair of this._pairs) callback.call(thisArg, pair[1], pair[0], this); }
    toString() { return this._pairs.map((pair) => encodeURIComponent(pair[0]) + "=" + encodeURIComponent(pair[1])).join("&"); }
    [Symbol.iterator]() { return this.entries(); }
  }

  class URL {
    constructor(input, base) { this._apply(__goja_parse_url(String(input), base === undefined ? "" : String(base))); }
    _apply(parts) {
      for (const key of ["href", "origin", "protocol", "username", "password", "host", "hostname", "port", "pathname", "search", "hash"]) {
        this[key] = parts[key] || "";
      }
      this.searchParams = new URLSearchParams(this.search);
    }
    toString() { return this.href; }
    toJSON() { return this.href; }
  }

  class TextEncoder {
    get encoding() { return "utf-8"; }
    encode(value = "") { return new Uint8Array(__goja_utf8_encode(String(value))); }
    encodeInto(value, destination) {
      const encoded = this.encode(value);
      const written = Math.min(encoded.length, destination.length);
      destination.set(encoded.subarray(0, written));
      return { read: String(value).length, written };
    }
  }

  class TextDecoder {
    constructor(label = "utf-8") {
      if (!/^utf-?8$/i.test(String(label))) throw new RangeError("only utf-8 is supported");
      this.encoding = "utf-8"; this.fatal = false; this.ignoreBOM = false;
    }
    decode(value) { return value == null ? "" : __goja_utf8_decode(value); }
  }

  const subtle = {
    digest(algorithm, data) {
      const name = typeof algorithm === "string" ? algorithm : algorithm && algorithm.name;
      return Promise.resolve(__goja_digest(String(name || ""), data));
    },
  };
  const crypto = {
    subtle,
    getRandomValues(view) {
      if (!ArrayBuffer.isView(view) || view instanceof DataView) throw new TypeError("expected an integer TypedArray");
      if (view.byteLength > 65536) throw new RangeError("requested too many random bytes");
      new Uint8Array(view.buffer, view.byteOffset, view.byteLength).set(new Uint8Array(__goja_random_bytes(view.byteLength)));
      return view;
    },
    randomUUID() {
      const bytes = crypto.getRandomValues(new Uint8Array(16));
      bytes[6] = (bytes[6] & 15) | 64; bytes[8] = (bytes[8] & 63) | 128;
      const hex = Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("");
      return hex.slice(0, 8) + "-" + hex.slice(8, 12) + "-" + hex.slice(12, 16) + "-" + hex.slice(16, 20) + "-" + hex.slice(20);
    },
  };

  let timerSequence = 0;
  const timers = new Map();
  function scheduleTimer(callback, delay, repeat, args) {
    const id = ++timerSequence;
    const interval = Math.max(0, Number(delay) || 0);
    timers.set(id, { callback, args, interval, repeat, due: Date.now() + interval });
    return id;
  }
  function clearTimer(id) { timers.delete(Number(id)); }
  function runTimers(now) {
    let ran = 0;
    for (const [id, timer] of Array.from(timers)) {
      if (timer.due > now) continue;
      if (timer.repeat) timer.due = now + Math.max(1, timer.interval);
      else timers.delete(id);
      timer.callback(...timer.args);
      ran += 1;
    }
    let next = 0;
    for (const timer of timers.values()) if (!next || timer.due < next) next = timer.due;
    return { ran, pending: timers.size, next };
  }

  const WebAssembly = {
    instantiate(bytes, imports) { return Promise.resolve(__goja_wasm_instantiate(bytes, imports || {})); },
    compile(bytes) { return Promise.resolve(bytes); },
    validate(bytes) { try { __goja_wasm_validate(bytes); return true; } catch (_) { return false; } },
  };

  Object.assign(global, {
    URL, URLSearchParams, TextEncoder, TextDecoder, crypto,
    performance: { timeOrigin: __goja_time_origin, now: () => Date.now() - __goja_time_origin },
    WebAssembly,
    atob: (value) => __goja_atob(String(value)),
    btoa: (value) => __goja_btoa(String(value)),
    setTimeout: (callback, delay, ...args) => scheduleTimer(callback, delay, false, args),
    clearTimeout: clearTimer,
    setInterval: (callback, delay, ...args) => scheduleTimer(callback, delay, true, args),
    clearInterval: clearTimer,
    queueMicrotask: (callback) => Promise.resolve().then(callback),
    console: { log() {}, info() {}, warn() {}, error() {}, debug() {} },
    __goja_webcrypto: crypto,
    __goja_performance: { timeOrigin: __goja_time_origin, now: () => Date.now() - __goja_time_origin },
    __goja_run_timers: runTimers,
  });
})(globalThis);
`

type pcweb_goja_host struct {
	ctx  context.Context
	vm   *goja.Runtime
	wasm wazero.Runtime
}

func run_pcweb_goja(script []byte, script_url, meta, target_url string) (string, error) {
	parsed_script_url, err := url.Parse(script_url)
	if err != nil {
		return "", fmt.Errorf("parse zhihu zse-ck script URL: %w", err)
	}
	script_name := filepath.Base(parsed_script_url.Path)
	if !pcweb_script_name_re.MatchString(script_name) {
		return "", fmt.Errorf("unexpected zhihu zse-ck script name %q", script_name)
	}
	if _, err := url.ParseRequestURI(target_url); err != nil {
		return "", fmt.Errorf("parse zhihu pcweb target URL: %w", err)
	}

	var profile any
	if err := json.Unmarshal(pcweb_vm_profile, &profile); err != nil {
		return "", fmt.Errorf("parse embedded zhihu pcweb profile: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), pcweb_vm_timeout)
	defer cancel()
	vm := goja.New()
	interrupt_timer := time.AfterFunc(pcweb_vm_timeout, func() {
		vm.Interrupt(context.DeadlineExceeded)
	})
	defer interrupt_timer.Stop()

	runtime_config := wazero.NewRuntimeConfig().WithCloseOnContextDone(true)
	host := &pcweb_goja_host{
		ctx:  ctx,
		vm:   vm,
		wasm: wazero.NewRuntimeWithConfig(ctx, runtime_config),
	}
	defer host.wasm.Close(context.Background())

	unhandled_rejections := make(map[*goja.Promise]goja.Value)
	vm.SetPromiseRejectionTracker(func(promise *goja.Promise, operation goja.PromiseRejectionOperation) {
		if operation == goja.PromiseRejectionReject {
			unhandled_rejections[promise] = promise.Result()
		} else {
			delete(unhandled_rejections, promise)
		}
	})

	for name, value := range map[string]any{
		"__goja_profile":               profile,
		"__goja_meta":                  meta,
		"__goja_target_url":            target_url,
		"__goja_script_url":            script_url,
		"__goja_blank_canvas_data_url": pcweb_blank_canvas_data_url(),
		"__goja_time_origin":           time.Now().UnixMilli(),
	} {
		if err := vm.Set(name, value); err != nil {
			return "", fmt.Errorf("initialize zhihu goja VM global %s: %w", name, err)
		}
	}
	if err := host.install_primitives(); err != nil {
		return "", err
	}
	if _, err := vm.RunString(pcweb_goja_polyfills); err != nil {
		return "", format_pcweb_goja_error("initialize zhihu goja host primitives", err)
	}
	if _, err := vm.RunScript("pcweb_runtime.js", string(pcweb_vm_runtime)); err != nil {
		return "", format_pcweb_goja_error("initialize zhihu pcweb browser runtime", err)
	}
	program, err := goja.Compile(script_name, string(script), false)
	if err != nil {
		return "", fmt.Errorf("compile zhihu zse-ck script with goja: %w", err)
	}
	if _, err := vm.RunProgram(program); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", errors.New("zhihu zse-ck goja VM timed out")
		}
		return "", format_pcweb_goja_error("run zhihu zse-ck script with goja", err)
	}

	read_result, ok := goja.AssertFunction(vm.Get("__pcwebReadResult"))
	if !ok {
		return "", errors.New("zhihu goja browser runtime did not install its result reader")
	}
	run_timers, ok := goja.AssertFunction(vm.Get("__goja_run_timers"))
	if !ok {
		return "", errors.New("zhihu goja host did not install its timer queue")
	}

	for {
		result_value, call_err := read_result(goja.Undefined())
		if call_err != nil {
			return "", format_pcweb_goja_error("read zhihu zse-ck result", call_err)
		}
		if result_value != nil && !goja.IsNull(result_value) && !goja.IsUndefined(result_value) {
			result_object := result_value.ToObject(vm)
			result := pcweb_vm_result{
				CK:     result_object.Get("ck").String(),
				Cookie: result_object.Get("cookie").String(),
			}
			if result.CK == "" || result.Cookie != result.CK+"-"+meta {
				return "", errors.New("zhihu zse-ck goja VM returned an incomplete cookie")
			}
			return result.Cookie, nil
		}

		if err := ctx.Err(); err != nil {
			return "", errors.New("zhihu zse-ck goja VM timed out")
		}
		timer_value, timer_err := run_timers(goja.Undefined(), vm.ToValue(time.Now().UnixMilli()))
		if timer_err != nil {
			return "", format_pcweb_goja_error("run zhihu goja timer", timer_err)
		}
		timer_state := timer_value.ToObject(vm)
		ran := timer_state.Get("ran").ToInteger()
		pending := timer_state.Get("pending").ToInteger()
		if ran > 0 {
			continue
		}
		if pending == 0 {
			for _, rejection := range unhandled_rejections {
				return "", fmt.Errorf("zhihu zse-ck promise rejected: %s", rejection.String())
			}
			return "", errors.New("zhihu zse-ck challenge did not set __zse_ck")
		}

		next := timer_state.Get("next").ToInteger()
		wait := time.Duration(next-time.Now().UnixMilli()) * time.Millisecond
		if wait < time.Millisecond {
			wait = time.Millisecond
		} else if wait > 20*time.Millisecond {
			wait = 20 * time.Millisecond
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", errors.New("zhihu zse-ck goja VM timed out")
		case <-timer.C:
		}
	}
}

func (host *pcweb_goja_host) install_primitives() error {
	vm := host.vm
	primitives := map[string]any{
		"__goja_parse_url": func(call goja.FunctionCall) goja.Value {
			base := ""
			if value := call.Argument(1); value != nil && !goja.IsUndefined(value) && !goja.IsNull(value) {
				base = value.String()
			}
			parts, err := parse_pcweb_goja_url(call.Argument(0).String(), base)
			if err != nil {
				panic(vm.NewTypeError(err.Error()))
			}
			return vm.ToValue(parts)
		},
		"__goja_utf8_encode": func(call goja.FunctionCall) goja.Value {
			return vm.ToValue(vm.NewArrayBuffer([]byte(call.Argument(0).String())))
		},
		"__goja_utf8_decode": func(call goja.FunctionCall) goja.Value {
			data, err := pcweb_goja_bytes(call.Argument(0))
			if err != nil {
				panic(vm.NewTypeError(err.Error()))
			}
			return vm.ToValue(string(data))
		},
		"__goja_random_bytes": func(call goja.FunctionCall) goja.Value {
			length := call.Argument(0).ToInteger()
			if length < 0 || length > 65536 {
				panic(vm.NewTypeError("invalid random byte length"))
			}
			data := make([]byte, int(length))
			if _, err := rand.Read(data); err != nil {
				panic(vm.NewGoError(err))
			}
			return vm.ToValue(vm.NewArrayBuffer(data))
		},
		"__goja_digest": func(call goja.FunctionCall) goja.Value {
			data, err := pcweb_goja_bytes(call.Argument(1))
			if err != nil {
				panic(vm.NewTypeError(err.Error()))
			}
			var digest []byte
			switch strings.ToUpper(strings.ReplaceAll(call.Argument(0).String(), "_", "-")) {
			case "SHA-1", "SHA1":
				sum := sha1.Sum(data)
				digest = sum[:]
			case "SHA-256", "SHA256":
				sum := sha256.Sum256(data)
				digest = sum[:]
			case "SHA-384", "SHA384":
				sum := sha512.Sum384(data)
				digest = sum[:]
			case "SHA-512", "SHA512":
				sum := sha512.Sum512(data)
				digest = sum[:]
			default:
				panic(vm.NewTypeError("unsupported digest algorithm"))
			}
			return vm.ToValue(vm.NewArrayBuffer(digest))
		},
		"__goja_atob": func(call goja.FunctionCall) goja.Value {
			decoded, err := base64.StdEncoding.DecodeString(call.Argument(0).String())
			if err != nil {
				panic(vm.NewTypeError("invalid base64 input"))
			}
			characters := make([]uint16, len(decoded))
			for index, value := range decoded {
				characters[index] = uint16(value)
			}
			return goja.StringFromUTF16(characters)
		},
		"__goja_btoa": func(call goja.FunctionCall) goja.Value {
			value, ok := call.Argument(0).ToString().(goja.String)
			if !ok {
				panic(vm.NewTypeError("invalid binary string"))
			}
			data := make([]byte, value.Length())
			for index := range data {
				character := value.CharAt(index)
				if character > 255 {
					panic(vm.NewTypeError("btoa input contains a character outside Latin-1"))
				}
				data[index] = byte(character)
			}
			return vm.ToValue(base64.StdEncoding.EncodeToString(data))
		},
		"__goja_wasm_validate": func(call goja.FunctionCall) goja.Value {
			data, err := pcweb_goja_bytes(call.Argument(0))
			if err != nil {
				panic(vm.NewTypeError(err.Error()))
			}
			compiled, err := host.wasm.CompileModule(host.ctx, data)
			if err != nil {
				panic(vm.NewTypeError(err.Error()))
			}
			_ = compiled.Close(host.ctx)
			return goja.Undefined()
		},
		"__goja_wasm_instantiate": host.instantiate_wasm,
	}
	for name, primitive := range primitives {
		if err := vm.Set(name, primitive); err != nil {
			return fmt.Errorf("install zhihu goja primitive %s: %w", name, err)
		}
	}
	return nil
}

func (host *pcweb_goja_host) instantiate_wasm(call goja.FunctionCall) goja.Value {
	vm := host.vm
	wasm_bytes, err := pcweb_goja_bytes(call.Argument(0))
	if err != nil {
		panic(vm.NewTypeError(err.Error()))
	}
	compiled, err := host.wasm.CompileModule(host.ctx, wasm_bytes)
	if err != nil {
		panic(vm.NewTypeError(fmt.Sprintf("compile WebAssembly: %v", err)))
	}
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
		builder := host.wasm.NewHostModuleBuilder(module_name)
		for _, imported := range functions {
			imported := imported
			builder.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(
				func(_ context.Context, _ api.Module, stack []uint64) {
					arguments := make([]goja.Value, len(imported.definition.ParamTypes()))
					for index, value_type := range imported.definition.ParamTypes() {
						arguments[index] = pcweb_wasm_to_goja(vm, stack[index], value_type)
					}
					result, call_err := imported.callback(goja.Undefined(), arguments...)
					if call_err != nil {
						panic(call_err)
					}
					result_types := imported.definition.ResultTypes()
					if len(result_types) == 1 {
						encoded, encode_err := pcweb_goja_to_wasm(result, result_types[0])
						if encode_err != nil {
							panic(encode_err)
						}
						stack[0] = encoded
					} else if len(result_types) > 1 {
						result_object := result.ToObject(vm)
						for index, value_type := range result_types {
							encoded, encode_err := pcweb_goja_to_wasm(result_object.Get(fmt.Sprintf("%d", index)), value_type)
							if encode_err != nil {
								panic(encode_err)
							}
							stack[index] = encoded
						}
					}
				}), imported.definition.ParamTypes(), imported.definition.ResultTypes()).Export(imported.name)
		}
		if _, err := builder.Instantiate(host.ctx); err != nil {
			panic(vm.NewGoError(fmt.Errorf("instantiate WebAssembly host module %s: %w", module_name, err)))
		}
	}

	module_config := wazero.NewModuleConfig().WithStartFunctions()
	instance, err := host.wasm.InstantiateModule(host.ctx, compiled, module_config)
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
				encoded, encode_err := pcweb_goja_to_wasm(call.Argument(index), value_type)
				if encode_err != nil {
					panic(vm.NewTypeError(encode_err.Error()))
				}
				params[index] = encoded
			}
			results, call_err := function.Call(host.ctx, params...)
			if call_err != nil {
				panic(vm.NewGoError(call_err))
			}
			if len(results) == 0 {
				return goja.Undefined()
			}
			if len(results) == 1 {
				return pcweb_wasm_to_goja(vm, results[0], definition.ResultTypes()[0])
			}
			values := make([]any, len(results))
			for index, result := range results {
				values[index] = pcweb_wasm_to_goja(vm, result, definition.ResultTypes()[index])
			}
			return vm.ToValue(values)
		}); err != nil {
			panic(vm.NewGoError(err))
		}
	}
	for export_name := range instance.ExportedMemoryDefinitions() {
		memory := instance.ExportedMemory(export_name)
		memory_object := vm.NewObject()
		getter := vm.ToValue(func(goja.FunctionCall) goja.Value {
			data, ok := memory.Read(0, memory.Size())
			if !ok {
				panic(vm.NewTypeError("WebAssembly memory is unavailable"))
			}
			return vm.ToValue(vm.NewArrayBuffer(data))
		})
		if err := memory_object.DefineAccessorProperty("buffer", getter, nil, goja.FLAG_FALSE, goja.FLAG_TRUE); err != nil {
			panic(vm.NewGoError(err))
		}
		if err := memory_object.Set("grow", func(call goja.FunctionCall) goja.Value {
			previous, ok := memory.Grow(uint32(call.Argument(0).ToInteger()))
			if !ok {
				panic(vm.NewTypeError("WebAssembly memory growth failed"))
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
	result := vm.NewObject()
	_ = result.Set("module", vm.NewObject())
	_ = result.Set("instance", instance_object)
	return result
}

func pcweb_goja_bytes(value goja.Value) ([]byte, error) {
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
	buffer, ok := buffer_value.Export().(goja.ArrayBuffer)
	if !ok {
		return nil, errors.New("typed array has an invalid buffer")
	}
	offset := object.Get("byteOffset").ToInteger()
	length := object.Get("byteLength").ToInteger()
	data := buffer.Bytes()
	if offset < 0 || length < 0 || offset+length > int64(len(data)) {
		return nil, errors.New("typed array is outside its buffer")
	}
	return data[offset : offset+length], nil
}

func pcweb_goja_to_wasm(value goja.Value, value_type api.ValueType) (uint64, error) {
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

func pcweb_wasm_to_goja(vm *goja.Runtime, value uint64, value_type api.ValueType) goja.Value {
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

func parse_pcweb_goja_url(raw_url, base_url string) (map[string]string, error) {
	parsed, err := url.Parse(raw_url)
	if err != nil {
		return nil, err
	}
	if base_url != "" {
		base, err := url.Parse(base_url)
		if err != nil {
			return nil, err
		}
		parsed = base.ResolveReference(parsed)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid absolute URL %q", raw_url)
	}
	username := ""
	password := ""
	if parsed.User != nil {
		username = parsed.User.Username()
		password, _ = parsed.User.Password()
	}
	pathname := parsed.EscapedPath()
	if pathname == "" {
		pathname = "/"
	}
	search := ""
	if parsed.RawQuery != "" {
		search = "?" + parsed.RawQuery
	}
	hash := ""
	if parsed.Fragment != "" {
		hash = "#" + parsed.EscapedFragment()
	}
	return map[string]string{
		"href":     parsed.String(),
		"origin":   parsed.Scheme + "://" + parsed.Host,
		"protocol": parsed.Scheme + ":",
		"username": username,
		"password": password,
		"host":     parsed.Host,
		"hostname": parsed.Hostname(),
		"port":     parsed.Port(),
		"pathname": pathname,
		"search":   search,
		"hash":     hash,
	}, nil
}

var (
	pcweb_blank_canvas_once sync.Once
	pcweb_blank_canvas_url  string
)

func pcweb_blank_canvas_data_url() string {
	pcweb_blank_canvas_once.Do(func() {
		const width = 300
		const height = 150
		header := make([]byte, 13)
		binary.BigEndian.PutUint32(header[0:4], width)
		binary.BigEndian.PutUint32(header[4:8], height)
		header[8] = 8
		header[9] = 6
		raw := make([]byte, height*(1+width*4))
		var compressed bytes.Buffer
		writer := zlib.NewWriter(&compressed)
		_, _ = writer.Write(raw)
		_ = writer.Close()

		png := append([]byte(nil), 137, 80, 78, 71, 13, 10, 26, 10)
		png = append_pcweb_png_chunk(png, "IHDR", header)
		png = append_pcweb_png_chunk(png, "IDAT", compressed.Bytes())
		png = append_pcweb_png_chunk(png, "IEND", nil)
		pcweb_blank_canvas_url = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	})
	return pcweb_blank_canvas_url
}

func append_pcweb_png_chunk(destination []byte, chunk_type string, data []byte) []byte {
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(data)))
	destination = append(destination, length...)
	destination = append(destination, chunk_type...)
	destination = append(destination, data...)
	checksum_data := append([]byte(chunk_type), data...)
	checksum := make([]byte, 4)
	binary.BigEndian.PutUint32(checksum, crc32.ChecksumIEEE(checksum_data))
	return append(destination, checksum...)
}

func format_pcweb_goja_error(prefix string, err error) error {
	if interrupted := new(goja.InterruptedError); errors.As(err, &interrupted) {
		return fmt.Errorf("%s: interrupted", prefix)
	}
	if exception := new(goja.Exception); errors.As(err, &exception) {
		return fmt.Errorf("%s: %s", prefix, exception.String())
	}
	return fmt.Errorf("%s: %w", prefix, err)
}
