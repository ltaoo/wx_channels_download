package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/viper"

	weibopkg "wx_channel/pkg/scraper/weibo"
)

func main() {
	rawURL := flag.String("url", "https://weibo.com/u/1926245291", "weibo user URL")
	page := flag.Int("page", 1, "mymblog page number")
	feature := flag.Int("feature", 0, "mymblog feature parameter")
	configFile := flag.String("config", "config.yaml", "optional config file for weibo.cookie")
	curlFile := flag.String("curl-file", "", "optional markdown/shell file containing curl -b 'cookie'")
	cookie := flag.String("cookie", "", "weibo cookie; falls back to WEIBO_COOKIE and config weibo.cookie")
	timeout := flag.Duration("timeout", 30*time.Second, "request timeout")
	limit := flag.Int("limit", 5, "number of posts to print")
	flag.Parse()

	resolvedCookie := firstNonEmpty(*cookie, os.Getenv("WEIBO_COOKIE"), curlFileCookie(*curlFile), configCookie(*configFile))
	client := weibopkg.NewClientWithOptions(nil, resolvedCookie, "")
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	target, ok := weibopkg.ParseUserURL(*rawURL)
	if !ok {
		log.Fatalf("unsupported weibo URL: %s", *rawURL)
	}

	fmt.Println("Request URL:", target.Canonical)
	fmt.Println("API page:", *page)
	fmt.Println("Has cookie:", resolvedCookie != "")

	timeline, err := client.FetchUserTimeline(ctx, *rawURL, weibopkg.TimelineOptions{
		Page:    *page,
		Feature: *feature,
	})
	if err != nil {
		log.Fatal(err)
	}
	if timeline == nil || len(timeline.Response.Data.List) == 0 {
		log.Fatalf("未请求到微博列表: uid=%s api=%s", target.UID, timelineAPIURL(timeline))
	}

	fmt.Println("API URL:", timeline.APIURL)
	fmt.Println("UID:", timeline.URL.UID)
	fmt.Println("Author:", firstNonEmpty(timeline.User.ScreenName, timeline.User.IDStr))
	fmt.Println("List count:", len(timeline.Response.Data.List))
	fmt.Println("Summary count:", len(timeline.Posts))
	fmt.Println("Total:", timeline.Total)
	fmt.Println("Since ID:", timeline.SinceID)
	fmt.Println()

	maxPosts := *limit
	if maxPosts <= 0 || maxPosts > len(timeline.Posts) {
		maxPosts = len(timeline.Posts)
	}
	for i, post := range timeline.Posts[:maxPosts] {
		fmt.Printf("%d. %s %s\n", i+1, firstNonEmpty(post.CreatedAt, "-"), firstNonEmpty(post.URL, post.ID))
		fmt.Printf("   %s\n", singleLine(post.Text, 160))
		if len(post.PicURLs) > 0 {
			fmt.Printf("   images: %d first=%s\n", len(post.PicURLs), post.PicURLs[0])
		}
		fmt.Printf("   counts: repost=%d comment=%d like=%d\n", post.RepostsCount, post.CommentsCount, post.AttitudesCount)
	}
}

func configCookie(configFile string) string {
	configFile = strings.TrimSpace(configFile)
	if configFile == "" {
		return ""
	}
	v := viper.New()
	v.SetConfigFile(configFile)
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			return ""
		}
		return ""
	}
	return strings.TrimSpace(v.GetString("weibo.cookie"))
}

func curlFileCookie(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	match := regexp.MustCompile(`(?m)-b '([^']*)'`).FindSubmatch(content)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(string(match[1]))
}

func timelineAPIURL(timeline *weibopkg.TimelinePage) string {
	if timeline == nil {
		return ""
	}
	return timeline.APIURL
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
