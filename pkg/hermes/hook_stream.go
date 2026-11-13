package hermes

import (
	"fmt"
	"io"
	"math"
	"os"
	"sync"

	"github.com/dop251/goja"
)

var hook_stream_symbol = goja.NewSymbol("hook ReadStream")

// shortcut: this ReadStream supports upload bodies; add Readable events and pipe when needed.
type hook_read_stream struct {
	mu               sync.Mutex
	path             string
	flags            int
	start            int64
	end              int64
	high_water_mark  int
	file             *os.File
	reader           io.Reader
	consumed         bool
	destroyed        bool
	ended            bool
	bytes_read       int64
	content_type_set bool
}

func hook_value_stream(value goja.Value) *hook_read_stream {
	if object, ok := value.(*goja.Object); ok {
		if stored := object.GetSymbol(hook_stream_symbol); stored != nil {
			stream, _ := stored.Export().(*hook_read_stream)
			return stream
		}
	}
	return nil
}

func (runtime *hook_runtime) new_read_stream(path_value, options goja.Value, flags int) *goja.Object {
	vm := runtime.vm
	stream := &hook_read_stream{path: hook_file_path(vm, path_value), flags: flags, end: -1, high_water_mark: 64 * 1024}
	if object, ok := options.(*goja.Object); ok {
		for _, name := range []string{"start", "end", "highWaterMark"} {
			value := object.Get(name)
			if value == nil || goja.IsUndefined(value) {
				continue
			}
			if name == "end" && goja.IsInfinity(value) && value.ToFloat() > 0 {
				continue
			}
			if !goja.IsNumber(value) {
				panic(hook_type_error(vm, "ERR_INVALID_ARG_TYPE", name+" must be a number"))
			}
			number := value.ToInteger()
			if float64(number) != value.ToFloat() || number < 0 || number > 9007199254740991 || name == "highWaterMark" && (number == 0 || number > math.MaxInt32) {
				constructor, _ := goja.AssertConstructor(vm.Get("RangeError"))
				error_value, _ := constructor(nil, vm.ToValue("invalid "+name))
				_ = error_value.Set("code", "ERR_OUT_OF_RANGE")
				panic(error_value)
			}
			switch name {
			case "start":
				stream.start = number
			case "end":
				stream.end = number
			case "highWaterMark":
				stream.high_water_mark = int(number)
			}
		}
		if stream.end >= 0 && stream.end < stream.start {
			panic(hook_type_error(vm, "ERR_OUT_OF_RANGE", "start must be <= end"))
		}
		for _, name := range []string{"fd", "fs", "signal"} {
			if value := object.Get(name); value != nil && !goja.IsNull(value) && !goja.IsUndefined(value) {
				panic(hook_type_error(vm, "ERR_INVALID_ARG_VALUE", "ReadStream option is not supported: "+name))
			}
		}
		if value := object.Get("autoClose"); value != nil && !goja.IsUndefined(value) && !value.ToBoolean() {
			panic(hook_type_error(vm, "ERR_INVALID_ARG_VALUE", "upload streams require autoClose"))
		}
	}
	object := vm.NewObject()
	_ = object.SetSymbol(hook_stream_symbol, stream)
	_ = object.Set("path", path_value)
	_ = object.Set("readableHighWaterMark", stream.high_water_mark)
	_ = object.Set("destroy", func() *goja.Object { _ = stream.Close(); return object })
	for name, getter := range map[string]func() goja.Value{
		"bytesRead":     func() goja.Value { return vm.ToValue(stream.bytes_read) },
		"destroyed":     func() goja.Value { return vm.ToValue(stream.destroyed) },
		"readableEnded": func() goja.Value { return vm.ToValue(stream.ended) },
	} {
		read_property := getter
		_ = object.DefineAccessorProperty(name, vm.ToValue(func() goja.Value {
			stream.mu.Lock()
			defer stream.mu.Unlock()
			return read_property()
		}), goja.Undefined(), goja.FLAG_FALSE, goja.FLAG_TRUE)
	}
	return object
}

func (stream *hook_read_stream) Read(data []byte) (int, error) {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.ended {
		return 0, io.EOF
	}
	if stream.destroyed {
		return 0, os.ErrClosed
	}
	if stream.file == nil {
		file, err := os.OpenFile(stream.path, stream.flags, 0666)
		if err != nil {
			stream.destroyed = true
			return 0, err
		}
		stream.file = file
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			stream.destroyed = true
			_ = file.Close()
			if err == nil {
				err = fmt.Errorf("stream path is not a regular file: %s", stream.path)
			}
			return 0, err
		}
		if _, err := file.Seek(stream.start, io.SeekStart); err != nil {
			stream.destroyed = true
			_ = file.Close()
			return 0, err
		}
		stream.reader = file
		if stream.end >= 0 {
			stream.reader = io.LimitReader(file, stream.end-stream.start+1)
		}
	}
	if len(data) > stream.high_water_mark {
		data = data[:stream.high_water_mark]
	}
	count, err := stream.reader.Read(data)
	stream.bytes_read += int64(count)
	if err != nil {
		stream.ended = err == io.EOF
		stream.destroyed = true
		_ = stream.file.Close()
	}
	return count, err
}

func (stream *hook_read_stream) Close() error {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.destroyed {
		return nil
	}
	stream.destroyed = true
	if stream.file != nil {
		return stream.file.Close()
	}
	return nil
}

func (runtime *hook_runtime) install_stream_fetch() error {
	vm := runtime.vm
	_ = vm.Set("__isHookReadStream", func(value goja.Value) bool { return hook_value_stream(value) != nil })
	_ = vm.Set("__setHookStreamContentType", func(value goja.Value, explicit bool) {
		if stream := hook_value_stream(value); stream != nil {
			stream.content_type_set = explicit
		}
	})
	_, err := vm.RunString(`(() => {
  const isStream = __isHookReadStream;
  const setContentType = __setHookStreamContentType;
  delete globalThis.__isHookReadStream;
  delete globalThis.__setHookStreamContentType;
  const NativeRequest = Request;
  const nativeFetch = fetch;
  function prepare(input, init) {
    const body = init && init.body != null ? init.body : input && input._bodyInit;
    if (!isStream(body)) return new NativeRequest(input, init);
    const duplex = init && init.duplex !== undefined ? init.duplex : input && input.duplex;
    if (duplex !== 'half') throw new TypeError("RequestInit: duplex option is required and must be 'half' for a streaming body");
    const headers = init && init.headers !== undefined ? new Headers(init.headers) : input instanceof NativeRequest ? input.headers : null;
    setContentType(body, !!headers && headers.has('content-type'));
    const request = new NativeRequest(input, init);
    if (!headers || !headers.has('content-type')) request.headers.delete('content-type');
    request.duplex = 'half';
    return request;
  }
  globalThis.Request = function Request(input, init) {
    if (!(this instanceof Request)) throw new TypeError("Request must be called with new");
    return prepare(input, init);
  };
  Request.prototype = NativeRequest.prototype;
  Object.setPrototypeOf(Request, NativeRequest);
  globalThis.fetch = function fetch(input, init) {
    const body = init && init.body != null ? init.body : input && input._bodyInit;
    if (!isStream(body)) return nativeFetch(input, init);
    try { return nativeFetch(prepare(input, init)); }
    catch (error) { return Promise.reject(error); }
  };
})()`)
	return err
}
