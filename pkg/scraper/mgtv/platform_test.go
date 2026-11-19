package mgtv

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

const test_mgtv_id = "24771585"
const test_mgtv_cover = "914227"
const test_mgtv_page = "https://www.mgtv.com/b/914227/24771585.html"

func platform_json(value any) string { raw, _ := json.Marshal(value); return string(raw) }
func mgtv_playback_fixture() map[string]any {
	return map[string]any{"code": 200, "data": map[string]any{
		"atc":  map[string]any{"pm2": "fixture-pm2"},
		"drm":  map[string]any{"drmType": ""},
		"info": map[string]any{"video_id": test_mgtv_id, "collection_id": test_mgtv_cover, "duration": "2", "title": "测试节目", "partName": "测试节目 第1期"},
	}}
}

func mgtv_source_fixture() map[string]any {
	return map[string]any{"code": 200, "data": map[string]any{
		"stream_domain": []string{"https://disp.titan.mgtv.com"},
		"stream": []any{
			map[string]any{"url": "/low", "filebitrate": "10", "vip": "0", "fileformat": "HLS-TS", "videoFormat": "H264"},
			map[string]any{"url": "/high", "filebitrate": "30", "vip": "1", "fileformat": "HLS-TS", "videoFormat": "H264"},
			map[string]any{"url": "/medium", "filebitrate": "20", "vip": "0", "fileformat": "HLS-TS", "videoFormat": "H264"},
		},
	}}
}

func mgtv_playlist_fixture() string {
	return "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXT-MGTV-VIDEO-WIDTH:32\n#EXT-MGTV-VIDEO-HEIGHT:32\n#EXTINF:2,\nsegment.ts\n#EXT-X-ENDLIST\n"
}

func TestMGTVRejectsIncompleteUntrustedAndUnauthorized(t *testing.T) {
	for _, mode := range []string{"playback denied", "wrong id", "wrong collection", "missing duration", "DRM", "missing credential", "source denied", "VIP only", "unsafe dispatch", "unsafe playlist", "dispatch denied", "preview", "missing dimensions"} {
		t.Run(mode, func(t *testing.T) {
			playback, source := mgtv_playback_fixture(), mgtv_source_fixture()
			data := playback["data"].(map[string]any)
			info := data["info"].(map[string]any)
			switch mode {
			case "playback denied":
				playback["code"] = 401
			case "wrong id":
				info["video_id"] = "99999"
			case "wrong collection":
				info["collection_id"] = "99999"
			case "missing duration":
				delete(info, "duration")
			case "DRM":
				data["drm"] = map[string]any{"drmType": "widevine"}
			case "missing credential":
				delete(data, "atc")
			case "source denied":
				source["code"] = 401
			case "VIP only":
				source["data"].(map[string]any)["stream"] = []any{map[string]any{"url": "/high", "vip": "1", "fileformat": "HLS-TS", "videoFormat": "H264"}}
			case "unsafe dispatch":
				source["data"].(map[string]any)["stream_domain"] = []string{"https://evil.test"}
			case "preview":
				info["duration"] = "4790"
			}
			api := mock_client(t, func(req *http.Request) (string, int) {
				if req.URL.Path == "/player/video" {
					return platform_json(playback), 200
				}
				if req.URL.Path == "/player/getSource" {
					return platform_json(source), 200
				}
				if req.URL.Hostname() == "disp.titan.mgtv.com" {
					if mode == "unsafe playlist" {
						return `{"status":"ok","info":"https://evil.test/private"}`, 200
					}
					if mode == "dispatch denied" {
						return `{"status":"denied","info":"private error details"}`, 200
					}
					return `{"status":"ok","info":"https://pcvideo.titan.mgtv.com/index.m3u8"}`, 200
				}
				if req.URL.Path == "/index.m3u8" {
					playlist := mgtv_playlist_fixture()
					if mode == "missing dimensions" {
						playlist = strings.ReplaceAll(playlist, "#EXT-MGTV-VIDEO-WIDTH:32", "#EXT-MGTV-VIDEO-WIDTH:0")
					}
					return playlist, 200
				}
				t.Fatal("untrusted address or media must not be downloaded")
				return "", 500
			})
			client := new_test_backend(&backend{RequestClient: &sourceutil.RequestClient{HTTPClient: api.HTTPClient, Policy: source_policy}})
			_, err := client.mgtv_video(context.Background(), test_mgtv_cover, test_mgtv_id)
			if err == nil || strings.Contains(err.Error(), "private") {
				t.Fatalf("accepted invalid source or exposed upstream details: %v", err)
			}
			if mode == "preview" && !strings.Contains(err.Error(), "Cookie") {
				t.Fatal("preview rejection must explain account requirement")
			}
		})
	}
}

func TestMGTVFallsBackFromUnauthorizedVIPQuality(t *testing.T) {
	playback := mgtv_playback_fixture()
	playback["data"].(map[string]any)["info"].(map[string]any)["duration"] = "600"
	dispatched := []string{}
	api := mock_client(t, func(req *http.Request) (string, int) {
		switch req.URL.Path {
		case "/player/video":
			return platform_json(playback), 200
		case "/player/getSource":
			return platform_json(mgtv_source_fixture()), 200
		case "/high", "/medium":
			dispatched = append(dispatched, req.URL.Path)
			return platform_json(map[string]string{"status": "ok", "info": "https://pcvideo.titan.mgtv.com" + req.URL.Path + ".m3u8"}), 200
		case "/high.m3u8":
			return strings.Replace(mgtv_playlist_fixture(), "#EXTINF:2,", "#EXTINF:300,", 1), 200
		case "/medium.m3u8":
			return strings.Replace(mgtv_playlist_fixture(), "#EXTINF:2,", "#EXTINF:600,", 1), 200
		}
		t.Fatal("unexpected lower quality or media request")
		return "", 500
	})
	client := new_test_backend(&backend{RequestClient: &sourceutil.RequestClient{HTTPClient: api.HTTPClient, Policy: source_policy, Cookie: "fixture_session=non-vip"}})
	item, err := client.mgtv_video(context.Background(), test_mgtv_cover, test_mgtv_id)
	if err != nil || item.Info.Duration != 600 || strings.Join(dispatched, ",") != "/high,/medium" {
		t.Fatalf("did not fall back to authorized full quality: %v %v", dispatched, err)
	}
}
