package vqq

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

const test_tencent_cover = "mzc002001w361jz"
const test_tencent_first = "n3371za9efh"
const test_tencent_second = "v4102hrb7cq"

func TestTencentCKeyReference(t *testing.T) {
	raw, err := os.ReadFile("testdata/tencent-ckey.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		ID        string `json:"id"`
		Timestamp int64  `json:"timestamp"`
		Page      string `json:"page"`
		GUID      string `json:"guid"`
		CKey      string `json:"ckey"`
	}
	if json.Unmarshal(raw, &fixture) != nil {
		t.Fatal("invalid reference fixture")
	}
	if got := tencent_ckey(fixture.ID, fixture.Page, fixture.GUID, fixture.Timestamp); got != fixture.CKey {
		t.Fatal("cKey differs from independent Python AES reference")
	}
	for _, item := range []struct {
		raw        string
		duration   float64
		restricted bool
	}{
		{"0", 54, false}, {"54", 54, false}, {"60", 1948, true}, {"true", 54, true}, {`"0"`, 54, false}, {"54", 0, true},
	} {
		if preview_flag(json.RawMessage(item.raw), item.duration) != item.restricted {
			t.Errorf("preview %s duration %v", item.raw, item.duration)
		}
	}
}

func platform_json(value any) string { raw, _ := json.Marshal(value); return string(raw) }

func tencent_episode_fixture(id, title, trailer string) map[string]any {
	return map[string]any{"cards": []any{map[string]any{"params": map[string]string{"cid": test_tencent_cover, "vid": id, "title": title, "is_trailer": trailer, "duration": "2"}}}}
}

func tencent_listing_fixture(children map[string]any, params map[string]string) string {
	return platform_json(map[string]any{"ret": 0, "data": map[string]any{"CardList": []any{
		map[string]any{"type": "pc_introduction", "params": map[string]string{"mod_title": "测试专辑"}},
		map[string]any{"type": "pc_web_episode_list", "params": params, "children_list": children},
	}}})
}

func TestTencentListingFiltersAndSorts(t *testing.T) {
	children := map[string]any{"0": tencent_episode_fixture(test_tencent_second, "2", "0"), "1": tencent_episode_fixture("preview1234", "3", "1"), "2": tencent_episode_fixture(test_tencent_first, "1", "0"), "3": tencent_episode_fixture("intro123456", "先导片", "0")}
	api := mock_client(t, func(req *http.Request) (string, int) {
		if req.Method != "POST" {
			t.Error("listing must use POST")
		}
		var body map[string]any
		if json.NewDecoder(req.Body).Decode(&body) != nil {
			t.Fatal("listing request invalid")
		}
		if body["page_params"].(map[string]any)["cid"] != test_tencent_cover {
			t.Fatal("missing requested CID")
		}
		return tencent_listing_fixture(children, map[string]string{"is_all_data": "true"}), 200
	})
	client := new_test_backend(&backend{RequestClient: &sourceutil.RequestClient{HTTPClient: api.HTTPClient, Policy: source_policy}})
	rows, title, err := client.tencent_episodes(context.Background(), test_tencent_cover, "")
	if err != nil || title != "测试专辑" || len(rows) != 2 || rows[0].ID != test_tencent_first || rows[1].ID != test_tencent_second {
		t.Fatalf("bad listing: %+v %v", rows, err)
	}
	rows, _, err = client.tencent_episodes(context.Background(), test_tencent_cover, "intro123456")
	if err != nil || len(rows) != 1 || rows[0].ID != "intro123456" {
		t.Fatalf("explicit intro was lost: %+v %v", rows, err)
	}
	for _, mode := range []string{"pagination", "missing episode", "duplicate episode"} {
		params := map[string]string{}
		bad := map[string]any{"0": tencent_episode_fixture(test_tencent_second, "2", "0")}
		if mode == "pagination" {
			params["has_next"] = "true"
		}
		if mode == "duplicate episode" {
			bad["1"] = tencent_episode_fixture(test_tencent_first, "2", "0")
		}
		api := mock_client(t, func(req *http.Request) (string, int) { return tencent_listing_fixture(bad, params), 200 })
		client.HTTPClient = api.HTTPClient
		if _, _, err := client.tencent_episodes(context.Background(), test_tencent_cover, ""); err == nil {
			t.Fatalf("accepted %s", mode)
		}
	}
}

func tencent_playback_fixture(id, preview string, duration float64) string {
	return "QZOutputJson=" + platform_json(map[string]any{"s": "o", "preview": json.RawMessage(preview), "fl": map[string]any{"fi": []any{map[string]any{"name": "hd"}, map[string]any{"name": "shd"}}}, "vl": map[string]any{"vi": []any{map[string]any{"vid": id, "ti": "测试视频", "td": duration, "vw": 32, "vh": 32, "drm": 0, "ul": map[string]any{"ui": []any{map[string]any{"url": "https://sourceutil.tc.qq.com/vod/", "hls": map[string]string{"pt": "index.m3u8"}}}}}}}}) + ";"
}
func TestTencentMovieListingKeepsFeatureFilm(t *testing.T) {
	children := map[string]any{"main": tencent_episode_fixture(test_tencent_first, "正片", "0")}
	api := mock_client(t, func(req *http.Request) (string, int) {
		return tencent_listing_fixture(children, map[string]string{"is_all_data": "true"}), 200
	})
	client := new_test_backend(&backend{RequestClient: &sourceutil.RequestClient{HTTPClient: api.HTTPClient, Policy: source_policy}})
	episodes, _, err := client.tencent_episodes(context.Background(), test_tencent_cover, "")
	if err != nil || len(episodes) != 1 || episodes[0].ID != test_tencent_first || episodes[0].Order != 0 {
		t.Fatalf("feature film rejected or episode number invented: %+v %v", episodes, err)
	}
}
