package zhihu

import (
	"encoding/json"
	"fmt"
	"testing"
)

const pcweb_test_script_url = "https://static.zhihu.com/zse-ck/v4/0123456789abcdef0123456789abcdef.js"
const pcweb_test_target_url = "https://www.zhihu.com/question/1/answer/2"

func TestGeneratePCWebZSECookieWithGoja(t *testing.T) {
	meta := "test-meta"
	script := []byte(fmt.Sprintf(`
if (atob("/w==").charCodeAt(0) !== 255 || btoa(atob("/w==")) !== "/w==") {
  throw new Error("binary base64 host functions are corrupt");
}
document.cookie = "__zse_ck=goja-%s";`, meta))

	cookie, err := generate_pcweb_zse_cookie(script, pcweb_test_script_url, meta, pcweb_test_target_url)
	if err != nil {
		t.Fatalf("generate cookie: %v", err)
	}
	if cookie != "goja-"+meta {
		t.Fatalf("unexpected cookie %q", cookie)
	}
}

func TestGeneratePCWebZSECookieRunsTimers(t *testing.T) {
	meta := "timer-meta"
	script := []byte(fmt.Sprintf(`setTimeout(() => { document.cookie = "__zse_ck=timer-%s"; }, 1);`, meta))

	cookie, err := generate_pcweb_zse_cookie(script, pcweb_test_script_url, meta, pcweb_test_target_url)
	if err != nil {
		t.Fatalf("generate cookie: %v", err)
	}
	if cookie != "timer-"+meta {
		t.Fatalf("unexpected cookie %q", cookie)
	}
}

func TestGeneratePCWebZSECookieBridgesWebAssemblyImports(t *testing.T) {
	// (module
	//   (import "env" "double" (func $double (param i32) (result i32)))
	//   (func (export "call") (param i32) (result i32)
	//     local.get 0
	//     call $double))
	wasm := []byte{
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00,
		0x01, 0x06, 0x01, 0x60, 0x01, 0x7f, 0x01, 0x7f,
		0x02, 0x0e, 0x01, 0x03, 0x65, 0x6e, 0x76, 0x06, 0x64, 0x6f, 0x75, 0x62, 0x6c, 0x65, 0x00, 0x00,
		0x03, 0x02, 0x01, 0x00,
		0x07, 0x08, 0x01, 0x04, 0x63, 0x61, 0x6c, 0x6c, 0x00, 0x01,
		0x0a, 0x08, 0x01, 0x06, 0x00, 0x20, 0x00, 0x10, 0x00, 0x0b,
	}
	meta := "wasm-meta"
	wasm_numbers := make([]int, len(wasm))
	for index, value := range wasm {
		wasm_numbers[index] = int(value)
	}
	wasm_json, err := json.Marshal(wasm_numbers)
	if err != nil {
		t.Fatalf("marshal WebAssembly test module: %v", err)
	}
	script := []byte(fmt.Sprintf(`
WebAssembly.instantiate(new Uint8Array(%v), {
  env: { double(value) { return value * 2; } },
}).then(({ instance }) => {
  document.cookie = "__zse_ck=" + instance.exports.call(21) + "-%s";
});`, string(wasm_json), meta))

	cookie, err := generate_pcweb_zse_cookie(script, pcweb_test_script_url, meta, pcweb_test_target_url)
	if err != nil {
		t.Fatalf("generate cookie: %v", err)
	}
	if cookie != "42-"+meta {
		t.Fatalf("unexpected cookie %q", cookie)
	}
}
