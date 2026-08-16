package douyin

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var http_url_pattern = regexp.MustCompile(`https?://[a-zA-Z0-9._~:/?#\[\]@!$&'()*+,;=%-]+`)

const trailing_url_punctuation = `.,!?;:)]}"'`

// ExtractURL extracts the first Douyin URL from a URL or copied share text.
func ExtractURL(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", fmt.Errorf("empty content")
	}

	for _, candidate_url := range http_url_pattern.FindAllString(content, -1) {
		candidate_url = strings.TrimRight(candidate_url, trailing_url_punctuation)
		parsed_url, err := url.Parse(candidate_url)
		if err != nil || parsed_url.Hostname() == "" {
			continue
		}
		if is_douyin_host(parsed_url.Hostname()) {
			return candidate_url, nil
		}
	}

	return "", fmt.Errorf("douyin URL not found")
}

func is_douyin_host(host string) bool {
	host = strings.ToLower(host)
	return host == "douyin.com" ||
		strings.HasSuffix(host, ".douyin.com") ||
		host == "iesdouyin.com" ||
		strings.HasSuffix(host, ".iesdouyin.com")
}
