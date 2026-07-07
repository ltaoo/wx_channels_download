package shuba69

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadLocalArchiveMatchesCatalogBySourceURL(t *testing.T) {
	root := writeLocalArchiveFixture(t)
	result, err := LoadLocalArchive(root)
	if err != nil {
		t.Fatalf("LoadLocalArchive: %v", err)
	}
	if result.Novel.Title != "book" || result.Novel.BookID != "1" {
		t.Fatalf("novel = %#v", result.Novel)
	}
	if len(result.Chapters) != 2 {
		t.Fatalf("chapters len = %d", len(result.Chapters))
	}
	if result.ChapterFiles[0] != "chapters/0001_chapter 1.html" {
		t.Fatalf("first chapter file = %#v", result.ChapterFiles)
	}
	if result.ChapterFiles[1] != "chapters/0002_same.html" {
		t.Fatalf("second chapter file = %#v", result.ChapterFiles)
	}
	if result.SkippedChapterFiles != 2 {
		t.Fatalf("skipped files = %d", result.SkippedChapterFiles)
	}
	if strings.Contains(result.Chapters[0].Content.Content, "chapter 1\nchapter 1") {
		t.Fatalf("repeated title was not trimmed: %q", result.Chapters[0].Content.Content)
	}
}

func TestCleanLocalArchivePersistsCatalogAndSkipsExistingText(t *testing.T) {
	root := writeLocalArchiveFixture(t)
	result, err := CleanLocalArchive(root, CleanArchiveOptions{})
	if err != nil {
		t.Fatalf("CleanLocalArchive: %v", err)
	}
	if result.CleanedChapters != 2 || result.SkippedChapters != 0 {
		t.Fatalf("first clean counters = cleaned %d skipped %d", result.CleanedChapters, result.SkippedChapters)
	}
	if _, err := os.Stat(filepath.Join(root, "clean", "catalog.json")); err != nil {
		t.Fatalf("catalog json missing: %v", err)
	}
	for _, chapter := range result.Catalog.Chapters {
		text, err := LoadCleanChapterText(root, chapter)
		if err != nil {
			t.Fatalf("read clean text %s: %v", chapter.TextPath, err)
		}
		if strings.Contains(text, chapter.Title+"\n"+chapter.Title) {
			t.Fatalf("title repeated in %s: %q", chapter.TextPath, text)
		}
	}

	second, err := CleanLocalArchive(root, CleanArchiveOptions{})
	if err != nil {
		t.Fatalf("second CleanLocalArchive: %v", err)
	}
	if second.CleanedChapters != 0 || second.SkippedChapters != 2 {
		t.Fatalf("second clean counters = cleaned %d skipped %d", second.CleanedChapters, second.SkippedChapters)
	}

	stalePath := filepath.Join(root, "clean", "chapters", "stale.txt")
	if err := os.WriteFile(stalePath, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := CleanLocalArchive(root, CleanArchiveOptions{Force: true}); err != nil {
		t.Fatalf("force CleanLocalArchive: %v", err)
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("stale txt should be removed on force clean, stat err = %v", err)
	}
}

func writeLocalArchiveFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "source"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "chapters"), 0o755); err != nil {
		t.Fatal(err)
	}
	bookHTML := `<!doctype html><html><head>
<meta property="og:novel:book_name" content="book">
<meta property="og:novel:author" content="author">
<script>var bookinfo = {articleid:'1', articlename:'book', author:'author', sortName:'kind'};</script>
</head><body><a class="more-btn" href="https://www.69shuba.com/book/1/">完整目录</a></body></html>`
	catalogHTML := `<!doctype html><html><body><h1>book</h1><div class="qustime"><ul>
<li data-num="1"><a href="https://www.69shuba.com/txt/1/1">chapter 1</a></li>
<li data-num="2"><a href="https://www.69shuba.com/txt/1/2">same</a></li>
</ul></div></body></html>`
	if err := os.WriteFile(filepath.Join(root, "source", "book.html"), []byte(bookHTML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "source", "full_catalog.html"), []byte(catalogHTML), 0o644); err != nil {
		t.Fatal(err)
	}
	writeChapter := func(name, title, sourceURL, body string) {
		t.Helper()
		html := BuildChapterHTML(&ChapterContent{Title: title, Content: title + "\n" + body}, sourceURL)
		if err := os.WriteFile(filepath.Join(root, "chapters", name), []byte(html), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeChapter("0001_chapter 1.html", "chapter 1", "https://www.69shuba.com/txt/1/1", "body one")
	writeChapter("0002_chapter 1.html", "chapter 1", "https://www.69shuba.com/txt/1/1", "duplicate")
	writeChapter("0001_same.html", "same", "https://www.69shuba.com/txt/1/2", "wrong prefix")
	writeChapter("0002_same.html", "same", "https://www.69shuba.com/txt/1/2", "body two")
	return root
}
