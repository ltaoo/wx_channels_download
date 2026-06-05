package youtube

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
)

func TestExtractVideoID(t *testing.T) {
	tests := map[string]string{
		"https://www.youtube.com/watch?v=abc123def45&feature=share": "abc123def45",
		"https://youtu.be/xyz789abc12?t=1":                          "xyz789abc12",
		"https://www.youtube.com/shorts/short123456":                "short123456",
		"https://www.youtube.com/embed/embed123456":                 "embed123456",
		"https://www.youtube.com/live/live1234567":                  "live1234567",
		"BaW_jenozKc": "BaW_jenozKc",
	}
	for rawURL, want := range tests {
		got, ok := ExtractVideoID(rawURL)
		if !ok {
			t.Fatalf("ExtractVideoID(%q) returned false", rawURL)
		}
		if got != want {
			t.Fatalf("ExtractVideoID(%q) = %q, want %q", rawURL, got, want)
		}
	}
}

func TestExtractVideoIDRejectsInvalidIDs(t *testing.T) {
	for _, rawURL := range []string{
		"abc123",
		"https://www.youtube.com/watch?v=abc123",
		"https://example.com/watch?v=abc123def45",
	} {
		if got, ok := ExtractVideoID(rawURL); ok {
			t.Fatalf("ExtractVideoID(%q) = %q, want unsupported", rawURL, got)
		}
	}
}

func TestProbeAndResolveFromInitialPlayerResponse(t *testing.T) {
	videoURL := "https://rr.example.com/videoplayback?itag=22&n=slow"
	audioURL := "https://rr.example.com/videoplayback?itag=140"
	coverURL := "https://i.ytimg.com/vi/abc123def45/hqdefault.jpg"
	server := youtubeTestServer(t, fmt.Sprintf(`{
		"playabilityStatus":{"status":"OK","playableInEmbed":true},
		"videoDetails":{
			"videoId":"abc123def45",
			"title":"demo title",
			"shortDescription":"demo description",
			"lengthSeconds":"42",
			"channelId":"UC1",
			"author":"demo channel",
			"viewCount":"123",
			"keywords":["tag1","tag2"],
			"thumbnail":{"thumbnails":[{"url":%q,"width":480,"height":360}]}
		},
		"microformat":{"playerMicroformatRenderer":{
			"category":"Education",
			"isFamilySafe":true,
			"ownerProfileUrl":"https://www.youtube.com/@demo"
		}},
		"streamingData":{
			"formats":[
				{"itag":18,"url":"https://rr.example.com/videoplayback?itag=18","mimeType":"video/mp4; codecs=\"avc1.42001E, mp4a.40.2\"","qualityLabel":"360p","width":640,"height":360,"bitrate":500000,"contentLength":"1000"},
				{"itag":22,"url":%q,"mimeType":"video/mp4; codecs=\"avc1.64001F, mp4a.40.2\"","qualityLabel":"720p","width":1280,"height":720,"bitrate":1500000,"contentLength":"2000"}
			],
			"adaptiveFormats":[
				{"itag":140,"url":%q,"mimeType":"audio/mp4; codecs=\"mp4a.40.2\"","audioQuality":"AUDIO_QUALITY_MEDIUM","bitrate":128000,"contentLength":"500"},
				{"itag":137,"url":"https://rr.example.com/videoplayback?itag=137","mimeType":"video/mp4; codecs=\"avc1.640028\"","qualityLabel":"1080p","width":1920,"height":1080,"bitrate":3000000},
				{"itag":248,"signatureCipher":"url=https%%3A%%2F%%2Frr.example.com%%2Fvideoplayback%%3Fitag%%3D248&s=encrypted&sp=sig","mimeType":"video/webm; codecs=\"vp9\"","qualityLabel":"1080p"}
			],
			"hlsManifestUrl":"https://manifest.example.com/index.m3u8"
		}
	}`, coverURL, videoURL, audioURL))
	defer server.Close()

	h := New(&Client{HTTPClient: server.Client(), BaseURL: server.URL, UserAgent: "test-agent"})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.youtube.com/watch?v=abc123def45"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if probe.ContentID != "abc123def45" || contentdownload.ContentTitle(probe.Content) != "demo title" {
		t.Fatalf("probe content = %#v", probe)
	}
	if contentdownload.ContentDuration(probe.Content) != 42 ||
		contentdownload.ContentAuthorNickname(probe.Content) != "demo channel" ||
		contentdownload.ContentCoverURL(probe.Content) != coverURL {
		t.Fatalf("probe summary = %#v", contentdownload.ContentSummaryOf(probe.Content))
	}
	if probe.Defaults.VariantID != "format_22" {
		t.Fatalf("default variant = %q", probe.Defaults.VariantID)
	}
	if !hasVariant(probe, "format_18") || !hasVariant(probe, "format_22") || !hasVariant(probe, "audio_mp3") || !hasVariant(probe, "cover") {
		t.Fatalf("variants = %#v", probe.Variants)
	}
	if !containsWarning(probe.Warnings, "解签") || !containsWarning(probe.Warnings, "n challenge") || !containsWarning(probe.Warnings, "HLS/DASH") {
		t.Fatalf("warnings = %#v", probe.Warnings)
	}

	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: "https://www.youtube.com/watch?v=abc123def45", Probe: probe})
	if err != nil {
		t.Fatalf("Resolve video: %v", err)
	}
	if resolved.Download.URL != videoURL || resolved.Suffix != ".mp4" {
		t.Fatalf("resolved video url=%q suffix=%q", resolved.Download.URL, resolved.Suffix)
	}
	if resolved.Download.Headers["Referer"] != "https://www.youtube.com/watch?v=abc123def45" ||
		resolved.Download.Headers["User-Agent"] != "test-agent" {
		t.Fatalf("headers = %#v", resolved.Download.Headers)
	}
	if resolved.Metadata["format_id"] != "22" || resolved.Metadata["format_type"] != "progressive" {
		t.Fatalf("metadata = %#v", resolved.Metadata)
	}
	if resolved.Pipeline == nil || len(resolved.Pipeline.Nodes) == 0 {
		t.Fatal("expected pipeline plan")
	}

	audio, err := h.Resolve(context.Background(), contentdownload.ResolveInput{
		URL:   "https://www.youtube.com/watch?v=abc123def45",
		Probe: probe,
		Options: contentdownload.Options{
			VariantID: "audio_mp3",
		},
	})
	if err != nil {
		t.Fatalf("Resolve audio: %v", err)
	}
	if audio.Download.URL != audioURL || audio.Suffix != ".mp3" {
		t.Fatalf("resolved audio url=%q suffix=%q", audio.Download.URL, audio.Suffix)
	}

	cover, err := h.Resolve(context.Background(), contentdownload.ResolveInput{
		URL:   "https://www.youtube.com/watch?v=abc123def45",
		Probe: probe,
		Options: contentdownload.Options{
			VariantID: "cover",
		},
	})
	if err != nil {
		t.Fatalf("Resolve cover: %v", err)
	}
	if cover.Download.URL != coverURL || cover.Suffix != ".jpg" {
		t.Fatalf("resolved cover url=%q suffix=%q", cover.Download.URL, cover.Suffix)
	}
}

func TestProbeFallsBackToPlayerAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/watch":
			_, _ = w.Write([]byte(`<script>ytcfg.set({"INNERTUBE_API_KEY":"test-key","INNERTUBE_CONTEXT":{"client":{"clientName":"WEB","clientVersion":"1.0"}}});</script>`))
		case "/youtubei/v1/player":
			if r.URL.Query().Get("key") != "test-key" {
				t.Fatalf("api key = %q", r.URL.Query().Get("key"))
			}
			w.Header().Set("content-type", "application/json")
			_, _ = w.Write([]byte(`{
				"playabilityStatus":{"status":"OK"},
				"videoDetails":{"videoId":"api123def45","title":"api title","lengthSeconds":"9"},
				"streamingData":{"formats":[{"itag":18,"url":"https://rr.example.com/api.mp4","mimeType":"video/mp4; codecs=\"avc1.42001E, mp4a.40.2\"","qualityLabel":"360p","height":360}]}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	h := New(&Client{HTTPClient: server.Client(), BaseURL: server.URL})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.youtube.com/watch?v=api123def45"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if contentdownload.ContentTitle(probe.Content) != "api title" || probe.Defaults.VariantID != "format_18" {
		t.Fatalf("probe = %#v", probe)
	}
}

func TestProbeReturnsUnavailableError(t *testing.T) {
	server := youtubeTestServer(t, `{
		"playabilityStatus":{"status":"LOGIN_REQUIRED","reason":"Sign in to confirm your age"},
		"videoDetails":{"videoId":"age123def45","title":"age gated"}
	}`)
	defer server.Close()

	h := New(&Client{HTTPClient: server.Client(), BaseURL: server.URL})
	_, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.youtube.com/watch?v=age123def45"})
	if err == nil || !strings.Contains(err.Error(), "Sign in") {
		t.Fatalf("Probe error = %v", err)
	}
}

func TestProbeRejectsMismatchedPlayerResponse(t *testing.T) {
	server := youtubeTestServer(t, `{
		"playabilityStatus":{"status":"OK"},
		"videoDetails":{"videoId":"other123456","title":"other video"},
		"streamingData":{"formats":[{"itag":18,"url":"https://rr.example.com/video.mp4","mimeType":"video/mp4; codecs=\"avc1.42001E, mp4a.40.2\""}]}
	}`)
	defer server.Close()

	h := New(&Client{HTTPClient: server.Client(), BaseURL: server.URL})
	_, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.youtube.com/watch?v=real1234567"})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("Probe error = %v", err)
	}
}

func youtubeTestServer(t *testing.T, playerResponse string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/watch" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("content-type", "text/html")
		_, _ = w.Write([]byte(`<html><script>var ytInitialPlayerResponse = ` + playerResponse + `;</script></html>`))
	}))
}

func hasVariant(probe *contentdownload.Probe, id string) bool {
	for _, variant := range probe.Variants {
		if variant.ID == id {
			return true
		}
	}
	return false
}

func containsWarning(warnings []string, needle string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, needle) {
			return true
		}
	}
	return false
}
