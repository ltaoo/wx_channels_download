package bilibili

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	contentdownload "wx_channel/pkg/contentplatform/download"
)

func TestExtractVideoKey(t *testing.T) {
	tests := []struct {
		raw  string
		want VideoKey
		ok   bool
	}{
		{"BV1z3jG63ELn", VideoKey{BVID: "BV1z3jG63ELn", Page: 1}, true},
		{"https://www.bilibili.com/video/BV1z3jG63ELn/?spm_id_from=333.1007.tianma.4-3-13.click&p=2", VideoKey{BVID: "BV1z3jG63ELn", Page: 2}, true},
		{"https://www.bilibili.com/video/av12345", VideoKey{AID: 12345, Page: 1}, true},
		{"https://www.bilibili.com/opus/171447707115708382?spm_id_from=333.1387.0.0", VideoKey{}, false},
		{"https://example.com/video/BV1z3jG63ELn", VideoKey{}, false},
	}
	for _, tt := range tests {
		got, ok := ExtractVideoKey(tt.raw)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("ExtractVideoKey(%q) = %#v, %v; want %#v, %v", tt.raw, got, ok, tt.want, tt.ok)
		}
	}
}

func TestExtractOpusID(t *testing.T) {
	tests := []struct {
		raw  string
		want string
		ok   bool
	}{
		{"https://www.bilibili.com/opus/171447707115708382?spm_id_from=333.1387.0.0", "171447707115708382", true},
		{"https://m.bilibili.com/opus/171447707115708382", "171447707115708382", true},
		{"https://www.bilibili.com/video/BV1z3jG63ELn", "", false},
		{"https://example.com/opus/171447707115708382", "", false},
	}
	for _, tt := range tests {
		got, ok := ExtractOpusID(tt.raw)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("ExtractOpusID(%q) = %q, %v; want %q, %v", tt.raw, got, ok, tt.want, tt.ok)
		}
	}
}

func TestProbeAndResolveDASHWithCookie(t *testing.T) {
	var viewCookie string
	var playCookie string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/x/web-interface/nav":
			http.NotFound(w, r)
		case "/x/web-interface/view":
			viewCookie = r.Header.Get("Cookie")
			if got := r.URL.Query().Get("bvid"); got != "BV1z3jG63ELn" {
				t.Fatalf("view bvid = %q", got)
			}
			_, _ = w.Write([]byte(`{
				"code": 0,
				"data": {
					"bvid": "BV1z3jG63ELn",
					"aid": 98765,
					"cid": 112233,
					"title": "测试视频",
					"desc": "简介",
					"pic": "https://i0.hdslb.com/bfs/archive/cover.jpg",
					"duration": 123,
					"owner": {"mid": 42, "name": "UP主", "face": "https://i0.hdslb.com/bfs/face/up.jpg"},
					"pages": [{"cid": 112233, "page": 1, "part": "P1", "duration": 123, "width": 1920, "height": 1080}]
				}
			}`))
		case "/x/player/playurl":
			playCookie = r.Header.Get("Cookie")
			if got := r.URL.Query().Get("cid"); got != "112233" {
				t.Fatalf("playurl cid = %q", got)
			}
			_, _ = w.Write([]byte(`{
				"code": 0,
				"data": {
					"quality": 80,
					"timelength": 123000,
					"accept_quality": [80, 64, 32],
					"accept_description": ["高清 1080P", "高清 720P", "清晰 480P"],
					"dash": {
						"duration": 123,
						"video": [
							{"id": 80, "baseUrl": "https://media.example/video-80-hevc.m4s", "bandwidth": 1200000, "mimeType": "video/mp4", "codecs": "hev1.1.6.L120.90", "width": 1920, "height": 1080},
							{"id": 80, "baseUrl": "https://media.example/video-80-avc.m4s", "bandwidth": 900000, "mimeType": "video/mp4", "codecs": "avc1.640028", "width": 1920, "height": 1080},
							{"id": 64, "baseUrl": "https://media.example/video-64.m4s", "bandwidth": 600000, "mimeType": "video/mp4", "codecs": "avc1.64001f", "width": 1280, "height": 720}
						],
						"audio": [
							{"id": 30280, "baseUrl": "https://media.example/audio.m4s", "bandwidth": 128000, "mimeType": "audio/mp4", "codecs": "mp4a.40.2"}
						]
					}
				}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	h := New(&Client{
		HTTPClient: server.Client(),
		APIBaseURL: server.URL,
		WebBaseURL: "https://www.bilibili.com",
		Cookie:     "SESSDATA=test-cookie",
	})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.bilibili.com/video/BV1z3jG63ELn/"})
	if err != nil {
		t.Fatal(err)
	}
	if viewCookie != "SESSDATA=test-cookie" || playCookie != "SESSDATA=test-cookie" {
		t.Fatalf("cookies = view %q play %q", viewCookie, playCookie)
	}
	if probe.ContentID != "BV1z3jG63ELn" {
		t.Fatalf("content id = %q", probe.ContentID)
	}
	if probe.Defaults.VariantID != "video_qn_80" {
		t.Fatalf("default variant = %q", probe.Defaults.VariantID)
	}
	if len(probe.Variants) < 4 {
		t.Fatalf("variants = %#v", probe.Variants)
	}

	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{
		URL:   "https://www.bilibili.com/video/BV1z3jG63ELn/",
		Probe: probe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Download.Protocol != contentdownload.ProtocolMultiHTTP {
		t.Fatalf("protocol = %q", resolved.Download.Protocol)
	}
	if resolved.Suffix != ".mp4" {
		t.Fatalf("suffix = %q", resolved.Suffix)
	}
	if requires, _ := resolved.Metadata["requires_ffmpeg"].(bool); !requires {
		t.Fatalf("requires_ffmpeg = %v", resolved.Metadata["requires_ffmpeg"])
	}
	var sources []contentdownload.MultiSourceSpec
	if err := json.Unmarshal(resolved.Download.Body, &sources); err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 {
		t.Fatalf("sources = %#v", sources)
	}
	if sources[0].URL != "https://media.example/video-80-avc.m4s" || !sources[0].HasVideo {
		t.Fatalf("video source = %#v", sources[0])
	}
	if sources[1].URL != "https://media.example/audio.m4s" || !sources[1].HasAudio {
		t.Fatalf("audio source = %#v", sources[1])
	}
	if sources[0].Headers["Cookie"] != "SESSDATA=test-cookie" ||
		sources[0].Headers["Referer"] != "https://www.bilibili.com/video/BV1z3jG63ELn" {
		t.Fatalf("source headers = %#v", sources[0].Headers)
	}
}

func TestProbeAndResolveOpus(t *testing.T) {
	var pageCookie string
	var pageAccept string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/opus/171447707115708382" {
			http.NotFound(w, r)
			return
		}
		pageCookie = r.Header.Get("Cookie")
		pageAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(testOpusHTML))
	}))
	defer server.Close()

	h := New(&Client{
		HTTPClient: server.Client(),
		WebBaseURL: server.URL,
		Cookie:     "SESSDATA=opus-cookie",
	})
	probe, err := h.Probe(context.Background(), contentdownload.ProbeInput{URL: "https://www.bilibili.com/opus/171447707115708382?spm_id_from=333.1387.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if pageCookie != "SESSDATA=opus-cookie" {
		t.Fatalf("page cookie = %q", pageCookie)
	}
	if !strings.Contains(pageAccept, "text/html") {
		t.Fatalf("page accept = %q", pageAccept)
	}
	if probe.ContentID != "171447707115708382" {
		t.Fatalf("content id = %q", probe.ContentID)
	}
	summary := contentdownload.ContentSummaryOf(probe.Content)
	if summary.Type != "article" || summary.Title != "新英雄爆料 | 朝逢夕遇，日月共生。游光&司夜人设情报公开！" {
		t.Fatalf("summary = %#v", summary)
	}
	if summary.Author != "非人学园" || summary.AuthorAvatarURL == "" || summary.CoverURL == "" {
		t.Fatalf("author/cover summary = %#v", summary)
	}
	if probe.Defaults.VariantID != "html" || probe.Defaults.Suffix != ".html" {
		t.Fatalf("defaults = %#v", probe.Defaults)
	}
	if !hasVariant(probe.Variants, "initial_state_json") {
		t.Fatalf("missing initial_state_json variant: %#v", probe.Variants)
	}
	if pageHTML, _ := probe.Internal["pagehtml"].(string); !strings.Contains(pageHTML, "window.__INITIAL_STATE__") {
		t.Fatalf("pagehtml = %.80q", pageHTML)
	}
	raw, ok := probe.Internal["pagejson"].(json.RawMessage)
	if !ok || !json.Valid(raw) {
		t.Fatalf("pagejson = %#v", probe.Internal["pagejson"])
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["id"] != "171447707115708382" {
		t.Fatalf("decoded id = %#v", decoded["id"])
	}

	resolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{
		URL:   "https://www.bilibili.com/opus/171447707115708382",
		Probe: probe,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Download.Protocol != "inline_html" || resolved.Suffix != ".html" {
		t.Fatalf("resolved html = protocol %q suffix %q", resolved.Download.Protocol, resolved.Suffix)
	}
	if bodyHTML, _ := resolved.Metadata["body_html"].(string); !strings.Contains(bodyHTML, "window.__INITIAL_STATE__") {
		t.Fatalf("resolved body_html = %.80q", bodyHTML)
	}

	jsonResolved, err := h.Resolve(context.Background(), contentdownload.ResolveInput{
		URL:     "https://www.bilibili.com/opus/171447707115708382",
		Probe:   probe,
		Options: contentdownload.Options{VariantID: "initial_state_json"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if jsonResolved.Download.Protocol != "inline_json" || jsonResolved.Suffix != ".json" {
		t.Fatalf("resolved json = protocol %q suffix %q", jsonResolved.Download.Protocol, jsonResolved.Suffix)
	}
	if raw, ok := jsonResolved.Metadata["json"].(json.RawMessage); !ok || !json.Valid(raw) {
		t.Fatalf("resolved json metadata = %#v", jsonResolved.Metadata["json"])
	}
}

func TestExtractOpusInitialStateFromReferenceFixture(t *testing.T) {
	rawHTML, err := os.ReadFile("../../../bilibili_article_260617.html")
	if os.IsNotExist(err) {
		t.Skip("reference bilibili article fixture is not present")
	}
	if err != nil {
		t.Fatal(err)
	}
	pageJSON, err := extractOpusInitialStateJSON(string(rawHTML))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(pageJSON) {
		t.Fatalf("invalid pagejson")
	}
	info, err := opusInfoFromInitialState(defaultWebBaseURL, "171447707115708382", string(rawHTML), pageJSON)
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "新英雄爆料 | 朝逢夕遇，日月共生。游光&司夜人设情报公开！" {
		t.Fatalf("title = %q", info.Title)
	}
	if info.AuthorName != "非人学园" || info.AuthorID != "223289804" {
		t.Fatalf("author = %#v", info)
	}
	if !strings.Contains(info.CoverURL, "/bfs/article/") {
		t.Fatalf("cover url = %q", info.CoverURL)
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

const testOpusHTML = `<!doctype html>
<html>
<head><title>新英雄爆料 | 朝逢夕遇，日月共生。游光&amp;司夜人设情报公开！ - 哔哩哔哩</title></head>
<body>
<script>
window.__INITIAL_STATE__ = {
  "id": "171447707115708382",
  "detail": {
    "id_str": "171447707115708382",
    "type": 1,
    "basic": {
      "title": "新英雄爆料 | 朝逢夕遇，日月共生。游光&司夜人设情报公开！ - 哔哩哔哩",
      "uid": 223289804,
      "rid_str": "1279205",
      "article_type": 0
    },
    "modules": [
      {
        "module_type": "MODULE_TYPE_TOP",
        "module_top": {
          "display": {
            "type": 1,
            "album": {
              "pics": [
                {"url": "https://i0.hdslb.com/bfs/article/cover.jpg", "width": 800, "height": 235}
              ]
            }
          }
        }
      },
      {
        "module_type": "MODULE_TYPE_TITLE",
        "module_title": {
          "text": "新英雄爆料 | 朝逢夕遇，日月共生。游光&司夜人设情报公开！"
        }
      },
      {
        "module_type": "MODULE_TYPE_AUTHOR",
        "module_author": {
          "face": "https://i2.hdslb.com/bfs/face/up.jpg",
          "name": "非人学园",
          "mid": 223289804,
          "jump_url": "//space.bilibili.com/223289804",
          "pub_time": "2018年10月06日 00:24",
          "pub_ts": 1538756680
        }
      },
      {
        "module_type": "MODULE_TYPE_BOTTOM",
        "module_bottom": {
          "share_info": {
            "title": "新英雄爆料 | 朝逢夕遇，日月共生。游光&司夜人设情报公开！",
            "summary": "自从制作人在QwQ杯现场给出了本赛季的最全爆料，包含 {花括号} 和 [图片]。",
            "pic": "https://static.hdslb.com/mobile/img/app_logo.png"
          }
        }
      }
    ]
  }
};
</script>
</body>
</html>`
