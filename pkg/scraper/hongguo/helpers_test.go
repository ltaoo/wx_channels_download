// Test helpers adapted from dramadl; see ../NOTICE and ../LICENSE.
package hongguo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

const test_series = "7688352674999176254"
const test_vid = "7688396347107445822"
const test_media = "https://v3-reading.qznovelvod.com/test.mp4"

type round_trip_func func(*http.Request) (*http.Response, error)

func (handler round_trip_func) RoundTrip(req *http.Request) (*http.Response, error) {
	return handler(req)
}

func mock_client(t *testing.T, handler func(*http.Request) (string, int)) *api_client {
	t.Helper()
	return &api_client{HTTPClient: &http.Client{Transport: round_trip_func(func(req *http.Request) (*http.Response, error) {
		body, status := handler(req)
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Header: make(http.Header), Request: req}, nil
	})}}
}

func router_html(series_id string, vids string) string {
	return fmt.Sprintf(`<script>window._ROUTER_DATA = {"loaderData":{"ignored":[],"page":{"isSuccess":true,"series_id":%q,"vid":%q,"seriesDetail":{"series_id":%q,"series_name":"测试 {剧} &amp; 名","vid_list":%s}}}};</script>`, series_id, test_vid, series_id, vids)
}

func TestParseSeriesID(t *testing.T) {
	for _, input := range []string{test_series, " " + test_series + " ", site + "/detail?series_id=" + test_series, site + "/player/" + test_series, site + "/player/" + test_series + "/" + test_vid + "#video"} {
		got, err := parse_series_id(input)
		if err != nil || got != test_series {
			t.Errorf("input=%q got=%q err=%v", input, got, err)
		}
	}
	for _, input := range []string{"", "../bad", strings.Repeat("1", 31), "https://evil.example/detail?series_id=" + test_series, "http://hongguoduanju.com/player/" + test_series, site + "/other/" + test_series, "https://user@hongguoduanju.com/player/" + test_series} {
		if _, err := parse_series_id(input); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
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
