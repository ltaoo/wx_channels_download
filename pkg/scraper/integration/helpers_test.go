package drama_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type round_trip_func func(*http.Request) (*http.Response, error)

func (handler round_trip_func) RoundTrip(req *http.Request) (*http.Response, error) {
	return handler(req)
}

type test_api struct{ HTTPClient *http.Client }

func mock_client(t *testing.T, handler func(*http.Request) (string, int)) *test_api {
	t.Helper()
	return &test_api{HTTPClient: &http.Client{Transport: round_trip_func(func(req *http.Request) (*http.Response, error) {
		body, status := handler(req)
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Header: make(http.Header), Request: req}, nil
	})}}
}

const site = "https://hongguoduanju.com"
const test_series = "7688352674999176254"
const test_vid = "7688396347107445822"
const test_youku_id = "XNjU0MjIxOTc4NA=="
const test_tencent_first = "n3371za9efh"
const test_mgtv_page = "https://www.mgtv.com/b/914227/24771585.html"

func router_html(series_id string, vids string) string {
	return fmt.Sprintf(`<script>window._ROUTER_DATA = {"loaderData":{"ignored":[],"page":{"isSuccess":true,"series_id":%q,"vid":%q,"seriesDetail":{"series_id":%q,"series_name":"测试 {剧} &amp; 名","vid_list":%s}}}};</script>`, series_id, test_vid, series_id, vids)
}

func model_response(vid string) string {
	model := map[string]any{"video_id": "synthetic-video-" + vid, "video_duration": 2, "video_list": []any{map[string]any{"main_url": "https://v3-reading.qznovelvod.com/" + vid + ".mp4", "codec_type": "h264", "width": 32, "height": 32, "encrypt": false}}}
	response, _ := json.Marshal(map[string]any{"code": 0, "data": map[string]any{vid: map[string]any{"video_model": model}}})
	return string(response)
}

func requested_vid(t *testing.T, req *http.Request) string {
	t.Helper()
	var payload struct {
		MixedVideoIDMap map[string][]string `json:"mixed_video_id_map"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	vids := payload.MixedVideoIDMap["1004"]
	if len(vids) != 1 {
		t.Fatalf("unexpected body: %+v", payload)
	}
	return vids[0]
}
func platform_json(value any) string { raw, _ := json.Marshal(value); return string(raw) }

func tencent_playback_fixture(id, preview string, duration float64) string {
	return "QZOutputJson=" + platform_json(map[string]any{"s": "o", "preview": json.RawMessage(preview), "fl": map[string]any{"fi": []any{map[string]any{"name": "hd"}, map[string]any{"name": "shd"}}}, "vl": map[string]any{"vi": []any{map[string]any{"vid": id, "ti": "测试视频", "td": duration, "vw": 32, "vh": 32, "drm": 0, "ul": map[string]any{"ui": []any{map[string]any{"url": "https://media.tc.qq.com/vod/", "hls": map[string]string{"pt": "index.m3u8"}}}}}}}}) + ";"
}

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

const test_mgtv_id = "24771585"
const test_mgtv_cover = "914227"
