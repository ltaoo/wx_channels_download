package novelsource

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisteredSourcesParseSyntheticHTML(t *testing.T) {
	sources := AllSources()
	if len(sources) != 26 {
		t.Fatalf("registered sources = %d, want 26", len(sources))
	}
	for _, source := range sources {
		source := source
		t.Run(source.ID, func(t *testing.T) {
			if _, ok := source.ParseURL(source.SampleNovelURL); !ok {
				t.Fatalf("sample novel URL did not parse: %s", source.SampleNovelURL)
			}
			chapterParts, ok := source.ParseURL(source.SampleChapterURL)
			if !ok || chapterParts.Kind != ContentTypeChapter {
				t.Fatalf("sample chapter URL did not parse: %#v", chapterParts)
			}

			bookHTML := syntheticBookHTML(source)
			novel, err := source.ParseNovelHTML(source.SampleNovelURL, bookHTML)
			if err != nil {
				t.Fatalf("ParseNovelHTML: %v", err)
			}
			if novel.Title != source.Name+"测试书" || novel.Author != "测试作者" {
				t.Fatalf("novel metadata = %#v", novel)
			}
			if len(novel.Chapters) == 0 {
				t.Fatalf("chapters len = %d: %#v", len(novel.Chapters), novel.Chapters)
			}

			chapter, err := source.ParseChapterHTML(syntheticChapterHTML())
			if err != nil {
				t.Fatalf("ParseChapterHTML: %v", err)
			}
			if chapter.Title != "第1章 开始" || !strings.Contains(chapter.Content, "第一段正文") {
				t.Fatalf("chapter = %#v", chapter)
			}
		})
	}
}

func TestParseDownloadedFixtures(t *testing.T) {
	fixtureRoot := filepath.Join("..", "..", "..", "scraper_examples")
	for _, source := range AllSources() {
		source := source
		dir := filepath.Join(fixtureRoot, source.ID, "260619")
		bookPath := filepath.Join(dir, "book.html")
		if body, ok := readFixture(t, bookPath); ok {
			novel, err := source.ParseNovelHTML(source.SampleNovelURL, body)
			if err != nil {
				t.Fatalf("%s ParseNovelHTML fixture %s: %v", source.ID, bookPath, err)
			}
			if strings.TrimSpace(novel.Title) == "" {
				t.Fatalf("%s fixture title is empty", source.ID)
			}
		}
		chapterPath := filepath.Join(dir, "chapter.html")
		if body, ok := readFixture(t, chapterPath); ok {
			chapter, err := source.ParseChapterHTML(body)
			if err != nil {
				t.Fatalf("%s ParseChapterHTML fixture %s: %v", source.ID, chapterPath, err)
			}
			if strings.TrimSpace(chapter.Content) == "" {
				t.Fatalf("%s fixture chapter content is empty", source.ID)
			}
		}
	}
}

func readFixture(t *testing.T, path string) (string, bool) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false
		}
		t.Fatal(err)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return "", false
	}
	decoded, err := DecodeHTML(body, "")
	if err != nil {
		t.Fatalf("DecodeHTML %s: %v", path, err)
	}
	return decoded, true
}

func syntheticBookHTML(source Source) string {
	return `<!doctype html><html><head>
<meta charset="utf-8">
<meta property="og:type" content="novel">
<meta property="og:novel:book_name" content="` + source.Name + `测试书">
<meta property="og:novel:author" content="测试作者">
<meta property="og:novel:category" content="测试分类">
<meta property="og:description" content="测试简介">
</head><body>
<div id="list"><dl>
<dd><a href="` + source.SampleChapterURL + `">第1章 开始</a></dd>
<dd><a href="` + source.SampleChapterURL + `?next=1">第2章 继续</a></dd>
</dl></div>
</body></html>`
}

func syntheticChapterHTML() string {
	return `<!doctype html><html><head><meta charset="utf-8"><title>第1章 开始</title></head><body>
<div id="content">
<h1>第1章 开始</h1>
<p>第一段正文。</p>
<p>第二段正文。</p>
</div>
</body></html>`
}
