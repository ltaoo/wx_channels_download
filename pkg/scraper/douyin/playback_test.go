package douyin

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func playback_test_box(kind string, payload []byte) []byte {
	box := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(box[:4], uint32(len(box)))
	copy(box[4:8], kind)
	copy(box[8:], payload)
	return box
}

func playback_test_mp4(version byte, width, height int) []byte {
	payload := make([]byte, 84)
	offset := 76
	if version == 1 {
		payload = make([]byte, 96)
		offset = 88
	}
	payload[0] = version
	binary.BigEndian.PutUint32(payload[offset:], uint32(width)<<16)
	binary.BigEndian.PutUint32(payload[offset+4:], uint32(height)<<16)
	return playback_test_box("moov", playback_test_box("trak", playback_test_box("tkhd", payload)))
}

func TestPlaybackMP4Dimensions(t *testing.T) {
	for _, version := range []byte{0, 1} {
		data := append(playback_test_mp4(version, 0, 0), playback_test_mp4(version, 720, 1280)...)
		width, height := playback_mp4_dimensions(data)
		if width != 720 || height != 1280 {
			t.Fatalf("version %d: dimensions=%dx%d", version, width, height)
		}
		for length := 0; length < len(data); length++ {
			playback_mp4_dimensions(data[:length])
		}
	}
	for _, invalid := range [][]byte{
		[]byte("not an MP4 video"),
		{0, 0, 0, 1, 'm', 'o', 'o', 'v'},
		{0, 0, 0, 4, 't', 'k', 'h', 'd'},
		{0, 0, 0, 1, 'm', 'o', 'o', 'v', 255, 255, 255, 255, 255, 255, 255, 255},
	} {
		if width, height := playback_mp4_dimensions(invalid); width != 0 || height != 0 {
			t.Fatal("invalid boxes must not yield dimensions")
		}
	}
}

func TestFetchEnrichesMobilePlaybackVariants(t *testing.T) {
	for _, test_case := range []struct {
		name          string
		media         string
		probe_failure bool
		duplicate     bool
		want_count    int
		want_requests int
	}{
		{"missing bitrates", `"video":{"width":2160,"height":3840,"play_addr":{"url_list":["https://aweme.snssdk.com/aweme/v1/playwm/?ratio=720p&video_id=test"]},"bit_rate":null}`, false, false, 3, 3},
		{"duplicate fallback", `"video":{"play_addr":{"url_list":["https://aweme.snssdk.com/aweme/v1/playwm/?ratio=720p&video_id=test"]}}`, false, true, 2, 3},
		{"upstream failure", `"video":{"play_addr":{"url_list":["https://aweme.snssdk.com/aweme/v1/playwm/?ratio=720p&video_id=test"]}}`, true, false, 0, 3},
		{"existing variants", `"video":{"bit_rate":[{"gear_name":"original","play_addr":{"url_list":["https://media.example/original.mp4"]}}]}`, false, false, 1, 0},
		{"album", `"images":[{}],"video":{"play_addr":{"url_list":["https://aweme.snssdk.com/aweme/v1/playwm/?video_id=test"]}}`, false, false, 0, 0},
		{"untrusted endpoint", `"video":{"play_addr":{"url_list":["https://other.example/aweme/v1/playwm/?video_id=test"]}}`, false, false, 0, 0},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			client := NewClient("")
			item := `{"aweme_id":"123","desc":"retained title","numeric_id":7707979101957383653,` + test_case.media + `}`
			router := `{"loaderData":{"video_(id)/page":{"videoInfoRes":{"status_code":0,"item_list":[` + item + `]}}}}`
			client.pc.http_client = &http.Client{Transport: home_test_transport(func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`<script>window._ROUTER_DATA = ` + router + `;</script>`)), Request: req}, nil
			})}
			var mu sync.Mutex
			requests := 0
			client.web.http_client = &http.Client{Transport: home_test_transport(func(req *http.Request) (*http.Response, error) {
				mu.Lock()
				requests++
				mu.Unlock()
				if req.URL.Host != "aweme.snssdk.com" || req.URL.Path != "/aweme/v1/play/" || req.Header.Get("Range") != "bytes=0-65535" {
					t.Error("probe must use a bounded range on the watermark-free play endpoint")
				}
				if test_case.probe_failure {
					return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader("forbidden"))}, nil
				}
				width, height := 720, 1280
				switch req.URL.Query().Get("ratio") {
				case "540p":
					width, height = 576, 1024
				case "1080p":
					if !test_case.duplicate {
						width, height = 1080, 1920
					}
				}
				data := playback_test_mp4(0, width, height)
				header := http.Header{"Content-Range": {fmt.Sprintf("bytes 0-%d/%d", len(data)-1, width*1000)}}
				return &http.Response{StatusCode: 206, Header: header, Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data))}, nil
			})}
			result, err := client.Fetch("https://www.douyin.com/video/123")
			if err != nil {
				t.Fatal(err)
			}
			var detail DouyinWebVideoProfileResp
			if err := json.Unmarshal(result.(json.RawMessage), &detail); err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(result.(json.RawMessage), []byte(`"numeric_id":7707979101957383653`)) {
				t.Fatal("enrichment must preserve large numeric IDs")
			}
			if len(detail.AwemeDetail.Video.BitRate) != test_case.want_count || requests != test_case.want_requests || detail.AwemeDetail.Desc != "retained title" {
				t.Fatalf("variants=%d requests=%d title=%s", len(detail.AwemeDetail.Video.BitRate), requests, detail.AwemeDetail.Desc)
			}
			if test_case.want_requests > 0 && !test_case.probe_failure {
				if detail.AwemeDetail.Video.BitRate[0].PlayAddr.Width != 576 || detail.AwemeDetail.Video.BitRate[1].PlayAddr.Width != 720 {
					t.Fatal("use measured dimensions rather than the source dimensions or ratio label")
				}
			}
			if test_case.want_requests == 3 {
				enriched_router := client.enrich_playback_variants(json.RawMessage(router))
				var legacy struct {
					LoaderData map[string]json.RawMessage `json:"loaderData"`
				}
				if err := json.Unmarshal(enriched_router, &legacy); err != nil || len(legacy.LoaderData) != 1 {
					t.Fatal("legacy router envelopes must remain usable")
				}
				var page struct {
					VideoInfoRes struct {
						ItemList []AwemeDetail `json:"item_list"`
					} `json:"videoInfoRes"`
				}
				if err := json.Unmarshal(legacy.LoaderData["video_(id)/page"], &page); err != nil || len(page.VideoInfoRes.ItemList) != 1 || len(page.VideoInfoRes.ItemList[0].Video.BitRate) != test_case.want_count {
					t.Fatal("legacy router results must expose the same measured variants")
				}
			}
		})
	}
}
