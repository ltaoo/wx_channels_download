package source

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"wx_channel/pkg/scraper"

	"github.com/PuerkitoBio/goquery"
)

// Page metadata augments playback metadata. Failure does not discard a playable
// source, and absence of a type never turns a single episode into a movie.
func MergeProfile(target *scraper.Profile, source scraper.Profile) {
	if target.ID == "" {
		target.ID = source.ID
	}
	if source.Kind != "" && source.Kind != "video" {
		target.Kind = source.Kind
	}
	if source.Title != "" {
		target.Title = source.Title
	}
	if source.Description != "" {
		target.Description = source.Description
	}
	if source.CoverURL != "" {
		target.CoverURL = source.CoverURL
	}
	if source.ReleaseDate != "" {
		target.ReleaseDate = source.ReleaseDate
	}
	if source.EpisodeCount > 0 {
		target.EpisodeCount = source.EpisodeCount
	}
	if len(source.Metadata) > 0 {
		target.Metadata = source.Metadata
	}
}

// ParsePageMetadata reads public JSON-LD and Open Graph movie/series metadata.
// The selected structured node is retained for credits, genres and identifiers.
func ParsePageMetadata(page []byte) scraper.Profile {
	profile := scraper.Profile{Kind: "video"}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(string(page)))
	if err != nil {
		return profile
	}
	meta := func(property string) string {
		value, _ := document.Find(`meta[property="` + property + `"],meta[name="` + property + `"]`).First().Attr("content")
		return strings.TrimSpace(value)
	}
	profile.Title = CleanText(meta("og:title"))
	profile.Description = CleanText(FirstText(meta("og:description"), meta("description")))
	profile.CoverURL = PublicImage(meta("og:image"))
	switch meta("og:type") {
	case "video.movie":
		profile.Kind = "movie"
	case "video.tv_show":
		profile.Kind = "series"
	}
	document.Find(`script[type="application/ld+json"]`).Each(func(_ int, node *goquery.Selection) {
		var value any
		decoder := json.NewDecoder(strings.NewReader(node.Text()))
		decoder.UseNumber()
		if decoder.Decode(&value) != nil {
			return
		}
		var walk func(any, int)
		walk = func(value any, depth int) {
			if depth > 12 {
				return
			}
			switch entry := value.(type) {
			case []any:
				for _, child := range entry {
					walk(child, depth+1)
				}
			case map[string]any:
				kind := ""
				types := []any{entry["@type"]}
				if list, ok := entry["@type"].([]any); ok {
					types = list
				}
				for _, raw_type := range types {
					switch raw_type {
					case "Movie":
						kind = "movie"
					case "TVSeries", "TVEpisode":
						kind = "series"
					}
				}
				if kind != "" {
					source := scraper.Profile{Kind: kind, Title: json_text(entry["name"]), Description: json_text(entry["description"]), ReleaseDate: json_text(entry["datePublished"])}
					switch image := entry["image"].(type) {
					case string:
						source.CoverURL = PublicImage(image)
					case map[string]any:
						source.CoverURL = PublicImage(json_text(image["url"]))
					case []any:
						if len(image) > 0 {
							source.CoverURL = PublicImage(json_text(image[0]))
						}
					}
					if count, ok := entry["numberOfEpisodes"].(json.Number); ok {
						source.EpisodeCount, _ = strconv.Atoi(string(count))
					}
					source.Metadata, _ = json.Marshal(entry)
					if parent, ok := entry["partOfSeries"].(map[string]any); ok {
						source.Title = FirstText(json_text(parent["name"]), source.Title)
					}
					MergeProfile(&profile, source)
				}
				if graph, ok := entry["@graph"]; ok {
					walk(graph, depth+1)
				}
			}
		}
		walk(value, 0)
	})
	return profile
}

func json_text(value any) string {
	if text, ok := value.(string); ok {
		return CleanText(text)
	}
	return ""
}
func PublicImage(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return ""
	}
	return parsed.String()
}
