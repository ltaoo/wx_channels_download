package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	qidianpkg "wx_channel/pkg/scraper/qidian"
)

const defaultURL = "https://www.qidian.com/book/1035420986/"

func main() {
	rawURL := flag.String("url", defaultURL, "qidian book URL")
	htmlFile := flag.String("html-file", "", "optional local qidian book HTML fixture")
	cookie := flag.String("cookie", "", "optional qidian cookie; falls back to QIDIAN_COOKIE")
	timeout := flag.Duration("timeout", 30*time.Second, "request timeout")
	limit := flag.Int("limit", 8, "number of chapters to print")
	dumpJSON := flag.Bool("json", false, "print parsed profile as JSON")
	flag.Parse()

	parts, ok := qidianpkg.ParseURL(*rawURL)
	if !ok {
		log.Fatalf("unsupported qidian URL: %s", *rawURL)
	}

	var profile *qidianpkg.BookProfile
	var err error
	if strings.TrimSpace(*htmlFile) != "" {
		body, readErr := os.ReadFile(*htmlFile)
		if readErr != nil {
			log.Fatal(readErr)
		}
		profile, err = qidianpkg.ParseBookProfile(parts.Canonical, body)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()
		client := qidianpkg.NewClient(&http.Client{Timeout: *timeout})
		client.Cookie = firstNonEmpty(*cookie, os.Getenv("QIDIAN_COOKIE"))
		profile, err = client.FetchBookProfileContext(ctx, parts.BookID)
	}
	if err != nil {
		log.Fatal(err)
	}
	if profile == nil || profile.Title == "" || profile.Author.Name == "" || profile.ChapterCount == 0 {
		log.Fatalf("未抓取到起点作品信息: %s", parts.Canonical)
	}

	if *dumpJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(profile); err != nil {
			log.Fatal(err)
		}
		return
	}

	fmt.Println("Canonical:", firstNonEmpty(profile.URL, parts.Canonical))
	fmt.Println("Book ID:", parts.BookID)
	fmt.Println("Title:", profile.Title)
	fmt.Println("Author:", profile.Author.Name)
	fmt.Println("Author ID:", firstNonEmpty(profile.Author.ID, "-"))
	fmt.Println("Category:", joinNonEmpty(" / ", profile.Category, profile.SubCategory))
	fmt.Println("Status:", firstNonEmpty(profile.Status, "-"))
	fmt.Println("Words:", firstNonEmpty(profile.DisplayWordCount, fmt.Sprint(profile.WordCount)))
	fmt.Println("Chapters:", profile.ChapterCount)
	fmt.Println("Latest:", firstNonEmpty(profile.LatestChapter.Title, "-"))
	fmt.Println("Cover:", firstNonEmpty(profile.CoverURL, "-"))
	fmt.Println()

	printed := 0
	for _, volume := range profile.Volumes {
		if printed >= *limit && *limit > 0 {
			break
		}
		fmt.Println("Volume:", firstNonEmpty(volume.Title, fmt.Sprint(volume.Idx)))
		for _, chapter := range volume.Chapters {
			if *limit > 0 && printed >= *limit {
				break
			}
			printed++
			fmt.Printf("%d. %s\n", chapter.Idx, chapter.Title)
			fmt.Printf("   %s\n", chapter.URL)
		}
	}
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

func joinNonEmpty(sep string, values ...string) string {
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return "-"
	}
	return strings.Join(out, sep)
}
