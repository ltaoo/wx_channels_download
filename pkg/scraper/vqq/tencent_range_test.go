// Tests adapted from dramadl; see ../NOTICE and ../LICENSE.
package vqq

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/iotest"
	sourceutil "wx_channel/pkg/scraper/internal/source"
)

func TestContentRangeValidation(t *testing.T) {
	for _, value := range []string{"", "bytes 0-8/*", "bytes 9-8/100", "bytes -1-8/100", "bytes 0-100/100", "bytes 0-8/100 extra", "bytes 0-8/9223372036854775808"} {
		if _, _, _, err := parse_content_range(value); err == nil {
			t.Errorf("accepted invalid range %q", value)
		}
	}
	start, end, total, err := parse_content_range("bytes 8183-16365/30000")
	if err != nil || start != 8183 || end != 16365 || total != 30000 {
		t.Fatal("valid range rejected")
	}
}

func range_mock(t *testing.T, mode string, data []byte) *backend {
	t.Helper()
	return new_test_backend(&backend{RequestClient: &sourceutil.RequestClient{Policy: source_policy, Cookie: "account=private", HTTPClient: &http.Client{Transport: round_trip_func(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Cookie") != "" {
			t.Error("cookie leaked to media")
		}
		response := &http.Response{StatusCode: 206, Header: make(http.Header), Request: req}
		var start, end int64
		if _, err := fmt.Sscanf(req.Header.Get("Range"), "bytes=%d-%d", &start, &end); err != nil {
			t.Fatal("missing range")
		}
		end = min_int64(end, int64(len(data))-1)
		if start < 0 || start > end || end-start >= tencent_range_size {
			t.Fatal("invalid requested byte span")
		}
		total := len(data)
		response.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
		response.Header.Set("ETag", `"stable"`)
		content := data[start : end+1]
		if start > 0 {
			switch mode {
			case "offset":
				response.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start+1, end+1, total))
			case "length":
				response.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total+1))
			case "version":
				response.Header.Set("ETag", `"changed"`)
			case "ignored":
				response.StatusCode = 200
			case "truncated":
				content = content[:len(content)/2]
			}
		}
		response.ContentLength = end - start + 1
		response.Body = io.NopCloser(bytes.NewReader(content))
		return response, nil
	})}}, small_ranges: true})
}

func TestTencentRangesReconstructAndRejectChangedResources(t *testing.T) {
	data := bytes.Repeat([]byte("synthetic transport payload\x00"), 2000)
	address := "https://sourceutil.tc.qq.com/one.ts"
	for _, mode := range []string{"valid", "offset", "length", "version", "ignored", "truncated"} {
		t.Run(mode, func(t *testing.T) {
			client := range_mock(t, mode, data)
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, address, nil)
			response, err := (&range_transport{client: client}).RoundTrip(req)
			var got []byte
			if err == nil {
				got, err = io.ReadAll(response.Body)
				response.Body.Close()
			}
			if mode == "valid" {
				if err != nil {
					t.Fatal(err)
				}
				if err != nil || !bytes.Equal(got, data) {
					t.Fatal("ranged bytes differ from source")
				}
			} else {
				if err == nil {
					t.Fatal("accepted inconsistent ranges")
				}
			}
		})
	}
}

func TestTencentProbeDetectsOneFrameResponses(t *testing.T) {
	data := bytes.Repeat([]byte{0x47}, 40000)
	client := range_mock(t, "valid", data)
	transport := client.HTTPClient.Transport
	client.small_ranges = false
	client.HTTPClient.Transport = round_trip_func(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Range") == "" {
			return &http.Response{StatusCode: 200, Header: make(http.Header), ContentLength: int64(len(data)), Request: req, Body: io.NopCloser(io.MultiReader(bytes.NewReader(data[:tencent_range_size]), iotest.ErrReader(context.DeadlineExceeded)))}, nil
		}
		return transport.RoundTrip(req)
	})
	playlist := "#EXTM3U\n#EXTINF:2,\nhttps://sourceutil.tc.qq.com/one.ts\n#EXT-X-ENDLIST\n"
	if !client.probe_hls(context.Background(), playlist) || !client.small_ranges {
		t.Fatal("bounded ranges were not selected for stalled stream")
	}
}

func TestTencentRangeCancellation(t *testing.T) {
	client := range_mock(t, "valid", []byte(strings.Repeat("x", 9000)))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://sourceutil.tc.qq.com/one.ts", nil)
	if response, err := (&range_transport{client: client}).RoundTrip(req); err == nil {
		response.Body.Close()
		t.Fatal("cancelled range completed")
	}

}
