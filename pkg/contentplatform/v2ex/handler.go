package v2ex

import (
	"context"
	"fmt"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/novelutil"
	v2expkg "wx_channel/pkg/v2ex"
)

const PlatformID = "v2ex"

const jsonVariantID = "json"

type TopicFetcher interface {
	FetchTopicPage(rawURL string) (*v2expkg.TopicPage, error)
}

type Handler struct {
	Fetcher TopicFetcher
}

func New(fetcher TopicFetcher) *Handler {
	if fetcher == nil {
		fetcher = v2expkg.NewClient(nil)
	}
	return &Handler{Fetcher: fetcher}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	_, ok := v2expkg.ParseTopicURL(rawURL)
	return ok
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	parts, ok := v2expkg.ParseTopicURL(input.URL)
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	page, err := h.Fetcher.FetchTopicPage(input.URL)
	if err != nil {
		return nil, fmt.Errorf("fetch v2ex topic: %w", err)
	}
	page.ID = firstNonEmpty(page.ID, parts.TopicID)
	page.CanonicalURL = firstNonEmpty(page.CanonicalURL, parts.Canonical)
	title := firstNonEmpty(page.Title, "v2ex_"+page.ID)
	bodyHTML := v2expkg.BuildHTML(page)
	authorName := firstNonEmpty(page.Author.Name)
	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: page.CanonicalURL,
		ContentID:    page.ID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            v2expkg.ContentTypeTopic,
			ID:              page.ID,
			Title:           title,
			Description:     page.Description,
			Author:          authorName,
			URL:             page.CanonicalURL,
			SourceURL:       page.CanonicalURL,
			AuthorNickname:  authorName,
			AuthorAvatarURL: page.Author.AvatarURL,
			CoverURL:        page.Author.AvatarURL,
		}, page, map[string]any{
			"topic_id":     page.ID,
			"node_name":    page.NodeName,
			"node_url":     page.NodeURL,
			"author_name":  page.Author.Name,
			"author_url":   page.Author.URL,
			"published_at": page.PublishedAt,
			"reply_count":  page.ReplyCount,
			"view_count":   page.ViewCount,
			"source_url":   page.CanonicalURL,
		}, ProbeOutput{
			Format:       "html",
			ContentType:  v2expkg.ContentTypeTopic,
			TopicID:      page.ID,
			Title:        title,
			SourceURL:    page.CanonicalURL,
			CanonicalURL: page.CanonicalURL,
			BodyHTML:     bodyHTML,
		}.Map()),
		Variants: []contentdownload.Variant{
			novelutil.HTMLVariant("网页 HTML", v2expkg.ContentTypeTopic),
			jsonVariant(v2expkg.ContentTypeTopic),
		},
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: map[string]any{
			"topic": page,
		},
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
	if variant.ID == jsonVariantID {
		return h.resolveJSON(ctx, input, probe, variant)
	}
	input.Probe = probe
	return novelutil.ResolveInlineHTML(ctx, PlatformID, input, h.Probe)
}

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	return novelutil.HTMLPlan(PlatformID), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func jsonVariant(contentType string) contentdownload.Variant {
	return contentdownload.Variant{
		ID:     jsonVariantID,
		Type:   jsonVariantID,
		Label:  "JSON",
		Suffix: ".json",
		Metadata: map[string]any{
			"format":       jsonVariantID,
			"content_type": contentType,
		},
	}
}

func (h *Handler) resolveJSON(ctx context.Context, input contentdownload.ResolveInput, probe *contentdownload.Probe, variant *contentdownload.Variant) (*contentdownload.ResolvedRequest, error) {
	summary := contentdownload.ContentSummaryOf(probe.Content)
	contentID := firstNonEmpty(probe.ContentID, summary.ID)
	title := firstNonEmpty(summary.Title, contentID, "v2ex")
	sourceURL := firstNonEmpty(probe.SourceURL, input.URL, summary.SourceURL, probe.CanonicalURL)
	canonicalURL := firstNonEmpty(probe.CanonicalURL, summary.URL, sourceURL)
	filename := firstNonEmpty(input.Options.Filename, title, contentID, "v2ex")
	suffix := firstNonEmpty(input.Options.Suffix, variant.Suffix, ".json")
	payload := map[string]any{
		"id":            probe.ID,
		"platform":      PlatformID,
		"source_url":    sourceURL,
		"canonical_url": canonicalURL,
		"content_id":    contentID,
		"content":       probe.Content,
		"variants":      probe.Variants,
		"defaults":      probe.Defaults,
		"warnings":      probe.Warnings,
		"output":        contentdownload.ContentOutputOf(probe.Content),
	}
	resolved := &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: canonicalURL,
		ContentID:    contentID,
		Title:        title,
		Filename:     filename,
		Suffix:       suffix,
		Download: contentdownload.DownloadSpec{
			URL:         "inline-json://v2ex/" + contentID,
			Method:      "GET",
			Protocol:    "inline_json",
			Connections: 1,
		},
		Labels: map[string]string{
			"platform":     PlatformID,
			"id":           contentID,
			"topic_id":     contentID,
			"title":        title,
			"key":          "0",
			"spec":         variant.Spec,
			"suffix":       suffix,
			"source_url":   canonicalURL,
			"content_type": firstNonEmpty(summary.Type, v2expkg.ContentTypeTopic),
		},
		Metadata: map[string]any{
			"variant_id":    variant.ID,
			"content_type":  firstNonEmpty(summary.Type, v2expkg.ContentTypeTopic),
			"source_url":    sourceURL,
			"canonical_url": canonicalURL,
			"json":          payload,
		},
		Content: probe.Content,
	}
	plan, err := h.Plan(ctx, resolved)
	if err != nil {
		return nil, err
	}
	resolved.Pipeline = plan
	return resolved, nil
}
