package youtube

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
)

func TestParseSamplePage(t *testing.T) {
	body, err := os.ReadFile("../../../youtube_260614.html")
	if err != nil {
		t.Fatal(err)
	}

	rawPlayerResponse, ok, err := ExtractInitialPlayerResponseJSON(body)
	if err != nil {
		t.Fatalf("ExtractInitialPlayerResponseJSON: %v", err)
	}
	if !ok || !json.Valid(rawPlayerResponse) {
		t.Fatalf("player response json ok=%v valid=%v", ok, json.Valid(rawPlayerResponse))
	}
	rawYTCfg, ok, err := ExtractYTCfgJSON(body)
	if err == nil && (!ok || !json.Valid(rawYTCfg)) {
		t.Fatalf("ytcfg json ok=%v valid=%v", ok, json.Valid(rawYTCfg))
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

	client := NewClient(server.Client())
	client.BaseURL = server.URL
	info, err := client.Extract(context.Background(), "https://www.youtube.com/watch?v=3ryh7PNhz3E")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if info.ID != "3ryh7PNhz3E" {
		t.Fatalf("id = %q", info.ID)
	}
	if info.Title != "The Best & Worst Glute Exercises (According To Science)" {
		t.Fatalf("title = %q", info.Title)
	}
	if info.Duration != 839 || info.Channel != "Jeff Nippard" {
		t.Fatalf("duration=%d channel=%q", info.Duration, info.Channel)
	}
	const channelURL = "http://www.youtube.com/@JeffNippard"
	const channelAvatarURL = "https://yt3.ggpht.com/ytc/AIdro_k3d_sCJXZcQk5KQTlFzdGMIJwJpZ9g2W07Z616E5DENGI=s176-c-k-c0x00ffffff-no-rj"
	if info.ChannelURL != channelURL || info.UploaderURL != channelURL {
		t.Fatalf("channel urls channel=%q uploader=%q", info.ChannelURL, info.UploaderURL)
	}
	if info.ChannelAvatarURL != channelAvatarURL || info.UploaderAvatarURL != channelAvatarURL {
		t.Fatalf("avatar urls channel=%q uploader=%q", info.ChannelAvatarURL, info.UploaderAvatarURL)
	}
	if len(info.Formats) == 0 {
		t.Fatal("expected formats")
	}
	if len(info.InitialPlayerResponseJSON) == 0 || info.PageHTML == "" {
		t.Fatalf("missing raw page data: player=%d ytcfg=%d html=%d", len(info.InitialPlayerResponseJSON), len(info.YTCfgJSON), len(info.PageHTML))
	}

	probe, err := client.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.youtube.com/watch?v=3ryh7PNhz3E"})
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	summary := contentdownload.ContentSummaryOf(probe.Content)
	if summary.AuthorAvatarURL != channelAvatarURL {
		t.Fatalf("summary author avatar = %q", summary.AuthorAvatarURL)
	}
	if metadata := contentdownload.ContentMetadataOf(probe.Content); metadata["channel_url"] != channelURL {
		t.Fatalf("metadata channel_url = %#v", metadata["channel_url"])
	}
}
