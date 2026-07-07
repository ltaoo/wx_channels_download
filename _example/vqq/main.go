package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	qqpkg "wx_channel/pkg/scraper/qq"
)

const defaultURL = "https://v.qq.com/x/cover/mzc00200whxf2zp/k41026bh3p0.html"

func main() {
	rawURL := flag.String("url", defaultURL, "v.qq.com video detail URL")
	timeout := flag.Duration("timeout", 30*time.Second, "request timeout")
	limit := flag.Int("limit", 8, "number of episodes to print")
	dumpJSON := flag.Bool("json", false, "print parsed detail as JSON")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	page, err := qqpkg.NewClient().FetchVideoDetailPage(ctx, *rawURL)
	if err != nil {
		log.Fatal(err)
	}
	if page == nil || page.Title == "" {
		log.Fatalf("vqq detail was not fetched: %s", *rawURL)
	}

	if *dumpJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(page); err != nil {
			log.Fatal(err)
		}
		return
	}

	fmt.Println("Canonical:", page.URL.Canonical)
	fmt.Println("API URL:", page.APIURL)
	fmt.Println("CID:", page.CID)
	fmt.Println("VID:", page.VID)
	fmt.Println("Title:", page.Title)
	fmt.Println("Description:", firstNonEmpty(page.Description, "-"))
	fmt.Println("Year:", firstNonEmpty(page.Year, "-"))
	fmt.Println("Area:", firstNonEmpty(page.AreaName, "-"))
	fmt.Println("Genres:", firstNonEmpty(join(page.Genres, " / "), "-"))
	fmt.Println("Score:", firstNonEmpty(page.Score, "-"))
	fmt.Println("Detail:", firstNonEmpty(page.DetailInfo, "-"))
	fmt.Println("Cover:", firstNonEmpty(page.CoverURL, "-"))
	if page.CurrentEpisode != nil {
		fmt.Println("Current:", firstNonEmpty(page.CurrentEpisode.PlayTitle, page.CurrentEpisode.VID))
	}
	fmt.Printf("Episodes parsed: %d", len(page.Episodes))
	if page.EpisodeAll > 0 {
		fmt.Printf(" / %d", page.EpisodeAll)
	}
	fmt.Println()

	maxEpisodes := *limit
	if maxEpisodes <= 0 || maxEpisodes > len(page.Episodes) {
		maxEpisodes = len(page.Episodes)
	}
	for i, episode := range page.Episodes[:maxEpisodes] {
		fmt.Printf("%d. %s %s\n", i+1, firstNonEmpty(episode.PlayTitle, episode.Title, episode.VID), episode.URL)
		if episode.Subtitle != "" {
			fmt.Printf("   %s\n", episode.Subtitle)
		}
		fmt.Printf("   vid=%s duration=%ds trailer=%v\n", episode.VID, episode.Duration, episode.IsTrailer)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func join(values []string, sep string) string {
	out := ""
	for _, value := range values {
		if value == "" {
			continue
		}
		if out != "" {
			out += sep
		}
		out += value
	}
	return out
}
