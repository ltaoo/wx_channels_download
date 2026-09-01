package minib

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestAsyncXHRFetchAbortAndNetworkConcurrency(t *testing.T) {
	var active_requests atomic.Int32
	var max_active_requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/" {
			response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(response_writer, `<!doctype html><body><script>
var xhrStates = [];
function request(path, name) {
  var xhr = new XMLHttpRequest();
  xhr.onreadystatechange = function() { if (name === 'one') { xhrStates.push(xhr.readyState); if (xhr.readyState === 4) document.body.setAttribute('data-states', xhrStates.join(',')); } };
  xhr.open('GET', path, true);
  xhr.onload = function() { document.body.setAttribute('data-' + name, xhr.responseText); };
  xhr.send();
}
request('/slow-one', 'one'); request('/slow-two', 'two');
var controller = new AbortController();
document.body.setAttribute('data-abort-api', [controller.signal instanceof EventTarget, typeof controller.signal.throwIfAborted].join(':'));
fetch('/abort', { signal: controller.signal }).catch(function(reason) { document.body.setAttribute('data-abort', reason.name); });
controller.abort();
Promise.all([fetch('/fetch-one').then(function(response) { return response.text(); }), fetch('/fetch-two').then(function(response) { return response.text(); })]).then(function(values) { document.body.setAttribute('data-fetches', values.join(',')); });
</script></body>`)
			return
		}
		active_count := active_requests.Add(1)
		defer active_requests.Add(-1)
		for {
			current_max := max_active_requests.Load()
			if active_count <= current_max || max_active_requests.CompareAndSwap(current_max, active_count) {
				break
			}
		}
		if request.URL.Path == "/abort" {
			time.Sleep(80 * time.Millisecond)
			return
		}
		time.Sleep(30 * time.Millisecond)
		_, _ = fmt.Fprint(response_writer, strings.TrimPrefix(request.URL.Path, "/"))
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL+"/", nil, NavigateOptions{
		JavaScriptTimeout: 5 * time.Millisecond,
		ResourceTimeout:   time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("script failures: %+v", page.ScriptFailures)
	}
	for _, marker := range []string{
		`data-one="slow-one"`,
		`data-two="slow-two"`,
		`data-fetches="fetch-one,fetch-two"`,
		`data-abort="AbortError"`,
		`data-abort-api="true:function"`,
		`data-states="1,2,3,4"`,
	} {
		if !strings.Contains(page.RenderedHTML, marker) {
			t.Fatalf("missing %s in %s", marker, page.RenderedHTML)
		}
	}
	if max_active_requests.Load() < 2 {
		t.Fatalf("XHR/fetch requests did not overlap; max active = %d", max_active_requests.Load())
	}
}

func TestNavigateWaitsForXHRWhenIntervalIsPending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/data" {
			time.Sleep(20 * time.Millisecond)
			_, _ = response_writer.Write([]byte("ready"))
			return
		}
		_, _ = response_writer.Write([]byte(`<html><body><script>
setInterval(function() {}, 1000);
var request = new XMLHttpRequest();
request.open('GET', '/data');
request.onload = function() { document.body.setAttribute('data-result', request.responseText); };
request.send();
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
	if !strings.Contains(page.RenderedHTML, `data-result="ready"`) {
		t.Fatalf("HTML missing XHR result: %s", page.RenderedHTML)
	}
}

func TestWebSocketLifecycleAndEvents(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin:  func(*http.Request) bool { return true },
		Subprotocols: []string{"chat"},
	}
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/ws" {
			connection, err := upgrader.Upgrade(response_writer, request, nil)
			if err != nil {
				return
			}
			defer connection.Close()
			message_type, message, err := connection.ReadMessage()
			if err == nil {
				_ = connection.WriteMessage(message_type, append([]byte("reply:"), message...))
				_, _, _ = connection.ReadMessage()
			}
			return
		}
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(response_writer, `<!doctype html><body><script>
var socket = new WebSocket('ws://%s/ws', ['chat']);
document.body.setAttribute('data-websocket-api', [socket instanceof EventTarget, WebSocket.CONNECTING, WebSocket.OPEN, socket.readyState].join(':'));
socket.addEventListener('open', function() { document.body.setAttribute('data-open', socket.protocol); socket.send('ping'); });
socket.addEventListener('message', function(event) { document.body.setAttribute('data-message', event.data + ':' + event.origin); socket.close(1000, 'done'); });
socket.addEventListener('close', function(event) { document.body.setAttribute('data-close', event.code + ':' + event.reason + ':' + event.wasClean); });
</script></body>`, request.Host)
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(page.RenderedHTML, `data-close="1000:done:true"`) && time.Now().Before(deadline) {
		_, err = browser.ExecuteJS(context.Background(), "0")
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("script failures: %+v", page.ScriptFailures)
	}
	for _, marker := range []string{
		`data-websocket-api="true:0:1:0"`,
		`data-open="chat"`,
		`data-message="reply:ping:http://`,
		`data-close="1000:done:true"`,
	} {
		if !strings.Contains(page.RenderedHTML, marker) {
			t.Fatalf("missing %s in %s", marker, page.RenderedHTML)
		}
	}
}

func TestBlobWorkerStructuredCloneAndIterableHeaders(t *testing.T) {
	request_headers := make(chan http.Header, 2)
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/headers" {
			request_headers <- request.Header.Clone()
			response_writer.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(response_writer, `{"ok":true,"setting":[1,2]}`)
			return
		}
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, `<!doctype html><body><script>
var source = 'onmessage=function(event){var input=event.data;postMessage(new Uint8Array([input[0]+1,input[1]+2]))}';
var worker = new Worker(URL.createObjectURL(new Blob([source], {type:'text/javascript'})));
worker.onmessage = function(event) {
  document.body.setAttribute('data-worker', [worker instanceof Worker, event.data instanceof Uint8Array, event.data[0], event.data[1]].join(':'));
};
worker.postMessage(new Uint8Array([4, 8]));
window.addEventListener('message', function(event) { document.body.setAttribute('data-window-message', event.data.kind + ':' + (event.source === window)); });
window.parent.postMessage({kind:'mounted'}, '*');
var headers = new Headers([['X-App-Za', 'OS=webplayer'], ['x-zse-96', '2.0_test']]);
fetch('/headers', {headers: headers}).then(function(response) {
  document.body.setAttribute('data-headers', [Object.fromEntries(response.headers).hasOwnProperty('content-type'), Array.from(headers.keys()).join(',')].join(':'));
});
var jsonXhr = new XMLHttpRequest();
jsonXhr.open('GET', '/headers');
jsonXhr.responseType = 'json';
jsonXhr.onload = function() { document.body.setAttribute('data-xhr-json-array', [Array.isArray(jsonXhr.response.setting), jsonXhr.response.setting.map(function(value) { return value * 2; }).join(',')].join(':')); };
jsonXhr.send();
var parsedDocument = new DOMParser().parseFromString('<main><b>parsed</b></main>', 'text/html');
document.body.setAttribute('data-dom-parser', parsedDocument.querySelector('b').textContent + ':' + (parsedDocument instanceof Document));
</script></body>`)
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), server.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("script failures: %+v", page.ScriptFailures)
	}
	for _, marker := range []string{
		`data-worker="true:true:5:10"`,
		`data-window-message="mounted:true"`,
		`data-headers="true:x-app-za,x-zse-96"`,
		`data-xhr-json-array="true:2,4"`,
		`data-dom-parser="parsed:true"`,
	} {
		if !strings.Contains(page.RenderedHTML, marker) {
			t.Fatalf("missing %s in %s", marker, page.RenderedHTML)
		}
	}
	var fetch_headers http.Header
	for len(request_headers) > 0 {
		headers := <-request_headers
		if headers.Get("X-App-Za") != "" {
			fetch_headers = headers
		}
	}
	if fetch_headers == nil {
		t.Fatal("fetch request was not sent")
	}
	if fetch_headers.Get("X-App-Za") != "OS=webplayer" || fetch_headers.Get("X-Zse-96") != "2.0_test" {
		t.Fatalf("unexpected fetch headers: %v", fetch_headers)
	}
	if fetch_headers.Get("0") != "" || fetch_headers.Get("1") != "" {
		t.Fatalf("iterable headers leaked numeric names: %v", fetch_headers)
	}
}
