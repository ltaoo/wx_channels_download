package minib

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dop251/goja"

	"wx_channel/pkg/cookies"
)

func TestCallJavaScriptHonorsContext(t *testing.T) {
	vm := goja.New()
	value, err := vm.RunString(`(function () { while (true) {} })`)
	if err != nil {
		t.Fatal(err)
	}
	callback, ok := goja.AssertFunction(value)
	if !ok {
		t.Fatal("expected JavaScript function")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := call_javascript(ctx, vm, callback, goja.Undefined()); err == nil {
		t.Fatal("callback ignored context deadline")
	}
}

func TestJavaScriptEnginePanicsBecomeErrors(t *testing.T) {
	vm := goja.New()
	if err := vm.Set("panicHost", func() { panic("host panic") }); err != nil {
		t.Fatal(err)
	}
	if _, err := run_javascript(context.Background(), vm, "panic.js", `panicHost()`); err == nil || !strings.Contains(err.Error(), "JavaScript engine panic: host panic") {
		t.Fatalf("run_javascript panic error = %v", err)
	}
	callback, ok := goja.AssertFunction(vm.Get("panicHost"))
	if !ok {
		t.Fatal("panicHost is not callable")
	}
	if _, err := call_javascript(context.Background(), vm, callback, goja.Undefined()); err == nil || !strings.Contains(err.Error(), "JavaScript engine panic: host panic") {
		t.Fatalf("call_javascript panic error = %v", err)
	}
	if err := javascript_panic_error("bounded"); len(err.Error()) > 9<<10 {
		t.Fatalf("JavaScript panic diagnostic is unexpectedly large: %d bytes", len(err.Error()))
	}
}

func TestHostCallbackPerformsPromiseMicrotaskCheckpoint(t *testing.T) {
	vm := goja.New()
	if _, err := vm.RunString(`var checkpointValue = false; function scheduleCheckpoint() { Promise.resolve().then(function() { checkpointValue = true; }); }`); err != nil {
		t.Fatal(err)
	}
	callback, ok := goja.AssertFunction(vm.Get("scheduleCheckpoint"))
	if !ok {
		t.Fatal("scheduleCheckpoint is not callable")
	}
	if _, err := call_javascript(context.Background(), vm, callback, goja.Undefined()); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunString(`checkpointValue`)
	if err != nil {
		t.Fatal(err)
	}
	if !value.ToBoolean() {
		t.Fatal("Promise microtask did not run after host callback")
	}
}

func TestNavigateJavaScriptTimeoutAndDOMContentLoadedMilestone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, `<!doctype html><body>
<script>while (true) {}</script>
<script>
document.body.setAttribute('data-after-timeout', 'done');
document.addEventListener('DOMContentLoaded', function() { document.body.setAttribute('data-dom-content-loaded', document.readyState); });
window.addEventListener('load', function() { document.body.setAttribute('data-load', 'done'); });
</script>
</body>`)
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	started_at := time.Now()
	page, err := browser.Navigate(context.Background(), server.URL, nil, NavigateOptions{
		JavaScriptTimeout: 20 * time.Millisecond,
		WaitUntil:         WaitUntilDOMContentLoaded,
	})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started_at); elapsed > time.Second {
		t.Fatalf("navigation took %s despite the JavaScript timeout", elapsed)
	}
	if len(page.ScriptFailures) != 1 || !strings.Contains(page.ScriptFailures[0].Err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("script failures = %+v, want one deadline failure", page.ScriptFailures)
	}
	if page.ExecutedScripts != 1 {
		t.Fatalf("executed scripts = %d, want 1", page.ExecutedScripts)
	}
	if !strings.Contains(page.RenderedHTML, `data-after-timeout="done"`) ||
		!strings.Contains(page.RenderedHTML, `data-dom-content-loaded="interactive"`) {
		t.Fatalf("DOMContentLoaded state missing: %s", page.RenderedHTML)
	}
	if strings.Contains(page.RenderedHTML, `data-load="done"`) {
		t.Fatalf("load fired before the requested milestone: %s", page.RenderedHTML)
	}
}

func TestNavigateCanDisableJavaScriptForSSRExtraction(t *testing.T) {
	var script_requests int
	var asset_requests int
	var request_mutex sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/":
			response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(response_writer, `<!doctype html><head><link rel="modulepreload" href="/preload.js"><link rel="icon" href="/icon.png"></head><body data-ssr="ready"><img src="/image.png"><video src="/video.mp4" poster="/poster.png"></video><iframe src="/frame.html"></iframe><script src="/app.js"></script><script>document.body.setAttribute('data-inline', 'ran')</script></body>`)
		case "/app.js", "/preload.js":
			request_mutex.Lock()
			script_requests++
			request_mutex.Unlock()
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `document.body.setAttribute('data-external', 'ran')`)
		case "/icon.png", "/image.png", "/poster.png", "/video.mp4", "/frame.html":
			request_mutex.Lock()
			asset_requests++
			request_mutex.Unlock()
			_, _ = response_writer.Write([]byte("asset"))
		default:
			http.NotFound(response_writer, request)
		}
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL, nil, NavigateOptions{DisableSubresources: true, DisableJavaScript: true, DisableImages: true, DisableMedia: true})
	if err != nil {
		t.Fatal(err)
	}
	request_mutex.Lock()
	captured_script_requests := script_requests
	captured_asset_requests := asset_requests
	request_mutex.Unlock()
	if captured_script_requests != 0 || captured_asset_requests != 0 || page.ExecutedScripts != 0 || len(page.ScriptFailures) != 0 {
		t.Fatalf("disabled resources performed work: scripts=%d assets=%d executed=%d failures=%+v", captured_script_requests, captured_asset_requests, page.ExecutedScripts, page.ScriptFailures)
	}
	for _, resource := range page.Resources {
		if resource.Kind == ScriptResource {
			t.Fatalf("disabled JavaScript discovered script resource %s", resource.URL)
		}
	}
	if !strings.Contains(page.RenderedHTML, `data-ssr="ready"`) || strings.Contains(page.RenderedHTML, `data-inline="`) || strings.Contains(page.RenderedHTML, `data-external="`) {
		t.Fatalf("SSR DOM was unexpectedly modified: %s", page.RenderedHTML)
	}
	value, err := browser.ExecuteJS(context.Background(), `document.body.getAttribute('data-ssr')`)
	if err != nil || value.String() != "ready" {
		t.Fatalf("queryable SSR DOM value=%v error=%v", value, err)
	}
	inline_page, err := browser.Navigate(context.Background(), server.URL, nil, NavigateOptions{DisableSubresources: true})
	if err != nil {
		t.Fatal(err)
	}
	if inline_page.ExecutedScripts != 1 || !strings.Contains(inline_page.RenderedHTML, `data-inline="ran"`) || strings.Contains(inline_page.RenderedHTML, `data-external="`) || len(inline_page.Resources) != 0 {
		t.Fatalf("subresource-free inline execution mismatch: scripts=%d resources=%d html=%s", inline_page.ExecutedScripts, len(inline_page.Resources), inline_page.RenderedHTML)
	}
}

func TestNavigateRejectsInvalidExecutionOptions(t *testing.T) {
	browser, err := NewMiniBrowser(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	if _, err := browser.Navigate(context.Background(), "https://example.invalid", nil, NavigateOptions{JavaScriptTimeout: -time.Second}); err == nil {
		t.Fatal("negative JavaScript timeout was accepted")
	}
	if _, err := browser.Navigate(context.Background(), "https://example.invalid", nil, NavigateOptions{ResourceTimeout: -time.Second}); err == nil {
		t.Fatal("negative resource timeout was accepted")
	}
	if _, err := browser.Navigate(context.Background(), "https://example.invalid", nil, NavigateOptions{WaitUntil: "networkidle"}); err == nil {
		t.Fatal("unsupported lifecycle milestone was accepted")
	}
}

func TestNavigateResourceTimeoutDoesNotDiscardPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/":
			response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(response_writer, `<body><script src="/slow.js"></script><script src="/ok.js"></script></body>`)
		case "/slow.js":
			<-request.Context().Done()
		case "/ok.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `document.body.setAttribute('data-ok', 'done')`)
		default:
			http.NotFound(response_writer, request)
		}
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	started_at := time.Now()
	page, err := browser.Navigate(context.Background(), server.URL, nil, NavigateOptions{ResourceTimeout: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started_at); elapsed > time.Second {
		t.Fatalf("resource timeout took %s", elapsed)
	}
	if !strings.Contains(page.RenderedHTML, `data-ok="done"`) || page.ExecutedScripts != 1 {
		t.Fatalf("successful script did not execute: scripts=%d html=%s", page.ExecutedScripts, page.RenderedHTML)
	}
	if len(page.ScriptFailures) != 1 || !strings.Contains(page.ScriptFailures[0].Err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("script failures = %+v", page.ScriptFailures)
	}
}

func TestTimersRunByDeadlineAndPreserveIntervalCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, `<!doctype html><body><script>
var timerEvents = [];
setTimeout(function() { timerEvents.push('long'); }, 30000);
setTimeout(function() { timerEvents.push('zero'); }, 0);
var intervalCount = 0;
var intervalId = setInterval(function() { intervalCount++; timerEvents.push('interval-' + intervalCount); if (intervalCount === 2) clearInterval(intervalId); }, 1);
setTimeout(function() { document.body.setAttribute('data-timers', timerEvents.join(',')); }, 20);
</script></body>`)
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("unexpected script failures: %+v", page.ScriptFailures)
	}
	if !strings.Contains(page.RenderedHTML, `data-timers="zero,interval-1,interval-2"`) {
		t.Fatalf("timer scheduling mismatch: %s", page.RenderedHTML)
	}
}

func TestPromiseRejectionEventPreservesNativePromise(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, `<!doctype html><body><script>
var nativePromise = Promise;
var selectedPromise = (function(candidate) {
  if (candidate && (typeof PromiseRejectionEvent !== 'undefined' || !window.Promise || window.Promise.toString().indexOf('[native code]') === -1)) return candidate;
  return function TimerPromisePolyfill() {};
})(Promise);
var rejectionEvent = new PromiseRejectionEvent('unhandledrejection', { promise: Promise.resolve('ok'), reason: 'reason' });
Promise.resolve().then(function() {
  document.body.setAttribute('data-promise-capability', [selectedPromise === nativePromise, rejectionEvent instanceof Event, rejectionEvent.promise instanceof Promise, rejectionEvent.reason, Object.prototype.toString.call(rejectionEvent)].join(':'));
});
</script></body>`)
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("unexpected script failures: %+v", page.ScriptFailures)
	}
	if !strings.Contains(page.RenderedHTML, `data-promise-capability="true:true:true:reason:[object PromiseRejectionEvent]"`) {
		t.Fatalf("Promise capability probe failed: %s", page.RenderedHTML)
	}
}

func TestDOMEventTargetDispatchSemantics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, `<!doctype html><html><body><main id="parent"><button id="child"></button></main><script>
var parent = document.getElementById('parent');
var child = document.getElementById('child');
var order = [];
function listen(target, name, capture) {
  target.addEventListener('phase', function(event) {
    order.push(name + ':' + event.eventPhase + ':' + (event.currentTarget === target));
  }, capture);
}
listen(window, 'window-capture', true);
listen(document, 'document-capture', true);
listen(parent, 'parent-capture', true);
listen(child, 'child-capture', true);
listen(child, 'child-bubble', false);
listen(parent, 'parent-bubble', false);
listen(document, 'document-bubble', false);
listen(window, 'window-bubble', false);
child.dispatchEvent(new CustomEvent('phase', { bubbles: true, composed: true }));

var duplicateCount = 0;
function duplicateListener() { duplicateCount++; }
child.addEventListener('duplicate', duplicateListener);
child.addEventListener('duplicate', duplicateListener);
child.dispatchEvent(new Event('duplicate'));
child.removeEventListener('duplicate', duplicateListener);
child.dispatchEvent(new Event('duplicate'));

var onceCount = 0;
child.addEventListener('once', function() { onceCount++; }, { once: true });
child.dispatchEvent(new Event('once'));
child.dispatchEvent(new Event('once'));

var listenerObject = { count: 0, self: false, handleEvent: function() { this.count++; this.self = this === listenerObject; } };
child.addEventListener('object', listenerObject);
child.dispatchEvent(new Event('object'));

var passiveEvent = new Event('passive', { cancelable: true });
child.addEventListener('passive', function(event) { event.preventDefault(); }, { passive: true });
var passiveResult = child.dispatchEvent(passiveEvent);
var cancelEvent = new Event('cancel', { cancelable: true });
child.addEventListener('cancel', function(event) { event.preventDefault(); });
var cancelResult = child.dispatchEvent(cancelEvent);

var stopped = [];
parent.addEventListener('stopped', function(event) { stopped.push('first'); event.stopImmediatePropagation(); });
parent.addEventListener('stopped', function() { stopped.push('second'); });
window.addEventListener('stopped', function() { stopped.push('window'); });
child.dispatchEvent(new Event('stopped', { bubbles: true }));

document.body.setAttribute('data-event-target', JSON.stringify({
  order: order,
  duplicateCount: duplicateCount,
  onceCount: onceCount,
  objectCount: listenerObject.count,
  objectSelf: listenerObject.self,
  passiveResult: passiveResult,
  passiveDefault: passiveEvent.defaultPrevented,
  cancelResult: cancelResult,
  cancelDefault: cancelEvent.defaultPrevented,
  stopped: stopped
}));
</script></body></html>`)
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("unexpected script failures: %+v", page.ScriptFailures)
	}
	expected := `{&#34;order&#34;:[&#34;window-capture:1:true&#34;,&#34;document-capture:1:true&#34;,&#34;parent-capture:1:true&#34;,&#34;child-capture:2:true&#34;,&#34;child-bubble:2:true&#34;,&#34;parent-bubble:3:true&#34;,&#34;document-bubble:3:true&#34;,&#34;window-bubble:3:true&#34;],&#34;duplicateCount&#34;:1,&#34;onceCount&#34;:1,&#34;objectCount&#34;:1,&#34;objectSelf&#34;:true,&#34;passiveResult&#34;:true,&#34;passiveDefault&#34;:false,&#34;cancelResult&#34;:false,&#34;cancelDefault&#34;:true,&#34;stopped&#34;:[&#34;first&#34;]}`
	if !strings.Contains(page.RenderedHTML, `data-event-target="`+expected+`"`) {
		t.Fatalf("event semantics mismatch: %s", page.RenderedHTML)
	}
}

func TestKeyboardEventInitializationAndExecuteJSRefreshesHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, `<!doctype html><body><input id="task"></body>`)
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	value, err := browser.ExecuteJS(context.Background(), `(function() {
  var input = document.getElementById('task'), captured, intercepted = 0;
  var nativeAddEventListener = EventTarget.prototype.addEventListener;
  EventTarget.prototype.addEventListener = function() { intercepted++; return nativeAddEventListener.apply(this, arguments); };
  input.addEventListener('keydown', function(event) {
    captured = [event.key, event.code, event.keyCode, event.which, event.ctrlKey, event.repeat, event.bubbles, event.cancelable].join(':');
  });
  input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', code: 'Enter', ctrlKey: true, repeat: true, bubbles: true, cancelable: true }));
  captured += ':' + intercepted + ':' + ('oninput' in document);
  document.body.setAttribute('data-keyboard', captured);
  setTimeout(function() { document.body.setAttribute('data-execute-timer', 'done'); }, 0);
  return captured;
})()`)
	if err != nil {
		t.Fatal(err)
	}
	expected := "Enter:Enter:13:13:true:true:true:true:1:true"
	if value.String() != expected {
		t.Fatalf("KeyboardEvent state = %q, want %q", value.String(), expected)
	}
	if !strings.Contains(page.RenderedHTML, `data-keyboard="`+expected+`"`) {
		t.Fatalf("ExecuteJS did not refresh rendered HTML: %s", page.RenderedHTML)
	}
	if !strings.Contains(page.RenderedHTML, `data-execute-timer="done"`) {
		t.Fatalf("ExecuteJS did not pump the page event loop: %s", page.RenderedHTML)
	}
}

func TestMessageChannelRunsPostedMessageTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, `<!doctype html><body><script>
var channel = new MessageChannel();
var deliveries = [];
channel.port1.addEventListener('message', function(event) { deliveries.push('listener:' + (event instanceof MessageEvent) + ':' + event.data); });
channel.port1.onmessage = function(event) { deliveries.push('handler:' + (event instanceof MessageEvent) + ':' + event.data); document.body.setAttribute('data-message', deliveries.join(',')); };
channel.port2.postMessage('ready');
</script></body>`)
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("unexpected script failures: %+v", page.ScriptFailures)
	}
	if !strings.Contains(page.RenderedHTML, `data-message="listener:true:ready,handler:true:ready"`) {
		value, runtime_err := browser.ExecuteJS(context.Background(), `JSON.stringify({bridge:typeof __minib_post_message_port,handler:typeof channel.port1.onmessage,target:channel.port2._target===channel.port1})`)
		t.Fatalf("posted message was not delivered: html=%s runtime=%v err=%v", page.RenderedHTML, value, runtime_err)
	}
}

func TestXMLHttpRequestUsesEventTargetPrototypeChain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, `<!doctype html><body><script>
var xhr = new XMLHttpRequest();
var zoneSymbol = '__zone_symbol__addEventListener';
EventTarget.prototype[zoneSymbol] = EventTarget.prototype.addEventListener;
document.body.setAttribute('data-xhr-prototype', [
  xhr instanceof EventTarget,
  Object.getPrototypeOf(XMLHttpRequest.prototype) === EventTarget.prototype,
  typeof xhr.addEventListener,
  typeof xhr[zoneSymbol]
].join(':'));
</script></body>`)
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("unexpected script failures: %+v", page.ScriptFailures)
	}
	if !strings.Contains(page.RenderedHTML, `data-xhr-prototype="true:true:function:function"`) {
		t.Fatalf("XMLHttpRequest prototype chain mismatch: %s", page.RenderedHTML)
	}
}

func TestModuleScriptsUseIndependentStrictScopes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, `<!doctype html><body>
<script type="module">var moduleCollision = 'first'; window.firstModule = moduleCollision; window.firstModuleThis = this; export {};</script>
<script type="module">var moduleCollision = 'second'; window.secondModule = moduleCollision; window.secondModuleThis = this; export {};</script>
<script>var classicBinding = 'classic';</script>
</body>`)
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("unexpected script failures: %+v", page.ScriptFailures)
	}
	value, err := browser.ExecuteJS(context.Background(), `JSON.stringify({first:firstModule,second:secondModule,firstThis:firstModuleThis===undefined,secondThis:secondModuleThis===undefined,moduleGlobal:typeof moduleCollision,classicGlobal:window.classicBinding})`)
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"first":"first","second":"second","firstThis":true,"secondThis":true,"moduleGlobal":"undefined","classicGlobal":"classic"}`
	if value.String() != expected {
		t.Fatalf("module scope state = %s, want %s", value.String(), expected)
	}
}

func TestCustomElementConnectionAndDisconnectionLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, `<!doctype html><body><div id="first"></div><div id="second"></div><script>
var lifecycleEvents = [];
class LifecycleElement extends HTMLElement {
  connectedCallback() { lifecycleEvents.push('connected:' + this.isConnected); }
  disconnectedCallback() { lifecycleEvents.push('disconnected:' + this.isConnected); }
}
customElements.define('lifecycle-element', LifecycleElement);
var lifecycleElement = document.createElement('lifecycle-element');
var first = document.getElementById('first'), second = document.getElementById('second');
first.appendChild(lifecycleElement);
first.removeChild(lifecycleElement);
first.appendChild(lifecycleElement);
second.appendChild(lifecycleElement);
second.textContent = '';
document.body.setAttribute('data-lifecycle', lifecycleEvents.join(','));
</script></body>`)
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("unexpected script failures: %+v", page.ScriptFailures)
	}
	expected := `connected:true,disconnected:false,connected:true,disconnected:false,connected:true,disconnected:false`
	if !strings.Contains(page.RenderedHTML, `data-lifecycle="`+expected+`"`) {
		t.Fatalf("custom-element lifecycle mismatch: %s", page.RenderedHTML)
	}
}

func TestDOMStandardConvenienceMixins(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, `<!doctype html><body><script>
var mixinHost = document.createElement('div');
document.body.append(mixinHost);
mixinHost.append('middle');
mixinHost.prepend('start-');
var marker = document.createElement('span');
mixinHost.append(marker);
marker.before('-before-');
marker.after('-after-');
marker.replaceWith('-replacement-');
mixinHost.insertAdjacentHTML('beforeend', '<b>html</b>');
mixinHost.insertAdjacentText('beforeend', '-text');
var beforeReplace = mixinHost.innerHTML;
var toggleResults = [mixinHost.toggleAttribute('data-active'), mixinHost.toggleAttribute('data-active', false), mixinHost.toggleAttribute('data-active', true)];
var firstText = document.createTextNode('abcdef');
mixinHost.replaceChildren(firstText);
var secondText = firstText.splitText(3);
firstText.appendData('!');
secondText.insertData(0, '?');
secondText.deleteData(1, 1);
secondText.replaceData(1, 2, 'XY');
var wholeText = secondText.wholeText;
mixinHost.normalize();
var removable = document.createElement('i');
mixinHost.append(removable);
removable.remove();
window.mixinState = {
  before: beforeReplace,
  toggles: toggleResults,
  text: mixinHost.textContent,
  wholeText: wholeText,
  childNodes: mixinHost.childNodes.length,
  root: mixinHost.getRootNode() === document,
  same: mixinHost.isSameNode(mixinHost),
  hasChildren: mixinHost.hasChildNodes(),
  selectorGroup: document.querySelectorAll('body,div').length,
  matchesGroup: mixinHost.matches('span,div'),
  closestGroup: mixinHost.closest('html,body') === document.body,
  positionConstant: Node.DOCUMENT_POSITION_CONTAINED_BY
};
</script></body>`)
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("unexpected script failures: %+v", page.ScriptFailures)
	}
	value, err := browser.ExecuteJS(context.Background(), `JSON.stringify(window.mixinState)`)
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"before":"start-middle-before--replacement--after-<b>html</b>-text","toggles":[true,false,true],"text":"abc!?XY","wholeText":"abc!?XY","childNodes":1,"root":true,"same":true,"hasChildren":true,"selectorGroup":2,"matchesGroup":true,"closestGroup":true,"positionConstant":16}`
	if value.String() != expected {
		t.Fatalf("DOM mixin state = %s, want %s", value.String(), expected)
	}
}

func TestCSSOMBuildsCascadedComputedStyleTree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/linked.css" {
			response_writer.Header().Set("Content-Type", "text/css; charset=utf-8")
			_, _ = fmt.Fprint(response_writer, `#target { color: purple; border-left-width: var(--local); background-color: var(--tone, black); }`)
			return
		}
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, `<!doctype html><html><head>
<style id="author-style">
:root { --tone: teal; color: navy; font-size: 18px; }
#target { color: red; display: flex; margin-left: 1px; }
.box { color: green !important; }
main .box { color: blue; padding: 4px; }
@media screen and (min-width: 1000px) { #target { width: 50px; } }
</style>
<link id="linked-style" rel="stylesheet" href="/linked.css" media="screen">
</head><body><main><div id="target" class="box" style="padding: 8px; --local: 2px; margin-left: 9px"><span id="child"></span></div></main>
<script>
var target = document.getElementById('target');
var child = document.getElementById('child');
var before = getComputedStyle(target);
var inherited = getComputedStyle(child);
var authorSheet = document.getElementById('author-style').sheet;
var linkedSheet = document.getElementById('linked-style').sheet;
var ruleCountBefore = authorSheet.cssRules.length;
authorSheet.insertRule('#target { height: 77px !important; }', authorSheet.cssRules.length);
target.style.marginTop = '3px';
target.style.cssFloat = 'left';
var removed = target.style.removeProperty('margin-left');
var after = getComputedStyle(target);
var constructed = new CSSStyleSheet();
constructed.replaceSync('.constructed { display: grid; }');
constructed.insertRule('.second { opacity: .5; }', 1);
document.body.setAttribute('data-cssom', JSON.stringify({
  sheetCount: document.styleSheets.length,
  sheetItem: document.styleSheets.item(0) === authorSheet,
  sheetTypes: authorSheet instanceof CSSStyleSheet && linkedSheet instanceof CSSStyleSheet,
  ruleType: authorSheet.cssRules[0] instanceof CSSStyleRule && authorSheet.cssRules[0].type === CSSRule.STYLE_RULE,
  ruleCountBefore: ruleCountBefore,
  ruleCountAfter: authorSheet.cssRules.length,
  color: before.color,
  display: before.display,
  padding: before.padding,
  width: before.width,
  inheritedColor: inherited.color,
  inheritedFontSize: inherited.fontSize,
  customFallback: before.backgroundColor,
  customLocal: before.borderLeftWidth,
  height: after.height,
  marginTop: after.marginTop,
  cssFloat: after.cssFloat,
  removed: removed,
  reflectedStyle: target.getAttribute('style'),
  declarationType: target.style instanceof CSSStyleDeclaration && after instanceof CSSStyleDeclaration,
  constructedRules: constructed.cssRules.length,
  constructedSelector: constructed.cssRules[0].selectorText
}));
</script></body></html>`)
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("unexpected script failures: %+v", page.ScriptFailures)
	}
	value, err := browser.ExecuteJS(context.Background(), `document.body.getAttribute('data-cssom')`)
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"sheetCount":2,"sheetItem":true,"sheetTypes":true,"ruleType":true,"ruleCountBefore":5,"ruleCountAfter":6,"color":"green","display":"flex","padding":"8px","width":"50px","inheritedColor":"green","inheritedFontSize":"18px","customFallback":"teal","customLocal":"2px","height":"77px","marginTop":"3px","cssFloat":"left","removed":"9px","reflectedStyle":"padding: 8px; --local: 2px; margin-top: 3px; float: left;","declarationType":true,"constructedRules":2,"constructedSelector":".constructed"}`
	if value.String() != expected {
		t.Fatalf("CSSOM state = %s, want %s", value.String(), expected)
	}
}

func TestNavigateCanDisableCSSForDataExtraction(t *testing.T) {
	request_counts := make(map[string]int)
	var request_mutex sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		request_mutex.Lock()
		request_counts[request.URL.Path]++
		request_mutex.Unlock()
		switch request.URL.Path {
		case "/":
			response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(response_writer, `<!doctype html><html><head>
<style>#target { color: blue; display: grid; }</style>
<link id="linked-style" rel="stylesheet" href="/linked.css">
<link rel="preload" as="font" href="/font.woff2">
</head><body><div id="target" style="color: red; --tone: green"></div><script>
var target = document.getElementById('target');
var computed = getComputedStyle(target);
var dynamicStyle = document.createElement('link');
dynamicStyle.rel = 'stylesheet';
dynamicStyle.href = '/dynamic.css';
dynamicStyle.onload = function() { document.body.setAttribute('data-dynamic-css-load', 'done'); };
document.head.appendChild(dynamicStyle);
document.body.setAttribute('data-css-disabled', [
  document.styleSheets.length,
  document.getElementById('linked-style').sheet === null,
  computed.color,
  computed.display,
  computed.getPropertyValue('--tone'),
  target.style.color
].join(':'));
</script></body></html>`)
		case "/linked.css", "/dynamic.css":
			response_writer.Header().Set("Content-Type", "text/css")
			_, _ = fmt.Fprint(response_writer, `#target { color: purple; }`)
		case "/font.woff2":
			response_writer.Header().Set("Content-Type", "font/woff2")
			_, _ = response_writer.Write([]byte("font"))
		default:
			http.NotFound(response_writer, request)
		}
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL, nil, NavigateOptions{DisableCSS: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("unexpected script failures: %+v", page.ScriptFailures)
	}
	value, err := browser.ExecuteJS(context.Background(), `document.body.getAttribute('data-css-disabled') + ':' + document.body.getAttribute('data-dynamic-css-load')`)
	if err != nil {
		t.Fatal(err)
	}
	if value.String() != "0:true:red:block:green:red:done" {
		t.Fatalf("disabled CSS state = %s", value.String())
	}
	for _, resource := range page.Resources {
		if resource.Kind == StyleResource || resource.Kind == FontResource {
			t.Fatalf("disabled CSS navigation retained resource %+v", resource)
		}
	}
	request_mutex.Lock()
	defer request_mutex.Unlock()
	for _, resource_path := range []string{"/linked.css", "/dynamic.css", "/font.woff2"} {
		if request_counts[resource_path] != 0 {
			t.Fatalf("disabled CSS requested %s %d times", resource_path, request_counts[resource_path])
		}
	}
}

func TestNavigateBuildsDOMDownloadsResourcesAndRunsScripts(t *testing.T) {
	requested := make(map[string]int)
	received_cookies := make(map[string]string)
	var requested_mutex sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		requested_mutex.Lock()
		requested[request.URL.Path]++
		received_cookies[request.URL.Path] = request.Header.Get("Cookie")
		requested_mutex.Unlock()
		switch request.URL.Path {
		case "/":
			response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			response_writer.Header().Set("Set-Cookie", "session=server; Path=/")
			_, _ = fmt.Fprint(response_writer, `<!doctype html><html><head>
<link rel="stylesheet" href="/style.css">
<script>document.querySelector('script[src="/app.js"]').onload = function() { document.documentElement.setAttribute('data-initial-script-load', 'done'); };</script>
<script src="/app.js"></script>
<script defer src="/defer.js"></script>
</head><body><div id="status">initial</div><img src="/pixel.png">
<script>
window.order.push('inline');
document.write('<scr' + 'ipt src="/written.js"></scr' + 'ipt>');
window.body_script_done = 'yes';
document.body.setAttribute('data-apply-null', (function() { return arguments.length; }).apply(null, null));
setTimeout(function() { document.body.setAttribute('data-single-argument-timer', 'done'); });
document.querySelector('#status').textContent += '-inline';
var script = document.createElement('script');
script.src = '/dynamic.js';
document.head.appendChild(script);
var dynamicStyle = document.createElement('link');
dynamicStyle.rel = 'stylesheet';
dynamicStyle.href = '/dynamic.css';
dynamicStyle.onload = function() { document.body.setAttribute('data-dynamic-style', 'loaded'); };
document.head.appendChild(dynamicStyle);
var dynamicImage = document.createElement('img');
dynamicImage.src = '/dynamic.png';
dynamicImage.onload = function() { document.body.setAttribute('data-dynamic-image', 'loaded'); };
document.body.appendChild(dynamicImage);
var fragment = document.createDocumentFragment();
var badge = document.createElement('span');
badge.textContent = 'fragment';
fragment.appendChild(badge);
document.body.appendChild(fragment);
var anchor = document.createElement('a');
anchor.href = '/jobs?q=go#details';
document.body.setAttribute('data-anchor', anchor.pathname + anchor.search + anchor.hash);
var probe = document.createElement('div');
probe.setAttribute('onsubmit', 'test');
document.body.setAttribute('data-attribute', probe.attributes.onsubmit.value);
document.body.setAttribute('data-has-attributes', probe.hasAttributes() + ':' + document.createElement('div').hasAttributes());
var classProbe = document.createElement('div'); classProbe.setAttribute('data-n', 'class-list-probe'); classProbe.className = 'one two'; document.body.setAttribute('data-class-list', Array.from(classProbe.classList).join(','));
var adjacent = document.createElement('em');
adjacent.id = 'namedElement';
adjacent.setAttribute('data-adjacent', 'done');
document.getElementById('status').insertAdjacentElement('afterend', adjacent);
var detached = document.implementation.createHTMLDocument('detached');
var treeRoot = document.createElement('div'); treeRoot.appendChild(document.createTextNode('skip')); var treeChild = document.createElement('span'); treeRoot.appendChild(treeChild);
var treeWalker = document.createTreeWalker(treeRoot, NodeFilter.SHOW_ELEMENT, null); var customEvent = document.createEvent('CustomEvent'); customEvent.initCustomEvent('ready', true, false, 'detail');
document.body.setAttribute('data-event-prototype', Object.getOwnPropertyDescriptor(Event.prototype, 'type').get.call(customEvent));
var nativeEvent = Event; var nativeCustomEvent = CustomEvent; function wrapEvent(constructor) { function WrappedEvent(type, init) { var event = new constructor(type, init); event.__composed = !!(init && init.composed); return event; } WrappedEvent.prototype = constructor.prototype; return WrappedEvent; } Event = wrapEvent(Event); CustomEvent = wrapEvent(CustomEvent); document.body.setAttribute('data-wrapped-event', new CustomEvent('wrapped', { composed: true }).type); Event = nativeEvent; CustomEvent = nativeCustomEvent;
try { document.body.dispatchEvent({}); } catch (error) { document.body.setAttribute('data-invalid-event', error instanceof TypeError); }
var eventParent = document.createElement('div'); eventParent.setAttribute('data-n', 'event-parent'); var eventChild = document.createElement('button'); eventChild.setAttribute('data-n', 'event-child'); eventParent.appendChild(eventChild); document.body.appendChild(eventParent); eventParent.addEventListener('minib-event', function(event) { var path = event.composedPath(); document.body.setAttribute('data-event-dispatch', [event.detail.value, event.target === eventChild, event.currentTarget === eventParent, path[0] === eventChild, path[1] === eventParent].join(':')); event.preventDefault(); }); document.body.setAttribute('data-event-result', eventChild.dispatchEvent(new CustomEvent('minib-event', { detail: { value: 'ready' }, bubbles: true, cancelable: true })));
var svgNode = document.createElementNS('http://www.w3.org/2000/svg', 'svg'); svgNode.setAttributeNS(null, 'viewBox', '0 0 1 1');
var cyclicArray = []; cyclicArray.push(cyclicArray);
var templateNode = document.createElement('template'); templateNode.innerHTML = '<strong data-n="template-clone">template</strong>'; document.body.appendChild(document.importNode(templateNode.content, true));
var nestedTemplate = document.createElement('template'); nestedTemplate.setAttribute('data-n', 'nested-template'); nestedTemplate.innerHTML = '<b data-n="nested-template-content">nested</b>'; document.body.setAttribute('data-fragment-query', nestedTemplate.content.querySelector('b').textContent + ':' + nestedTemplate.content.querySelectorAll('b').length); document.body.appendChild(nestedTemplate.cloneNode(true).content);
var fragmentInsertTarget = document.createElement('div'); fragmentInsertTarget.setAttribute('data-n', 'fragment-insert-target'); var fragmentInsertMark = document.createElement('span'); fragmentInsertMark.setAttribute('data-n', 'fragment-insert-mark'); fragmentInsertTarget.appendChild(fragmentInsertMark); var fragmentInsert = document.createDocumentFragment(); fragmentInsert.appendChild(document.createElement('em')).setAttribute('data-n', 'fragment-insert-child'); fragmentInsertTarget.insertBefore(fragmentInsert, fragmentInsertMark); var fragmentReplace = document.createDocumentFragment(); fragmentReplace.appendChild(document.createElement('b')).setAttribute('data-n', 'fragment-replace-child'); fragmentInsertTarget.replaceChild(fragmentReplace, fragmentInsertMark); document.body.appendChild(fragmentInsertTarget);
document.body.setAttribute('data-wrapped-import', document.importNode({ node: document.body }, false).tagName);
var range = document.createRange(); range.selectNode(document.body); document.body.appendChild(range.createContextualFragment('<i data-n="range-fragment">range</i>'));
var messageChannel = new MessageChannel(); messageChannel.port1.onmessage = function(event) { document.body.setAttribute('data-message-channel', event.data); }; messageChannel.port2.postMessage('ready');
document.body.setAttribute('data-youtube-apis', [treeWalker.nextNode() === treeChild, Intl.DateTimeFormat.supportedLocalesOf(['en-US'])[0], Intl.NumberFormat.supportedLocalesOf(['en-US'])[0], new MouseEvent('click').type, new KeyboardEvent('keydown', { key: 'Enter' }).key, customEvent.detail, svgNode.getAttributeNS(null, 'viewBox'), window instanceof Window, MediaSource.isTypeSupported('video/mp4'), typeof URL.createObjectURL === 'function', templateNode instanceof HTMLTemplateElement, String(cyclicArray) === '', getComputedStyle(document.documentElement).fontSize, performance.timing.responseStart > 0].join(':'));
var existingCustom = document.createElement('minib-existing'); existingCustom.setAttribute('data-n', 'pre-upgrade-custom-element'); document.body.appendChild(existingCustom);
var inertConstructionCount = 0; var inertRoot = document.createElement('div'); inertRoot.setAttribute('data-n', 'inert-custom-element-root'); var inertCustom = document.createElement('minib-inert'); inertCustom.setAttribute('data-n', 'inert-custom-element'); inertRoot.appendChild(inertCustom); class MinibInert extends HTMLElement { constructor() { super(); inertConstructionCount++; } } customElements.define('minib-inert', MinibInert); var inertBefore = inertConstructionCount; document.body.appendChild(inertRoot); document.body.setAttribute('data-template-inert', inertBefore + ':' + inertConstructionCount);
var customElementInstances = [];
class MinibExisting extends HTMLElement { static get observedAttributes() { return ['data-state']; } constructor() { super(); customElementInstances.push(this); this.textContent = 'upgraded'; } connectedCallback() { this.setAttribute('data-connected', 'yes'); } attributeChangedCallback(name, oldValue, newValue) { this.setAttribute('data-observed', String(oldValue) + '>' + String(newValue)); } }
customElements.define('minib-existing', MinibExisting);
var createdCustom = document.createElement('minib-existing'); createdCustom.setAttribute('data-n', 'constructed-custom-element'); document.body.appendChild(createdCustom);
createdCustom.setAttribute('data-state', 'on'); createdCustom.removeAttribute('data-state');
var disabledCustom = document.createElement('minib-disabled'); disabledCustom.setAttribute('data-n', 'disabled-custom-element'); disabledCustom.setAttribute('disable-upgrade', ''); document.body.appendChild(disabledCustom);
class MinibDisabled extends HTMLElement { static get observedAttributes() { return ['disable-upgrade']; } constructor() { super(); disabledCustom.removeAttribute('disable-upgrade'); } attributeChangedCallback(name, oldValue, newValue) { if (newValue === null && this.isConnected) this.setAttribute('data-upgraded', 'yes'); } }
customElements.define('minib-disabled', MinibDisabled);
var customConnectOrder = [];
class MinibNestedChild extends HTMLElement { connectedCallback() { customConnectOrder.push('child'); this.dispatchEvent(new CustomEvent('minib-nested-ready', { bubbles: true })); } }
customElements.define('minib-nested-child', MinibNestedChild);
var nestedParent = document.createElement('minib-nested-parent'); document.body.appendChild(nestedParent);
class MinibNestedParent extends HTMLElement { constructor() { super(); this.appendChild(document.createElement('minib-nested-child')); } connectedCallback() { var self = this; customConnectOrder.push('parent'); this.addEventListener('minib-nested-ready', function() { self.setAttribute('data-connect-order', customConnectOrder.join(',')); }); } }
customElements.define('minib-nested-parent', MinibNestedParent);
customElements.whenDefined('minib-existing').then(function() { document.body.setAttribute('data-custom-elements', [existingCustom instanceof MinibExisting, existingCustom.textContent, existingCustom.getAttribute('data-connected'), createdCustom instanceof MinibExisting, createdCustom.getAttribute('data-connected'), createdCustom.getAttribute('data-observed'), customElements.get('minib-existing') === MinibExisting, customElementInstances[0] === existingCustom, customElementInstances[1] === createdCustom].join(':')); });
var canvas = document.createElement('canvas');
canvas.getContext('2d').fillRect(0, 0, 1, 1);
document.body.setAttribute('data-browser-api', detached.title + ':' + typeof new Date().toGMTString);
document.body.setAttribute('data-canvas-api', (canvas.getContext('webgl') === null) + ':' + (document.location === location));
var iframe = document.createElement('iframe');
document.body.appendChild(iframe);
document.body.setAttribute('data-native-dom', [new TextDecoder().decode(new Uint8Array([111, 107])), new TextDecoder().decode() === '', document instanceof HTMLDocument, document.body instanceof HTMLBodyElement, document.documentElement instanceof HTMLHtmlElement, new Image() instanceof HTMLImageElement, iframe instanceof HTMLIFrameElement, new Audio() instanceof HTMLAudioElement, new Audio().canPlayType('audio/mpeg'), document.createElement('video') instanceof HTMLVideoElement, document.createElement('video').canPlayType('video/mp4'), document.createTextNode('x') instanceof CharacterData, location instanceof Location, iframe.contentWindow === window, document.createElement('div') instanceof HTMLDivElement, document.body.isConnected, document.createElement('div').isConnected, typeof Node.prototype.appendChild === 'function', Element.prototype.getAttribute.call(probe, 'onsubmit') === 'test', Object.getOwnPropertyDescriptor(Node.prototype, 'firstChild').get.call(document.body) === document.body.firstChild, Node.prototype.constructor === Node, Element.prototype.constructor === Element, HTMLElement.prototype.constructor === HTMLElement, CustomEvent.prototype.constructor === CustomEvent].join(':'));
var bodyRect = document.body.getBoundingClientRect();
document.body.setAttribute('data-layout', bodyRect.right + ':' + bodyRect.bottom + ':' + document.createElement('div').getBoundingClientRect().right + ':' + document.body.getClientRects().length + ':' + document.createElement('div').getClientRects().length);
new IntersectionObserver(function(entries) { document.body.setAttribute('data-intersection', entries[0].isIntersecting + ':' + entries[0].target.tagName); }).observe(document.body);
var mutationOrder = ['sync'];
var mutationText = document.createTextNode('0');
new MutationObserver(function() { mutationOrder.push('observer'); document.body.setAttribute('data-mutation', mutationOrder.join(',')); }).observe(mutationText, { characterData: true });
mutationText.data = '1';
mutationOrder.push('after');
var xhr = new XMLHttpRequest();
xhr.open('GET', '/api');
xhr.onreadystatechange = function() { if (xhr.readyState === 4) document.body.setAttribute('data-xhr', xhr.status + ':' + xhr.responseText); };
xhr.send();
Promise.resolve().then(function() {
  var promiseXhr = new XMLHttpRequest();
  promiseXhr.open('GET', '/promise-api');
  promiseXhr.onreadystatechange = function() { if (promiseXhr.readyState === 4) document.body.setAttribute('data-promise-xhr', promiseXhr.status); };
  promiseXhr.send();
});
new Promise(function(resolve, reject) {
  var axiosXhr = new XMLHttpRequest();
  function finish() { resolve({ data: axiosXhr.responseText, status: axiosXhr.status }); axiosXhr = null; }
  axiosXhr.open('GET', '/axios-api', true);
  if ('onloadend' in axiosXhr) axiosXhr.onloadend = finish;
  else axiosXhr.onreadystatechange = function() { if (axiosXhr && axiosXhr.readyState === 4) setTimeout(finish); };
  axiosXhr.send(null);
}).then(function(response) { document.body.setAttribute('data-axios-xhr', response.status + ':' + response.data); });
async function asyncAxiosRequest() {
  return await new Promise(function(resolve) {
    var asyncXhr = new XMLHttpRequest();
    asyncXhr.open('GET', '/async-axios-api', true);
    asyncXhr.onloadend = function() { resolve(asyncXhr.status); };
    asyncXhr.send(null);
  });
}

(async function() {
  try { await asyncAxiosRequest(); }
  finally { document.body.setAttribute('data-async-finally', 'mounted'); }
})();
(async function() {
  var delayed = await new Promise(function(resolve, reject) {
    var delayedXhr = new XMLHttpRequest();
    delayedXhr.open('GET', '/delayed-api', true);
    delayedXhr.responseType = 'json';
    delayedXhr.onreadystatechange = function() { if (delayedXhr.readyState === 4) setTimeout(function() { resolve(delayedXhr.response); }); };
    delayedXhr.onerror = reject;
    delayedXhr.send(null);
  });
  document.body.setAttribute('data-delayed-xhr', delayed.value);
})();
Promise.resolve(new Promise(function(resolve, reject) {
  var kvXhr = new XMLHttpRequest();
  function finishKv() {
    if (!kvXhr) return;
    var responseType = kvXhr.responseType;
    resolve({ data: responseType && responseType !== 'text' ? kvXhr.response : kvXhr.responseText, status: kvXhr.status });
    kvXhr = null;
  }
  kvXhr.open('GET', '/kv-api', true);
  kvXhr.responseType = 'json';
  kvXhr.onreadystatechange = function() { if (kvXhr && kvXhr.readyState === 4 && kvXhr.status !== 0) setTimeout(finishKv); };
  kvXhr.onerror = reject;
  kvXhr.send(null);
})).then(function(response) {
  if (response.data.code === 0) return response.data.data;
  throw new Error(response.data.message);
}).then(function(data) { localStorage.setItem('kv', JSON.stringify(data)); document.body.setAttribute('data-kv-xhr', data.value); });
fetch('/fetch-api?source=fetch', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{"fetch":true}' })
  .then(function(response) { return response.json(); })
  .then(function(response) { document.body.setAttribute('data-fetch', response.fetch); });
</script><script type="module">if (false) import('/unused.js'); document.body.setAttribute('data-module', 'done'); document.body.setAttribute('data-named-global', namedElement.getAttribute('data-adjacent')); export {};</script></body></html>`)
		case "/style.css":
			response_writer.Header().Set("Content-Type", "text/css")
			_, _ = fmt.Fprint(response_writer, "body { color: green; }")
		case "/dynamic.css":
			response_writer.Header().Set("Content-Type", "text/css")
			_, _ = fmt.Fprint(response_writer, "body { background: white; }")
		case "/app.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `window.order = ['external']; document.getElementById('status').textContent = 'external';`)
		case "/dynamic.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `window.order.push('dynamic'); document.body.setAttribute('data-dynamic', 'done');`)
		case "/written.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `window.order.push('written'); document.body.setAttribute('data-written', 'done');`)
		case "/defer.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `window.order.push('defer'); document.body.setAttribute('data-defer-saw-body', window.body_script_done);`)
		case "/api":
			response_writer.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(response_writer, `{"ok":true}`)
		case "/promise-api":
			response_writer.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(response_writer, `{"promise":true}`)
		case "/axios-api":
			response_writer.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(response_writer, `{"axios":true}`)
		case "/async-axios-api":
			response_writer.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(response_writer, `{"async":true}`)
		case "/delayed-api":
			response_writer.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(response_writer, `{"value":"done"}`)
		case "/kv-api":
			response_writer.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(response_writer, `{"code":0,"message":"","data":{"value":"ready"}}`)
		case "/fetch-api":
			response_writer.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(response_writer, `{"fetch":true}`)
		case "/pixel.png":
			response_writer.Header().Set("Content-Type", "image/png")
			_, _ = response_writer.Write([]byte("png"))
		case "/dynamic.png":
			response_writer.Header().Set("Content-Type", "image/png")
			_, _ = response_writer.Write([]byte("dynamic-png"))
		default:
			http.NotFound(response_writer, request)
		}
	}))
	defer server.Close()

	cookie_dir := t.TempDir()
	if err := cookies.SaveJSON([]cookies.Cookie{
		{Name: "session", Value: "persistent", Domain: "127.0.0.1", Path: "/", Expires: -1},
		{Name: "provider_only", Value: "yes", Domain: "127.0.0.1", Path: "/", Expires: -1},
	}, filepath.Join(cookie_dir, "cookies.json")); err != nil {
		t.Fatal(err)
	}
	browser, err := NewMiniBrowser(10*time.Second, cookies.NewPersistentReader(cookie_dir))
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("unexpected script failures: %+v", page.ScriptFailures)
	}
	if page.ExecutedScripts != 7 {
		t.Fatalf("executed %d scripts, want 7; failures=%+v", page.ExecutedScripts, page.ScriptFailures)
	}
	if !strings.Contains(page.RenderedHTML, `external-inline`) || !strings.Contains(page.RenderedHTML, `data-initial-script-load="done"`) || !strings.Contains(page.RenderedHTML, `data-written="done"`) || !strings.Contains(page.RenderedHTML, `data-apply-null="0"`) || !strings.Contains(page.RenderedHTML, `data-single-argument-timer="done"`) || !strings.Contains(page.RenderedHTML, `data-dynamic="done"`) || !strings.Contains(page.RenderedHTML, `data-dynamic-style="loaded"`) || !strings.Contains(page.RenderedHTML, `data-dynamic-image="loaded"`) || !strings.Contains(page.RenderedHTML, `data-defer-saw-body="yes"`) || !strings.Contains(page.RenderedHTML, `data-anchor="/jobs?q=go#details"`) || !strings.Contains(page.RenderedHTML, `data-attribute="test"`) || !strings.Contains(page.RenderedHTML, `data-has-attributes="true:false"`) || !strings.Contains(page.RenderedHTML, `data-class-list="one,two"`) || !strings.Contains(page.RenderedHTML, `data-event-prototype="ready"`) || !strings.Contains(page.RenderedHTML, `data-wrapped-event="wrapped"`) || !strings.Contains(page.RenderedHTML, `data-invalid-event="true"`) || !strings.Contains(page.RenderedHTML, `data-adjacent="done"`) || !strings.Contains(page.RenderedHTML, `data-module="done"`) || !strings.Contains(page.RenderedHTML, `data-named-global="done"`) || !strings.Contains(page.RenderedHTML, `data-browser-api="detached:function"`) || !strings.Contains(page.RenderedHTML, `data-youtube-apis="true:en-US:en-US:click:Enter:detail:0 0 1 1:true:false:true:true:true:16px:true"`) || !strings.Contains(page.RenderedHTML, `data-custom-elements="true:upgraded:yes:true:yes:on&gt;null:true:true:true"`) || !strings.Contains(page.RenderedHTML, `data-template-inert="0:1"`) || !strings.Contains(page.RenderedHTML, `data-upgraded="yes"`) || !strings.Contains(page.RenderedHTML, `data-connect-order="parent,child"`) || !strings.Contains(page.RenderedHTML, `data-wrapped-import="BODY"`) || !strings.Contains(page.RenderedHTML, `data-message-channel="ready"`) || !strings.Contains(page.RenderedHTML, `<strong data-n="template-clone">template</strong>`) || !strings.Contains(page.RenderedHTML, `data-fragment-query="nested:1"`) || !strings.Contains(page.RenderedHTML, `<b data-n="nested-template-content">nested</b>`) || !strings.Contains(page.RenderedHTML, `<em data-n="fragment-insert-child"></em><b data-n="fragment-replace-child"></b>`) || !strings.Contains(page.RenderedHTML, `<i data-n="range-fragment">range</i>`) || !strings.Contains(page.RenderedHTML, `data-canvas-api="true:true"`) || !strings.Contains(page.RenderedHTML, `data-native-dom="ok:true:true:true:true:true:true:true:probably:true:probably:true:true:true:true:true:false:true:true:true:true:true:true:true"`) || !strings.Contains(page.RenderedHTML, `data-layout="100:20:0:1:0"`) || !strings.Contains(page.RenderedHTML, `data-intersection="true:BODY"`) || !strings.Contains(page.RenderedHTML, `data-mutation="sync,after,observer"`) || !strings.Contains(page.RenderedHTML, `data-xhr="200:{&#34;ok&#34;:true}"`) || !strings.Contains(page.RenderedHTML, `data-promise-xhr="200"`) || !strings.Contains(page.RenderedHTML, `data-axios-xhr="200:{&#34;axios&#34;:true}"`) || !strings.Contains(page.RenderedHTML, `data-async-finally="mounted"`) || !strings.Contains(page.RenderedHTML, `data-delayed-xhr="done"`) || !strings.Contains(page.RenderedHTML, `data-kv-xhr="ready"`) || !strings.Contains(page.RenderedHTML, `data-fetch="true"`) || !strings.Contains(page.RenderedHTML, `<span>fragment</span>`) {
		t.Fatalf("script DOM changes missing from rendered HTML: %s", page.RenderedHTML)
	}
	if !strings.Contains(page.RenderedHTML, `data-event-dispatch="ready:true:true:true:true"`) || !strings.Contains(page.RenderedHTML, `data-event-result="false"`) {
		t.Fatalf("event dispatch semantics missing from rendered HTML: %s", page.RenderedHTML)
	}
	value, err := browser.ExecuteJS(context.Background(), `window.order.join(',')`)
	if err != nil {
		t.Fatal(err)
	}
	if value.String() != "external,inline,written,dynamic,defer" {
		t.Fatalf("script order = %q", value.String())
	}
	for _, path := range []string{"/", "/style.css", "/dynamic.css", "/app.js", "/defer.js", "/dynamic.js", "/written.js", "/pixel.png", "/dynamic.png", "/api", "/promise-api", "/axios-api", "/async-axios-api", "/delayed-api", "/kv-api", "/fetch-api"} {
		if requested[path] != 1 {
			t.Fatalf("request count for %s = %d, want 1", path, requested[path])
		}
		if !strings.Contains(received_cookies[path], "provider_only=yes") {
			t.Fatalf("persistent cookie missing from %s: %q", path, received_cookies[path])
		}
	}
	if !strings.Contains(received_cookies["/"], "session=persistent") {
		t.Fatalf("document cookie = %q, want persistent session", received_cookies["/"])
	}
	for _, path := range []string{"/style.css", "/dynamic.css", "/app.js", "/defer.js", "/dynamic.js", "/written.js", "/pixel.png", "/dynamic.png", "/api", "/promise-api", "/axios-api", "/async-axios-api", "/delayed-api", "/kv-api", "/fetch-api"} {
		if !strings.Contains(received_cookies[path], "session=server") {
			t.Fatalf("resource cookie for %s = %q, want updated session", path, received_cookies[path])
		}
	}
	if len(page.FetchRequests) != 1 || !strings.Contains(page.FetchRequests[0], "/fetch-api?source=fetch") {
		t.Fatalf("fetch requests = %q", page.FetchRequests)
	}
}

func TestCompileJavaScriptPreservesClassicGlobals(t *testing.T) {
	vm := goja.New()
	if _, err := run_javascript(context.Background(), vm, "classic.js", `class CrossScript { static value = 'visible' } var cross_script_global = CrossScript.value;`); err != nil {
		t.Fatal(err)
	}
	if value := vm.Get("cross_script_global"); value == nil || value.String() != "visible" {
		t.Fatalf("classic script global = %v", value)
	}
}

func TestNavigateZhipin(t *testing.T) {
	if os.Getenv("MINIB_LIVE_TEST") == "" {
		t.Skip("set MINIB_LIVE_TEST=1 to run the live zhipin chain")
	}
	cookie_providers := make([]*cookies.Reader, 0, 1)
	if cookie_work_dir := strings.TrimSpace(os.Getenv("MINIB_COOKIE_WORKDIR")); cookie_work_dir != "" {
		cookie_providers = append(cookie_providers, cookies.NewPersistentReader(cookie_work_dir))
	}
	browser, err := NewMiniBrowser(2*time.Minute, cookie_providers...)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), "https://www.zhipin.com/web/geek/jobs", nil)
	if err != nil {
		t.Fatal(err)
	}
	if page.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", page.StatusCode)
	}
	if page.Document == nil || len(page.Resources) == 0 {
		t.Fatalf("incomplete page: document=%v resources=%d", page.Document != nil, len(page.Resources))
	}
	runtime_state, runtime_err := browser.ExecuteJS(context.Background(), `(function(){var wrap=document.querySelector('#wrap'),root=wrap&&wrap.__vue__;return JSON.stringify({readyState:document.readyState,vueApp:typeof Vue==='function'&&!!Vue.app,chunkCount:webpackChunkgeek&&webpackChunkgeek.length,app:!!document.querySelector('#app'),wrap:!!wrap,rootVue:!!root,route:root&&root.$route&&root.$route.fullPath,jobCards:document.querySelectorAll('.job-card-box').length,jobLike:document.querySelectorAll('[class*="job"]').length,wrapText:wrap&&wrap.textContent.slice(0,400)})})()`)
	if runtime_err != nil {
		t.Fatal(runtime_err)
	}
	job_list_requested := false
	for _, request_url := range page.XHRRequests {
		if strings.Contains(request_url, "/wapi/zpgeek/pc/recommend/job/list.json") {
			job_list_requested = true
			break
		}
	}
	if find_by_attribute(page.Document, "id", "app") != nil || find_by_attribute(page.Document, "id", "wrap") == nil || strings.Contains(page.RenderedHTML, "加载中，请稍候") || !strings.Contains(page.RenderedHTML, `class="job-card-box`) || !job_list_requested {
		console_messages := make([]string, 0, 5)
		for _, message := range page.ConsoleMessages {
			if !strings.Contains(message, "54294") {
				console_messages = append(console_messages, message)
				if len(console_messages) == 5 {
					break
				}
			}
		}
		t.Fatalf("page did not finish rendering: runtime=%s rendered=%d job_list_requested=%t failures=%+v console=%q", runtime_state.String(), len(page.RenderedHTML), job_list_requested, page.ScriptFailures, console_messages)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("script failures: %+v", page.ScriptFailures)
	}
	if output_path := strings.TrimSpace(os.Getenv("MINIB_OUTPUT_HTML")); output_path != "" {
		if err := os.MkdirAll(filepath.Dir(output_path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output_path, []byte(page.RenderedHTML), 0600); err != nil {
			t.Fatal(err)
		}
		t.Logf("rendered HTML saved to %s", output_path)
	}
	t.Logf("runtime=%s title=%q rendered=%d resources=%d scripts=%d xhr=%d", runtime_state.String(), text_content(find_element(page.Document, "title")), len(page.RenderedHTML), len(page.Resources), page.ExecutedScripts, len(page.XHRRequests))
}
