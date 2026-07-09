package mgtv

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/internal/jsonartifact"
	mgtvpkg "wx_channel/pkg/scraper/mgtv"
)

const PlatformID = "mgtv"

var mgtvIDRE = regexp.MustCompile(`/(?:b|h)/([^/?#]+)`)

type ProfileFetcher interface {
	FetchTVProfile(ctx context.Context, rawURL string) (*mgtvpkg.TVProfile, error)
}

type Handler struct {
	Client ProfileFetcher
}

func New(client ProfileFetcher) *Handler {
	if client == nil {
		client = mgtvpkg.NewClient()
	}
	return &Handler{Client: client}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	return matchHost(rawURL, "mgtv.com")
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	if !h.Match(input.URL) {
		return nil, contentdownload.ErrUnsupportedURL
	}
	profile, err := h.Client.FetchTVProfile(ctx, input.URL)
	if err != nil {
		return nil, fmt.Errorf("fetch mgtv profile: %w", err)
	}
	if profile == nil {
		return nil, fmt.Errorf("fetch mgtv profile: empty profile")
	}
	contentID := jsonartifact.FirstNonEmpty(extractMGTVID(input.URL), profile.ID, input.URL)
	profile.ID = contentID
	contentType := "tv"
	title := jsonartifact.FirstNonEmpty(profile.Name, profile.OriginalName, contentID)
	content := contentdownload.NewContent(contentdownload.ContentSummary{
		Platform:    PlatformID,
		Type:        contentType,
		ID:          contentID,
		Title:       title,
		Description: profile.Overview,
		URL:         input.URL,
		SourceURL:   input.URL,
		CoverURL:    profile.PosterPath,
	}, profile, map[string]any{
		"mgtv_id":           contentID,
		"first_air_date":    profile.FirstAirDate,
		"number_of_seasons": profile.NumberOfSeasons,
		"vote_average":      profile.VoteAverage,
	}, map[string]any{
		"format":            "json",
		"content_type":      contentType,
		"id":                contentID,
		"title":             title,
		"source_url":        input.URL,
		"canonical_url":     input.URL,
		"number_of_seasons": profile.NumberOfSeasons,
	})
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: input.URL,
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

func extractMGTVID(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u == nil {
		return ""
	}
	match := mgtvIDRE.FindStringSubmatch(u.Path)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSuffix(match[1], ".html")
}
