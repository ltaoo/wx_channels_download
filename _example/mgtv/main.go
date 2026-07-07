package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	mgtvpkg "wx_channel/pkg/scraper/mgtv"
)

const defaultURL = "https://www.mgtv.com/b/648984/23111391.html?_source_=B"

func main() {
	rawURL := flag.String("url", defaultURL, "mgtv play URL")
	timeout := flag.Duration("timeout", 30*time.Second, "request timeout")
	limit := flag.Int("limit", 8, "number of episodes to print")
	dumpJSON := flag.Bool("json", false, "print full profile JSON")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client := mgtvpkg.NewClient()
	profile, err := client.FetchTVProfile(ctx, *rawURL)
	if err != nil {
		log.Fatal(err)
	}
	if profile == nil || profile.ID == "" || profile.Name == "" {
		log.Fatalf("mgtv profile was not fetched: %s", *rawURL)
	}

	if *dumpJSON {
		data, err := json.MarshalIndent(profile, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(data))
		return
	}

	fmt.Println("Source URL:", profile.SourceURL)
	fmt.Println("API URL:", profile.APIURL)
	fmt.Println("Clip ID:", profile.ClipID)
	fmt.Println("Video ID:", profile.VideoID)
	fmt.Println("Title:", profile.Name)
	fmt.Println("Kind:", firstNonEmpty(profile.Kind, "-"))
	fmt.Println("Overview:", singleLine(profile.Overview, 120))
	fmt.Println("Cover:", firstNonEmpty(profile.PosterPath, "-"))
	if profile.CurrentEpisode != nil {
		fmt.Println("Current:", firstNonEmpty(profile.CurrentEpisode.Name, profile.CurrentEpisode.ID), firstNonEmpty(profile.CurrentEpisode.Duration, "-"))
	}

	episodes := episodes(profile)
	fmt.Println("Episodes fetched:", len(episodes))
	max := *limit
	if max <= 0 || max > len(episodes) {
		max = len(episodes)
	}
	for i, episode := range episodes[:max] {
		fmt.Printf("%d. %s %s\n", i+1, firstNonEmpty(episode.Name, episode.ID), firstNonEmpty(episode.Duration, "-"))
		fmt.Printf("   %s\n", firstNonEmpty(episode.URL, "-"))
	}
}

func episodes(profile *mgtvpkg.TVProfile) []mgtvpkg.Episode {
	if profile == nil || len(profile.Seasons) == 0 {
		return nil
	}
	return profile.Seasons[0].Episodes
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func singleLine(value string, max int) string {
	runes := []rune(value)
	if max <= 0 || len(runes) <= max {
		return value
	}
	return string(runes[:max]) + "..."
}
