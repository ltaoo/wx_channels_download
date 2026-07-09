package jsonartifact

import (
	"context"
	"strings"

	contentdownload "wx_channel/pkg/contentplatform/download"
)

const VariantID = "json"

func Variant(contentType string) contentdownload.Variant {
	metadata := map[string]any{"format": VariantID}
	if strings.TrimSpace(contentType) != "" {
		metadata["content_type"] = contentType
	}
	return contentdownload.Variant{
		ID:       VariantID,
		Type:     VariantID,
		Label:    "JSON",
		Suffix:   ".json",
		Metadata: metadata,
	}
}

func Defaults() contentdownload.Defaults {
	return contentdownload.Defaults{VariantID: VariantID, Suffix: ".json"}
}

func Resolve(_ context.Context, platformID string, input contentdownload.ResolveInput, probe *contentdownload.Probe, variant *contentdownload.Variant) (*contentdownload.ResolvedRequest, error) {
	if probe == nil || variant == nil || !strings.EqualFold(variant.ID, VariantID) {
		return nil, contentdownload.ErrVariantNotFound
	}
	summary := contentdownload.ContentSummaryOf(probe.Content)
	contentID := FirstNonEmpty(probe.ContentID, summary.ID, platformID)
	title := FirstNonEmpty(summary.Title, contentID, platformID)
	sourceURL := FirstNonEmpty(probe.SourceURL, input.URL, summary.SourceURL, probe.CanonicalURL)
	canonicalURL := FirstNonEmpty(probe.CanonicalURL, summary.URL, sourceURL)
	filename := FirstNonEmpty(input.Options.Filename, title, contentID, platformID)
	suffix := FirstNonEmpty(input.Options.Suffix, variant.Suffix, ".json")
	contentType := FirstNonEmpty(summary.Type, variantContentType(variant))
	payload := map[string]any{
		"id":            probe.ID,
		"platform":      platformID,
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
		Platform:     platformID,
		SourceURL:    sourceURL,
		CanonicalURL: canonicalURL,
		ContentID:    contentID,
		Title:        title,
		Filename:     filename,
		Suffix:       suffix,
		Download: contentdownload.DownloadSpec{
			URL:         "inline-json://" + platformID + "/" + contentID,
			Method:      "GET",
			Protocol:    "inline_json",
			Connections: 1,
		},
		Labels: map[string]string{
			"platform":     platformID,
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
	resolved.Pipeline = Plan(platformID)
	return resolved, nil
}

func Plan(platformID string) *contentdownload.PipelinePlan {
	return &contentdownload.PipelinePlan{
		Platform: platformID,
		Nodes: []contentdownload.PipelineNode{
			{ID: "download", Type: "download_asset", Stage: "download"},
			{ID: "persist", Type: "persist_artifacts", Stage: "persist", DependsOn: []string{"download"}},
		},
	}
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func variantContentType(variant *contentdownload.Variant) string {
	if variant == nil || variant.Metadata == nil {
		return ""
	}
	value, _ := variant.Metadata["content_type"].(string)
	return value
}
