package webassets

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestRegistryServeHTTP(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("/__assets/platform/wxchannels", fstest.MapFS{
		"channels.ws.js": &fstest.MapFile{Data: []byte("window.ws = true;")},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/__assets/platform/wxchannels/channels.ws.js", nil)
	response := httptest.NewRecorder()
	registry.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Body.String(); got != "window.ws = true;" {
		t.Fatalf("body = %q", got)
	}
	if got := response.Header().Get("Content-Type"); got != "application/javascript; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
	if got := response.Header().Get("ETag"); got == "" {
		t.Fatal("missing ETag")
	}
}

func TestRegistryRejectsInvalidRegistration(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("/__assets", nil); !errors.Is(err, ErrNilFS) {
		t.Fatalf("nil fs error = %v", err)
	}
	if err := registry.Register("assets", fstest.MapFS{}); !errors.Is(err, ErrInvalidPrefix) {
		t.Fatalf("invalid prefix error = %v", err)
	}
	if err := registry.Register("/__assets", fstest.MapFS{}); err != nil {
		t.Fatalf("initial registration: %v", err)
	}
	if err := registry.Register("/__assets", fstest.MapFS{}); !errors.Is(err, ErrDuplicateMount) {
		t.Fatalf("duplicate mount error = %v", err)
	}
}

func TestRegistryRejectsTraversalAndSupportsHead(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("/__assets", fstest.MapFS{
		"file.js": &fstest.MapFile{Data: []byte("ok")},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	traversal := httptest.NewRecorder()
	registry.ServeHTTP(traversal, httptest.NewRequest(http.MethodGet, "/__assets/a/../file.js", nil))
	if traversal.Code != http.StatusNotFound {
		t.Fatalf("traversal status = %d", traversal.Code)
	}

	head := httptest.NewRecorder()
	registry.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/__assets/file.js", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("head = status %d, body %q", head.Code, head.Body.String())
	}
}

func TestRegistryServesExplicitCompatibilityFile(t *testing.T) {
	registry := NewRegistry()
	assets := fstest.MapFS{"channels.ws.js": &fstest.MapFile{Data: []byte("ok")}}
	if err := registry.RegisterFile("/__assets/inject/channels.ws.js", assets, "channels.ws.js"); err != nil {
		t.Fatalf("register compatibility file: %v", err)
	}

	response := httptest.NewRecorder()
	registry.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/__assets/inject/channels.ws.js", nil))
	if response.Code != http.StatusOK || response.Body.String() != "ok" {
		t.Fatalf("response = status %d, body %q", response.Code, response.Body.String())
	}
}
