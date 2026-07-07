package qq

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/internal/jsonartifact"
	qqpkg "wx_channel/pkg/scraper/qq"
)

const PlatformID = "qq"

type ProfileFetcher interface {
	FetchTVProfile(ctx context.Context, idOrURL string) (*qqpkg.TVProfile, error)
}

type Handler struct {
	Client ProfileFetcher
}

func New(client ProfileFetcher) *Handler {
	if client == nil {
		client = qqpkg.NewClient()
	}
	return &Handler{Client: client}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	return matchHost(rawURL, "v.qq.com")
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	if !h.Match(input.URL) {
		return nil, contentdownload.ErrUnsupportedURL
	}
	contentID := extractCoverID(input.URL)
	fetchKey := jsonartifact.FirstNonEmpty(contentID, input.URL)
	profile, err := h.Client.FetchTVProfile(ctx, fetchKey)
	if err != nil {
		return nil, fmt.Errorf("fetch qq profile: %w", err)
	}
	if profile == nil {
		return nil, fmt.Errorf("fetch qq profile: empty profile")
	}
	contentID = jsonartifact.FirstNonEmpty(contentID, profile.ID, input.URL)
	profile.ID = contentID
	canonicalURL := input.URL
	if contentID != "" && !strings.HasPrefix(contentID, "http") {
		canonicalURL = "https://v.qq.com/x/cover/" + contentID + ".html"
	}
	contentType := "tv"
	title := jsonartifact.FirstNonEmpty(profile.Name, contentID)
	content := contentdownload.NewContent(contentdownload.ContentSummary{
		Platform:    PlatformID,
		Type:        contentType,
		ID:          contentID,
		Title:       title,
		Description: profile.Overview,
		URL:         canonicalURL,
		SourceURL:   input.URL,
		CoverURL:    profile.PosterPath,
	}, profile, map[string]any{
		"qq_id":             contentID,
		"number_of_seasons": profile.NumberOfSeasons,
	}, map[string]any{
		"format":            "json",
		"content_type":      contentType,
		"id":                contentID,
		"title":             title,
		"source_url":        input.URL,
		"canonical_url":     canonicalURL,
		"number_of_seasons": profile.NumberOfSeasons,
	})
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: canonicalURL,
		ContentID:    contentID,
		Content:      content,
		Variants:     []contentdownload.Variant{jsonartifact.Variant(contentType)},
		Defaults:     jsonartifact.Defaults(),
		Internal:     map[string]any{"profile": profile},
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

func matchHost(rawURL string, suffix string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u == nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == suffix || strings.HasSuffix(host, "."+suffix)
}

func extractCoverID(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u == nil {
		return ""
	}
	if cid := strings.TrimSpace(u.Query().Get("cid")); cid != "" {
		return cid
	}
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i+1 < len(segments); i++ {
		if segments[i] == "cover" {
			return strings.TrimSuffix(segments[i+1], ".html")
		}
	}
	return ""
}
