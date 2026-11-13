// Protocol tests adapted from dramadl; see ../NOTICE and ../LICENSE.
package iqiyi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

func iqiyi_test_url(tvid string, album bool) string {
	number, _ := strconv.ParseUint(tvid, 10, 64)
	prefix := "v"
	if album {
		prefix = "a"
	}
	return "https://www.iqiyi.com/" + prefix + "_" + strconv.FormatUint(number^0x75706971676c, 36) + ".html"
}

func TestIQiyiURLAndSignatures(t *testing.T) {
	for input, want := range map[string]string{
		"https://www.iqiyi.com/v_eienwprun0.html":         "1487716838906800",
		"https://m.iqiyi.com/v_eienwprun0.html?from=test": "1487716838906800",
		"https://www.iqiyi.com/a_13jun9wzsv9.html":        "3993682943621001",
	} {
		_, got, _, err := iqiyi_page(input)
		if err != nil || got != want {
			t.Errorf("input=%q got=%q err=%v", input, got, err)
		}
	}
	for _, input := range []string{"https://evil.example/v_eienwprun0.html", "https://iqiyi.com.evil.example/v_eienwprun0.html", "https://user@www.iqiyi.com/v_eienwprun0.html", "file://www.iqiyi.com/v_eienwprun0.html", "https://www.iqiyi.com:123/v_eienwprun0.html", "https://www.iqiyi.com/", "https://www.iqiyi.com/v_" + strings.Repeat("z", 100) + ".html"} {
		if _, _, _, err := iqiyi_page(input); err == nil {
			t.Errorf("accepted invalid link %q", input)
		}
	}
	raw, err := os.ReadFile("testdata/iqiyi-signing.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		DashURL        string         `json:"dash_url"`
		MetadataParams map[string]any `json:"metadata_params"`
		MetadataSign   string         `json:"metadata_sign"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	got := iqiyi_dash_url("1234567890123456", "0123456789abcdef0123456789abcdef", "", "", 1793856000123)
	if got != fixture.DashURL {
		t.Fatalf("signed query differs from reference\ngot %s\nwant %s", got, fixture.DashURL)
	}
	values := make(url.Values)
	for key, value := range fixture.MetadataParams {
		if number, ok := value.(float64); ok {
			values.Set(key, strconv.FormatInt(int64(number), 10))
		} else {
			values.Set(key, fmt.Sprint(value))
		}
	}
	if iqiyi_sign(values) != fixture.MetadataSign {
		t.Fatal("metadata signature differs from reference")
	}
}

func iqiyi_test_metadata(rows any) string {
	value := map[string]any{"status_code": 0, "data": map[string]any{
		"base_data": map[string]any{"qipu_id": "3993682943621001", "title": "测试剧", "page_url": iqiyi_test_url("1487716838906800", false)},
		"template": map[string]any{"tabs": []any{map[string]any{"blocks": []any{
			map[string]any{"bk_id": "unrelated", "data": []any{"ignored"}},
			map[string]any{"bk_id": "selector_bk", "data": map[string]any{"data": rows}},
		}}}},
	}}
	raw, _ := json.Marshal(value)
	return string(raw)
}

func iqiyi_init_body(tvid string) string {
	return `window.QiyiPlayerProphetData = {"tvid":"` + tvid + `","videoInfo":{"tvId":"` + tvid + `","albumId":"3993682943621001","effective":true}};`
}

func iqiyi_test_row(tvid string, order int) map[string]any {
	return map[string]any{"qipu_id": tvid, "album_order": order, "title": fmt.Sprint("第", order, "集"), "page_url": iqiyi_test_url(tvid, false), "pay_mark": 0}
}

func TestIQiyiFullSeriesEnumeration(t *testing.T) {
	first, second := "1487716838906800", "4942645144948600"
	cases := []any{
		[]any{
			map[string]any{"entity_id": "999001", "language": "other", "videos": "https://mesh.if.iqiyi.com/tvg/v2/selector?album_id=999001"},
			map[string]any{"entity_id": "999002", "videos": map[string]any{"feature_paged": map[string]any{"1-1": []any{iqiyi_test_row("999002", 1)}}}},
			map[string]any{"entity_id": "3993682943621001", "videos": []any{map[string]any{"title": "1-2", "data": []any{iqiyi_test_row(first, 1), iqiyi_test_row(second, 2)}}}},
		},
		map[string]any{"videos": []any{iqiyi_test_row(first, 1), iqiyi_test_row(second, 2)}},
		map[string]any{"videos": []any{iqiyi_test_row(first, 1), map[string]any{"qipu_id": "999001", "album_order": 2, "content_type": 3}, iqiyi_test_row(second, 2), map[string]any{"qipu_id": "999002", "album_order": 1000000, "content_type": 28}}},
		[]any{map[string]any{"videos": map[string]any{"feature_paged": map[string]any{"2-2": []any{iqiyi_test_row(second, 2)}, "1-1": []any{iqiyi_test_row(first, 1)}}}}},
		[]any{map[string]any{"videos": []any{map[string]any{"title": "1-1", "data": []any{iqiyi_test_row(first, 1)}}, map[string]any{"title": "2-2", "data": []any{iqiyi_test_row(second, 2)}}}}},
	}
	for _, rows := range cases {
		requests := 0
		api := mock_client(t, func(req *http.Request) (string, int) {
			requests++
			if strings.HasSuffix(req.URL.Path, "accelerator.js") {
				if req.URL.Query().Get("tvId") != first {
					t.Fatal("init must bind TVID")
				}
				return iqiyi_init_body(first), 200
			}
			if req.URL.Query().Get("entity_id") != first || req.URL.Query().Get("sign") == "" {
				t.Fatal("missing metadata entity or signature")
			}
			return iqiyi_test_metadata(rows), 200
		})
		client := new_test_backend(&backend{RequestClient: &sourceutil.RequestClient{HTTPClient: api.HTTPClient, Referer: iqiyi_test_url(first, false)}})
		series, err := client.series(context.Background(), client.Referer, first, false)
		if err != nil || len(series.episodes) != 2 || requests != 2 || series.episodes[1].TVID != sourceutil.TextValue(second) {
			t.Fatalf("series=%+v err=%v", series, err)
		}
	}
}

func TestIQiyiRejectsWrongOrIncompleteSeries(t *testing.T) {
	first, second := "1487716838906800", "4942645144948600"
	for _, mode := range []string{"wrong init", "wrong album", "missing seed", "gap", "wrong URL"} {
		t.Run(mode, func(t *testing.T) {
			rows := []any{iqiyi_test_row(first, 1), iqiyi_test_row(second, 2)}
			if mode == "missing seed" {
				rows = []any{iqiyi_test_row(second, 1)}
			}
			if mode == "gap" {
				rows[1] = iqiyi_test_row(second, 3)
			}
			if mode == "wrong URL" {
				bad := iqiyi_test_row(first, 1)
				bad["page_url"] = iqiyi_test_url(second, false)
				rows[0] = bad
			}
			api := mock_client(t, func(req *http.Request) (string, int) {
				if strings.HasSuffix(req.URL.Path, "accelerator.js") {
					if mode == "wrong init" {
						return iqiyi_init_body(second), 200
					}
					return iqiyi_init_body(first), 200
				}
				body := iqiyi_test_metadata(map[string]any{"videos": rows})
				if mode == "wrong album" {
					body = strings.ReplaceAll(body, "3993682943621001", "999999")
				}
				return body, 200
			})
			client := new_test_backend(&backend{RequestClient: &sourceutil.RequestClient{HTTPClient: api.HTTPClient, Referer: iqiyi_test_url(first, false)}})
			if _, err := client.series(context.Background(), client.Referer, first, false); err == nil {
				t.Fatal("accepted mismatched or incomplete series")
			}
		})
	}
}

func TestIQiyiAlbumExpandsThroughCanonicalEpisode(t *testing.T) {
	first, second, album_id := "1487716838906800", "4942645144948600", "3993682943621001"
	calls := 0
	api := mock_client(t, func(req *http.Request) (string, int) {
		calls++
		if strings.HasSuffix(req.URL.Path, "accelerator.js") {
			return iqiyi_init_body(first), 200
		}
		if req.URL.Query().Get("entity_id") == album_id {
			return iqiyi_test_metadata(nil), 200
		}
		if req.URL.Query().Get("entity_id") != first {
			t.Error("album must resolve the canonical episode")
		}
		return iqiyi_test_metadata(map[string]any{"videos": []any{iqiyi_test_row(first, 1), iqiyi_test_row(second, 2)}}), 200
	})
	client := new_test_backend(&backend{RequestClient: &sourceutil.RequestClient{HTTPClient: api.HTTPClient, Referer: iqiyi_test_url(album_id, true)}})
	series, err := client.series(context.Background(), client.Referer, album_id, true)
	if err != nil || len(series.episodes) != 2 || calls != 3 {
		t.Fatalf("album=%+v calls=%d err=%v", series, calls, err)
	}
}

func TestIQiyiAlbumResolvesLazyEpisodeSelector(t *testing.T) {
	first, second, album_id := "1487716838906800", "4942645144948600", "3993682943621001"
	api := mock_client(t, func(req *http.Request) (string, int) {
		if req.URL.Path == "/tvg/entity_play_url" {
			if req.URL.Query().Get("entityId") != album_id || req.URL.Query().Get("sign") == "" {
				t.Error("missing album resolver signature")
			}
			return `{"data":{"resolve_type":"album_episode","episode_id":"` + first + `"}}`, 200
		}
		if strings.HasSuffix(req.URL.Path, "accelerator.js") {
			return iqiyi_init_body(first), 200
		}
		if req.URL.Query().Get("entity_id") == album_id {
			return strings.ReplaceAll(iqiyi_test_metadata(nil), iqiyi_test_url(first, false), ""), 200
		}
		return iqiyi_test_metadata(map[string]any{"videos": []any{iqiyi_test_row(first, 1), iqiyi_test_row(second, 2)}}), 200
	})
	client := new_test_backend(&backend{RequestClient: &sourceutil.RequestClient{HTTPClient: api.HTTPClient, Referer: iqiyi_test_url(album_id, true)}})
	series, err := client.series(context.Background(), client.Referer, album_id, true)
	if err != nil || len(series.episodes) != 2 {
		t.Fatalf("lazy album=%+v err=%v", series, err)
	}
}

func iqiyi_dash_body(tvid, playlist string, duration float64) string {
	value := map[string]any{"code": "A00000", "data": map[string]any{"tvid": tvid, "program": map[string]any{"video": []any{
		map[string]any{"bid": 800, "ff": "dash", "m3u8": "not a HLS stream"},
		map[string]any{"bid": 500, "ff": "ts", "m3u8": playlist, "duration": duration, "scrsz": "32x32"},
	}}}}
	raw, _ := json.Marshal(value)
	return string(raw)
}

func TestIQiyiPlaylistValidationAndCookie(t *testing.T) {
	tvid := "1487716838906800"
	playlist := "#EXTM3U\n#EXTINF:2,\nhttps://pcw-data.video.iqiyi.com/test.ts\n#EXT-X-ENDLIST\n"
	for _, mode := range []string{"valid", "wrong episode", "preview", "no permission", "paid without cookie"} {
		t.Run(mode, func(t *testing.T) {
			requests := 0
			api := mock_client(t, func(req *http.Request) (string, int) {
				requests++
				if req.Header.Get("Cookie") != "P00001=synthetic; P00003=123" || req.URL.Query().Get("pck") != "synthetic" || req.URL.Query().Get("uid") != "123" {
					t.Fatal("missing authorized cookie parameters")
				}
				id, duration := tvid, 2.0
				if mode == "wrong episode" {
					id = "999999"
				}
				if mode == "preview" {
					duration = 77
				}
				if mode == "no permission" {
					return `{"code":"A00000","data":{"tvid":"` + tvid + `","boss_ts":{"code":"Q00505"}}}`, 200
				}
				return iqiyi_dash_body(id, playlist, duration), 200
			})
			client := new_test_backend(&backend{RequestClient: &sourceutil.RequestClient{HTTPClient: api.HTTPClient, Cookie: "P00001=synthetic; P00003=123", Referer: iqiyi_test_url(tvid, false)}})
			episode := iqiyi_episode{TVID: sourceutil.TextValue(tvid)}
			if mode == "paid without cookie" {
				client.Cookie = ""
				episode.PayMark = 1
			}
			_, info, err := client.playlist(context.Background(), episode)
			if (err == nil) != (mode == "valid") || (mode == "valid" && info.Duration != 2) {
				t.Fatalf("playlist validation mismatch: info=%+v err=%v", info, err)
			}
			if mode == "paid without cookie" && requests != 0 {
				t.Fatal("paid episode should fail before a request")
			}
		})
	}
}
