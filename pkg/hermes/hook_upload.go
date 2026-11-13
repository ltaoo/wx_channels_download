package hermes

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"os"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/buffer"
)

var hook_blob_symbol = goja.NewSymbol("hook Blob")
var hook_form_symbol = goja.NewSymbol("hook FormData")
var hook_form_escape = strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\r", "%0D", "\n", "%0A")

type hook_blob_part struct {
	data     []byte
	path     string
	size     int64
	modified time.Time
}

type hook_blob struct {
	parts      []hook_blob_part
	size       int64
	media_type string
}

type hook_form_field struct {
	name     string
	filename string
	text     string
	blob     *hook_blob
}

type hook_form_data struct{ fields []hook_form_field }

type hook_body_reader struct {
	io.Reader
	closers []io.Closer
}

func (reader *hook_body_reader) Close() error {
	var first_err error
	for _, closer := range reader.closers {
		if err := closer.Close(); first_err == nil {
			first_err = err
		}
	}
	return first_err
}

type hook_request_body struct {
	io.ReadCloser
	size   int64
	reopen func() (io.ReadCloser, error)
}

func hook_media_type(value string) string {
	for _, char := range value {
		if char < 32 || char > 126 {
			return ""
		}
	}
	return strings.ToLower(value)
}

func hook_value_blob(value goja.Value) *hook_blob {
	if object, ok := value.(*goja.Object); ok {
		if value := object.GetSymbol(hook_blob_symbol); value != nil {
			blob, _ := value.Export().(*hook_blob)
			return blob
		}
	}
	return nil
}

func (runtime *hook_runtime) install_upload_types() {
	vm := runtime.vm
	_ = vm.Set("Blob", func(call goja.ConstructorCall) *goja.Object {
		blob := &hook_blob{}
		parts := call.Argument(0)
		if !goja.IsUndefined(parts) && !goja.IsNull(parts) {
			var values []goja.Value
			if err := vm.ExportTo(parts, &values); err != nil {
				panic(vm.NewTypeError("Blob parts must be an array"))
			}
			for _, value := range values {
				if nested := hook_value_blob(value); nested != nil {
					blob.parts = append(blob.parts, nested.parts...)
					blob.size += nested.size
					continue
				}
				var data []byte
				switch value.Export().(type) {
				case goja.ArrayBuffer, []byte:
					data = append([]byte(nil), buffer.DecodeBytes(vm, value, goja.Undefined())...)
				default:
					data = []byte(value.String())
				}
				blob.parts = append(blob.parts, hook_blob_part{data: data, size: int64(len(data))})
				blob.size += int64(len(data))
			}
		}
		if options, ok := call.Argument(1).(*goja.Object); ok {
			if value := options.Get("type"); value != nil && !goja.IsUndefined(value) {
				blob.media_type = hook_media_type(value.String())
			}
		}
		return runtime.init_blob_object(call.This, blob)
	})
	_ = vm.Set("FormData", func(call goja.ConstructorCall) *goja.Object {
		form := &hook_form_data{}
		_ = call.This.SetSymbol(hook_form_symbol, form)
		_ = call.This.Set("append", func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 2 {
				panic(vm.NewTypeError("FormData.append requires name and value"))
			}
			field := hook_form_field{name: call.Argument(0).String(), blob: hook_value_blob(call.Argument(1))}
			if field.blob == nil {
				if len(call.Arguments) > 2 {
					panic(vm.NewTypeError("FormData file value must be a Blob"))
				}
				field.text = call.Argument(1).String()
			} else {
				field.filename = "blob"
				if len(call.Arguments) > 2 {
					field.filename = call.Argument(2).String()
				}
			}
			form.fields = append(form.fields, field)
			return goja.Undefined()
		})
		return call.This
	})
	_ = vm.Set("FileReader", func(call goja.ConstructorCall) *goja.Object {
		reader := call.This
		read := func(value goja.Value, text bool) {
			blob := hook_value_blob(value)
			if blob == nil {
				panic(vm.NewTypeError("FileReader requires a Blob"))
			}
			promise := runtime.read_blob(blob, text)
			then, _ := goja.AssertFunction(vm.ToValue(promise).ToObject(vm).Get("then"))
			_, _ = then(vm.ToValue(promise), vm.ToValue(func(value goja.Value) {
				_ = reader.Set("result", value)
				if callback, ok := goja.AssertFunction(reader.Get("onload")); ok {
					_, _ = callback(reader)
				}
			}), vm.ToValue(func(reason goja.Value) {
				_ = reader.Set("error", reason)
				if callback, ok := goja.AssertFunction(reader.Get("onerror")); ok {
					_, _ = callback(reader)
				}
			}))
		}
		_ = reader.Set("readAsText", func(value goja.Value) { read(value, true) })
		_ = reader.Set("readAsArrayBuffer", func(value goja.Value) { read(value, false) })
		return reader
	})
}

func (runtime *hook_runtime) blob_object(blob *hook_blob) *goja.Object {
	object := runtime.vm.NewObject()
	_ = object.SetPrototype(runtime.vm.Get("Blob").ToObject(runtime.vm).Get("prototype").ToObject(runtime.vm))
	return runtime.init_blob_object(object, blob)
}

func (runtime *hook_runtime) init_blob_object(object *goja.Object, blob *hook_blob) *goja.Object {
	_ = object.SetSymbol(hook_blob_symbol, blob)
	_ = object.DefineDataProperty("size", runtime.vm.ToValue(blob.size), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_FALSE)
	_ = object.DefineDataProperty("type", runtime.vm.ToValue(blob.media_type), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_FALSE)
	_ = object.Set("arrayBuffer", func() *goja.Promise { return runtime.read_blob(blob, false) })
	_ = object.Set("text", func() *goja.Promise { return runtime.read_blob(blob, true) })
	return object
}

func (runtime *hook_runtime) read_blob(blob *hook_blob, text bool) *goja.Promise {
	return runtime.async_io(func() (any, error) {
		reader, err := open_hook_blob(blob)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return io.ReadAll(reader)
	}, func(value any) goja.Value {
		if text {
			return runtime.vm.ToValue(string(value.([]byte)))
		}
		return runtime.vm.ToValue(runtime.vm.NewArrayBuffer(value.([]byte)))
	})
}

func open_hook_blob(blob *hook_blob) (io.ReadCloser, error) {
	body := &hook_body_reader{}
	var readers []io.Reader
	for _, part := range blob.parts {
		if part.path == "" {
			readers = append(readers, bytes.NewReader(part.data))
			continue
		}
		path := part.path
		file, err := os.Open(path)
		if err != nil {
			_ = body.Close()
			return nil, err
		}
		body.closers = append(body.closers, file)
		info, err := file.Stat()
		if err != nil {
			_ = body.Close()
			return nil, err
		}
		if info.Size() != part.size || !info.ModTime().Equal(part.modified) {
			_ = body.Close()
			return nil, fmt.Errorf("file changed after openAsBlob: %s", path)
		}
		readers = append(readers, io.LimitReader(file, part.size))
	}
	body.Reader = io.MultiReader(readers...)
	return body, nil
}

func (runtime *hook_runtime) request_body(value goja.Value) (*hook_request_body, string, error) {
	if value == nil || goja.IsNull(value) || goja.IsUndefined(value) {
		return nil, "", nil
	}
	if stream := hook_value_stream(value); stream != nil {
		stream.mu.Lock()
		if stream.consumed || stream.destroyed {
			stream.mu.Unlock()
			return nil, "", fmt.Errorf("ReadStream has already been consumed or destroyed")
		}
		stream.consumed = true
		stream.mu.Unlock()
		runtime.streams = append(runtime.streams, stream)
		return &hook_request_body{ReadCloser: stream, size: -1}, "", nil
	}
	if object, ok := value.(*goja.Object); ok {
		if stored := object.GetSymbol(hook_form_symbol); stored != nil {
			form, ok := stored.Export().(*hook_form_data)
			if !ok {
				return nil, "", fmt.Errorf("invalid FormData body")
			}
			fields := append([]hook_form_field(nil), form.fields...)
			boundary := multipart.NewWriter(io.Discard).Boundary()
			reader, size, err := open_hook_form(fields, boundary)
			if err != nil {
				return nil, "", err
			}
			return &hook_request_body{ReadCloser: reader, size: size, reopen: func() (io.ReadCloser, error) {
				reader, _, err := open_hook_form(fields, boundary)
				return reader, err
			}}, "multipart/form-data; boundary=" + boundary, nil
		}
	}
	if blob := hook_value_blob(value); blob != nil {
		reader, err := open_hook_blob(blob)
		if err != nil {
			return nil, "", err
		}
		return &hook_request_body{ReadCloser: reader, size: blob.size, reopen: func() (io.ReadCloser, error) { return open_hook_blob(blob) }}, blob.media_type, nil
	}
	data := append([]byte(nil), buffer.DecodeBytes(runtime.vm, value, goja.Undefined())...)
	reopen := func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }
	reader, _ := reopen()
	return &hook_request_body{ReadCloser: reader, size: int64(len(data)), reopen: reopen}, "", nil
}

func open_hook_form(fields []hook_form_field, boundary string) (io.ReadCloser, int64, error) {
	body := &hook_body_reader{}
	var header bytes.Buffer
	writer := multipart.NewWriter(&header)
	if err := writer.SetBoundary(boundary); err != nil {
		return nil, 0, err
	}
	var readers []io.Reader
	var size int64
	flush_header := func() {
		data := append([]byte(nil), header.Bytes()...)
		readers = append(readers, bytes.NewReader(data))
		size += int64(len(data))
		header.Reset()
	}
	for _, field := range fields {
		disposition := `form-data; name="` + hook_form_escape.Replace(field.name) + `"`
		mime_header := make(textproto.MIMEHeader)
		if field.blob != nil {
			disposition += `; filename="` + hook_form_escape.Replace(field.filename) + `"`
			media_type := field.blob.media_type
			if media_type == "" {
				media_type = "application/octet-stream"
			}
			mime_header.Set("Content-Type", media_type)
		}
		mime_header.Set("Content-Disposition", disposition)
		if _, err := writer.CreatePart(mime_header); err != nil {
			_ = body.Close()
			return nil, 0, err
		}
		flush_header()
		if field.blob == nil {
			readers = append(readers, strings.NewReader(field.text))
			size += int64(len(field.text))
		} else {
			reader, err := open_hook_blob(field.blob)
			if err != nil {
				_ = body.Close()
				return nil, 0, err
			}
			body.closers = append(body.closers, reader)
			readers = append(readers, reader)
			size += field.blob.size
		}
	}
	if err := writer.Close(); err != nil {
		_ = body.Close()
		return nil, 0, err
	}
	flush_header()
	body.Reader = io.MultiReader(readers...)
	return body, size, nil
}
