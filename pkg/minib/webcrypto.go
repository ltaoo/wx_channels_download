package minib

import (
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"strings"

	"github.com/dop251/goja"
)

func (runtime *page_runtime) install_web_crypto(window *goja.Object) error {
	crypto_object := runtime.vm.NewObject()
	subtle_object := runtime.vm.NewObject()

	if err := crypto_object.Set("getRandomValues", func(call goja.FunctionCall) goja.Value {
		value := call.Argument(0)
		data, err := javascript_bytes(value)
		if err != nil {
			panic(runtime.vm.NewTypeError("getRandomValues requires an integer TypedArray"))
		}
		if len(data) > 65536 {
			panic(runtime.vm.NewGoError(fmt.Errorf("QuotaExceededError: requested more than 65536 random bytes")))
		}
		if _, err := rand.Read(data); err != nil {
			panic(runtime.vm.NewGoError(err))
		}
		return value
	}); err != nil {
		return err
	}
	if err := crypto_object.Set("randomUUID", func() string {
		data := make([]byte, 16)
		if _, err := rand.Read(data); err != nil {
			panic(runtime.vm.NewGoError(err))
		}
		data[6] = data[6]&0x0f | 0x40
		data[8] = data[8]&0x3f | 0x80
		return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
			data[0:4], data[4:6], data[6:8], data[8:10], data[10:16])
	}); err != nil {
		return err
	}
	if err := subtle_object.Set("digest", func(call goja.FunctionCall) goja.Value {
		promise, resolve, reject := runtime.vm.NewPromise()
		algorithm := call.Argument(0)
		algorithm_name := algorithm.String()
		if object, ok := algorithm.(*goja.Object); ok {
			algorithm_name = object.Get("name").String()
		}
		data, err := javascript_bytes(call.Argument(1))
		if err != nil {
			_ = reject(runtime.vm.NewTypeError(err.Error()))
			return runtime.vm.ToValue(promise)
		}
		var digest []byte
		switch strings.ToUpper(strings.ReplaceAll(algorithm_name, "_", "-")) {
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
			_ = reject(runtime.vm.NewGoError(fmt.Errorf("NotSupportedError: unsupported digest algorithm %q", algorithm_name)))
			return runtime.vm.ToValue(promise)
		}
		_ = resolve(runtime.vm.NewArrayBuffer(digest))
		return runtime.vm.ToValue(promise)
	}); err != nil {
		return err
	}
	if err := crypto_object.Set("subtle", subtle_object); err != nil {
		return err
	}
	return window.Set("crypto", crypto_object)
}
