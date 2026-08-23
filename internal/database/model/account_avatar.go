package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ApplyObservedAvatarURL updates the current avatar URL and appends the
// previous URL to PastAvatars when the observed URL changes. PastAvatars is a
// JSON array that keeps URLs in first-seen order without duplicates.
func (a *Account) ApplyObservedAvatarURL(observed_url string) (bool, error) {
	if a == nil {
		return false, fmt.Errorf("account is nil")
	}

	observed_url = strings.TrimSpace(observed_url)
	current_url := strings.TrimSpace(a.AvatarURL)
	if observed_url == "" || observed_url == current_url {
		return false, nil
	}

	past_avatars, err := parse_past_avatar_urls(a.PastAvatars)
	if err != nil {
		return false, fmt.Errorf("parse account %q past_avatars: %w", a.Id, err)
	}

	seen_urls := make(map[string]struct{}, len(past_avatars)+1)
	normalized_urls := make([]string, 0, len(past_avatars)+1)
	for _, past_avatar := range past_avatars {
		past_avatar = strings.TrimSpace(past_avatar)
		if past_avatar == "" {
			continue
		}
		if _, exists := seen_urls[past_avatar]; exists {
			continue
		}
		seen_urls[past_avatar] = struct{}{}
		normalized_urls = append(normalized_urls, past_avatar)
	}
	if current_url != "" {
		if _, exists := seen_urls[current_url]; !exists {
			normalized_urls = append(normalized_urls, current_url)
		}
	}

	past_avatars_json, err := json.Marshal(normalized_urls)
	if err != nil {
		return false, fmt.Errorf("encode account %q past_avatars: %w", a.Id, err)
	}

	a.AvatarURL = observed_url
	a.PastAvatars = string(past_avatars_json)
	return true, nil
}

func parse_past_avatar_urls(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{}, nil
	}

	var urls []string
	if err := json.Unmarshal([]byte(value), &urls); err != nil {
		return nil, err
	}
	if urls == nil {
		return []string{}, nil
	}
	return urls, nil
}
