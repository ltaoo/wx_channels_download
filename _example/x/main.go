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

	xpkg "wx_channel/pkg/scraper/x"
)

type curlAuth struct {
	Cookie      string
	BearerToken string
	GuestToken  string
	CSRFToken   string
}

func main() {
	rawURL := flag.String("url", "https://x.com/Barret_China", "x profile URL")
	count := flag.Int("count", 20, "number of posts requested from UserTweets")
	configFile := flag.String("config", "config.yaml", "optional config file for x.cookie/x.bearer_token/x.guest_token/x.csrf_token")
	curlFile := flag.String("curl-file", "", "optional markdown/shell file containing captured x curl headers")
	cookie := flag.String("cookie", "", "x cookie; falls back to X_COOKIE, curl file, and config x.cookie")
	bearerToken := flag.String("bearer-token", "", "x bearer token; falls back to X_BEARER_TOKEN, curl file, config, and default web bearer")
	guestToken := flag.String("guest-token", "", "x guest token; falls back to X_GUEST_TOKEN, curl file, and config x.guest_token")
	csrfToken := flag.String("csrf-token", "", "x csrf token; falls back to X_CSRF_TOKEN, curl file, config, and ct0 cookie")
	timeout := flag.Duration("timeout", 30*time.Second, "request timeout")
	limit := flag.Int("limit", 5, "number of posts to print")
	flag.Parse()

	fileAuth := curlFileAuth(*curlFile)
	configAuth := configFileAuth(*configFile)
	resolvedCookie := firstNonEmpty(*cookie, os.Getenv("X_COOKIE"), fileAuth.Cookie, configAuth.Cookie)
	resolvedBearer := firstNonEmpty(*bearerToken, os.Getenv("X_BEARER_TOKEN"), fileAuth.BearerToken, configAuth.BearerToken)
	resolvedGuest := firstNonEmpty(*guestToken, os.Getenv("X_GUEST_TOKEN"), fileAuth.GuestToken, configAuth.GuestToken)
	resolvedCSRF := firstNonEmpty(*csrfToken, os.Getenv("X_CSRF_TOKEN"), fileAuth.CSRFToken, configAuth.CSRFToken)

	client := xpkg.NewClientWithOptions(nil, resolvedCookie, resolvedBearer, resolvedGuest, resolvedCSRF, "")
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	target, ok := xpkg.ParseProfileURL(*rawURL)
	if !ok {
		log.Fatalf("unsupported x URL: %s", *rawURL)
	}

	fmt.Println("Request URL:", target.Canonical)
	fmt.Println("Count:", *count)
	fmt.Println("Has cookie:", resolvedCookie != "")
	fmt.Println("Has bearer token:", resolvedBearer != "")
	fmt.Println("Has guest token:", resolvedGuest != "")
	fmt.Println("Has csrf token:", resolvedCSRF != "")

	page, err := client.FetchUserTimeline(ctx, *rawURL, xpkg.TimelineOptions{
		Count:       maxInt(*count, 1),
		Cookie:      resolvedCookie,
		BearerToken: resolvedBearer,
		GuestToken:  resolvedGuest,
		CSRFToken:   resolvedCSRF,
	})
	if err != nil {
		log.Fatal(err)
	}
	if page == nil || len(page.Posts) == 0 {
		log.Fatalf("未请求到 X 时间线: username=%s api=%s", target.Username, timelineAPIURL(page))
	}

	fmt.Println("Canonical:", page.URL.Canonical)
	fmt.Println("API URL:", page.APIURL)
	fmt.Println("User ID:", page.Profile.ID)
	fmt.Println("Username:", page.Profile.Username)
	fmt.Println("Name:", firstNonEmpty(page.Profile.Name, "-"))
	fmt.Println("Followers:", page.Profile.FollowersCount)
	fmt.Println("Following:", page.Profile.FollowingCount)
	fmt.Println("Statuses:", page.Profile.StatusesCount)
	fmt.Println("Posts fetched:", len(page.Posts))
	fmt.Println("Bottom cursor:", firstNonEmpty(page.BottomCursor, "-"))
	for _, warning := range page.Warnings {
		fmt.Println("Warning:", warning)
	}
	fmt.Println()

	maxPosts := *limit
	if maxPosts <= 0 || maxPosts > len(page.Posts) {
		maxPosts = len(page.Posts)
	}
	for i, post := range page.Posts[:maxPosts] {
		fmt.Printf("%d. %s %s\n", i+1, firstNonEmpty(post.CreatedAt, "-"), firstNonEmpty(post.URL, post.ID))
		fmt.Printf("   %s\n", singleLine(post.Text, 160))
		if len(post.ImageURLs) > 0 {
			fmt.Printf("   images: %d first=%s\n", len(post.ImageURLs), post.ImageURLs[0])
		}
		if len(post.VideoURLs) > 0 {
			fmt.Printf("   videos: %d first=%s\n", len(post.VideoURLs), post.VideoURLs[0])
		}
		fmt.Printf("   counts: reply=%d repost=%d quote=%d like=%d view=%d\n", post.ReplyCount, post.RetweetCount, post.QuoteCount, post.FavoriteCount, post.ViewCount)
	}
}

func configFileAuth(configFile string) curlAuth {
	configFile = strings.TrimSpace(configFile)
	if configFile == "" {
		return curlAuth{}
	}
	v := viper.New()
	v.SetConfigFile(configFile)
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			return curlAuth{}
		}
		return curlAuth{}
	}
	return curlAuth{
		Cookie:      strings.TrimSpace(v.GetString("x.cookie")),
		BearerToken: strings.TrimSpace(v.GetString("x.bearer_token")),
		GuestToken:  strings.TrimSpace(v.GetString("x.guest_token")),
		CSRFToken:   firstNonEmpty(v.GetString("x.csrf_token"), v.GetString("x.ct0")),
	}
}

func curlFileAuth(path string) curlAuth {
	path = strings.TrimSpace(path)
	if path == "" {
		return curlAuth{}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return curlAuth{}
	}
	text := string(content)
	return curlAuth{
		Cookie:      firstMatch(text, `(?m)-b '([^']*)'`),
		BearerToken: firstMatch(text, `(?im)-H 'authorization:\s*Bearer ([^']+)'`),
		GuestToken:  firstMatch(text, `(?im)-H 'x-guest-token:\s*([^']+)'`),
		CSRFToken:   firstMatch(text, `(?im)-H 'x-csrf-token:\s*([^']+)'`),
	}
}

func firstMatch(text string, pattern string) string {
	match := regexp.MustCompile(pattern).FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func timelineAPIURL(page *xpkg.TimelinePage) string {
	if page == nil {
		return ""
	}
	return page.APIURL
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

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
