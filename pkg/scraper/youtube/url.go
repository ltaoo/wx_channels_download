package youtube

import (
	"net/url"
	"strings"
)

const PlatformID = "youtube"

func ExtractVideoID(raw_url string) (string, bool) {
	raw_url = strings.TrimSpace(raw_url)
	if raw_url == "" {
		return "", false
	}
	if is_likely_video_id(raw_url) {
		return raw_url, true
	}
	parsed, err := url.Parse(raw_url)
	if err != nil {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.Trim(parsed.EscapedPath(), "/")
	switch {
	case host == "youtu.be" || host == "www.youtu.be":
		id := first_path_segment(path)
		return id, is_likely_video_id(id)
	case is_youtube_host(host):
		if parsed.EscapedPath() == "/watch" {
			id := parsed.Query().Get("v")
			return id, is_likely_video_id(id)
		}
		for _, prefix := range []string{"shorts/", "embed/", "v/", "live/"} {
			if strings.HasPrefix(path, prefix) {
				id := first_path_segment(strings.TrimPrefix(path, prefix))
				return id, is_likely_video_id(id)
			}
		}
		if parsed.Query().Get("video_id") != "" {
			id := parsed.Query().Get("video_id")
			return id, is_likely_video_id(id)
		}
	}
	return "", false
}

func is_youtube_host(host string) bool {
	return host == "youtube.com" ||
		host == "www.youtube.com" ||
		host == "m.youtube.com" ||
		host == "music.youtube.com" ||
		host == "youtube-nocookie.com" ||
		host == "www.youtube-nocookie.com"
}

func first_path_segment(path string) string {
	if i := strings.Index(path, "/"); i >= 0 {
		return path[:i]
	}
	return path
}

func is_likely_video_id(value string) bool {
	if len(value) != 11 || strings.ContainsAny(value, "/:?&=#") {
		return false
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') ||
			(r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}
