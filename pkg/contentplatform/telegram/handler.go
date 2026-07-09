package telegram

import (
	"context"
	"fmt"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/novelutil"
	telegrampkg "wx_channel/pkg/scraper/telegram"
)

const PlatformID = "telegram"

const jsonVariantID = "json"

type PageFetcher interface {
	FetchPage(ctx context.Context, rawURL string) (*telegrampkg.Page, error)
}

type Handler struct {
	Fetcher PageFetcher
}

func New(fetcher PageFetcher) *Handler {
	if fetcher == nil {
		fetcher = telegrampkg.NewClient(nil)
	}
	return &Handler{Fetcher: fetcher}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	return telegrampkg.CanParse(rawURL)
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	shareURL := telegrampkg.ExtractShareURL(input.URL)
	if shareURL == "" {
		return nil, contentdownload.ErrUnsupportedURL
	}
	parts, ok := telegrampkg.ParseURL(shareURL)
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	page, err := h.Fetcher.FetchPage(ctx, shareURL)
	if err != nil {
		return nil, fmt.Errorf("fetch telegram page: %w", err)
	}
	if page == nil {
		return nil, fmt.Errorf("fetch telegram page: empty page")
	}
	if page.URL.Username == "" {
		page.URL = parts
	}
	if page.CanonicalURL == "" {
		page.CanonicalURL = parts.Canonical
	}
	if page.SourceURL == "" {
		page.SourceURL = firstNonEmpty(input.URL, parts.WebURL)
	}

	contentType := firstNonEmpty(page.ContentType(), telegrampkg.ContentTypeChannel)
	contentID := firstNonEmpty(page.ContentID(), parts.Username)
	title := firstNonEmpty(telegrampkg.PageTitle(page), contentID, "telegram")
	bodyHTML := telegrampkg.BuildHTML(page)
	canonicalURL := firstNonEmpty(page.CanonicalURL, parts.Canonical)
	sourceURL := firstNonEmpty(page.SourceURL, input.URL, parts.WebURL, canonicalURL)
	coverURL := telegrampkg.FirstMediaURL(page)
	authorName := firstNonEmpty(page.Channel.Title, page.Channel.Username, parts.Username)
	messageID := 0
	if contentType == telegrampkg.ContentTypeMessage {
		messageID = parts.MessageID
		if messageID == 0 && len(page.Messages) > 0 {
			messageID = page.Messages[0].ID
		}
	}

	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: canonicalURL,
		ContentID:    contentID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:        PlatformID,
			Type:            contentType,
			ID:              contentID,
			Title:           title,
			Description:     telegrampkg.PageDescription(page),
			Author:          authorName,
			URL:             canonicalURL,
			SourceURL:       sourceURL,
			AuthorNickname:  authorName,
			AuthorAvatarURL: page.Channel.AvatarURL,
			CoverURL:        coverURL,
		}, page, map[string]any{
			"username":      firstNonEmpty(page.Channel.Username, parts.Username),
			"message_id":    messageID,
			"message_count": len(page.Messages),
			"source_url":    sourceURL,
			"canonical_url": canonicalURL,
			"channel":       page.Channel,
		}, ProbeOutput{
			Format:       "html",
			ContentType:  contentType,
			Username:     firstNonEmpty(page.Channel.Username, parts.Username),
			MessageID:    messageID,
			Title:        title,
			SourceURL:    sourceURL,
			CanonicalURL: canonicalURL,
			BodyHTML:     bodyHTML,
			MessageCount: len(page.Messages),
		}.Map()),
		Variants: []contentdownload.Variant{
			novelutil.HTMLVariant("网页 HTML", contentType),
			jsonVariant(contentType),
		},
		Defaults: contentdownload.Defaults{VariantID: "html", Suffix: ".html"},
		Internal: map[string]any{
			"page":     page,
			"pagehtml": page.PageHTML,
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
	title := firstNonEmpty(summary.Title, contentID, "telegram")
	sourceURL := firstNonEmpty(probe.SourceURL, input.URL, summary.SourceURL, probe.CanonicalURL)
	canonicalURL := firstNonEmpty(probe.CanonicalURL, summary.URL, sourceURL)
	filename := firstNonEmpty(input.Options.Filename, title, contentID, "telegram")
	suffix := firstNonEmpty(input.Options.Suffix, variant.Suffix, ".json")
	contentType := firstNonEmpty(summary.Type, telegrampkg.ContentTypeChannel)
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
			URL:         "inline-json://telegram/" + contentID,
			Method:      "GET",
			Protocol:    "inline_json",
			Connections: 1,
		},
		Labels: map[string]string{
			"platform":     PlatformID,
			"id":           contentID,
			"title":        title,
			"key":          "0",
			"spec":         variant.Spec,
			"suffix":       suffix,
			"source_url":   canonicalURL,
			"content_type": contentType,
		},
		Metadata: map[string]any{
			"variant_id":    variant.ID,
			"content_type":  contentType,
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
