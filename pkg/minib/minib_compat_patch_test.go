package minib

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWindowEventMethodsCanBePatched(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, `<!doctype html><body><script>
var native = window.addEventListener;
var patched_calls = [];
window.addEventListener = function(type, listener, options) { patched_calls.push(type); };
window.addEventListener('hashchange', function() {});
window.addEventListener = native;
document.body.setAttribute('data-event-patch', patched_calls.join(',') + ':' + typeof window.addEventListener);
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
	if !strings.Contains(page.RenderedHTML, `data-event-patch="hashchange:function"`) {
		t.Fatalf("window event patch mismatch: %s", page.RenderedHTML)
	}
}

func TestTextDecoderStreamPipesThroughTransformStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(response_writer, `<!doctype html><body><script>
var source = new ReadableStream({ start: function(controller) { controller.enqueue(new TextEncoder().encode('ok')); controller.close(); } });
source.pipeThrough(new TextDecoderStream()).getReader().read().then(function(result) {
  document.body.setAttribute('data-transform-stream', result.value);
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
	if !strings.Contains(page.RenderedHTML, `data-transform-stream="ok"`) {
		t.Fatalf("TextDecoderStream mismatch: %s", page.RenderedHTML)
	}
}
