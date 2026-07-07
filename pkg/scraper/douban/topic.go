package douban

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	topicIDRE           = regexp.MustCompile(`^/group/topic/([0-9]+)`)
	topicTitleRE        = regexp.MustCompile(`<h1[^>]*>([\s\S]*?)</h1>`)
	topicAuthorRE       = regexp.MustCompile(`<span class="from">\s*<a href="([^"]+)">([^<]+)</a>`)
	topicAvatarRE       = regexp.MustCompile(`<div class="topic-content clearfix" id="topic-content">[\s\S]*?<div class="user-face">\s*<a href="([^"]+)"><img[^>]*src="([^"]+)"[^>]*alt="([^"]*)"`)
	topicBodyRE         = regexp.MustCompile(`<div class="rich-content topic-richtext">([\s\S]*?)</div>`)
	topicCreateTimeRE   = regexp.MustCompile(`<span class="create-time">([^<]+)</span>`)
	topicUpdateTimeRE   = regexp.MustCompile(`<span class="update-time">([^<]+)</span>`)
	topicGroupConfigRE  = regexp.MustCompile(`window\._CONFIG\.group\s*=\s*(\{[^\n;]+\})`)
	topicSchemaJSONRE   = regexp.MustCompile(`<script type="application/ld\+json">\s*([\s\S]*?)\s*</script>`)
	topicCommentCountRE = regexp.MustCompile(`"commentCount"\s*:\s*"?([0-9]+)"?`)
	peopleIDRE          = regexp.MustCompile(`/people/([^/]+)/?`)
)

func ParseGroupTopicHTML(htmlText string, rawURL string) (*GroupTopicProfile, error) {
	id, canonicalURL, ok := ParseGroupTopicURL(rawURL)
	if !ok {
		return nil, fmt.Errorf("invalid douban group topic url")
	}
	profile := &GroupTopicProfile{
		ID:        id,
		URL:       canonicalURL,
		SourceURL: rawURL,
	}
	if title := stripTags(firstSubmatch(topicTitleRE, htmlText)); title != "" {
		profile.Title = title
	}
	if match := topicAuthorRE.FindStringSubmatch(htmlText); len(match) > 2 {
		profile.AuthorURL = html.UnescapeString(match[1])
		profile.AuthorName = strings.TrimSpace(html.UnescapeString(match[2]))
		profile.AuthorID = peopleID(profile.AuthorURL)
	}
	if match := topicAvatarRE.FindStringSubmatch(htmlText); len(match) > 3 {
		if profile.AuthorURL == "" {
			profile.AuthorURL = html.UnescapeString(match[1])
			profile.AuthorID = peopleID(profile.AuthorURL)
		}
		profile.AuthorAvatarURL = html.UnescapeString(match[2])
		if profile.AuthorName == "" {
			profile.AuthorName = strings.TrimSpace(html.UnescapeString(match[3]))
		}
	}
	if bodyHTML := strings.TrimSpace(firstSubmatch(topicBodyRE, htmlText)); bodyHTML != "" {
		profile.BodyHTML = bodyHTML
		profile.BodyText = stripTags(bodyHTML)
	}
	profile.CreatedAt = strings.TrimSpace(html.UnescapeString(firstSubmatch(topicCreateTimeRE, htmlText)))
	profile.UpdatedAt = strings.TrimSpace(html.UnescapeString(firstSubmatch(topicUpdateTimeRE, htmlText)))
	applyTopicGroupConfig(profile, htmlText)
	applyTopicSchema(profile, htmlText)
	if profile.Title == "" {
		return nil, fmt.Errorf("missing douban group topic title")
	}
	if profile.AuthorName == "" && profile.AuthorID == "" {
		return nil, fmt.Errorf("missing douban group topic author")
	}
	return profile, nil
}

func ParseGroupTopicURL(rawURL string) (id string, canonicalURL string, ok bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u == nil {
		return "", "", false
	}
	host := strings.ToLower(u.Hostname())
	if host != "douban.com" && !strings.HasSuffix(host, ".douban.com") {
		return "", "", false
	}
	match := topicIDRE.FindStringSubmatch(u.Path)
	if len(match) < 2 {
		return "", "", false
	}
	id = match[1]
	return id, "https://www.douban.com/group/topic/" + id + "/", true
}

func firstSubmatch(re *regexp.Regexp, text string) string {
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func peopleID(rawURL string) string {
	if match := peopleIDRE.FindStringSubmatch(rawURL); len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func applyTopicGroupConfig(profile *GroupTopicProfile, htmlText string) {
	raw := firstSubmatch(topicGroupConfigRE, htmlText)
	if raw == "" {
		return
	}
	var cfg struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return
	}
	profile.GroupID = cfg.ID
	profile.GroupName = firstNonEmpty(cfg.Title, cfg.Name)
}

func applyTopicSchema(profile *GroupTopicProfile, htmlText string) {
	raw := firstSubmatch(topicSchemaJSONRE, htmlText)
	if raw == "" {
		if match := topicCommentCountRE.FindStringSubmatch(htmlText); len(match) > 1 {
			profile.CommentCount, _ = strconv.Atoi(match[1])
		}
		return
	}
	var schema struct {
		Text         string `json:"text"`
		Name         string `json:"name"`
		DateCreated  string `json:"dateCreated"`
		CommentCount any    `json:"commentCount"`
	}
	if err := json.Unmarshal([]byte(raw), &schema); err != nil {
		return
	}
	if profile.Title == "" {
		profile.Title = strings.TrimSpace(schema.Name)
	}
	if profile.BodyText == "" {
		profile.BodyText = strings.TrimSpace(schema.Text)
	}
	if profile.CreatedAt == "" {
		profile.CreatedAt = strings.TrimSpace(schema.DateCreated)
	}
	switch v := schema.CommentCount.(type) {
	case string:
		profile.CommentCount, _ = strconv.Atoi(v)
	case float64:
		profile.CommentCount = int(v)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
