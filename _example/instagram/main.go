package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"

	instagrampkg "wx_channel/pkg/scraper/instagram"
)

func main() {
	rawURL := flag.String("url", "https://www.instagram.com/r_ap82_/", "instagram profile URL")
	configFile := flag.String("config", "config.yaml", "optional config file for instagram.cookie")
	cookie := flag.String("cookie", "", "instagram cookie; falls back to INSTAGRAM_COOKIE and config instagram.cookie")
	appID := flag.String("app-id", "", "instagram X-IG-App-ID; defaults to page/default app id")
	timeout := flag.Duration("timeout", 30*time.Second, "request timeout")
	limit := flag.Int("limit", 5, "number of posts to print")
	flag.Parse()

	resolvedCookie := firstNonEmpty(*cookie, os.Getenv("INSTAGRAM_COOKIE"), configCookie(*configFile))
	client := instagrampkg.NewClientWithOptions(nil, resolvedCookie, *appID, "")
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	target, ok := instagrampkg.ParseProfileURL(*rawURL)
	if !ok {
		log.Fatalf("unsupported instagram URL: %s", *rawURL)
	}

	fmt.Println("Request URL:", target.Canonical)
	fmt.Println("Has cookie:", resolvedCookie != "")

	page, err := client.FetchUserProfile(ctx, *rawURL, instagrampkg.ProfileOptions{
		Count:  maxInt(*limit, 1),
		Cookie: resolvedCookie,
		AppID:  *appID,
	})
	if err != nil {
		log.Fatal(err)
	}
	if page == nil || page.Profile.Username == "" {
		log.Fatalf("instagram profile was not fetched: %s", target.Canonical)
	}

	fmt.Println("Canonical:", page.URL.Canonical)
	fmt.Println("API URL:", page.APIURL)
	fmt.Println("App ID:", page.AppID)
	fmt.Println("User ID:", page.Profile.ID)
	fmt.Println("Username:", page.Profile.Username)
	fmt.Println("Full name:", firstNonEmpty(page.Profile.FullName, "-"))
	fmt.Println("Followers:", page.Profile.FollowersCount)
	fmt.Println("Following:", page.Profile.FollowingCount)
	fmt.Println("Media count:", page.Profile.MediaCount)
	fmt.Println("Posts fetched:", len(page.Posts))
	for _, warning := range page.Warnings {
		fmt.Println("Warning:", warning)
	}
	fmt.Println()

	maxPosts := *limit
	if maxPosts <= 0 || maxPosts > len(page.Posts) {
		maxPosts = len(page.Posts)
	}
	for i, post := range page.Posts[:maxPosts] {
		fmt.Printf("%d. %s %s\n", i+1, firstNonEmpty(post.Shortcode, post.ID), firstNonEmpty(post.URL, "-"))
		fmt.Printf("   %s\n", singleLine(post.Caption, 160))
		fmt.Printf("   media: image=%s video=%v likes=%d comments=%d\n", firstNonEmpty(post.DisplayURL, post.ThumbnailURL, "-"), post.IsVideo, post.LikeCount, post.CommentCount)
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
	return strings.TrimSpace(v.GetString("instagram.cookie"))
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
