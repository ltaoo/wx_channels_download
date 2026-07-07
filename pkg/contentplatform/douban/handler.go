package douban

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/internal/jsonartifact"
	doubanpkg "wx_channel/pkg/scraper/douban"
)

const PlatformID = "douban"

const (
	doubanOfficialAuthorID       = "douban"
	doubanOfficialAuthorName     = "豆瓣"
	doubanOfficialAuthorHomepage = "https://www.douban.com/"
)

var subjectIDRE = regexp.MustCompile(`^/subject/([0-9]+)`)

type ProfileFetcher interface {
	FetchMediaProfile(ctx context.Context, id any) (*doubanpkg.MediaProfile, error)
}

type SubjectFetcher interface {
	FetchSubjectProfile(ctx context.Context, rawURL string) (*doubanpkg.MediaProfile, error)
}

type TopicFetcher interface {
	FetchGroupTopic(ctx context.Context, rawURL string) (*doubanpkg.GroupTopicProfile, error)
}

type Handler struct {
	Client ProfileFetcher
}

func New(client ProfileFetcher) *Handler {
	if client == nil {
		client = doubanpkg.NewClient()
	}
	return &Handler{Client: client}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	return parseDoubanURL(rawURL).Kind != ""
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	target := parseDoubanURL(input.URL)
	switch target.Kind {
	case "subject":
		return h.probeSubject(ctx, input, target)
	case "group_topic":
		return h.probeGroupTopic(ctx, input, target)
	default:
		return nil, contentdownload.ErrUnsupportedURL
	}
}

func (h *Handler) probeSubject(ctx context.Context, input contentdownload.ProbeInput, target doubanURLTarget) (*contentdownload.Probe, error) {
	profile, err := h.fetchSubjectProfile(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("fetch douban profile: %w", err)
	}
	if profile == nil {
		return nil, fmt.Errorf("fetch douban profile: empty profile")
	}
	if strings.TrimSpace(profile.ID) == "" {
		profile.ID = target.ID
	}
	contentType := jsonartifact.FirstNonEmpty(profile.Type, "media")
	title := jsonartifact.FirstNonEmpty(profile.Name, profile.OriginalName, target.ID)
	coverURL := jsonartifact.FirstNonEmpty(profile.CoverURL, profile.PosterPath)
	content := contentdownload.NewContent(contentdownload.ContentSummary{
		Platform:       PlatformID,
		Type:           contentType,
		ID:             target.ID,
		Title:          title,
		Description:    profile.Overview,
		URL:            target.CanonicalURL,
		SourceURL:      input.URL,
		Author:         doubanOfficialAuthorName,
		AuthorNickname: doubanOfficialAuthorName,
		CoverURL:       coverURL,
	}, profile, map[string]any{
		"douban_id":           target.ID,
		"imdb":                profile.IMDB,
		"type":                profile.Type,
		"air_date":            profile.AirDate,
		"vote_average":        profile.VoteAverage,
		"origin_country":      profile.OriginCountry,
		"cover_url":           coverURL,
		"account_external_id": doubanOfficialAuthorID,
		"account_username":    doubanOfficialAuthorID,
		"author_id":           doubanOfficialAuthorID,
		"author_homepage_url": doubanOfficialAuthorHomepage,
	}, map[string]any{
		"format":              "json",
		"content_type":        contentType,
		"id":                  target.ID,
		"title":               title,
		"source_url":          input.URL,
		"canonical_url":       target.CanonicalURL,
		"cover_url":           coverURL,
		"vote_average":        profile.VoteAverage,
		"air_date":            profile.AirDate,
		"genres":              profile.Genres,
		"author_homepage_url": doubanOfficialAuthorHomepage,
	})
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: target.CanonicalURL,
		ContentID:    target.ID,
		Content:      content,
		Variants:     []contentdownload.Variant{jsonartifact.Variant(contentType)},
		Defaults:     jsonartifact.Defaults(),
		Internal:     map[string]any{"profile": profile},
	}, nil
}

func (h *Handler) fetchSubjectProfile(ctx context.Context, target doubanURLTarget) (*doubanpkg.MediaProfile, error) {
	if fetcher, ok := h.Client.(SubjectFetcher); ok {
		return fetcher.FetchSubjectProfile(ctx, target.CanonicalURL)
	}
	return h.Client.FetchMediaProfile(ctx, target.ID)
}

func (h *Handler) probeGroupTopic(ctx context.Context, input contentdownload.ProbeInput, target doubanURLTarget) (*contentdownload.Probe, error) {
	fetcher, ok := h.Client.(TopicFetcher)
	if !ok {
		return nil, contentdownload.ErrResolveUnavailable
	}
	topic, err := fetcher.FetchGroupTopic(ctx, input.URL)
	if err != nil {
		return nil, fmt.Errorf("fetch douban group topic: %w", err)
	}
	if topic == nil {
		return nil, fmt.Errorf("fetch douban group topic: empty topic")
	}
	if strings.TrimSpace(topic.ID) == "" {
		topic.ID = target.ID
	}
	title := jsonartifact.FirstNonEmpty(topic.Title, target.ID)
	authorName := jsonartifact.FirstNonEmpty(topic.AuthorName, topic.AuthorID)
	description := jsonartifact.FirstNonEmpty(topic.BodyText, topic.Title)
	content := contentdownload.NewContent(contentdownload.ContentSummary{
		Platform:        PlatformID,
		Type:            "topic",
		ID:              target.ID,
		Title:           title,
		Description:     description,
		URL:             target.CanonicalURL,
		SourceURL:       input.URL,
		Author:          authorName,
		AuthorNickname:  authorName,
		AuthorAvatarURL: topic.AuthorAvatarURL,
	}, topic, map[string]any{
		"topic_id":            target.ID,
		"group_id":            topic.GroupID,
		"group_name":          topic.GroupName,
		"author_id":           topic.AuthorID,
		"account_external_id": jsonartifact.FirstNonEmpty(topic.AuthorID, topic.AuthorURL, authorName),
		"account_username":    jsonartifact.FirstNonEmpty(topic.AuthorID, authorName),
		"author_homepage_url": topic.AuthorURL,
		"author_avatar_url":   topic.AuthorAvatarURL,
		"created_at":          topic.CreatedAt,
		"updated_at":          topic.UpdatedAt,
		"comment_count":       topic.CommentCount,
	}, map[string]any{
		"format":              "json",
		"content_type":        "topic",
		"id":                  target.ID,
		"title":               title,
		"source_url":          input.URL,
		"canonical_url":       target.CanonicalURL,
		"author_homepage_url": topic.AuthorURL,
		"author_avatar_url":   topic.AuthorAvatarURL,
		"body_html":           topic.BodyHTML,
		"body_text":           topic.BodyText,
	})
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: target.CanonicalURL,
		ContentID:    target.ID,
		Content:      content,
		Variants:     []contentdownload.Variant{jsonartifact.Variant("topic")},
		Defaults:     jsonartifact.Defaults(),
		Internal:     map[string]any{"topic": topic},
	}, nil
}

func (h *Handler) Resolve(ctx context.Context, input contentdownload.ResolveInput) (*contentdownload.ResolvedRequest, error) {
	probe := input.Probe
	if probe == nil {
		var err error
		probe, err = h.Probe(ctx, contentdownload.ProbeInput{URL: input.URL, Extra: input.Extra})
		if err != nil {
			return nil, err
		}
	}
	variant, err := contentdownload.SelectVariant(probe, input.Options)
	if err != nil {
		return nil, err
	}
	return jsonartifact.Resolve(ctx, PlatformID, input, probe, variant)
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	return jsonartifact.Plan(PlatformID), nil
}

type doubanURLTarget struct {
	Kind         string
	ID           string
	CanonicalURL string
}

func parseDoubanURL(rawURL string) doubanURLTarget {
	if id, canonicalURL, ok := parseSubjectURL(rawURL); ok {
		return doubanURLTarget{Kind: "subject", ID: id, CanonicalURL: canonicalURL}
	}
	if id, canonicalURL, ok := doubanpkg.ParseGroupTopicURL(rawURL); ok {
		return doubanURLTarget{Kind: "group_topic", ID: id, CanonicalURL: canonicalURL}
	}
	return doubanURLTarget{}
}

func parseSubjectURL(rawURL string) (id string, canonicalURL string, ok bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u == nil {
		return "", "", false
	}
	host := strings.ToLower(u.Hostname())
	if host != "douban.com" && !strings.HasSuffix(host, ".douban.com") {
		return "", "", false
	}
	match := subjectIDRE.FindStringSubmatch(u.Path)
	if len(match) < 2 {
		return "", "", false
	}
	id = match[1]
	canonicalHost := host
	if canonicalHost == "douban.com" {
		canonicalHost = "www.douban.com"
	}
	return id, "https://" + canonicalHost + "/subject/" + id + "/", true
}
