package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wx_channel/pkg/scraper/ciweimao"
	"wx_channel/pkg/scraper/fanqienovel"
	"wx_channel/pkg/scraper/jjwxc"
	"wx_channel/pkg/scraper/novelprofile"
	"wx_channel/pkg/scraper/sfacg"
	"wx_channel/pkg/scraper/zongheng"
)

type summary struct {
	Platform     string
	URL          string
	Title        string
	Author       string
	Description  string
	ChapterCount int
	Latest       string
}

type target struct {
	platform   string
	defaultURL string
	fixture    string
	parse      func(string, []byte) (summary, error)
	fetch      func(context.Context, string) (summary, error)
}

func main() {
	platform := flag.String("platform", "all", "platform: all, zongheng, jjwxc, sfacg, ciweimao, fanqienovel")
	fixtures := flag.String("fixtures", "scraper_examples", "fixture root")
	live := flag.Bool("live", false, "fetch live pages instead of reading fixtures")
	urlOverride := flag.String("url", "", "override URL for a single platform")
	flag.Parse()

	targets := allTargets()
	selected, err := selectTargets(targets, *platform)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *urlOverride != "" && len(selected) != 1 {
		fmt.Fprintln(os.Stderr, "-url requires a single -platform")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	for _, item := range selected {
		reqURL := item.defaultURL
		if *urlOverride != "" {
			reqURL = *urlOverride
		}
		var got summary
		if *live {
			got, err = item.fetch(ctx, reqURL)
		} else {
			var body []byte
			body, err = os.ReadFile(filepath.Join(*fixtures, item.fixture))
			if err == nil {
				got, err = item.parse(reqURL, body)
			}
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", item.platform, err)
			os.Exit(1)
		}
		if err := validate(got); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", item.platform, err)
			os.Exit(1)
		}
		fmt.Printf("%-12s title=%s author=%s chapters=%d latest=%s\n", got.Platform, got.Title, got.Author, got.ChapterCount, got.Latest)
	}
}

func allTargets() []target {
	return []target{
		{
			platform:   zongheng.PlatformID,
			defaultURL: "https://www.zongheng.com/detail/672340",
			fixture:    "zongheng/260619/book.html",
			parse: func(reqURL string, body []byte) (summary, error) {
				profile, err := zongheng.ParseBookProfile(reqURL, body)
				return fromProfile(zongheng.PlatformID, profile), err
			},
			fetch: func(ctx context.Context, reqURL string) (summary, error) {
				profile, err := zongheng.NewClient(nil).FetchBookProfileContext(ctx, reqURL)
				return fromProfile(zongheng.PlatformID, profile), err
			},
		},
		{
			platform:   jjwxc.PlatformID,
			defaultURL: "https://m.jjwxc.net/book2/245452",
			fixture:    "jjwxc/260619/book.html",
			parse: func(reqURL string, body []byte) (summary, error) {
				profile, err := jjwxc.ParseBookProfile(reqURL, body)
				return fromProfile(jjwxc.PlatformID, profile), err
			},
			fetch: func(ctx context.Context, reqURL string) (summary, error) {
				profile, err := jjwxc.NewClient(nil).FetchBookProfileContext(ctx, reqURL)
				return fromProfile(jjwxc.PlatformID, profile), err
			},
		},
		{
			platform:   sfacg.PlatformID,
			defaultURL: "https://book.sfacg.com/Novel/672419/",
			fixture:    "sfacg/260619/book.html",
			parse: func(reqURL string, body []byte) (summary, error) {
				profile, err := sfacg.ParseBookProfile(reqURL, body)
				return fromProfile(sfacg.PlatformID, profile), err
			},
			fetch: func(ctx context.Context, reqURL string) (summary, error) {
				profile, err := sfacg.NewClient(nil).FetchBookProfileContext(ctx, reqURL)
				return fromProfile(sfacg.PlatformID, profile), err
			},
		},
		{
			platform:   ciweimao.PlatformID,
			defaultURL: "https://www.ciweimao.com/book/100337734",
			fixture:    "ciweimao/260619/book.html",
			parse: func(reqURL string, body []byte) (summary, error) {
				profile, err := ciweimao.ParseBookProfile(reqURL, body)
				return fromProfile(ciweimao.PlatformID, profile), err
			},
			fetch: func(ctx context.Context, reqURL string) (summary, error) {
				profile, err := ciweimao.NewClient(nil).FetchBookProfileContext(ctx, reqURL)
				return fromProfile(ciweimao.PlatformID, profile), err
			},
		},
		{
			platform:   "fanqienovel",
			defaultURL: "https://fanqienovel.com/page/7069948840148732967",
			fixture:    "fanqienovel/260614/fanqienovel_260614.html",
			parse: func(reqURL string, body []byte) (summary, error) {
				profile, err := fanqienovel.ParseBookProfileHTML(reqURL, string(body))
				return fromFanqie(profile), err
			},
			fetch: func(ctx context.Context, reqURL string) (summary, error) {
				htmlText, err := novelprofile.FetchHTML(ctx, nil, reqURL, "https://fanqienovel.com/", novelprofile.DefaultUserAgent, "")
				if err != nil {
					return summary{}, err
				}
				profile, err := fanqienovel.ParseBookProfileHTML(reqURL, htmlText)
				return fromFanqie(profile), err
			},
		},
	}
}

func selectTargets(targets []target, platform string) ([]target, error) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" || platform == "all" {
		return targets, nil
	}
	for _, item := range targets {
		if item.platform == platform {
			return []target{item}, nil
		}
	}
	return nil, fmt.Errorf("unknown platform %q", platform)
}

func fromProfile(platform string, profile *novelprofile.BookProfile) summary {
	if profile == nil {
		return summary{Platform: platform}
	}
	return summary{
		Platform:     platform,
		URL:          profile.URL,
		Title:        profile.Title,
		Author:       profile.Author.Name,
		Description:  profile.Description,
		ChapterCount: profile.ChapterCount,
		Latest:       profile.LatestChapter.Title,
	}
}

func fromFanqie(profile *fanqienovel.BookProfile) summary {
	if profile == nil {
		return summary{Platform: "fanqienovel"}
	}
	return summary{
		Platform:     "fanqienovel",
		URL:          profile.URL,
		Title:        profile.Title,
		Author:       profile.Author.Name,
		Description:  profile.Description,
		ChapterCount: profile.ChapterCount,
		Latest:       profile.LatestChapter.Title,
	}
}

func validate(got summary) error {
	switch {
	case strings.TrimSpace(got.Title) == "":
		return fmt.Errorf("missing title")
	case strings.TrimSpace(got.Author) == "":
		return fmt.Errorf("missing author")
	case strings.TrimSpace(got.Description) == "":
		return fmt.Errorf("missing description")
	case got.ChapterCount <= 0:
		return fmt.Errorf("missing chapters")
	default:
		return nil
	}
}
