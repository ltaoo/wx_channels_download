package youku

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	sourceutil "wx_channel/pkg/scraper/internal/source"
)

const test_show_id = "bcaa87c7834c46f99e7b"

func catalogue_video_id(number int) string {
	return "X" + base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(number*4)))
}

func catalogue_episode(number, order int) catalogue_node {
	data, _ := json.Marshal(map[string]any{"title": fmt.Sprintf("示例剧 第%d集", order), "stage": order, "videoType": "正片", "episodeInfo": map[string]any{"isZp": true}, "img": "https://m.ykimg.com/episode.jpg", "action": map[string]any{"type": "JUMP_TO_VIDEO", "value": catalogue_video_id(number), "extra": map[string]string{"showId": test_show_id}}})
	return catalogue_node{ID: sourceutil.TextValue(strconv.Itoa(number)), Type: 10013, Data: data}
}

func catalogue_fixture(rows ...catalogue_node) catalogue_node {
	extra := catalogue_extra{ShowID: test_show_id, ShowLongID: "736526", ShowName: "示例剧", ShowCategory: "电视剧", EpisodeTotal: 2, EpisodeLast: 2, VideoID: catalogue_video_id(10001), ShowReleaseTime: "0000-00-00", VideoPublishTime: "2026-03-04 12:00:00", Duration: 120}
	root_data, _ := json.Marshal(map[string]any{"extra": extra, "account": "private-account", "session": "private-tracking"})
	intro_data := json.RawMessage(`{"introTitle":"示例剧","introSubTitle":"中国·2026·古装","desc":"完整剧情简介","showImgV":"http://m.ykimg.com/poster.jpg","private":"private-account"}`)
	intro := catalogue_node{Type: 20009, Nodes: []catalogue_node{{Type: 20010, Data: intro_data}, {ID: "director", Type: 10011, Data: json.RawMessage(`{"title":"导演甲","subtitle":"导演"}`)}, {ID: "actor", Type: 10011, Data: json.RawMessage(`{"title":"演员乙","subtitle":"饰角色乙","img":"https://m.ykimg.com/actor.jpg"}`)}}}
	component := catalogue_node{Type: 10013, Data: json.RawMessage(`{"subtitle":"更新至2集"}`), Nodes: rows}
	return catalogue_node{Type: 10001, Data: root_data, Nodes: []catalogue_node{intro, component, {Type: 10322, Nodes: []catalogue_node{catalogue_episode(19999, 1)}}}}
}

func catalogue_reply(root catalogue_node) string {
	data, _ := json.Marshal(map[string]any{"ret": []string{"SUCCESS::ok"}, "data": map[string]any{"2019030100": map[string]any{"success": true, "data": root}}})
	return string(data)
}

func catalogue_http(t *testing.T, respond func(*http.Request) string) *http.Client {
	t.Helper()
	return &http.Client{Transport: round_trip_func(func(req *http.Request) (*http.Response, error) {
		if req.URL.Hostname() != "acs.youku.com" || req.URL.Path != "/h5/mtop.youku.columbus.gateway.new.execute/1.0/" {
			t.Fatalf("series details invoked playback, a browser or another endpoint: %s", req.URL.Hostname())
		}
		body := respond(req)
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})}
}

func TestSeriesDetailsAndSeriesFetchDoNotResolveMedia(t *testing.T) {
	root := catalogue_fixture(catalogue_episode(10001, 1), catalogue_episode(10002, 2))
	preview := catalogue_episode(10003, 3)
	preview.Data = json.RawMessage(strings.Replace(string(preview.Data), `"isZp":true`, `"isZp":false`, 1))
	root.Nodes[1].Nodes = append(root.Nodes[1].Nodes, preview)
	client := NewClient("cna=fixture; _m_h5_tk=token_9999999999999; P_sck=private-account")
	client.SetHTTPClient(catalogue_http(t, func(req *http.Request) string {
		if !strings.Contains(req.Header.Get("Cookie"), "P_sck=private-account") {
			t.Fatal("catalogue lost the supplied account")
		}
		return catalogue_reply(root)
	}))
	page := "https://v.youku.com/v_nextstage/id_" + test_show_id + ".html"
	result, err := client.FetchContext(context.Background(), page)
	if err != nil {
		t.Fatal(err)
	}
	if result.Profile.ID != "736526" || result.Profile.Kind != "series" || result.Profile.Title != "示例剧" || result.Profile.Description != "完整剧情简介" || result.Profile.ReleaseDate != "" || result.Profile.ReleaseYear != 2026 || result.Profile.EpisodeCount != 2 || result.Profile.AvailableEpisodeCount != 2 || len(result.Episodes) != 2 || len(result.Profile.Credits) != 2 || result.Profile.Credits[1].Character != "角色乙" || result.Episodes[0].ReleaseDate != "2026-03-04" {
		t.Fatalf("lost series details or accepted promotional material: %+v", result)
	}
	if result.Profile.CoverURL != "https://m.ykimg.com/poster.jpg" || result.Profile.Homepage != page || result.Episodes[1].Order != 2 {
		t.Fatal("incorrect artwork, series homepage or episode ordering")
	}
	encoded, _ := json.Marshal(result)
	for _, private := range []string{"private-account", "private-tracking", "_m_h5_tk", "signature"} {
		if strings.Contains(string(encoded), private) {
			t.Fatal("private session fields leaked into catalogue metadata")
		}
	}
}

func TestSeriesPaginationUsesComponentSessionAndAllowsBoundaryOverlap(t *testing.T) {
	first := catalogue_fixture(catalogue_episode(10001, 1))
	first.Nodes[1].More = true
	first.Nodes[1].Data = json.RawMessage(`{"session":"{\"cursor\":\"next-page\",\"private\":\"tracking-only\"}"}`)
	second := catalogue_fixture(catalogue_episode(10001, 1), catalogue_episode(10002, 2))
	second.Data = json.RawMessage(`{}`)
	client := NewClient("cna=fixture")
	calls := 0
	client.SetHTTPClient(catalogue_http(t, func(req *http.Request) string {
		calls++
		if calls == 1 {
			return catalogue_reply(first)
		}
		var data map[string]string
		var params map[string]any
		_ = json.Unmarshal([]byte(req.URL.Query().Get("data")), &data)
		_ = json.Unmarshal([]byte(data["params"]), &params)
		if params["scene"] != "component" || !strings.Contains(fmt.Sprint(params["nextSession"]), "next-page") || params["showId"] != test_show_id {
			t.Fatal("pagination did not retain the official component cursor and identity")
		}
		return catalogue_reply(second)
	}))
	detail, err := client.FetchSeriesDetails("https://v.youku.com/v_nextstage/id_" + test_show_id + ".html")
	if err != nil || calls != 2 || len(detail.Episodes) != 2 || !detail.ListingComplete || strings.Contains(string(detail.Profile.Metadata), "tracking-only") {
		t.Fatalf("incomplete or unsanitized pagination: calls=%d error=%v", calls, err)
	}
}

func TestSeriesCatalogueRejectsIdentityGapsAndConflictingOrders(t *testing.T) {
	for name, mutate := range map[string]func(*catalogue_node){
		"wrong show": func(root *catalogue_node) {
			root.Data = json.RawMessage(strings.Replace(string(root.Data), test_show_id, "different_show", 1))
		},
		"wrong action show": func(root *catalogue_node) {
			root.Nodes[1].Nodes[0].Data = json.RawMessage(strings.Replace(string(root.Nodes[1].Nodes[0].Data), test_show_id, "different_show", 1))
		},
		"wrong numeric video": func(root *catalogue_node) { root.Nodes[1].Nodes[0].ID = "99999" },
		"duplicate order":     func(root *catalogue_node) { root.Nodes[1].Nodes[1] = catalogue_episode(10002, 1) },
		"duplicate ID":        func(root *catalogue_node) { root.Nodes[1].Nodes[1] = catalogue_episode(10001, 2) },
		"missing episode":     func(root *catalogue_node) { root.Nodes[1].Nodes = []catalogue_node{catalogue_episode(10001, 1)} },
		"gap":                 func(root *catalogue_node) { root.Nodes[1].Nodes[1] = catalogue_episode(10002, 3) },
		"missing cursor":      func(root *catalogue_node) { root.Nodes[1].More = true },
		"no progress": func(root *catalogue_node) {
			root.Nodes[1].More = true
			root.Nodes[1].Data = json.RawMessage(`{"session":{"cursor":"unchanged"}}`)
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := catalogue_fixture(catalogue_episode(10001, 1), catalogue_episode(10002, 2))
			mutate(&root)
			client := NewClient("")
			calls := 0
			client.SetHTTPClient(catalogue_http(t, func(*http.Request) string { calls++; return catalogue_reply(root) }))
			if _, err := client.FetchSeriesDetails("https://v.youku.com/v_nextstage/id_" + test_show_id + ".html"); err == nil || calls > 2 {
				t.Fatalf("invalid catalogue accepted or unbounded pagination: calls=%d error=%v", calls, err)
			}
		})
	}
}

func TestVideoEnrichmentRetainsSelectedEpisodeAndVerifiedMediaFacts(t *testing.T) {
	selected := catalogue_video_id(10001)
	root := catalogue_fixture(catalogue_episode(10001, 1), catalogue_episode(10002, 2))
	playback_calls := 0
	client := NewClient("session=fixture")
	client.SetHTTPClient(&http.Client{Transport: round_trip_func(func(req *http.Request) (*http.Response, error) {
		body, header := "", make(http.Header)
		switch req.URL.Path {
		case "/eg.js":
			header.Set("ETag", `"fixture"`)
		case "/ups/get.json":
			playback_calls++
			body = fmt.Sprintf(`{"data":{"video":{"encodeid":%q,"title":"当前集","seconds":2},"stream":[{"m3u8_url":"https://pl-ali.youku.com/index.m3u8","width":32,"height":32}]}}`, selected)
		case "/index.m3u8":
			body = "#EXTM3U\n#EXTINF:2,\nsegment.ts\n#EXT-X-ENDLIST\n"
		case "/h5/mtop.youku.columbus.gateway.new.execute/1.0/":
			body = catalogue_reply(root)
		}
		return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})})
	result, err := client.Fetch("https://v.youku.com/v_show/id_" + selected + ".html")
	if err != nil || playback_calls != 1 || len(result.Episodes) != 1 || result.Episodes[0].ID != selected || result.Episodes[0].Duration != 2 || result.Episodes[0].Width != 32 || result.Profile.EpisodeCount != 2 || result.Profile.Title != "示例剧" {
		t.Fatalf("enrichment changed selection or media facts: calls=%d error=%v", playback_calls, err)
	}
}
