package iqiyi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestPublicClientFetchAlbumSingleEpisodeAndResolve(t *testing.T) {
	first, second, album_id := "1487716838906800", "4942645144948600", "3993682943621001"
	segment, err := os.ReadFile("testdata/tiny.ts")
	if err != nil {
		t.Fatal(err)
	}
	playback_calls := 0
	api := mock_client(t, func(req *http.Request) (string, int) {
		switch req.URL.Path {
		case "/player/lw/lwplay/accelerator.js":
			return iqiyi_init_body(req.URL.Query().Get("tvId")), 200
		case "/tvg/v2/lw/base_info":
			return iqiyi_test_metadata(map[string]any{"videos": []any{iqiyi_test_row(first, 1), iqiyi_test_row(second, 2)}}), 200
		case "/dash":
			playback_calls++
			if req.Header.Get("Cookie") == "" {
				t.Error("missing account cookie on playback API")
			}
			return iqiyi_dash_body(req.URL.Query().Get("tvid"), "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2,\nhttps://pcw-data.video.iqiyi.com/segment.ts\n#EXT-X-ENDLIST\n", 2), 200
		case "/segment.ts":
			if req.Header.Get("Cookie") != "" {
				t.Error("account cookie leaked to media CDN")
			}
			return string(segment), 200
		default:
			if strings.HasPrefix(req.URL.Path, "/a_") || strings.HasPrefix(req.URL.Path, "/v_") {
				return `<script type="application/ld+json">{"@type":"TVSeries","name":"测试剧","numberOfEpisodes":2}</script>`, 200
			}
			t.Errorf("unexpected fixture request %s", req.URL)
			return "", 404
		}
	})
	client := NewClient("P00001=synthetic; P00003=123")
	client.SetHTTPClient(api.HTTPClient)
	result, err := client.Fetch(iqiyi_test_url(album_id, true))
	if err != nil {
		t.Fatal(err)
	}
	if result.Platform != PlatformID || len(result.Episodes) != 2 || playback_calls != 1 || result.Profile.Kind != "series" || result.Episodes[0].Duration != 2 {
		t.Fatalf("album metadata/lazy resolution mismatch: %+v", result)
	}
	single, err := client.Fetch(iqiyi_test_url(second, false))
	if err != nil {
		t.Fatal(err)
	}
	if len(single.Episodes) != 1 || single.Episodes[0].ID != second || single.Episodes[0].Order != 2 || single.Profile.ID != album_id {
		t.Fatalf("selected wrong episode or parent: %+v", single)
	}
	episode := result.Episodes[1]
	source, err := client.ResolveContext(context.Background(), episode.SourceURL, episode)
	if err != nil {
		t.Fatal(err)
	}
	if source.CheckMediaAddress == nil || source.CheckMediaAddress("https://evil.example/resource") == nil {
		t.Fatal("resolved playback lost its platform address policy")
	}
	if source.Info.Duration != 2 || !strings.Contains(source.Playlist, "https://pcw-data.video.iqiyi.com/segment.ts") {
		t.Fatalf("missing playback source: %+v", source)
	}
}

func TestFetchDetailsPreservesMetadataWhenPlaybackIsRegionRestricted(t *testing.T) {
	first, second := "1487716838906800", "4942645144948600"
	playback_calls := 0
	api := mock_client(t, func(req *http.Request) (string, int) {
		switch req.URL.Path {
		case "/player/lw/lwplay/accelerator.js":
			return iqiyi_init_body(second), 200
		case "/tvg/v2/lw/base_info":
			var payload map[string]any
			json.Unmarshal([]byte(iqiyi_test_metadata(map[string]any{"videos": []any{iqiyi_test_row(first, 1), iqiyi_test_row(second, 2)}})), &payload)
			base := payload["data"].(map[string]any)["base_data"].(map[string]any)
			base["desc"], base["current_video_year"] = "剧集简介", "1986"
			base["image_url"] = "https://pic0.iqiyipic.com/poster.jpg"
			base["horizontal_image_url"] = "https://pic0.iqiyipic.com/backdrop.jpg"
			base["channel_id"], base["total_episode"] = 2, 2
			base["publish_date"] = "2026-11-01 08:23"
			base["update_info"] = map[string]any{"album_finish": true, "update_status": "2集全"}
			raw, _ := json.Marshal(payload)
			return string(raw), 200
		case "/dash":
			playback_calls++
			return `{"code":"A00000","data":{"tvid":"` + second + `","st":502,"program":[]}}`, 200
		default:
			t.Errorf("unexpected request %s", req.URL.Path)
			return "", 404
		}
	})
	client := NewClient("")
	client.SetHTTPClient(api.HTTPClient)
	result, err := client.FetchDetails(iqiyi_test_url(second, false))
	if err != nil {
		t.Fatal(err)
	}
	if playback_calls != 0 {
		t.Fatal("metadata fetch attempted to resolve a stream")
	}
	profile := result.Profile
	if profile.ID != "3993682943621001" || profile.Kind != "series" || profile.Description != "剧集简介" || profile.CoverURL == "" || profile.BackdropURL == "" || profile.EpisodeCount != 2 || profile.AvailableEpisodeCount != 2 || profile.ReleaseYear != 1986 || profile.ReleaseDate != "" || profile.Status != "已完结" {
		t.Fatalf("native catalogue metadata lost or publication date became air date: %+v", profile)
	}
	if len(result.Episodes) != 1 || result.Episodes[0].ID != second || result.Episodes[0].Order != 2 || result.Episodes[0].URL != "" || result.Episodes[0].Duration != 0 {
		t.Fatalf("selected episode changed or unavailable media was invented: %+v", result.Episodes)
	}
	_, err = client.ResolveContext(context.Background(), result.SourceURL, result.Episodes[0])
	var playback_error *PlaybackError
	if !errors.As(err, &playback_error) || playback_error.Kind != "region" || playback_calls != 1 {
		t.Fatalf("download preparation did not preserve region denial: %v, calls=%d", err, playback_calls)
	}
	_, err = client.Fetch(result.SourceURL)
	if !errors.As(err, &playback_error) || playback_error.Kind != "region" || playback_calls != 2 {
		t.Fatalf("strict playback fetch lost its denial: %v, calls=%d", err, playback_calls)
	}
}
