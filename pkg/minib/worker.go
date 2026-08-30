package minib

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/dop251/goja"
)

type worker_cloned_value struct {
	kind              string
	primitive         any
	bytes             []byte
	typed_constructor string
	items             []*worker_cloned_value
	properties        map[string]*worker_cloned_value
}

type dedicated_worker struct {
	runtime    *page_runtime
	object     *goja.Object
	source     string
	terminated bool
	listeners  map[string][]goja.Callable
}

func (runtime *page_runtime) create_object_url(call goja.FunctionCall) goja.Value {
	blob, ok := call.Argument(0).(*goja.Object)
	if !ok || blob.ClassName() != "Object" && blob.ClassName() != "Blob" {
		panic(runtime.vm.NewTypeError("URL.createObjectURL requires a Blob"))
	}
	text_value := blob.Get("_text")
	if text_value == nil || goja.IsUndefined(text_value) {
		panic(runtime.vm.NewTypeError("URL.createObjectURL requires a Blob"))
	}
	runtime.next_blob_id++
	raw_url := fmt.Sprintf("blob:%s://%s/minib/%d", runtime.page_url.Scheme, runtime.page_url.Host, runtime.next_blob_id)
	runtime.blob_urls[raw_url] = text_value.String()
	return runtime.vm.ToValue(raw_url)
}

func (runtime *page_runtime) revoke_object_url(raw_url string) {
	delete(runtime.blob_urls, raw_url)
}

func (runtime *page_runtime) worker_constructor(call goja.ConstructorCall) *goja.Object {
	raw_url := call.Argument(0).String()
	source, ok := runtime.blob_urls[raw_url]
	if !ok {
		var err error
		source, err = data_worker_source(raw_url)
		if err != nil {
			panic(runtime.vm.NewTypeError("Failed to construct 'Worker': %s", err.Error()))
		}
	}
	worker := &dedicated_worker{
		runtime:   runtime,
		object:    call.This,
		source:    source,
		listeners: make(map[string][]goja.Callable),
	}
	_ = worker.object.Set("onmessage", nil)
	_ = worker.object.Set("onerror", nil)
	_ = worker.object.Set("__minib_postMessage", func(message_call goja.FunctionCall) goja.Value {
		if worker.terminated {
			return goja.Undefined()
		}
		message := clone_worker_value(message_call.Argument(0), make(map[*goja.Object]*worker_cloned_value))
		outputs, worker_err := worker.run(message)
		worker.runtime.queue_host_job(func() {
			if worker.terminated {
				return
			}
			if worker_err != nil {
				worker.fire_error(worker_err)
				return
			}
			for _, output := range outputs {
				worker.fire_message(output)
			}
		})
		return goja.Undefined()
	})
	_ = worker.object.Set("__minib_terminate", func() { worker.terminated = true })
	_ = worker.object.Set("__minib_addEventListener", func(event_call goja.FunctionCall) goja.Value {
		if callback, callback_ok := goja.AssertFunction(event_call.Argument(1)); callback_ok {
			event_name := event_call.Argument(0).String()
			worker.listeners[event_name] = append(worker.listeners[event_name], callback)
		}
		return goja.Undefined()
	})
	_ = worker.object.Set("__minib_removeEventListener", func(event_call goja.FunctionCall) goja.Value {
		event_name := event_call.Argument(0).String()
		callback_value := event_call.Argument(1)
		callbacks := worker.listeners[event_name]
		kept := callbacks[:0]
		for _, callback := range callbacks {
			if callback_value != runtime.vm.ToValue(callback) {
				kept = append(kept, callback)
			}
		}
		worker.listeners[event_name] = kept
		return goja.Undefined()
	})
	return worker.object
}

func (runtime *page_runtime) install_worker_prototype(constructor *goja.Object) {
	prototype := constructor.Get("prototype").ToObject(runtime.vm)
	for _, name := range []string{"postMessage", "terminate", "addEventListener", "removeEventListener"} {
		method_name := name
		_ = prototype.Set(method_name, func(call goja.FunctionCall) goja.Value {
			method, ok := goja.AssertFunction(call.This.ToObject(runtime.vm).Get("__minib_" + method_name))
			if !ok {
				panic(runtime.vm.NewTypeError("Worker.%s called on incompatible receiver", method_name))
			}
			value, err := method(call.This, call.Arguments...)
			if err != nil {
				panic(err)
			}
			return value
		})
	}
}

func data_worker_source(raw_url string) (string, error) {
	if !strings.HasPrefix(raw_url, "data:") {
		return "", fmt.Errorf("only Blob and data URLs are currently supported")
	}
	separator := strings.IndexByte(raw_url, ',')
	if separator < 0 {
		return "", fmt.Errorf("invalid data URL")
	}
	metadata, payload := raw_url[:separator], raw_url[separator+1:]
	if strings.HasSuffix(strings.ToLower(metadata), ";base64") {
		decoded, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return "", fmt.Errorf("invalid base64 data URL")
		}
		return string(decoded), nil
	}
	decoded, err := url.PathUnescape(payload)
	if err != nil {
		return "", fmt.Errorf("invalid escaped data URL")
	}
	return decoded, nil
}

func (worker *dedicated_worker) run(message *worker_cloned_value) ([]*worker_cloned_value, error) {
	vm := goja.New()
	vm.SetMaxCallStackSize(max_call_stack_depth)
	outputs := make([]*worker_cloned_value, 0, 1)
	_ = vm.Set("__minib_worker_post_message", func(call goja.FunctionCall) goja.Value {
		outputs = append(outputs, clone_worker_value(call.Argument(0), make(map[*goja.Object]*worker_cloned_value)))
		return goja.Undefined()
	})
	if _, err := vm.RunString(worker_runtime_prelude); err != nil {
		return nil, fmt.Errorf("initialize Worker: %w", err)
	}
	worker_runtime := &page_runtime{vm: vm, ctx: worker.runtime.ctx}
	if err := worker_runtime.install_web_crypto(vm.GlobalObject()); err != nil {
		return nil, fmt.Errorf("initialize Worker crypto: %w", err)
	}
	if err := worker_runtime.install_webassembly(vm.GlobalObject()); err != nil {
		return nil, fmt.Errorf("initialize Worker WebAssembly: %w", err)
	}
	defer worker_runtime.close_webassembly()
	if _, err := vm.RunString(worker.source); err != nil {
		return nil, fmt.Errorf("evaluate Worker: %w", err)
	}
	dispatch, ok := goja.AssertFunction(vm.Get("__minib_dispatch_worker_message"))
	if !ok {
		return nil, fmt.Errorf("Worker message dispatcher is unavailable")
	}
	message_value := restore_worker_value(vm, message)
	if _, err := dispatch(goja.Undefined(), message_value); err != nil {
		return nil, fmt.Errorf("dispatch Worker message: %w", err)
	}
	return outputs, nil
}

func (worker *dedicated_worker) fire_message(message *worker_cloned_value) {
	event := worker.runtime.vm.NewObject()
	_ = event.Set("type", "message")
	_ = event.Set("data", restore_worker_value(worker.runtime.vm, message))
	_ = event.Set("target", worker.object)
	_ = event.Set("currentTarget", worker.object)
	worker.fire("message", event)
}

func (worker *dedicated_worker) fire_error(worker_err error) {
	event := worker.runtime.vm.NewObject()
	_ = event.Set("type", "error")
	_ = event.Set("message", worker_err.Error())
	_ = event.Set("error", worker.runtime.vm.NewGoError(worker_err))
	_ = event.Set("target", worker.object)
	_ = event.Set("currentTarget", worker.object)
	worker.fire("error", event)
}

func (worker *dedicated_worker) fire(event_name string, event *goja.Object) {
	for _, callback := range worker.listeners[event_name] {
		if _, err := worker.runtime.call_javascript(worker.runtime.ctx, callback, worker.object, event); err != nil {
			worker.runtime.fail_script(worker.runtime.page.URL+"#worker-"+event_name, err)
		}
	}
	if handler, ok := goja.AssertFunction(worker.object.Get("on" + event_name)); ok {
		if _, err := worker.runtime.call_javascript(worker.runtime.ctx, handler, worker.object, event); err != nil {
			worker.runtime.fail_script(worker.runtime.page.URL+"#worker-"+event_name, err)
		}
	}
}

func clone_worker_value(value goja.Value, seen map[*goja.Object]*worker_cloned_value) *worker_cloned_value {
	if value == nil || goja.IsUndefined(value) {
		return &worker_cloned_value{kind: "undefined"}
	}
	if goja.IsNull(value) {
		return &worker_cloned_value{kind: "null"}
	}
	object, is_object := value.(*goja.Object)
	if !is_object {
		return &worker_cloned_value{kind: "primitive", primitive: value.Export()}
	}
	if cloned, ok := seen[object]; ok {
		return cloned
	}
	if buffer, ok := value.Export().(goja.ArrayBuffer); ok {
		return &worker_cloned_value{kind: "array_buffer", bytes: append([]byte(nil), buffer.Bytes()...)}
	}
	class_name := object.ClassName()
	constructor_name := class_name
	if constructor, ok := object.Get("constructor").(*goja.Object); ok {
		if name := constructor.Get("name"); name != nil && !goja.IsUndefined(name) && name.String() != "" {
			constructor_name = name.String()
		}
	}
	if strings.HasSuffix(constructor_name, "Array") && constructor_name != "Array" || constructor_name == "DataView" {
		if data, err := javascript_bytes(value); err == nil {
			return &worker_cloned_value{kind: "typed_array", bytes: append([]byte(nil), data...), typed_constructor: constructor_name}
		}
	}
	if class_name == "Array" {
		cloned := &worker_cloned_value{kind: "array"}
		seen[object] = cloned
		length := int(object.Get("length").ToInteger())
		cloned.items = make([]*worker_cloned_value, length)
		for index := 0; index < length; index++ {
			cloned.items[index] = clone_worker_value(object.Get(fmt.Sprintf("%d", index)), seen)
		}
		return cloned
	}
	cloned := &worker_cloned_value{kind: "object", properties: make(map[string]*worker_cloned_value)}
	seen[object] = cloned
	for _, key := range object.Keys() {
		cloned.properties[key] = clone_worker_value(object.Get(key), seen)
	}
	return cloned
}

func restore_worker_value(vm *goja.Runtime, cloned *worker_cloned_value) goja.Value {
	if cloned == nil {
		return goja.Undefined()
	}
	switch cloned.kind {
	case "undefined":
		return goja.Undefined()
	case "null":
		return goja.Null()
	case "primitive":
		return vm.ToValue(cloned.primitive)
	case "array_buffer":
		return vm.ToValue(vm.NewArrayBuffer(append([]byte(nil), cloned.bytes...)))
	case "typed_array":
		constructor, ok := goja.AssertConstructor(vm.Get(cloned.typed_constructor))
		if !ok {
			return vm.ToValue(vm.NewArrayBuffer(append([]byte(nil), cloned.bytes...)))
		}
		value, err := constructor(nil, vm.ToValue(vm.NewArrayBuffer(append([]byte(nil), cloned.bytes...))))
		if err != nil {
			return vm.ToValue(vm.NewArrayBuffer(append([]byte(nil), cloned.bytes...)))
		}
		return value
	case "array":
		array := vm.NewArray()
		for index, item := range cloned.items {
			_ = array.Set(fmt.Sprintf("%d", index), restore_worker_value(vm, item))
		}
		return array
	case "object":
		object := vm.NewObject()
		for key, property := range cloned.properties {
			_ = object.Set(key, restore_worker_value(vm, property))
		}
		return object
	default:
		return goja.Undefined()
	}
}

const worker_runtime_prelude = `
var self = globalThis;
var onmessage = null;
var onerror = null;
var __minib_worker_listeners = Object.create(null);
function addEventListener(type, callback) {
  type = String(type);
  (__minib_worker_listeners[type] || (__minib_worker_listeners[type] = [])).push(callback);
}
function removeEventListener(type, callback) {
  var listeners = __minib_worker_listeners[String(type)] || [];
  __minib_worker_listeners[String(type)] = listeners.filter(function(listener) { return listener !== callback; });
}
function postMessage(message) { return __minib_worker_post_message(message); }
function close() {}
function queueMicrotask(callback) { Promise.resolve().then(callback); }
function setTimeout(callback) { callback(); return 1; }
function clearTimeout() {}
function setInterval() { return 1; }
function clearInterval() {}
function TextEncoder() { this.encoding = 'utf-8'; }
TextEncoder.prototype.encode = function(input) {
  var text = unescape(encodeURIComponent(String(input === undefined ? '' : input))), bytes = new Uint8Array(text.length);
  for (var index = 0; index < text.length; index++) bytes[index] = text.charCodeAt(index);
  return bytes;
};
function TextDecoder() { this.encoding = 'utf-8'; }
TextDecoder.prototype.decode = function(input) {
  if (input == null) return '';
  var bytes = input instanceof ArrayBuffer ? new Uint8Array(input) : new Uint8Array(input.buffer, input.byteOffset || 0, input.byteLength), text = '';
  for (var index = 0; index < bytes.length; index++) text += String.fromCharCode(bytes[index]);
  try { return decodeURIComponent(escape(text)); } catch (_) { return text; }
};
var performance = { now: function() { return Date.now(); }, timeOrigin: Date.now() };
var location = { href: '', origin: '', protocol: '', host: '', hostname: '', pathname: '/', search: '', hash: '' };
var console = { log: function(){}, info: function(){}, debug: function(){}, warn: function(){}, error: function(){} };
function __minib_dispatch_worker_message(data) {
  var event = { type: 'message', data: data, target: self, currentTarget: self };
  if (typeof onmessage === 'function') onmessage.call(self, event);
  (__minib_worker_listeners.message || []).slice().forEach(function(callback) { callback.call(self, event); });
}
`
