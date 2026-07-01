package main

import (
	"os"
	"strings"
	"testing"
)

func TestDownloadTargetArticle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network download in short mode")
	}

	path, article, err := downloadArticle(targetURL, t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(article.Title, "Omni-Tools") {
		t.Fatalf("Title = %q, want Omni-Tools article", article.Title)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	if !strings.Contains(html, "OmniTools") {
		t.Fatalf("downloaded HTML does not contain expected article body")
	}
	if !strings.Contains(html, "data:image/") {
		t.Fatalf("downloaded HTML does not contain embedded images")
	}
}
