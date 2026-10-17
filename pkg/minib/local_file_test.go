package minib

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNavigateFileURLLoadsDocumentResourcesAndFetch(t *testing.T) {
	root_dir := t.TempDir()
	write_local_file := func(name string, contents string) string {
		file_path := filepath.Join(root_dir, name)
		if err := os.WriteFile(file_path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		return file_path
	}

	document_path := write_local_file("index.html", `<!doctype html>
<html><head><title>local document</title><link rel="stylesheet" href="style.css"></head>
<body><script src="classic.js"></script><script type="module" src="module.js"></script>
<script>fetch('data.txt').then(function(response){return response.text()}).then(function(value){document.body.setAttribute('data-fetch',value)})</script>
</body></html>`)
	write_local_file("style.css", "body{background-color:rgb(1,2,3)}")
	write_local_file("classic.js", `document.body.setAttribute('data-classic','loaded');document.body.setAttribute('data-css',getComputedStyle(document.body).backgroundColor)`)
	write_local_file("module.js", `import value from './module-dependency.js';document.body.setAttribute('data-module',value)`)
	write_local_file("module-dependency.js", `export default 'loaded'`)
	write_local_file("data.txt", "local data")

	document_url := local_file_url(document_path)
	browser, err := NewMiniBrowser(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Navigate(context.Background(), document_url, nil, NavigateOptions{DisableImages: true})
	if err != nil {
		t.Fatal(err)
	}

	if page.URL != document_url || page.StatusCode != http.StatusOK || !strings.HasPrefix(page.ContentType, "text/html") {
		t.Fatalf("document response mismatch: url=%q status=%d content-type=%q", page.URL, page.StatusCode, page.ContentType)
	}
	for _, marker := range []string{"data-classic=\"loaded\"", "data-css=\"rgb(1,2,3)\"", "data-module=\"loaded\"", "data-fetch=\"local data\""} {
		if !strings.Contains(page.RenderedHTML, marker) {
			t.Fatalf("rendered HTML missing %s; failures=%+v html=%s", marker, page.ScriptFailures, page.RenderedHTML)
		}
	}

	resource_by_url := make(map[string]Resource)
	for _, resource := range page.Resources {
		resource_by_url[resource.URL] = resource
	}
	for _, resource_name := range []string{"style.css", "classic.js", "module.js", "module-dependency.js"} {
		resource_url := local_file_url(filepath.Join(root_dir, resource_name))
		resource, resource_found := resource_by_url[resource_url]
		if !resource_found {
			t.Fatalf("resource %s was not discovered; resources=%+v", resource_name, page.Resources)
		}
		if resource.Err != nil || resource.StatusCode != http.StatusOK {
			t.Fatalf("resource %s failed: status=%d err=%v", resource_name, resource.StatusCode, resource.Err)
		}
	}
	missing_response, missing_err := browser.Get(context.Background(), local_file_url("/minib/does-not-exist"), nil)
	if missing_err != nil || missing_response.StatusCode != http.StatusNotFound {
		t.Fatalf("missing file response = %+v, %v", missing_response, missing_err)
	}
	method_response, method_err := browser.Request(context.Background(), "POST", document_url, strings.NewReader("unused"), nil)
	if method_err != nil || method_response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST file response = %+v, %v", method_response, method_err)
	}
	head_response, head_err := browser.Request(context.Background(), http.MethodHead, document_url, nil, nil)
	if head_err != nil || head_response.StatusCode != http.StatusOK || len(head_response.Body) != 0 ||
		head_response.Header.Get("Content-Length") != strconv.Itoa(len(page.HTML)) {
		t.Fatalf("HEAD file response = %+v, %v", head_response, head_err)
	}
	if host, host_err := request_host(document_url); host_err != nil || host != "file" {
		t.Fatalf("pool file host = %q, %v", host, host_err)
	}
}

func local_file_url(file_path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(file_path)}).String()
}
