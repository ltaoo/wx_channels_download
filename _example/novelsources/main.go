package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wx_channel/pkg/scraper/novelsource"
)

func main() {
	sourceID := flag.String("source", "all", "source id to verify, or all")
	rawURL := flag.String("url", "", "override novel URL when verifying one live source")
	fixtureRoot := flag.String("fixtures", "scraper_examples", "fixture root directory")
	live := flag.Bool("live", false, "fetch live pages instead of local fixtures")
	timeout := flag.Duration("timeout", 30*time.Second, "live request timeout")
	flag.Parse()

	sources := selectSources(*sourceID)
	if len(sources) == 0 {
		log.Fatalf("unknown source: %s", *sourceID)
	}
	for _, source := range sources {
		if *live {
			verifyLive(source, *rawURL, *timeout)
			continue
		}
		verifyFixture(source, *fixtureRoot)
	}
}

func selectSources(id string) []novelsource.Source {
	id = strings.TrimSpace(id)
	if id == "" || id == "all" {
		return novelsource.AllSources()
	}
	source, ok := novelsource.SourceByID(id)
	if !ok {
		return nil
	}
	return []novelsource.Source{source}
}

func verifyFixture(source novelsource.Source, fixtureRoot string) {
	dir := filepath.Join(fixtureRoot, source.ID, "260619")
	bookPath := filepath.Join(dir, "book.html")
	chapterPath := filepath.Join(dir, "chapter.html")
	fmt.Printf("[%s] %s\n", source.ID, source.Name)

	if body, ok := readFixture(bookPath); ok {
		novel, err := source.ParseNovelHTML(source.SampleNovelURL, body)
		if err != nil {
			fmt.Printf("  book: parse failed: %v\n", err)
		} else {
			fmt.Printf("  book: %s by %s, chapters=%d\n", firstNonEmpty(novel.Title, "-"), firstNonEmpty(novel.Author, "-"), len(novel.Chapters))
		}
	} else {
		fmt.Printf("  book: fixture missing (%s)\n", bookPath)
	}

	if body, ok := readFixture(chapterPath); ok {
		chapter, err := source.ParseChapterHTML(body)
		if err != nil {
			fmt.Printf("  chapter: parse failed: %v\n", err)
		} else {
			fmt.Printf("  chapter: %s, text=%d chars\n", firstNonEmpty(chapter.Title, "-"), len([]rune(chapter.Content)))
		}
	} else {
		fmt.Printf("  chapter: fixture missing (%s)\n", chapterPath)
	}
}

func verifyLive(source novelsource.Source, overrideURL string, timeout time.Duration) {
	target := firstNonEmpty(overrideURL, source.SampleNovelURL)
	client := novelsource.NewClient(source, &http.Client{Timeout: timeout})
	fmt.Printf("[%s] %s\n", source.ID, target)
	novel, err := client.FetchNovelChapters(target)
	if err != nil {
		fmt.Printf("  book: fetch failed: %v\n", err)
		return
	}
	fmt.Printf("  book: %s by %s, chapters=%d\n", firstNonEmpty(novel.Title, "-"), firstNonEmpty(novel.Author, "-"), len(novel.Chapters))
	if len(novel.Chapters) == 0 {
		return
	}
	chapter, err := client.FetchChapterContent(novel.Chapters[0].URL)
	if err != nil {
		fmt.Printf("  chapter: fetch failed: %v\n", err)
		return
	}
	fmt.Printf("  chapter: %s, text=%d chars\n", firstNonEmpty(chapter.Title, "-"), len([]rune(chapter.Content)))
}

func readFixture(path string) (string, bool) {
	body, err := os.ReadFile(path)
	if err != nil || len(strings.TrimSpace(string(body))) == 0 {
		return "", false
	}
	decoded, err := novelsource.DecodeHTML(body, "")
	if err != nil {
		return "", false
	}
	return decoded, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
