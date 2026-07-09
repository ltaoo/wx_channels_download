package soundgasm

import (
	"context"
	"strings"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
	soundgasmpkg "wx_channel/pkg/scraper/soundgasm"
)

type fakeAudioFetcher struct{}

func (fakeAudioFetcher) FetchAudioPage(rawURL string) (*soundgasmpkg.AudioPage, error) {
	return &soundgasmpkg.AudioPage{
		ID:              "BrittanyBabbles_Cum-Quietly-Baby-We-Cant-Wake-Up-Daddy",
		URL:             rawURL,
		CanonicalURL:    "https://soundgasm.net/u/BrittanyBabbles/Cum-Quietly-Baby-We-Cant-Wake-Up-Daddy",
		Title:           "Cum Quietly, Baby. We Can\u2019t Wake Up Daddy",
		Description:     "audio description",
		DescriptionHTML: "<p>audio description</p>",
		Author:          soundgasmpkg.Author{Name: "BrittanyBabbles", URL: "https://soundgasm.net/u/BrittanyBabbles"},
		AudioURL:        "https://media.soundgasm.net/sounds/demo.m4a",
		AudioType:       "m4a",
		Tags:            []string{"Fdom", "Stealth Orgasm"},
	}, nil
}

func TestMatchProbeResolve(t *testing.T) {
	h := New(fakeAudioFetcher{})
	if !h.Match("https://soundgasm.net/u/BrittanyBabbles/Cum-Quietly-Baby-We-Cant-Wake-Up-Daddy") {
		t.Fatal("expected soundgasm match")
	}
	if h.Match("https://soundgasm.net/u/BrittanyBabbles") {
		t.Fatal("unexpected profile match")
	}

	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://soundgasm.net/u/BrittanyBabbles/Cum-Quietly-Baby-We-Cant-Wake-Up-Daddy"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if probe.Platform != PlatformID || probe.ContentID != "BrittanyBabbles_Cum-Quietly-Baby-We-Cant-Wake-Up-Daddy" {
		t.Fatalf("probe = %#v", probe)
	}
	if probe.Defaults.VariantID != audioVariantID || probe.Defaults.Suffix != ".m4a" {
		t.Fatalf("defaults = %#v", probe.Defaults)
	}
	if contentdownload.ContentType(probe.Content) != soundgasmpkg.ContentTypeAudio {
		t.Fatalf("content type = %q", contentdownload.ContentType(probe.Content))
	}
	if contentdownload.ContentAuthorHomepageURL(probe.Content) != "https://soundgasm.net/u/BrittanyBabbles" {
		t.Fatalf("author homepage = %q", contentdownload.ContentAuthorHomepageURL(probe.Content))
	}
	output := contentdownload.ContentOutputOf(probe.Content)
	bodyHTML, _ := output["body_html"].(string)
	if !strings.Contains(bodyHTML, "<audio controls") || output["audio_url"] != "https://media.soundgasm.net/sounds/demo.m4a" {
		t.Fatalf("output = %#v", output)
	}
	if !hasVariant(probe.Variants, audioVariantID) || !hasVariant(probe.Variants, htmlVariantID) || !hasVariant(probe.Variants, jsonVariantID) {
		t.Fatalf("variants = %#v", probe.Variants)
	}

	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: probe.SourceURL, Probe: probe})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Download.Protocol != "http" || resolved.Download.URL != "https://media.soundgasm.net/sounds/demo.m4a" {
		t.Fatalf("resolved download = %#v", resolved.Download)
	}
	if resolved.Suffix != ".m4a" || resolved.Filename != "Cum Quietly, Baby. We Can\u2019t Wake Up Daddy" {
		t.Fatalf("resolved name fields = %#v", resolved)
	}
	if resolved.Labels["content_type"] != soundgasmpkg.ContentTypeAudio || resolved.Metadata["audio_type"] != "m4a" {
		t.Fatalf("resolved metadata labels = %#v %#v", resolved.Labels, resolved.Metadata)
	}
	if resolved.Pipeline == nil || len(resolved.Pipeline.Nodes) != 2 {
		t.Fatalf("pipeline = %#v", resolved.Pipeline)
	}

	htmlResolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{
		URL:     probe.SourceURL,
		Probe:   probe,
		Options: contentdownload.Options{VariantID: htmlVariantID},
	})
	if err != nil {
		t.Fatalf("Resolve HTML: %v", err)
	}
	if htmlResolved.Download.Protocol != "inline_html" || htmlResolved.Suffix != ".html" {
		t.Fatalf("html resolved = %#v", htmlResolved)
	}
	if body, _ := htmlResolved.Metadata["body_html"].(string); !strings.Contains(body, "<audio controls") {
		t.Fatalf("html body = %q", body)
	}

	jsonResolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{
		URL:     probe.SourceURL,
		Probe:   probe,
		Options: contentdownload.Options{VariantID: jsonVariantID},
	})
	if err != nil {
		t.Fatalf("Resolve JSON: %v", err)
	}
	if jsonResolved.Download.Protocol != "inline_json" || jsonResolved.Suffix != ".json" {
		t.Fatalf("json resolved = %#v", jsonResolved)
	}
	if jsonResolved.Metadata["json"] == nil {
		t.Fatalf("json payload missing: %#v", jsonResolved.Metadata)
	}
}

func hasVariant(variants []contentdownload.Variant, id string) bool {
	for _, variant := range variants {
		if variant.ID == id {
			return true
		}
	}
	return false
}
