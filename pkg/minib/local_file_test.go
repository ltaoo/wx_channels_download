package minib

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemotePageCannotReadLocalFiles(t *testing.T) {
	root := t.TempDir()
	secret_path := filepath.Join(root, "secret.js")
	if err := os.WriteFile(secret_path, []byte(`globalThis.local_script_ran = true;`), 0600); err != nil {
		t.Fatal(err)
	}
	file_url := (&url.URL{Scheme: "file", Path: secret_path}).String()
	xhr_script := fmt.Sprintf(`try { var xhr = new XMLHttpRequest(); xhr.open('GET', %q, false); xhr.send(); globalThis.xhr_result = xhr.status === 0 ? 'blocked' : xhr.responseText; } catch (_) { globalThis.xhr_result = 'blocked'; }`, file_url)
	fetch_script := fmt.Sprintf(`fetch(%q).then(function(r) { return r.text(); }).then(function(text) { globalThis.fetch_result = text; }, function() { globalThis.fetch_result = 'blocked'; });`, file_url)
	worker_source := fmt.Sprintf(`onmessage = function() { try { importScripts(%q); postMessage('allowed'); } catch (_) { postMessage('blocked'); } };`, file_url)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		if request.URL.Path == "/navigate" {
			fmt.Fprintf(writer, `<script>location.href = %q;</script>`, file_url)
			return
		}
		fmt.Fprintf(writer, `<html><head><script src=%q></script></head><body><a id="local" href=%q>local</a><script>%s %s
		document.getElementById('local').addEventListener('click', function() { location.href = this.href; });
		var worker = new Worker(%q); worker.onmessage = function(event) { globalThis.worker_result = event.data; }; worker.postMessage(null);
		</script></body></html>`, file_url, file_url, xhr_script, fetch_script, "data:text/javascript,"+url.PathEscape(worker_source))
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
	if len(page.ScriptFailures) == 0 {
		t.Fatal("local script resource was not rejected")
	}
	for _, script := range []string{"", xhr_script + fetch_script} {
		if _, err := browser.ExecuteJS(context.Background(), script); err != nil {
			t.Fatal(err)
		}
		result, err := browser.ExecuteJS(context.Background(), `[xhr_result, fetch_result, worker_result, typeof local_script_ran].join(',')`)
		if err != nil || result.String() != "blocked,blocked,blocked,undefined" {
			t.Fatalf("local file access was not blocked: result=%v error=%v", result, err)
		}
	}
	if err := browser.Click(nil, "#local"); err == nil {
		t.Fatal("click navigation to a local file was allowed")
	}
	if _, err := browser.Navigate(context.Background(), server.URL+"/navigate", nil); err == nil {
		t.Fatal("remote navigation to a local file was allowed")
	}
	local_page := filepath.Join(root, "index.html")
	if err := os.WriteFile(local_page, []byte(`<script src="secret.js"></script><script>`+xhr_script+`</script>`), 0600); err != nil {
		t.Fatal(err)
	}
	page, err = browser.Navigate(context.Background(), (&url.URL{Scheme: "file", Path: local_page}).String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := browser.ExecuteJS(context.Background(), `local_script_ran && xhr_result`)
	if err != nil || !strings.Contains(result.String(), "local_script_ran") {
		t.Fatalf("local page access stopped working: result=%v error=%v scripts=%v", result, err, page.ScriptFailures)
	}
}
