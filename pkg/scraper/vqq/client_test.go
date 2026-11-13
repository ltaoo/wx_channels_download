package vqq

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"wx_channel/pkg/scraper"
)

func tencent_nested_metadata_fixture() string {
	return platform_json(map[string]any{"ret": 0, "data": map[string]any{"CardList": []any{
		map[string]any{"type": "pc_introduction", "params": map[string]string{"mod_title": "尚公主"}, "children_list": map[string]any{"0": map[string]any{"cards": []any{map[string]any{"params": map[string]string{
			"report.cid": test_tencent_cover, "title": "尚公主", "type": "2", "cover_description": "剧集简介", "new_pic_vt": "https://puui.qpic.cn/poster.jpg", "new_pic_hz": "https://puui.qpic.cn/backdrop.jpg", "year": "2026", "broadcast_time": "2026-10-23", "episode_all": "40", "area_name": "内地", "main_genres": "爱情", "sub_genre": "古装", "detail_info": "更新至33集 · 全40集", "matrix_infos": "private request context",
		}}}}}},
		map[string]any{"type": "pc_web_episode_list", "params": map[string]string{"is_all_data": "true", "sub_title": "更新至33集", "tabs": `[{"begin":1,"end":30,"selected":true},{"begin":31,"end":37,"selected":false}]`}, "children_list": map[string]any{
			"0": map[string]any{"cards": []any{map[string]any{"params": map[string]string{"cid": test_tencent_cover, "vid": test_tencent_first, "title": "先导片", "is_trailer": "0", "duration": "54", "c_title_output": "55秒故事前情", "play_title": "先导片", "image_url": "https://puui.qpic.cn/intro.jpg"}}}},
			"1": tencent_episode_fixture(test_tencent_second, "1", "0"),
			"2": tencent_episode_fixture("x4102olnagg", "2", "1"),
		}},
	}}})
}

func TestFetchDetailsPreservesSeriesAndExplicitIntroIdentity(t *testing.T) {
	requests := 0
	api := mock_client(t, func(req *http.Request) (string, int) {
		requests++
		if !strings.HasSuffix(req.URL.Path, "/getPage") {
			t.Fatalf("metadata fetch requested a playback endpoint: %s", req.URL.Path)
		}
		return tencent_nested_metadata_fixture(), 200
	})
	client := NewClient("")
	client.SetHTTPClient(api.HTTPClient)
	page := "https://v.qq.com/x/cover/" + test_tencent_cover + "/" + test_tencent_first + ".html"
	result, err := client.FetchDetails(page)
	if err != nil {
		t.Fatal(err)
	}
	profile := result.Profile
	if requests != 1 || profile.ID != test_tencent_cover || profile.Kind != "series" || profile.Title != "尚公主" || profile.Description != "剧集简介" || profile.CoverURL == "" || profile.BackdropURL == "" || profile.ReleaseYear != 2026 || profile.ReleaseDate != "2026-10-23" || profile.EpisodeCount != 40 || profile.AvailableEpisodeCount != 1 || len(profile.Genres) != 2 || len(profile.Countries) != 1 {
		t.Fatalf("nested native metadata lost: %+v", profile)
	}
	if strings.Contains(string(profile.Metadata), "private") {
		t.Fatal("reporting context leaked into content metadata")
	}
	if len(result.Episodes) != 1 {
		t.Fatalf("explicit video became a series download: %+v", result.Episodes)
	}
	episode := result.Episodes[0]
	if episode.ID != test_tencent_first || episode.Badge != "先导片" || episode.Order != 0 || episode.Duration != 54 || episode.Title != "55秒故事前情" || episode.CoverURL == "" {
		t.Fatalf("intro was mislabeled as the first full episode: %+v", episode)
	}
	// The actual feature is a different ID and retains its source episode order.
	feature, err := client.FetchDetails("https://v.qq.com/x/cover/" + test_tencent_cover + "/" + test_tencent_second + ".html")
	if err != nil || len(feature.Episodes) != 1 || feature.Episodes[0].ID != test_tencent_second || feature.Episodes[0].Order != 1 || feature.Episodes[0].Badge != "" {
		t.Fatalf("first full episode changed identity: %+v %v", feature, err)
	}
	// is_all_data=true applies only to one tab. Never silently drop later episodes.
	if _, err := client.FetchDetails("https://v.qq.com/x/cover/" + test_tencent_cover + ".html"); err == nil || !strings.Contains(err.Error(), "分页") {
		t.Fatalf("accepted incomplete full-series catalogue: %v", err)
	}
}

func TestTencentPrefersAVCAndPreservesReturnedCodec(t *testing.T) {
	for _, row := range []struct {
		hevc  int
		codec string
	}{{0, "h264"}, {1, "hevc"}} {
		t.Run(row.codec, func(t *testing.T) {
			api := mock_client(t, func(req *http.Request) (string, int) {
				switch req.URL.Path {
				case "/getvinfo":
					if req.URL.Query().Get("hevclv") != "0" {
						t.Error("requested HEVC instead of compatible AVC")
					}
					text := strings.TrimSuffix(strings.TrimPrefix(tencent_playback_fixture(test_tencent_first, "0", 2), "QZOutputJson="), ";")
					var payload map[string]any
					json.Unmarshal([]byte(text), &payload)
					payload["vl"].(map[string]any)["vi"].([]any)[0].(map[string]any)["hevc"] = row.hevc
					return "QZOutputJson=" + platform_json(payload) + ";", 200
				case "/vod/index.m3u8":
					return "#EXTM3U\n#EXTINF:2,\npart.ts\n#EXT-X-ENDLIST\n", 200
				case "/vod/part.ts":
					return strings.Repeat("fixture media", 100), 200
				default:
					t.Errorf("unexpected playback path %s", req.URL.Path)
					return "", 404
				}
			})
			client := NewClient("")
			client.SetHTTPClient(api.HTTPClient)
			result, err := client.ResolveContext(context.Background(), "https://v.qq.com/x/cover/"+test_tencent_cover+"/"+test_tencent_first+".html", scraper.Episode{ID: test_tencent_first, Duration: 2})
			if err != nil || result.Info.Codec != row.codec || result.Playlist == "" {
				t.Fatalf("codec lost before postprocessing: %+v %v", result.Info, err)
			}
		})
	}
}
