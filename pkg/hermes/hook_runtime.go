package hermes

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/buffer"
	"github.com/dop251/goja_nodejs/eventloop"
	node_url "github.com/dop251/goja_nodejs/url"
	"github.com/rs/zerolog"
)

//go:embed third_party/whatwg-fetch/fetch.umd.js
var fetch_polyfill string

var fetch_program = goja.MustCompile("whatwg-fetch-3.6.20.js", fetch_polyfill, false)

// Upstream Buffer lacks isBuffer and the numeric constructor used by TypedArray.slice.
var buffer_compat_program = goja.MustCompile("hook-buffer.js", `
Buffer.isBuffer = value => value instanceof Buffer;
Object.defineProperty(Buffer, Symbol.species, { value: function (...args) {
  return typeof args[0] === "number" ? Buffer.alloc(args[0]) : Buffer.from(...args);
}});
`, false)

type hook_runtime struct {
	vm            *goja.Runtime
	loop          *eventloop.EventLoop
	request_ctx   context.Context
	timeout       time.Duration
	pending_io    int
	stop_if_idle  func(error)
	keepalive     goja.Value
	set_timeout   goja.Callable
	clear_timeout goja.Callable
	timers        []hook_timer
	streams       []*hook_read_stream
	logger        zerolog.Logger
}

type hook_timer struct {
	clear  goja.Callable
	handle goja.Value
}

func new_hook_runtime(logger zerolog.Logger) (*hook_runtime, error) {
	runtime := &hook_runtime{
		loop:   eventloop.NewEventLoop(eventloop.EnableConsole(false)),
		logger: logger,
		// shortcut: hooks share a ten-minute deadline; make it configurable for longer uploads.
		timeout: 10 * time.Minute,
	}
	var err error
	runtime.loop.Run(func(vm *goja.Runtime) {
		runtime.vm = vm
		if err = runtime.install_console(); err != nil {
			return
		}
		runtime.set_timeout, _ = goja.AssertFunction(vm.Get("setTimeout"))
		runtime.clear_timeout, _ = goja.AssertFunction(vm.Get("clearTimeout"))
		for _, names := range [][2]string{{"setTimeout", "clearTimeout"}, {"setInterval", "clearInterval"}, {"setImmediate", "clearImmediate"}} {
			set_timer, _ := goja.AssertFunction(vm.Get(names[0]))
			clear_timer, _ := goja.AssertFunction(vm.Get(names[1]))
			_ = vm.Set(names[0], func(call goja.FunctionCall) goja.Value {
				handle, call_err := set_timer(call.This, call.Arguments...)
				if call_err != nil {
					panic(call_err)
				}
				if handle != nil && !goja.IsUndefined(handle) {
					runtime.timers = append(runtime.timers, hook_timer{clear: clear_timer, handle: handle})
				}
				return handle
			})
		}
		buffer.Enable(vm)
		node_url.Enable(vm)
		if _, err = vm.RunProgram(buffer_compat_program); err != nil {
			return
		}
		runtime.install_file_modules()
		register_builtins(vm)
		_ = vm.Set("XMLHttpRequest", runtime.new_xml_http_request)
		_, err = vm.RunProgram(fetch_program)
		if err == nil {
			err = runtime.install_stream_fetch()
		}
	})
	return runtime, err
}

func (runtime *hook_runtime) run(callback func() (goja.Value, error)) (result goja.Value, err error) {
	return runtime.run_context(context.Background(), callback)
}

func (runtime *hook_runtime) run_context(parent context.Context, callback func() (goja.Value, error)) (result goja.Value, err error) {
	request_ctx, cancel := context.WithTimeout(parent, runtime.timeout)
	if err := request_ctx.Err(); err != nil {
		cancel()
		return nil, err
	}
	runtime.request_ctx = request_ctx
	runtime.pending_io = 0
	completed := false
	rejections := make(map[*goja.Promise]struct{})
	runtime.vm.SetPromiseRejectionTracker(func(promise *goja.Promise, operation goja.PromiseRejectionOperation) {
		if operation == goja.PromiseRejectionReject {
			rejections[promise] = struct{}{}
		} else {
			delete(rejections, promise)
		}
	})
	runtime.stop_if_idle = func(failure error) {
		if failure != nil {
			err = failure
		}
		if err != nil {
			runtime.loop.StopNoWait()
		} else if completed && runtime.pending_io == 0 {
			for promise := range rejections {
				err = hook_rejection_error(promise.Result())
				break
			}
			if err != nil {
				runtime.loop.StopNoWait()
			} else if runtime.keepalive != nil {
				_, _ = runtime.clear_timeout(goja.Undefined(), runtime.keepalive)
				runtime.keepalive = nil
			}
		}
	}
	watchdog_done := make(chan struct{})
	watchdog_stop := make(chan struct{})
	go func() {
		defer close(watchdog_done)
		select {
		case <-request_ctx.Done():
			runtime.vm.Interrupt(request_ctx.Err())
			runtime.loop.RunOnLoop(func(*goja.Runtime) { runtime.loop.StopNoWait() })
		case <-watchdog_stop:
		}
	}()
	defer func() {
		close(watchdog_stop)
		<-watchdog_done
		cancel()
		runtime.vm.ClearInterrupt()
		for _, stream := range runtime.streams {
			_ = stream.Close()
		}
		runtime.streams = nil
		// Clear timers before Terminate so the reusable loop's job count stays balanced.
		if runtime.keepalive != nil {
			_, _ = runtime.clear_timeout(goja.Undefined(), runtime.keepalive)
		}
		for _, timer := range runtime.timers {
			_, _ = timer.clear(goja.Undefined(), timer.handle)
		}
		runtime.timers = nil
		runtime.loop.Terminate()
		runtime.keepalive = nil
	}()
	runtime.loop.Run(func(vm *goja.Runtime) {
		runtime.keep_alive()
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
					completed = true
					runtime.stop_if_idle(nil)
				}), vm.ToValue(func(reason goja.Value) {
					err = hook_rejection_error(reason)
					runtime.loop.StopNoWait()
				}))
				if then_err == nil {
					return
				}
				err = then_err
			}
		}
		completed = true
		runtime.stop_if_idle(nil)
	})
	if request_ctx.Err() != nil {
		if request_ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("script timed out: %w", request_ctx.Err())
		}
		return nil, fmt.Errorf("script execution stopped: %w", request_ctx.Err())
	}
	if err == nil {
		for promise := range rejections {
			err = hook_rejection_error(promise.Result())
			break
		}
	}
	return result, err
}

func (runtime *hook_runtime) keep_alive() {
	if runtime.keepalive == nil {
		runtime.keepalive, _ = runtime.set_timeout(goja.Undefined(), runtime.vm.ToValue(func() {}), runtime.vm.ToValue(runtime.timeout.Milliseconds()*2))
	}
}

func (runtime *hook_runtime) begin_io() {
	runtime.keep_alive()
	runtime.pending_io++
}

// The community polyfill owns Fetch/Headers/Request/Response; this adapter only
// supplies the XMLHttpRequest transport it expects, on the Goja event loop.
func (runtime *hook_runtime) new_xml_http_request(call goja.ConstructorCall) *goja.Object {
	vm := runtime.vm
	xhr := call.This
	_ = xhr.Set("responseType", "")
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
		payload, content_type, err := runtime.request_body(body)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		if stream := hook_value_stream(body); stream != nil && !stream.content_type_set {
			headers.Del("Content-Type")
		}
		parent_ctx := runtime.request_ctx
		request_ctx, cancel := context.WithCancel(parent_ctx)
		cancel_request = cancel
		var request_reader io.Reader
		if payload != nil {
			request_reader = payload
		}
		request, err := http.NewRequestWithContext(request_ctx, method, raw_url, request_reader)
		if err != nil {
			cancel()
			if payload != nil {
				_ = payload.Close()
			}
			panic(vm.NewTypeError(err.Error()))
		}
		request.Header = headers.Clone()
		if content_type != "" && request.Header.Get("Content-Type") == "" {
			request.Header.Set("Content-Type", content_type)
		}
		if payload != nil {
			request.ContentLength = payload.size
			request.GetBody = payload.reopen
			if payload.size == 0 {
				_ = payload.Close()
				request.Body = http.NoBody
			}
		}
		runtime.begin_io()
		go func() {
			defer cancel()
			response, request_err := http.DefaultClient.Do(request)
			var data []byte
			if request_err == nil {
				data, request_err = io.ReadAll(response.Body)
				_ = response.Body.Close()
				if payload != nil && payload.reopen == nil && (response.StatusCode == 307 || response.StatusCode == 308) && response.Header.Get("Location") != "" {
					request_err = fmt.Errorf("cannot replay a streaming request body after redirect")
				}
			}
			was_aborted := request_ctx.Err() != nil
			runtime.loop.RunOnLoop(func(*goja.Runtime) {
				if parent_ctx.Err() != nil {
					return
				}
				defer func() {
					runtime.pending_io--
					runtime.stop_if_idle(nil)
				}()
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
				if response_type := xhr.Get("responseType"); response_type != nil && response_type.String() == "arraybuffer" {
					_ = xhr.Set("response", vm.NewArrayBuffer(data))
				} else if response_type != nil && response_type.String() == "blob" {
					_ = xhr.Set("response", runtime.blob_object(&hook_blob{parts: []hook_blob_part{{data: data, size: int64(len(data))}}, size: int64(len(data)), media_type: response.Header.Get("Content-Type")}))
				}
				emit("onload")
			})
		}()
	})
	return xhr
}
