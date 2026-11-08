package youku

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentClientKeepsPlaybackStateSeparate(t *testing.T) {
	arrived := make(chan struct{}, 2)
	release := make(chan struct{})
	client := NewClient("session=fixture")
	client.SetHTTPClient(&http.Client{Transport: round_trip_func(func(req *http.Request) (*http.Response, error) {
		body := ""
		headers := make(http.Header)
		switch req.URL.Path {
		case "/eg.js":
			headers.Set("ETag", `"fixture-utid"`)
		case "/ups/get.json":
			id := req.URL.Query().Get("vid")
			arrived <- struct{}{}
			<-release
			body = fmt.Sprintf(`{"data":{"video":{"encodeid":%q,"title":"测试","seconds":2},"stream":[{"m3u8_url":"https://pl-ali.youku.com/%s/index.m3u8","width":32,"height":32,"milliseconds_video":2000}]}}`, id, id)
		default:
			if req.URL.Hostname() == "pl-ali.youku.com" {
				id := strings.Split(strings.Trim(req.URL.Path, "/"), "/")[0]
				if !strings.Contains(req.Header.Get("Referer"), id) {
					t.Errorf("mixed playback referer for %s", id)
				}
				body = "#EXTM3U\n#EXTINF:2,\nsegment.ts\n#EXT-X-ENDLIST\n"
			}
		}
		return &http.Response{StatusCode: 200, Header: headers, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body)), Request: req}, nil
	})})
	var workers sync.WaitGroup
	for _, id := range []string{"XNjU0MjIxOTc4NA==", "XOtherVideo123"} {
		workers.Add(1)
		go func(id string) {
			defer workers.Done()
			if _, err := client.Fetch("https://v.youku.com/v_show/id_" + id + ".html"); err != nil {
				t.Error(err)
			}
		}(id)
	}
	<-arrived
	<-arrived
	close(release)
	workers.Wait()
}
