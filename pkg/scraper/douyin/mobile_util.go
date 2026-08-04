package douyin

import (
	"net/http"
	"regexp"
	"strings"
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
func resolveRedirects(url, ua string) (string, error) {
	current := url
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for i := 0; i < 5; i++ {
		req, _ := http.NewRequest("GET", current, nil)
		req.Header.Set("User-Agent", ua)

		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}

		location := resp.Header.Get("Location")
		resp.Body.Close()

		if location == "" {
			return current, nil
		}

		if strings.HasPrefix(location, "/") {
			location = "https://v.douyin.com" + location
		}

		if strings.Contains(location, "iesdouyin.com/share/video/") {
			return location, nil
		}

		current = location
	}
	return current, nil
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
