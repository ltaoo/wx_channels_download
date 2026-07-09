package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	iqiyipkg "wx_channel/pkg/scraper/iqiyi"
)

const defaultURL = "https://www.iqiyi.com/v_2dkhwocyjk4.html"

func main() {
	rawURL := flag.String("url", defaultURL, "iqiyi play page URL")
	cookie := flag.String("cookie", "", "iqiyi cookie; falls back to IQIYI_COOKIE")
	timeout := flag.Duration("timeout", 30*time.Second, "request timeout")
	limit := flag.Int("limit", 5, "number of episodes to print")
	flag.Parse()

	resolvedCookie := firstNonEmpty(*cookie, os.Getenv("IQIYI_COOKIE"))
	client := iqiyipkg.NewClient(iqiyipkg.WithCookie(resolvedCookie))
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	fmt.Println("Request URL:", *rawURL)
	fmt.Println("Has cookie:", resolvedCookie != "")
	if tvid, ok := iqiyipkg.ParseTVID(*rawURL); ok {
		fmt.Println("TVID:", tvid)
	}

	profile, err := client.FetchProfileWithSeasons(ctx, *rawURL)
	if err != nil {
		log.Fatal(err)
	}
	if profile == nil || profile.ID == 0 || strings.TrimSpace(profile.Name) == "" {
		log.Fatalf("未抓取到爱奇艺内容数据: %s", *rawURL)
	}
	episodeCount := countEpisodes(profile.Seasons)
	if episodeCount == 0 {
		log.Fatalf("未抓取到爱奇艺剧集数据: id=%d title=%s", profile.ID, profile.Name)
	}

	fmt.Println("Content ID:", profile.ID)
	fmt.Println("Title:", profile.Name)
	fmt.Println("Type:", profile.Type)
	fmt.Println("Poster:", firstNonEmpty(profile.PosterPath, "-"))
	fmt.Println("Overview:", singleLine(profile.Overview, 160))
	fmt.Println("Seasons:", len(profile.Seasons))
	fmt.Println("Episodes:", episodeCount)
	fmt.Println()

	printed := 0
	for _, season := range profile.Seasons {
		if len(profile.Seasons) > 1 {
			fmt.Println("Season:", firstNonEmpty(season.Name, fmt.Sprint(season.ID)))
		}
		for _, episode := range season.Episodes {
			if *limit > 0 && printed >= *limit {
				return
			}
			printed++
			fmt.Printf("%d. #%d %s\n", printed, episode.EpisodeNumber, firstNonEmpty(episode.Name, "-"))
			fmt.Printf("   %s\n", firstNonEmpty(episode.ID, "-"))
			if episode.Thumbnail != "" {
				fmt.Printf("   thumbnail: %s\n", episode.Thumbnail)
			}
		}
	}
}

func countEpisodes(seasons []iqiyipkg.Season) int {
	total := 0
	for _, season := range seasons {
		total += len(season.Episodes)
	}
	return total
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

func singleLine(value string, max int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if max <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max]) + "..."
}
