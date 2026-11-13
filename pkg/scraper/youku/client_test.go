package youku

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

const test_youku_id = "XNjU0MjIxOTc4NA=="

func TestYoukuDenialAttemptsOfficialJSExtraction(t *testing.T) {
	calls := 0
	client := NewClient("")
	client.SetHTTPClient(&http.Client{Transport: round_trip_func(func(req *http.Request) (*http.Response, error) {
		calls++
		body := `{"data":{"error":{"code":-6004}}}`
		header := make(http.Header)
		header.Set("ETag", `"fixture"`)
		return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})})
	_, err := client.Fetch("https://v.youku.com/v_show/id_" + test_youku_id + ".html")
	if err == nil || calls != 5 || !strings.Contains(err.Error(), "-6004") || !strings.Contains(err.Error(), "JS") {
		t.Fatalf("missing native JS extraction attempt: calls=%d err=%v", calls, err)
	}
}
