package minib

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestESModuleGraphImportMapCyclesAndDynamicImport(t *testing.T) {
	request_counts := make(map[string]int)
	var request_mutex sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		request_mutex.Lock()
		request_counts[request.URL.Path]++
		request_mutex.Unlock()
		response_writer.Header().Set("Cache-Control", "no-store")
		switch request.URL.Path {
		case "/":
			response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(response_writer, `<!doctype html><body><script>window.moduleEvaluationCount=0;window.cycleOrder=[];</script><script type="importmap">{"imports":{"lib/":"/modules/"}}</script><link rel="modulepreload" href="/modules/shared.js"><script type="module" src="/entry.js"></script></body>`)
		case "/entry.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `import sharedDefault,{counter,increment} from 'lib/shared.js';import {cycleValue} from './cycle-a.js';import data from './data.json' with {type:'json'};increment();document.body.setAttribute('data-module-static',[sharedDefault,counter,cycleValue,data.name,import.meta.url.endsWith('/entry.js')].join(':'));document.body.setAttribute('data-bigint',String(9007199254740992n+1n));const lazyURL=new URL('./lazy.js',import.meta.url);lazyURL.searchParams.set('version','one two');const params=new URLSearchParams('a=1&a=2');params.append('b','x y');document.body.setAttribute('data-url-api',[lazyURL.search,lazyURL.searchParams.get('version'),params.getAll('a').join(','),params.toString()].join(':'));import(lazyURL.href).then(function(namespace){document.body.setAttribute('data-module-dynamic',namespace.default+':'+namespace.value+':'+window.moduleEvaluationCount);});`)
		case "/modules/shared.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `window.moduleEvaluationCount++;export let counter=1;export function increment(){counter++}export default 'shared';`)
		case "/cycle-a.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `import './cycle-b.js';window.cycleOrder.push('a');export const cycleValue=window.cycleOrder.join(',');`)
		case "/cycle-b.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `import './cycle-a.js';window.cycleOrder.push('b');`)
		case "/data.json":
			response_writer.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(response_writer, `{"name":"json"}`)
		case "/lazy.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `import sharedDefault from 'lib/shared.js';export const value=42;export default sharedDefault+'-lazy';`)
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
	page, err := browser.Navigate(context.Background(), server.URL+"/", nil, NavigateOptions{DisableCSS: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 {
		t.Fatalf("module failures: %+v", page.ScriptFailures)
	}
	if !strings.Contains(page.RenderedHTML, `data-module-static="shared:2:b,a:json:true"`) {
		t.Fatalf("static module graph did not execute: %s", page.RenderedHTML)
	}
	if !strings.Contains(page.RenderedHTML, `data-bigint="9007199254740993"`) {
		t.Fatalf("BigInt module syntax did not execute: %s", page.RenderedHTML)
	}
	if !strings.Contains(page.RenderedHTML, `data-module-dynamic="shared-lazy:42:1"`) {
		t.Fatalf("dynamic module did not execute or shared module was re-evaluated: %s", page.RenderedHTML)
	}
	if !strings.Contains(page.RenderedHTML, `data-url-api="?version=one+two:one two:1,2:a=1&amp;a=2&amp;b=x+y"`) {
		t.Fatalf("URL and URLSearchParams did not stay linked: %s", page.RenderedHTML)
	}
	request_mutex.Lock()
	defer request_mutex.Unlock()
	for _, resource_path := range []string{"/entry.js", "/modules/shared.js", "/cycle-a.js", "/cycle-b.js", "/data.json", "/lazy.js"} {
		if request_counts[resource_path] != 1 {
			t.Fatalf("request count for %s = %d, want 1", resource_path, request_counts[resource_path])
		}
	}
}

func TestModulePreloadIsDeferredUntilImported(t *testing.T) {
	request_counts := make(map[string]int)
	var request_mutex sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		request_mutex.Lock()
		request_counts[request.URL.Path]++
		request_mutex.Unlock()
		response_writer.Header().Set("Cache-Control", "no-store")
		switch request.URL.Path {
		case "/":
			response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(response_writer, `<!doctype html><body><link rel="modulepreload" href="/unused-preload.js"><link rel="modulepreload" href="/used.js"><script type="module" src="/entry.js"></script></body>`)
		case "/entry.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `import value from './used.js'; document.body.setAttribute('data-module-preload', value);`)
		case "/used.js":
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `export default 'used';`)
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
	page, err := browser.Navigate(context.Background(), server.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 || !strings.Contains(page.RenderedHTML, `data-module-preload="used"`) {
		t.Fatalf("module preload navigation failed: failures=%+v html=%s", page.ScriptFailures, page.RenderedHTML)
	}
	request_mutex.Lock()
	defer request_mutex.Unlock()
	if request_counts["/unused-preload.js"] != 0 || request_counts["/used.js"] != 1 {
		t.Fatalf("module preload requests were not deferred: %#v", request_counts)
	}
}

func TestDynamicModuleRetriesTransientConnectionReset(t *testing.T) {
	request_count := 0
	var request_mutex sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/":
			response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(response_writer, `<!doctype html><body><script type="module">import('./flaky.js').then(function() {}, function() {});</script></body>`)
		case "/flaky.js":
			request_mutex.Lock()
			request_count++
			current_request_count := request_count
			request_mutex.Unlock()
			if current_request_count == 1 {
				panic(http.ErrAbortHandler)
			}
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `document.body.setAttribute('data-flaky-module', 'loaded');`)
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
	page, err := browser.Navigate(context.Background(), server.URL+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.ScriptFailures) != 0 || !strings.Contains(page.RenderedHTML, `data-flaky-module="loaded"`) {
		t.Fatalf("transient module import was not recovered: failures=%+v html=%s", page.ScriptFailures, page.RenderedHTML)
	}
}

func TestModuleGraphPrefetchesDependenciesConcurrently(t *testing.T) {
	const dependency_count = 8
	var active_requests atomic.Int32
	var max_active_requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response_writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/":
			response_writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(response_writer, `<!doctype html><body><script type="module" src="/entry.js"></script></body>`)
		case "/entry.js":
			imports := make([]string, dependency_count)
			for index := range imports {
				imports[index] = fmt.Sprintf("./dependency-%d.js", index)
			}
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprintf(response_writer, "%sdocument.body.setAttribute('data-module-graph', [%s].join(','));", strings.Join(import_module_statements(imports), ""), strings.Join(value_references(dependency_count), ","))
		default:
			if !strings.HasPrefix(request.URL.Path, "/dependency-") {
				http.NotFound(response_writer, request)
				return
			}
			active := active_requests.Add(1)
			for {
				previous := max_active_requests.Load()
				if active <= previous || max_active_requests.CompareAndSwap(previous, active) {
					break
				}
			}
			time.Sleep(30 * time.Millisecond)
			active_requests.Add(-1)
			response_writer.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(response_writer, `export default 1;`)
		}
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
	if len(page.ScriptFailures) != 0 || !strings.Contains(page.RenderedHTML, `data-module-graph="1,1,1,1,1,1,1,1"`) {
		t.Fatalf("module graph did not execute: failures=%+v html=%s", page.ScriptFailures, page.RenderedHTML)
	}
	if max_active_requests.Load() <= 1 {
		t.Fatalf("module dependencies were fetched serially, max active=%d", max_active_requests.Load())
	}
}

func import_module_statements(specifiers []string) []string {
	statements := make([]string, len(specifiers))
	for index, specifier := range specifiers {
		statements[index] = fmt.Sprintf("import value%d from %q;", index, specifier)
	}
	return statements
}

func value_references(count int) []string {
	references := make([]string, count)
	for index := range references {
		references[index] = fmt.Sprintf("value%d", index)
	}
	return references
}
