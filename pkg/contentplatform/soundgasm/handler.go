package soundgasm

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
	"wx_channel/pkg/contentplatform/novelutil"
	soundgasmpkg "wx_channel/pkg/scraper/soundgasm"
)

type AudioPageFetcher interface {
	FetchAudioPage(rawURL string) (*soundgasmpkg.AudioPage, error)
}

type Handler struct {
	Fetcher AudioPageFetcher
}

func New(fetcher AudioPageFetcher) *Handler {
	if fetcher == nil {
		fetcher = soundgasmpkg.NewClient(nil)
	}
	return &Handler{Fetcher: fetcher}
}

func (h *Handler) Platform() string {
	return PlatformID
}

func (h *Handler) Match(rawURL string) bool {
	_, ok := soundgasmpkg.ParseURL(rawURL)
	return ok
}

func (h *Handler) Probe(ctx context.Context, input contentdownload.ProbeInput) (*contentdownload.Probe, error) {
	parts, ok := soundgasmpkg.ParseURL(input.URL)
	if !ok {
		return nil, contentdownload.ErrUnsupportedURL
	}
	page, err := h.Fetcher.FetchAudioPage(parts.Canonical)
	if err != nil {
		return nil, fmt.Errorf("fetch soundgasm audio: %w", err)
	}
	if page == nil {
		return nil, fmt.Errorf("fetch soundgasm audio: empty page")
	}
	page.ID = firstNonEmpty(page.ID, soundgasmpkg.ContentID(parts))
	page.CanonicalURL = firstNonEmpty(page.CanonicalURL, parts.Canonical)
	page.URL = firstNonEmpty(page.URL, input.URL)
	title := firstNonEmpty(page.Title, page.ID, "soundgasm")
	sourceURL := firstNonEmpty(page.URL, input.URL, page.CanonicalURL)
	canonicalURL := firstNonEmpty(page.CanonicalURL, parts.Canonical, sourceURL)
	authorName := firstNonEmpty(page.Author.Name, parts.Username)
	bodyHTML := soundgasmpkg.BuildHTML(page)
	audioType := firstNonEmpty(page.AudioType, strings.TrimPrefix(audioSuffix(page.AudioURL), "."))
	suffix := firstNonEmpty(audioSuffix(page.AudioURL), suffixForAudioType(audioType), ".m4a")

	return &contentdownload.Probe{
		Platform:     PlatformID,
		SourceURL:    input.URL,
		CanonicalURL: canonicalURL,
		ContentID:    page.ID,
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform:          PlatformID,
			Type:              soundgasmpkg.ContentTypeAudio,
			ID:                page.ID,
			Title:             title,
			Description:       page.Description,
			Author:            authorName,
			URL:               canonicalURL,
			SourceURL:         sourceURL,
			AuthorNickname:    authorName,
			AuthorHomepageURL: page.Author.URL,
		}, page, map[string]any{
			"audio_id":            page.ID,
			"username":            parts.Username,
			"slug":                parts.Slug,
			"author_name":         authorName,
			"author_url":          page.Author.URL,
			"author_homepage_url": page.Author.URL,
			"audio_url":           page.AudioURL,
			"audio_type":          audioType,
			"source_url":          sourceURL,
			"canonical_url":       canonicalURL,
			"tags":                page.Tags,
		}, ProbeOutput{
			Format:       "audio",
			ContentType:  soundgasmpkg.ContentTypeAudio,
			AudioID:      page.ID,
			Title:        title,
			SourceURL:    sourceURL,
			CanonicalURL: canonicalURL,
			AudioURL:     page.AudioURL,
			AudioType:    audioType,
			BodyHTML:     bodyHTML,
			Tags:         page.Tags,
		}.Map()),
		Variants: []contentdownload.Variant{
			audioVariant(audioType, suffix, page.AudioURL),
			novelutil.HTMLVariant("网页 HTML", soundgasmpkg.ContentTypeAudio),
			jsonVariant(soundgasmpkg.ContentTypeAudio),
		},
		Defaults: contentdownload.Defaults{VariantID: audioVariantID, Suffix: suffix},
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
	switch variant.ID {
	case htmlVariantID:
		input.Probe = probe
		return novelutil.ResolveInlineHTML(ctx, PlatformID, input, h.Probe)
	case jsonVariantID:
		return h.resolveJSON(ctx, input, probe, variant)
	case audioVariantID:
		return h.resolveAudio(ctx, input, probe, variant)
	default:
		return nil, contentdownload.ErrVariantNotFound
	}
}

func (h *Handler) resolveAudio(ctx context.Context, input contentdownload.ResolveInput, probe *contentdownload.Probe, variant *contentdownload.Variant) (*contentdownload.ResolvedRequest, error) {
	page := audioPageFromProbe(probe)
	summary := contentdownload.ContentSummaryOf(probe.Content)
	contentID := firstNonEmpty(probe.ContentID, summary.ID)
	title := firstNonEmpty(summary.Title, contentID, "soundgasm")
	sourceURL := firstNonEmpty(probe.SourceURL, input.URL, summary.SourceURL, probe.CanonicalURL)
	canonicalURL := firstNonEmpty(probe.CanonicalURL, summary.URL, sourceURL)
	audioURL := firstNonEmpty(pageAudioURL(page), metadataString(contentdownload.ContentMetadataOf(probe.Content), "audio_url"), metadataString(contentdownload.ContentOutputOf(probe.Content), "audio_url"))
	if strings.TrimSpace(audioURL) == "" {
		return nil, contentdownload.ErrResolveUnavailable
	}
	audioType := firstNonEmpty(pageAudioType(page), metadataString(contentdownload.ContentMetadataOf(probe.Content), "audio_type"), metadataString(contentdownload.ContentOutputOf(probe.Content), "audio_type"))
	suffix := firstNonEmpty(input.Options.Suffix, variant.Suffix, audioSuffix(audioURL), suffixForAudioType(audioType), ".m4a")
	filename := firstNonEmpty(input.Options.Filename, title, contentID, "soundgasm")
	authorName := firstNonEmpty(summary.Author, summary.AuthorNickname, pageAuthorName(page))
	authorURL := firstNonEmpty(summary.AuthorHomepageURL, pageAuthorURL(page), metadataString(contentdownload.ContentMetadataOf(probe.Content), "author_url"))

	resolved := &contentdownload.ResolvedRequest{
		Platform:     PlatformID,
		SourceURL:    sourceURL,
		CanonicalURL: canonicalURL,
		ContentID:    contentID,
		Title:        title,
		Filename:     filename,
		Suffix:       suffix,
		Download: contentdownload.DownloadSpec{
			URL:         audioURL,
			Method:      "GET",
			Protocol:    "http",
			Connections: 4,
			Headers: map[string]string{
				"Accept":          "audio/*,*/*;q=0.8",
				"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
				"Referer":         canonicalURL,
				"User-Agent":      defaultUserAgent(),
			},
		},
		Labels: map[string]string{
			"platform":     PlatformID,
			"id":           contentID,
			"audio_id":     contentID,
			"title":        title,
			"key":          "0",
			"spec":         variant.Spec,
			"suffix":       suffix,
			"source_url":   canonicalURL,
			"content_type": soundgasmpkg.ContentTypeAudio,
		},
		Metadata: map[string]any{
			"variant_id":          variant.ID,
			"content_type":        soundgasmpkg.ContentTypeAudio,
			"source_url":          sourceURL,
			"canonical_url":       canonicalURL,
			"audio_url":           audioURL,
			"audio_type":          audioType,
			"author_name":         authorName,
			"author_url":          authorURL,
			"author_homepage_url": authorURL,
		},
		Content: soundgasmContentWithSummary(probe.Content, contentdownload.ContentSummary{
			Platform:          PlatformID,
			Type:              soundgasmpkg.ContentTypeAudio,
			ID:                contentID,
			Title:             title,
			Description:       summary.Description,
			Author:            authorName,
			URL:               canonicalURL,
			SourceURL:         sourceURL,
			AuthorNickname:    authorName,
			AuthorAvatarURL:   summary.AuthorAvatarURL,
			AuthorHomepageURL: authorURL,
			CoverURL:          summary.CoverURL,
			Duration:          summary.Duration,
		}),
		Internal: probe.Internal,
	}
	plan, err := h.Plan(ctx, resolved)
	if err != nil {
		return nil, err
	}
	resolved.Pipeline = plan
	return resolved, nil
}

func (h *Handler) resolveJSON(ctx context.Context, input contentdownload.ResolveInput, probe *contentdownload.Probe, variant *contentdownload.Variant) (*contentdownload.ResolvedRequest, error) {
	summary := contentdownload.ContentSummaryOf(probe.Content)
	contentID := firstNonEmpty(probe.ContentID, summary.ID)
	title := firstNonEmpty(summary.Title, contentID, "soundgasm")
	sourceURL := firstNonEmpty(probe.SourceURL, input.URL, summary.SourceURL, probe.CanonicalURL)
	canonicalURL := firstNonEmpty(probe.CanonicalURL, summary.URL, sourceURL)
	filename := firstNonEmpty(input.Options.Filename, title, contentID, "soundgasm")
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
			URL:         "inline-json://soundgasm/" + contentID,
			Method:      "GET",
			Protocol:    "inline_json",
			Connections: 1,
		},
		Labels: map[string]string{
			"platform":     PlatformID,
			"id":           contentID,
			"audio_id":     contentID,
			"title":        title,
			"key":          "0",
			"spec":         variant.Spec,
			"suffix":       suffix,
			"source_url":   canonicalURL,
			"content_type": soundgasmpkg.ContentTypeAudio,
		},
		Metadata: map[string]any{
			"variant_id":    variant.ID,
			"content_type":  soundgasmpkg.ContentTypeAudio,
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

func (h *Handler) Plan(ctx context.Context, resolved *contentdownload.ResolvedRequest) (*contentdownload.PipelinePlan, error) {
	return &contentdownload.PipelinePlan{
		Platform: PlatformID,
		Nodes: []contentdownload.PipelineNode{
			{ID: "download", Type: "download_asset", Stage: "download"},
			{ID: "persist", Type: "persist_artifacts", Stage: "persist", DependsOn: []string{"download"}},
		},
	}, nil
}

func audioVariant(audioType string, suffix string, audioURL string) contentdownload.Variant {
	label := strings.ToUpper(firstNonEmpty(audioType, strings.TrimPrefix(suffix, "."), "audio"))
	return contentdownload.Variant{
		ID:     audioVariantID,
		Type:   "audio",
		Label:  label,
		Suffix: firstNonEmpty(suffix, ".m4a"),
		Metadata: map[string]any{
			"format":     firstNonEmpty(audioType, strings.TrimPrefix(suffix, ".")),
			"audio_url":  audioURL,
			"direct_url": audioURL,
		},
	}
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

func audioPageFromProbe(probe *contentdownload.Probe) *soundgasmpkg.AudioPage {
	if probe == nil {
		return nil
	}
	if probe.Internal != nil {
		page, _ := probe.Internal["page"].(*soundgasmpkg.AudioPage)
		if page != nil {
			return page
		}
	}
	page, _ := contentdownload.ContentDataOf(probe.Content).(*soundgasmpkg.AudioPage)
	return page
}

func pageAudioURL(page *soundgasmpkg.AudioPage) string {
	if page == nil {
		return ""
	}
	return page.AudioURL
}

func pageAudioType(page *soundgasmpkg.AudioPage) string {
	if page == nil {
		return ""
	}
	return page.AudioType
}

func pageAuthorName(page *soundgasmpkg.AudioPage) string {
	if page == nil {
		return ""
	}
	return page.Author.Name
}

func pageAuthorURL(page *soundgasmpkg.AudioPage) string {
	if page == nil {
		return ""
	}
	return page.Author.URL
}

func soundgasmContentWithSummary(content any, summary contentdownload.ContentSummary) any {
	return contentdownload.NewContent(summary, contentdownload.ContentDataOf(content), contentdownload.ContentMetadataOf(content), contentdownload.ContentOutputOf(content))
}

func audioSuffix(rawURL string) string {
	pathValue := strings.TrimSpace(rawURL)
	if parsed, err := url.Parse(pathValue); err == nil && parsed.Path != "" {
		pathValue = parsed.Path
	}
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(pathValue)))
	switch ext {
	case ".m4a", ".mp3", ".oga", ".ogg", ".wav":
		return ext
	default:
		return ""
	}
}

func suffixForAudioType(audioType string) string {
	audioType = strings.ToLower(strings.TrimSpace(audioType))
	switch audioType {
	case "m4a", "mp3", "oga", "ogg", "wav":
		return "." + audioType
	default:
		return ""
	}
}

func metadataString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value, _ := values[key].(string)
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func defaultUserAgent() string {
	return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
