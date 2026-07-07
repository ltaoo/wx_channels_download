package interceptor

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"
)

type OfficialAccountArticleProfile struct {
	UniqueMark    string          `json:"unique_mark"`
	Title         string          `json:"title"`
	URL           string          `json:"url"`
	SourceURL     string          `json:"source_url"`
	CoverURL      string          `json:"cover_url"`
	Biz           string          `json:"biz"`
	Username      string          `json:"username"`
	Nickname      string          `json:"nickname"`
	AvatarURL     string          `json:"avatar_url"`
	Mid           string          `json:"mid"`
	Idx           string          `json:"idx"`
	Sn            string          `json:"sn"`
	RawCgiDataNew json.RawMessage `json:"cgiDataNew"`
}

func NewOfficialAccountArticleProfile(raw json.RawMessage) (*OfficialAccountArticleProfile, error) {
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}

	profile := &OfficialAccountArticleProfile{
		Title:         jsonString(data, "title"),
		URL:           html.UnescapeString(jsonString(data, "link")),
		SourceURL:     html.UnescapeString(jsonString(data, "source_url")),
		CoverURL:      html.UnescapeString(jsonString(data, "cdn_url")),
		Biz:           jsonString(data, "bizuin"),
		Username:      jsonString(data, "user_name"),
		Nickname:      jsonString(data, "nick_name"),
		AvatarURL:     html.UnescapeString(firstOfficialAccountValue(jsonString(data, "round_head_img"), jsonString(data, "ori_head_img_url"), jsonString(data, "hd_head_img"))),
		Mid:           jsonScalarString(data, "mid"),
		Idx:           jsonScalarString(data, "idx"),
		Sn:            jsonString(data, "sn"),
		RawCgiDataNew: raw,
	}
	fillOfficialAccountArticleFromURL(profile)
	profile.UniqueMark = buildOfficialAccountArticleUniqueMark(profile)
	return profile, nil
}

func fillOfficialAccountArticleFromURL(profile *OfficialAccountArticleProfile) {
	if profile == nil || profile.URL == "" {
		return
	}
	u, err := url.Parse(profile.URL)
	if err != nil {
		return
	}
	query := u.Query()
	if profile.Biz == "" {
		profile.Biz = query.Get("__biz")
	}
	if profile.Mid == "" {
		profile.Mid = query.Get("mid")
	}
	if profile.Idx == "" {
		profile.Idx = query.Get("idx")
	}
	if profile.Sn == "" {
		profile.Sn = query.Get("sn")
	}
}

func buildOfficialAccountArticleUniqueMark(profile *OfficialAccountArticleProfile) string {
	parts := []string{profile.Biz, profile.Mid, profile.Idx, profile.Sn}
	allPresent := true
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			allPresent = false
			break
		}
	}
	if allPresent {
		return strings.Join(parts, "_")
	}
	return firstOfficialAccountValue(profile.URL, profile.SourceURL, profile.Title)
}

func jsonString(data map[string]json.RawMessage, key string) string {
	raw, ok := data[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	return ""
}

func jsonScalarString(data map[string]json.RawMessage, key string) string {
	if s := jsonString(data, key); s != "" {
		return s
	}
	raw, ok := data[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return fmt.Sprintf("%t", b)
	}
	return ""
}

func firstOfficialAccountValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
