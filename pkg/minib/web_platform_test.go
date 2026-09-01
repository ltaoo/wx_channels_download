package minib

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dop251/goja"
)

func TestWebPlatformCryptoBase64AndWebAssembly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte(`<!doctype html><html><body><script>
const wasmBytes = new Uint8Array([0,97,115,109,1,0,0,0,1,7,1,96,2,127,127,1,127,3,2,1,0,7,7,1,3,97,100,100,0,0,10,9,1,7,0,32,0,32,1,106,11]);
globalThis.platformState = {
  base64: [atob("AP+A").charCodeAt(0), atob("AP+A").charCodeAt(1), atob("AP+A").charCodeAt(2)].join(","),
  wasmValid: WebAssembly.validate(wasmBytes),
  uuid: crypto.randomUUID(),
  randomLength: crypto.getRandomValues(new Uint8Array(16)).length,
  wasmResult: 0,
  digestLength: 0,
  wasmRejected: false,
};
WebAssembly.instantiate(wasmBytes).then(result => { platformState.wasmResult = result.instance.exports.add(20, 22); });
WebAssembly.instantiate(new Uint8Array([0])).catch(() => { platformState.wasmRejected = true; });
crypto.subtle.digest("SHA-256", new TextEncoder().encode("abc")).then(result => { platformState.digestLength = result.byteLength; });
</script></body></html>`))
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(10 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("script failures: %+v", page.ScriptFailures)
	}
	value, err := browser.ExecuteJS(context.Background(), `JSON.stringify(platformState)`)
	if err != nil {
		t.Fatal(err)
	}
	state := value.String()
	for _, expected := range []string{`"base64":"0,255,128"`, `"wasmValid":true`, `"randomLength":16`, `"wasmResult":42`, `"digestLength":32`, `"wasmRejected":true`} {
		if !regexp.MustCompile(regexp.QuoteMeta(expected)).MatchString(state) {
			t.Fatalf("platform state %s missing %s", state, expected)
		}
	}
	if !regexp.MustCompile(`"uuid":"[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}"`).MatchString(state) {
		t.Fatalf("invalid randomUUID in %s", state)
	}
}

func TestMissingDOMQueriesReturnNull(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`<html><head></head><body data-present="yes"><script>document.body.setAttribute('data-ok', String(document.querySelector('#missing') === null && document.head.querySelector('#missing') === null && document.body.getAttributeNames().includes('data-present') && typeof ShadowRoot === 'function'))</script></body></html>`))
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(10 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 || !strings.Contains(page.RenderedHTML, `data-ok="true"`) {
		t.Fatalf("html=%s failures=%+v", page.RenderedHTML, page.ScriptFailures)
	}
}

func TestCustomRuntimeHooksAndCookieInspection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Add("Set-Cookie", "challenge=ready; Path=/")
		_, _ = writer.Write([]byte(`<script>globalThis.challengeResult = customDOM.value + 1;</script>`))
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(10 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	var cleaned atomic.Bool
	page, err := browser.Navigate(context.Background(), server.URL, nil, NavigateOptions{
		UseCustomRuntime: true,
		RuntimeInitializer: func(vm *goja.Runtime, _ *Page) error {
			return vm.Set("customDOM", map[string]int{"value": 41})
		},
		RuntimeFinalizer: func(vm *goja.Runtime, _ *Page) error {
			return vm.Set("challengeFinalized", true)
		},
		RuntimeCleanup: func() { cleaned.Store(true) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 || page.ExecutedScripts != 1 {
		t.Fatalf("scripts=%d failures=%+v", page.ExecutedScripts, page.ScriptFailures)
	}
	if !cleaned.Load() {
		t.Fatal("custom runtime cleanup did not run")
	}
	value, err := browser.ExecuteJS(context.Background(), `challengeResult === 42 && challengeFinalized === true`)
	if err != nil || !value.ToBoolean() {
		t.Fatalf("custom runtime state=%v err=%v", value, err)
	}
	cookies, err := browser.Cookies(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(cookies) != 1 || cookies[0].Name != "challenge" || cookies[0].Value != "ready" {
		t.Fatalf("cookies=%+v", cookies)
	}
	cookies[0].Value = "mutated"
	again, err := browser.Cookies(server.URL)
	if err != nil || len(again) != 1 || again[0].Value != "ready" {
		t.Fatalf("cookie copies were not isolated: cookies=%+v err=%v", again, err)
	}
}

func TestHostWindowAndDocumentExposeEventTargetMethods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte(`<!doctype html><html><body><script>
var eventCalls = [];
function windowListener() { eventCalls.push('window'); }
function documentListener() { eventCalls.push('document'); }
window.addEventListener('minib-window', windowListener);
document.addEventListener('minib-document', documentListener);
window.removeEventListener('minib-window', windowListener);
document.removeEventListener('minib-document', documentListener);
window.dispatchEvent(new Event('minib-window'));
document.dispatchEvent(new Event('minib-document'));
document.body.setAttribute('data-event-targets', [
  typeof window.addEventListener,
  typeof window.removeEventListener,
  typeof document.addEventListener,
  typeof document.removeEventListener,
  eventCalls.length
].join(':'));
</script></body></html>`))
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(10 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("script failures: %+v", page.ScriptFailures)
	}
	if !regexp.MustCompile(`data-event-targets="function:function:function:function:0"`).MatchString(page.RenderedHTML) {
		t.Fatalf("host EventTarget methods are unavailable: %s", page.RenderedHTML)
	}
}
