package douyin

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

var userAgents = []string{
	"Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/120.0.6099.119 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) EdgiOS/121.0.2277.107 Version/17.0 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/122.0 Mobile/15E148 Safari/605.1.15",
}

// canParse checks whether the URL is a Douyin domain.
func canParse(url string) bool {
	return strings.Contains(url, "douyin.com") || strings.Contains(url, "iesdouyin.com")
}

// resolveRedirects follows redirects from v.douyin.com short links until reaching the video page.
func resolveRedirects(raw_url, user_agent string, logger *zerolog.Logger) (string, error) {
	current_url := raw_url
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for redirect_index := 0; redirect_index < 5; redirect_index++ {
		req, err := http.NewRequest("GET", current_url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", user_agent)

		request_started_at := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}

		location := resp.Header.Get("Location")
		resp.Body.Close()
		if logger != nil {
			logger.Info().
				Int("redirect_step", redirect_index+1).
				Int("http_status", resp.StatusCode).
				Str("request_url", current_url).
				Str("location", location).
				Dur("request_elapsed", time.Since(request_started_at)).
				Msg("douyin mobile: redirect response received")
		}

		if location == "" {
			return current_url, nil
		}

		if strings.HasPrefix(location, "/") {
			location = "https://v.douyin.com" + location
		}

		if strings.Contains(location, "iesdouyin.com/share/video/") {
			return location, nil
		}

		current_url = location
	}
	if logger != nil {
		logger.Warn().
			Str("final_url", current_url).
			Int("redirect_limit", 5).
			Msg("douyin mobile: redirect limit reached")
	}
	return current_url, nil
}

// parseVideoID extracts the video ID from a URL.
func parseVideoID(url string) string {
	noQuery := strings.Split(url, "?")[0]
	noQuery = strings.TrimSuffix(noQuery, "/")
	parts := strings.Split(noQuery, "/")
	return parts[len(parts)-1]
}

// extractByRegex extracts a field from a JSON string using a regex pattern.
func extractByRegex(json string, pattern string) string {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(json)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// sanitizeFilename removes illegal characters from filenames.
func sanitizeFilename(name string) string {
	re := regexp.MustCompile(`[\\/:*?"<>|#\n\r]`)
	name = re.ReplaceAllString(name, "_")
	re2 := regexp.MustCompile(`\.{2,}`)
	name = re2.ReplaceAllString(name, ".")
	name = strings.Trim(name, " .")
	if len(name) > 80 {
		name = name[:80]
	}
	return name
}
