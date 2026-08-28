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
var eventParent = document.createElement('div'); eventParent.setAttribute('data-n', 'event-parent'); var eventChild = document.createElement('button'); eventChild.setAttribute('data-n', 'event-child'); eventParent.appendChild(eventChild); document.body.appendChild(eventParent); eventParent.addEventListener('minib-event', function(event) { document.body.setAttribute('data-event-dispatch', [event.detail.value, event.target === eventChild, event.currentTarget === eventParent].join(':')); event.preventDefault(); }); document.body.setAttribute('data-event-result', eventChild.dispatchEvent(new CustomEvent('minib-event', { detail: { value: 'ready' }, bubbles: true, cancelable: true })));
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
class MinibExisting extends HTMLElement { static get observedAttributes() { return ['data-state']; } constructor() { super(); this.textContent = 'upgraded'; } connectedCallback() { this.setAttribute('data-connected', 'yes'); } attributeChangedCallback(name, oldValue, newValue) { this.setAttribute('data-observed', String(oldValue) + '>' + String(newValue)); } }
customElements.define('minib-existing', MinibExisting);
var createdCustom = document.createElement('minib-existing'); createdCustom.setAttribute('data-n', 'constructed-custom-element'); document.body.appendChild(createdCustom);
createdCustom.setAttribute('data-state', 'on'); createdCustom.removeAttribute('data-state');
var disabledCustom = document.createElement('minib-disabled'); disabledCustom.setAttribute('data-n', 'disabled-custom-element'); disabledCustom.setAttribute('disable-upgrade', ''); document.body.appendChild(disabledCustom);
class MinibDisabled extends HTMLElement { static get observedAttributes() { return ['disable-upgrade']; } constructor() { super(); disabledCustom.removeAttribute('disable-upgrade'); } attributeChangedCallback(name, oldValue, newValue) { if (newValue === null && this.isConnected) this.setAttribute('data-upgraded', 'yes'); } }
customElements.define('minib-disabled', MinibDisabled);
customElements.whenDefined('minib-existing').then(function() { document.body.setAttribute('data-custom-elements', [existingCustom instanceof MinibExisting, existingCustom.textContent, existingCustom.getAttribute('data-connected'), createdCustom instanceof MinibExisting, createdCustom.getAttribute('data-connected'), createdCustom.getAttribute('data-observed'), customElements.get('minib-existing') === MinibExisting].join(':')); });
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
	if !strings.Contains(page.RenderedHTML, `external-inline`) || !strings.Contains(page.RenderedHTML, `data-initial-script-load="done"`) || !strings.Contains(page.RenderedHTML, `data-written="done"`) || !strings.Contains(page.RenderedHTML, `data-apply-null="0"`) || !strings.Contains(page.RenderedHTML, `data-single-argument-timer="done"`) || !strings.Contains(page.RenderedHTML, `data-dynamic="done"`) || !strings.Contains(page.RenderedHTML, `data-dynamic-style="loaded"`) || !strings.Contains(page.RenderedHTML, `data-dynamic-image="loaded"`) || !strings.Contains(page.RenderedHTML, `data-defer-saw-body="yes"`) || !strings.Contains(page.RenderedHTML, `data-anchor="/jobs?q=go#details"`) || !strings.Contains(page.RenderedHTML, `data-attribute="test"`) || !strings.Contains(page.RenderedHTML, `data-has-attributes="true:false"`) || !strings.Contains(page.RenderedHTML, `data-class-list="one,two"`) || !strings.Contains(page.RenderedHTML, `data-event-prototype="ready"`) || !strings.Contains(page.RenderedHTML, `data-wrapped-event="wrapped"`) || !strings.Contains(page.RenderedHTML, `data-invalid-event="true"`) || !strings.Contains(page.RenderedHTML, `data-adjacent="done"`) || !strings.Contains(page.RenderedHTML, `data-module="done"`) || !strings.Contains(page.RenderedHTML, `data-named-global="done"`) || !strings.Contains(page.RenderedHTML, `data-browser-api="detached:function"`) || !strings.Contains(page.RenderedHTML, `data-youtube-apis="true:en-US:en-US:click:Enter:detail:0 0 1 1:true:false:true:true:true:16px:true"`) || !strings.Contains(page.RenderedHTML, `data-custom-elements="true:upgraded:yes:true:yes:on&gt;null:true"`) || !strings.Contains(page.RenderedHTML, `data-template-inert="0:1"`) || !strings.Contains(page.RenderedHTML, `data-upgraded="yes"`) || !strings.Contains(page.RenderedHTML, `data-wrapped-import="BODY"`) || !strings.Contains(page.RenderedHTML, `data-message-channel="ready"`) || !strings.Contains(page.RenderedHTML, `<strong data-n="template-clone">template</strong>`) || !strings.Contains(page.RenderedHTML, `data-fragment-query="nested:1"`) || !strings.Contains(page.RenderedHTML, `<b data-n="nested-template-content">nested</b>`) || !strings.Contains(page.RenderedHTML, `<em data-n="fragment-insert-child"></em><b data-n="fragment-replace-child"></b>`) || !strings.Contains(page.RenderedHTML, `<i data-n="range-fragment">range</i>`) || !strings.Contains(page.RenderedHTML, `data-canvas-api="true:true"`) || !strings.Contains(page.RenderedHTML, `data-native-dom="ok:true:true:true:true:true:true:true:probably:true:probably:true:true:true:true:true:false:true:true:true:true:true:true:true"`) || !strings.Contains(page.RenderedHTML, `data-layout="100:20:0:1:0"`) || !strings.Contains(page.RenderedHTML, `data-intersection="true:BODY"`) || !strings.Contains(page.RenderedHTML, `data-mutation="sync,after,observer"`) || !strings.Contains(page.RenderedHTML, `data-xhr="200:{&#34;ok&#34;:true}"`) || !strings.Contains(page.RenderedHTML, `data-promise-xhr="200"`) || !strings.Contains(page.RenderedHTML, `data-axios-xhr="200:{&#34;axios&#34;:true}"`) || !strings.Contains(page.RenderedHTML, `data-async-finally="mounted"`) || !strings.Contains(page.RenderedHTML, `data-delayed-xhr="done"`) || !strings.Contains(page.RenderedHTML, `data-kv-xhr="ready"`) || !strings.Contains(page.RenderedHTML, `data-fetch="true"`) || !strings.Contains(page.RenderedHTML, `<span>fragment</span>`) {
		t.Fatalf("script DOM changes missing from rendered HTML: %s", page.RenderedHTML)
	}
	if !strings.Contains(page.RenderedHTML, `data-event-dispatch="ready:true:true"`) || !strings.Contains(page.RenderedHTML, `data-event-result="false"`) {
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
