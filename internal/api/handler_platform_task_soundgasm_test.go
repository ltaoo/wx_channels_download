package api

import (
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	contentsoundgasm "wx_channel/pkg/contentplatform/soundgasm"
	soundgasmpkg "wx_channel/pkg/scraper/soundgasm"
)

func TestPlatformDownloadRouterMatchesSoundgasm(t *testing.T) {
	client := &APIClient{cfg: &APIConfig{}}
	handler := client.platformDownloadRouter().Match("https://soundgasm.net/u/BrittanyBabbles/Cum-Quietly-Baby-We-Cant-Wake-Up-Daddy")
	if handler == nil {
		t.Fatal("expected soundgasm handler")
	}
	if handler.Platform() != "soundgasm" {
		t.Fatalf("platform = %q", handler.Platform())
	}
}

func TestPlatformProbeAddJSONDefaultKeepsSoundgasmAudioDefault(t *testing.T) {
	probe := &contentdownload.Probe{
		Platform:  contentsoundgasm.PlatformID,
		ContentID: "audio_1",
		Content: contentdownload.NewContent(contentdownload.ContentSummary{
			Platform: contentsoundgasm.PlatformID,
			Type:     soundgasmpkg.ContentTypeAudio,
			ID:       "audio_1",
			Title:    "audio title",
		}, map[string]any{"id": "audio_1"}, nil, nil),
		Variants: []contentdownload.Variant{
			{ID: "audio", Type: "audio", Label: "M4A", Suffix: ".m4a"},
		},
		Defaults: contentdownload.Defaults{VariantID: "audio", Suffix: ".m4a"},
	}

	platformProbeAddJSONDefault(probe)

	if probe.Defaults.VariantID != "audio" || probe.Defaults.Suffix != ".m4a" {
		t.Fatalf("defaults = %#v", probe.Defaults)
	}
	if !hasProbeVariant(probe.Variants, platformJSONVariantID) {
		t.Fatalf("expected JSON variant: %#v", probe.Variants)
	}
}

func hasProbeVariant(variants []contentdownload.Variant, id string) bool {
	for _, variant := range variants {
		if variant.ID == id {
			return true
		}
	}
	return false
}
