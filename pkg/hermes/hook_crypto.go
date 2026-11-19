package hermes

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"hash"
	"strings"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/buffer"
)

func hook_crypto_module(vm *goja.Runtime) *goja.Object {
	module := vm.NewObject()
	_ = module.Set("createHmac", func(algorithm string, key goja.Value) *goja.Object {
		var new_hash func() hash.Hash
		switch strings.ToLower(algorithm) {
		case "sha1":
			new_hash = sha1.New
		case "sha256":
			new_hash = sha256.New
		case "sha512":
			new_hash = sha512.New
		default:
			panic(hook_type_error(vm, "ERR_CRYPTO_INVALID_DIGEST", "unsupported HMAC algorithm: "+algorithm))
		}
		if key == nil || goja.IsUndefined(key) || goja.IsNull(key) {
			panic(hook_type_error(vm, "ERR_INVALID_ARG_TYPE", "HMAC key is required"))
		}
		mac := hmac.New(new_hash, buffer.DecodeBytes(vm, key, goja.Undefined()))
		result := vm.NewObject()
		finalized := false
		check_finalized := func() {
			if finalized {
				error_value := vm.NewGoError(errors.New("Digest already called"))
				_ = error_value.Set("name", "Error")
				_ = error_value.Set("code", "ERR_CRYPTO_HASH_FINALIZED")
				panic(error_value)
			}
		}
		_ = result.Set("update", func(data, encoding goja.Value) *goja.Object {
			check_finalized()
			if data == nil || goja.IsUndefined(data) || goja.IsNull(data) {
				panic(hook_type_error(vm, "ERR_INVALID_ARG_TYPE", "HMAC data is required"))
			}
			if encoding == nil {
				encoding = goja.Undefined()
			}
			if goja.IsString(data) && !goja.IsUndefined(encoding) && !goja.IsNull(encoding) && buffer.StringCodecByName(encoding.String()) == nil {
				panic(hook_type_error(vm, "ERR_UNKNOWN_ENCODING", "unknown encoding: "+encoding.String()))
			}
			_, _ = mac.Write(buffer.DecodeBytes(vm, data, encoding))
			return result
		})
		_ = result.Set("digest", func(encoding goja.Value) goja.Value {
			if encoding == nil {
				encoding = goja.Undefined()
			}
			if finalized {
				return buffer.EncodeBytes(vm, nil, encoding)
			}
			finalized = true
			if goja.IsString(encoding) {
				name := strings.ToLower(encoding.String())
				if name == "base64url" {
					name = "base64Url"
				}
				encoding = vm.ToValue(name)
			}
			return buffer.EncodeBytes(vm, mac.Sum(nil), encoding)
		})
		return result
	})
	return module
}
