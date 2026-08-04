package services

import (
	"os"
	"path/filepath"
	"testing"
)

func makeFilesystemFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFixtureFile(t, filepath.Join(dir, "readme.txt"), "readme")
	writeFixtureFile(t, filepath.Join(dir, "video.mp4"), "video")
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}
	writeFixtureFile(t, filepath.Join(dir, "nested", "deep-note.md"), "note")
	return dir
}

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

func TestFSServiceListFiles(t *testing.T) {
	dir := makeFilesystemFixture(t)
	writeFixtureFile(t, filepath.Join(dir, ".DS_Store"), "metadata")
	service := NewFSService()

	result, err := service.ListFiles(ListFilesOptions{Dir: dir})
	if err != nil {
		t.Fatalf("list files: %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("expected three root entries, got %d", result.Total)
	}
	if len(result.Files) != 3 || !result.Files[0].IsDir || result.Files[0].Name != "nested" {
		t.Fatalf("expected directory-first listing, got %#v", result.Files)
	}
	for _, file := range result.Files {
		if file.Name == ".DS_Store" {
			t.Fatal("expected .DS_Store to be ignored")
		}
	}
}

func TestFSServiceListFilesDefaultsToDocumentsDirectory(t *testing.T) {
	home := t.TempDir()
	documents := filepath.Join(home, "Documents")
	if err := os.Mkdir(documents, 0o755); err != nil {
		t.Fatalf("create Documents directory: %v", err)
	}
	writeFixtureFile(t, filepath.Join(documents, "document.txt"), "document")
	t.Setenv("HOME", home)

	result, err := NewFSService().ListFiles(ListFilesOptions{})
	if err != nil {
		t.Fatalf("list default Documents directory: %v", err)
	}
	if result.Total != 1 || result.Files[0].Name != "document.txt" {
		t.Fatalf("expected Documents contents, got %#v", result)
	}
}

func TestFSServiceSearchFilesRecursivelyByDefault(t *testing.T) {
	dir := makeFilesystemFixture(t)
	service := NewFSService()

	result, err := service.SearchFiles(SearchFilesOptions{
		Dir:   dir,
		Query: "dpnt",
	})
	if err != nil {
		t.Fatalf("search files: %v", err)
	}
	if result.Total != 1 || len(result.Files) != 1 {
		t.Fatalf("expected one fuzzy match, got %#v", result)
	}
	if result.Files[0].Name != "deep-note.md" {
		t.Fatalf("expected nested match, got %#v", result.Files[0])
	}
}

func TestFSServiceSearchCanStayInCurrentDirectory(t *testing.T) {
	dir := makeFilesystemFixture(t)
	service := NewFSService()
	recursive := false

	result, err := service.SearchFiles(SearchFilesOptions{
		Dir:       dir,
		Query:     "deep",
		Recursive: &recursive,
	})
	if err != nil {
		t.Fatalf("search files: %v", err)
	}
	if result.Total != 0 {
		t.Fatalf("expected no root-level match, got %#v", result)
	}
}

func TestFSServiceListFilesSupportsFiltersAndPagination(t *testing.T) {
	dir := makeFilesystemFixture(t)
	service := NewFSService()

	result, err := service.ListFiles(ListFilesOptions{
		Dir:    dir,
		Accept: []string{".mp4"},
		Offset: 1,
		Limit:  1,
	})
	if err != nil {
		t.Fatalf("list filtered files: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("expected directory and accepted file before pagination, got %d", result.Total)
	}
	if len(result.Files) != 1 || result.Files[0].Name != "video.mp4" {
		t.Fatalf("unexpected page: %#v", result.Files)
	}
}
