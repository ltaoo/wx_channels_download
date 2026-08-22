package zhihu

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type round_trip_func func(*http.Request) (*http.Response, error)

func (fn round_trip_func) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestFetchVideoPlayInfo(t *testing.T) {
	var captured_request *http.Request
	var captured_body video_play_info_request
	client := NewClient(nil, nil)
	client.http_client = &http.Client{Transport: round_trip_func(func(req *http.Request) (*http.Response, error) {
		captured_request = req.Clone(req.Context())
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &captured_body); err != nil {
			t.Fatal(err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"video_play": {
					"id": "2072359233719482104",
					"default_cover": "https://pic.zhimg.com/cover.jpg",
					"meta": {"mime":"video/mp4","duration":12.133333,"resolution":{"quality":"HD","width":720,"height":1280}},
					"playlist": {"mp4":[{"key":20012,"quality":"HD","format":"mp4","codec":"H264","width":720,"height":1280,"size":1096396,"url":["https://vdn6.vzuu.com/video.mp4"]}]}
				}
			}`)),
		}, nil
	})}

	info, err := client.FetchVideoPlayInfo(
		"2072359423868248228",
		"answer",
		"2072359233719482104",
		"answer_detail_web",
		"https://www.zhihu.com/question/277577266/answer/2072359423868248228",
	)
	if err != nil {
		t.Fatal(err)
	}
	if info.VideoPlay.ID != "2072359233719482104" || len(info.VideoPlay.Playlist.MP4) != 1 {
		t.Fatalf("video info = %#v", info)
	}
	if captured_request == nil {
		t.Fatal("request was not captured")
	}
	if captured_request.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", captured_request.Method)
	}
	if captured_request.URL.Query().Get("r") != "20723592337194821042072359423868248228" {
		t.Errorf("r = %q", captured_request.URL.Query().Get("r"))
	}
	if captured_request.Header.Get("Content-Type") != "application/json" ||
		captured_request.Header.Get("Origin") != "https://www.zhihu.com" ||
		captured_request.Header.Get("Referer") == "" ||
		captured_request.Header.Get("User-Agent") == "" {
		t.Errorf("request headers = %#v", captured_request.Header)
	}
	if captured_body.ContentID != "2072359423868248228" ||
		captured_body.ContentTypeStr != "answer" ||
		captured_body.VideoID != "2072359233719482104" ||
		captured_body.SceneCode != "answer_detail_web" ||
		!captured_body.IsOnlyVideo {
		t.Errorf("request body = %#v", captured_body)
	}
}
