package protocol

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"wx_channel/pkg/hermes"
)

func TestRedirectedProbeRetainsUsableEndpointAndSafeHeaders(t *testing.T) {
	const media = "0123456789"
	var entry_requests int32
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("entry credentials leaked to the CDN")
		}
		if r.Header.Get("User-Agent") != "fixture browser" {
			t.Error("download headers were lost")
		}
		if r.Header.Get("Referer") != "https://www.douyin.com/" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		http.ServeContent(w, r, "video.mp4", time.Time{}, strings.NewReader(media))
	}))
	defer cdn.Close()
	cdn_url := strings.Replace(cdn.URL, "127.0.0.1", "localhost", 1) + "/video.mp4"
	entry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&entry_requests, 1) != 1 {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		http.Redirect(w, r, cdn_url, http.StatusFound)
	}))
	defer entry.Close()
	driver := NewHTTPDriver()
	endpoint := hermes.Endpoint{ID: 7, Protocol: "http", URL: entry.URL, Headers: map[string]string{"Authorization": "Bearer private", "User-Agent": "fixture browser", "Referer": "https://www.douyin.com/"}, Cookies: "session=private"}
	prepared, err := driver.Prepare(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.ResolvedEndpoint == nil || prepared.ResolvedEndpoint.URL != cdn_url || prepared.ResolvedEndpoint.ID != endpoint.ID {
		t.Fatalf("missing resolved endpoint: %+v", prepared.ResolvedEndpoint)
	}
	if prepared.ResolvedEndpoint.Cookies != "" || prepared.ResolvedEndpoint.Headers["Range"] != "" {
		t.Fatal("probe cookies or byte range were copied into subsequent reads")
	}
	for _, request := range []hermes.ReadRequest{
		{UseRange: true, OffsetStart: 3, OffsetEnd: 7},
		{},
	} {
		reader, err := driver.Open(context.Background(), *prepared.ResolvedEndpoint, request)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(reader)
		reader.Close()
		want := media
		if request.UseRange {
			want = media[3:8]
		}
		if err != nil || string(body) != want {
			t.Fatalf("download = %q, want %q: %v", body, want, err)
		}
	}
	if atomic.LoadInt32(&entry_requests) != 1 {
		t.Fatalf("entry was requested %d times", entry_requests)
	}
}

func TestRedirectRefererPreservesSafetyAndLimit(t *testing.T) {
	original, _ := http.NewRequest("GET", "https://entry.example/video", nil)
	original.Header.Set("Referer", "https://www.douyin.com/")
	redirected, _ := http.NewRequest("GET", "https://cdn.example/video", nil)
	redirected.Header.Set("Referer", original.URL.String())
	if err := preserve_redirect_referer(redirected, []*http.Request{original}); err != nil || redirected.Header.Get("Referer") != "https://www.douyin.com/" {
		t.Fatal("redirect replaced the configured Referer")
	}
	redirected.URL.Scheme = "http"
	if err := preserve_redirect_referer(redirected, []*http.Request{original}); err != nil || redirected.Header.Get("Referer") != "" {
		t.Fatal("HTTPS Referer leaked on an HTTP downgrade")
	}
	if err := preserve_redirect_referer(redirected, make([]*http.Request, 10)); err == nil {
		t.Fatal("redirect limit was disabled")
	}
}

func TestEngineReusesSuccessfulProbeRedirect(t *testing.T) {
	const media = "probe and download the same media"
	var entry_requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/entry" {
			if atomic.AddInt32(&entry_requests, 1) > 1 {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			http.Redirect(w, r, "/media", http.StatusFound)
			return
		}
		http.ServeContent(w, r, "fixture.mp4", time.Time{}, strings.NewReader(media))
	}))
	defer server.Close()
	engine := hermes.New(hermes.HermesNewConfig{Config: hermes.HermesEngineConfig{BasePath: t.TempDir()}})
	engine.RegisterProtocol(NewHTTPDriver())
	task := engine.CreateTask(server.URL+"/entry", hermes.WithFilename("fixture"))
	select {
	case <-task.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("download did not finish")
	}
	if err := task.Err(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(task.FilePath())
	if err != nil || string(body) != media {
		t.Fatalf("download = %q, error = %v", body, err)
	}
	if atomic.LoadInt32(&entry_requests) != 1 {
		t.Fatal("download returned to the entry URL after probing")
	}
}
