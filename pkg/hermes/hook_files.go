package hermes

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/buffer"
)

func (runtime *hook_runtime) install_file_modules() {
	vm := runtime.vm
	fs := vm.NewObject()
	promises := vm.NewObject()
	encoding := func(options goja.Value) goja.Value {
		if options == nil || goja.IsUndefined(options) || goja.IsNull(options) {
			return goja.Undefined()
		}
		if object, ok := options.(*goja.Object); ok {
			options = object.Get("encoding")
		} else if !goja.IsString(options) {
			panic(hook_type_error(vm, "ERR_INVALID_ARG_TYPE", "options must be a string or an object"))
		}
		if options == nil || goja.IsNull(options) || goja.IsUndefined(options) {
			return goja.Undefined()
		}
		if !goja.IsString(options) {
			panic(hook_type_error(vm, "ERR_INVALID_ARG_TYPE", "encoding must be a string"))
		}
		name := strings.ToLower(options.String())
		if name == "" {
			return goja.Undefined()
		}
		if name == "base64url" {
			name = "base64Url"
		}
		switch name {
		case "ascii", "latin1", "binary", "utf16le", "utf-16le", "ucs2", "ucs-2":
			return vm.ToValue(name)
		}
		if buffer.StringCodecByName(name) == nil {
			panic(hook_type_error(vm, "ERR_INVALID_ARG_VALUE", "Unknown encoding: "+options.String()))
		}
		return vm.ToValue(name)
	}
	flags := func(options goja.Value) int {
		object, ok := options.(*goja.Object)
		if !ok {
			return os.O_RDONLY
		}
		value := object.Get("flag")
		if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
			return os.O_RDONLY
		}
		if goja.IsNumber(value) {
			flag := value.ToInteger()
			if float64(flag) != value.ToFloat() || flag < -2147483648 || flag > 2147483647 {
				constructor, _ := goja.AssertConstructor(vm.Get("RangeError"))
				error_value, _ := constructor(nil, vm.ToValue("flag must be a 32-bit integer"))
				_ = error_value.Set("code", "ERR_OUT_OF_RANGE")
				panic(error_value)
			}
			return int(flag)
		}
		flag, ok := map[string]int{
			"r": os.O_RDONLY, "r+": os.O_RDWR,
			"rs": os.O_RDONLY | os.O_SYNC, "sr": os.O_RDONLY | os.O_SYNC,
			"rs+": os.O_RDWR | os.O_SYNC, "sr+": os.O_RDWR | os.O_SYNC,
			"w": os.O_WRONLY | os.O_CREATE | os.O_TRUNC, "w+": os.O_RDWR | os.O_CREATE | os.O_TRUNC,
			"wx": os.O_WRONLY | os.O_CREATE | os.O_TRUNC | os.O_EXCL, "xw": os.O_WRONLY | os.O_CREATE | os.O_TRUNC | os.O_EXCL,
			"wx+": os.O_RDWR | os.O_CREATE | os.O_TRUNC | os.O_EXCL, "xw+": os.O_RDWR | os.O_CREATE | os.O_TRUNC | os.O_EXCL,
			"a": os.O_WRONLY | os.O_CREATE | os.O_APPEND, "a+": os.O_RDWR | os.O_CREATE | os.O_APPEND,
			"ax": os.O_WRONLY | os.O_CREATE | os.O_APPEND | os.O_EXCL, "xa": os.O_WRONLY | os.O_CREATE | os.O_APPEND | os.O_EXCL,
			"ax+": os.O_RDWR | os.O_CREATE | os.O_APPEND | os.O_EXCL, "xa+": os.O_RDWR | os.O_CREATE | os.O_APPEND | os.O_EXCL,
			"as": os.O_WRONLY | os.O_CREATE | os.O_APPEND | os.O_SYNC, "sa": os.O_WRONLY | os.O_CREATE | os.O_APPEND | os.O_SYNC,
			"as+": os.O_RDWR | os.O_CREATE | os.O_APPEND | os.O_SYNC, "sa+": os.O_RDWR | os.O_CREATE | os.O_APPEND | os.O_SYNC,
		}[value.String()]
		if !ok {
			panic(hook_type_error(vm, "ERR_INVALID_ARG_VALUE", "invalid file flag: "+value.String()))
		}
		return flag
	}
	_ = fs.Set("readFileSync", func(path_value, options goja.Value) goja.Value {
		path := hook_file_path(vm, path_value)
		codec := encoding(options)
		data, err := read_hook_file(path, flags(options))
		if err != nil {
			panic(hook_file_error(vm, err))
		}
		return hook_file_data(vm, data, codec)
	})
	_ = promises.Set("readFile", func(path_value, options goja.Value) *goja.Promise {
		var path string
		var codec goja.Value
		var flag int
		if exception := vm.Try(func() {
			path = hook_file_path(vm, path_value)
			codec = encoding(options)
			flag = flags(options)
		}); exception != nil {
			promise, _, reject := vm.NewPromise()
			_ = reject(exception.Value())
			return promise
		}
		return runtime.async_io(func() (any, error) { return read_hook_file(path, flag) },
			func(value any) goja.Value { return hook_file_data(vm, value.([]byte), codec) })
	})
	_ = fs.Set("readFile", func(call goja.FunctionCall) goja.Value {
		options, callback_value := call.Argument(1), call.Argument(2)
		if len(call.Arguments) == 2 {
			options, callback_value = goja.Undefined(), options
		}
		callback, ok := goja.AssertFunction(callback_value)
		if !ok {
			panic(hook_type_error(vm, "ERR_INVALID_ARG_TYPE", "callback must be a function"))
		}
		path := hook_file_path(vm, call.Argument(0))
		codec := encoding(options)
		flag := flags(options)
		ctx := runtime.request_ctx
		runtime.begin_io()
		go func() {
			data, read_err := read_hook_file(path, flag)
			runtime.loop.RunOnLoop(func(*goja.Runtime) {
				if ctx.Err() != nil {
					return
				}
				var callback_err error
				if read_err != nil {
					_, callback_err = callback(goja.Undefined(), hook_file_error(vm, read_err), goja.Undefined())
				} else {
					_, callback_err = callback(goja.Undefined(), goja.Null(), hook_file_data(vm, data, codec))
				}
				runtime.pending_io--
				runtime.stop_if_idle(callback_err)
			})
		}()
		return goja.Undefined()
	})
	_ = fs.Set("promises", promises)
	_ = fs.Set("createReadStream", func(path_value, options goja.Value) *goja.Object {
		if !goja.IsUndefined(encoding(options)) {
			panic(hook_type_error(vm, "ERR_INVALID_ARG_VALUE", "upload ReadStream requires binary encoding (null)"))
		}
		stream_options := vm.NewObject()
		if object, ok := options.(*goja.Object); ok {
			if value := object.Get("flags"); value != nil {
				_ = stream_options.Set("flag", value)
			}
		}
		return runtime.new_read_stream(path_value, options, flags(stream_options))
	})
	_ = fs.Set("openAsBlob", func(path_value, options goja.Value) *goja.Promise {
		path, err := filepath.Abs(hook_file_path(vm, path_value))
		if err != nil {
			panic(hook_file_error(vm, err))
		}
		media_type := ""
		if object, ok := options.(*goja.Object); ok {
			if value := object.Get("type"); value != nil && !goja.IsUndefined(value) {
				media_type = value.String()
			}
		}
		return runtime.async_io(func() (any, error) {
			info, err := os.Stat(path)
			if err != nil {
				return nil, err
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("path is not a regular file: %s", path)
			}
			return &hook_blob{parts: []hook_blob_part{{path: path, size: info.Size(), modified: info.ModTime()}}, size: info.Size(), media_type: hook_media_type(media_type)}, nil
		}, func(value any) goja.Value { return runtime.blob_object(value.(*hook_blob)) })
	})
	_ = fs.Set("existsSync", func(path_value goja.Value) bool {
		var path string
		if exception := vm.Try(func() { path = hook_file_path(vm, path_value) }); exception != nil {
			return false
		}
		_, err := os.Stat(path)
		return err == nil
	})
	buffer_module := vm.NewObject()
	_ = buffer_module.Set("Buffer", vm.Get("Buffer"))
	url_module := vm.NewObject()
	_ = url_module.Set("URL", vm.Get("URL"))
	_ = url_module.Set("URLSearchParams", vm.Get("URLSearchParams"))
	crypto_module := hook_crypto_module(vm)
	_ = vm.Set("require", func(name string) goja.Value {
		switch name {
		case "fs", "node:fs":
			return fs
		case "fs/promises", "node:fs/promises":
			return promises
		case "buffer", "node:buffer":
			return buffer_module
		case "url", "node:url":
			return url_module
		case "crypto", "node:crypto":
			return crypto_module
		case "console", "node:console":
			return vm.Get("console")
		default:
			panic(vm.NewTypeError("Unsupported hook module: %s", name))
		}
	})
	module := vm.NewObject()
	exports := vm.NewObject()
	_ = module.Set("exports", exports)
	_ = vm.Set("module", module)
	_ = vm.Set("exports", exports)
	runtime.install_upload_types()
}

func read_hook_file(path string, flags int) ([]byte, error) {
	file, err := os.OpenFile(path, flags, 0666)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func hook_file_data(vm *goja.Runtime, data []byte, encoding goja.Value) goja.Value {
	switch encoding.String() {
	case "ascii", "latin1", "binary":
		chars := make([]uint16, len(data))
		for index, value := range data {
			if encoding.String() == "ascii" {
				value &= 0x7f
			}
			chars[index] = uint16(value)
		}
		return goja.StringFromUTF16(chars)
	case "utf16le", "utf-16le", "ucs2", "ucs-2":
		chars := make([]uint16, len(data)/2)
		for index := range chars {
			chars[index] = uint16(data[index*2]) | uint16(data[index*2+1])<<8
		}
		return goja.StringFromUTF16(chars)
	}
	return buffer.EncodeBytes(vm, data, encoding)
}

func (runtime *hook_runtime) async_io(work func() (any, error), convert func(any) goja.Value) *goja.Promise {
	promise, resolve, reject := runtime.vm.NewPromise()
	ctx := runtime.request_ctx
	runtime.begin_io()
	go func() {
		value, err := work()
		runtime.loop.RunOnLoop(func(vm *goja.Runtime) {
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				_ = reject(hook_file_error(vm, err))
			} else {
				_ = resolve(convert(value))
			}
			runtime.pending_io--
			runtime.stop_if_idle(nil)
		})
	}()
	return promise
}

func hook_file_error(vm *goja.Runtime, err error) *goja.Object {
	value := vm.NewGoError(err)
	_ = value.Set("name", "Error")
	if errors.Is(err, os.ErrNotExist) {
		_ = value.Set("code", "ENOENT")
	}
	if errors.Is(err, os.ErrPermission) {
		_ = value.Set("code", "EACCES")
	}
	for code, target := range map[string]error{"EISDIR": syscall.EISDIR, "ENOTDIR": syscall.ENOTDIR, "ELOOP": syscall.ELOOP, "EBADF": syscall.EBADF, "EINVAL": syscall.EINVAL, "EEXIST": os.ErrExist} {
		if errors.Is(err, target) {
			_ = value.Set("code", code)
		}
	}
	var path_error *os.PathError
	if errors.As(err, &path_error) {
		_ = value.Set("path", path_error.Path)
		_ = value.Set("syscall", path_error.Op)
	}
	return value
}

func hook_type_error(vm *goja.Runtime, code, message string) *goja.Object {
	value := vm.NewTypeError(message)
	_ = value.Set("code", code)
	return value
}

func hook_file_path(vm *goja.Runtime, value goja.Value) string {
	var path string
	if value != nil && goja.IsString(value) {
		path = value.String()
	} else if value != nil && vm.InstanceOf(value, vm.Get("Buffer").ToObject(vm)) {
		path = string(buffer.DecodeBytes(vm, value, goja.Undefined()))
	} else if value != nil && vm.InstanceOf(value, vm.Get("URL").ToObject(vm)) {
		parsed, err := url.Parse(value.String())
		if err != nil || parsed.Scheme != "file" {
			panic(hook_type_error(vm, "ERR_INVALID_URL_SCHEME", "URL must use the file: scheme"))
		}
		escaped := strings.ToLower(parsed.EscapedPath())
		if strings.Contains(escaped, "%2f") || runtime.GOOS == "windows" && strings.Contains(escaped, "%5c") {
			panic(hook_type_error(vm, "ERR_INVALID_FILE_URL_PATH", "file URL must not contain encoded path separators"))
		}
		path = parsed.Path
		if runtime.GOOS == "windows" {
			if parsed.Host != "" && parsed.Host != "localhost" {
				path = "//" + parsed.Host + path
			} else if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
				path = path[1:]
			} else {
				panic(hook_type_error(vm, "ERR_INVALID_FILE_URL_PATH", "file URL path must be absolute"))
			}
		} else if parsed.Host != "" && parsed.Host != "localhost" {
			panic(hook_type_error(vm, "ERR_INVALID_FILE_URL_HOST", "file URL host must be localhost or empty"))
		}
		path = filepath.FromSlash(path)
	} else {
		panic(hook_type_error(vm, "ERR_INVALID_ARG_TYPE", "path must be a string, Buffer or file URL"))
	}
	if strings.ContainsRune(path, 0) {
		panic(hook_type_error(vm, "ERR_INVALID_ARG_VALUE", "path must not contain null bytes"))
	}
	return path
}
