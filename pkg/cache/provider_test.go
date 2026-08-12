package cache

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCacheProviderNamespacesAreIsolated(t *testing.T) {
	work_dir := t.TempDir()
	provider_registry, err := NewProviderRegistry(work_dir)
	if err != nil {
		t.Fatalf("NewProviderRegistry() error = %v", err)
	}
	first_cache, err := provider_registry.Namespace("first")
	if err != nil {
		t.Fatalf("Namespace(first) error = %v", err)
	}
	second_cache, err := provider_registry.Namespace("second")
	if err != nil {
		t.Fatalf("Namespace(second) error = %v", err)
	}
	first_cache_again, err := provider_registry.Namespace("first")
	if err != nil {
		t.Fatalf("second Namespace(first) error = %v", err)
	}
	if first_cache_again != first_cache {
		t.Fatal("Namespace(first) did not return the shared CacheProvider reference")
	}

	if err := first_cache.Write("book/profile.html", []byte("first")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	data, err := first_cache.Read("book/profile.html")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if string(data) != "first" {
		t.Fatalf("Read() = %q, want first", data)
	}
	if _, err := second_cache.Read("book/profile.html"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("second namespace Read() error = %v, want os.ErrNotExist", err)
	}

	cache_path, err := first_cache.Path("book/profile.html")
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	want_path := filepath.Join(work_dir, "cache", "first", "book", "profile.html")
	if cache_path != want_path {
		t.Fatalf("Path() = %q, want %q", cache_path, want_path)
	}
}

func TestCacheProviderRejectsTraversal(t *testing.T) {
	provider_registry, err := NewProviderRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewProviderRegistry() error = %v", err)
	}
	for _, namespace := range []string{"", ".", "..", "one/two", "../other"} {
		if _, err := provider_registry.Namespace(namespace); err == nil {
			t.Errorf("Namespace(%q) expected error", namespace)
		}
	}
	file_cache, err := provider_registry.Namespace("safe")
	if err != nil {
		t.Fatalf("Namespace(safe) error = %v", err)
	}
	for _, relative_path := range []string{"", ".", "..", "../other", "/absolute", "nested/../../other"} {
		if _, err := file_cache.Path(relative_path); err == nil {
			t.Errorf("Path(%q) expected error", relative_path)
		}
	}
}

func TestFileCacheRemoveAll(t *testing.T) {
	provider_registry, err := NewProviderRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewProviderRegistry() error = %v", err)
	}
	file_cache, err := provider_registry.Namespace("fanqienovel")
	if err != nil {
		t.Fatalf("Namespace() error = %v", err)
	}
	if err := file_cache.Write("book/one.html", []byte("one")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	removed, err := file_cache.RemoveAll("book")
	if err != nil || !removed {
		t.Fatalf("RemoveAll() = %v, %v; want true, nil", removed, err)
	}
	removed, err = file_cache.RemoveAll("book")
	if err != nil || removed {
		t.Fatalf("second RemoveAll() = %v, %v; want false, nil", removed, err)
	}
}
