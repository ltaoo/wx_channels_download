package minib

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPageResourceSchedulerHonorsPriorityConcurrencyAndScriptSemantics(t *testing.T) {
	var active_requests atomic.Int32
	var max_active_requests atomic.Int32
	priorities := make(map[string]string)
	client_hints := make(map[string]string)
	var priority_mutex sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/" {
			response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			var markup strings.Builder
			markup.WriteString(`<!doctype html><body><script>window.scheduleOrder=[]</script>`)
			for image_index := 0; image_index < 12; image_index++ {
				fmt.Fprintf(&markup, `<img src="/image-%d.png">`, image_index)
			}
			markup.WriteString(`<script src="/blocking.js"></script><script async src="/async-slow.js"></script><script async src="/async-fast.js"></script><script defer src="/defer.js"></script></body>`)
			_, _ = response_writer.Write([]byte(markup.String()))
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
		priority_mutex.Lock()
		priorities[request.URL.Path] = request.Header.Get("Priority")
		client_hints[request.URL.Path] = request.Header.Get("Sec-Ch-Ua")
		priority_mutex.Unlock()
		switch request.URL.Path {
		case "/blocking.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `window.scheduleOrder.push('blocking')`)
		case "/async-fast.js":
			time.Sleep(5 * time.Millisecond)
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `window.scheduleOrder.push('async-fast')`)
		case "/async-slow.js":
			time.Sleep(40 * time.Millisecond)
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `window.scheduleOrder.push('async-slow')`)
		case "/defer.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `window.scheduleOrder.push('defer');document.body.setAttribute('data-schedule',window.scheduleOrder.join(','))`)
		default:
			if strings.HasPrefix(request.URL.Path, "/image-") {
				time.Sleep(10 * time.Millisecond)
				response_writer.Header().Set("Content-Type", "image/png")
				_, _ = response_writer.Write([]byte(strconv.Itoa(len(request.URL.Path))))
				return
			}
			http.NotFound(response_writer, request)
		}
	}))
	defer server.Close()

	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	navigation_headers := http.Header{"Sec-Ch-Ua": {`"Chromium";v="151"`}}
	page, err := browser.Navigate(context.Background(), server.URL+"/", navigation_headers)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("script failures: %+v", page.ScriptFailures)
	}
	if !strings.Contains(page.RenderedHTML, `data-schedule="blocking,async-fast,async-slow,defer"`) {
		t.Fatalf("script scheduling order mismatch: %s", page.RenderedHTML)
	}
	if max_active_requests.Load() > per_host_resource_concurrency {
		t.Fatalf("per-host concurrency = %d, limit = %d", max_active_requests.Load(), per_host_resource_concurrency)
	}
	priority_mutex.Lock()
	defer priority_mutex.Unlock()
	if priorities["/blocking.js"] != "u=0" || priorities["/async-fast.js"] != "u=1" || priorities["/defer.js"] != "u=2" || priorities["/image-0.png"] != "u=5, i" {
		t.Fatalf("resource priority headers = %+v", priorities)
	}
	if client_hints["/blocking.js"] != navigation_headers.Get("Sec-Ch-Ua") || client_hints["/image-0.png"] != navigation_headers.Get("Sec-Ch-Ua") {
		t.Fatalf("resource client hints = %+v", client_hints)
	}
}
