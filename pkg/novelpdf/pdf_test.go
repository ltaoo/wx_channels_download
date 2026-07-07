package novelpdf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportWritesPDF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.pdf")
	err := Export(path, &Novel{
		Title:       "book",
		Author:      "author",
		Description: "intro",
		Chapters: []Chapter{
			{Title: "chapter 1", Content: "line one\nline two"},
		},
	}, Options{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pdf: %v", err)
	}
	if !strings.HasPrefix(string(data), "%PDF") {
		t.Fatalf("pdf header = %q", string(data[:4]))
	}
}
