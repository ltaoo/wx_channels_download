package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"wx_channel/pkg/clawreq"
)

func main() {
	rawURL := flag.String("url", "https://www.69shuba.com/book/34567/", "69shuba URL")
	profile := flag.String("profile", string(clawreq.ProfileChrome), "fingerprint profile: chrome, firefox, safari, safari-ios, random")
	timeout := flag.Duration("timeout", 30*time.Second, "request timeout")
	out := flag.String("out", "", "optional path to save decoded HTML")
	flag.Parse()

	client, err := clawreq.New(clawreq.Config{
		Profile:         clawreq.Profile(strings.TrimSpace(*profile)),
		Timeout:         *timeout,
		FollowRedirects: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.CloseIdleConnections()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	resp, err := client.Get(ctx, *rawURL)
	if err != nil {
		log.Fatal(err)
	}
	text, err := resp.Text()
	if err != nil {
		log.Fatal(err)
	}

	chapterLinks := strings.Count(text, "/txt/34567/")
	challenge := containsAny(strings.ToLower(text), []string{
		"just a moment",
		"checking your browser",
		"cf-chl",
		"cf-ray",
		"cloudflare",
		"verify you are human",
		"attention required",
	})

	fmt.Println("url:", *rawURL)
	fmt.Println("profile:", *profile)
	fmt.Println("status:", resp.Status)
	fmt.Println("final_url:", resp.FinalURL)
	fmt.Println("content_type:", resp.ContentType())
	fmt.Println("bytes:", len(resp.Body))
	fmt.Println("decoded_chars:", len([]rune(text)))
	fmt.Println("chapter_href_count:", chapterLinks)
	fmt.Println("cloudflare_challenge:", challenge)
	if title := between(text, "<title>", "</title>"); title != "" {
		fmt.Println("title:", strings.TrimSpace(title))
	}

	if strings.TrimSpace(*out) != "" {
		if err := os.WriteFile(*out, []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Println("saved:", *out)
	}
}

func containsAny(text string, values []string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}

func between(text string, start string, end string) string {
	idx := strings.Index(text, start)
	if idx < 0 {
		return ""
	}
	idx += len(start)
	rest := text[idx:]
	endIdx := strings.Index(rest, end)
	if endIdx < 0 {
		return ""
	}
	return rest[:endIdx]
}
