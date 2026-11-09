package hermes

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
)

//go:embed third_party/whatwg-fetch/fetch.umd.js
var fetch_polyfill string

var fetch_program = goja.MustCompile("whatwg-fetch-3.6.20.js", fetch_polyfill, false)

type hook_runtime struct {
	vm          *goja.Runtime
	loop        *eventloop.EventLoop
	request_ctx context.Context
	timeout     time.Duration
}

func new_hook_runtime() (*hook_runtime, error) {
	runtime := &hook_runtime{
		loop:    eventloop.NewEventLoop(eventloop.EnableConsole(false)),
		timeout: 10 * time.Second,
	}
	var err error
	runtime.loop.Run(func(vm *goja.Runtime) {
		runtime.vm = vm
		// Keep filesystem access limited to the existing task-scoped builtins.
		_ = vm.GlobalObject().Delete("require")
		register_builtins(vm)
		_ = vm.Set("XMLHttpRequest", runtime.new_xml_http_request)
		_, err = vm.RunProgram(fetch_program)
	})
	return runtime, err
}

func (runtime *hook_runtime) run(callback func() (goja.Value, error)) (result goja.Value, err error) {
	request_ctx, cancel := context.WithTimeout(context.Background(), runtime.timeout)
	runtime.request_ctx = request_ctx
	var deadline_timer *eventloop.Timer
	defer func() {
		cancel()
		runtime.loop.ClearTimeout(deadline_timer)
		runtime.loop.Terminate()
	}()
	runtime.loop.Run(func(vm *goja.Runtime) {
		deadline_timer = runtime.loop.SetTimeout(func(*goja.Runtime) {
			err = fmt.Errorf("hook timed out after %s", runtime.timeout)
			runtime.loop.StopNoWait()
		}, runtime.timeout)
		result, err = callback()
		if err == nil && result != nil {
			if _, is_promise := result.Export().(*goja.Promise); is_promise {
				then, ok := goja.AssertFunction(result.ToObject(vm).Get("then"))
				if !ok {
					err = fmt.Errorf("hook Promise.then is not a function")
					runtime.loop.StopNoWait()
					return
				}
				_, then_err := then(result, vm.ToValue(func(value goja.Value) {
					result = value
					runtime.loop.StopNoWait()
				}), vm.ToValue(func(reason goja.Value) {
					err = fmt.Errorf("%s", reason.String())
					runtime.loop.StopNoWait()
				}))
				if then_err == nil {
					return
				}
				err = then_err
			}
		}
		runtime.loop.StopNoWait()
	})
	return result, err
}

// The community polyfill owns Fetch/Headers/Request/Response; this adapter only
// supplies the XMLHttpRequest transport it expects, on the Goja event loop.
func (runtime *hook_runtime) new_xml_http_request(call goja.ConstructorCall) *goja.Object {
	vm := runtime.vm
	xhr := call.This
	var method, raw_url, response_headers string
	headers := make(http.Header)
	var cancel_request context.CancelFunc
	emit := func(name string) {
		if callback, ok := goja.AssertFunction(xhr.Get(name)); ok {
			_, _ = callback(xhr)
		}
	}
	_ = xhr.Set("open", func(request_method, request_url string) {
		method, raw_url = request_method, request_url
	})
	_ = xhr.Set("setRequestHeader", func(name, value string) { headers.Add(name, value) })
	_ = xhr.Set("getAllResponseHeaders", func() string { return response_headers })
	_ = xhr.Set("abort", func() {
		if cancel_request != nil {
			cancel_request()
		}
	})
	_ = xhr.Set("send", func(body goja.Value) {
		var payload io.Reader
		if body != nil && !goja.IsNull(body) && !goja.IsUndefined(body) {
			switch value := body.Export().(type) {
			case string:
				payload = strings.NewReader(value)
			case goja.ArrayBuffer:
				payload = bytes.NewReader(value.Bytes())
			default:
				panic(vm.NewTypeError("fetch body must be a string or ArrayBuffer"))
			}
		}
		parent_ctx := runtime.request_ctx
		request_ctx, cancel := context.WithCancel(parent_ctx)
		cancel_request = cancel
		request, err := http.NewRequestWithContext(request_ctx, method, raw_url, payload)
		if err != nil {
			cancel()
			panic(vm.NewTypeError(err.Error()))
		}
		request.Header = headers.Clone()
		go func() {
			defer cancel()
			response, request_err := http.DefaultClient.Do(request)
			var data []byte
			if request_err == nil {
				data, request_err = io.ReadAll(response.Body)
				_ = response.Body.Close()
			}
			was_aborted := request_ctx.Err() != nil
			runtime.loop.RunOnLoop(func(*goja.Runtime) {
				if parent_ctx.Err() != nil {
					return
				}
				_ = xhr.Set("readyState", 4)
				emit("onreadystatechange")
				if request_err != nil {
					if was_aborted {
						emit("onabort")
					} else {
						emit("onerror")
					}
					return
				}
				var buffer bytes.Buffer
				_ = response.Header.Write(&buffer)
				response_headers = buffer.String()
				_ = xhr.Set("status", response.StatusCode)
				_ = xhr.Set("statusText", http.StatusText(response.StatusCode))
				_ = xhr.Set("responseURL", response.Request.URL.String())
				_ = xhr.Set("responseText", string(data))
				emit("onload")
			})
		}()
	})
	return xhr
}
