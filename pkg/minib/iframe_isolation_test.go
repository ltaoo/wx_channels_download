package minib

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Fingerprint-style font enumeration creates a hidden iframe, appends
// measurement spans to iframe.contentWindow.document, measures them, and
// removes the iframe. When contentDocument aliased the main document those
// spans leaked into the rendered HTML (mmMwWLliI0O&1 repeated ~150 times).
const iframe_probe_page = `<!doctype html>
<html><body>
<div id="probe-target">content</div>
<script>
var frame = document.createElement('iframe');
document.body.appendChild(frame);
var frameDocument = frame.contentWindow.document;
var holder = frameDocument.createElement('div');
holder.style.visibility = 'hidden';
frameDocument.body.appendChild(holder);
for (var i = 0; i < 5; i++) {
  var span = frameDocument.createElement('span');
  span.style.position = 'absolute';
  span.textContent = 'mmMwWLliI0O&1';
  holder.appendChild(span);
}
document.body.setAttribute('data-frame-globals', [
  frame.contentWindow !== window,
  frame.contentWindow.parent === window,
  frame.contentWindow.document === frame.contentDocument,
  frame.contentDocument !== document,
  frame.contentDocument.body !== document.body,
  typeof frame.contentWindow.setTimeout === 'function'
].join(':'));
frame.parentNode.removeChild(frame);
</script>
</body></html>`

func TestIframeContentDocumentIsolated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response_writer.Write([]byte(iframe_probe_page))
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(30 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	page, err := browser.Navigate(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(page.RenderedHTML, ">mmMwWLliI0O&amp;1<") {
		t.Fatalf("iframe measurement spans leaked into the page HTML: %s", render_excerpt(page.RenderedHTML, "mmMwWLliI0O"))
	}
	if !strings.Contains(page.RenderedHTML, `data-frame-globals="true:true:true:true:true:true"`) {
		t.Fatalf("iframe window/document relationship mismatch: %s", render_excerpt(page.RenderedHTML, "data-frame-globals"))
	}
}

// A returning browser profile restores localStorage before page scripts run;
// NavigateOptions.LocalStorage reproduces that state for first-visit dialogs
// and feature flags.
func TestLocalStorageSeededFromNavigateOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response_writer.Write([]byte(`<!doctype html><html><body><script>
document.body.setAttribute('data-seeded', localStorage.getItem('inviteDialogLastShowTime'));
document.body.setAttribute('data-missing', String(localStorage.getItem('nope')));
</script></body></html>`))
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(30 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	page, err := browser.Navigate(context.Background(), server.URL, nil, NavigateOptions{
		LocalStorage: map[string]string{"inviteDialogLastShowTime": "1789000000000"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page.RenderedHTML, `data-seeded="1789000000000"`) {
		t.Fatalf("localStorage seed not visible to page scripts: %s", render_excerpt(page.RenderedHTML, "data-seeded"))
	}
	if !strings.Contains(page.RenderedHTML, `data-missing="null"`) {
		t.Fatalf("unseeded localStorage keys must read null: %s", render_excerpt(page.RenderedHTML, "data-missing"))
	}
}
