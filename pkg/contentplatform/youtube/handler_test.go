package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestExtractYTCfgSkipsNonObjectSetCalls(t *testing.T) {
	webpage := []byte(`<script>window.ytcfg.set('EMERGENCY_BASE_URL','/error_204');</script>` +
		`<script>ytcfg.set({"PLAYER_JS_URL":"/s/player/abc/player_ias.vflset/en_US/base.js","INNERTUBE_API_KEY":"key"});</script>`)
	ytcfg, ok := parseYTCfg(webpage)
	if !ok {
		t.Fatal("parseYTCfg returned false")
	}
	if got := stringFromMap(ytcfg, "PLAYER_JS_URL"); got != "/s/player/abc/player_ias.vflset/en_US/base.js" {
		t.Fatalf("PLAYER_JS_URL = %q", got)
	}
	client := NewClient(nil)
	if got := client.extractPlayerJSURL(webpage, ytcfg); got != "https://www.youtube.com/s/player/abc/player_ias.vflset/en_US/base.js" {
		t.Fatalf("player JS URL = %q", got)
	}
}

func TestProbeExtractsAuthorAvatarFromInitialData(t *testing.T) {
	body, err := os.ReadFile("../../../youtube_260614.html")
	if err != nil {
		t.Fatal(err)
	}
	rawInitialData, ok, err := ExtractInitialDataJSON(body)
	if err != nil {
		t.Fatalf("ExtractInitialDataJSON: %v", err)
	}
	if !ok || !json.Valid(rawInitialData) {
		t.Fatalf("initial data json ok=%v valid=%v", ok, json.Valid(rawInitialData))
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/watch" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("content-type", "text/html")
		_, _ = w.Write(body)
	}))
	defer server.Close()

	h := New(&Client{HTTPClient: server.Client(), BaseURL: server.URL})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.youtube.com/watch?v=3ryh7PNhz3E"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	summary := contentdownload.ContentSummaryOf(probe.Content)
	const channelURL = "http://www.youtube.com/@JeffNippard"
	const channelAvatarURL = "https://yt3.ggpht.com/ytc/AIdro_k3d_sCJXZcQk5KQTlFzdGMIJwJpZ9g2W07Z616E5DENGI=s176-c-k-c0x00ffffff-no-rj"
	if summary.AuthorAvatarURL != channelAvatarURL {
		t.Fatalf("summary author avatar = %q", summary.AuthorAvatarURL)
	}
	metadata := contentdownload.ContentMetadataOf(probe.Content)
	if metadata["author_homepage_url"] != channelURL ||
		metadata["channel_url"] != channelURL ||
		metadata["channel_avatar_url"] != channelAvatarURL {
		t.Fatalf("metadata = %#v", metadata)
	}
	output := contentdownload.ContentOutputOf(probe.Content)
	if output["author_homepage_url"] != channelURL ||
		output["channel_url"] != channelURL ||
		output["channel_avatar_url"] != channelAvatarURL ||
		output["author_avatar_url"] != channelAvatarURL {
		t.Fatalf("output = %#v", output)
	}
}

func TestProbeAndResolveFromInitialPlayerResponse(t *testing.T) {
	videoURL := "https://rr.example.com/videoplayback?itag=22&n=wols&pot=test-pot"
	audioURL := "https://rr.example.com/videoplayback?itag=140&pot=test-pot"
	coverURL := "https://i.ytimg.com/vi/abc123def45/hqdefault.jpg"
	seenCookie := false
	server := youtubeTestServerWithHook(t, fmt.Sprintf(`{
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
				{"itag":22,"url":"https://rr.example.com/videoplayback?itag=22&n=slow","mimeType":"video/mp4; codecs=\"avc1.64001F, mp4a.40.2\"","qualityLabel":"720p","width":1280,"height":720,"bitrate":1500000,"contentLength":"2000"}
			],
			"adaptiveFormats":[
				{"itag":140,"url":%q,"mimeType":"audio/mp4; codecs=\"mp4a.40.2\"","audioQuality":"AUDIO_QUALITY_MEDIUM","bitrate":128000,"contentLength":"500"},
				{"itag":137,"url":"https://rr.example.com/videoplayback?itag=137","mimeType":"video/mp4; codecs=\"avc1.640028\"","qualityLabel":"1080p","width":1920,"height":1080,"bitrate":3000000},
				{"itag":248,"signatureCipher":"url=https%%3A%%2F%%2Frr.example.com%%2Fvideoplayback%%3Fitag%%3D248&s=encrypted&sp=sig","mimeType":"video/webm; codecs=\"vp9\"","qualityLabel":"1080p"}
			],
			"hlsManifestUrl":"https://manifest.example.com/index.m3u8"
		}
	}`, coverURL, audioURL), func(r *http.Request) {
		if r.URL.Path == "/watch" && r.Header.Get("Cookie") == "SID=test-cookie" {
			seenCookie = true
		}
	})
	defer server.Close()

	h := New(&Client{HTTPClient: server.Client(), BaseURL: server.URL, UserAgent: "test-agent", Cookie: "SID=test-cookie", PoToken: "web.gvs+test-pot"})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.youtube.com/watch?v=abc123def45"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !seenCookie {
		t.Fatal("watch request did not receive youtube cookie")
	}
	if probe.ContentID != "abc123def45" || contentdownload.ContentTitle(probe.Content) != "demo title" {
		t.Fatalf("probe content = %#v", probe)
	}
	if contentdownload.ContentDuration(probe.Content) != 42 ||
		contentdownload.ContentAuthorNickname(probe.Content) != "demo channel" ||
		contentdownload.ContentCoverURL(probe.Content) != coverURL {
		t.Fatalf("probe summary = %#v", contentdownload.ContentSummaryOf(probe.Content))
	}
	if probe.Defaults.VariantID != "best" {
		t.Fatalf("default variant = %q", probe.Defaults.VariantID)
	}
	if !hasVariant(probe, "best") || !hasVariant(probe, "format_18") || !hasVariant(probe, "format_22") || !hasVariant(probe, "audio_mp3") || !hasVariant(probe, "cover") || !hasVariant(probe, "player_response_json") {
		t.Fatalf("variants = %#v warnings=%#v", probe.Variants, probe.Warnings)
	}
	if info := videoInfoFromProbe(probe); info == nil || info.FindFormat("248") == nil || !strings.Contains(info.FindFormat("248").URL, "sig=detpyrcne") {
		t.Fatalf("decoded signature format missing: %#v", info)
	}
	if !containsWarning(probe.Warnings, "已解算部分 YouTube player JS 签名格式") || !containsWarning(probe.Warnings, "已解算 YouTube n challenge") || !containsWarning(probe.Warnings, "HLS/DASH") {
		t.Fatalf("warnings = %#v", probe.Warnings)
	}

	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{URL: "https://www.youtube.com/watch?v=abc123def45", Probe: probe})
	if err != nil {
		t.Fatalf("Resolve video: %v", err)
	}
	if resolved.Download.Protocol != contentdownload.ProtocolMultiHTTP || resolved.Suffix != ".mp4" {
		t.Fatalf("resolved video protocol=%q url=%q suffix=%q", resolved.Download.Protocol, resolved.Download.URL, resolved.Suffix)
	}
	if resolved.Metadata["format_id"] != "137+140" || resolved.Metadata["format_type"] != "merged" {
		t.Fatalf("metadata = %#v", resolved.Metadata)
	}
	sources, ok := resolved.Metadata["sources"].([]contentdownload.MultiSourceSpec)
	if !ok || len(sources) != 2 {
		t.Fatalf("sources = %#v", resolved.Metadata["sources"])
	}
	if sources[0].URL != "https://rr.example.com/videoplayback?itag=137&pot=test-pot" || sources[1].URL != audioURL {
		t.Fatalf("sources = %#v", sources)
	}
	if sources[0].Headers["Referer"] != "https://www.youtube.com/watch?v=abc123def45" ||
		sources[0].Headers["User-Agent"] != "test-agent" ||
		sources[0].Headers["Cookie"] != "SID=test-cookie" {
		t.Fatalf("source headers = %#v", sources[0].Headers)
	}
	if resolved.Pipeline == nil || len(resolved.Pipeline.Nodes) == 0 {
		t.Fatal("expected pipeline plan")
	}
	if resolved.Pipeline.Nodes[0].Args["merge"] != "ffmpeg" {
		t.Fatalf("pipeline = %#v", resolved.Pipeline)
	}

	single, err := h.Resolve(context.Background(), contentdownload.ResolveInput{
		URL:   "https://www.youtube.com/watch?v=abc123def45",
		Probe: probe,
		Options: contentdownload.Options{
			VariantID: "format_22",
		},
	})
	if err != nil {
		t.Fatalf("Resolve single video: %v", err)
	}
	if single.Download.Protocol != "http" || single.Download.URL != videoURL || single.Suffix != ".mp4" {
		t.Fatalf("resolved single protocol=%q url=%q suffix=%q", single.Download.Protocol, single.Download.URL, single.Suffix)
	}
	if single.Metadata["format_id"] != "22" || single.Metadata["format_type"] != "progressive" {
		t.Fatalf("single metadata = %#v", single.Metadata)
	}

	noFFmpeg, err := h.Resolve(context.Background(), contentdownload.ResolveInput{
		URL:   "https://www.youtube.com/watch?v=abc123def45",
		Probe: probe,
		Options: contentdownload.Options{
			Extra: map[string]any{"ffmpeg_available": false},
		},
	})
	if err != nil {
		t.Fatalf("Resolve without ffmpeg: %v", err)
	}
	if noFFmpeg.Download.Protocol != "http" || noFFmpeg.Download.URL != videoURL || noFFmpeg.Metadata["format_id"] != "22" {
		t.Fatalf("no-ffmpeg fallback = %#v", noFFmpeg)
	}

	playerJSON, err := h.Resolve(context.Background(), contentdownload.ResolveInput{
		URL:   "https://www.youtube.com/watch?v=abc123def45",
		Probe: probe,
		Options: contentdownload.Options{
			VariantID: "player_response_json",
		},
	})
	if err != nil {
		t.Fatalf("Resolve player response JSON: %v", err)
	}
	if playerJSON.Download.Protocol != "inline_json" || playerJSON.Suffix != ".json" {
		t.Fatalf("resolved json protocol=%q suffix=%q", playerJSON.Download.Protocol, playerJSON.Suffix)
	}
	raw, ok := playerJSON.Metadata["json"].(json.RawMessage)
	if !ok || !json.Valid(raw) {
		t.Fatalf("resolved json = %#v", playerJSON.Metadata["json"])
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
	if audio.Download.Protocol != "http" || audio.Download.URL != audioURL || audio.Suffix != ".mp3" {
		t.Fatalf("resolved audio protocol=%q url=%q suffix=%q", audio.Download.Protocol, audio.Download.URL, audio.Suffix)
	}
	if audio.Metadata["direct_url"] != audioURL {
		t.Fatalf("audio direct_url = %q, want %q", audio.Metadata["direct_url"], audioURL)
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
			_, _ = w.Write([]byte(`<script>ytcfg.set({"INNERTUBE_API_KEY":"test-key","VISITOR_DATA":"visitor-test","INNERTUBE_CONTEXT":{"client":{"clientName":"WEB","clientVersion":"1.0"}}});</script>`))
		case "/youtubei/v1/player":
			if r.URL.Query().Get("prettyPrint") != "false" {
				t.Fatalf("prettyPrint = %q", r.URL.Query().Get("prettyPrint"))
			}
			if r.Header.Get("X-YouTube-Client-Name") == "28" {
				if r.URL.Query().Get("key") != "" {
					t.Fatalf("android vr api key = %q", r.URL.Query().Get("key"))
				}
				if r.Header.Get("X-Goog-Visitor-Id") != "visitor-test" {
					t.Fatalf("visitor id = %q", r.Header.Get("X-Goog-Visitor-Id"))
				}
			} else if r.URL.Query().Get("key") != "test-key" {
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

func TestProbeSamplePage(t *testing.T) {
	body, err := os.ReadFile("../../../youtube_260614.html")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/watch" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("content-type", "text/html")
		_, _ = w.Write(body)
	}))
	defer server.Close()

	h := New(&Client{HTTPClient: server.Client(), BaseURL: server.URL})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.youtube.com/watch?v=3ryh7PNhz3E"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if probe.ContentID != "3ryh7PNhz3E" || contentdownload.ContentTitle(probe.Content) != "The Best & Worst Glute Exercises (According To Science)" {
		t.Fatalf("probe = %#v", probe)
	}
	if !hasVariant(probe, "player_response_json") {
		t.Fatalf("variants = %#v", probe.Variants)
	}
	if raw, ok := probe.Internal["pagejson"].(json.RawMessage); !ok || !json.Valid(raw) {
		t.Fatalf("probe pagejson = %#v", probe.Internal["pagejson"])
	}
}

func youtubeTestServer(t *testing.T, playerResponse string) *httptest.Server {
	t.Helper()
	return youtubeTestServerWithHook(t, playerResponse, nil)
}

func youtubeTestServerWithHook(t *testing.T, playerResponse string, hook func(*http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hook != nil {
			hook(r)
		}
		switch r.URL.Path {
		case "/watch":
			w.Header().Set("content-type", "text/html")
			_, _ = w.Write([]byte(`<html><script>ytcfg.set({"PLAYER_JS_URL":"/s/player/test/player_es6.vflset/en_US/base.js"});</script><script>var ytInitialPlayerResponse = ` + playerResponse + `;</script></html>`))
		case "/s/player/test/player_es6.vflset/en_US/base.js":
			w.Header().Set("content-type", "application/javascript")
			_, _ = w.Write([]byte(`function sig(a){a=a.split("");a.reverse();return a.join("")} function nn(a){a=a.split("");a.reverse();return a.join("")} c.get("n"))&&(b=nn(b));`))
		default:
			http.NotFound(w, r)
		}
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
