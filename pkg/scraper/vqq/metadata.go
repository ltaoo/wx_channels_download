package vqq

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"wx_channel/pkg/scraper"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

type tencent_card struct {
	Type     string            `json:"type"`
	Params   map[string]string `json:"params"`
	Children map[string]struct {
		Cards []struct {
			Params map[string]string `json:"params"`
		} `json:"cards"`
	} `json:"children_list"`
}

func tencent_profile(cover_id string, card tencent_card) (scraper.Profile, error) {
	params := make(map[string]string)
	for key, value := range card.Params {
		params[key] = value
	}
	keys := make([]string, 0, len(card.Children))
	for key := range card.Children {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, item := range card.Children[key].Cards {
			if cid := item.Params["report.cid"]; cid != "" && cid != cover_id {
				return scraper.Profile{}, errors.New("腾讯内容资料的剧集编号与请求不匹配")
			}
			for key, value := range item.Params {
				if value != "" {
					params[key] = value
				}
			}
		}
	}
	profile := scraper.Profile{ID: cover_id, Kind: "video", Homepage: "https://v.qq.com/x/cover/" + cover_id + ".html",
		Title:       sourceutil.CleanText(sourceutil.FirstText(params["title"], params["mod_title"])),
		Description: sourceutil.CleanText(sourceutil.FirstText(params["cover_description"], params["introduction"], params["description"])),
		CoverURL:    sourceutil.PublicImage(sourceutil.FirstText(params["new_pic_vt"], params["cover_image_url"])),
		BackdropURL: sourceutil.PublicImage(params["new_pic_hz"]), Tips: sourceutil.CleanText(params["detail_info"]),
	}
	switch sourceutil.FirstText(params["type_name"], params["category_name"], params["video_type_name"], params["type"]) {
	case "电影", "movie", "1":
		profile.Kind = "movie"
	case "电视剧", "动漫", "综艺", "纪录片", "series", "2", "3", "9", "10":
		profile.Kind = "series"
	}
	if total, err := strconv.Atoi(params["episode_all"]); err == nil && total > 0 && total <= 10000 {
		profile.EpisodeCount = total
	}
	if year, err := strconv.Atoi(sourceutil.FirstText(params["year"], params["cover_year"], params["show_year"])); err == nil && year >= 1800 && year <= 2200 {
		profile.ReleaseYear = year
	}
	if date, err := time.Parse("2006-01-02", params["broadcast_time"]); err == nil {
		profile.ReleaseDate = date.Format("2006-01-02")
	}
	profile.Genres = metadata_labels(params["main_genres"], params["sub_genre"])
	profile.Countries = metadata_labels(params["area_name"])
	// Retain only content fields, excluding reporting and per-request context.
	profile.Metadata, _ = json.Marshal(map[string]string{"tencent_cover_id": cover_id, "type": params["type"]})
	return profile, nil
}

func metadata_labels(values ...string) []string {
	labels := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		for _, label := range strings.FieldsFunc(sourceutil.CleanText(value), func(char rune) bool {
			return char == ',' || char == '，' || char == '/' || char == '、' || char == ';' || char == '；'
		}) {
			label = strings.TrimSpace(label)
			if label != "" && !seen[label] {
				labels = append(labels, label)
				seen[label] = true
			}
		}
	}
	return labels
}
