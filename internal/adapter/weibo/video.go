package weiboadapter

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"wx_channel/internal/adapter"
	"wx_channel/internal/database/model"
)

func parse_video(article *goquery.Selection) *Video {
	if article == nil {
		return nil
	}
	raw_url := strings.TrimSpace(article.Find("video[src]").First().AttrOr("src", ""))
	if raw_url == "" {
		raw_url = strings.TrimSpace(article.Find("video source[src]").First().AttrOr("src", ""))
	}
	video_url := normalize_video_url(raw_url)
	if video_url == "" {
		return nil
	}
	video_element := article.Find("video[src]").First()
	cover_url := normalize_weibo_image_url(video_element.AttrOr("poster", ""))
	if cover_url == "" {
		cover_url = normalize_weibo_image_url(article.Find(".vjs-poster img[src]").First().AttrOr("src", ""))
	}
	parsed_url, _ := url.Parse(video_url)
	query := parsed_url.Query()
	template := strings.TrimSpace(query.Get("template"))
	width, height, fps := video_template_dimensions(template)
	return &Video{
		URL:       video_url,
		CoverURL:  cover_url,
		MediaID:   strings.TrimSpace(query.Get("media_id")),
		Quality:   strings.TrimSpace(query.Get("label")),
		Template:  template,
		Duration:  parse_video_duration(article.Find(".vjs-duration-display").First().Text()),
		Width:     width,
		Height:    height,
		FPS:       fps,
		ExpiresAt: video_url_expires_at(parsed_url),
	}
}

func normalize_video_url(raw_url string) string {
	raw_url = strings.TrimSpace(raw_url)
	if strings.HasPrefix(raw_url, "//") {
		raw_url = "https:" + raw_url
	}
	parsed_url, err := url.Parse(raw_url)
	if err != nil || !strings.EqualFold(parsed_url.Scheme, "https") {
		return ""
	}
	host := strings.ToLower(strings.TrimSuffix(parsed_url.Hostname(), "."))
	if !host_matches(host, "weibocdn.com") && !host_matches(host, "sinaimg.cn") && !host_matches(host, "sinaimg.com") {
		return ""
	}
	parsed_url.Fragment = ""
	return parsed_url.String()
}

func normalize_weibo_image_url(raw_url string) string {
	raw_url = strings.TrimSpace(raw_url)
	if strings.HasPrefix(raw_url, "//") {
		raw_url = "https:" + raw_url
	}
	parsed_url, err := url.Parse(raw_url)
	if err != nil || !strings.EqualFold(parsed_url.Scheme, "https") {
		return ""
	}
	host := strings.ToLower(strings.TrimSuffix(parsed_url.Hostname(), "."))
	if !host_matches(host, "sinaimg.cn") && !host_matches(host, "sinaimg.com") {
		return ""
	}
	parsed_url.Fragment = ""
	return parsed_url.String()
}

func host_matches(host string, root string) bool {
	return host == root || strings.HasSuffix(host, "."+root)
}

func video_template_dimensions(template string) (int, int, int) {
	parts := strings.Split(strings.TrimSpace(template), ".")
	dimensions := strings.Split(parts[0], "x")
	if len(dimensions) != 2 {
		return 0, 0, 0
	}
	width, _ := strconv.Atoi(dimensions[0])
	height, _ := strconv.Atoi(dimensions[1])
	fps := 0
	if len(parts) > 1 {
		fps, _ = strconv.Atoi(parts[1])
	}
	return width, height, fps
}

func parse_video_duration(duration_text string) int64 {
	parts := strings.Split(strings.TrimSpace(duration_text), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0
	}
	var duration int64
	for _, part := range parts {
		value, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || value < 0 {
			return 0
		}
		duration = duration*60 + value
	}
	return duration
}

func video_url_expires_at(video_url *url.URL) *int64 {
	if video_url == nil {
		return nil
	}
	expires_at, err := strconv.ParseInt(video_url.Query().Get("Expires"), 10, 64)
	if err != nil || expires_at <= 0 {
		return nil
	}
	if expires_at < 1_000_000_000_000 {
		expires_at *= 1000
	}
	return &expires_at
}

func content_video_from_result(result *FetchResult) *model.ContentVideo {
	if result == nil || result.Video == nil || result.Video.URL == "" {
		return nil
	}
	content_id := PlatformID + ":" + result.ExternalID
	metadata, _ := json.Marshal(map[string]string{
		"media_id": result.Video.MediaID,
		"template": result.Video.Template,
	})
	variant := model.ContentVideoVariant{
		VideoId:      content_id,
		VariantKey:   "default",
		Spec:         result.Video.Template,
		Quality:      result.Video.Quality,
		Width:        positive_int_pointer(result.Video.Width),
		Height:       positive_int_pointer(result.Video.Height),
		FPS:          positive_int_pointer(result.Video.FPS),
		Format:       "mp4",
		StreamType:   model.ContentVideoVariantStreamTypeProgressive,
		HasVideo:     1,
		HasAudio:     1,
		IsDefault:    1,
		URL:          result.Video.URL,
		URLExpiresAt: result.Video.ExpiresAt,
		Metadata:     string(metadata),
	}
	return &model.ContentVideo{
		Id:       content_id,
		Duration: result.Video.Duration,
		Width:    result.Video.Width,
		Height:   result.Video.Height,
		FPS:      result.Video.FPS,
		Format:   "mp4",
		URL:      result.Video.URL,
		Variants: []model.ContentVideoVariant{variant},
	}
}

func content_video_details(content_video *model.ContentVideo) []adapter.ContentDetail {
	if content_video == nil {
		return nil
	}
	return []adapter.ContentDetail{{Type: model.ContentTypeVideo, Key: content_video.Id, Data: content_video}}
}

func positive_int_pointer(value int) *int {
	if value <= 0 {
		return nil
	}
	result := value
	return &result
}

func fetch_result_cover_url(result *FetchResult) string {
	if result == nil {
		return ""
	}
	if result.Video != nil && result.Video.CoverURL != "" {
		return result.Video.CoverURL
	}
	return first_image_url(result.Images)
}

func image_list_contains(images []Image, target_url string) bool {
	target_url = strings.TrimSpace(target_url)
	for _, image := range images {
		if strings.TrimSpace(image.URL) == target_url {
			return true
		}
	}
	return false
}
