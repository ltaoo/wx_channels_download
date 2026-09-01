package minib

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
