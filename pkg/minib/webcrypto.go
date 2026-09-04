package minib

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
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
	type crypto_key struct {
		ecdh_private *ecdh.PrivateKey
		ecdh_public  *ecdh.PublicKey
		secret       []byte
	}
	crypto_keys := make(map[*goja.Object]crypto_key)
	new_crypto_key := func(key crypto_key, key_type string, extractable bool, algorithm map[string]any, usages goja.Value) *goja.Object {
		object := runtime.vm.NewObject()
		_ = object.Set("type", key_type)
		_ = object.Set("extractable", extractable)
		_ = object.Set("algorithm", algorithm)
		_ = object.Set("usages", usages)
		crypto_keys[object] = key
		return object
	}

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
	if err := subtle_object.Set("generateKey", func(call goja.FunctionCall) goja.Value {
		promise, resolve, reject := runtime.vm.NewPromise()
		algorithm := call.Argument(0).ToObject(runtime.vm)
		if !strings.EqualFold(algorithm.Get("name").String(), "ECDH") || !strings.EqualFold(algorithm.Get("namedCurve").String(), "P-256") {
			_ = reject(runtime.vm.NewGoError(fmt.Errorf("NotSupportedError: only ECDH P-256 is supported")))
			return runtime.vm.ToValue(promise)
		}
		private_key, err := ecdh.P256().GenerateKey(rand.Reader)
		if err != nil {
			_ = reject(runtime.vm.NewGoError(err))
			return runtime.vm.ToValue(promise)
		}
		key_pair := runtime.vm.NewObject()
		_ = key_pair.Set("privateKey", new_crypto_key(crypto_key{ecdh_private: private_key}, "private", call.Argument(1).ToBoolean(), map[string]any{"name": "ECDH", "namedCurve": "P-256"}, call.Argument(2)))
		_ = key_pair.Set("publicKey", new_crypto_key(crypto_key{ecdh_public: private_key.PublicKey()}, "public", true, map[string]any{"name": "ECDH", "namedCurve": "P-256"}, runtime.vm.NewArray()))
		_ = resolve(key_pair)
		return runtime.vm.ToValue(promise)
	}); err != nil {
		return err
	}
	if err := subtle_object.Set("importKey", func(call goja.FunctionCall) goja.Value {
		promise, resolve, reject := runtime.vm.NewPromise()
		if !strings.EqualFold(call.Argument(0).String(), "raw") {
			_ = reject(runtime.vm.NewGoError(fmt.Errorf("NotSupportedError: only raw keys are supported")))
			return runtime.vm.ToValue(promise)
		}
		data, err := javascript_bytes(call.Argument(1))
		if err != nil {
			_ = reject(runtime.vm.NewTypeError(err.Error()))
			return runtime.vm.ToValue(promise)
		}
		algorithm := call.Argument(2).ToObject(runtime.vm)
		algorithm_name := algorithm.Get("name").String()
		switch strings.ToUpper(algorithm_name) {
		case "ECDH":
			public_key, key_err := ecdh.P256().NewPublicKey(data)
			if key_err != nil {
				_ = reject(runtime.vm.NewGoError(key_err))
				return runtime.vm.ToValue(promise)
			}
			_ = resolve(new_crypto_key(crypto_key{ecdh_public: public_key}, "public", call.Argument(3).ToBoolean(), map[string]any{"name": "ECDH", "namedCurve": "P-256"}, call.Argument(4)))
		case "AES-CBC":
			if _, key_err := aes.NewCipher(data); key_err != nil {
				_ = reject(runtime.vm.NewGoError(key_err))
				return runtime.vm.ToValue(promise)
			}
			_ = resolve(new_crypto_key(crypto_key{secret: append([]byte(nil), data...)}, "secret", call.Argument(3).ToBoolean(), map[string]any{"name": "AES-CBC", "length": len(data) * 8}, call.Argument(4)))
		default:
			_ = reject(runtime.vm.NewGoError(fmt.Errorf("NotSupportedError: unsupported key algorithm %q", algorithm_name)))
		}
		return runtime.vm.ToValue(promise)
	}); err != nil {
		return err
	}
	if err := subtle_object.Set("exportKey", func(call goja.FunctionCall) goja.Value {
		promise, resolve, reject := runtime.vm.NewPromise()
		object, ok := call.Argument(1).(*goja.Object)
		key, found := crypto_keys[object]
		if !ok || !found || !strings.EqualFold(call.Argument(0).String(), "raw") || key.ecdh_public == nil {
			_ = reject(runtime.vm.NewGoError(fmt.Errorf("NotSupportedError: key cannot be exported as raw")))
			return runtime.vm.ToValue(promise)
		}
		_ = resolve(runtime.vm.NewArrayBuffer(key.ecdh_public.Bytes()))
		return runtime.vm.ToValue(promise)
	}); err != nil {
		return err
	}
	if err := subtle_object.Set("deriveBits", func(call goja.FunctionCall) goja.Value {
		promise, resolve, reject := runtime.vm.NewPromise()
		algorithm := call.Argument(0).ToObject(runtime.vm)
		public_object, public_ok := algorithm.Get("public").(*goja.Object)
		private_object, private_ok := call.Argument(1).(*goja.Object)
		public_key, public_found := crypto_keys[public_object]
		private_key, private_found := crypto_keys[private_object]
		if !public_ok || !private_ok || !public_found || !private_found || public_key.ecdh_public == nil || private_key.ecdh_private == nil {
			_ = reject(runtime.vm.NewTypeError("deriveBits requires ECDH public and private keys"))
			return runtime.vm.ToValue(promise)
		}
		secret, err := private_key.ecdh_private.ECDH(public_key.ecdh_public)
		if err != nil {
			_ = reject(runtime.vm.NewGoError(err))
			return runtime.vm.ToValue(promise)
		}
		byte_length := int(call.Argument(2).ToInteger() / 8)
		if byte_length < 0 || byte_length > len(secret) {
			_ = reject(runtime.vm.NewGoError(fmt.Errorf("OperationError: invalid derived bit length")))
			return runtime.vm.ToValue(promise)
		}
		_ = resolve(runtime.vm.NewArrayBuffer(secret[:byte_length]))
		return runtime.vm.ToValue(promise)
	}); err != nil {
		return err
	}
	if err := subtle_object.Set("encrypt", func(call goja.FunctionCall) goja.Value {
		promise, resolve, reject := runtime.vm.NewPromise()
		algorithm := call.Argument(0).ToObject(runtime.vm)
		key_object, ok := call.Argument(1).(*goja.Object)
		key, found := crypto_keys[key_object]
		iv, iv_err := javascript_bytes(algorithm.Get("iv"))
		plaintext, data_err := javascript_bytes(call.Argument(2))
		block, key_err := aes.NewCipher(key.secret)
		if !ok || !found || !strings.EqualFold(algorithm.Get("name").String(), "AES-CBC") || iv_err != nil || data_err != nil || key_err != nil || len(iv) != aes.BlockSize {
			_ = reject(runtime.vm.NewGoError(fmt.Errorf("OperationError: invalid AES-CBC parameters")))
			return runtime.vm.ToValue(promise)
		}
		padding_length := aes.BlockSize - len(plaintext)%aes.BlockSize
		padded := make([]byte, len(plaintext)+padding_length)
		copy(padded, plaintext)
		for index := len(plaintext); index < len(padded); index++ {
			padded[index] = byte(padding_length)
		}
		cipher.NewCBCEncrypter(block, iv).CryptBlocks(padded, padded)
		_ = resolve(runtime.vm.NewArrayBuffer(padded))
		return runtime.vm.ToValue(promise)
	}); err != nil {
		return err
	}
	if err := crypto_object.Set("subtle", subtle_object); err != nil {
		return err
	}
	return window.Set("crypto", crypto_object)
}
